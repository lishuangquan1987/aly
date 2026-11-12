package tc

// 用例 138：宿主退出后 UI 跟随退出（宿主跟随不失效）
//
// 背景（现场报告，2026-11-12 修复）：手动「检查更新」弹出卡片后关闭宿主程序，
// 右下角的 UI 一直留在屏幕上不退出。根因：宿主跟随按 **PID 轮询**判断宿主存活，
// 宿主退出后 PID 被系统复用给别的进程时 processAlive 永远为真 → UI 永不退出。
// 修复：winui 改为**等待宿主进程句柄**（SYNCHRONIZE，宿主退出立即触发、不受
// PID 复用影响；句柄打不开才回落 PID 轮询）；linuxui 轮询时比对
// /proc/<pid>/stat 的 starttime 校验进程身份。
//
// 覆盖：
//	138  宿主退出 → UI 跟随退出（apply 例外不在此测：apply 窗口宿主本来就会
//	     被 updater 关闭，属崩溃恢复口径，见 spec/host-contract.md §15）
//
// 观测设计（时序是关键）：
//   - UI 的宿主必须是"会退出的独立进程"——用 PowerShell 代拟：Start-Process
//     拉起 UI（父子关系成立）→ 存活 8s → 退出，即模拟"宿主退出"；
//   - **宿主存活期间**先确认 UI 已出现（UI 被正确跟随的话，宿主一退出它就会
//     消失，宿主退出后才开始观测永远看不到活着的 UI）；
//   - 宿主退出后再确认 UI 消失。

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCase138(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-host-follow"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	// 服务端放一个未消费的更新：-mode check 检查到它就展示卡片、常驻等待
	// 用户操作——保证"UI 存活"不是靠模态框阻塞，而是和现场一致的卡片形态。
	e.PutFile(t, proj, "app.exe", "app-110")
	e.Publish(t, proj, "1.1.0", []string{"case 138 follow"}, "", "")

	uiExe := e.BuildUI(t)
	uiSelf := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiSelf)

	// 代拟宿主：PowerShell 拉起 UI（父子关系成立）→ 存活 8s → 退出。
	// UI 的工作目录必须是 UpdateFolder（与 SDK 的启动口径一致：找得到
	// aly-client.exe / version.json / client.json）。
	hostScript := "Start-Process -FilePath '" + uiSelf + "' -ArgumentList '-mode','check' -WorkingDirectory '" + c.Up() + "'; Start-Sleep -Seconds 8"
	host := exec.Command("powershell", "-NoProfile", "-Command", hostScript)
	if err := host.Start(); err != nil {
		t.Fatalf("代拟宿主启动失败: %v", err)
	}

	// ① 宿主存活期间：UI 必须已经起来（卡片/检查窗口常驻）。
	//    宿主 8s 后才退出，这个窗口足够观测。
	True(t, waitUIProcess(8*time.Second), "① 宿主存活期间 UI 进程应出现")

	// ② 等宿主退出（PowerShell 睡满 8s 后结束）。
	if err := host.Wait(); err != nil {
		t.Fatalf("代拟宿主异常退出: %v", err)
	}

	// ③ 核心：宿主退出后 UI 必须跟随退出。
	//    句柄等待在宿主退出的瞬间触发；留 8s 余量覆盖慢机器与杀软扫描。
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !uiProcessRunning() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	True(t, !uiProcessRunning(),
		"③ 宿主退出后 UI 必须跟随退出（PID 复用不得把'宿主已退'误判成存活）")
	defer killUIProcesses()

	// ④ 复核 UI 的退出是"跟随宿主"而不是"自己检查完就退"：
	//    服务端有未消费更新，正常路径 UI 会带卡片常驻——它退出只能是因为宿主。
	True(t, Exists(filepath.Join(c.Up(), "version.json")), "④ 复核：客户端现场完好")
	Eq(t, "④ 复核：版本未被消费（UI 没有自行走完更新）",
		ReadVersionFile(t, c.Up()).Version, "1.0.0")
}
