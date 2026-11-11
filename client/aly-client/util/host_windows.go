// +build windows

package util

import (
	"os"
	"unsafe"
)

// hostProcessID Windows：遍历 Toolhelp32 快照，取本进程条目的 ParentProcessID。
// 复用与 FindProcessesByName 完全相同的快照机制，不额外引入依赖。
func hostProcessID() uint32 {
	self := uint32(os.Getpid())
	if self == 0 {
		return 0
	}

	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPPROCESS),
		0,
	)
	if snapshot == INVALID_HANDLE_VALUE {
		return 0
	}
	defer procCloseHandle.Call(snapshot)

	var entry PROCESSENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(
		snapshot,
		uintptr(unsafe.Pointer(&entry)),
	)
	if ret == 0 {
		return 0
	}

	for {
		if entry.ProcessID == self {
			return entry.ParentProcessID
		}

		ret, _, _ := procProcess32NextW.Call(
			snapshot,
			uintptr(unsafe.Pointer(&entry)),
		)
		if ret == 0 {
			break
		}
	}

	return 0
}
