// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 按需显示）。
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"aly/client/aly-client/ui/winui/i18n"
	"aly/client/aly-client/ui/winui/view"
)

// autoCloseDelay 是 -mode check 下「无更新」提示的自动关闭时长。
const autoCloseDelay = 5 * time.Second

func main() {
	mode := flag.String("mode", "auto", "auto|check|update|rollback|silent")
	lang := flag.String("lang", "zh-CN", "zh-CN|en-US")
	flag.Parse()

	// 语言只由主程序传入；未传或未知一律中文，不按系统区域推断。
	i18n.SetLang(i18n.ParseLang(*lang))

	w, err := view.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create window failed: %v\n", err)
		os.Exit(1)
	}

	switch *mode {
	case "auto", "silent":
		// 启动模式：静默检查，有更新才显示。
		// 真实检查逻辑在任务 13 接线；在此之前无更新 → 直接退出，不显示窗口。
		os.Exit(0)

	case "check":
		// 手动检查：总是弹窗；无更新时显示提示并在 autoCloseDelay 后自动关闭。
		// （任务 13 接入真实的 check 结果；这里先验证显示/自动关闭链路）
		w.Show()
		_ = w.AutoCloseAfter(autoCloseDelay)
		os.Exit(w.Run())

	default:
		// update / rollback / 未知模式：显示窗口并进入消息循环
		w.Show()
		os.Exit(w.Run())
	}
}
