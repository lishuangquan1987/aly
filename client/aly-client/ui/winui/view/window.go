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
	// maxH 是带发布内容时的窗口高度上限（逻辑像素）：避免盖住主程序太多区域。
	maxH = 300
)

// Window 是贴屏幕右下角的顶层窗口。
//
// 默认**不显示**：由调用方按场景调用 Show()（见 DEVELOPMENT_PLAN §4.1）。
type Window struct {
	mw   *walk.MainWindow
	card *Card
	// h 是当前窗口的逻辑高度：默认 defaultH，展示发布内容/按钮时由 Relayout 增加。
	h int
	// onButton 是按钮点击回调（在 UI 线程执行；由 app 层注入，转发到后台 goroutine）。
	onButton func(ButtonID)
}

// New 创建窗口：定位到所在显示器工作区右下角，设为 topmost + 工具窗口，
// 但保持隐藏。调用方需自行调用 Show()。
func New() (*Window, error) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return nil, err
	}
	w := &Window{mw: mw, h: defaultH}

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

	// 按钮点击：MouseDown 的坐标是**物理像素**（窗口客户区），
	// Card.HitTest 需要 96-DPI 逻辑坐标 → 用 IntTo96DPI 换算。
	// 点击命中后转发给 onButton（已由 SetOnButton 注入）。
	card.cw.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		d := dpiOf(mw.Handle())
		lx := walk.IntTo96DPI(x, d)
		ly := walk.IntTo96DPI(y, d)
		id := card.HitTest(lx, ly)
		if id == BtnNone {
			return
		}
		// 交互即取消自动关闭（如「无更新 5 秒后关窗」期间用户点了重试）。
		if w.onButton != nil {
			cb := w.onButton
			cb(id)
		}
	})

	w.positionBottomRight()
	w.setTopMost()
	return w, nil
}

// SetOnButton 注入按钮点击回调。回调在 **UI 线程**执行，
// 需要切回后台 goroutine 的逻辑请用 channel / Synchronize 自行调度。
func (w *Window) SetOnButton(cb func(ButtonID)) {
	w.onButton = cb
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

// SetButtons 显示卡片按钮排（ids 为空隐藏）。按钮区会改变卡片高度需求，
// 因此紧接着应调用 Relayout 调整窗口高度并重新贴角。
func (w *Window) SetButtons(ids ...ButtonID) {
	if w.card != nil {
		w.card.SetButtons(ids...)
	}
}

// SetPaused 切换下载暂停状态（影响 PauseResume 按钮文案与状态行展示，由 app 层组合）。
func (w *Window) SetPaused(v bool) {
	if w.card != nil {
		w.card.SetPaused(v)
	}
}

// Relayout 依据发布内容与按钮区重新计算窗口高度并重新贴右下角。
// 高度公式：defaultH + notesHeight + buttonsHeight，上限 maxH。
// 每次卡片高度需求变化（SetNotes / SetButtons）后调用一次。
func (w *Window) Relayout() {
	if w.card == nil {
		return
	}
	h := defaultH + w.card.NotesHeight() + w.card.ButtonsHeight()
	if h > maxH {
		h = maxH
	}
	if h == w.h {
		return
	}
	w.h = h
	w.positionBottomRight()
}

// SetNotes 设置新版本的发布内容（变更日志）。
//
// 有内容时窗口自动加高并在右下角重新定位（向上生长），无内容时回落默认高度。
// 传 nil/空 表示不展示。
func (w *Window) SetNotes(notes []string) {
	if w.card == nil {
		return
	}
	w.card.SetNotes(notes)
	w.Relayout()
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
	winSize := Size{W: sx(defaultW, d), H: sx(w.h, d)}
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

// AskYesNo 弹出一个模态确认框，**阻塞**直到用户选择，返回是否选了「是」。
//
// 注意：walk 的 WindowBase.Synchronize 是**异步**的（入队 + PostMessage，
// 不等待），直接在里面写外部变量会读到未赋值的结果。这里用 channel 等待
// 回调真正执行完，因此本方法只能在**非 UI 线程**调用（后台 goroutine）。
func (w *Window) AskYesNo(title, text string) bool {
	ch := make(chan bool, 1)
	w.mw.Synchronize(func() {
		res := walk.MsgBox(w.mw, title, text, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
		ch <- res == walk.DlgCmdYes
	})
	return <-ch
}

// ShowInfo 弹出模态提示框（只有一个「确定」），**阻塞**直到用户点确定。
// 与 AskYesNo 一样，只能在**非 UI 线程**调用（后台 goroutine）。
//
// 用途：用户主动「检查更新」且服务端无更新时，明确告知"没有可用的更新"
// （卡片 5 秒自动关闭容易被忽略，用户会以为点了没反应）。
func (w *Window) ShowInfo(title, text string) {
	ch := make(chan struct{}, 1)
	w.mw.Synchronize(func() {
		walk.MsgBox(w.mw, title, text, walk.MsgBoxOK|walk.MsgBoxIconInformation)
		ch <- struct{}{}
	})
	<-ch
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

// WaitButton 阻塞等待用户点击任一按钮，返回其 ID。
//
// 与 AskYesNo 的区别（精简计划条目 1）：不打断全屏主程序——按钮画在卡片上，
// 用户想点才点；没有系统模态框抢焦点。必须在**非 UI 线程**（后台 goroutine）
// 调用：内部用 channel 等待 UI 线程的 MouseDown 回调投递结果。
//
// timeout <= 0 表示无限等待；>0 时到期返回 BtnNone（调用方按"未选择"处理，
// 如强制更新倒计时到期自动继续）。
func (w *Window) WaitButton(timeout time.Duration) ButtonID {
	ch := make(chan ButtonID, 1)
	w.SetOnButton(func(id ButtonID) {
		select {
		case ch <- id:
		default: // 已有人等/已有结果：丢弃后续点击（幂等）
		}
	})
	defer w.SetOnButton(nil)

	if timeout <= 0 {
		return <-ch
	}
	select {
	case id := <-ch:
		return id
	case <-time.After(timeout):
		return BtnNone
	}
}

// ButtonClicks 返回一个只发一次的按钮点击通道（用于"下载中暂停"这类
// 需要边跑边听点击的场景；调用方从后台 goroutine select 它）。
//
// 返回的通道在窗口销毁前不会被关闭——调用方应配合 ctx/超时使用，
// 或在拿到结果后调用 StopButtonClicks 拆除监听。
func (w *Window) ButtonClicks() (<-chan ButtonID, func()) {
	ch := make(chan ButtonID, 4)
	stop := func() { w.SetOnButton(nil) }
	w.SetOnButton(func(id ButtonID) {
		select {
		case ch <- id:
		default: // 通道满：丢弃（不阻塞 UI 线程）
		}
	})
	return ch, stop
}
