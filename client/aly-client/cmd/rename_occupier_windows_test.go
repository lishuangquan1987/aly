// +build windows

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"aly/client/aly-client/util"
)

// 重命名真实占用场景集成测试（Windows 专用）：
// 验证乐观重命名能在以下三种占用下"探测到占用者 → 杀掉 → 重命名成功"：
//   1. 文件的子文件夹被 explorer 打开（Explorer 持目录句柄，RM/CWD 探不到，
//      靠深扫/closeExplorerWindows 兜底）；
//   2. 文件夹里面的文件被另一进程独占打开（RM 应能探测到文件句柄持有者）；
//   3. cmd 占用着文件夹（CWD 持有者，FindProcessesWithCWDUnder 应能探测到）。
// 测试在真实 Windows 环境执行，依赖 explorer/子进程可用；无桌面会话时跳过。

// startExplorerFor 用 explorer 打开指定文件夹窗口，返回是否成功启动。
func startExplorerFor(t *testing.T, folder string) bool {
	t.Helper()
	cmd := exec.Command("explorer.exe", folder)
	if err := cmd.Start(); err != nil {
		t.Logf("explorer 不可用，跳过: %v", err)
		return false
	}
	// 等待窗口创建并持有句柄
	time.Sleep(3 * time.Second)
	return true
}

// startFileLock 启动子进程独占打开指定文件（FileShare.None，60 秒后自动释放）。
func startFileLock(t *testing.T, file string) *exec.Cmd {
	t.Helper()
	ps := fmt.Sprintf(
		"$s=[System.IO.File]::Open('%s',[System.IO.FileMode]::Open,[System.IO.FileAccess]::ReadWrite,[System.IO.FileShare]::None); Start-Sleep -Seconds 60",
		file)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 powershell 保持文件句柄失败: %v", err)
	}
	// 等待句柄真正打开
	time.Sleep(2 * time.Second)
	return cmd
}

// startCmdCwd 启动 cmd.exe 使其工作目录停留在 folder（CWD 占用者，挂起 60 秒）。
// 通过 cmd.Dir 直接设定工作目录（比 /c "cd /d ..." 拼接引号更可靠，避免 cmd 引号解析
// 破坏导致 cd 未生效）；使用 SysWOW64(32 位) cmd 以兼容 32 位客户端的 PEB 读取，
// 不存在时回退系统 cmd。
func startCmdCwd(t *testing.T, folder string) *exec.Cmd {
	t.Helper()
	sysRoot := os.Getenv("SystemRoot")
	sysWOW := filepath.Join(sysRoot, "SysWOW64", "cmd.exe")
	exe := "cmd.exe"
	if _, err := os.Stat(sysWOW); err == nil {
		exe = sysWOW
	}
	// 注意：cmd /c 的引号解析会破坏 "cd /d \"dir\"" 嵌套引号（cd 静默失败），
	// 因此用 Dir 字段直接设定工作目录，保证 CWD 真实落在 folder。
	cmd := exec.Command(exe, "/c", "ping -n 60 127.0.0.1 >nul")
	cmd.Dir = folder
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 cmd 占用 CWD 失败: %v", err)
	}
	time.Sleep(1 * time.Second)
	return cmd
}

// killAndWait 强制结束辅助进程并等待其退出。
func killAndWait(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// TestRenameDirWithKillExplorerHoldsSubfolder 场景 1：
// 子文件夹被 explorer 打开，父目录重命名应被占用拦截，最终被探测/清理后成功。
func TestRenameDirWithKillExplorerHoldsSubfolder(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-exp-sub")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_2.0")
	sub := filepath.Join(from, "config")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("创建子文件夹失败: %v", err)
	}
	mustMkdirFile(t, from, "app.exe", "app")
	mustMkdirFile(t, sub, "a.ini", "ini")

	if !startExplorerFor(t, sub) {
		t.Skip("explorer 不可用，跳过场景 1")
	}
	defer func() {
		// 清理：精准关闭浏览子文件夹的窗口，避免误杀全部 explorer
		_, _ = util.CloseExplorerWindowsBrowsing(sub)
	}()

	// 预检：explorer 打开子文件夹后，直接 os.Rename 应失败（占用确实存在）
	if err := os.Rename(from, to); err == nil {
		t.Skip("explorer 未实际占用目录（无桌面会话），跳过场景 1")
	} else {
		// 挪回去（保持后续路径有效）
		_ = os.Rename(to, from)
	}

	// 乐观重命名：应探测到占用者（深扫 explorer 或 closeExplorerWindows 兜底）并成功
	start := time.Now()
	if err := renameDirWithKill(from, to, 30*time.Second); err != nil {
		t.Fatalf("explorer 占用子文件夹时 renameDirWithKill 应成功，实际失败: %v (耗时 %v)", err, time.Since(start))
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含 app.exe: %v", to, err)
	}
	t.Logf("场景 1 通过：explorer 占用子文件夹被清理，rename 成功（耗时 %v）", time.Since(start))
}

// TestRenameDirWithKillFileLockedByProcess 场景 2：
// 文件夹内的文件被另一进程独占打开（FileShare.None），RM 应探测到持有者并杀掉。
func TestRenameDirWithKillFileLockedByProcess(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-lock-file")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_3.0")
	mustMkdirFile(t, from, "app.exe", "app")
	locked := filepath.Join(from, "data.bin")
	if err := ioutil.WriteFile(locked, []byte("locked"), 0644); err != nil {
		t.Fatalf("写被锁文件失败: %v", err)
	}

	locker := startFileLock(t, locked)
	defer killAndWait(t, locker)

	// 预检：文件被独占时直接 os.Rename 应失败
	if err := os.Rename(from, to); err == nil {
		_ = os.Rename(to, from)
		t.Fatal("预检失败：文件被独占打开但 os.Rename 竟然成功，说明句柄未生效")
	} else {
		_ = os.Rename(to, from)
	}

	start := time.Now()
	if err := renameDirWithKill(from, to, 30*time.Second); err != nil {
		t.Fatalf("文件被独占打开时 renameDirWithKill 应成功，实际失败: %v (耗时 %v)", err, time.Since(start))
	}
	if _, err := os.Stat(filepath.Join(to, "data.bin")); err != nil {
		t.Errorf("目标 %s 应包含 data.bin: %v", to, err)
	}
	// 持有者进程应已被杀（TerminateProcess）
	wait := make(chan struct{})
	go func() { locker.Wait(); close(wait) }()
	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		t.Errorf("占用文件的进程应已被击杀，实际仍在运行")
	}
	t.Logf("场景 2 通过：独占文件进程被探测并击杀，rename 成功（耗时 %v）", time.Since(start))
}

// TestRenameDirWithKillCmdCwdOccupiesFolder 场景 3：
// cmd 的工作目录停留在该文件夹（CWD 占用者），应被探测到并杀掉。
func TestRenameDirWithKillCmdCwdOccupiesFolder(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-cmd-cwd")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_4.0")
	mustMkdirFile(t, from, "app.exe", "app")

	occupier := startCmdCwd(t, from)
	defer killAndWait(t, occupier)

	// 预检：cmd 停在目录里时直接 os.Rename 应失败
	if err := os.Rename(from, to); err == nil {
		_ = os.Rename(to, from)
		t.Fatal("预检失败：cmd CWD 占用目录但 os.Rename 竟然成功")
	} else {
		_ = os.Rename(to, from)
	}

	start := time.Now()
	if err := renameDirWithKill(from, to, 30*time.Second); err != nil {
		t.Fatalf("cmd 占用目录时 renameDirWithKill 应成功，实际失败: %v (耗时 %v)", err, time.Since(start))
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含 app.exe: %v", to, err)
	}
	// 占用者进程应已被杀
	wait := make(chan struct{})
	go func() { occupier.Wait(); close(wait) }()
	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		t.Errorf("占用 CWD 的 cmd 进程应已被击杀，实际仍在运行")
	}
	t.Logf("场景 3 通过：cmd CWD 占用者被探测并击杀，rename 成功（耗时 %v）", time.Since(start))
}
