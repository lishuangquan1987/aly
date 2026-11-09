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
	// 客户端现对单文件大下载每 3s 输出 PROGRESS 行（download_update.go），
	// "无输出"即真停流，不会误杀大文件下载。
	idleTimeout = 90 * time.Second
	// applyTimeout 用于 apply/rollback：内部含 3 次重试 ×2s、
	// 目录复制、深扫（10–60s/路径）与 close-timeout，原 600s。
	// 提到 900s：病态深扫场景（每路径 60s × 多路径 + 杀进程重试 + 哈希校验）
	// 600s 偏紧，超时被杀会把 apply 留在中间态（虽有崩溃恢复兜底，体验差）。
	applyTimeout = 900 * time.Second
	// selfUpdateTimeout 用于 self_update：命令内部最多 3 次复制 + 3 次落位重试
	//（各 1s 间隔）加两次 SHA256，正常 <10s。超时只作最后兜底——命令已改为
	//“先备后换”，被 kill 时 self 通常完好；若恰在两连 rename 之间，则下一次
	// 启动由 UI/SDK 的兜底落位恢复。
	selfUpdateTimeout = 20 * time.Second
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

// SelfUpdate 调用 self_update（20s）：用主程序目录的新版更新器替换自身。
//
// UI 的 UpdateUpdater 正常路径统一走本命令；若命令不可用（旧版更新器不认识
// self_update / 进程崩溃），调用方据 err（而非 Parsed.Success）决定是否回退
// 到直接复制。锁被占用等业务失败会以 JSON isSuccess=false 返回（err==nil）。
func (c *Client) SelfUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"self_update"}, Options{TotalTimeout: selfUpdateTimeout}, nil)
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
