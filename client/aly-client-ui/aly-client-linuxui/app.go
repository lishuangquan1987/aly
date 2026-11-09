package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	userVisible bool
	pending     bool
	pendingArgs []string
}

func (a *app) t(key string) string                    { return tr(a.lang, key) }
func (a *app) tf(key string, v ...interface{}) string { return trf(a.lang, key, v...) }

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
	a.update(func() {
		a.phase = "check"
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
			a.forced = p.Check.ForceUpdate != nil && *p.Check.ForceUpdate
		})
		a.handleCheck(*p.Check)
	}()
}

func decide(chk client.CheckUpdate) (action string, ask bool) {
	if !chk.HasUpdate {
		return "none", false
	}
	forced := chk.ForceUpdate != nil && *chk.ForceUpdate
	if chk.NeedDownloadUpdate {
		return "download", !forced
	}
	return "apply", !forced
}

func (a *app) handleCheck(chk client.CheckUpdate) {
	a.forced = chk.ForceUpdate != nil && *chk.ForceUpdate
	newer := chk.NewVersion
	a.update(func() {
		a.current = chk.CurrentVersion
		a.check = chk
		a.newer = newer
		a.notes = append([]string(nil), chk.ReleaseNotes...)
	})
	action, ask := decide(chk)
	switch action {
	case "none":
		a.noUpdate()
	case "download":
		if !ask {
			a.startDownload()
			return
		}
		a.update(func() {
			a.phase = "found"
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
		a.update(func() {
			a.phase = "downloaded"
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
	a.update(func() {
		a.phase = "no_update"
		a.status = a.t(keyNoUpdate)
		a.detail = a.t(keyNoUpdateBox)
		a.showProgress = false
		a.buttons = []uiButton{{id: btnClose, label: a.t(keyClose), primary: true}}
	})
	if a.mode == "check" || a.mode == "update" || a.mode == "auto" || a.mode == "silent" {
		a.autoClose = time.AfterFunc(5*time.Second, a.quit)
	}
}

func (a *app) startDownload() {
	if a.cl == nil {
		a.fail("download", "aly-client not found next to aly-client-ui")
		return
	}
	if a.downloadCancel != nil {
		a.downloadCancel()
	}
	a.downloadGen++
	gen := a.downloadGen
	ctx, cancel := context.WithCancel(context.Background())
	a.downloadCancel = cancel
	a.update(func() {
		a.phase = "download"
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
				if gen != a.downloadGen {
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
		if gen != a.downloadGen {
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
		a.update(func() {
			a.phase = "downloaded"
			a.progress = 1
			a.status = a.t(keyDownloaded)
			a.showProgress = false
			a.buttons = nil
		})
		if a.forced {
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
	a.downloadGen++
	if a.downloadCancel != nil {
		a.downloadCancel()
		a.downloadCancel = nil
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
	if a.countdownCancel != nil {
		a.countdownCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.countdownCancel = cancel
	a.update(func() {
		a.phase = "countdown"
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
	if a.phase == "apply" {
		return
	}
	if a.cl == nil {
		a.fail("apply", "aly-client not found next to aly-client-ui")
		return
	}
	if a.countdownCancel != nil {
		a.countdownCancel()
		a.countdownCancel = nil
	}
	a.phase = "apply"
	a.update(func() {
		a.phase = "apply"
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
		a.update(func() {
			a.phase = "done"
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
	a.update(func() {
		a.phase = "rollback"
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
	a.update(func() {
		a.phase = "apply"
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
		a.update(func() {
			a.phase = "done"
			a.status = a.t(keyApplyDone)
		})
		time.Sleep(1200 * time.Millisecond)
		a.quit()
	}()
}

func (a *app) fail(stage, msg string) {
	a.retryStage = stage
	if msg == "" {
		msg = a.t(keyErrorGeneric)
	}
	a.update(func() {
		a.phase = "error"
		a.status = a.t(keyErrorGeneric)
		a.detail = msg
		a.showProgress = false
		a.buttons = []uiButton{
			{id: btnRetry, label: a.t(keyRetry), primary: true},
			{id: btnClose, label: a.t(keyClose)},
		}
	})
	if a.mode == "check" {
		a.autoClose = time.AfterFunc(5*time.Second, a.quit)
	}
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
	switch a.retryStage {
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
	if a.autoClose != nil {
		a.autoClose.Stop()
		a.autoClose = nil
	}
}

func (a *app) quit() {
	a.cancelAutoClose()
	if a.downloadCancel != nil {
		a.downloadCancel()
	}
	if a.countdownCancel != nil {
		a.countdownCancel()
	}
	mygo.App.Quit()
}

func (a *app) handleSecond(args []string) {
	a.userVisible = true
	if a.win == nil {
		a.pending = true
		a.pendingArgs = append([]string(nil), args...)
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
	switch {
	case a.phase == "no_update" || a.phase == "error" || a.phase == "":
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
