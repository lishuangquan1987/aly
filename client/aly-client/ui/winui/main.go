// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 按需显示）。
package main

import (
	"flag"
	"os"

	"aly/client/aly-client/ui/winui/app"
)

func main() {
	mode := flag.String("mode", "auto", "auto|check|update|rollback|silent")
	lang := flag.String("lang", "zh-CN", "zh-CN|en-US")
	serverURL := flag.String("url", "", "server url override (currently unused; aly-client reads client.json)")
	project := flag.String("project-name", "", "project name override (currently unused)")
	flag.Parse()

	os.Exit(app.Run(app.Options{
		Mode:        *mode,
		Lang:        *lang,
		ServerURL:   *serverURL,
		ProjectName: *project,
	}))
}
