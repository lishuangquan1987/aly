// +build windows

// Package i18n 提供中英文案。
//
// 语言由主程序经 -lang 启动参数传入；UI 不按系统区域自动切换，
// 也不在界面内提供语言切换入口（见 DEVELOPMENT_PLAN §4.5）。
package i18n

import (
	"fmt"
	"strings"
)

// Lang 是界面语言。
type Lang int

const (
	// ZH 简体中文（默认）。
	ZH Lang = iota
	// EN 英文。
	EN
)

// 文案键。新增键时必须同时加到 zh_CN.go 与 en_US.go，
// 否则 i18n_test.go 的 TestKeySetsMatch 会失败。
const (
	KeyChecking        = "checking"
	KeyNoUpdate        = "no_update"
	KeyNoUpdateBox     = "no_update_box"
	KeyFoundVersion    = "found_version"
	KeyDownloading     = "downloading"
	KeyDownloaded      = "downloaded"
	KeyApplying        = "applying"
	KeyApplyDone       = "apply_done"
	KeyCheckNow        = "check_now"
	KeyDownload        = "download"
	KeyCancel          = "cancel"
	KeyApply           = "apply"
	KeyLater           = "later"
	KeyRetry           = "retry"
	KeyClose           = "close"
	KeyCurrentVersion  = "current_version"
	KeyAskDownload     = "ask_download"
	KeyAskApply        = "ask_apply"
	KeyOffline         = "offline"
	KeyErrorGeneric    = "error_generic"
	KeyDetail          = "detail"
	KeyUpdateAvailable = "update_available"
	KeyRollback        = "rollback"
	KeyVersionList     = "version_list"

	// 精简计划条目 1/2/3 新增：按钮卡片 / 倒计时 / 暂停。
	KeyCountdown   = "countdown"    // "将在 %d 秒后自动重启…"
	KeyPause       = "pause"        // 暂停下载
	KeyResume      = "resume"       // 继续下载
	KeyPaused      = "paused"       // 已暂停（状态行）
	KeyUpdateNow   = "update_now"   // 立即更新（卡片主按钮）
	KeyRetryLater  = "retry_later"  // 稍后再说（失败态的次要按钮）
	KeySpeedFmt    = "speed_fmt"    // "已下载 %s / %s · %s/s"
	KeyRetrying    = "retrying"     // 正在重试…
	KeyWillRestart = "will_restart" // "点［立即重启］可跳过等待"
	KeyRestartNow  = "restart_now"  // 立即重启（强制更新倒计时期间的主按钮）

	// 精简计划条目 4 新增：字节级进度。
	// KeyDownloadingSimple：下载启动瞬间（尚无进度行）的占位文案，
	// 避免 KeyDownloading 的 %s/%d 占位符裸露。
	KeyDownloadingSimple = "downloading_simple" // "正在下载…"
)

var current = ZH

// ParseLang 解析 -lang 参数。
//
// 空串或未知值一律回退中文（不按系统区域推断）。
// 容忍大小写与下划线/连字符差异，如 en-US / EN_us / en 均视为英文。
func ParseLang(s string) Lang {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en-us", "en_us", "en":
		return EN
	default:
		return ZH
	}
}

// SetLang 设置当前语言。
func SetLang(l Lang) {
	current = l
}

// Current 返回当前语言。
func Current() Lang {
	return current
}

// T 取当前语言的文案；键不存在时返回键名本身（便于发现漏配）。
func T(key string) string {
	table := zhCN
	if current == EN {
		table = enUS
	}
	if s, ok := table[key]; ok {
		return s
	}
	return key
}

// Fmt 取文案并用 fmt.Sprintf 填充占位符。
func Fmt(key string, args ...interface{}) string {
	return fmt.Sprintf(T(key), args...)
}
