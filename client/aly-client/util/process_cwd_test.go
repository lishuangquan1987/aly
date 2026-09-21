// +build windows

package util

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestFindProcessesWithCWDUnder 验证 CWD 探测：
// 启动一个工作目录位于临时目录内的 32 位 cmd.exe（模拟"站在目录里"的占用者），
// 应能被 FindProcessesWithCWDUnder 探测到；结束后不再被探测到。
// 注：32 位客户端只能读取 32 位进程的 PEB，故 64 位系统上使用 SysWOW64 的 32 位 cmd。
func TestFindProcessesWithCWDUnder(t *testing.T) {
	dir, err := ioutil.TempDir("", "cwd-holder")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	cmdExe := "cmd.exe"
	if osIs64Bit() {
		// 64 位系统：使用 WOW64 的 32 位 cmd
		cmdExe = filepath.Join(os.Getenv("SystemRoot"), "SysWOW64", "cmd.exe")
		if _, err := os.Stat(cmdExe); err != nil {
			t.Skipf("SysWOW64 cmd.exe 不可用: %v", err)
		}
	}

	// 启动 32 位 cmd，工作目录 = dir，用 ping 挂住进程（XP 兼容）
	cmd := exec.Command(cmdExe, "/c", "ping -n 30 127.0.0.1 >nul")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 cmd 失败: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
	}()

	// 等待进程进入运行态并设置 CWD
	time.Sleep(500 * time.Millisecond)

	found := FindProcessesWithCWDUnder(dir)
	matched := false
	for _, pid := range found {
		if pid == uint32(cmd.Process.Pid) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("应探测到 CWD 位于 %s 的 cmd 进程 (pid=%d)，实际: %v", dir, cmd.Process.Pid, found)
	}

	// 结束进程后不应再被探测到
	cmd.Process.Kill()
	cmd.Process.Wait()
	time.Sleep(300 * time.Millisecond)
	for _, pid := range FindProcessesWithCWDUnder(dir) {
		if pid == uint32(cmd.Process.Pid) {
			t.Fatalf("进程结束后仍被探测到 pid=%d", pid)
		}
	}
}

// TestPathIsUnder 验证路径前缀判断
func TestPathIsUnder(t *testing.T) {
	base := normPath(filepath.Join("C:\\", "app"))
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join("C:\\", "app"), true},
		{filepath.Join("C:\\", "app", "sub"), true},
		{filepath.Join("C:\\", "app2"), false},
		{filepath.Join("C:\\", "ap"), false},
	}
	for _, c := range cases {
		if got := pathIsUnder(normPath(c.path), base); got != c.want {
			t.Errorf("pathIsUnder(%q, %q) = %v, want %v", c.path, base, got, c.want)
		}
	}
}
