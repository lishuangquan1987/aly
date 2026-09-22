// +build windows

package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// ⚠ 测试安全约束：本文件只调用"纯过滤/纯判断"函数（FilterKillablePIDs / isCriticalProcess），
// 绝不对真实系统关键进程调用 ForceKillPIDs 或 SendCloseMessageToProcess——
// 若保护逻辑失效，那会真的触发 Windows 关机倒计时/注销。

// TestFilterKillablePIDsProtectsCritical 验证系统关键进程绝不进入可杀列表。
// 强杀 csrss/winlogon/services/lsass 等会触发 Windows 关机倒计时（本次事故根因）。
func TestFilterKillablePIDsProtectsCritical(t *testing.T) {
	var criticalPids []uint32
	for _, name := range []string{"csrss", "winlogon", "services", "lsass", "smss", "svchost", "wininit"} {
		pids, err := FindProcessesByName(name)
		if err != nil || len(pids) == 0 {
			continue
		}
		criticalPids = append(criticalPids, pids[0])
	}
	if len(criticalPids) == 0 {
		t.Skip("未枚举到关键系统进程（环境/权限限制），跳过")
	}
	killable, blocked := FilterKillablePIDs(criticalPids)
	if len(killable) != 0 {
		t.Errorf("关键系统进程不得进入可杀列表，实际 killable=%v (blocked=%v)", killable, blocked)
	}
	if len(blocked) != len(criticalPids) {
		t.Errorf("全部关键进程都应被保护: want %d blocked, got %v", len(criticalPids), blocked)
	}
}

// TestFilterKillablePIDsProtectsSelfAndSystemPids 验证 PID 0/4 与当前进程自身被保护。
func TestFilterKillablePIDsProtectsSelfAndSystemPids(t *testing.T) {
	self := uint32(os.Getpid())
	killable, blocked := FilterKillablePIDs([]uint32{0, 4, self})
	if len(killable) != 0 {
		t.Errorf("0/4/自身 不应可杀，实际 killable=%v", killable)
	}
	if len(blocked) != 3 {
		t.Errorf("应保护 3 个 PID，实际 blocked=%v", blocked)
	}
}

// TestFilterKillablePIDsProtectsExplorer 验证 shell（explorer）被保护：
// 强杀 explorer 会黑屏，占用应改用 WM_CLOSE 释放（#24）。
func TestFilterKillablePIDsProtectsExplorer(t *testing.T) {
	pids, err := FindProcessesByName("explorer")
	if err != nil || len(pids) == 0 {
		t.Skip("explorer 未运行，跳过")
	}
	killable, _ := FilterKillablePIDs(pids)
	if len(killable) != 0 {
		t.Errorf("explorer 不得进入可杀列表（黑屏风险），实际 killable=%v", killable)
	}
}

// TestFilterKillablePIDsAllowsNormalProcess 验证保护不过度：普通进程仍可被杀。
func TestFilterKillablePIDsAllowsNormalProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >nul")
	if err := cmd.Start(); err != nil {
		t.Skipf("无法启动 cmd 测试进程: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	time.Sleep(500 * time.Millisecond)
	pid := uint32(cmd.Process.Pid)
	killable, blocked := FilterKillablePIDs([]uint32{pid})
	if len(killable) != 1 || killable[0] != pid {
		t.Errorf("普通 cmd 进程应可杀，实际 killable=%v blocked=%v", killable, blocked)
	}
}

// TestIsCriticalProcess 验证"不可发关闭消息"的判断（WM_CLOSE 给 winlogon 会触发注销/关机）。
func TestIsCriticalProcess(t *testing.T) {
	if !isCriticalProcess(0) {
		t.Error("PID 0 应视为关键")
	}
	if !isCriticalProcess(4) {
		t.Error("PID 4 (System) 应视为关键")
	}
	// 不存在的 PID：名字未知，保守视为关键（宁可不发）
	if !isCriticalProcess(999999) {
		t.Error("无法识别的 PID 应保守视为关键")
	}
	// 真实关键进程
	for _, name := range []string{"csrss", "winlogon", "services"} {
		pids, err := FindProcessesByName(name)
		if err != nil || len(pids) == 0 {
			continue
		}
		if !isCriticalProcess(pids[0]) {
			t.Errorf("%s (pid=%d) 应视为关键", name, pids[0])
		}
	}
	// explorer 不是"关键进程"（允许发 WM_CLOSE 释放目录占用，#24）
	pids, err := FindProcessesByName("explorer")
	if err == nil && len(pids) > 0 {
		if isCriticalProcess(pids[0]) {
			t.Error("explorer 不应被视为关键进程（#24 需要向它发 WM_CLOSE 关窗口）")
		}
	}
}

// TestExplorerFileWindowClassesExcludesShellWindows 回归防护：
// 可发送 WM_CLOSE 的 explorer 窗口类**只允许文件浏览窗口**，
// 绝不能包含 shell 桌面/任务栏窗口——向它们发 WM_CLOSE 可能触发
// Windows 的关机/注销流程提示（用户实测弹出手机关机选项的原因之一）。
func TestExplorerFileWindowClassesExcludesShellWindows(t *testing.T) {
	// 必须排除的 shell 窗口类
	for _, c := range []string{"Shell_TrayWnd", "Shell_SecondaryTrayWnd", "Progman", "WorkerW", "TaskManagerWindow", "SysShadow"} {
		if explorerFileWindowClasses[c] {
			t.Errorf("shell 窗口类 %q 绝不能列入可关闭名单（会触发关机/注销提示）", c)
		}
	}
	// 必须包含的文件浏览窗口类
	for _, c := range []string{"CabinetWClass", "ExploreWClass"} {
		if !explorerFileWindowClasses[c] {
			t.Errorf("文件浏览窗口类 %q 应列入可关闭名单", c)
		}
	}
	// SendCloseMessageToExplorer 对无效 PID 不应 panic
	SendCloseMessageToExplorer(0)
	SendCloseMessageToProcess(0)
}

// TestIsProcessAliveForRunningAndInvalidPIDs 基础语义：
// 当前进程存活、PID 0 与不存在的 PID 判定为已死。
func TestIsProcessAliveForRunningAndInvalidPIDs(t *testing.T) {
	if !IsProcessAlive(uint32(os.Getpid())) {
		t.Error("当前进程应判定为存活")
	}
	if IsProcessAlive(0) {
		t.Error("PID 0 应判定为不存活")
	}
	if IsProcessAlive(0x7FFFFFF0) {
		t.Error("不存在的 PID 应判定为不存活")
	}
}

// TestIsProcessAliveFalseForKilledButUnreapedProcess 回归防护（关键）：
// 子进程被强杀后，只要父进程仍持有 Process 句柄，其内核对象就不会被回收，
// OpenProcess(该 PID) 依旧成功、PID 依旧有效——仅凭 OpenProcess 会把
// "已被强杀的进程"误判为存活。
//
// 后果：.aly.lock 陈旧锁无法回收（cleanStaleLock 认为持有者还活着），
// 更新进程被强杀后立刻重试更新会报"另一更新正在进行中"并阻塞到 30 分钟 TTL。
// 因此必须再查 GetExitCodeProcess：已结束的进程返回真实退出码，而非 STILL_ACTIVE。
func TestIsProcessAliveFalseForKilledButUnreapedProcess(t *testing.T) {
	cmdExe := "cmd.exe"
	if osIs64Bit() {
		cmdExe = filepath.Join(os.Getenv("SystemRoot"), "SysWOW64", "cmd.exe")
		if _, err := os.Stat(cmdExe); err != nil {
			cmdExe = "cmd.exe"
		}
	}
	cmd := exec.Command(cmdExe, "/c", "ping -n 30 127.0.0.1 >nul")
	if err := cmd.Start(); err != nil {
		t.Skipf("启动子进程失败（跳过）: %v", err)
	}
	pid := uint32(cmd.Process.Pid)

	if !IsProcessAlive(pid) {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("刚启动的子进程 (pid=%d) 应判定为存活", pid)
	}

	// 强杀但**不**调用 Wait：句柄未释放、进程对象未回收，模拟"被强杀后立刻重试"现场
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("强杀子进程失败: %v", err)
	}

	// 轮询等待判定转为"已死"（TerminateProcess 是异步的）
	deadline := time.Now().Add(5 * time.Second)
	for IsProcessAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if IsProcessAlive(pid) {
		t.Errorf("已被强杀的进程 (pid=%d) 必须判定为已死，否则陈旧锁无法回收", pid)
	}

	cmd.Wait() // 回收进程对象
}
