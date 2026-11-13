package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/lishuangquan1987/aly/client/aly-client-ui/aly-client-linuxui/client"
)

var spawnFinishFn = spawnFinish

// updateUpdater 用 MainFolder/aly-client 同步 UpdateFolder/aly-client。
// 正常走 self_update；更新器缺失/旧版（进程级失败）才直接复制兜底；
// 锁占用（JSON isSuccess=false）不兜底。
func updateUpdater(l layout) error {
	src := filepath.Join(l.main, "aly-client")
	target := l.updater
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	cleanArtifacts(target)

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return replaceFromFile(src, target)
	}
	same, err := sameFile(src, target)
	if err == nil && same {
		return nil
	}

	jp := journalPath(target)
	j := loadJournal(jp)
	cl := client.New(target)
	p, cmdErr := cl.SelfUpdate(context.Background())
	if cmdErr == nil {
		if p.Success {
			j.markSuccess()
			saveJournal(jp, j)
			return nil
		}
		j.markFailure("self_update: " + p.ErrMsg)
		saveJournal(jp, j)
		return fmt.Errorf("self_update: %s", p.ErrMsg)
	}

	// 进程级失败：旧版更新器不认识 self_update，或更新器损坏。
	if _, err := os.Stat(filepath.Join(l.update, ".aly.lock")); err == nil {
		j.markFailure(cmdErr.Error())
		saveJournal(jp, j)
		return cmdErr
	}
	if fbErr := replaceFromFile(src, target); fbErr != nil {
		j.markFailure(fmt.Sprintf("self_update failed (%v); direct fallback failed: %v", cmdErr, fbErr))
		saveJournal(jp, j)
		return fmt.Errorf("self_update failed (%v); direct fallback failed: %v", cmdErr, fbErr)
	}
	j.markSuccess()
	saveJournal(jp, j)
	return nil
}

// updateSelfUI 比对 MainFolder/aly-client-ui 与 UpdateFolder/aly-client-ui；
// 有差异时复制 .new 并派生 .new 的 -finish-self-update 收尾进程。
// 返回 true 表示调用方应立即退出。
func updateSelfUI(l layout, restartArgs []string) (bool, error) {
	src := filepath.Join(l.main, "aly-client-ui")
	target := l.ui
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return false, nil
	}
	jp := journalPath(target)
	j := loadJournal(jp)
	if j.shouldSkip(maxSelfUpdateAttempts, selfUpdateRetryWindow) {
		return false, nil
	}
	same, err := sameFile(src, target)
	if err != nil {
		j.markFailure(err.Error())
		saveJournal(jp, j)
		return false, err
	}
	if same {
		cleanArtifacts(target)
		j.markSuccess()
		saveJournal(jp, j)
		return false, nil
	}
	if j.Pending {
		j.markFailure("previous self-update did not converge")
		saveJournal(jp, j)
		if j.shouldSkip(maxSelfUpdateAttempts, selfUpdateRetryWindow) {
			return false, nil
		}
	}

	newPath := target + ".new"
	if err := copyVerify(src, newPath); err != nil {
		j.markFailure(err.Error())
		saveJournal(jp, j)
		return false, err
	}
	pid := os.Getpid()
	// 把原始宿主 PID 传给收尾链：新 UI 的父进程是收尾进程（拉起后即退出），
	// 若不显式传 PID，新 UI 的宿主跟随会误判"宿主已退出"而自杀（2026-11-12 修复）。
	// 当前 UI 自己若是被收尾进程重启的，则把收到的原始宿主继续往下传。
	hostPID := hostPidFromArgs()
	if hostPID == 0 {
		// 审查（轻微项）：os.Getppid() 在拿不到父进程时返回 -1，
		// uint32 回绕成 0xFFFFFFFF 会让新 UI 误判宿主已退出而自杀。取值守卫。
		if ppid := os.Getppid(); ppid > 0 {
			hostPID = uint32(ppid)
		}
	}
	helperArgs := []string{
		"-finish-self-update",
		"--target", target,
		"--pid", strconv.Itoa(pid),
		"--created", processStartTimeString(pid),
		"--host-pid", strconv.FormatUint(uint64(hostPID), 10),
		"--new", newPath,
		"--",
	}
	helperArgs = append(helperArgs, restartArgs...)

	j.markAttempt()
	saveJournal(jp, j)
	if err := spawnFinishFn(newPath, helperArgs); err != nil {
		j.markFailure(err.Error())
		saveJournal(jp, j)
		return false, err
	}
	return true, nil
}

// processStartTimeString 读取 /proc/<pid>/stat 的 starttime；失败返回 "0"。
func processStartTimeString(pid int) string {
	t := processStartTime(pid)
	if t == "" {
		return "0"
	}
	return t
}

// processStartTime 返回 /proc/<pid>/stat 的 starttime（字段 22）。
func processStartTime(pid int) string {
	fields := processStatFields(pid)
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}

// processGone 判断进程是否已退出（/proc 不存在，或 state=Z/X）。
func processGone(pid int) bool {
	fields := processStatFields(pid)
	if len(fields) == 0 {
		return true
	}
	return fields[0] == "Z" || fields[0] == "X" || fields[0] == "x"
}

// processStatFields 返回 /proc/<pid>/stat 最后一个 ')' 之后的字段：
// fields[0]=state(field 3)，fields[19]=starttime(field 22)。
func processStatFields(pid int) []string {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil
	}
	s := string(b)
	i := len(s) - 1
	for i >= 0 && s[i] != ')' {
		i--
	}
	if i < 0 || i+2 >= len(s) {
		return nil
	}
	return stringsFields(s[i+2:])
}

func stringsFields(s string) []string {
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' || s[i] == '\n' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}
