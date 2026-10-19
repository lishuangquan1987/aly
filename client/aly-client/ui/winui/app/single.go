// +build windows

package app

import (
	"syscall"
	"unsafe"
)

const (
	errorAlreadyExists = 183
	// 不加 Global\ 前缀：默认走**会话**命名空间。
	// Global\ 在 XP 多用户/快速用户切换下会因默认 ACL 而失败，
	// 而 UI 本来就是每用户一个实例。
	muName = `AlyClientUI_SingleInstance`
)

var (
	procCreateMutex = kernel32DLL.NewProc("CreateMutexW")
	procCloseHandle = kernel32DLL.NewProc("CloseHandle")
)

// AcquireSingleInstance 用命名互斥体实现单实例。
//
// 返回 (是否首个实例, 释放函数)。拿不到互斥体（极端情况）时按「首个实例」
// 处理并放行，避免因为系统调用失败而让 UI 完全起不来。
func AcquireSingleInstance() (bool, func()) {
	name, err := syscall.UTF16PtrFromString(muName)
	if err != nil {
		return true, func() {}
	}

	h, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true, func() {}
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		procCloseHandle.Call(h)
		return false, func() {}
	}

	released := false
	return true, func() {
		if released {
			return
		}
		released = true
		procCloseHandle.Call(h)
	}
}
