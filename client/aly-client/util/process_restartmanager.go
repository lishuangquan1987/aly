// +build windows

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// Restart Manager API 用于探测并定位占用指定文件/文件夹的进程。
// 参考: https://learn.microsoft.com/en-us/windows/win32/api/restartmanager/
// rstrtmgr.dll 在 Windows XP 及以上系统提供（XP 上若 API 缺失则返回错误，由调用方降级处理）。
//
// 注意：Restart Manager 注册"文件夹"时检测不到持有文件夹内文件的进程
// （实测：文件夹内文件被打开，注册文件夹返回"无占用者"，注册该文件才能找到）。
// 因此对目录必须逐个注册其下所有文件。

const (
	errorMoreData     = 234 // ERROR_MORE_DATA
	errorSuccess      = 0   // ERROR_SUCCESS
	rmSessionKeyLen   = 33  // CCH_RM_SESSION_KEY (32) + 1
	rmMaxAppName      = 255
	rmMaxSvcName      = 63
	// rmBatchSize RmRegisterResources 单次注册资源数量上限（实测 400 安全）
	rmBatchSize = 400
)

var (
	restartMgrDLL        = syscall.NewLazyDLL("rstrtmgr.dll")
	procRmStartSession   = restartMgrDLL.NewProc("RmStartSession")
	procRmRegisterRes    = restartMgrDLL.NewProc("RmRegisterResources")
	procRmGetList        = restartMgrDLL.NewProc("RmGetList")
	procRmEndSession     = restartMgrDLL.NewProc("RmEndSession")
)

type rmUniqueProcess struct {
	ProcessID        uint32
	ProcessStartTime syscall.Filetime
}

type rmProcessInfo struct {
	Process             rmUniqueProcess
	StrAppName          [rmMaxAppName + 1]uint16
	StrServiceShortName [rmMaxSvcName + 1]uint16
	ApplicationType     uint32
	AppStatus           uint32
	TSSessionID         uint32
	BRestartable        int32
}

// FindProcessesHoldingPath 使用 Restart Manager API 找出占用指定文件/文件夹的进程 PID。
// 文件夹会被展开为其中的所有文件逐一注册（否则检测不到持有文件夹内文件的进程）。
// 返回 nil 且无错误表示该 API 不可用或当前没有占用者，不视为失败。
func FindProcessesHoldingPath(path string) ([]uint32, error) {
	// XP 等不支持 Restart Manager 的环境，函数地址为 0
	if procRmStartSession.Addr() == 0 {
		return nil, nil
	}

	var sessionHandle uint32
	var sessionKey [rmSessionKeyLen]uint16
	ret, _, _ := procRmStartSession.Call(
		uintptr(unsafe.Pointer(&sessionHandle)),
		0, // dwSessionFlags: 0
		uintptr(unsafe.Pointer(&sessionKey[0])),
	)
	if ret != errorSuccess {
		return nil, fmt.Errorf("RmStartSession failed: 0x%x", ret)
	}
	defer procRmEndSession.Call(uintptr(sessionHandle))

	resources := collectResources(path)
	if err := registerResources(sessionHandle, resources); err != nil {
		return nil, err
	}

	var procInfoNeeded uint32
	var procInfo uint32
	var rebootReasons uint32
	ret, _, _ = procRmGetList.Call(
		uintptr(sessionHandle),
		uintptr(unsafe.Pointer(&procInfoNeeded)),
		uintptr(unsafe.Pointer(&procInfo)),
		0,
		uintptr(unsafe.Pointer(&rebootReasons)),
	)
	if procInfoNeeded == 0 {
		return nil, nil // 无占用者
	}

	sizeOfInfo := unsafe.Sizeof(rmProcessInfo{})
	buf := make([]byte, int(procInfoNeeded)*int(sizeOfInfo))
	// pnProcInfo 是 in/out 参数：输入时必须为缓冲区能容纳的条目数
	procInfo = procInfoNeeded
	ret, _, _ = procRmGetList.Call(
		uintptr(sessionHandle),
		uintptr(unsafe.Pointer(&procInfoNeeded)),
		uintptr(unsafe.Pointer(&procInfo)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&rebootReasons)),
	)
	if ret != errorSuccess {
		return nil, fmt.Errorf("RmGetList failed: 0x%x", ret)
	}

	var pids []uint32
	base := uintptr(unsafe.Pointer(&buf[0]))
	for i := uint32(0); i < procInfo; i++ {
		info := (*rmProcessInfo)(unsafe.Pointer(base + uintptr(i)*sizeOfInfo))
		pids = append(pids, info.Process.ProcessID)
	}
	return pids, nil
}

// collectResources 收集需要注册的资源：
// 文件直接注册本身；文件夹展开为其下所有文件（含子目录），并保留文件夹自身，
// 以便覆盖持有目录句柄（如 CWD）的进程。
func collectResources(path string) []string {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return []string{path}
	}
	var res []string
	_ = filepath.Walk(path, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // 跳过无法访问的条目，不影响其他文件
		}
		if !fi.IsDir() {
			res = append(res, p)
		}
		return nil
	})
	if len(res) == 0 {
		// 空目录：注册目录本身
		res = append(res, path)
	}
	return res
}

// registerResources 分批调用 RmRegisterResources 注册一组资源路径
func registerResources(sessionHandle uint32, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	for i := 0; i < len(paths); i += rmBatchSize {
		end := i + rmBatchSize
		if end > len(paths) {
			end = len(paths)
		}
		chunk := paths[i:end]
		ptrs := make([]uintptr, len(chunk))
		for j, p := range chunk {
			ptr, err := syscall.UTF16PtrFromString(p)
			if err != nil {
				return err
			}
			ptrs[j] = uintptr(unsafe.Pointer(ptr))
		}
		ret, _, _ := procRmRegisterRes.Call(
			uintptr(sessionHandle),
			uintptr(len(chunk)),
			uintptr(unsafe.Pointer(&ptrs[0])),
			0, // nApplications
			0,
			0, // nServices
			0,
		)
		if ret != errorSuccess {
			return fmt.Errorf("RmRegisterResources failed: 0x%x", ret)
		}
	}
	return nil
}
