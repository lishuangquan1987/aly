// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 按需显示）。
package main

import (
	"flag"
	"os"

	"aly/client/aly-client-ui/aly-client-winui/app"
)

func main() {
	// 隐藏收尾模式：旧 UI 已把新 exe 复制为 .new，并派生出本进程。
	// 必须在单实例互斥体/窗口创建之前处理；参数见 app.FinishSelfUpdate。
	if len(os.Args) >= 2 && os.Args[1] == "-finish-self-update" {
		os.Exit(app.FinishSelfUpdate(os.Args[2:]))
	}

	mode := flag.String("mode", "auto", "auto|check|update|rollback|silent")
	lang := flag.String("lang", "zh-CN", "zh-CN|en-US")
	serverURL := flag.String("url", "", "server url override (currently unused; aly-client reads client.json)")
	project := flag.String("project-name", "", "window title override (default: .updator/shared.json project_name)")
	// 收尾进程重启链传下来的原始宿主 PID：注册仅为让 flag.Parse 认识它
	//（未知参数会 ExitOnError），真正的消费在 app.WatchHostExit 里扫 os.Args。
	_ = flag.Uint64("host-pid", 0, "original host pid (passed down by -finish-self-update restart)")
	flag.Parse()

	// 宿主生命周期（规范见 spec/host-contract.md §15）：宿主退出后 UI 跟随退出；
	// 例外是 version_status == applying 的 apply 窗口。
	app.WatchHostExit()

	os.Exit(app.Run(app.Options{
		Mode:        *mode,
		Lang:        *lang,
		ServerURL:   *serverURL,
		ProjectName: *project,
	}))
}
