// +build !windows

package util

import (
	"syscall"
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

// KillPIDsAndWait 等待指定 PID 列表的进程退出，超时后强杀（Unix 兼容实现）
func KillPIDsAndWait(pids []uint32, timeout time.Duration) error {
	return nil
}

// ForceKillPIDs 直接强制结束指定 PID 列表的进程（Unix 兼容实现，当前为空操作）
func ForceKillPIDs(pids []uint32, waitTimeout time.Duration) {
}

// IsProcessAlive 判断指定 PID 的进程是否存活（Unix 实现：kill -0 探测）。
//
// 2026-11-09 前为恒 false 的编译兼容桩，导致 .aly.lock 陈旧锁检测失效：
// 持有者活着也会被判定为残留、锁被抢占 → 可能双实例并发更新（Linux 下
// 3 个锁单测失败）。改为 kill -0 真语义：
//   - err == nil：信号送达（有权限）→ 存活；
//   - err == EPERM：进程存在但无权限发信号 → 仍算存活；
//   - ESRCH 及其他：不存在 / 已退出 → 不存活。
//
// 与 Windows 版（OpenProcess 探测）语义对齐，是 self_update / apply_update
// 并发锁（cmd/lock.go cleanStaleLock）在 Unix 上正确性的前提。
func IsProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}
	err := syscall.Kill(int(pid), 0)
	return err == nil || err == syscall.EPERM
}

// SendCloseMessageToProcess 向指定 PID 的所有可见顶层窗口发送 WM_CLOSE 消息（Unix 无窗口系统兼容）
func SendCloseMessageToProcess(pid uint32) {
	// Unix 系统无窗口消息机制，跳过
}

// SendCloseMessageToExplorer 关闭 explorer 文件浏览窗口（Unix 无 explorer 兼容）
func SendCloseMessageToExplorer(pid uint32) {
	// Unix 系统无资源管理器，跳过
}

// FilterKillablePIDs 过滤可安全强杀的 PID（Unix 兼容实现：一律保护，不做任何强杀）
func FilterKillablePIDs(pids []uint32) (killable []uint32, blocked []uint32) {
	return nil, pids
}

// FindProcessNamesByPIDs 返回给定 PID 集合的进程名映射（Unix 兼容实现：
// 返回空映射——仅用于日志展示"击杀了哪些进程"，空映射只影响可读性）。
// 完整实现可读 /proc/<pid>/comm。
func FindProcessNamesByPIDs(pids []uint32) map[uint32]string {
	return map[uint32]string{}
}

// FindProcessesWithCWDUnder 查找 CWD 位于指定目录下的进程（Unix 兼容实现：
// 不做探测，返回空。完整实现可比对 /proc/<pid>/cwd 与目录前缀）。
func FindProcessesWithCWDUnder(dir string) []uint32 {
	return nil
}

// FindProcessesWithHandlesUnder 查找持有指定目录下句柄的进程（Unix 兼容实现：
// 不做探测，返回空。Windows 侧依赖 NtQuerySystemInformation 句柄表，Unix 无对等
// 通用接口；目录占用对 Unix 的 rename 影响也远小于 Windows）。
func FindProcessesWithHandlesUnder(dir string) []uint32 {
	return nil
}

// CloseExplorerWindowsBrowsing 关闭正在浏览指定目录的资源管理器窗口
//（Unix 兼容实现：无 explorer，直接跳过并返回 0）。
func CloseExplorerWindowsBrowsing(folder string) (int, error) {
	return 0, nil
}
