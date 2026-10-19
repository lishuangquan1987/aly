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
	mw *walk.MainWindow
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

	w.positionBottomRight()
	w.setTopMost()
	return w, nil
}

// positionBottomRight 取窗口所在显示器的工作区（已扣除任务栏）并贴右下角。
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
	x, y := CalcBottomRight(wa, Size{W: defaultW, H: defaultH}, margin)
	// 先定位再显示，避免窗口先闪现在屏幕左上角
	_ = w.mw.SetBounds(walk.Rectangle{X: x, Y: y, Width: defaultW, Height: defaultH})
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
