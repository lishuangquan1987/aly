// +build windows

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	ntdll    = syscall.NewLazyDLL("ntdll.dll")

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
	procGetClassNameW            = user32.NewProc("GetClassNameW")

	procNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
	procNtReadVirtualMemory       = ntdll.NewProc("NtReadVirtualMemory")
	procIsWow64Process            = kernel32.NewProc("IsWow64Process")
)

const (
	TH32CS_SNAPPROCESS   = 0x00000002
	INVALID_HANDLE_VALUE = ^uintptr(0)

	PROCESS_TERMINATE         = 0x0001
	PROCESS_VM_READ           = 0x0010
	SYNCHRONIZE               = 0x00100000
	PROCESS_QUERY_INFORMATION = 0x0400

	WAIT_OBJECT_0 = 0
	WAIT_TIMEOUT  = 0x00000102

	WM_CLOSE = 0x0010

	MAX_PATH = 260
)

// processBasicInformation 对应 NT 的 PROCESS_BASIC_INFORMATION（32/64 位通用，uintptr 对齐）
type processBasicInformation struct {
	Reserved1       uintptr
	PebBaseAddress  uintptr
	Reserved2       [2]uintptr
	UniqueProcessId uintptr
	Reserved3       uintptr
}

// 客户端固定 GOARCH=386（32 位）。32 位客户端只能读取"32 位目标进程"的 PEB：
//   - 32 位系统（XP）：所有进程均为 32 位，直接用 32 位偏移；
//   - 64 位系统：WOW64（32 位）进程用 32 位偏移；64 位原生进程无法由 32 位客户端读取 PEB，跳过。
// PEB / RTL_USER_PROCESS_PARAMETERS 的 32 位偏移（实测验证）：
//   - PEB.ProcessParameters              = PEB + 0x10
//   - ProcessParameters.CurrentDirectory = + 0x24（CURDIR 的 DosPath：UNICODE_STRING
//     { Length(2); MaximumLength(2); Buffer(4) }，即结构体基址在 0x24，Buffer 字段在 +4）
const (
	pebProcessParametersOffset32 = 0x10
	procParamsCurDirOffset       = 0x24
)

// processWow64Information 对应 NtQueryInformationProcess 的 ProcessWow64Information(26) 类
const processWow64Information = 26

// osIs64Bit 判断当前运行环境是否为 64 位 Windows（结果缓存，仅计算一次）。
// 客户端固定 32 位：若自身处于 WOW64（32 位跑在 64 位系统上）则为 64 位系统，否则为 32 位系统。
// IsWow64Process 自 XP SP2 起提供：XP RTM/SP1 上导出缺失时（proc.Addr()==0），
// 直接视为 32 位系统（避免 LazyProc.Call 对缺失导出 panic）。
var (
	osBitsOnce  sync.Once
	osIs64BitV  bool
)

func osIs64Bit() bool {
	osBitsOnce.Do(func() {
		if procIsWow64Process.Addr() == 0 {
			osIs64BitV = false // XP RTM/SP1 无此导出，视为 32 位系统
			return
		}
		var isWow uint32
		ret, _, _ := procIsWow64Process.Call(
			uintptr(INVALID_HANDLE_VALUE), // GetCurrentProcess() 伪句柄
			uintptr(unsafe.Pointer(&isWow)),
		)
		osIs64BitV = ret != 0 && isWow != 0
	})
	return osIs64BitV
}

// processIs32Bit 判断目标进程是否为 32 位。
// 32 位系统上恒为 true；64 位系统上通过 ProcessWow64Information 判断（非 0 = WOW64 32 位进程）。
func processIs32Bit(handle uintptr) bool {
	if !osIs64Bit() {
		return true
	}
	var wow64 uintptr
	procNtQueryInformationProcess.Call(
		handle,
		uintptr(processWow64Information),
		uintptr(unsafe.Pointer(&wow64)),
		uintptr(unsafe.Sizeof(wow64)),
		0,
	)
	return wow64 != 0
}

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

// FindProcessNamesByPIDs 返回给定 PID 集合的进程名映射（小写、去 .exe 后缀）。
// 用于更新前识别非白名单占用进程并向用户提示（#13）；找不到的 PID 不包含在结果中。
func FindProcessNamesByPIDs(pids []uint32) map[uint32]string {
	result := make(map[uint32]string)
	if len(pids) == 0 {
		return result
	}
	want := make(map[uint32]bool, len(pids))
	for _, p := range pids {
		want[p] = true
	}

	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPPROCESS),
		0,
	)
	if snapshot == INVALID_HANDLE_VALUE {
		return result
	}
	defer procCloseHandle.Call(snapshot)

	var entry PROCESSENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32FirstW.Call(
		snapshot,
		uintptr(unsafe.Pointer(&entry)),
	)
	for ret != 0 {
		if want[entry.ProcessID] {
			exeName := syscall.UTF16ToString(entry.ExeFile[:])
			result[entry.ProcessID] = toLowerWithoutExt(exeName)
		}
		ret, _, _ = procProcess32NextW.Call(
			snapshot,
			uintptr(unsafe.Pointer(&entry)),
		)
	}
	return result
}

// FindProcessesWithCWDUnder 返回工作目录位于 dir（或其子目录）内的进程 PID 列表。
// 用于识别 cmd.exe 等"站在目录里"的 CWD 占用者——这类锁 Restart Manager 不一定报得出，
// 但同样会让目录无法重命名/删除。
// 实现：读取目标进程 PEB 的 RTL_USER_PROCESS_PARAMETERS.CurrentDirectory（32 位偏移，
// 客户端固定 GOARCH=386），纯 syscall、XP 兼容；读取失败或无权限的进程直接跳过。
func FindProcessesWithCWDUnder(dir string) []uint32 {
	var result []uint32
	dirNorm := normPath(dir)

	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPPROCESS),
		0,
	)
	if snapshot == INVALID_HANDLE_VALUE {
		return result
	}
	defer procCloseHandle.Call(snapshot)

	var entry PROCESSENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32FirstW.Call(
		snapshot,
		uintptr(unsafe.Pointer(&entry)),
	)
	for ret != 0 {
		if cwd := processCWDPath(entry.ProcessID); cwd != "" && pathIsUnder(cwd, dirNorm) {
			result = append(result, entry.ProcessID)
		}
		ret, _, _ = procProcess32NextW.Call(
			snapshot,
			uintptr(unsafe.Pointer(&entry)),
		)
	}
	return result
}

// processCWDPath 读取指定进程的当前工作目录（归一化为盘符小写路径），失败返回 ""。
// 32 位客户端无法读取 64 位原生进程的 PEB（指针宽度不匹配），故直接跳过——
// 注意：这只是"读不到"，32 位进程仍可用 TerminateProcess 结束 64 位进程；
// 64 位 cmd.exe 这类 CWD 持有者对 RM 与本探测均不可见，由重试兜底与
// closeExplorerWindows 分支处理（见 common.go 中相关日志提示）。
func processCWDPath(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	if !processIs32Bit(handle) {
		return "" // 64 位原生进程：32 位客户端无法读取其 PEB
	}

	var pbi processBasicInformation
	ret, _, _ := procNtQueryInformationProcess.Call(
		handle,
		uintptr(0), // ProcessBasicInformation
		uintptr(unsafe.Pointer(&pbi)),
		uintptr(unsafe.Sizeof(pbi)),
		0,
	)
	if ret != 0 || pbi.PebBaseAddress == 0 {
		return ""
	}

	var processParams uintptr
	ret, _, _ = procNtReadVirtualMemory.Call(
		handle,
		pbi.PebBaseAddress+pebProcessParametersOffset32,
		uintptr(unsafe.Pointer(&processParams)),
		uintptr(unsafe.Sizeof(processParams)),
		0,
	)
	if ret != 0 || processParams == 0 {
		return ""
	}

	// CurrentDirectory.DosPath：UNICODE_STRING{ Length(2); MaximumLength(2); Buffer(4) }
	var us struct {
		Length        uint16
		MaximumLength uint16
		Buffer        uintptr
	}
	ret, _, _ = procNtReadVirtualMemory.Call(
		handle,
		processParams+procParamsCurDirOffset,
		uintptr(unsafe.Pointer(&us)),
		uintptr(unsafe.Sizeof(us)),
		0,
	)
	if ret != 0 || us.Buffer == 0 || us.Length == 0 || us.Length > 4096 {
		return ""
	}

	buf := make([]uint16, (us.Length+1)/2)
	var readBytes uint32
	ret, _, _ = procNtReadVirtualMemory.Call(
		handle,
		us.Buffer,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(us.Length),
		uintptr(unsafe.Pointer(&readBytes)),
	)
	if ret != 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	// NT 路径形如 \??\C:\... 或 \\?\C:\... 或 \??\UNC\server\share\...：
	// 去掉前缀并归一化为盘符 / UNC 形式
	path = strings.TrimPrefix(path, `\??\`)
	path = strings.TrimPrefix(path, `\\?\`)
	if strings.HasPrefix(path, `UNC\`) {
		// \??\UNC\server\share\app → \\server\share\app
		path = `\` + path
	}
	return normPath(path)
}

// normPath 归一化路径用于比较：Clean + 小写 + 解析 8.3 短名（EvalSymlinks），失败回退 Clean。
func normPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return strings.ToLower(filepath.Clean(resolved))
	}
	return strings.ToLower(filepath.Clean(p))
}

// pathIsUnder 判断 path 是否等于 baseDir 或其子目录（两者均已归一化）。
// baseDir 为驱动器根（如 c:\）时也能正确匹配其下所有路径。
func pathIsUnder(path, baseDir string) bool {
	if path == baseDir {
		return true
	}
	base := strings.TrimRight(baseDir, string(filepath.Separator))
	return strings.HasPrefix(path, base+string(filepath.Separator))
}

// KillProcess 终止指定 PID 的进程。
//
// 安全（最后一道闸）：内置关键进程保护——系统关键进程（csrss/winlogon/services/
// lsass 等，强杀会触发 Windows 关机倒计时）、shell（explorer，强杀黑屏）、
// PID 0/4、当前进程自身一律拒绝，即使调用方未预先过滤也不会误杀。
func KillProcess(pid uint32) error {
	killable, _ := FilterKillablePIDs([]uint32{pid})
	if len(killable) == 0 {
		return fmt.Errorf("拒绝结束受保护进程 %d", pid)
	}
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

// IsProcessAlive 判断指定 PID 的进程是否存活。
// 用 OpenProcess(PROCESS_QUERY_INFORMATION) 探测：
//   - ERROR_INVALID_PARAMETER (87)：进程不存在 → false
//   - ERROR_ACCESS_DENIED (5)：进程存在但权限不足 → true（保守）
//   - 其他失败：保守返回 true
func IsProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}
	handle, _, callErr := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_INFORMATION),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 87 {
			return false
		}
		return true // 权限不足或其他：保守认为存活
	}
	procCloseHandle.Call(handle)
	return true
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

	return KillPIDsAndWait(allPIDs, timeout)
}

// KillPIDsAndWait 等待指定 PID 列表的进程退出，超时后强杀。
//
// 安全：入口即用 FilterKillablePIDs 剔除系统关键进程与 shell——即使调用方
// （如误配的 must_close_process_name）传入关键进程，也绝不结束它们。
func KillPIDsAndWait(pids []uint32, timeout time.Duration) error {
	if len(pids) == 0 {
		return nil
	}
	killable, blocked := FilterKillablePIDs(pids)
	if len(blocked) > 0 {
		fmt.Fprintf(os.Stderr, "KillPIDsAndWait: skip protected pids %v\n", blocked)
	}
	pids = killable
	if len(pids) == 0 {
		return nil
	}

	// 先等待进程自行退出（给优雅关闭的时间）
	dead := make(map[uint32]bool)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allDead := true
		for _, pid := range pids {
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
	for _, pid := range pids {
		if !dead[pid] {
			if err := KillProcess(pid); err != nil {
				fmt.Fprintf(os.Stderr, "KillProcess %d failed: %v\n", pid, err)
			}
		}
	}

	// 再次等待残留进程退出（给 TerminateProcess 生效时间）
	for _, pid := range pids {
		if !dead[pid] {
			WaitForProcessExit(pid, 2*time.Second)
		}
	}

	return nil
}

// criticalProcessNames 绝不能强制结束、也不能发送关闭消息的系统关键进程
// （小写、去 .exe）。
//
// 安全红线：强杀或向这些进程发送 WM_CLOSE 会立刻破坏系统——
//   - csrss / winlogon / smss / wininit / services / lsass：Windows 判定关键进程终止，
//     立即弹出"系统将在 60 秒内关机"的关机倒计时（或直接蓝屏）；
//     winlogon 收到 WM_CLOSE 还可能直接触发注销/关机流程；
//   - svchost / fontdrvhost / dwm 等承载系统服务与桌面合成；
//   - 安全/索引/打印等系统服务进程。
//
// 探测（尤其深扫的全系统句柄枚举）可能把持有目录句柄的系统进程误判为"占用者"，
// 因此必须在真正 TerminateProcess / SendMessage 之前做白名单过滤。
var criticalProcessNames = map[string]bool{
	// 系统内核/关键进程
	"system": true, "registry": true, "idle": true, "secure system": true,
	"smss": true, "csrss": true, "wininit": true, "winlogon": true,
	"services": true, "lsass": true, "lsaiso": true,
	// 系统服务宿主与桌面/会话组件
	"svchost": true, "fontdrvhost": true, "dwm": true, "sihost": true,
	"ctfmon": true, "taskhostw": true, "runtimebroker": true, "shellexperiencehost": true,
	"startmenuexperiencehost": true, "searchhost": true, "textinputhost": true,
	// 安全/索引/打印等系统服务
	"searchindexer": true, "spoolsv": true, "audiodg": true, "msmpeng": true,
	"securityhealthservice": true, "windefend": true, "nissrv": true,
}

// shellProcessNames 允许发送 WM_CLOSE（关闭其文件窗口以释放目录占用，见 #24），
// 但**绝不允许强制结束**——强杀 explorer 会导致桌面/任务栏黑屏。
var shellProcessNames = map[string]bool{
	"explorer": true,
}

// FilterKillablePIDs 从待杀 PID 中剔除**绝对不可杀**的进程，返回（可杀列表, 被保护列表）。
// 保护规则：
//   - PID 0（Idle）/ PID 4（System）/ 当前进程自身；
//   - 系统关键进程（criticalProcessNames）与 shell（shellProcessNames）；
//   - 进程名无法识别（查询失败）时保守保护，宁可不杀也不误杀。
func FilterKillablePIDs(pids []uint32) (killable []uint32, blocked []uint32) {
	if len(pids) == 0 {
		return nil, nil
	}
	selfPid := uint32(os.Getpid())
	names := FindProcessNamesByPIDs(pids)
	seen := make(map[uint32]bool)
	for _, pid := range pids {
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if pid == 0 || pid == 4 || pid == selfPid {
			blocked = append(blocked, pid)
			continue
		}
		name, known := names[pid]
		if !known || name == "" {
			// 进程名无法识别（可能已退出，也可能是受保护/系统进程）：保守保护
			blocked = append(blocked, pid)
			continue
		}
		if criticalProcessNames[name] || shellProcessNames[name] {
			blocked = append(blocked, pid)
			continue
		}
		killable = append(killable, pid)
	}
	return killable, blocked
}

// isCriticalProcess 判断 PID 是否属于"不可发关闭消息"的系统关键进程
// （名字无法识别时保守视为关键进程）。
func isCriticalProcess(pid uint32) bool {
	if pid == 0 || pid == 4 {
		return true
	}
	names := FindProcessNamesByPIDs([]uint32{pid})
	name, known := names[pid]
	if !known || name == "" {
		return true // 无法识别：保守不发
	}
	return criticalProcessNames[name]
}

// ForceKillPIDs 直接强制结束指定 PID 列表的进程（不等待优雅关闭），
// 用于更新替换目录前清理占用进程。TerminateProcess 是异步的，结束后会
// 等待进程退出（最长 waitTimeout），确保重命名前句柄已被释放。
//
// 安全：调用 FilterKillablePIDs 剔除系统关键进程（强杀会触发 Windows 关机倒计时）
// 与 shell（explorer，强杀导致黑屏），被保护的 PID 仅记录到 stderr。
func ForceKillPIDs(pids []uint32, waitTimeout time.Duration) {
	if len(pids) == 0 {
		return
	}
	killable, blocked := FilterKillablePIDs(pids)
	if len(blocked) > 0 {
		fmt.Fprintf(os.Stderr, "ForceKillPIDs: skip protected pids %v\n", blocked)
	}
	if len(killable) == 0 {
		return
	}
	for _, pid := range killable {
		if err := KillProcess(pid); err != nil {
			fmt.Fprintf(os.Stderr, "ForceKillPIDs: kill %d failed: %v\n", pid, err)
		}
	}
	deadline := time.Now().Add(waitTimeout)
	for _, pid := range killable {
		remaining := deadline.Sub(time.Now())
		if remaining <= 0 {
			break
		}
		WaitForProcessExit(pid, remaining)
	}
}

// SendCloseMessageToProcess 向指定 PID 的所有可见顶层窗口发送 WM_CLOSE 消息。
//
// 用途：优雅关闭业务主程序（must_close_process_name）——主程序窗口通常是自定义类，
// 需要向它的全部顶层窗口发送。
//
// 安全：对系统关键进程（winlogon/csrss/services 等）发送 WM_CLOSE 会触发注销/关机，
// 因此在发送前用 isCriticalProcess 过滤。
//
// ⚠ 不要用它关闭 explorer：explorer 的 shell 窗口（Shell_TrayWnd/Progman/WorkerW）
// 收到 WM_CLOSE 可能被解释为"退出 shell"，触发关机/注销提示；关闭 explorer 文件窗口
// 请用 SendCloseMessageToExplorer（仅 CabinetWClass/ExploreWClass）。
func SendCloseMessageToProcess(pid uint32) {
	if isCriticalProcess(pid) {
		fmt.Fprintf(os.Stderr, "SendCloseMessageToProcess: skip critical pid %d\n", pid)
		return
	}
	sendCloseToWindows(pid, func(hwnd syscall.Handle) bool { return true })
}

// explorerFileWindowClasses explorer 的**文件浏览窗口**类名——只对这些窗口发 WM_CLOSE
// 是安全的（关闭文件夹窗口）。刻意排除 shell 窗口类（Shell_TrayWnd / Progman /
// WorkerW / TaskManagerWindow 等）：向它们发送 WM_CLOSE 可能触发 Windows 的
// 关机/注销流程提示（用户实测弹出手机关机选项的可能原因之一）。
var explorerFileWindowClasses = map[string]bool{
	"CabinetWClass": true, // Vista+ 资源管理器文件窗口
	"ExploreWClass": true, // Windows XP 文件窗口
}

// isExplorerFileWindow 判断窗口是否为 explorer 的文件浏览窗口（按窗口类名）。
func isExplorerFileWindow(hwnd syscall.Handle) bool {
	var buf [256]uint16
	n, _, _ := procGetClassNameW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if n == 0 {
		return false
	}
	return explorerFileWindowClasses[syscall.UTF16ToString(buf[:n])]
}

// SendCloseMessageToExplorer 关闭 explorer 的**文件浏览窗口**（仅 CabinetWClass/ExploreWClass），
// 用于释放"打开着目标文件夹"的资源管理器窗口对目录的占用。
//
// 安全：绝不向 shell 桌面/任务栏窗口发送 WM_CLOSE——那可能触发关机/注销提示；
// 也因此这里不会关闭用户的其他类型的窗口。
func SendCloseMessageToExplorer(pid uint32) {
	if isCriticalProcess(pid) {
		return
	}
	sendCloseToWindows(pid, isExplorerFileWindow)
}

// sendCloseToWindows 枚举顶层窗口，对指定 PID 的、可见且 filter 通过的窗口发送 WM_CLOSE。
func sendCloseToWindows(pid uint32, filter func(hwnd syscall.Handle) bool) {
	pidPtr := pid
	procEnumWindows.Call(syscall.NewCallback(func(hwnd syscall.Handle, lParam uintptr) uintptr {
		var windowPid uint32
		procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&windowPid)))
		if windowPid == pidPtr {
			visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
			if visible != 0 && filter(hwnd) {
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
