// +build windows

// Package theme 集中定义配色、字体与尺寸。
//
// 视觉约束（DEVELOPMENT_PLAN §4.2）：XP 无 DWM，没有圆角/阴影/毛玻璃；
// "现代化" 靠扁平配色 + 留白 + 统一字体 + 自绘实现。
package theme

import (
	"sync"

	"github.com/lxn/walk"
)

// 配色：中性灰阶 + 单一强调色。
var (
	// Background 卡片底色。
	Background = walk.RGB(0xFF, 0xFF, 0xFF)
	// Border 1px 描边（XP 无阴影，用描边代替投影）。
	Border = walk.RGB(0xE5, 0xE7, 0xEB)
	// TextPrimary 主文本。
	TextPrimary = walk.RGB(0x11, 0x18, 0x27)
	// TextSecondary 次要文本（版本号、状态）。
	TextSecondary = walk.RGB(0x6B, 0x72, 0x80)
	// Accent 强调色（按钮、进度填充）。
	Accent = walk.RGB(0x25, 0x63, 0xEB)
	// Track 进度条底槽。
	Track = walk.RGB(0xE5, 0xE7, 0xEB)
	// Error 错误文本。
	Error = walk.RGB(0xDC, 0x26, 0x26)
)

// 尺寸（96 DPI 基准）。
const (
	// CardW / CardH 窗口与卡片尺寸。
	CardW = 380
	CardH = 160
	// Pad 卡片内边距。
	Pad = 16
	// ProgressH 进度条高度。
	ProgressH = 6
	// TitlePt 标题字号。
	TitlePt = 11
	// BodyPt 正文字号。
	BodyPt = 9
	// SmallPt 次要文本字号（版本号）。
	SmallPt = 8
)

var (
	fontOnce  sync.Once
	bodyFont  *walk.Font
	titleFont *walk.Font
)

// BodyFont 返回正文字体：优先 Segoe UI，XP 回退 Tahoma。
//
// 字体只创建一次并复用：每次 Paint 新建 *walk.Font 会泄漏 GDI 对象。
func BodyFont() *walk.Font {
	fontOnce.Do(initFonts)
	return bodyFont
}

// TitleFont 返回标题字体（半粗）。
func TitleFont() *walk.Font {
	fontOnce.Do(initFonts)
	return titleFont
}

func initFonts() {
	if f, err := walk.NewFont("Segoe UI", TitlePt, walk.FontBold); err == nil {
		titleFont = f
	} else if f2, err2 := walk.NewFont("Tahoma", TitlePt, walk.FontBold); err2 == nil {
		titleFont = f2
	}
	// 标题字体拿不到时退化为正文字体，保证绘制不 panic
	if body, err := walk.NewFont("Segoe UI", BodyPt, 0); err == nil {
		bodyFont = body
	} else if body2, err2 := walk.NewFont("Tahoma", BodyPt, 0); err2 == nil {
		bodyFont = body2
	}
	if titleFont == nil {
		titleFont = bodyFont
	}
}
