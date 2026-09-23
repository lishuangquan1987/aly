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

	scriptPath := filepath.Join(workDir, "post_update.bat")
	// cmd /c 执行 .bat：写入相对路径文件，用于验证工作目录
	if err := ioutil.WriteFile(scriptPath, []byte("@echo off\r\necho ok > out.txt\r\n"), 0644); err != nil {
		t.Fatalf("写脚本失败: %v", err)
	}

	runScript(scriptPath, workDir)

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

	scriptPath := filepath.Join(workDir, "hang.bat")
	if err := ioutil.WriteFile(scriptPath, []byte("@echo off\r\nping -n 30 127.0.0.1 >nul\r\n"), 0644); err != nil {
		t.Fatalf("写脚本失败: %v", err)
	}

	orig := scriptTimeout
	scriptTimeout = 200 * time.Millisecond
	defer func() { scriptTimeout = orig }()

	start := time.Now()
	runScript(scriptPath, workDir)
	elapsed := time.Since(start)

	// 超时后应快速返回（远小于脚本自身 30s 时长）
	if elapsed > 5*time.Second {
		t.Errorf("脚本超时后未及时返回，耗时 %v", elapsed)
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
	scriptPath := filepath.Join(appDir, "post.bat")
	if err := ioutil.WriteFile(scriptPath, []byte("@echo off\r\necho x >> counter.txt\r\n"), 0644); err != nil {
		t.Fatalf("写脚本失败: %v", err)
	}

	fc := &FullConfig{MainFolder: appDir}
	runPostApplyScript(fc, "post.bat", "2.0.0")
	runPostApplyScript(fc, "post.bat", "2.0.0")

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
