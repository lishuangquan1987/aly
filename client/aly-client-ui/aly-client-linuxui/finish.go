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
	if err := startNewUIFn(opt.target, opt.restart); err != nil {
		finishLog(opt.target, "start new UI failed: %v", err)
		return 1
	}
	finishLog(opt.target, "finish OK")
	return 0
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
