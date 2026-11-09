// +build !windows

package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// procPIDs 枚举 /proc 下的数字 PID（Linux）。
func procPIDs() []uint32 {
	entries, err := ioutil.ReadDir("/proc")
	if err != nil {
		return nil
	}
	pids := make([]uint32, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil || n == 0 {
			continue
		}
		pids = append(pids, uint32(n))
	}
	return pids
}

// readProcComm 读取 /proc/<pid>/comm；失败返回空串。
func readProcComm(pid uint32) string {
	b, err := ioutil.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// readProcCmdline 读取 /proc/<pid>/cmdline 的第一个参数（可补足 comm 的 15 字符截断）。
func readProcCmdline(pid uint32) string {
	b, err := ioutil.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "cmdline"))
	if err != nil || len(b) == 0 {
		return ""
	}
	first := b
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		first = b[:i]
	}
	return filepath.Base(string(first))
}

// FindProcessesByName 查找名称匹配的进程；Windows 语义大小写不敏感，这里沿用。
// 同时匹配 comm（Linux 截断到 15 字符）与 cmdline 的第一个参数。
func FindProcessesByName(name string) ([]uint32, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	want := strings.TrimSuffix(name, ".exe")
	var out []uint32
	for _, pid := range procPIDs() {
		comm := readProcComm(pid)
		arg0 := readProcCmdline(pid)
		if strings.EqualFold(comm, want) || strings.EqualFold(arg0, want) ||
			strings.EqualFold(strings.TrimSuffix(comm, ".exe"), want) {
			out = append(out, pid)
		}
	}
	return out, nil
}

// KillProcess 强杀指定 PID；进程已不存在视为成功。
func KillProcess(pid uint32) error {
	if pid == 0 {
		return nil
	}
	err := syscall.Kill(int(pid), syscall.SIGKILL)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

// WaitForProcessExit 等待 PID 退出；超时返回 false。
// kill(pid,0) 的 EPERM 表示进程存在但无权发信号，仍算存活。
func WaitForProcessExit(pid uint32, timeout time.Duration) bool {
	if pid == 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(int(pid), 0)
		if err == syscall.ESRCH {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// KillProcessesAndWait 强杀名称匹配的进程并等待退出（带保护过滤）。
func KillProcessesAndWait(names []string, timeout time.Duration) error {
	var all []uint32
	for _, name := range names {
		pids, err := FindProcessesByName(name)
		if err != nil {
			return err
		}
		killable, _ := FilterKillablePIDs(pids)
		for _, pid := range killable {
			if err := KillProcess(pid); err != nil {
				return err
			}
			all = append(all, pid)
		}
	}
	return waitPIDsGone(all, timeout)
}

// KillPIDsAndWait 强杀指定 PID 列表并等待退出（带保护过滤）。
func KillPIDsAndWait(pids []uint32, timeout time.Duration) error {
	killable, _ := FilterKillablePIDs(pids)
	for _, pid := range killable {
		if err := KillProcess(pid); err != nil {
			return err
		}
	}
	return waitPIDsGone(killable, timeout)
}

// waitPIDsGone 等待指定 PID 退出；空列表表示等待所有已杀进程（调用方
// 已 filter/kill）。
func waitPIDsGone(pids []uint32, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		alive := false
		for _, pid := range pids {
			if syscall.Kill(int(pid), 0) != syscall.ESRCH {
				alive = true
				break
			}
		}
		if !alive || time.Now().After(deadline) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ForceKillPIDs 直接强杀给定 PID（保护自身/父进程/PID 1）。
func ForceKillPIDs(pids []uint32, waitTimeout time.Duration) {
	killable, _ := FilterKillablePIDs(pids)
	for _, pid := range killable {
		_ = KillProcess(pid)
	}
	_ = waitPIDsGone(killable, waitTimeout)
}

// IsProcessAlive 判断 PID 是否存活（Unix 实现：kill -0 探测）。
func IsProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}
	err := syscall.Kill(int(pid), 0)
	return err == nil || err == syscall.EPERM
}

// SendCloseMessageToProcess 向 PID 发送 SIGTERM（Unix 的"请求关闭"）。
func SendCloseMessageToProcess(pid uint32) {
	if pid == 0 {
		return
	}
	_ = syscall.Kill(int(pid), syscall.SIGTERM)
}

// SendCloseMessageToExplorer Unix 无 explorer，空操作。
func SendCloseMessageToExplorer(pid uint32) {}

// FilterKillablePIDs 过滤自身、父进程与 PID 1；同 UID 的其他进程可杀。
func FilterKillablePIDs(pids []uint32) (killable []uint32, blocked []uint32) {
	self := uint32(os.Getpid())
	parent := uint32(os.Getppid())
	for _, pid := range pids {
		if pid == 0 || pid == 1 || pid == self || pid == parent {
			blocked = append(blocked, pid)
			continue
		}
		killable = append(killable, pid)
	}
	return killable, blocked
}

// FindProcessNamesByPIDs 返回 PID → 进程名（comm）。
func FindProcessNamesByPIDs(pids []uint32) map[uint32]string {
	out := make(map[uint32]string, len(pids))
	for _, pid := range pids {
		if name := readProcComm(pid); name != "" {
			out[pid] = name
		}
	}
	return out
}

// FindProcessesWithCWDUnder 查找 CWD 位于 dir 下（含 dir 本身）的进程。
func FindProcessesWithCWDUnder(dir string) []uint32 {
	if dir == "" {
		return nil
	}
	base := filepath.Clean(dir)
	var out []uint32
	for _, pid := range procPIDs() {
		link, err := os.Readlink(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "cwd"))
		if err != nil {
			continue
		}
		cwd := filepath.Clean(link)
		if cwd == base || strings.HasPrefix(cwd, base+string(os.PathSeparator)) {
			out = append(out, pid)
		}
	}
	return out
}

// FindProcessesWithHandlesUnder Unix 无 Win32 句柄表；Linux rename 不依赖句柄扫描。
func FindProcessesWithHandlesUnder(dir string) []uint32 { return nil }

// CloseExplorerWindowsBrowsing Unix 无 explorer，空操作。
func CloseExplorerWindowsBrowsing(folder string) (int, error) { return 0, nil }
