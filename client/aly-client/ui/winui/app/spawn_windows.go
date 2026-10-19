// +build windows

package app

import (
	"os/exec"
	"syscall"
)

const (
	// DETACHED_PROCESS：新进程不继承控制台。
	detachedProcess = 0x00000008
	// CREATE_NO_WINDOW：即使有控制台也不创建窗口。
	createNoWindowFlag = 0x08000000
)

// spawnDetached 启动一个完全脱离、不显示窗口的 cmd 进程。
//
// **不能用 exec.Command("cmd.exe", "/c", cmdLine)**：Go 会按 CreateProcess
// 的规则给含空格的参数加引号并用反斜杠转义内部引号，而 cmd.exe 不认这种
// 转义（它用 "" 表示引号），结果整条命令行会被拆错、move/start 全部失效。
// 因此这里用 SysProcAttr.CmdLine **原样**传递命令行。
func spawnDetached(cmdLine string) error {
	c := exec.Command("cmd.exe")
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd.exe /c ` + cmdLine,
		HideWindow:    true,
		CreationFlags: detachedProcess | createNoWindowFlag,
	}
	return c.Start()
}
