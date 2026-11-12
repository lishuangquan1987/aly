package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const (
	finishWaitTimeout   = 10 * time.Second
	finishKillGrace     = 2 * time.Second
	finishRenameRetries = 20
	finishRetryDelay    = 250 * time.Millisecond
)

type finishOptions struct {
	target  string
	newPath string
	pid     int
	created string
	hostPID uint32 // 原始宿主 PID（随收尾链传递，拉起新 UI 时继续下发）；0=未知
	restart []string
}

// finishSelfUpdate 是 Linux UI 的隐藏收尾入口（-finish-self-update）。
// 必须在 MyGo/GTK 初始化之前调用；只使用 os/exec 与 /proc。
func finishSelfUpdate(args []string) int {
	opt, err := parseFinishArgs(args)
	if err != nil {
		return 2
	}
	finishLog(opt.target, "finish start: old_pid=%d created=%s new=%s",
		opt.pid, opt.created, opt.newPath)

	if opt.pid > 0 {
		if !waitForOldProcess(opt.pid, opt.created, opt.target) {
			finishLog(opt.target, "finish aborted: old process state unknown")
			return 1
		}
	}
	if err := replaceWithBackup(opt.newPath, opt.target); err != nil {
		finishLog(opt.target, "finish failed: %v", err)
		return 1
	}
	// 新 UI 的父进程是本收尾进程（拉起新 UI 后立即退出）。把原始宿主 PID
	// 下发进去，新 UI 的宿主跟随据此继续跟随真正的宿主，而不是在 1 秒后
	// 误判"宿主已退出"自杀（2026-11-12 修复）。restartArgs 里可能残留
	// 旧链路传来的 --host-pid（连续两次自更新），先剥掉再追加规范值。
	if opt.hostPID > 0 {
		opt.restart = append(stripHostPIDArgs(opt.restart),
			"--host-pid", strconv.FormatUint(uint64(opt.hostPID), 10))
	}
	if err := startNewUIFn(opt.target, opt.restart); err != nil {
		finishLog(opt.target, "start new UI failed: %v", err)
		return 1
	}
	finishLog(opt.target, "finish OK")
	return 0
}

// stripHostPIDArgs 去掉参数序列里的 --host-pid / -host-pid 及其值。
func stripHostPIDArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--host-pid" || args[i] == "-host-pid" {
			i++ // 跳过它的值
			continue
		}
		out = append(out, args[i])
	}
	return out
}

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
				return opt, fmt.Errorf("--target missing value")
			}
			opt.target = args[i]
			haveTarget = true
		case "--pid":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--pid missing value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				return opt, fmt.Errorf("bad --pid %q", args[i])
			}
			opt.pid = n
			havePID = true
		case "--created":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--created missing value")
			}
			opt.created = args[i]
		case "--host-pid":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--host-pid missing value")
			}
			n, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return opt, fmt.Errorf("bad --host-pid %q", args[i])
			}
			opt.hostPID = uint32(n)
		case "--new":
			i++
			if i >= len(args) {
				return opt, fmt.Errorf("--new missing value")
			}
			opt.newPath = args[i]
			haveNew = true
		default:
			return opt, fmt.Errorf("unknown arg %q", args[i])
		}
		i++
	}
	if !haveTarget || !haveNew || !havePID {
		return opt, fmt.Errorf("--target/--new/--pid are required")
	}
	opt.target = filepath.Clean(opt.target)
	opt.newPath = filepath.Clean(opt.newPath)
	if !filepath.IsAbs(opt.target) || !filepath.IsAbs(opt.newPath) {
		return opt, fmt.Errorf("--target/--new must be absolute")
	}
	if opt.target == opt.newPath {
		return opt, fmt.Errorf("--target and --new must differ")
	}
	if _, err := os.Stat(opt.newPath); err != nil {
		return opt, fmt.Errorf("--new missing: %v", err)
	}
	return opt, nil
}

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
		oldPath = nextAsidePath(target, ".old")
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

func startNewUI(target string, args []string) error {
	c := exec.Command(target, args...)
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	return c.Start()
}

var startNewUIFn = startNewUI

func finishLog(target string, format string, a ...interface{}) {
	f, err := os.OpenFile(target+".update.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, a...))
}
