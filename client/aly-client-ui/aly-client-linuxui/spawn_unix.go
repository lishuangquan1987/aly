//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// spawnFinish 以新会话派生收尾进程，旧 UI 退出后它继续运行。
func spawnFinish(exePath string, args []string) error {
	c := exec.Command(exePath, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	return c.Start()
}
