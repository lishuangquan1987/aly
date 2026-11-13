package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lishuangquan1987/aly/client/aly-client-ui/aly-client-linuxui/client"
)

type buttonID int

const (
	btnNone buttonID = iota
	btnUpdate
	btnLater
	btnPause
	btnResume
	btnRetry
	btnClose
	btnRestart
	btnVersion
)

type uiButton struct {
	id      buttonID
	label   string
	version string
	primary bool
}

type app struct {
	// 审查 M23：生命周期字段的互斥锁。
	//
	// UI 可见字段（status/detail/buttons/progress/...）仍通过 a.update 编组到 UI 线程；
	// 但 downloadGen/downloadCancel/countdownCancel/autoClose/forced/phase/retryStage/
	// pending 会被多个 goroutine 读写（下载 goroutine、倒计时 goroutine、第二个实例的
	// goroutine、点击回调），**必须**用本锁串行化。注意 a.update 在 win==nil 时直接在
	// 调用 goroutine 执行（见 update 实现），不能代替加锁。
	//
	// 规则：持有本锁时不得调用 a.update / mygo 窗口 API（避免与 UI 线程编组互相等待）。
	mu sync.Mutex

	l       layout
	mode    string
	lang    string
	project string
	cl      *client.Client

	win      *mygo.Window
	precheck *client.CheckUpdate
	check    client.CheckUpdate
	current  string
	newer    string
	notes    []string

	status       string
	detail       string
	buttons      []uiButton
	progress     float64
	showProgress bool
	paused       bool
	forced       bool
	phase        string
	retryStage   string
	err          bool

	downloadCancel  context.CancelFunc
	downloadGen     int
	countdownCancel context.CancelFunc
	autoClose       *time.Timer

	// pending/pendingArgs 由第二个实例的 goroutine 写、主 goroutine 读（审查 M23），
	// 访问一律在 a.mu 下进行。
	pending     bool
	pendingArgs []string
}

func (a *app) t(key string) string                    { return tr(a.lang, key) }
func (a *app) tf(key string, v ...interface{}) string { return trf(a.lang, key, v...) }

// ── 生命周期字段的线程安全访问器（审查 M23）────────────────────────────
//
// 这些字段跨 goroutine 读写；集中到此处的访问器里加锁，避免调用点各自漏锁。
// 注意：持有 a.mu 期间一律不做 UI 操作（不调 a.update / a.win.*）。

// nextDownloadGen 开启新的一代下载并返回代数；旧一代的进度/结果据此丢弃。
func (a *app) nextDownloadGen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.downloadGen++
	return a.downloadGen
}

// downloadGenIs 判断代数是否仍是最新（用于下载 goroutine 过滤陈旧进度/结果）。
func (a *app) downloadGenIs(gen int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return gen == a.downloadGen
}

// takeDownloadCancel 取走并返回当前的下载取消函数（同时置空）。
func (a *app) takeDownloadCancel() context.CancelFunc {
	a.mu.Lock()
	defer a.mu.Unlock()
	cancel := a.downloadCancel
	a.downloadCancel = nil
	return cancel
}

// setDownloadCancel 设置当前下载取消函数。
func (a *app) setDownloadCancel(cancel context.CancelFunc) {
	a.mu.Lock()
	a.downloadCancel = cancel
	a.mu.Unlock()
}

// takeCountdownCancel 取走并返回倒计时取消函数。
func (a *app) takeCountdownCancel() context.CancelFunc {
	a.mu.Lock()
	defer a.mu.Unlock()
	cancel := a.countdownCancel
	a.countdownCancel = nil
	return cancel
}

func (a *app) setCountdownCancel(cancel context.CancelFunc) {
	a.mu.Lock()
	a.countdownCancel = cancel
	a.mu.Unlock()
}

// setAutoClose 设置自动关闭定时器（替换旧的）。
func (a *app) setAutoClose(d time.Duration, fn func()) {
	timer := time.AfterFunc(d, fn)
	a.mu.Lock()
	old := a.autoClose
	a.autoClose = timer
	a.mu.Unlock()
	if old != nil {
		old.Stop()
	}
}

// takeAutoClose 取走自动关闭定时器（并停止）。
func (a *app) takeAutoClose() *time.Timer {
	a.mu.Lock()
	defer a.mu.Unlock()
	timer := a.autoClose
	a.autoClose = nil
	return timer
}

// setForced / isForced 保护 force_update 现场。
func (a *app) setForced(v bool) {
	a.mu.Lock()
	a.forced = v
	a.mu.Unlock()
}

func (a *app) isForced() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.forced
}

// setPhase 记录当前阶段。
func (a *app) setPhase(p string) {
	a.mu.Lock()
	a.phase = p
	a.mu.Unlock()
}

// currentPhase 读取当前阶段（供跨 goroutine 判断）。
func (a *app) currentPhase() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.phase
}

// enterPhase 阶段双跑守卫：已是目标阶段则返回 false（防止重复发起 apply）。
func (a *app) enterPhase(p string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.phase == p {
		return false
	}
	a.phase = p
	return true
}

func (a *app) setRetryStage(stage string) {
	a.mu.Lock()
	a.retryStage = stage
	a.mu.Unlock()
}

func (a *app) currentRetryStage() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.retryStage
}

// setPending 记录"第二个实例请求显示 UI"。
func (a *app) setPending(args []string) {
	a.mu.Lock()
	a.pending = true
	a.pendingArgs = append([]string(nil), args...)
	a.mu.Unlock()
}

// takePending 读取并清空 pending 现场。
func (a *app) takePending() (bool, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	pending := a.pending
	args := append([]string(nil), a.pendingArgs...)
	a.pending = false
	a.pendingArgs = nil
	return pending, args
}

func (a *app) update(fn func()) {
	if a.win != nil {
		if a.win.IsDestroyed() {
			return
		}
		a.win.Update(fn)
		return
	}
	fn()
}

func (a *app) runUI() int {
	mygo.App.WhenReady(func() {
		a.createWindow()
		a.startMode()
	})
	if err := mygo.App.Run(); err != nil {
		log.Printf("mygo run: %v", err)
		return 2
	}
	return 0
}

func (a *app) createWindow() {
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:           a.project,
		Width:           420,
		Height:          250,
		MinWidth:        380,
		MinHeight:       210,
		Frameless:       true,
		DisableResize:   true,
		AlwaysOnTop:     true,
		SkipTaskbar:     true,
		BackgroundColor: "#ffffff",
		Content:         ui.View(a.view),
	})
	a.win = w
	d := mygo.Screen.PrimaryDisplay()
	wa := d.WorkArea
	w.SetPosition(wa.X+wa.Width-420-12, wa.Y+wa.Height-250-12)
	w.Show()
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(18).Gap(8).Children(func() {
		ui.Text(c, a.project).FontSize(18).Bold()
		if a.current != "" {
			ui.Text(c, a.tf(keyCurrentVersion, a.current)).
				FontSize(11).TextColor(t.TextMuted)
		}
		if a.status != "" {
			ui.Text(c, a.status).FontSize(14)
		}
		if a.showProgress {
			ui.Progress(c, a.progress).Width(384).Height(8)
		}
		if a.detail != "" {
			ui.Text(c, a.detail).FontSize(11).TextColor(t.TextMuted)
		}
		for i, n := range a.notes {
			if i >= 3 {
				break
			}
			ui.Text(c, "· "+n).FontSize(10).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if len(a.buttons) > 0 {
			if len(a.buttons) > 4 {
				ui.Column(c).Gap(6).Children(func() {
					a.buildButtons(c)
				})
			} else {
				ui.Row(c).Gap(8).Children(func() {
					a.buildButtons(c)
				})
			}
		}
	})
}

func (a *app) buildButtons(c *ui.Context) {
	for _, b := range a.buttons {
		var e ui.Element
		if b.primary {
			e = ui.PrimaryButton(c, b.label)
		} else {
			e = ui.Button(c, b.label)
		}
		if e.Clicked() {
			a.click(b)
		}
	}
}

func (a *app) startCheck() {
	if a.cl == nil {
		a.fail("check", "aly-client not found next to aly-client-ui")
		return
	}
	a.cancelAutoClose()
	a.setPhase("check")
	a.update(func() {
		a.status = a.t(keyChecking)
		a.detail = ""
		a.buttons = nil
		a.notes = nil
		a.showProgress = false
		a.err = false
	})
	go func() {
		p, err := a.cl.CheckUpdate(context.Background())
		if err != nil {
			a.fail("check", err.Error())
			return
		}
		if !p.Success || p.Check == nil {
			a.fail("check", p.ErrMsg)
			return
		}
		a.update(func() {
			a.current = p.Check.CurrentVersion
			a.check = *p.Check
			a.newer = p.Check.NewVersion
			a.notes = append([]string(nil), p.Check.ReleaseNotes...)
			a.setForced(p.Check.ForceUpdate != nil && *p.Check.ForceUpdate)
		})
		a.handleCheck(*p.Check)
	}()
}

// decide 依据 check_update 结果与现场决定下一步（与 winui flow.Decide 同款语义）。
//
//  1. applying（崩溃恢复 / 回滚续跑）→ 无条件 apply、**不询问**：
//     主目录可能已被改名甚至丢失，应用处于不可用状态，必须自愈；
//     询问会把机器留在坏状态里（审查 B10，linuxui 原缺此分支）。
//  2. 无更新 → none。
//  3. 需要下载 → download；force_update 明确为 true 才自动。
//  4. 已下载只差应用 → apply；同样只有明确 true 才自动。
func decide(chk client.CheckUpdate, applying bool) (action string, ask bool) {
	if !chk.HasUpdate {
		return "none", false
	}
	if applying {
		return "apply", false
	}
	forced := chk.ForceUpdate != nil && *chk.ForceUpdate
	if chk.NeedDownloadUpdate {
		return "download", !forced
	}
	return "apply", !forced
}

func (a *app) handleCheck(chk client.CheckUpdate) {
	a.setForced(chk.ForceUpdate != nil && *chk.ForceUpdate)
	newer := chk.NewVersion
	a.update(func() {
		a.current = chk.CurrentVersion
		a.check = chk
		a.newer = newer
		a.notes = append([]string(nil), chk.ReleaseNotes...)
	})
	action, ask := decide(chk, applyingNow())
	switch action {
	case "none":
		a.noUpdate()
	case "download":
		if !ask {
			a.startDownload()
			return
		}
		a.setPhase("found")
		a.update(func() {
			a.status = a.tf(keyFoundVersion, newer)
			a.showProgress = false
			a.detail = ""
			a.buttons = []uiButton{
				{id: btnUpdate, label: a.t(keyUpdateNow), primary: true},
				{id: btnLater, label: a.t(keyLater)},
			}
		})
	default:
		if !ask {
			a.startCountdown()
			return
		}
		a.setPhase("downloaded")
		a.update(func() {
			a.status = a.t(keyDownloaded)
			a.showProgress = false
			a.detail = a.t(keyWillRestart)
			a.buttons = []uiButton{
				{id: btnRestart, label: a.t(keyRestartNow), primary: true},
				{id: btnLater, label: a.t(keyLater)},
			}
		})
	}
}

func (a *app) noUpdate() {
	a.setPhase("no_update")
	a.update(func() {
		a.status = a.t(keyNoUpdate)
		a.detail = a.t(keyNoUpdateBox)
		a.showProgress = false
		a.buttons = []uiButton{{id: btnClose, label: a.t(keyClose), primary: true}}
	})
	if a.mode == "check" || a.mode == "update" || a.mode == "auto" || a.mode == "silent" {
		a.setAutoClose(5*time.Second, a.quit)
	}
}

func (a *app) startDownload() {
	if a.cl == nil {
		a.fail("download", "aly-client not found next to aly-client-ui")
		return
	}
	if cancel := a.takeDownloadCancel(); cancel != nil {
		cancel()
	}
	gen := a.nextDownloadGen()
	ctx, cancel := context.WithCancel(context.Background())
	a.setDownloadCancel(cancel)
	a.setPhase("download")
	a.update(func() {
		a.paused = false
		a.err = false
		a.detail = ""
		a.status = a.t(keyDownloadingSimple)
		a.progress = 0
		a.showProgress = true
		a.buttons = []uiButton{{id: btnPause, label: a.t(keyPause)}}
	})
	go func() {
		p, err := a.cl.DownloadUpdate(ctx, func(pr client.Progress) {
			a.update(func() {
				if !a.downloadGenIs(gen) {
					return
				}
				if pr.TotalBytes > 0 {
					a.progress = float64(pr.DownloadedBytes) / float64(pr.TotalBytes)
					if a.progress < 0 {
						a.progress = 0
					}
					if a.progress > 1 {
						a.progress = 1
					}
					a.status = a.tf(keySpeedFmt,
						formatBytes(pr.DownloadedBytes),
						formatBytes(pr.TotalBytes),
						formatBytes(pr.SpeedBps))
				} else if pr.Total > 0 {
					a.progress = float64(pr.Index) / float64(pr.Total)
					if a.progress > 1 {
						a.progress = 1
					}
					a.status = a.tf(keyDownloading, pr.File, pr.Index, pr.Total)
				}
			})
		})
		if ctx.Err() != nil {
			return // 暂停/取消
		}
		if !a.downloadGenIs(gen) {
			return // 已被更新的一代取代
		}
		if err != nil {
			a.fail("download", err.Error())
			return
		}
		if !p.Success {
			a.fail("download", p.ErrMsg)
			return
		}
		a.setPhase("downloaded")
		a.update(func() {
			a.progress = 1
			a.status = a.t(keyDownloaded)
			a.showProgress = false
			a.buttons = nil
		})
		if a.isForced() {
			a.startCountdown()
			return
		}
		a.update(func() {
			a.detail = a.t(keyWillRestart)
			a.buttons = []uiButton{
				{id: btnRestart, label: a.t(keyRestartNow), primary: true},
				{id: btnLater, label: a.t(keyLater)},
			}
		})
	}()
}

func (a *app) pauseDownload() {
	a.nextDownloadGen()
	if cancel := a.takeDownloadCancel(); cancel != nil {
		cancel()
	}
	a.update(func() {
		a.paused = true
		a.status = a.t(keyPaused)
		a.buttons = []uiButton{
			{id: btnResume, label: a.t(keyResume), primary: true},
			{id: btnClose, label: a.t(keyClose)},
		}
	})
}

func (a *app) startCountdown() {
	if cancel := a.takeCountdownCancel(); cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.setCountdownCancel(cancel)
	a.setPhase("countdown")
	a.update(func() {
		a.status = a.tf(keyCountdown, 10)
		a.detail = a.t(keyWillRestart)
		a.showProgress = false
		a.buttons = []uiButton{{id: btnRestart, label: a.t(keyRestartNow), primary: true}}
	})
	go func() {
		for left := 10; left > 0; left-- {
			select {
			case <-ctx.Done():
				return
			default:
			}
			a.update(func() { a.status = a.tf(keyCountdown, left) })
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		a.startApply()
	}()
}

func (a *app) startApply() {
	// 审查 M23：阶段双跑守卫必须在锁内 check-and-set，
	// 否则倒计时 goroutine 与用户点击并发时可能双拉 ApplyUpdate。
	if !a.enterPhase("apply") {
		return
	}
	if a.cl == nil {
		a.fail("apply", "aly-client not found next to aly-client-ui")
		return
	}
	if cancel := a.takeCountdownCancel(); cancel != nil {
		cancel()
	}
	a.update(func() {
		a.status = a.t(keyApplying)
		a.detail = ""
		a.buttons = nil
		a.showProgress = false
	})
	go func() {
		p, err := a.cl.ApplyUpdate(context.Background())
		if err != nil {
			a.fail("apply", err.Error())
			return
		}
		if !p.Success {
			a.fail("apply", p.ErrMsg)
			return
		}
		a.setPhase("done")
		a.update(func() {
			a.status = a.t(keyApplyDone)
			a.detail = ""
		})
		time.Sleep(1200 * time.Millisecond)
		a.quit()
	}()
}

func (a *app) startRollback() {
	if a.cl == nil {
		a.fail("rollback", "aly-client not found next to aly-client-ui")
		return
	}
	a.setPhase("rollback")
	a.update(func() {
		a.status = a.t(keyVersionList)
		a.detail = ""
		a.showProgress = false
		a.buttons = nil
	})
	go func() {
		p, err := a.cl.ListRollbackVersions(context.Background())
		if err != nil {
			a.fail("rollback", err.Error())
			return
		}
		if !p.Success || p.Versions == nil {
			a.fail("rollback", p.ErrMsg)
			return
		}
		a.update(func() {
			a.current = p.Versions.CurrentVersion
			a.buttons = nil
			for _, v := range p.Versions.Versions {
				a.buttons = append(a.buttons, uiButton{
					id: btnVersion, version: v, label: a.t(keyRollback) + " " + v,
				})
			}
			a.buttons = append(a.buttons, uiButton{id: btnClose, label: a.t(keyClose)})
		})
	}()
}

func (a *app) runRollback(version string) {
	a.setPhase("apply")
	a.update(func() {
		a.status = a.t(keyApplying)
		a.buttons = nil
	})
	go func() {
		p, err := a.cl.Rollback(context.Background(), version)
		if err != nil {
			a.fail("rollback", err.Error())
			return
		}
		if !p.Success {
			a.fail("rollback", p.ErrMsg)
			return
		}
		a.setPhase("done")
		a.update(func() {
			a.status = a.t(keyApplyDone)
		})
		time.Sleep(1200 * time.Millisecond)
		a.quit()
	}()
}

func (a *app) fail(stage, msg string) {
	a.setRetryStage(stage)
	if msg == "" {
		msg = a.t(keyErrorGeneric)
	}
	a.setPhase("error")
	a.update(func() {
		a.status = a.t(keyErrorGeneric)
		a.detail = msg
		a.showProgress = false
		a.buttons = []uiButton{
			{id: btnRetry, label: a.t(keyRetry), primary: true},
			{id: btnClose, label: a.t(keyClose)},
		}
	})
	// 审查 M26：失败卡片不再 5 秒自动关闭——用户只有 5 秒读错误详情，
	// "重试"按钮形同虚设，且与 winui（带按钮的失败卡片常驻）行为相悖。
	// 无按钮的"无更新"提示仍保留 5 秒自动关（见上方 noUpdate）。
}

func (a *app) click(b uiButton) {
	a.cancelAutoClose()
	switch b.id {
	case btnUpdate, btnResume:
		a.startDownload()
	case btnLater, btnClose:
		a.quit()
	case btnPause:
		a.pauseDownload()
	case btnRetry:
		a.retry()
	case btnRestart:
		a.startApply()
	case btnVersion:
		a.runRollback(b.version)
	}
}

func (a *app) retry() {
	a.update(func() {
		a.status = a.t(keyRetrying)
		a.detail = ""
		a.buttons = nil
	})
	switch a.currentRetryStage() {
	case "download":
		a.startDownload()
	case "apply":
		a.startApply()
	case "rollback":
		a.startRollback()
	default:
		a.startCheck()
	}
}

func (a *app) cancelAutoClose() {
	if timer := a.takeAutoClose(); timer != nil {
		timer.Stop()
	}
}

func (a *app) quit() {
	a.cancelAutoClose()
	if cancel := a.takeDownloadCancel(); cancel != nil {
		cancel()
	}
	if cancel := a.takeCountdownCancel(); cancel != nil {
		cancel()
	}
	mygo.App.Quit()
}

func (a *app) handleSecond(args []string) {
	if a.win == nil {
		a.setPending(args)
		return
	}
	if !a.win.IsDestroyed() {
		a.update(func() {
			a.win.Show()
			a.win.Focus()
		})
	}
	mode := modeFromArgs(args)
	if mode != "" && mode != a.mode {
		a.mode = mode
	}
	switch phase := a.currentPhase(); {
	case phase == "no_update" || phase == "error" || phase == "":
		if a.mode == "rollback" {
			a.startRollback()
		} else {
			a.startCheck()
		}
	case a.mode == "rollback":
		a.startRollback()
	}
}

// modeFromArgs 取第二个实例转发的 -mode；无则空。
func modeFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-mode" && i+1 < len(args) {
			return normalizeMode(args[i+1])
		}
		if len(args[i]) > 6 && args[i][:6] == "-mode=" {
			return normalizeMode(args[i][6:])
		}
	}
	return ""
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKiB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1fMiB", float64(n)/1024/1024)
	default:
		return fmt.Sprintf("%.1fGiB", float64(n)/1024/1024/1024)
	}
}

func appendUpdateLog(l layout, format string, a ...interface{}) {
	if l.update == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(l.update, "update.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, a...))
}
