package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// TestBuildMainExeCmdDir 验证主程序启动命令的工作目录被显式设为主程序目录（#10）。
func TestBuildMainExeCmdDir(t *testing.T) {
	cmd := buildMainExeCmd(`C:\pkg\ApplicationFolder\app.exe`, `C:\pkg\ApplicationFolder`)
	if cmd.Dir != `C:\pkg\ApplicationFolder` {
		t.Errorf("cmd.Dir 应为主程序目录，实际 %q", cmd.Dir)
	}
	if cmd.Path != `C:\pkg\ApplicationFolder\app.exe` {
		t.Errorf("cmd.Path 应为主程序 exe，实际 %q", cmd.Path)
	}
}

// writeScriptForPlatform 按当前平台写测试用后置脚本：
// Windows 写 .bat（runScript 走 cmd /c，内容逐字节保持历史用例不变）；
// Unix 写 .sh（runScript 直 exec sh scriptPath，换行用 \n）。
// 两端语义对齐，保证同一用例在 Windows / Linux 都可运行。
func writeScriptForPlatform(t *testing.T, dir, base, winBody, nixBody string) string {
	t.Helper()
	if os.PathSeparator == '\\' {
		p := filepath.Join(dir, base+".bat")
		if err := ioutil.WriteFile(p, []byte(winBody), 0644); err != nil {
			t.Fatalf("写脚本失败: %v", err)
		}
		return p
	}
	p := filepath.Join(dir, base+".sh")
	if err := ioutil.WriteFile(p, []byte(nixBody), 0755); err != nil {
		t.Fatalf("写脚本失败: %v", err)
	}
	return p
}

// TestRunScriptExecutesInWorkDir 验证后置脚本在指定工作目录下同步执行（#10/#11）：
// 脚本创建的相对文件落在 workDir，而非调用方 CWD。
func TestRunScriptExecutesInWorkDir(t *testing.T) {
	workDir, err := ioutil.TempDir("", "script-work")
	if err != nil {
		t.Fatalf("创建 workDir 失败: %v", err)
	}
	defer os.RemoveAll(workDir)
	otherDir, err := ioutil.TempDir("", "script-other")
	if err != nil {
		t.Fatalf("创建 otherDir 失败: %v", err)
	}
	defer os.RemoveAll(otherDir)

	// cmd /c 执行 .bat：写入相对路径文件，用于验证工作目录
	scriptPath := writeScriptForPlatform(t, workDir, "post_update",
		"@echo off\r\necho ok > out.txt\r\n",
		"echo ok > out.txt\n")

	// 脚本成功执行：不应返回 error（B9 签名改造）
	if err := runScript(scriptPath, workDir); err != nil {
		t.Fatalf("脚本成功执行不应返回 error: %v", err)
	}

	// 脚本同步执行完成：workDir/out.txt 存在
	if _, err := os.Stat(filepath.Join(workDir, "out.txt")); err != nil {
		t.Errorf("脚本应在 workDir 下执行并生成 out.txt: %v", err)
	}
	// otherDir 不受影响
	if _, err := os.Stat(filepath.Join(otherDir, "out.txt")); err == nil {
		t.Errorf("otherDir 不应生成 out.txt（脚本未在其下执行）")
	}
}

// TestRunScriptTimeout 验证脚本挂死时按超时强制结束并返回（#11）。
func TestRunScriptTimeout(t *testing.T) {
	workDir, err := ioutil.TempDir("", "script-timeout")
	if err != nil {
		t.Fatalf("创建 workDir 失败: %v", err)
	}
	defer os.RemoveAll(workDir)

	// 脚本挂死 30s：Windows 用 ping 延时，Unix 用 sleep，语义对齐
	scriptPath := writeScriptForPlatform(t, workDir, "hang",
		"@echo off\r\nping -n 30 127.0.0.1 >nul\r\n",
		"sleep 30\n")

	orig := scriptTimeout
	scriptTimeout = 200 * time.Millisecond
	defer func() { scriptTimeout = orig }()

	start := time.Now()
	runErr := runScript(scriptPath, workDir)
	elapsed := time.Since(start)

	// 超时后应快速返回（远小于脚本自身 30s 时长），且必须返回 error（B9：会让恢复重跑）
	if elapsed > 5*time.Second {
		t.Errorf("脚本超时后未及时返回，耗时 %v", elapsed)
	}
	if runErr == nil {
		t.Error("脚本超时被 Kill 应返回 error，实际 nil")
	}
}

// TestRunPostApplyScriptMarker 验证加固②：脚本成功完成后写 marker，第二次调用跳过（不重跑），
// 崩溃恢复据此不再重跑已完成的后置脚本。
func TestRunPostApplyScriptMarker(t *testing.T) {
	workDir, err := ioutil.TempDir("", "post-script-marker")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	config.SetExeDir(workDir)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(workDir) })

	appDir := filepath.Join(workDir, "ApplicationFolder")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("创建 appDir 失败: %v", err)
	}
	scriptPath := writeScriptForPlatform(t, appDir, "post",
		"@echo off\r\necho x >> counter.txt\r\n",
		"echo x >> counter.txt\n")

	fc := &FullConfig{MainFolder: appDir}
	runPostApplyScript(fc, filepath.Base(scriptPath), "2.0.0")
	runPostApplyScript(fc, filepath.Base(scriptPath), "2.0.0")

	data, err := ioutil.ReadFile(filepath.Join(appDir, "counter.txt"))
	if err != nil {
		t.Fatalf("读 counter.txt 失败: %v", err)
	}
	if strings.Count(string(data), "x") != 1 {
		t.Errorf("脚本应只执行一次（marker 跳过第二次），counter 内容: %q", data)
	}
	if _, statErr := os.Stat(filepath.Join(appDir, ".updator", "after_apply_2.0.0.done")); statErr != nil {
		t.Errorf("marker 应已写入: %v", statErr)
	}
}
