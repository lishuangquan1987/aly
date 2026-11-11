package app

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// 宿主生命周期（规范见 spec/host-contract.md §15）。
//
// aly-client-ui 是宿主（SDK 宿住）拉起的子进程。宿主退出后 UI 不应该留在屏幕上，默认跟随退出。
// 例外是 apply 阶段：更新器正在关闭宿主、替换主目录并重启主程序（apply 由 aly-client 的独立进程
// 负责，UI 自己触发的回滚/应用也走同一条路），这时 UI 退出会把"正在应用更新"的现场提前抹掉。
// 判定用状态级例外：version.json 的 version_status == applying。
const hostPollInterval = 1 * time.Second

// appliesMark 是 version.json 里 applying 状态的字面量（与 aly-client 侧序列化格式一致）。
const appliesMark = `"version_status":"applying"`

// ShouldExitOnHostGone 是纯决策函数：宿主已不在时，UI 是否应该退出。
func ShouldExitOnHostGone(applying bool) bool {
	return !applying
}

// applyingNow 读 version.json 判断是否处在 applying 例外窗口：
// 先按工作目录（UpdateFolder/）相对路径读，再按 UI 自身所在目录读。
// 读不到 / 格式不符都按 false 处理（状态不明时不做例外）。
func applyingNow() bool {
	for _, p := range versionPaths() {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), appliesMark) {
			return true
		}
	}
	return false
}

// versionPaths 返回 version.json 的候选路径：工作目录相对路径 + UI 自身目录。
func versionPaths() []string {
	paths := []string{"version.json"}
	if self, err := SelfDirOf(os.Args[0]); err == nil {
		paths = append(paths, filepath.Join(self, "version.json"))
	}
	return paths
}

// processAlive 判断宿主 PID 是否仍存活：打开进程后取进程时间，任一步失败都按"已退出"处理。
func processAlive(pid uint32) bool {
	h, err := syscall.OpenProcess(finishProcessQueryInfo, false, pid)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)

	var ct, et, kt, ut syscall.Filetime
	if err := syscall.GetProcessTimes(h, &ct, &et, &kt, &ut); err != nil {
		return false
	}
	return true
}

// WatchHostExit 起守护协程跟随宿主：宿主退出且不在 applying 窗口时写一行 stderr 诊断并以 0 退出。
// 宿主 PID 拿不到时不动作（宁可不跟随，也不误退）。
func WatchHostExit() {
	host := uint32(syscall.Getppid())
	if host == 0 {
		return
	}
	go func() {
		for {
			time.Sleep(hostPollInterval)
			if processAlive(host) {
				continue
			}
			if !ShouldExitOnHostGone(applyingNow()) {
				return
			}
			fmt.Fprintf(os.Stderr, "host process exited; UI exits\n")
			os.Exit(0)
		}
	}()
}
