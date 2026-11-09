//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func waitForOldProcess(pid int, created string, target string) bool {
	if created != "" && created != "0" {
		got := processStartTime(pid)
		if got == "" {
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); os.IsNotExist(err) {
				finishLog(target, "old process already gone (pid=%d)", pid)
				return true
			}
			finishLog(target, "cannot read /proc/%d/stat; abort", pid)
			return false
		}
		if got != created {
			finishLog(target, "pid %d reused (starttime %s != %s); treat as gone",
				pid, got, created)
			return true
		}
	}

	deadline := time.Now().Add(finishWaitTimeout)
	for {
		if processGone(pid) {
			finishLog(target, "old process %d exited", pid)
			return true
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	finishLog(target, "old process %d still alive after %v; SIGKILL", pid, finishWaitTimeout)
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		finishLog(target, "SIGKILL(pid=%d) failed: %v", pid, err)
		return false
	}
	graceDeadline := time.Now().Add(finishKillGrace)
	for {
		if processGone(pid) {
			finishLog(target, "old process %d force-killed", pid)
			return true
		}
		if time.Now().After(graceDeadline) {
			finishLog(target, "old process %d still alive after SIGKILL", pid)
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
