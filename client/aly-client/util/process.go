// +build windows

package util

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procTerminateProcess         = kernel32.NewProc("TerminateProcess")
	procWaitForSingleObject      = kernel32.NewProc("WaitForSingleObject")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
)

const (
	TH32CS_SNAPPROCESS  = 0x00000002
	INVALID_HANDLE_VALUE = ^uintptr(0)

	PROCESS_TERMINATE = 0x0001
	SYNCHRONIZE       = 0x00100000

	WAIT_OBJECT_0 = 0
	WAIT_TIMEOUT  = 0x00000102

	WM_CLOSE = 0x0010

	MAX_PATH = 260
)

type PROCESSENTRY32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [MAX_PATH]uint16
}

// FindProcessesByName 查找指定名称的所有进程 PID
func FindProcessesByName(name string) ([]uint32, error) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPPROCESS),
		0,
	)
	if snapshot == INVALID_HANDLE_VALUE {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot 失败")
	}
	defer procCloseHandle.Call(snapshot)

	var entry PROCESSENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(
		snapshot,
		uintptr(unsafe.Pointer(&entry)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("Process32First 失败")
	}

	nameLower := toLower(name)
	var pids []uint32

	for {
		exeName := syscall.UTF16ToString(entry.ExeFile[:])
		if toLower(exeName) == nameLower || toLowerWithoutExt(exeName) == nameLower {
			pids = append(pids, entry.ProcessID)
		}

		ret, _, _ := procProcess32NextW.Call(
			snapshot,
			uintptr(unsafe.Pointer(&entry)),
		)
		if ret == 0 {
			break
		}
	}

	return pids, nil
}

// KillProcess 终止指定 PID 的进程
func KillProcess(pid uint32) error {
	handle, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_TERMINATE),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return fmt.Errorf("无法打开进程 %d", pid)
	}
	defer procCloseHandle.Call(handle)

	ret, _, _ := procTerminateProcess.Call(handle, 1)
	if ret == 0 {
		return fmt.Errorf("无法终止进程 %d", pid)
	}

	return nil
}

// WaitForProcessExit 等待指定 PID 的进程退出
// 返回 true 表示进程已退出，false 表示超时
func WaitForProcessExit(pid uint32, timeout time.Duration) bool {
	handle, _, callErr := procOpenProcess.Call(
		uintptr(SYNCHRONIZE),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		// OpenProcess 失败：通过错误码区分原因
		// ERROR_INVALID_PARAMETER (87) 通常表示进程不存在
		// ERROR_ACCESS_DENIED (5) 表示进程存在但权限不足
		if errno, ok := callErr.(syscall.Errno); ok && errno == 87 {
			return true // 进程已退出
		}
		// 权限不足或其他错误，无法判断进程状态，保守返回 false
		return false
	}
	defer procCloseHandle.Call(handle)

	timeoutMs := uintptr(timeout / time.Millisecond)
	if timeoutMs == 0 {
		timeoutMs = 1
	}

	ret, _, _ := procWaitForSingleObject.Call(handle, timeoutMs)
	return ret == WAIT_OBJECT_0
}

// KillProcessesAndWait 等待指定名称列表的进程退出，超时后强杀
func KillProcessesAndWait(names []string, timeout time.Duration) error {
	var allPIDs []uint32

	// 查找所有需要关闭的进程
	for _, name := range names {
		pids, err := FindProcessesByName(name)
		if err != nil {
			return fmt.Errorf("查找进程 %s 失败: %v", name, err)
		}
		allPIDs = append(allPIDs, pids...)
	}

	if len(allPIDs) == 0 {
		return nil
	}

	// 先等待进程自行退出（给优雅关闭的时间）
	dead := make(map[uint32]bool)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allDead := true
		for _, pid := range allPIDs {
			if dead[pid] {
				continue
			}
			if WaitForProcessExit(pid, 500*time.Millisecond) {
				dead[pid] = true
			} else {
				allDead = false
			}
		}
		if allDead {
			return nil
		}
	}

	// 超时后强制终止残留进程
	for _, pid := range allPIDs {
		if !dead[pid] {
			if err := KillProcess(pid); err != nil {
				fmt.Fprintf(os.Stderr, "KillProcess %d failed: %v\n", pid, err)
			}
		}
	}

	// 再次等待残留进程退出（给 TerminateProcess 生效时间）
	for _, pid := range allPIDs {
		if !dead[pid] {
			WaitForProcessExit(pid, 2*time.Second)
		}
	}

	return nil
}

// SendCloseMessageToProcess 向指定 PID 的所有可见顶层窗口发送 WM_CLOSE 消息
func SendCloseMessageToProcess(pid uint32) {
	pidPtr := pid
	procEnumWindows.Call(syscall.NewCallback(func(hwnd syscall.Handle, lParam uintptr) uintptr {
		var windowPid uint32
		procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&windowPid)))
		if windowPid == pidPtr {
			visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
			if visible != 0 {
				procSendMessageW.Call(uintptr(hwnd), uintptr(WM_CLOSE), 0, 0)
			}
		}
		return 1 // 继续枚举
	}), 0)
}

// toLower 简单的字符串转小写（仅处理 ASCII）
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// toLowerWithoutExt 去除 .exe 后缀后转小写
func toLowerWithoutExt(s string) string {
	lowered := toLower(s)
	if len(lowered) > 4 && lowered[len(lowered)-4:] == ".exe" {
		return lowered[:len(lowered)-4]
	}
	return lowered
}
