// +build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"aly/client/aly-client/selfupdate"
)

// 自更新收尾（隐藏模式 -finish-self-update）的等待/强杀参数。
//
// 旧 UI 在派生本进程后立即退出；本进程持有旧进程的句柄等待其终止，而不是
// 用 ping 猜一个固定延时。慢机器上等满 finishWaitTimeout 仍不退出才强杀，
// 强杀后再给 finishKillGrace 让内核完成收尾。
const (
	finishWaitTimeout = 10 * time.Second
	finishKillGrace   = 2 * time.Second

	// rename 重试：旧进程退出/强杀后句柄释放是即时的，但杀软可能短暂占用
	// 目标文件；20 × 250ms = 5s 足够，失败则回滚 .old。
	finishRenameRetries = 20
	finishRetryDelay    = 250 * time.Millisecond
)

const (
	finishProcessTerminate  = 0x0001
	finishProcessQueryInfo  = 0x0400
	finishSynchronize       = 0x00100000
	finishWaitObject0       = 0
	finishWaitTimeoutEvent  = 258
	finishErrorInvalidParam = 87 // ERROR_INVALID_PARAMETER：PID 不存在
)

// finishOptions 是 -finish-self-update 的参数。
type finishOptions struct {
	target  string
	newPath string
	pid     uint32
	created uint64 // 旧进程创建时间（FILETIME），用于防 PID 复用；0=未知
	restart []string
}

// FinishSelfUpdate 是 aly-client-ui.exe 的隐藏收尾入口（-finish-self-update）。
//
// 参数（由正在运行的旧 UI 传入）：
//
//	--target  <旧 UI 的 exe 路径>
//	--pid     <旧 UI 的 PID>
//	--created <旧 UI 的创建时间，十进制 FILETIME；0=未知>
//	--new     <已校验的 .new 路径（即本进程自身）>
//	--        <原启动参数，原样透传给新 UI>
//
// 流程：等旧进程退出（最多 10s）→ 超时强杀 → 把旧 exe 改名 .old →
// .new 改名到目标 → 启动新 UI → 退出。全程不经过 cmd.exe，避免引号、
// PATH、ping 延时与 move 不重试的问题。
func FinishSelfUpdate(args []string) int {
	opt, err := parseFinishArgs(args)
	if err != nil {
		// 参数错误时没有可信的 target 可写日志；GUI 无 stderr，只能返回码。
		return 2
	}

	finishLog(opt.target, "finish start: old_pid=%d created=%d new=%s",
		opt.pid, opt.created, opt.newPath)

	if opt.pid > 0 {
		if !waitForOldProcess(opt.pid, opt.created, finishWaitTimeout, finishKillGrace, opt.target) {
			// 无法确认旧进程已退出：宁可不换，也不要在旧进程仍持锁时
			// 改文件并拉起一个会撞单实例互斥体的新 UI。旧 UI 若还活着，
			// 用户仍可用；旧 UI 若已退出，下一次宿主启动会重试。
			finishLog(opt.target, "finish aborted: old process state unknown")
			return 1
		}
	}

	if err := replaceWithBackup(opt.newPath, opt.target); err != nil {
		finishLog(opt.target, "finish failed: %v", err)
		return 1
	}
	if err := startNewUIFn(opt.target, opt.restart); err != nil {
		finishLog(opt.target, "start new UI failed: %v", err)
		return 1
	}
	finishLog(opt.target, "finish OK")
	return 0
}

// parseFinishArgs 手工解析（不能用 flag 包：-- 之后是原样透传的 UI 参数）。
func parseFinishArgs(args []string) (finishOptions, error) {
	var opt finishOptions
	haveTarget, haveNew, havePID := false, false, false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--":
			opt.restart = append([]string(nil), args[i+1:]...)
			i = len(args)
			continue
		case "--target":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--target 缺少值")
			}
			opt.target = args[i]
			haveTarget = true
		case "--pid":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--pid 缺少值")
			}
			n, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil || n == 0 {
				return opt, fmt.Errorf("--pid 非法: %q", args[i])
			}
			opt.pid = uint32(n)
			havePID = true
		case "--created":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--created 缺少值")
			}
			n, err := strconv.ParseUint(args[i], 10, 64)
			if err != nil {
				return opt, fmt.Errorf("--created 非法: %q", args[i])
			}
			opt.created = n
		case "--new":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--new 缺少值")
			}
			opt.newPath = args[i]
			haveNew = true
		default:
			return opt, fmt.Errorf("未知参数: %q", args[i])
		}
		i++
	}

	if !haveTarget || !haveNew || !havePID {
		return opt, fmt.Errorf("必须同时提供 --target、--new 与 --pid")
	}
	opt.target = filepath.Clean(opt.target)
	opt.newPath = filepath.Clean(opt.newPath)
	if !filepath.IsAbs(opt.target) || !filepath.IsAbs(opt.newPath) {
		return opt, fmt.Errorf("--target/--new 必须是绝对路径")
	}
	if opt.target == opt.newPath {
		return opt, fmt.Errorf("--target 与 --new 不得相同")
	}
	if _, err := os.Stat(opt.newPath); err != nil {
		return opt, fmt.Errorf("--new 不存在: %v", err)
	}
	return opt, nil
}

// waitForOldProcess 等待旧 UI 退出；超时则 TerminateProcess。
//
// created 非 0 时先比对进程创建时间：若 PID 已被复用，直接视为旧进程已退出，
// 绝不对新进程下杀手。返回 true 表示可以继续换文件（旧进程已退出/不存在）。
func waitForOldProcess(pid uint32, created uint64, wait, grace time.Duration, target string) bool {
	h, err := syscall.OpenProcess(
		finishProcessQueryInfo|finishSynchronize|finishProcessTerminate, false, pid)
	if err != nil {
		if isFinishErrno(err, finishErrorInvalidParam) {
			finishLog(target, "old process already gone (pid=%d)", pid)
			return true
		}
		finishLog(target, "OpenProcess(pid=%d) failed: %v", pid, err)
		return false
	}
	defer syscall.CloseHandle(h)

	if created != 0 {
		var ct, et, kt, ut syscall.Filetime
		if err := syscall.GetProcessTimes(h, &ct, &et, &kt, &ut); err != nil {
			// 无法校验身份就不等/不杀：宁可 abort，也不能误杀一个可能
			// 复用了该 PID 的进程。
			finishLog(target, "GetProcessTimes(pid=%d) failed: %v; abort", pid, err)
			return false
		}
		got := uint64(ct.HighDateTime)<<32 | uint64(ct.LowDateTime)
		if got != created {
			// PID 已被复用：原来的旧 UI 已经不在了，不能等/杀这个新进程。
			finishLog(target, "pid %d was reused (created %d != expected %d); treat as gone",
				pid, got, created)
			return true
		}
	}

	event, err := syscall.WaitForSingleObject(h, finishMillis(wait))
	if err != nil {
		finishLog(target, "WaitForSingleObject(pid=%d) failed: %v", pid, err)
		return false
	}
	switch event {
	case finishWaitObject0:
		finishLog(target, "old process %d exited", pid)
		return true
	case finishWaitTimeoutEvent:
		finishLog(target, "old process %d still alive after %v; force kill", pid, wait)
		if err := syscall.TerminateProcess(h, 1); err != nil {
			finishLog(target, "TerminateProcess(pid=%d) failed: %v", pid, err)
			return false
		}
		event2, err := syscall.WaitForSingleObject(h, finishMillis(grace))
		if err == nil && event2 == finishWaitObject0 {
			finishLog(target, "old process %d force-killed", pid)
			return true
		}
		finishLog(target, "old process %d still alive after force kill (event=%d err=%v)",
			pid, event2, err)
		return false
	default:
		finishLog(target, "unexpected wait event %d (pid=%d)", event, pid)
		return false
	}
}

// replaceWithBackup 把 target 改名到 .old（保留可回滚备份），再把 newPath
// 改名到 target；失败回滚。此时旧进程已退出，target 不再是运行中的镜像。
func replaceWithBackup(newPath, target string) error {
	if _, err := os.Stat(newPath); err != nil {
		return fmt.Errorf("new file missing: %v", err)
	}

	targetExists := false
	if fi, err := os.Stat(target); err == nil {
		if fi.IsDir() {
			return fmt.Errorf("target is a directory: %s", target)
		}
		targetExists = true
	}

	oldPath := ""
	if targetExists {
		oldPath = selfupdate.NextAsidePath(target, ".old")
		if err := renameWithRetry(target, oldPath); err != nil {
			return fmt.Errorf("rename old UI to %s: %v", oldPath, err)
		}
	}

	if err := renameWithRetry(newPath, target); err != nil {
		if targetExists {
			if rbErr := renameWithRetry(oldPath, target); rbErr != nil {
				return fmt.Errorf("rename new failed (%v) AND rollback failed (%v)", err, rbErr)
			}
		}
		return fmt.Errorf("rename new UI: %v", err)
	}
	return nil
}

// renameWithRetry 重试 rename，覆盖杀软短暂占用目标文件的窗口。
func renameWithRetry(src, dst string) error {
	var lastErr error
	for i := 0; i < finishRenameRetries; i++ {
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(finishRetryDelay)
	}
	return lastErr
}

// startNewUI 启动目标 exe；不等待。参数由 exec 直接传递，不经 cmd 解析。
func startNewUI(target string, args []string) error {
	c := exec.Command(target, args...)
	if err := c.Start(); err != nil {
		return err
	}
	return nil
}

// startNewUIFn 是 startNewUI 的注入点（单测避免真的拉起 UI）。
var startNewUIFn = startNewUI

// processCreationTime 返回进程创建时间（FILETIME 合并成 uint64）；失败返回 0。
// 旧 UI 用它把"自己的身份"传给收尾进程，避免 PID 复用后误杀新进程。
func processCreationTime(pid int) uint64 {
	h, err := syscall.OpenProcess(finishProcessQueryInfo, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer syscall.CloseHandle(h)

	var ct, et, kt, ut syscall.Filetime
	if err := syscall.GetProcessTimes(h, &ct, &et, &kt, &ut); err != nil {
		return 0
	}
	return uint64(ct.HighDateTime)<<32 | uint64(ct.LowDateTime)
}

// finishLog 追加到 <target>.update.log（失败静默；日志只影响可诊断性）。
func finishLog(target string, format string, a ...interface{}) {
	f, err := os.OpenFile(target+".update.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, a...))
}

func finishMillis(d time.Duration) uint32 {
	ms := d / time.Millisecond
	if ms <= 0 {
		return 1
	}
	return uint32(ms)
}

func isFinishErrno(err error, code uintptr) bool {
	if e, ok := err.(syscall.Errno); ok {
		return uintptr(e) == code
	}
	return false
}
