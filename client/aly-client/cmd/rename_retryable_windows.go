package cmd

import (
	"os"
	"syscall"
)

// isRenameRetryableErr 判断重命名失败错误是否属于"占用类"错误（值得探测+击杀+重试）。
// Windows 版（审查修复 code-review-2026-11-23 #12）：
// 占用类（可重试）：ERROR_SHARING_VIOLATION(32) / ERROR_LOCK_VIOLATION(33) /
// ERROR_USER_MAPPED_FILE(1224) / ERROR_ACCESS_DENIED(5)。
// 非占用类（重试无意义，立即失败）：ERROR_NOT_SAME_DEVICE(17 跨卷) /
// ERROR_DIR_NOT_EMPTY(145) / ERROR_ALREADY_EXISTS(183 目标已存在，走旁移) /
// ERROR_PATH_NOT_FOUND(3 源不存在)。
// 无法分类时保守视为占用类（保持旧行为：探测→击杀→重试）。
//
// 注意：这些数值是 Windows 的 Win32 错误码，与 Linux errno 完全不同语义
//（Linux 的 5=EIO、32=EPIPE），因此按 GOOS 拆分文件——旧实现把 Windows
// 数值硬编码进无标签文件，Linux 构建下会把 I/O 错误误判为"占用"，
// 触发无意义的探测+杀进程+重试。
func isRenameRetryableErr(err error) bool {
	le, ok := err.(*os.LinkError)
	if !ok {
		return true
	}
	errno, ok := le.Err.(syscall.Errno)
	if !ok {
		return true
	}
	switch errno {
	case 32, 33, 1224, 5:
		return true
	default:
		return false
	}
}
