package alyclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// DefaultShortTimeout 短命令默认超时。
	DefaultShortTimeout = 30 * time.Second
	// DefaultDownloadStallTimeout 下载停流看门狗默认值。
	DefaultDownloadStallTimeout = 1800 * time.Second
	// RollbackTimeout 回滚默认超时。
	RollbackTimeout = 600 * time.Second
)

// CloseOptions 对应 apply_update / rollback 的进程关闭参数。
type CloseOptions struct {
	// MustCloseProcessNames 逗号分隔、不带 .exe 的进程名。
	MustCloseProcessNames string
	// CloseTimeoutSeconds 关闭进程超时；0 或 30 时不传参。
	CloseTimeoutSeconds int
}

// CleanOptions 对应 clean 命令参数。
type CleanOptions struct {
	Keep int
	All  bool
}

// DownloadOptions 控制下载过程。
type DownloadOptions struct {
	// StallTimeout 停流看门狗；<=0 时使用 DefaultDownloadStallTimeout。
	StallTimeout time.Duration
	// OnProgress 每个 DONE 行触发（文件计数粒度）。
	OnProgress func(DownloadProgressInfo)
}

// DownloadProgressInfo 是进度的宿主视图。
type DownloadProgressInfo struct {
	File            string
	Ratio           float64
	Status          DownloadProgressStatus
	Index           int64
	Total           int64
	DownloadedBytes int64
	TotalBytes      int64
	SpeedBps        int64
}

// API 封装一个 aly-client 可执行文件。Path 为空时使用 DefaultPath。
type API struct {
	Path string
	// Env 追加到宿主环境（不是替换）；为空则继承。
	Env []string
}

// NewAPI 创建 API；path 为空时使用 DefaultPath。
func NewAPI(path string) *API {
	if path == "" {
		path = DefaultPath()
	}
	return &API{Path: filepath.Clean(path)}
}

// DefaultPath 返回 <可执行文件目录>/../UpdateFolder/aly-client[.exe]。
func DefaultPath() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	name := "aly-client"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "UpdateFolder", name))
}

func (a *API) path() string {
	if a.Path == "" {
		return DefaultPath()
	}
	return a.Path
}

func (a *API) childEnv() []string {
	if len(a.Env) == 0 {
		return nil
	}
	return append(os.Environ(), a.Env...)
}

type runResult struct {
	stdout   []byte
	stderr   []byte
	code     int
	timedOut bool
	canceled bool
	startErr error
}

func (a *API) runCollect(ctx context.Context, timeout time.Duration, args ...string) runResult {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, a.path(), args...)
	cmd.Dir = filepath.Dir(a.path())
	cmd.Env = a.childEnv()
	hideWindow(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res := runResult{}
	err := cmd.Run()
	res.stdout = stdout.Bytes()
	res.stderr = stderr.Bytes()
	if err == nil {
		return res
	}

	res.code = -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.code = exitErr.ExitCode()
	} else {
		res.startErr = err
	}
	switch runCtx.Err() {
	case context.DeadlineExceeded:
		res.timedOut = true
	case context.Canceled:
		res.canceled = true
	}
	return res
}

func parseResponse[T any](res runResult, timeout time.Duration) Response[T] {
	if res.canceled {
		return NG[T]("aly-client call canceled")
	}
	if res.timedOut {
		return NG[T](fmt.Sprintf("aly-client timed out after %ds and was killed", int(timeout/time.Second)))
	}
	if res.startErr != nil {
		return NG[T](res.startErr.Error())
	}
	text := strings.TrimSpace(string(res.stdout))
	if text == "" {
		errText := strings.TrimSpace(string(res.stderr))
		if res.code != 0 {
			if errText != "" {
				return NG[T](errText)
			}
			return NG[T](fmt.Sprintf("aly-client exited with code %d (may require admin)", res.code))
		}
		return NG[T]("aly-client returned no output (may require admin)")
	}
	var response Response[T]
	if err := json.Unmarshal([]byte(text), &response); err != nil {
		return NG[T](err.Error())
	}
	return response
}

// CheckUpdate 检查应用是否有新版本。服务器不可达时 HasUpdate=false，不报错。
func (a *API) CheckUpdate(ctx context.Context) Response[CheckUpdateData] {
	res := a.runCollect(ctx, DefaultShortTimeout, "check_update")
	return parseResponse[CheckUpdateData](res, DefaultShortTimeout)
}

// CheckSelfUpdate 检查更新器自身副本是否需要更新。
func (a *API) CheckSelfUpdate(ctx context.Context) Response[CheckSelfUpdateData] {
	res := a.runCollect(ctx, DefaultShortTimeout, "check_self_update")
	return parseResponse[CheckSelfUpdateData](res, DefaultShortTimeout)
}

// SelfUpdate 执行更新器自更新。
func (a *API) SelfUpdate(ctx context.Context) Response[SelfUpdateData] {
	res := a.runCollect(ctx, DefaultShortTimeout, "self_update")
	return parseResponse[SelfUpdateData](res, DefaultShortTimeout)
}

// ListRollbackVersions 列出可回滚的历史版本。
func (a *API) ListRollbackVersions(ctx context.Context) Response[RollbackListData] {
	res := a.runCollect(ctx, DefaultShortTimeout, "list_rollback_versions")
	return parseResponse[RollbackListData](res, DefaultShortTimeout)
}

// CheckDiff 列出需要下载的差异文件（调试/可视化）。
func (a *API) CheckDiff(ctx context.Context) Response[CheckDiffData] {
	res := a.runCollect(ctx, DefaultShortTimeout, "check_diff")
	return parseResponse[CheckDiffData](res, DefaultShortTimeout)
}

// Clean 清理历史快照与临时文件。
func (a *API) Clean(ctx context.Context, options CleanOptions) Response[CleanData] {
	args := []string{"clean"}
	if options.All {
		args = append(args, "--all")
	} else if options.Keep > 0 {
		args = append(args, "--keep", fmt.Sprintf("%d", options.Keep))
	}
	res := a.runCollect(ctx, DefaultShortTimeout, args...)
	return parseResponse[CleanData](res, DefaultShortTimeout)
}

func appendCloseArgs(args []string, options CloseOptions) []string {
	if names := strings.TrimSpace(options.MustCloseProcessNames); names != "" {
		args = append(args, "--must-close-process-name", names)
	}
	if options.CloseTimeoutSeconds != 0 && options.CloseTimeoutSeconds != 30 {
		args = append(args, "--close-timeout", fmt.Sprintf("%d", options.CloseTimeoutSeconds))
	}
	return args
}

// Rollback 回滚到指定版本（默认 600s 超时）。
func (a *API) Rollback(ctx context.Context, version string, options CloseOptions) Response[RollbackData] {
	args := appendCloseArgs([]string{"rollback", "--version", version}, options)
	res := a.runCollect(ctx, RollbackTimeout, args...)
	return parseResponse[RollbackData](res, RollbackTimeout)
}

// DownloadUpdate 下载差异文件。没有总时长上限，只有停流看门狗。
func (a *API) DownloadUpdate(ctx context.Context, options DownloadOptions) Response[Empty] {
	stall := options.StallTimeout
	if stall <= 0 {
		stall = DefaultDownloadStallTimeout
	}

	cmd := exec.Command(a.path(), "download_update")
	cmd.Dir = filepath.Dir(a.path())
	cmd.Env = a.childEnv()
	hideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return NG[Empty](err.Error())
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return NG[Empty](err.Error())
	}

	done := make(chan struct{})
	defer close(done)
	lines := make(chan string, 256)
	scanErr := make(chan error, 1)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			select {
			case lines <- line:
			case <-done:
				return
			}
		}
		scanErr <- scanner.Err()
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	timer := time.NewTimer(stall)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				waitErr := <-waitCh
				if ctx.Err() != nil {
					return NG[Empty]("download canceled: " + ctx.Err().Error())
				}
				if err := <-scanErr; err != nil {
					return NG[Empty](err.Error())
				}
				_ = waitErr
				return NG[Empty]("download interrupted (aly-client exited before finish line)")
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(stall)

			var env Response[json.RawMessage]
			if err := json.Unmarshal([]byte(line), &env); err != nil {
				_ = cmd.Process.Kill()
				return NG[Empty]("invalid progress line: " + err.Error())
			}
			if !env.IsSuccess {
				// 与 C#/TS 一致：任一行失败即返回该行 errorMsg。
				_ = cmd.Process.Kill()
				if env.ErrorMsg == "" {
					return NG[Empty]("download failed")
				}
				return NG[Empty](env.ErrorMsg)
			}
			if len(bytes.TrimSpace(env.Data)) == 0 || string(bytes.TrimSpace(env.Data)) == "null" {
				return OK(Empty{})
			}
			var progress DownloadProgress
			if err := json.Unmarshal(env.Data, &progress); err != nil {
				_ = cmd.Process.Kill()
				return NG[Empty]("invalid progress data: " + err.Error())
			}
			if progress.Status == DownloadProgressStatusDone && progress.Total > 0 {
				reportProgress(options.OnProgress, DownloadProgressInfo{
					File:            progress.File,
					Ratio:           float64(progress.Index) / float64(progress.Total),
					Status:          progress.Status,
					Index:           progress.Index,
					Total:           progress.Total,
					DownloadedBytes: progress.DownloadedBytes,
					TotalBytes:      progress.TotalBytes,
					SpeedBps:        progress.SpeedBps,
				})
			}
		case <-timer.C:
			_ = cmd.Process.Kill()
			return NG[Empty](fmt.Sprintf("download stalled: no output for %ds, process killed", int(stall/time.Second)))
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return NG[Empty]("download canceled: " + ctx.Err().Error())
		}
	}
}

func reportProgress(fn func(DownloadProgressInfo), info DownloadProgressInfo) {
	if fn == nil {
		return
	}
	defer func() { _ = recover() }()
	fn(info)
}

// ApplyUpdate 拉起独立更新进程后立即返回，不等待。
// 更新器会关闭宿主并重启主程序，调用方不要在此之后做清理工作。
func (a *API) ApplyUpdate(options CloseOptions) Response[Empty] {
	args := appendCloseArgs([]string{"apply_update"}, options)
	cmd := exec.Command(a.path(), args...)
	cmd.Dir = filepath.Dir(a.path())
	cmd.Env = a.childEnv()
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return NG[Empty](err.Error())
	}
	_ = cmd.Process.Release()
	return OK(Empty{})
}
