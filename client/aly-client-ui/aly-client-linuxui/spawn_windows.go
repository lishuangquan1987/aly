//go:build windows

package main

import "os/exec"

// Windows 仅用于编译/视图测试；Linux UI 的收尾进程不在此平台使用。
func spawnFinish(exePath string, args []string) error {
	c := exec.Command(exePath, args...)
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	return c.Start()
}
