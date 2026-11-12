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

// hostPidFromArgs 从命令行找 --host-pid（收尾进程重启链传下来的原始宿主 PID）。
//
// 为什么需要它：UI 自更新走"旧 UI 退出 → 收尾进程替换 → 拉起新 UI"的链路，
// 新 UI 的父进程是**收尾进程**，而它在拉起新 UI 后立即退出。若此时按 Getppid
// 跟随宿主，新 UI 会在启动约 1 秒后误判"宿主已退出"而自杀——auto 模式的更新
// 卡片永远弹不出来（2026-11-12 修复的现场回归）。收尾链把最初拉起旧 UI 的
// 宿主 PID 一路传下来，新 UI 据此继续跟随真正的宿主。
//
// 取最后一个匹配（重复传参时后面的生效）；缺失/非法/为 0 都返回 0（调用方回落 Getppid）。
func hostPidFromArgs() uint32 {
	host := uint32(0)
	for i := 0; i < len(os.Args)-1; i++ {
		if os.Args[i] != "--host-pid" && os.Args[i] != "-host-pid" {
			continue
		}
		if n, err := strconv.ParseUint(os.Args[i+1], 10, 32); err == nil {
			host = uint32(n)
		}
	}
	return host
}

// WatchHostExit 起守护协程跟随宿主：宿主退出且不在 applying 窗口时写一行 stderr 诊断并以 0 退出。
// 宿主 PID 优先取 --host-pid（收尾重启链传下来的原始宿主），否则取父进程；
// 都拿不到时不动作（宁可不跟随，也不误退）。
func WatchHostExit() {
	host := hostPidFromArgs()
	if host == 0 {
		host = uint32(os.Getppid())
	}
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
