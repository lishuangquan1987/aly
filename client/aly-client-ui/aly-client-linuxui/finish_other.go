//go:build !linux

package main

// 非 Linux 构建（仅用于 windows/macOS 上的编译与视图测试）：收尾模式不可用。
func waitForOldProcess(pid int, created string, target string) bool { return false }
