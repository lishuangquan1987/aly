// +build windows

// Package flow 实现与 C# AlyUpdateClient 对齐的更新状态机。
//
// 分层约定：flow 不 import bridge 与 view。bridge 的调用通过 Bridge 接口注入，
// UI 通知通过 Sink 接口回调（两者都由 app 层适配）。
package flow

// Status 对齐 C# AlyClientStatus（None/DiscoveredUpdate/DownloadingUpdate/
// DownloadedUpdate/ApplyUpdate），语义保持一致。
type Status int

const (
	// StatusNone 空闲：无待处理更新。
	StatusNone Status = iota
	// StatusDiscoveredUpdate 已发现新版本。
	StatusDiscoveredUpdate
	// StatusDownloadingUpdate 正在下载。
	StatusDownloadingUpdate
	// StatusDownloadedUpdate 下载完成，待应用。
	StatusDownloadedUpdate
	// StatusApplyUpdate 正在应用。
	StatusApplyUpdate
)

// String 便于日志与测试输出。
func (s Status) String() string {
	switch s {
	case StatusNone:
		return "None"
	case StatusDiscoveredUpdate:
		return "DiscoveredUpdate"
	case StatusDownloadingUpdate:
		return "DownloadingUpdate"
	case StatusDownloadedUpdate:
		return "DownloadedUpdate"
	case StatusApplyUpdate:
		return "ApplyUpdate"
	}
	return "Unknown"
}

// Action 是状态机决定的下一步动作。
type Action int

const (
	// ActionNone 不做任何事。
	ActionNone Action = iota
	// ActionDownload 需要下载。
	ActionDownload
	// ActionApply 需要应用（含崩溃恢复续跑）。
	ActionApply
)

// String 便于日志与测试输出。
func (a Action) String() string {
	switch a {
	case ActionNone:
		return "None"
	case ActionDownload:
		return "Download"
	case ActionApply:
		return "Apply"
	}
	return "Unknown"
}

// CheckView 是 check_update 结果中状态机关心的字段。
type CheckView struct {
	// HasUpdate 服务端是否有更新。
	HasUpdate bool
	// NeedDownload 是否需要下载（false 表示已下载、只差应用）。
	NeedDownload bool
	// ForceUpdate 为 nil 表示服务端未返回该字段（见 DEVELOPMENT_PLAN §3.5）。
	ForceUpdate *bool
	// Applying 表示现场是 version_status == applying（更新或回滚被中断）。
	Applying bool
	// NewVersion / CurrentVersion 仅用于展示。
	NewVersion     string
	CurrentVersion string
}

// Decision 是状态机的输出。
type Decision struct {
	Action Action
	// Ask 为 true 表示必须先询问用户（非强制更新）。
	Ask bool
}

// IsForced 报告服务端是否明确要求强制更新。
//
// 注意：nil（字段缺失）**不是**强制 —— 缺少明确指令时应保守地询问用户。
func (c CheckView) IsForced() bool {
	return c.ForceUpdate != nil && *c.ForceUpdate
}
