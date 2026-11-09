// +build windows

package view

import (
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// defaultDPI 是 96 DPI（100% 缩放）基准。
const defaultDPI = 96

// dpiOf 返回窗口的系统 DPI；取不到时回退 96。
//
// 本项目不做 Per-Monitor V2（XP 不支持），只需系统 DPI。
// win.GetDpiForWindow 内部走 GetDC(0) + GetDeviceCaps(LOGPIXELSY)。
func dpiOf(hwnd win.HWND) int {
	if d := win.GetDpiForWindow(hwnd); d > 0 {
		return int(d)
	}
	return defaultDPI
}

// sx 把 96-DPI 基准值按当前 DPI 缩放。
//
// 必需：walk 的字体尺寸是 point，GDI 会随 DPI 自动放大；
// 若窗口尺寸/内边距仍用固定像素，高 DPI 下文字会挤出卡片。
func sx(v, dpi int) int {
	n := walk.IntFrom96DPI(v, dpi)
	if n < 1 {
		return 1
	}
	return n
}
