// +build windows

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// 全系统句柄枚举：找出持有目标路径（文件/目录）句柄的所有进程。
//
// 覆盖 Restart Manager 与 32 位 CWD 探测的盲区：
//   - 64 位原生进程（32 位客户端读不了其 PEB，但能枚举其句柄并杀掉）；
//   - 仅持有目录句柄的进程（如 cmd.exe 工作目录、Explorer 窗口浏览的文件夹）；
//   - RM 探测不到的句柄（权限/会话差异）。
//
// 实现为 handle.exe 的原理：NtQuerySystemInformation(句柄表) + DuplicateHandle +
// NtQueryObject(对象名)，按对象名匹配目标路径。耗时（全系统句柄遍历），仅作最后手段。

const (
	scanSystemHandleInformation         = 16 // XP 支持的基础类
	scanSystemExtendedHandleInformation = 64 // Vista+ 扩展类（条目含 ObjectTypeIndex）
	scanObjectNameInformation           = 1
	scanProcessDupHandle                = 0x0040
	scanDuplicateSameAccess             = 0x2
	scanStatusInfoLengthMismatch        = 0xC0000004
)

var (
	scanNtdll             = syscall.NewLazyDLL("ntdll.dll")
	scanKernel32          = syscall.NewLazyDLL("kernel32.dll")
	procNtQuerySysInfo    = scanNtdll.NewProc("NtQuerySystemInformation")
	procNtQueryObject     = scanNtdll.NewProc("NtQueryObject")
	procDuplicateHandle2  = scanKernel32.NewProc("DuplicateHandle")
	procQueryDosDevice    = scanKernel32.NewProc("QueryDosDeviceW")
)

// systemHandleTableEntryInfo XP 基础类条目（16 字节，32 位）
type systemHandleTableEntryInfo struct {
	UniqueProcessId     uint16
	CreatorBackTraceIdx uint16
	ObjectTypeIndex     uint8
	HandleAttributes    uint8
	HandleValue         uint16
	Object              uintptr
	GrantedAccess       uint32
}

// systemHandleTableEntryInfoEx 扩展类条目（32 位 28 字节；客户端固定 GOARCH=386）
type systemHandleTableEntryInfoEx struct {
	Object               uintptr
	UniqueProcessId      uintptr
	HandleValue          uintptr
	GrantedAccess        uint32
	CreatorBackTraceIdx  uint16
	ObjectTypeIndex      uint16
	HandleAttributes     uint32
	Reserved             uint32
}

// FindProcessesWithHandlesUnder 返回持有 dir（或 dir 下任意文件/子目录）句柄的进程 PID。
// 慢（全系统句柄枚举），仅作最后手段调用；环境不支持句柄枚举时返回 nil。
func FindProcessesWithHandlesUnder(dir string) []uint32 {
	prefixes := buildTargetNTForms(dir)
	if len(prefixes) == 0 {
		return nil
	}

	buf, count, err := queryHandleTable(scanSystemExtendedHandleInformation)
	extended := err == nil
	if err != nil {
		// XP 无扩展类，回退基础类
		buf, count, err = queryHandleTable(scanSystemHandleInformation)
		if err != nil {
			return nil
		}
	}

	selfPid := uint32(os.Getpid())
	openCache := make(map[uint32]uintptr)
	failedPIDs := make(map[uint32]bool)
	defer func() {
		for _, h := range openCache {
			procCloseHandle.Call(h)
		}
	}()

	var result []uint32
	seen := make(map[uint32]bool)

	if extended {
		entrySize := unsafe.Sizeof(systemHandleTableEntryInfoEx{})
		base := uintptr(unsafe.Pointer(&buf[0])) + 8 // 头 8 字节为句柄数量
		for i := 0; i < count; i++ {
			e := (*systemHandleTableEntryInfoEx)(unsafe.Pointer(base + uintptr(i)*entrySize))
			if e.HandleValue == 0 {
				continue
			}
			pid := uint32(e.UniqueProcessId)
			if pid == selfPid {
				continue
			}
			name := queryObjectName(pid, e.HandleValue, openCache, failedPIDs)
			if name != "" && handleNameMatches(strings.ToLower(name), prefixes) && !seen[pid] {
				seen[pid] = true
				result = append(result, pid)
			}
		}
	} else {
		entrySize := unsafe.Sizeof(systemHandleTableEntryInfo{})
		base := uintptr(unsafe.Pointer(&buf[0])) + 8
		for i := 0; i < count; i++ {
			e := (*systemHandleTableEntryInfo)(unsafe.Pointer(base + uintptr(i)*entrySize))
			if e.HandleValue == 0 {
				continue
			}
			pid := uint32(e.UniqueProcessId)
			if pid == selfPid {
				continue
			}
			name := queryObjectName(pid, uintptr(e.HandleValue), openCache, failedPIDs)
			if name != "" && handleNameMatches(strings.ToLower(name), prefixes) && !seen[pid] {
				seen[pid] = true
				result = append(result, pid)
			}
		}
	}
	return result
}

// queryHandleTable 查询句柄表，循环扩容直到成功（0xC0000004 = 缓冲区不足）。
// 增加重试上限与缓冲区上限：若系统句柄表异常导致 retLen 不增长（或持续返回 mismatch），
// 直接返回错误由调用方降级处理，避免无限循环挂死/内存膨胀（#20）。
func queryHandleTable(infoClass uintptr) ([]byte, int, error) {
	const maxRetries = 16
	const maxCap = 1 << 30 // 1GB 缓冲区上限，防止恶意/异常系统状态撑爆内存

	var retLen uint32
	procNtQuerySysInfo.Call(infoClass, 0, 0, uintptr(unsafe.Pointer(&retLen)))
	// 注意：GOARCH=386 时 int 是 32 位，retLen 是 uint32，必须先在 uint32 空间
	// 与 maxCap 比较，再转 int，避免 int(retLen) 溢出为负绕过上限（#20）。
	if retLen > uint32(maxCap) {
		return nil, 0, fmt.Errorf("NtQuerySystemInformation required buffer %d exceeds cap %d", retLen, maxCap)
	}
	capLen := int(retLen) + 65536
	if capLen > maxCap {
		capLen = maxCap
	}
	buf := make([]byte, capLen)
	for attempt := 0; attempt < maxRetries; attempt++ {
		r, _, _ := procNtQuerySysInfo.Call(infoClass, uintptr(unsafe.Pointer(&buf[0])), uintptr(capLen), uintptr(unsafe.Pointer(&retLen)))
		if r == scanStatusInfoLengthMismatch {
			// 系统要求的长度没有超过当前缓冲区（未增长）或超出上限：继续扩容无意义，直接失败。
			// 同样在 uint32 空间比较，避免 386 下 int 溢出（#20）。
			if retLen <= uint32(capLen) || retLen > uint32(maxCap) {
				return nil, 0, fmt.Errorf("NtQuerySystemInformation buffer did not converge (retLen=%d, capLen=%d)", retLen, capLen)
			}
			capLen = int(retLen) + 65536
			if capLen > maxCap {
				capLen = maxCap
			}
			buf = make([]byte, capLen)
			continue
		}
		if r != 0 {
			return nil, 0, fmt.Errorf("NtQuerySystemInformation class %d failed: 0x%x", infoClass, r)
		}
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		return buf, int(count), nil
	}
	return nil, 0, fmt.Errorf("NtQuerySystemInformation class %d exceeded %d retries", infoClass, maxRetries)
}

// queryObjectName 复制句柄到本进程并查询对象名（NT 命名空间形式，如 \??\C:\...）。
// 无法打开目标进程（权限）或对象无名字时返回 ""。
func queryObjectName(pid uint32, handle uintptr, openCache map[uint32]uintptr, failedPIDs map[uint32]bool) string {
	ph, ok := openCache[pid]
	if !ok {
		if failedPIDs[pid] {
			return "" // 已知打不开（如系统/受保护进程），跳过，避免重复 OpenProcess
		}
		h, _, _ := procOpenProcess.Call(uintptr(scanProcessDupHandle), 0, uintptr(pid))
		if h == 0 {
			failedPIDs[pid] = true
			return ""
		}
		openCache[pid] = h
		ph = h
	}
	var dup uintptr
	dr, _, _ := procDuplicateHandle2.Call(
		uintptr(INVALID_HANDLE_VALUE), // GetCurrentProcess()
		handle,
		ph,
		uintptr(unsafe.Pointer(&dup)),
		0,
		0,
		uintptr(scanDuplicateSameAccess),
	)
	if dr == 0 {
		return ""
	}
	defer procCloseHandle.Call(dup)

	var retLen uint32
	procNtQueryObject.Call(dup, uintptr(scanObjectNameInformation), 0, 0, uintptr(unsafe.Pointer(&retLen)))
	if retLen == 0 || retLen > 1<<20 {
		return ""
	}
	buf := make([]byte, int(retLen)+8)
	r, _, _ := procNtQueryObject.Call(dup, uintptr(scanObjectNameInformation), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&retLen)))
	if r != 0 {
		return ""
	}
	length := *(*uint16)(unsafe.Pointer(&buf[0]))
	if length == 0 || length > 4096 {
		return ""
	}
	// OBJECT_NAME_INFORMATION = { UNICODE_STRING Name; }，Buffer 指针位于
	// Length(2)+MaximumLength(2) 之后，按指针宽度对齐：32 位偏移 4、64 位偏移 8。
	bufferOff := unsafe.Sizeof(uintptr(0))
	ptr := *(*uintptr)(unsafe.Pointer(&buf[bufferOff]))
	return utf16PtrToString(ptr, int(length)/2)
}

// buildTargetNTForms 生成目标目录的 NT 命名空间前缀（小写）：
//   - \??\c:\otdr3001\win-x64（对象名最常见形态）
//   - \device\harddiskvolumeN\otdr3001\win-x64（盘符经 QueryDosDevice 映射）
func buildTargetNTForms(dir string) []string {
	dir = strings.ToLower(filepath.Clean(dir))
	if dir == "" {
		return nil
	}
	forms := []string{`\??\` + dir}
	if len(dir) >= 2 && dir[1] == ':' {
		drive := strings.ToUpper(dir[:2])
		drivePtr, perr := syscall.UTF16PtrFromString(drive)
		if perr == nil {
			buf := make([]uint16, 256)
			r, _, _ := procQueryDosDevice.Call(
				uintptr(unsafe.Pointer(drivePtr)),
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(len(buf)),
			)
			if r > 0 {
				dev := strings.ToLower(syscall.UTF16ToString(buf[:r]))
				forms = append(forms, dev+dir[2:])
			}
		}
	}
	return forms
}

// handleNameMatches 判断对象名（NT 命名空间，已小写）是否命中目标前缀（目录本身或其下任意路径）。
func handleNameMatches(nameLower string, prefixes []string) bool {
	for _, p := range prefixes {
		if nameLower == p || strings.HasPrefix(nameLower, p+`\`) {
			return true
		}
	}
	return false
}

// utf16PtrToString 从内存指针读取 UTF-16 字符串
func utf16PtrToString(p uintptr, n int) string {
	if p == 0 || n <= 0 {
		return ""
	}
	u := (*[1 << 20]uint16)(unsafe.Pointer(p))[:n]
	return syscall.UTF16ToString(u)
}
