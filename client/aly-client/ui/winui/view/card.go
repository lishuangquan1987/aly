// +build windows

package view

import (
	"github.com/lxn/walk"

	"aly/client/aly-client/ui/winui/view/theme"
)

// Card 是自绘的扁平卡片：白底 + 1px 描边 + 标题 / 版本 / 状态 / 进度条。
//
// 用 walk.NewCustomWidget 实现（walk 官方自绘入口），而不是给 WidgetBase
// 套 InitWidget —— 后者需要自己指定窗口类名，容易出 TTM_ADDTOOL 一类的错。
// PaintMode 设为 PaintBuffered：双缓冲，避免 XP 上重绘闪烁。
//
// **坐标约定（重要）**：walk 的 canvas 全部使用 96-DPI 逻辑坐标 ——
// ClientBounds() 内部已 RectangleTo96DPI，FillRectangle/DrawRectangle/DrawText
// 内部又会 RectangleFrom96DPI 转回物理像素。因此这里**只能用主题里的
// 96-DPI 基准常量直接绘制，绝不能再自己按 DPI 缩放**，否则会双重缩放，
// 把底部元素画到可视区外（曾导致进度条完全不显示）。
type Card struct {
	cw       *walk.CustomWidget
	title    string
	version  string
	status   string
	detail   string
	cur, max int
	isError  bool
}

// NewCard 创建自绘卡片。
func NewCard(parent walk.Container) (*Card, error) {
	c := &Card{max: 1}

	cw, err := walk.NewCustomWidget(parent, 0,
		func(canvas *walk.Canvas, updateBounds walk.Rectangle) error {
			return c.paint(canvas)
		})
	if err != nil {
		return nil, err
	}
	cw.SetPaintMode(walk.PaintBuffered)
	cw.SetInvalidatesOnResize(true)
	c.cw = cw
	return c, nil
}

// Widget 返回底层控件，供布局使用（可能为 nil）。
func (c *Card) Widget() walk.Widget {
	if c == nil {
		return nil
	}
	return c.cw
}

// SetContent 更新标题 / 版本 / 状态文本。
func (c *Card) SetContent(title, version, status string) {
	if c == nil {
		return
	}
	c.title, c.version, c.status = title, version, status
	c.invalidate()
}

// SetDetail 设置错误详情（次要文本色，可为空）。
func (c *Card) SetDetail(detail string) {
	if c == nil {
		return
	}
	c.detail = detail
	c.invalidate()
}

// SetError 标记当前状态是否为错误（影响状态行颜色）。
func (c *Card) SetError(v bool) {
	if c == nil || c.isError == v {
		return
	}
	c.isError = v
	c.invalidate()
}

// SetProgress 设置进度（cur/max）；cur<=0 表示不显示进度条。
func (c *Card) SetProgress(cur, max int) {
	if c == nil {
		return
	}
	if max <= 0 {
		max = 1
	}
	c.cur, c.max = cur, max
	c.invalidate()
}

func (c *Card) invalidate() {
	if c.cw != nil {
		c.cw.Invalidate()
	}
}

// paint 绘制整张卡片。全部使用 96-DPI 逻辑坐标（见类型注释）。
func (c *Card) paint(canvas *walk.Canvas) error {
	if c.cw == nil {
		return nil
	}
	b := c.cw.ClientBounds()

	// 白底
	bg, err := walk.NewSolidColorBrush(theme.Background)
	if err != nil {
		return err
	}
	fillErr := canvas.FillRectangle(bg, b)
	bg.Dispose()
	if fillErr != nil {
		return fillErr
	}

	// 1px 直角描边（XP 无阴影，用描边代替投影；圆角本期不做）
	pen, err := walk.NewCosmeticPen(walk.PenSolid, theme.Border)
	if err != nil {
		return err
	}
	drawErr := canvas.DrawRectangle(pen, walk.Rectangle{
		X: b.X, Y: b.Y, Width: b.Width - 1, Height: b.Height - 1,
	})
	pen.Dispose()
	if drawErr != nil {
		return drawErr
	}

	pad := theme.Pad
	lineH := 22
	left := b.X + pad
	contentW := b.Width - 2*pad
	if contentW < 20 {
		contentW = 20
	}

	// 进度条（贴卡片底部）。仅在真正有进度时才画：
	// 空闲/无更新时留白，避免露出空底槽显得像坏掉的控件。
	if c.cur > 0 {
		barY := b.Y + b.Height - pad - theme.ProgressH
		if err := c.paintProgress(canvas, left, barY, contentW, theme.ProgressH); err != nil {
			return err
		}
	}

	// 标题（占左侧，给右侧版本号留出空间）
	if c.title != "" {
		titleW := contentW - 90
		if titleW < 40 {
			titleW = 40
		}
		_ = drawText(canvas, c.title, theme.TitleFont(), theme.TextPrimary,
			left, b.Y+pad, titleW, lineH,
			walk.TextLeft|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
	}

	// 版本号（右上，右对齐）
	if c.version != "" {
		_ = drawText(canvas, c.version, theme.BodyFont(), theme.TextSecondary,
			left, b.Y+pad+2, contentW, lineH,
			walk.TextRight|walk.TextVCenter|walk.TextSingleLine)
	}

	// 状态行
	statusColor := theme.TextSecondary
	if c.isError {
		statusColor = theme.Error
	}
	if c.status != "" {
		_ = drawText(canvas, c.status, theme.BodyFont(), statusColor,
			left, b.Y+pad+lineH+8, contentW, lineH,
			walk.TextLeft|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
	}

	// 详情行（次要色，比状态行再低一行）
	if c.detail != "" {
		_ = drawText(canvas, c.detail, theme.BodyFont(), theme.TextSecondary,
			left, b.Y+pad+2*lineH+10, contentW, 18,
			walk.TextLeft|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
	}
	return nil
}

// paintProgress 画底槽 + 填充（逻辑坐标）。
func (c *Card) paintProgress(canvas *walk.Canvas, x, y, w, h int) error {
	track, err := walk.NewSolidColorBrush(theme.Track)
	if err != nil {
		return err
	}
	trackErr := canvas.FillRectangle(track, walk.Rectangle{X: x, Y: y, Width: w, Height: h})
	track.Dispose()
	if trackErr != nil {
		return trackErr
	}

	fillW := ProgressFillWidth(w, c.cur, c.max)
	if fillW <= 0 {
		return nil
	}
	fill, err := walk.NewSolidColorBrush(theme.Accent)
	if err != nil {
		return err
	}
	fillErr := canvas.FillRectangle(fill, walk.Rectangle{X: x, Y: y, Width: fillW, Height: h})
	fill.Dispose()
	return fillErr
}

// drawText 是 canvas.DrawText 的薄封装，统一处理空串与 nil 字体。
//
// Canvas.DrawText(text string, font *Font, color Color, bounds Rectangle, format DrawTextFormat) error
// bounds 为 96-DPI 逻辑坐标。
func drawText(canvas *walk.Canvas, s string, f *walk.Font, color walk.Color,
	x, y, w, h int, format walk.DrawTextFormat) error {
	if s == "" || f == nil {
		return nil
	}
	return canvas.DrawText(s, f, color, walk.Rectangle{X: x, Y: y, Width: w, Height: h}, format)
}
