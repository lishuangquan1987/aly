// 假主程序（E2E 测试用）：
// 扮演"应用主程序"——apply_update 成功后会启动它。它在当前工作目录写一个
// fake_main_launched.txt（内容为工作目录路径），用于端到端断言主程序确实被启动。
//
// 编译（Go 1.10 GOPATH 模式兼容）：
//
//	go build -o fake_main.exe .
package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	// 工作目录即 mainFolder（由 launchMainExe 显式设置 cmd.Dir = mainFolder）
	wd, err := os.Getwd()
	if err != nil {
		os.Exit(1)
	}
	marker := filepath.Join(wd, "fake_main_launched.txt")
	ioutil.WriteFile(marker, []byte(wd+"\n"), 0644)
	// 主程序通常常驻；E2E 里立即退出即可（避免遗留进程）。
	// 但不立即退出更贴近真实主程序——这里保留一个短暂 sleep，便于测试期间探测。
	os.Exit(0)
}
