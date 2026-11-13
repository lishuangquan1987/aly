package alyclient

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Status 是更新生命周期阶段。
type Status string

const (
	StatusNone              Status = "none"
	StatusDiscoveredUpdate  Status = "discoveredUpdate"
	StatusDownloadingUpdate Status = "downloadingUpdate"
	StatusDownloadedUpdate  Status = "downloadedUpdate"
	StatusApplyUpdate       Status = "applyUpdate"
)

// UpdateClient 是 aly-client 的托管生命周期：自更新 → check → download → apply。
type UpdateClient struct {
	API *API

	// IdlePollInterval 空闲轮询间隔：默认 300s，最小 60s，叠加 0–60s 抖动。
	IdlePollInterval time.Duration
	// MustCloseProcessNames 传给 apply_update，逗号分隔、不带 .exe。
	MustCloseProcessNames string
	// CloseTimeoutSeconds 关闭进程超时；0 或 30 时不传参。
	CloseTimeoutSeconds int

	// OnStatusChange 每次状态迁移触发；消息含文件名与百分比。
	OnStatusChange func(status Status, message string)
	// OnError 错误出现/变化时触发；恢复时传空串。
	OnError func(message string)
	// ConfirmDownload 非强制更新的确认；返回即确认，未设置视为自动确认。
	ConfirmDownload func(ctx context.Context, version string)
	// ConfirmApply 应用确认；返回即确认。
	ConfirmApply func(ctx context.Context, version string)

	mu       sync.Mutex
	status   Status
	errMsg   string
	running  bool
	stopping bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewUpdateClient 创建托管客户端；path 为空时使用 DefaultPath。
func NewUpdateClient(path string) *UpdateClient {
	return &UpdateClient{
		API:              NewAPI(path),
		IdlePollInterval: 300 * time.Second,
		status:           StatusNone,
	}
}

// Status 返回当前阶段。
func (c *UpdateClient) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// LastError 返回最近一次错误；空串表示已恢复。
func (c *UpdateClient) LastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errMsg
}

// IsRunning 返回后台循环是否在运行。
func (c *UpdateClient) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Start 启动后台循环（立即返回）。重复调用无副作用。
func (c *UpdateClient) Start() {
	c.mu.Lock()
	if c.running || c.stopping {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.running = true
	c.wg.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.wg.Done()
		c.loop(ctx)
	}()
}

// Stop 停止轮询并等待循环退出。不结束 apply_update 拉起的进程，也不删除文件。
func (c *UpdateClient) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.stopping = true
	cancel := c.cancel
	c.mu.Unlock()

	cancel()
	c.wg.Wait()

	c.mu.Lock()
	c.running = false
	c.stopping = false
	c.mu.Unlock()
}

func (c *UpdateClient) loop(ctx context.Context) {
	c.trySelfUpdate(ctx)
	for ctx.Err() == nil {
		check := c.API.CheckUpdate(ctx)
		if ctx.Err() != nil {
			return
		}
		if !check.IsSuccess {
			c.reportError(check.ErrorMsg)
			if !sleepCtx(ctx, 5*time.Second) {
				return
			}
			continue
		}
		c.clearError()

		data := check.Data
		if !data.HasUpdate {
			if c.Status() != StatusNone {
				c.setStatus(StatusNone, "Update status cleared")
			}
			if !sleepCtx(ctx, c.idleDelay()) {
				return
			}
			continue
		}

		version := strings.TrimSpace(data.NewVersion)
		// 三态分派（与 C# UpdateClient 一致）：HasUpdate && !NeedDownloadUpdate
		// 表示本地已下载待应用 / applying 崩溃恢复 / 回滚续跑，直接 ApplyUpdate，
		// 不下载、不询问（恢复场景自愈）。
		if data.NeedDownloadUpdate {
			message := "Found new version"
			if version != "" {
				message += " " + version
			}
			c.setStatus(StatusDiscoveredUpdate, message)
			if !data.ForceUpdate {
				if !c.waitConfirm(ctx, c.ConfirmDownload, version) {
					return
				}
			}

			c.setStatus(StatusDownloadingUpdate, "Downloading update...")
			downloaded := c.API.DownloadUpdate(ctx, DownloadOptions{
				OnProgress: func(info DownloadProgressInfo) {
					c.setStatus(
						StatusDownloadingUpdate,
						fmt.Sprintf("Downloading %s... %d%%", info.File, int(info.Ratio*100)),
					)
				},
			})
			if ctx.Err() != nil {
				return
			}
			if !downloaded.IsSuccess {
				if strings.Contains(downloaded.ErrorMsg, "already at latest version") {
					c.setStatus(StatusNone, "Update status cleared")
					if !sleepCtx(ctx, c.idleDelay()) {
						return
					}
					continue
				}
				c.reportError(downloaded.ErrorMsg)
				if !sleepCtx(ctx, time.Second) {
					return
				}
				continue
			}
		}

		c.setStatus(StatusDownloadedUpdate, "Update downloaded")
		if !data.ForceUpdate {
			if !c.waitConfirm(ctx, c.ConfirmApply, version) {
				return
			}
		}

		c.setStatus(StatusApplyUpdate, "Applying update...")
		applied := c.API.ApplyUpdate(CloseOptions{
			MustCloseProcessNames: c.MustCloseProcessNames,
			CloseTimeoutSeconds:   c.CloseTimeoutSeconds,
		})
		if !applied.IsSuccess {
			c.reportError("apply spawn failed: " + applied.ErrorMsg)
		}
		if !sleepCtx(ctx, 30*time.Second) {
			return
		}
	}
}

func (c *UpdateClient) trySelfUpdate(ctx context.Context) {
	check := c.API.CheckSelfUpdate(ctx)
	if ctx.Err() != nil || !check.IsSuccess || !check.Data.NeedUpdate {
		return
	}
	updated := c.API.SelfUpdate(ctx)
	if !updated.IsSuccess {
		c.reportError(updated.ErrorMsg)
	}
}

func (c *UpdateClient) waitConfirm(
	ctx context.Context,
	callback func(context.Context, string),
	version string,
) bool {
	if callback == nil {
		return true // 未设置确认回调视为已确认（与 C#/TS 一致）
	}
	done := make(chan struct{})
	go func() {
		defer func() {
			_ = recover()
			close(done)
		}()
		callback(ctx, version)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *UpdateClient) setStatus(status Status, message string) {
	c.mu.Lock()
	c.status = status
	callback := c.OnStatusChange
	c.mu.Unlock()
	if callback != nil {
		func() {
			defer func() { _ = recover() }()
			callback(status, message)
		}()
	}
}

func (c *UpdateClient) reportError(message string) {
	c.mu.Lock()
	if c.errMsg == message {
		c.mu.Unlock()
		return
	}
	c.errMsg = message
	callback := c.OnError
	c.mu.Unlock()
	if callback != nil {
		func() {
			defer func() { _ = recover() }()
			callback(message)
		}()
	}
}

func (c *UpdateClient) clearError() {
	c.mu.Lock()
	hadError := c.errMsg != ""
	c.errMsg = ""
	callback := c.OnError
	c.mu.Unlock()
	if hadError && callback != nil {
		func() {
			defer func() { _ = recover() }()
			callback("")
		}()
	}
}

func (c *UpdateClient) idleDelay() time.Duration {
	interval := c.IdlePollInterval
	if interval <= 0 {
		interval = 300 * time.Second
	}
	if interval < 60*time.Second {
		interval = 60 * time.Second
	}
	jitter := time.Duration(rand.Intn(61)) * time.Second
	return interval + jitter
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
