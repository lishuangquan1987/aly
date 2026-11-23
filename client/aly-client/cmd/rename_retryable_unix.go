// +build !windows

package cmd

import (
	"os"
	"syscall"
)

// isRenameRetryableErr 判断重命名失败错误是否属于"占用类"错误（值得探测+重试）。
// POSIX 版（审查修复 code-review-2026-11-23 #12）：
// 占用类（可重试）：EBUSY(16 资源忙) / EAGAIN(11 资源暂不可用) /
// ETXTBSY(26 文本文件忙——正在执行的二进制，Linux 更新场景常见) /
// EACCES(13 权限不足，保守可重试一次)。
// 非占用类（重试无意义，立即失败）：EXDEV(18 跨卷) / ENOTEMPTY(39) /
// EEXIST(17 目标已存在，走旁移) / ENOENT(2 源不存在)。
// 无法分类时保守视为占用类（保持旧行为：探测→重试）。
//
// 旧实现复用 Windows 的 errno 数值（32/33/1224/5），在 Linux 下
// 5=EIO、32=EPIPE——会把 I/O 错误误判为"占用"进而触发无意义重试。
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
	case syscall.EBUSY, syscall.EAGAIN, syscall.ETXTBSY, syscall.EACCES:
		return true
	default:
		return false
	}
}
