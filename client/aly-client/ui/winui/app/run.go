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

	self, err := SelfDirOf(os.Args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve self dir failed: %v\n", err)
		return 2
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

	go runFlow(w, interactive, exePath, findErr)

	return w.Run()
}

// runFlow 在后台 goroutine 里跑完整流程；所有 UI 变更都经 Synchronize。
//
// 终止策略：每条路径都必须让窗口**退出或自动关闭**，绝不留下一个
// 无标题栏、无关闭按钮的窗口卡住用户。
func runFlow(w *view.Window, interactive bool, exePath string, findErr error) {
	// ① 定位 aly-client.exe 失败
	if findErr != nil {
		fail(w, interactive, "", i18n.T(i18n.KeyOffline), findErr.Error())
		return
	}

	cl := bridge.New(exePath)
	ctx := context.Background()

	// ② 检查更新
	res := checkUpdate(cl, ctx)
	if res.problem != "" {
		// check（用户主动）：必须可见，否则「点了没反应」。
		// auto（启动时自动跑）：静默退出——网络不通是常态，弹框会变骚扰。
		if !interactive {
			quit(w)
			return
		}
		fail(w, true, res.current, i18n.T(i18n.KeyOffline), res.problem)
		return
	}

	// ③ 无更新
	if res.decision.Action == flow.ActionNone {
		if !interactive {
			quit(w)
			return
		}
		w.Synchronize(func() {
			w.SetContent(title, versionLabel(res.current), i18n.T(i18n.KeyNoUpdate))
			w.SetDetail("")
			w.SetError(false)
		})
		autoClose(w)
		return
	}

	newer := res.newer
	show := func(status string) {
		w.Synchronize(func() {
			w.SetContent(title, versionLabel(newer), status)
			w.SetDetail("")
			w.SetError(false)
			w.Show()
		})
	}

	// ④ 需要下载
	if res.decision.Action == flow.ActionDownload {
		if res.decision.Ask && !w.AskYesNo(title, i18n.Fmt(i18n.KeyAskDownload, newer)) {
			quit(w) // 用户拒绝
			return
		}

		show(i18n.T(i18n.KeyDownloading))
		_, dlErr := cl.DownloadUpdate(ctx, func(pi bridge.ProgressInfo) {
			w.Synchronize(func() {
				w.SetProgress(pi.Index, pi.Total)
				w.SetContent(title, versionLabel(newer),
					i18n.Fmt(i18n.KeyDownloading, pi.File, pi.Index, pi.Total))
			})
		})
		if dlErr != nil {
			fail(w, interactive, newer, i18n.T(i18n.KeyErrorGeneric), dlErr.Error())
			return
		}
		w.Synchronize(func() {
			w.SetProgress(0, 0) // 收起进度条
			w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyDownloaded))
		})
	}

	// ⑤ 应用
	if res.decision.Ask && !w.AskYesNo(title, i18n.Fmt(i18n.KeyAskApply, newer)) {
		quit(w) // 用户选择稍后
		return
	}
	show(i18n.T(i18n.KeyApplying))

	if _, apErr := cl.ApplyUpdate(ctx); apErr != nil {
		fail(w, interactive, newer, i18n.T(i18n.KeyErrorGeneric), apErr.Error())
		return
	}

	// 成功后 aly-client 会 launchMainExe 拉起主程序，UI 使命结束。
	w.Synchronize(func() {
		w.SetContent(title, versionLabel(newer), i18n.T(i18n.KeyApplyDone))
	})
	time.Sleep(1200 * time.Millisecond) // 让用户看清「更新完成」
	quit(w)
}

// checkOutcome 是一次检查的可展示结果。
type checkOutcome struct {
	decision flow.Decision
	current  string
	newer    string
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
	res := checkOutcome{current: c.CurrentVersion, newer: c.NewVersion}

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

// fail 展示错误态。interactive 为 false（auto/silent）时静默退出。
func fail(w *view.Window, interactive bool, version, status, detail string) {
	if !interactive {
		quit(w)
		return
	}
	w.Synchronize(func() {
		w.SetContent(title, versionLabel(version), status)
		w.SetDetail(detail)
		w.SetError(true)
		w.Show()
	})
	autoClose(w)
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
