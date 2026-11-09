// +build !windows

package util

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestUnixFindKillAndWait 验证 Linux 下按名称找到 sleep、强杀并等待退出。
func TestUnixFindKillAndWait(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := uint32(cmd.Process.Pid)
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	pids, err := FindProcessesByName("sleep")
	if err != nil {
		t.Fatalf("FindProcessesByName: %v", err)
	}
	found := false
	for _, p := range pids {
		if p == pid {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sleep pid %d not found in %v", pid, pids)
	}

	if err := KillPIDsAndWait([]uint32{pid}, 3*time.Second); err != nil {
		t.Fatalf("KillPIDsAndWait: %v", err)
	}
	// 本用例中的 sleep 是测试进程的子进程：SIGKILL 后若未 Wait，会以 zombie
	// 形式继续出现在 kill(pid,0) 中。先回收，再判断“真正消失”。
	if err := cmd.Wait(); err != nil {
		// 被 SIGKILL 的进程 Wait 返回 ExitError，属预期
	}
	if IsProcessAlive(pid) {
		t.Fatalf("pid %d should be dead", pid)
	}
	if !WaitForProcessExit(pid, time.Second) {
		t.Fatalf("WaitForProcessExit should report gone")
	}
}

// TestUnixFindProcessesWithCWDUnder 验证按 /proc/<pid>/cwd 找到进程。
func TestUnixFindProcessesWithCWDUnder(t *testing.T) {
	dir, err := ioutil.TempDir("", "cwdscan")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	cmd := exec.Command("sleep", "30")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := uint32(cmd.Process.Pid)
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	pids := FindProcessesWithCWDUnder(dir)
	found := false
	for _, p := range pids {
		if p == pid {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("pid %d (cwd=%s) not found in %v", pid, dir, pids)
	}
}

// TestUnixFilterKillableProtectsSelfAndInit 自身/父进程/PID 1 必须被保护。
func TestUnixFilterKillableProtectsSelfAndInit(t *testing.T) {
	self := uint32(os.Getpid())
	parent := uint32(os.Getppid())
	killable, blocked := FilterKillablePIDs([]uint32{1, self, parent, 999999})
	for _, p := range []uint32{1, self, parent} {
		found := false
		for _, b := range blocked {
			if b == p {
				found = true
			}
		}
		if !found {
			t.Fatalf("pid %d should be blocked; blocked=%v", p, blocked)
		}
	}
	if len(killable) != 1 || killable[0] != 999999 {
		t.Fatalf("unexpected killable=%v blocked=%v", killable, blocked)
	}
}
