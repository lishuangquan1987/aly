// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 按需显示）。
package main

import (
	"fmt"
	"os"

	"aly/client/aly-client/ui/winui/view"
)

func main() {
	w, err := view.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create window failed: %v\n", err)
		os.Exit(1)
	}
	// M0 阶段：直接显示，用于验证右下角定位 / topmost / 不进任务栏。
	// 按需显示与 5s 自动关闭在任务 11 接入。
	w.Show()
	os.Exit(w.Run())
}
