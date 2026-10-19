// +build windows

package bridge

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// createNoWindow 是 Win32 的 CREATE_NO_WINDOW 标志。
const createNoWindow = 0x08000000

// Options 控制单次调用。
type Options struct {
	// TotalTimeout 为 0 表示不设总超时（下载用）。
	TotalTimeout time.Duration
	// IdleTimeout 为 0 表示不做活性检测；
	// >0 时若超过该时长没有任何 stdout 行，则判定子进程卡死并结束它。
	IdleTimeout time.Duration
}

// ProgressInfo 是一次进度快照（从 DownloadProgress 精简而来）。
type ProgressInfo struct {
	Index  int
	Total  int
	File   string
	Status string
}

// Client 以子进程方式调用 aly-client.exe。
type Client struct {
	exePath string
}

// New 创建 Client；exePath 为 aly-client.exe 的路径。
func New(exePath string) *Client {
	return &Client{exePath: exePath}
}

// ExePath 返回 aly-client.exe 路径。
func (c *Client) ExePath() string {
	return c.exePath
}

// ExeDir 返回 aly-client.exe 所在目录（即 UpdateFolder）。
func (c *Client) ExeDir() string {
	return filepath.Dir(c.exePath)
}

// Run 执行命令并返回最终结果。
//
// 输出契约（DEVELOPMENT_PLAN §3.3）：
//   - 逐行读取 stdout；读到 EOF 即结束循环（**绝不忙等**）
//   - 非 JSON 行跳过，不中断
//   - 拿到任何进度/结果行即算 found；否则报错
//   - stdout 有结果但退出码非 0：以结果为准（与 C# SDK 一致）
//   - stderr 只并入错误文本，不单独返回
//   - ctx 取消 / 总超时 / 空闲超时：杀掉子进程并报错
func (c *Client) Run(ctx context.Context, args []string, opt Options, onProgress func(ProgressInfo)) (Parsed, error) {
	if opt.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.TotalTimeout)
		defer cancel()
	}

	cmd := exec.Command(c.exePath, args...)
	cmd.Dir = c.ExeDir() // 对齐 C# SDK，且保证 client.json 相对路径解析正确
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Parsed{}, fmt.Errorf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Parsed{}, fmt.Errorf("start %s: %v", filepath.Base(c.exePath), err)
	}

	// killFn 只终止**直接子进程**，不按进程树杀。
	//
	// 这是有意的：apply_update 成功后会 launchMainExe 拉起主程序，
	// 若按进程树（Job Object）终止，会把刚启动的主程序一起杀掉。
	// aly-client.exe 的下载/检查/回滚路径本身不产生子进程，
	// 因此单个 TerminateProcess 足以取消这些操作。
	killFn := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	// ctx 取消（含总超时）→ 杀进程
	stopWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killFn()
		case <-stopWatch:
		}
	}()
	defer close(stopWatch)

	// 空闲超时（下载场景：长时间无进度行视为卡死）
	var idleTimer *time.Timer
	if opt.IdleTimeout > 0 {
		idleTimer = time.AfterFunc(opt.IdleTimeout, killFn)
		defer idleTimer.Stop()
	}

	var (
		result Parsed
		found  bool
	)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		if idleTimer != nil {
			idleTimer.Reset(opt.IdleTimeout)
		}
		p, _ := ParseLine(sc.Text())
		switch p.Kind {
		case KindProgress:
			if onProgress != nil && p.Progress != nil {
				onProgress(ProgressInfo{
					Index:  p.Progress.Index,
					Total:  p.Progress.Total,
					File:   p.Progress.File,
					Status: p.Progress.Status,
				})
			}
		case KindDone, KindResult, KindFail:
			// 末行优先：下载场景下 Fail 会覆盖之前的进度
			result = p
			found = true
		}
	}
	// 到这里只可能是 EOF 或进程被杀，不存在忙等。

	waitErr := cmd.Wait()

	if !found {
		stderrText := strings.TrimSpace(stderr.String())
		switch {
		case ctx.Err() == context.DeadlineExceeded:
			return Parsed{}, fmt.Errorf("%s %v: timed out", filepath.Base(c.exePath), args)
		case ctx.Err() == context.Canceled:
			return Parsed{}, fmt.Errorf("%s %v: canceled", filepath.Base(c.exePath), args)
		case opt.IdleTimeout > 0 && waitErr != nil && stderrText == "":
			return Parsed{}, fmt.Errorf("%s %v: no output for %v, giving up",
				filepath.Base(c.exePath), args, opt.IdleTimeout)
		}
		code := exitCode(waitErr)
		if stderrText == "" {
			return Parsed{}, fmt.Errorf("%s %v: exited with code %d",
				filepath.Base(c.exePath), args, code)
		}
		return Parsed{}, fmt.Errorf("%s %v: exited with code %d: %s",
			filepath.Base(c.exePath), args, code, stderrText)
	}

	// 有结果就以结果为准；退出码/ stderr 仅丢弃（调用方可从 Success 判断）
	return result, nil
}

// exitCode 从 Wait 的 error 中取出退出码；无错误时为 0，无法判定时为 -1。
//
// 注意：不能用 exec.ExitError.ExitCode() —— 那是 Go 1.12 才加的 API。
// Go 1.10 必须走 Sys() -> syscall.WaitStatus.ExitStatus()。
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok2 := ee.Sys().(syscall.WaitStatus); ok2 {
			return ws.ExitStatus()
		}
	}
	return -1
}
