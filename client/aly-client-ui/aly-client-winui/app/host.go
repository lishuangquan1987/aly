// +build windows

package app

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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
//
// 跟随机制分两档（2026-11-12 修复"关闭宿主后 UI 不退出"）：
//  1. **首选等待宿主进程句柄**：启动时按 PID 打开 SYNCHRONIZE 句柄并阻塞等待。
//     句柄精确指向打开时的那个进程——宿主退出立即触发，**不受 PID 复用影响**。
//     之前按 PID 每 1s 轮询，宿主退出后 PID 一旦被系统复用给别的进程，
//     processAlive 永远为真，UI 就一直留在屏幕上（现场报告的形态）。
//  2. 句柄打不开（宿主已先退出 / 权限不足）→ 回落 PID 轮询（fail-open：复用
//     场景宁可不退，也不误退）。
func WatchHostExit() {
	host := hostPidFromArgs()
	if host == 0 {
		host = uint32(syscall.Getppid())
	}
	if host == 0 {
		return
	}
	if h, err := syscall.OpenProcess(finishSynchronize, false, host); err == nil {
		go followHostByHandle(h)
		return
	}
	go followHostByPID(host)
}

// followHostByHandle 等待宿主句柄触发（宿主退出）。句柄已 signal 后若命中
// applying 例外，转为轮询 version.json——apply 结束（状态离开 applying）后退出。
func followHostByHandle(h syscall.Handle) {
	defer syscall.CloseHandle(h)
	for {
		event, err := syscall.WaitForSingleObject(h, finishMillis(hostPollInterval))
		if err != nil {
			// 等待失败（句柄失效等）：停止跟随，宁可不退也不误退。
			return
		}
		if event != finishWaitObject0 {
			continue // 超时：宿主仍存活
		}
		if !ShouldExitOnHostGone(applyingNow()) {
			time.Sleep(hostPollInterval)
			continue // applying 窗口：继续等（句柄已 signal，此后等效轮询状态）
		}
		fmt.Fprintf(os.Stderr, "host process exited; UI exits\n")
		os.Exit(0)
	}
}

// followHostByPID 是句柄打不开时的回落：按 PID 轮询（存在 PID 复用误判的弱点）。
func followHostByPID(host uint32) {
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
}
