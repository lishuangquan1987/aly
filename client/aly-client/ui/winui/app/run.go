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
//  1. 与 UI 同目录 —— UI 部署在 UpdateFolder，aly-client.exe 也在那里，
//     这是最常见的布局，也最省事；
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

// outcome 是一次检查的可展示结果。
type outcome struct {
	decision flow.Decision
	current  string
	newer    string
	// problem 非空表示检查没成功（找不到 exe / 网络失败 / 服务端报错）。
	problem string
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

	go func() {
		res := runCheck(exePath, findErr)

		w.Synchronize(func() {
			// ① 检查没成功
			//
			// check（用户主动点按钮）：必须让用户看见，否则就是「点了没反应」。
			// auto（主程序启动时自动跑）：**静默退出**——网络不通/未配置服务器
			// 是常态，每次启动都弹错误框会变成骚扰（DEVELOPMENT_PLAN §4.1）。
			if res.problem != "" {
				if mode != "check" {
					w.Quit()
					return
				}
				w.SetContent(title, res.current, i18n.T(i18n.KeyOffline))
				w.SetDetail(res.problem)
				w.SetError(true)
				w.Show()
				_ = w.AutoCloseAfter(autoCloseDelay)
				return
			}

			// ② 无更新
			if res.decision.Action == flow.ActionNone {
				if mode == "check" {
					w.SetContent(title, res.current, i18n.T(i18n.KeyNoUpdate))
					w.SetDetail("")
					w.SetError(false)
					_ = w.AutoCloseAfter(autoCloseDelay)
				} else {
					// auto：无更新不显示 —— 结束消息循环，进程直接退出
					w.Quit()
				}
				return
			}

			// ③ 有更新：显示（下载/应用编排在任务 14 接入）
			w.SetContent(title, res.newer, i18n.Fmt(i18n.KeyFoundVersion, res.newer))
			w.SetDetail("")
			w.SetError(false)
			w.Show()
		})
	}()

	return w.Run()
}

// runCheck 调用 check_update 并交给状态机决策。
func runCheck(exePath string, findErr error) outcome {
	if findErr != nil {
		return outcome{problem: findErr.Error()}
	}

	cl := bridge.New(exePath)
	p, err := cl.CheckUpdate(context.Background())
	if err != nil {
		// 传输层失败（超时/进程异常退出）：报「无法检查」，不要谎称已是最新
		return outcome{problem: err.Error()}
	}
	if p.Check == nil {
		return outcome{problem: "unexpected response from aly-client check_update"}
	}

	c := p.Check
	res := outcome{current: c.CurrentVersion, newer: c.NewVersion}

	if !p.Success {
		// 服务端明确返回失败（例如未配置 server_url）
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
