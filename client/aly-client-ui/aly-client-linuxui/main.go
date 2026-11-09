package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"

	"github.com/lishuangquan1987/aly/client/aly-client-ui/aly-client-linuxui/client"
)

var secondCh = make(chan []string, 4)

func main() {
	// 隐藏收尾模式：必须在 MyGo/GTK、单实例之前处理。
	if len(os.Args) >= 2 && os.Args[1] == "-finish-self-update" {
		os.Exit(finishSelfUpdate(os.Args[2:]))
	}

	fs := flag.NewFlagSet("aly-client-ui", flag.ExitOnError)
	mode := fs.String("mode", "auto", "auto|check|update|rollback|silent")
	lang := fs.String("lang", "zh-CN", "zh-CN|en-US")
	_ = fs.String("url", "", "reserved")
	projectOverride := fs.String("project-name", "", "window title override")
	_ = fs.Parse(os.Args[1:])

	m := normalizeMode(*mode)
	lng := normalizeLang(*lang)

	mygo.App.SetName("aly-client-ui")
	mygo.App.SetVersion("1.0.0")
	mygo.App.SetLanguages("en", "zh-Hans")
	mygo.App.SetLocale(lng)

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		select {
		case secondCh <- args:
		default:
		}
	})

	project := "aly-client-ui"
	a := &app{mode: m, lang: lng, project: project, userVisible: m != "auto" && m != "silent"}

	go func() {
		for args := range secondCh {
			a.handleSecond(args)
		}
	}()

	l, err := loadLayout()
	if err == nil {
		a.l = l
		if p := resolveProjectName(l, *projectOverride); p != "" {
			a.project = p
		}
		if _, statErr := os.Stat(l.updater); statErr == nil {
			a.cl = client.New(l.updater)
		}
		// 启动自更新：更新器优先，再 UI 自身。失败只记录，不阻断主流程。
		if uErr := updateUpdater(l); uErr != nil {
			appendUpdateLog(l, "updateUpdater failed: %v", uErr)
		}
		if replaced, uErr := updateSelfUI(l, os.Args[1:]); uErr != nil {
			appendUpdateLog(l, "updateSelfUI failed: %v", uErr)
		} else if replaced {
			return
		}
	} else {
		a.detail = ""
		appendUpdateLog(layout{}, "layout failed: %v", err)
	}

	if m == "auto" || m == "silent" {
		if a.cl == nil {
			return
		}
		p, cErr := a.cl.CheckUpdate(contextBackground())
		if cErr != nil || !p.Success || p.Check == nil {
			if a.pending {
				if m := modeFromArgs(a.pendingArgs); m != "" {
					a.mode = m
				}
				a.precheck = nil
				os.Exit(a.runUI())
			}
			return
		}
		if !p.Check.HasUpdate {
			if !a.pending {
				return
			}
			a.precheck = nil
		} else {
			chk := *p.Check
			a.precheck = &chk
		}
		if a.pending {
			if m := modeFromArgs(a.pendingArgs); m != "" {
				a.mode = m
			}
		}
	}

	os.Exit(a.runUI())
}

func normalizeMode(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "check", "update", "rollback", "silent":
		return strings.ToLower(strings.TrimSpace(m))
	default:
		return "auto"
	}
}

func (a *app) startMode() {
	if a.cl == nil {
		a.fail("check", "aly-client not found next to aly-client-ui")
		return
	}
	switch a.mode {
	case "rollback":
		a.startRollback()
	case "check", "update":
		a.startCheck()
	case "auto", "silent":
		if a.precheck != nil {
			a.handleCheck(*a.precheck)
			return
		}
		a.startCheck()
	default:
		a.startCheck()
	}
}

// contextBackground 是 context.Background 的薄封装，避免 main 直接 import
// context 只为一处调用。
func contextBackground() context.Context { return context.Background() }
