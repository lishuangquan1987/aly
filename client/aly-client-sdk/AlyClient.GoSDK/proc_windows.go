//go:build windows

package alyclient

import (
	"os/exec"
	"syscall"
)

// hideWindow 让子进程不弹出控制台窗口。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
