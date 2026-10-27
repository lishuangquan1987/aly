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
