// +build windows

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aly/client/aly-client/ui/winui/bridge"
	"aly/client/aly-client/ui/winui/flow"
	"aly/client/aly-client/ui/winui/i18n"
	"aly/client/aly-client/ui/winui/view"
)

// autoCloseDelay 是 -mode check 下「无更新 / 出错」提示的自动关闭时长。
// 用户一旦交互（下载开始等）应取消，见 DEVELOPMENT_PLAN §4.1。
const autoCloseDelay = 5 * time.Second

// forceCountdown 是强制更新自动重启的倒计时时长（精简计划条目 3 + 用户决策 §五.1）。
const forceCountdown = 10 * time.Second

// title 是卡片标题（不随语言变化，产品名）。
const title = "aly-client-ui"

// Options 是启动参数。
type Options struct {
	// Mode 为 auto|check|update|rollback|silent（见 NormalizeMode）。
	Mode string
	// Lang 为 zh-CN|en-US。
	Lang string
	// ServerURL / ProjectName 目前不下发：aly-client 自己读 client.json
	// （DEVELOPMENT_PLAN §3.4），保留字段供后续按需覆盖。
	ServerURL   string
	ProjectName string
}

// NormalizeMode 归一化 -mode：去空白、转小写；未知值回退 auto。
func NormalizeMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "check":
		return "check"
	case "silent":
		return "silent"
	case "update":
		return "update"
	case "rollback":
		return "rollback"
	default:
		return "auto"
	}
}

// SelfDirOf 返回可执行文件所在目录。
func SelfDirOf(exePath string) (string, error) {
	if exePath == "" {
		return "", fmt.Errorf("exe path is empty")
	}
	abs, err := filepath.Abs(exePath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(abs), nil
}

// FindAlyClient 定位 aly-client.exe。
//
// 候选顺序（DEVELOPMENT_PLAN §3.2）：
//  1. 与 UI 同目录 —— UI 部署在 UpdateFolder，aly-client.exe 也在那里；
//  2. 由 client.json 的 main_exe_relative_path 推导主程序目录再找。
func FindAlyClient(selfDir string) (string, error) {
	same := filepath.Join(selfDir, "aly-client.exe")
	if fileExists(same) {
		return same, nil
	}

	rel, err := ReadMainExeRelativePath(selfDir)
	if err == nil {
		if mf, mfErr := MainFolder(selfDir, rel); mfErr == nil {
			p := filepath.Join(mf, "aly-client.exe")
			if fileExists(p) {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("cannot find aly-client.exe (tried \"%s\" and the main folder from client.json)", same)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Run 是 UI 主入口：装配窗口、按模式决定是否立即显示、后台调用 aly-client。
// 返回值即进程退出码。
func Run(opt Options) int {
	mode := NormalizeMode(opt.Mode)
	i18n.SetLang(i18n.ParseLang(opt.Lang))

	// 注入按钮文案（view 包不依赖 i18n，由这里搭桥）。
	view.SetButtonLabels(
		i18n.T(i18n.KeyUpdateNow),
		i18n.T(i18n.KeyLater),
		i18n.T(i18n.KeyRetry),
		i18n.T(i18n.KeyClose),
		i18n.T(i18n.KeyRestartNow),
		i18n.T(i18n.KeyPause),
		i18n.T(i18n.KeyResume),
	)

	// 单实例：第二个实例直接退出，避免两个右下角窗口互相覆盖、
	// 也避免两个进程同时去替换 aly-client.exe。
	// 互斥体需持有到进程结束，故用 defer。
	first, release := AcquireSingleInstance()
	if !first {
		return 0
	}
	defer release()

	self, err := SelfDirOf(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve self dir failed: %v\n", err)
		return 2
	}

	// 启动时自更新（对齐 SDK 构造函数顺序：在首次 check 之前）。
	// 两者都是「有差异才动」，失败不阻断启动。
	if mf, mfErr := mainFolderOf(self); mfErr == nil {
		_ = UpdateUpdater(self, mf)
		// 重启时透传原参数，保证自更新后仍按同一模式运行
		//（否则手动「检查更新」重启后会变成静默的 auto 模式）
		if replaced, upErr := UpdateSelfUI(self, mf, os.Args[1:]); upErr == nil && replaced {
			// 已派生延迟替换脚本，本实例立即退出让位
			return 0
		}
	}

	w, err := view.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create window failed: %v\n", err)
		return 2
	}

	// check 模式：立即显示「正在检查」，让用户知道点了按钮有反应。
	// auto/silent：保持隐藏，只有确认有更新才显示（§4.1）。
	if mode == "check" {
		w.SetContent(title, "", i18n.T(i18n.KeyChecking))
		w.Show()
	}

	exePath, findErr := FindAlyClient(self)
	interactive := mode == "check"

	if mode == "rollback" {
		go runRollback(w, exePath, findErr)
	} else {
		go runFlow(w, interactive, exePath, findErr)
	}

	return w.Run()
}

// retryStage 表示重试应回到的阶段（精简计划条目 2 验收：重试=重跑当前阶段）。
type retryStage int

const (
	retryNone retryStage = iota
	retryCheck
	retryDownload
	retryApply
)

// flowOutcome 是一轮 check→download→apply 的结果。
type flowOutcome struct {
	// retry 非 retryNone 表示用户点了重试，外层应从该阶段重跑。
	retry retryStage
	// newer / ask / notes 缓存本轮 check 结果：阶段化重跑时跳过重复 check。
	newer string
	ask   bool
	notes []string
}

// runFlow 在后台 goroutine 里跑完整流程；所有 UI 变更都经 Synchronize。
//
// 终止策略：每条路径都必须让窗口**退出或自动关闭**，绝不留下一个
// 无标题栏、无关闭按钮的窗口卡住用户。
//
// 精简计划条目 1/2/3：更新询问全部走卡片按钮（无模态框、不抢焦点）；
// 失败态给 [重试] [关闭]，重试回到**失败的那个阶段**；强制更新给
// 10s 倒计时 + [立即重启]；下载中给 [暂停/继续]。
func runFlow(w *view.Window, interactive bool, exePath string, findErr error) {
	// ① 定位 aly-client.exe 失败
	if findErr != nil {
		failRetryable(w, interactive, "", i18n.T(i18n.KeyOffline), findErr.Error(), nil)
		return
	}

	cl := bridge.New(exePath)
	ctx := context.Background()

	// 阶段机：从 check 开始；失败重试回到失败阶段（不重跑已成功的阶段）。
	stage := retryCheck
	var carried flowOutcome // 承接上一轮的 check 结果（阶段化重跑用）

	for {
		carried = runFlowOnce(w, interactive, cl, ctx, stage, carried)
		switch carried.retry {
		case retryNone:
			return
		case retryCheck, retryDownload, retryApply:
			stage = carried.retry
			w.Synchronize(func() {
				w.SetContent(title, "", i18n.T(i18n.KeyRetrying))
				w.SetDetail("")
				w.SetError(false)
			})
		}
	}
}

// runFlowOnce 跑一轮流程（从 stage 指定的阶段开始）。
// carried 携带上一轮的 check 结果；从 retryCheck 重跑时会重新 check 并覆盖。
func runFlowOnce(w *view.Window, interactive bool, cl *bridge.Client, ctx context.Context, stage retryStage, carried flowOutcome) flowOutcome {
	needDownload := false

	// ② 检查更新（stage=retryCheck 时执行；其他阶段沿用 carried 结果）
	if stage == retryCheck {
		res := checkUpdate(cl, ctx)
		if res.problem != "" {
			// check（用户主动）：可见 + 可重试；auto（启动时自动）：静默退出。
			if !interactive {
				quit(w)
				return flowOutcome{}
			}
			failRetryable(w, true, res.current, i18n.T(i18n.KeyOffline), res.problem, func() {})
			return flowOutcome{retry: retryCheck}
		}

		// ③ 无更新
		if res.decision.Action == flow.ActionNone {
			if !interactive {
				quit(w)
				return flowOutcome{}
			}
			w.Synchronize(func() {
				w.SetNotes(nil) // 无更新：清掉可能残留的发布内容
				w.SetButtons()  // 清按钮
				w.SetContent(title, versionLabel(res.current), i18n.T(i18n.KeyNoUpdate))
				w.SetDetail("")
				w.SetError(false)
				w.Relayout()
			})
			// 用户主动检查且服务端没有更新：弹框明确告知"没有可用的更新"。
			w.ShowInfo(title, i18n.T(i18n.KeyNoUpdateBox))
			quit(w)
			return flowOutcome{}
		}

		carried = flowOutcome{
			newer: res.newer,
			ask:   res.decision.Ask,
			notes: res.notes,
		}
		needDownload = res.decision.Action == flow.ActionDownload

		// 把新版本的发布内容交给卡片：有内容时窗口自动加高并在右下角重新定位。
		// 放在任何 show() 之前，保证第一次弹出就带着"本次更新内容"。
		w.Synchronize(func() { w.SetNotes(res.notes) })
	}

	newer := carried.newer

	// 阶段化重跑（retryDownload/retryApply）：失败态把发布内容清空过，
	// 这里用 carried 缓存恢复"本次更新了什么"——否则重试后用户看到的
	// 卡片不再有变更日志（SetNotes(nil) 无害：空内容回落默认高度）。
	if stage != retryCheck {
		w.Synchronize(func() { w.SetNotes(carried.notes) })
	}

	show := func(status string) {
		w.Synchronize(func() {
			w.SetContent(title, versionLabel(newer), status)
			w.SetDetail("")
			w.SetError(false)
			w.Show()
		})
	}

	// ④ 需要下载
	if stage == retryCheck && needDownload || stage == retryDownload {
		show(i18n.Fmt(i18n.KeyFoundVersion, newer))

		if carried.ask && stage == retryCheck {
			// 精简计划条目 1：按钮卡片替代模态框。
			w.Synchronize(func() {
				w.SetButtons(view.BtnUpdate, view.BtnLater)
				w.Relayout()
			})
			choice := w.WaitButton(0)
			w.Synchronize(func() { w.SetButtons() }) // 清按钮（无论选了什么都进入下一阶段）
			if choice != view.BtnUpdate {
				quit(w) // 用户拒绝（稍后）
				return flowOutcome{}
			}
		}

		if runDownload(w, cl, ctx, newer) {
			return flowOutcome{retry: retryDownload, newer: newer, ask: carried.ask, notes: carried.notes}
		}
	}

	// ⑤ 应用
	// 重试 apply 时跳过询问/倒计时：用户刚点过「重试」，再问一次反而打断
	//（"重试=直接重跑当前阶段"——精简计划条目 2 验收语义）。
	if carried.ask && stage != retryApply {
		// 已下载待应用：同样用按钮卡片询问。
		show(i18n.Fmt(i18n.KeyFoundVersion, newer))
		w.Synchronize(func() {
			w.SetButtons(view.BtnRestartNow, view.BtnLater)
			w.Relayout()
		})
		choice := w.WaitButton(0)
		w.Synchronize(func() { w.SetButtons() })
		if choice != view.BtnRestartNow {
			quit(w) // 用户选择稍后
			return flowOutcome{}
		}
	} else if !carried.ask && stage != retryApply {
		// 强制更新：10 秒倒计时 + 可提前点（精简计划条目 3 + 用户决策 §五.1）。
		// 不再"不倒计时直接干"——那对全屏作业场景太粗暴。
		// auto 模式下窗口尚未显示，这里补上（检查/下载阶段已显示过则无害）。
		show(i18n.Fmt(i18n.KeyCountdown, int(forceCountdown/time.Second)))
		w.Synchronize(func() {
			w.SetButtons(view.BtnRestartNow)
			w.SetDetail(i18n.T(i18n.KeyWillRestart))
			w.Relayout()
		})
		// 逐秒倒计时：状态行每秒刷新（10…9…8…），点「立即重启」随时提前。
		clicks, stopClicks := w.ButtonClicks()
		left := int(forceCountdown / time.Second)
	restartLoop:
		for left > 0 {
			w.Synchronize(func() {
				w.SetContent(title, versionLabel(newer), i18n.Fmt(i18n.KeyCountdown, left))
			})
			select {
			case <-clicks: // 用户点了「立即重启」
				break restartLoop
			case <-time.After(time.Second):
				left--
			}
		}
		stopClicks()
		w.Synchronize(func() {
			w.SetButtons()
			w.SetDetail("")
		})
		// 倒计时归零或用户提前点击：都继续应用
	}

	show(i18n.T(i18n.KeyApplying))

	if ap, apErr := cl.ApplyUpdate(ctx); apErr != nil || !ap.Success {
		// bridge 契约同上：apply 失败兜底（含三段 rename 失败的恢复路径）
		// 输出 isSuccess:false 结果行且正常退出，必须查 Success。
		// 此前窗口必然已显示（倒计时/询问/重试），不能因 auto 模式静默消失。
		msg := ""
		if apErr != nil {
			msg = apErr.Error()
		} else {
			msg = failText("apply_update", ap.ErrMsg)
		}
		failRetryable(w, true, newer, i18n.T(i18n.KeyErrorGeneric), msg, func() {})
		return flowOutcome{retry: retryApply, newer: newer, ask: carried.ask, notes: carried.notes}
	}

	// 成功后 aly-client 会 launchMainExe 拉起主程序，UI 使命结束。
	w.Synchronize(func() {
		w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyApplyDone))
	})
	time.Sleep(1200 * time.Millisecond) // 让用户看清「更新完成」
	quit(w)
	return flowOutcome{}
}

// runDownload 执行下载阶段（含字节进度 + 暂停/继续）。
// 返回 true 表示用户点了重试（外层重跑）；false 表示成功或已终止。
func runDownload(w *view.Window, cl *bridge.Client, ctx context.Context, newer string) bool {
	w.Synchronize(func() {
		w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyDownloadingSimple))
		w.SetButtons(view.BtnPauseResume)
		w.Relayout()
		w.Show()
	})

	// 下载会话：点「暂停」杀子进程；点「继续」用**新的会话 ctx** 重新发起
	//（断点续传跳过已完成文件）。每代下载持有独立 ctx，防止"复用已取消的
	// ctx 导致二次暂停失效"——dlCancel 只作用于当代会话。
	clicks, stopClicks := w.ButtonClicks()
	defer stopClicks()

	done := make(chan downloadResult, 4)
	paused := false
	// gen 是下载代数：done 收到的错误若来自旧代（暂停时被杀）则忽略，
	// 只认当前代的结果——防止"暂停后被杀的错误被误判为真失败"。
	gen := 0
	cancelCur := func() {}
	// 出口统一释放当代会话（进程已结束/将重启，cancel 无副作用，仅释放资源）。
	defer func() { cancelCur() }()

	startDownload := func() {
		gen++
		dlCtx, dlCancel := context.WithCancel(ctx)
		cancelCur = dlCancel
		g := gen
		go func() {
			p, err := cl.DownloadUpdate(dlCtx, func(pi bridge.ProgressInfo) {
				w.Synchronize(func() { updateDownloadUI(w, newer, pi) })
			})
			// bridge 契约：stdout 有结果行时 err==nil，成败看 Parsed.Success
			//（如"下载完成但有 N 个文件失败"是 isSuccess:false 的结果行且
			// 进程正常退出）。不查 Success 会把业务失败当成功继续 apply。
			if err == nil && !p.Success {
				err = fmt.Errorf("%s", failText("download_update", p.ErrMsg))
			}
			done <- downloadResult{gen: g, err: err}
		}()
	}

	// 立即发起第一代下载。
	startDownload()

	for {
		select {
		case r := <-done:
			if r.gen != gen {
				continue // 旧代（被暂停杀掉）的结果：丢弃
			}
			if r.err == nil {
				// 下载成功：收起按钮与进度条。
				w.Synchronize(func() {
					w.SetButtons()
					w.SetProgress(0, 0)
					w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyDownloaded))
					w.Relayout()
				})
				return false
			}
			// 真失败：错误人话化 + 重试按钮（精简计划条目 2）。
			failRetryable(w, true, newer, i18n.T(i18n.KeyErrorGeneric), r.err.Error(), func() {})
			return true

		case id := <-clicks:
			if id != view.BtnPauseResume {
				continue
			}
			if !paused {
				// 暂停：杀掉下载子进程（.part 保留 = 断点续传）。
				// gen++ 使当代结果作废——被杀 goroutine 随后送来的
				// "canceled" 错误会在上面被代数过滤掉，不误报失败。
				paused = true
				gen++
				cancelCur()
				w.Synchronize(func() {
					w.SetPaused(true)
					w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyPaused))
				})
			} else {
				// 继续：用新会话重新发起下载。
				paused = false
				w.Synchronize(func() {
					w.SetPaused(false)
					w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyDownloadingSimple))
				})
				startDownload()
			}
		}
	}
}

// downloadResult 是一代下载的完成结果（带代数，用于过期判定）。
type downloadResult struct {
	gen int
	err error
}

// updateDownloadUI 把进度快照刷到卡片（字节级优先，旧版 aly-client 回退文件计数）。
// 在 UI 线程执行。
func updateDownloadUI(w *view.Window, newer string, pi bridge.ProgressInfo) {
	w.SetProgress(pi.Index, pi.Total)
	if pi.TotalBytes > 0 {
		// 字节级进度（精简计划条目 4）："已下载 156MB / 230MB · 2.4MB/s"
		w.SetContent(title, versionLabel(newer), i18n.Fmt(i18n.KeySpeedFmt,
			formatBytes(pi.DownloadedBytes), formatBytes(pi.TotalBytes), formatBytes(pi.SpeedBps)))
	} else {
		// 旧版 aly-client（无字节字段）回退为文件名 + 计数。
		w.SetContent(title, versionLabel(newer),
			i18n.Fmt(i18n.KeyDownloading, pi.File, pi.Index, pi.Total))
	}
}

// formatBytes 把字节数格式化为人类可读字符串（1024 进制，一位小数）。
// 零值返回 "0B"，避免 "0.0MB" 的零宽展示。
func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1fMB", float64(n)/1024/1024)
	default:
		return fmt.Sprintf("%.1fGB", float64(n)/1024/1024/1024)
	}
}

// runRollback 执行 -mode rollback：列出可回滚版本，让用户选一个并回滚。
//
// 终止策略同 runFlow：每条路径都必须退出或自动关闭。
func runRollback(w *view.Window, exePath string, findErr error) {
	if findErr != nil {
		failRetryable(w, true, "", i18n.T(i18n.KeyOffline), findErr.Error(), nil)
		return
	}

	cl := bridge.New(exePath)
	ctx := context.Background()

	// 先让窗口可见，否则获取列表期间用户看不到任何反馈
	w.Synchronize(func() {
		w.SetContent(title, "", i18n.T(i18n.KeyVersionList))
		w.SetDetail("")
		w.SetError(false)
		w.Show()
	})

	p, err := cl.ListRollbackVersions(ctx)
	if err != nil {
		failRetryable(w, true, "", i18n.T(i18n.KeyErrorGeneric), err.Error(), nil)
		return
	}
	if p.Versions == nil {
		msg := p.ErrMsg
		if msg == "" {
			msg = "list_rollback_versions returned no data"
		}
		failRetryable(w, true, "", i18n.T(i18n.KeyErrorGeneric), msg, nil)
		return
	}

	v := w.ChooseVersion(p.Versions.CurrentVersion, p.Versions.Versions)
	if v == "" {
		quit(w) // 用户取消
		return
	}

	w.Synchronize(func() {
		w.SetContent(title, versionLabel(v), i18n.T(i18n.KeyApplying))
		w.SetDetail("")
		w.SetError(false)
	})

	if rb, rbErr := cl.Rollback(ctx, v); rbErr != nil || !rb.Success {
		// bridge 契约：回滚失败也是 isSuccess:false 结果行，必须查 Success。
		msg := ""
		if rbErr != nil {
			msg = rbErr.Error()
		} else {
			msg = failText("rollback", rb.ErrMsg)
		}
		failRetryable(w, true, versionLabel(v), i18n.T(i18n.KeyErrorGeneric), msg, nil)
		return
	}

	w.Synchronize(func() {
		w.SetContent(title, versionLabel(v), i18n.T(i18n.KeyApplyDone))
	})
	time.Sleep(1200 * time.Millisecond)
	quit(w)
}

// mainFolderOf 由 UI 自身目录推导主程序目录（读 client.json）。
// 失败时调用方通常忽略——自更新不是启动的前提条件。
func mainFolderOf(selfDir string) (string, error) {
	rel, err := ReadMainExeRelativePath(selfDir)
	if err != nil {
		return "", err
	}
	return MainFolder(selfDir, rel)
}

// checkOutcome 是一次检查的可展示结果。
type checkOutcome struct {
	decision flow.Decision
	current  string
	newer    string
	// notes 是新版本的发布内容（服务端变更日志），为空表示没有可展示的内容。
	notes []string
	// problem 非空表示检查没成功（网络失败 / 服务端报错）。
	problem string
}

// checkUpdate 调用 check_update 并交给状态机决策。
func checkUpdate(cl *bridge.Client, ctx context.Context) checkOutcome {
	p, err := cl.CheckUpdate(ctx)
	if err != nil {
		// 传输层失败（超时/进程异常退出）：报「无法检查」，不谎称已是最新
		return checkOutcome{problem: err.Error()}
	}
	if p.Check == nil {
		return checkOutcome{problem: "unexpected response from aly-client check_update"}
	}

	c := p.Check
	res := checkOutcome{current: c.CurrentVersion, newer: c.NewVersion, notes: c.ReleaseNotes}

	if !p.Success {
		res.problem = p.ErrMsg
		if res.problem == "" {
			res.problem = "check_update failed"
		}
		return res
	}

	res.decision = flow.Decide(flow.CheckView{
		HasUpdate:      c.HasUpdate,
		NeedDownload:   c.NeedDownloadUpdate,
		ForceUpdate:    c.ForceUpdate,
		NewVersion:     c.NewVersion,
		CurrentVersion: c.CurrentVersion,
	})
	return res
}

// versionLabel 把版本号格式化成「v1.0.1」；空串返回空（不显示 v）。
func versionLabel(v string) string {
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// failText 是"结果行说失败但没给原因"时的兜底文案（bridge 契约要求
// 调用方检查 Parsed.Success；此时 ErrMsg 可能为空，直接展示会让用户
// 看到"（空）"的详情行）。带上命令名便于定位问题。
func failText(cmd, errMsg string) string {
	if msg := strings.TrimSpace(errMsg); msg != "" {
		return msg
	}
	return cmd + " failed (no error message)"
}

// failRetryable 展示失败态 + [重试] [关闭] 按钮（精简计划条目 2）。
//
// onRetry 为 nil 表示该场景不可重试（如定位失败在交互上无从重试）——
// 此时按旧 fail 语义处理（展示后自动关闭）。
// interactive 为 false（auto/silent）时静默退出。
func failRetryable(w *view.Window, interactive bool, version, status, rawErr string, onRetry func()) {
	if !interactive {
		quit(w)
		return
	}

	// 错误人话化（精简计划条目 2）：命中映射用人话，否则通用文案 + 原文详情。
	detail := rawErr
	if msg, ok := i18n.HumanizeError(rawErr); ok {
		status = msg
		detail = rawErr // 原文降级为"详细信息"，技术线索不丢
	}

	w.Synchronize(func() {
		w.SetNotes(nil) // 错误态不展示发布内容
		if onRetry != nil {
			w.SetButtons(view.BtnRetry, view.BtnClose)
		} else {
			w.SetButtons()
		}
		w.SetContent(title, versionLabel(version), status)
		w.SetDetail(detail)
		w.SetError(true)
		w.Relayout()
		w.Show()
	})

	if onRetry == nil {
		autoClose(w)
		return
	}

	// 等用户选择：重试 / 关闭。
	choice := w.WaitButton(0)
	w.Synchronize(func() { w.SetButtons() })
	switch choice {
	case view.BtnRetry:
		onRetry() // 由调用方触发外层重跑
	case view.BtnClose, view.BtnNone:
		quit(w)
	}
}

// fail 是 failRetryable 的不可重试别名（保留旧调用兼容）。
func fail(w *view.Window, interactive bool, version, status, detail string) {
	failRetryable(w, interactive, version, status, detail, nil)
}

// autoClose 在 autoCloseDelay 后关闭窗口。
func autoClose(w *view.Window) {
	w.Synchronize(func() {
		_ = w.AutoCloseAfter(autoCloseDelay)
	})
}

// quit 结束消息循环。
func quit(w *view.Window) {
	w.Synchronize(func() { w.Quit() })
}
