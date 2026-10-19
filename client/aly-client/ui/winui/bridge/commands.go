// +build windows

package bridge

import (
	"context"
	"time"
)

// 超时策略（DEVELOPMENT_PLAN §3.3）。
const (
	// quickTimeout 用于一次 HTTP 往返即完成、且必须快速返回给 UI 的命令。
	quickTimeout = 30 * time.Second
	// idleTimeout 用于下载：不设总超时（大包可能很久），
	// 但只要 90s 没有任何进度行就判定卡死。
	idleTimeout = 90 * time.Second
	// applyTimeout 用于 apply/rollback：内部含 3 次重试 ×2s、
	// 目录复制、深扫（10–60s/路径）与 close-timeout，故给到 10 分钟。
	applyTimeout = 600 * time.Second
)

// 以下命令**一律不传可选参数**，由 aly-client.exe 自己读
// UpdateFolder/client.json 与 .updator/shared.json（DEVELOPMENT_PLAN §3.4）。
// 这样 UI 无需解析配置，也无需共享 aly-client 的 config 包。

// CheckUpdate 调用 check_update（30s）。
func (c *Client) CheckUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"check_update"}, Options{TotalTimeout: quickTimeout}, nil)
}

// CheckSelfUpdate 调用 check_self_update（30s）。
func (c *Client) CheckSelfUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"check_self_update"}, Options{TotalTimeout: quickTimeout}, nil)
}

// DownloadUpdate 调用 download_update：无总超时 + 90s 活性检测。
func (c *Client) DownloadUpdate(ctx context.Context, onProgress func(ProgressInfo)) (Parsed, error) {
	return c.Run(ctx, []string{"download_update"},
		Options{IdleTimeout: idleTimeout}, onProgress)
}

// ApplyUpdate 调用 apply_update（--close-timeout 用 aly-client 默认的 30s）。
func (c *Client) ApplyUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"apply_update"}, Options{TotalTimeout: applyTimeout}, nil)
}

// ListRollbackVersions 调用 list_rollback_versions（30s）。
func (c *Client) ListRollbackVersions(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"list_rollback_versions"}, Options{TotalTimeout: quickTimeout}, nil)
}

// Rollback 调用 rollback --version <version>（10 分钟）。
func (c *Client) Rollback(ctx context.Context, version string) (Parsed, error) {
	return c.Run(ctx, []string{"rollback", "--version", version},
		Options{TotalTimeout: applyTimeout}, nil)
}
