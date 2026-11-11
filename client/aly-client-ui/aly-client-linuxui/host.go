package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 宿主生命周期（规范见 spec/host-contract.md §15）。
//
// aly-client-ui（Linux 前端）同样是宿主拉起的子进程：宿主退出后 UI 不应该留在屏幕上，
// 默认跟随退出。例外是 apply 阶段——更新器正在关宿主、替换主目录并重启主程序，
// 这时退出会把"正在应用更新"的现场抹掉。判定用 version.json 的 version_status == applying。
//
// 与 Windows 前端一致，version.json 按工作目录（UpdateFolder/）相对路径读：
// 契约要求更新器与内置调用方都以更新器所在目录为工作目录（见 spec/host-contract.md §1）。
const hostPollInterval = 1 * time.Second

// appliesMark 是 version.json 里 applying 状态的字面量（与 aly-client 侧序列化格式一致）。
const appliesMark = `"version_status":"applying"`

// ShouldExitOnHostGone 是纯决策函数：宿主已不在时，UI 是否应该退出。
func ShouldExitOnHostGone(applying bool) bool {
	return !applying
}

// processAlive 判断宿主 PID 是否仍存活：/proc/<pid> 存在即存活。
// 读不到（无 /proc、权限不足）按"已退出"处理——宁可不跟随，也不误判存活把 UI 留着。
func processAlive(pid uint32) bool {
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(int(pid)))); err != nil {
		return false
	}
	return true
}

// applyingNow 读工作目录的 version.json，判断是否处在 applying 例外窗口。
// 读不到 / 格式不符都按 false 处理。
func applyingNow() bool {
	data, err := ioutil.ReadFile("version.json")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), appliesMark)
}

// WatchHostExit 起守护协程跟随宿主：宿主退出且不在 applying 窗口时写一行 stderr 诊断并以 0 退出。
// 宿主 PID 拿不到时不动作（宁可不跟随，也不误退）。
func WatchHostExit() {
	host := uint32(os.Getppid())
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
