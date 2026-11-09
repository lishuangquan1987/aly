package client

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	quickTimeout      = 30 * time.Second
	downloadIdle      = 90 * time.Second
	applyTimeout      = 900 * time.Second
	selfUpdateTimeout = 20 * time.Second
)

// Options 控制单次调用。
type Options struct {
	TotalTimeout time.Duration // 0 = 不设总超时
	IdleTimeout  time.Duration // >0：该时长无 stdout 行则杀子进程
}

// Client 以子进程方式调用 Linux 的 aly-client（无 .exe）。
type Client struct {
	exePath string
}

func New(exePath string) *Client { return &Client{exePath: exePath} }

func (c *Client) ExePath() string { return c.exePath }
func (c *Client) ExeDir() string  { return filepath.Dir(c.exePath) }

// Run 执行命令并返回最后一个结果行；进度行通过 onProgress 回调。
func (c *Client) Run(ctx context.Context, args []string, opt Options,
	onProgress func(Progress)) (Parsed, error) {
	if opt.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.TotalTimeout)
		defer cancel()
	}

	cmd := exec.Command(c.exePath, args...)
	cmd.Dir = c.ExeDir()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Parsed{}, fmt.Errorf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Parsed{}, fmt.Errorf("start %s: %v", filepath.Base(c.exePath), err)
	}

	killFn := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	stopWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killFn()
		case <-stopWatch:
		}
	}()
	defer close(stopWatch)

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
				onProgress(*p.Progress)
			}
		case KindDone, KindResult, KindFail:
			result = p
			found = true
		}
	}
	waitErr := cmd.Wait()

	if !found {
		stderrText := strings.TrimSpace(stderr.String())
		if waitErr != nil {
			if stderrText != "" {
				return Parsed{}, fmt.Errorf("%s failed: %v: %s",
					filepath.Base(c.exePath), waitErr, stderrText)
			}
			return Parsed{}, fmt.Errorf("%s failed: %v",
				filepath.Base(c.exePath), waitErr)
		}
		if stderrText != "" {
			return Parsed{}, fmt.Errorf("%s: %s", filepath.Base(c.exePath), stderrText)
		}
		return Parsed{}, fmt.Errorf("%s: no JSON output", filepath.Base(c.exePath))
	}
	// stdout 有结果行但退出码非 0：以结果为准（与 winui/C# SDK 一致）。
	return result, nil
}

func (c *Client) CheckUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"check_update"}, Options{TotalTimeout: quickTimeout}, nil)
}

func (c *Client) CheckSelfUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"check_self_update"}, Options{TotalTimeout: quickTimeout}, nil)
}

func (c *Client) SelfUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"self_update"}, Options{TotalTimeout: selfUpdateTimeout}, nil)
}

func (c *Client) DownloadUpdate(ctx context.Context, onProgress func(Progress)) (Parsed, error) {
	return c.Run(ctx, []string{"download_update"},
		Options{IdleTimeout: downloadIdle}, onProgress)
}

func (c *Client) ApplyUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"apply_update"}, Options{TotalTimeout: applyTimeout}, nil)
}

func (c *Client) ListRollbackVersions(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"list_rollback_versions"}, Options{TotalTimeout: quickTimeout}, nil)
}

func (c *Client) Rollback(ctx context.Context, version string) (Parsed, error) {
	return c.Run(ctx, []string{"rollback", "--version", version},
		Options{TotalTimeout: applyTimeout}, nil)
}
