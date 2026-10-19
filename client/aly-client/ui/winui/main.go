// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 按需显示）。
package main

import (
	"fmt"
	"os"

	"github.com/lxn/walk"
)

func main() {
	// M0 骨架：仅验证 walk 能在 Go 1.10.8 + GOARCH=386 下编译链接。
	mw, err := walk.NewMainWindow()
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk init failed: %v\n", err)
		os.Exit(1)
	}
	mw.Dispose()
}
