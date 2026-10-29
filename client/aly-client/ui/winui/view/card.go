// +build windows

package view

import (
	"github.com/lxn/walk"

	"aly/client/aly-client/ui/winui/view/theme"
)

// ButtonID 标识卡片上的一个按钮。
type ButtonID int

const (
	// BtnNone 未命中任何按钮。
	BtnNone ButtonID = iota
	// BtnUpdate 「立即更新」主按钮（发现新版本）。
	BtnUpdate
	// BtnLater 「稍后」次要按钮（发现新版本）。
	BtnLater
	// BtnRetry 「重试」主按钮（失败态）。
	BtnRetry
	// BtnClose 「关闭」次要按钮（失败态）。
	BtnClose
	// BtnRestartNow 「立即重启」（强制更新倒计时期间）。
	BtnRestartNow
	// BtnPauseResume 「暂停/继续」（下载中切换）。
	BtnPauseResume
)

// ButtonLabel 返回按钮文案（跟随 i18n；PauseResume 由调用方按暂停状态切换）。
func (b ButtonID) Label() string {
	switch b {
	case BtnUpdate:
		return buttonLabelUpdate
	case BtnLater:
		return buttonLabelLater
	case BtnRetry:
		return buttonLabelRetry
	case BtnClose:
		return buttonLabelClose
	case BtnRestartNow:
		return buttonLabelRestart
	case BtnPauseResume:
		return buttonLabelPause
	}
	return ""
}

// 按钮文案变量：由 Window.setButtonLabels 从 i18n 注入（避免 view 依赖 i18n
// 造成包环——theme 与 geometry 均为独立小包，view 保持零 i18n 依赖）。
var (
	buttonLabelUpdate  = "Update now"
	buttonLabelLater   = "Later"
	buttonLabelRetry   = "Retry"
	buttonLabelClose   = "Close"
	buttonLabelRestart = "Restart now"
	buttonLabelPause   = "Pause"
	buttonLabelResume  = "Resume"
)

// SetButtonLabels 由 app 层启动时注入 i18n 文案。
func SetButtonLabels(update, later, retry, close, restart, pause, resume string) {
	buttonLabelUpdate, buttonLabelLater = update, later
	buttonLabelRetry, buttonLabelClose = retry, close
	buttonLabelRestart, buttonLabelPause, buttonLabelResume = restart, pause, resume
}

// cardButton 是一个逻辑按钮：ID + 矩形热区（96-DPI 逻辑坐标）+ 是否主按钮。
type cardButton struct {
	id    ButtonID
	rect  walk.Rectangle
	primary bool
}

// Card 是自绘的扁平卡片：白底 + 1px 描边 + 标题 / 版本 / 状态 / 进度条 / 按钮排。
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
	notes    []string
	cur, max int
	isError  bool
	// buttons 是当前显示的按钮集合（nil = 不显示按钮区）。
	buttons []cardButton
	// buttonsHeight 缓存按钮区占用的高度（含上方留白），0 = 无按钮区。
	buttonsHeight int
	// paused 由 app 层维护（下载暂停状态切换按钮文案）。
	paused bool
}

// 发布内容（变更日志）区域的排版常量（96-DPI 逻辑像素）。
const (
	// notesLineH 单行发布内容的高度。
	notesLineH = 20
	// notesMaxLines 最多显示几行，超出部分不画（卡片高度有上限，避免遮挡主程序）。
	notesMaxLines = 5
	// notesGap 状态/详情与发布内容之间的留白，也计入卡片增高量。
	notesGap = 12
)

// 按钮区排版常量（96-DPI 逻辑像素）。
const (
	// btnH 按钮高度。
	btnH = 28
	// btnGap 按钮之间的水平间距。
	btnGap = 10
	// btnMinW 按钮最小宽度（短文案如「暂停」）。
	btnMinW = 72
	// btnPrimaryW 主按钮宽度（文案更长）。
	btnPrimaryW = 104
	// btnAreaGap 按钮区与上方内容（进度条/详情）之间的留白。
	btnAreaGap = 14
)

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

// SetNotes 设置新版本的发布内容（变更日志，每项一行）。
// 传 nil/空 切片表示不展示，卡片回落默认高度。
func (c *Card) SetNotes(notes []string) {
	if c == nil {
		return
	}
	c.notes = notes
	c.invalidate()
}

// NotesLines 返回实际会绘制的发布内容行数（最多 notesMaxLines 行）。
func (c *Card) NotesLines() int {
	if c == nil {
		return 0
	}
	n := len(c.notes)
	if n > notesMaxLines {
		n = notesMaxLines
	}
	return n
}

// NotesHeight 返回发布内容区域需要额外增加的高度（逻辑像素）；0 表示不需要加高。
func (c *Card) NotesHeight() int {
	n := c.NotesLines()
	if n == 0 {
		return 0
	}
	return notesGap + n*notesLineH
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

// SetButtons 设置要显示的按钮集合（nil/空 = 隐藏按钮区）。
// ids 为空时按钮区高度归零。返回是否发生高度变化（调用方据此重定位窗口）。
func (c *Card) SetButtons(ids ...ButtonID) {
	if c == nil {
		return
	}
	if len(ids) == 0 {
		c.buttons = nil
		if c.buttonsHeight != 0 {
			c.buttonsHeight = 0
			c.invalidate()
		}
		return
	}
	// 稳定顺序：主按钮（靠左）在前，次要按钮在后，与 ids 传入顺序无关。
	// 注意不做值排序——按值排会把 BtnLater(2) 排到 BtnRestartNow(5) 左边，
	// 造成"下载询问主按钮在左、应用询问主按钮在右"的视觉不一致。
	primary := make([]ButtonID, 0, len(ids))
	secondary := make([]ButtonID, 0, len(ids))
	for _, id := range ids {
		switch id {
		case BtnUpdate, BtnRetry, BtnRestartNow:
			primary = append(primary, id)
		default:
			secondary = append(secondary, id)
		}
	}
	order := append(primary, secondary...)

	c.buttons = make([]cardButton, 0, len(order))
	for _, id := range order {
		c.buttons = append(c.buttons, cardButton{id: id})
	}
	// 高度不变（按钮行固定一排），只重绘
	c.buttonsHeight = btnAreaGap + btnH
	c.layoutButtons()
	c.invalidate()
}

// SetPaused 切换暂停状态（影响 PauseResume 按钮文案）。
func (c *Card) SetPaused(v bool) {
	if c == nil || c.paused == v {
		return
	}
	c.paused = v
	c.invalidate()
}

// Paused 返回当前暂停状态。
func (c *Card) Paused() bool {
	return c != nil && c.paused
}

// ButtonsHeight 返回按钮区占用的额外高度（含上方留白）；0 = 无按钮。
func (c *Card) ButtonsHeight() int {
	if c == nil {
		return 0
	}
	return c.buttonsHeight
}

// HitTest 给定卡片内逻辑坐标，返回命中按钮的 ID（未命中返回 BtnNone）。
func (c *Card) HitTest(x, y int) ButtonID {
	if c == nil {
		return BtnNone
	}
	for _, b := range c.buttons {
		if x >= b.rect.X && x < b.rect.X+b.rect.Width &&
			y >= b.rect.Y && y < b.rect.Y+b.rect.Height {
			return b.id
		}
	}
	return BtnNone
}

// HasButtons 报告当前是否有按钮显示。
func (c *Card) HasButtons() bool {
	return c != nil && len(c.buttons) > 0
}

func (c *Card) invalidate() {
	if c.cw != nil {
		c.cw.Invalidate()
	}
}

// layoutButtons 计算每个按钮的热区矩形（96-DPI 逻辑坐标）。
// 一行排布：左起为主按钮，随后次要按钮，整体靠左；宽度不足时压缩次要按钮。
func (c *Card) layoutButtons() {
	if c.cw == nil || len(c.buttons) == 0 {
		return
	}
	b := c.cw.ClientBounds()
	if b.Width <= 2*theme.Pad {
		return
	}
	x := b.X + theme.Pad
	y := b.Y + b.Height - theme.Pad - btnH + 1
	contentW := b.Width - 2*theme.Pad
	// 先按期望宽度排，超出则从最后一个次要按钮开始压缩到 btnMinW。
	widths := make([]int, len(c.buttons))
	total := 0
	for i, btn := range c.buttons {
		w := btnMinW
		if btn.primaryWidth() {
			w = btnPrimaryW
		}
		widths[i] = w
		total += w
	}
	total += btnGap * (len(c.buttons) - 1)
	if total > contentW {
		// 等比压缩，下限 btnMinW。
		scale := float64(contentW) / float64(total)
		total = 0
		for i, w := range widths {
			nw := int(float64(w) * scale)
			if nw < btnMinW {
				nw = btnMinW
			}
			widths[i] = nw
			total += nw
		}
		total += btnGap * (len(c.buttons) - 1)
		// 仍超（极窄卡片）：丢掉间距压缩，直接截断最后一个按钮宽度。
		if total > contentW {
			over := total - contentW
			widths[len(widths)-1] -= over
			if widths[len(widths)-1] < 40 {
				widths[len(widths)-1] = 40
			}
			total = contentW
		}
	}
	_ = total
	for i := range c.buttons {
		c.buttons[i].rect = walk.Rectangle{X: x, Y: y, Width: widths[i], Height: btnH}
		x += widths[i] + btnGap
	}
}

// primaryWidth 报告按钮是否用主按钮宽度。
func (b cardButton) primaryWidth() bool {
	switch b.id {
	case BtnUpdate, BtnRetry, BtnRestartNow:
		return true
	}
	return false
}

// paint 绘制整张卡片。全部使用 96-DPI 逻辑坐标（见类型注释）。
func (c *Card) paint(canvas *walk.Canvas) error {
	if c.cw == nil {
		return nil
	}
	b := c.cw.ClientBounds()

	// 布局在绘制时同步一次（窗口尺寸可能在 SetButtons 之后变化）。
	c.layoutButtons()

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

	// 进度条（贴卡片底部；有按钮区时让位按钮，位于其上方）。
	if c.cur > 0 {
		barY := b.Y + b.Height - pad - theme.ProgressH
		if c.buttonsHeight > 0 {
			barY -= c.buttonsHeight
		}
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

	// 发布内容（新版本变更日志）：每项一行，单行超长省略。
	// 行数超过 notesMaxLines 时只画前几行（窗口高度由 Window.SetNotes 按 NotesLines 调整）。
	if n := c.NotesLines(); n > 0 {
		top := b.Y + pad + 3*lineH + 8
		for i := 0; i < n; i++ {
			_ = drawText(canvas, "· "+c.notes[i], theme.BodyFont(), theme.TextSecondary,
				left, top+i*notesLineH, contentW, notesLineH,
				walk.TextLeft|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		}
	}

	// 按钮排（底部）
	if err := c.paintButtons(canvas); err != nil {
		return err
	}
	return nil
}

// paintButtons 绘制按钮排：主按钮 = 强调色底白字；次要按钮 = 白底灰边灰字。
func (c *Card) paintButtons(canvas *walk.Canvas) error {
	for _, b := range c.buttons {
		var fillErr, textErr, borderErr error
		if b.primaryWidth() {
			fill, err := walk.NewSolidColorBrush(theme.Accent)
			if err != nil {
				return err
			}
			fillErr = canvas.FillRectangle(fill, b.rect)
			fill.Dispose()
			if fillErr != nil {
				return fillErr
			}
			textErr = drawText(canvas, c.buttonLabel(b.id), theme.BodyFont(), theme.Background,
				b.rect.X, b.rect.Y, b.rect.Width, b.rect.Height,
				walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		} else {
			pen, err := walk.NewCosmeticPen(walk.PenSolid, theme.Border)
			if err != nil {
				return err
			}
			borderErr = canvas.DrawRectangle(pen, walk.Rectangle{
				X: b.rect.X, Y: b.rect.Y, Width: b.rect.Width - 1, Height: b.rect.Height - 1,
			})
			pen.Dispose()
			if borderErr != nil {
				return borderErr
			}
			textErr = drawText(canvas, c.buttonLabel(b.id), theme.BodyFont(), theme.TextPrimary,
				b.rect.X, b.rect.Y, b.rect.Width, b.rect.Height,
				walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		}
		_ = textErr // 绘制文本失败不致命：其余元素继续画
	}
	return nil
}

// buttonLabel 返回按钮当前文案（PauseResume 按暂停状态切换）。
func (c *Card) buttonLabel(id ButtonID) string {
	if id == BtnPauseResume && c.paused {
		return buttonLabelResume
	}
	return id.Label()
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
