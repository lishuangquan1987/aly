// +build windows

package app

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const deadPID = 2147483632 // 0x7FFFFFF0：必然 OpenProcess 失败

func TestParseFinishArgsValid(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	target := filepath.Join(dir, "aly-client-ui.exe")
	newPath := target + ".new"
	writeF(t, newPath, "NEW")

	opt, err := parseFinishArgs([]string{
		"--target", target,
		"--pid", "1234",
		"--created", "987654321",
		"--host-pid", "4242",
		"--new", newPath,
		"--", "-mode", "check",
	})
	if err != nil {
		t.Fatalf("parseFinishArgs: %v", err)
	}
	if opt.target != target || opt.newPath != newPath || opt.pid != 1234 || opt.created != 987654321 {
		t.Fatalf("bad parsed options: %+v", opt)
	}
	if opt.hostPID != 4242 {
		t.Fatalf("hostPID = %d; want 4242", opt.hostPID)
	}
	if len(opt.restart) != 2 || opt.restart[0] != "-mode" || opt.restart[1] != "check" {
		t.Fatalf("bad restart args: %#v", opt.restart)
	}
}

func TestParseFinishArgsErrors(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	target := filepath.Join(dir, "ui.exe")
	newPath := target + ".new"
	writeF(t, newPath, "NEW")

	cases := [][]string{
		{"--new", newPath, "--pid", "1"},                           // 缺 target
		{"--target", target, "--new", newPath},                     // 缺 pid
		{"--target", target, "--pid", "0", "--new", newPath},       // pid=0
		{"--target", "relative", "--pid", "1", "--new", newPath},   // 非绝对
		{"--target", target, "--pid", "1", "--new", target},        // target==new
		{"--target", target, "--pid", "1", "--new", target + ".x"}, // new 不存在
		{"--bogus", "x"},                                           // 未知参数
	}
	for _, args := range cases {
		if _, err := parseFinishArgs(args); err == nil {
			t.Fatalf("expected error for %#v", args)
		}
	}
}

func TestFinishSelfUpdateParseError(t *testing.T) {
	if code := FinishSelfUpdate([]string{"--target"}); code != 2 {
		t.Fatalf("parse error exit code = %d; want 2", code)
	}
}

// TestFinishSelfUpdateFlow 用“必然已退出的 PID”走完整收尾：改名 + .old 备份 +
// 启动（注入桩，不真的拉起 UI）+ 日志。
func TestFinishSelfUpdateFlow(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	target := filepath.Join(dir, "aly-client-ui.exe")
	newPath := target + ".new"
	writeF(t, target, "OLD")
	writeF(t, newPath, "NEW")

	var startedTarget string
	var startedArgs []string
	orig := startNewUIFn
	startNewUIFn = func(t string, args []string) error {
		startedTarget = t
		startedArgs = append([]string(nil), args...)
		return nil
	}
	defer func() { startNewUIFn = orig }()

	code := FinishSelfUpdate([]string{
		"--target", target,
		"--pid", strconv.Itoa(deadPID),
		"--created", "0",
		"--host-pid", "4242",
		"--new", newPath,
		"--", "-mode", "auto",
	})
	if code != 0 {
		t.Fatalf("FinishSelfUpdate exit code = %d; want 0", code)
	}
	if got := readF(t, target); got != "NEW" {
		t.Fatalf("target content = %q; want NEW", got)
	}
	if got := readF(t, target+".old"); got != "OLD" {
		t.Fatalf(".old content = %q; want OLD", got)
	}
	if startedTarget != target {
		t.Fatalf("started %q; want %q", startedTarget, target)
	}
	// 重启参数必须带上原始宿主 PID：新 UI 的父进程（收尾进程）马上就退出，
	// 不传的话宿主跟随会在 1 秒后误判"宿主已退出"而自杀。
	if len(startedArgs) != 4 || startedArgs[0] != "-mode" || startedArgs[1] != "auto" ||
		startedArgs[2] != "--host-pid" || startedArgs[3] != "4242" {
		t.Fatalf("restart args = %#v; want [-mode auto --host-pid 4242]", startedArgs)
	}
	logData, err := ioutil.ReadFile(target + ".update.log")
	if err != nil {
		t.Fatalf("update log missing: %v", err)
	}
	if !strings.Contains(string(logData), "finish OK") {
		t.Fatalf("log should contain finish OK, got:\n%s", logData)
	}
}

func TestReplaceWithBackupMissingTarget(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	target := filepath.Join(dir, "ui.exe")
	newPath := target + ".new"
	writeF(t, newPath, "NEW")

	if err := replaceWithBackup(newPath, target); err != nil {
		t.Fatalf("replaceWithBackup: %v", err)
	}
	if got := readF(t, target); got != "NEW" {
		t.Fatalf("target = %q", got)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf("no .old expected when target is missing (err=%v)", err)
	}
}

func TestWaitForOldProcessAlreadyGone(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()
	target := filepath.Join(dir, "ui.exe")

	if !waitForOldProcess(deadPID, 0, 100*time.Millisecond, time.Second, target) {
		t.Fatal("dead PID should be treated as already exited")
	}
}

// TestWaitForOldProcessForceKill 慢进程等超时后必须强杀，而不是无限等。
func TestWaitForOldProcessForceKill(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()
	target := filepath.Join(dir, "ui.exe")

	cmd := exec.Command("ping.exe", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	pid := uint32(cmd.Process.Pid)

	if !waitForOldProcess(pid, 0, 300*time.Millisecond, 2*time.Second, target) {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal("long-running child should be force-killed and reported exited")
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		// 已被强杀，Wait 返回（通常带 ExitError）
	case <-time.After(5 * time.Second):
		t.Fatal("child still alive after force kill")
	}

	logData, err := ioutil.ReadFile(target + ".update.log")
	if err != nil {
		t.Fatalf("update log missing: %v", err)
	}
	if !strings.Contains(string(logData), "force-killed") {
		t.Fatalf("log should mention force-killed, got:\n%s", logData)
	}
}

// TestWaitForOldProcessCreationMismatch PID 被复用（创建时间不匹配）时：
// 视为旧进程已退出，绝不能杀当前占用该 PID 的新进程。
func TestWaitForOldProcessCreationMismatch(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()
	target := filepath.Join(dir, "ui.exe")

	cmd := exec.Command("ping.exe", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	if !waitForOldProcess(uint32(cmd.Process.Pid), 1, 200*time.Millisecond, time.Second, target) {
		t.Fatal("creation-time mismatch should be treated as gone")
	}
	// 仍存活：说明 helper 没有误杀
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("process should still be alive (not killed on mismatch): %v", err)
	}
}
