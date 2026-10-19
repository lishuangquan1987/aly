// +build windows

package view

import (
	"time"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

const (
	defaultW = 380
	defaultH = 160
	margin   = 16
)

// Window 是贴屏幕右下角的顶层窗口。
//
// 默认**不显示**：由调用方按场景调用 Show()（见 DEVELOPMENT_PLAN §4.1）。
type Window struct {
	mw   *walk.MainWindow
	card *Card
}

// New 创建窗口：定位到所在显示器工作区右下角，设为 topmost + 工具窗口，
// 但保持隐藏。调用方需自行调用 Show()。
func New() (*Window, error) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return nil, err
	}
	w := &Window{mw: mw}

	if err := mw.SetTitle("aly-client-ui"); err != nil {
		return nil, err
	}

	// 工具窗口：不出现在任务栏、不出现在 Alt-Tab
	ex := win.GetWindowLong(mw.Handle(), win.GWL_EXSTYLE)
	win.SetWindowLong(mw.Handle(), win.GWL_EXSTYLE, ex|win.WS_EX_TOOLWINDOW)

	// 无边框：去掉标题栏与缩放边框，整窗就是那张卡片。
	// 位置固定贴右下角，不需要用户拖动/缩放。
	//
	// 注意：不要 OR 上 WS_POPUP —— 它的值是 0x80000000，超出
	// SetWindowLong 的 int32 参数。清掉 CAPTION/THICKFRAME 已足够
	// 得到无标题栏、无边框的外观。
	style := win.GetWindowLong(mw.Handle(), win.GWL_STYLE)
	style &^= win.WS_CAPTION | win.WS_THICKFRAME | win.WS_MINIMIZEBOX | win.WS_MAXIMIZEBOX | win.WS_SYSMENU
	win.SetWindowLong(mw.Handle(), win.GWL_STYLE, style)
	// 改样式后必须 SWP_FRAMECHANGED 才会生效
	win.SetWindowPos(mw.Handle(), 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)

	// 整窗只放一个自绘卡片（含进度条），避免 walk 布局把卡片和进度条拆开。
	// BoxLayout 默认 margins 是 9px，会把卡片缩进、露出窗口底色，故清零。
	layout := walk.NewVBoxLayout()
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := mw.SetLayout(layout); err != nil {
		return nil, err
	}
	card, err := NewCard(mw)
	if err != nil {
		return nil, err
	}
	w.card = card

	w.positionBottomRight()
	w.setTopMost()
	return w, nil
}

// SetContent 更新卡片文案。
func (w *Window) SetContent(title, version, status string) {
	if w.card != nil {
		w.card.SetContent(title, version, status)
	}
}

// SetProgress 更新进度（cur/max）。
func (w *Window) SetProgress(cur, max int) {
	if w.card != nil {
		w.card.SetProgress(cur, max)
	}
}

// SetDetail 设置错误详情（可为空）。
func (w *Window) SetDetail(detail string) {
	if w.card != nil {
		w.card.SetDetail(detail)
	}
}

// SetError 标记当前状态是否为错误。
func (w *Window) SetError(v bool) {
	if w.card != nil {
		w.card.SetError(v)
	}
}

// positionBottomRight 取窗口所在显示器的工作区（已扣除任务栏）并贴右下角。
//
// **坐标约定（重要）**：GetMonitorInfo 返回**物理像素**，而 walk 的 SetBounds
// 接收**96-DPI 逻辑值**（内部会 RectangleFrom96DPI 再乘 dpi/96）。混用会在
// 高 DPI 下双重缩放、把窗口推到屏幕外。
// 这里统一在物理像素下计算，再用 SetBoundsPixels 直接落地，不做二次换算。
func (w *Window) positionBottomRight() {
	hmon := win.MonitorFromWindow(w.mw.Handle(), win.MONITOR_DEFAULTTONEAREST)
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(hmon, &mi) {
		return
	}
	wa := Rect{
		Left:   int(mi.RcWork.Left),
		Top:    int(mi.RcWork.Top),
		Right:  int(mi.RcWork.Right),
		Bottom: int(mi.RcWork.Bottom),
	}

	d := dpiOf(w.mw.Handle())
	winSize := Size{W: sx(defaultW, d), H: sx(defaultH, d)}
	x, y := CalcBottomRight(wa, winSize, sx(margin, d))

	// 先定位再显示，避免窗口先闪现在屏幕左上角
	_ = w.mw.SetBoundsPixels(walk.Rectangle{X: x, Y: y, Width: winSize.W, Height: winSize.H})
}

// setTopMost 置顶，且不抢焦点。
// SWP_NOACTIVATE 很关键：不要打断全屏主程序（OTDR）的键盘输入。
func (w *Window) setTopMost() {
	win.SetWindowPos(w.mw.Handle(), win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
}

// Show 重新定位并显示窗口。
func (w *Window) Show() {
	w.positionBottomRight()
	w.setTopMost()
	w.mw.SetVisible(true)
}

// Hide 隐藏窗口（不退出进程）。
func (w *Window) Hide() {
	w.mw.SetVisible(false)
}

// Run 进入消息循环，返回退出码。
func (w *Window) Run() int {
	return w.mw.Run()
}

// Quit 结束消息循环并释放窗口，使 Run 返回。
//
// 用于「无更新 → 不显示 → 直接退出」的静默路径：窗口从未显示过，
// 也必须让 Run() 返回，否则进程会带着一个隐藏窗口常驻。
func (w *Window) Quit() {
	_ = w.mw.Close()
}

// Synchronize 把 fn 投递到 UI 线程执行（供后台 goroutine 使用）。
func (w *Window) Synchronize(fn func()) {
	w.mw.Synchronize(fn)
}

// AutoCloseAfter 在 d 之后关闭窗口（并结束消息循环），返回可取消的句柄。
//
// 用 Close() 而不是 Hide()：-mode check 由主程序按需启动，提示展示完必须让
// 进程退出；只隐藏会让 Run() 的消息循环常驻，每点一次「检查更新」漏一个进程。
//
// 回调经 walk.Synchronize 回到 UI 线程，不能直接操作控件。
// 用途：-mode check 下「无更新」提示显示 5s 后自动关闭（DEVELOPMENT_PLAN §4.1）。
// 若期间用户已交互（点按/下载开始），调用方需主动 Cancel。
func (w *Window) AutoCloseAfter(d time.Duration) *AutoClose {
	return ScheduleAutoClose(d, func() {
		w.Synchronize(func() {
			_ = w.mw.Close()
		})
	})
}
