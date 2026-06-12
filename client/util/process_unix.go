// +build !windows

package util

import (
	"time"
)

// FindProcessesByName 查找指定名称的所有进程 PID（Unix 兼容实现）
func FindProcessesByName(name string) ([]uint32, error) {
	// 跨平台编译兼容：Unix 上进程管理通过外部命令（pgrep/kill）实现
	// 当前返回空，避免编译失败；完整实现可调用 pgrep
	return nil, nil
}

// KillProcess 终止指定 PID 的进程（Unix 兼容实现）
func KillProcess(pid uint32) error {
	return nil
}

// WaitForProcessExit 等待指定 PID 的进程退出（Unix 兼容实现）
func WaitForProcessExit(pid uint32, timeout time.Duration) bool {
	return true
}

// KillProcessesAndWait 终止指定名称列表的所有进程并等待它们退出（Unix 兼容实现）
func KillProcessesAndWait(names []string, timeout time.Duration) error {
	return nil
}

// SendCloseMessageToProcess 向指定 PID 的所有可见顶层窗口发送 WM_CLOSE 消息（Unix 无窗口系统兼容）
func SendCloseMessageToProcess(pid uint32) {
	// Unix 系统无窗口消息机制，跳过
}
