package cmd

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// TestRunScriptNonZeroExitReturnsError B9：脚本退出码非零必须返回 error
// （Windows 下 cmd /c 把 ERRORLEVEL 作为自身退出码传出，cmd.Wait 返回 *exec.ExitError）。
func TestRunScriptNonZeroExitReturnsError(t *testing.T) {
	workDir, err := ioutil.TempDir("", "script-exit-nonzero")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(workDir)
	config.SetExeDir(workDir)
	defer config.SetExeDir("")

	scriptPath := writeScriptForPlatform(t, workDir, "fail",
		"@echo off\r\nexit /b 3\r\n",
		"exit 3\n")
	if err := runScript(scriptPath, workDir); err == nil {
		t.Fatal("非零退出码的脚本应返回 error")
	}
}

// TestRunScriptTimeoutReturnsError B9：脚本超时被 Kill 必须返回 error（崩溃恢复会重跑）。
func TestRunScriptTimeoutReturnsError(t *testing.T) {
	workDir, err := ioutil.TempDir("", "script-timeout-err")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(workDir)
	config.SetExeDir(workDir)
	defer config.SetExeDir("")

	scriptPath := writeScriptForPlatform(t, workDir, "hang",
		"@echo off\r\nping -n 30 127.0.0.1 >nul\r\n",
		"sleep 30\n")
	orig := scriptTimeout
	scriptTimeout = 200 * time.Millisecond
	defer func() { scriptTimeout = orig }()

	if err := runScript(scriptPath, workDir); err == nil {
		t.Fatal("超时被 Kill 的脚本应返回 error")
	}
}

// TestRunScriptStartFailureReturnsError B9：启动失败必须返回 error。
// 通过 scriptCmdFn 注入一个不存在的可执行文件稳定触发 Start 失败。
func TestRunScriptStartFailureReturnsError(t *testing.T) {
	workDir, err := ioutil.TempDir("", "script-start-fail")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(workDir)
	config.SetExeDir(workDir)
	defer config.SetExeDir("")

	scriptPath := writeScriptForPlatform(t, workDir, "noop", "@echo off\r\n", ":\n")
	orig := scriptCmdFn
	scriptCmdFn = func(scriptPath string) *exec.Cmd {
		return exec.Command("aly-client-definitely-not-an-exe-for-test")
	}
	defer func() { scriptCmdFn = orig }()

	if err := runScript(scriptPath, workDir); err == nil {
		t.Fatal("脚本启动失败应返回 error")
	}
}

// TestRunPostApplyScriptDoesNotMarkOnFailure B9：三条失败路径（非零退出 / 超时 / 启动失败）
// 都不得写 marker —— 否则失败的迁移脚本被标记"已完成"，崩溃恢复永远不会重跑。
func TestRunPostApplyScriptDoesNotMarkOnFailure(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, appDir string) // 写脚本/注入故障
	}{
		{
			name: "非零退出",
			setup: func(t *testing.T, appDir string) {
				writeScriptForPlatform(t, appDir, "post",
					"@echo off\r\nexit /b 1\r\n",
					"exit 1\n")
			},
		},
		{
			name: "超时被 Kill",
			setup: func(t *testing.T, appDir string) {
				writeScriptForPlatform(t, appDir, "post",
					"@echo off\r\nping -n 30 127.0.0.1 >nul\r\n",
					"sleep 30\n")
				orig := scriptTimeout
				scriptTimeout = 200 * time.Millisecond
				t.Cleanup(func() { scriptTimeout = orig })
			},
		},
		{
			name: "启动失败",
			setup: func(t *testing.T, appDir string) {
				writeScriptForPlatform(t, appDir, "post", "@echo off\r\n", ":\n")
				orig := scriptCmdFn
				scriptCmdFn = func(scriptPath string) *exec.Cmd {
					return exec.Command("aly-client-definitely-not-an-exe-for-test")
				}
				t.Cleanup(func() { scriptCmdFn = orig })
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			workDir, err := ioutil.TempDir("", "post-script-fail")
			if err != nil {
				t.Fatalf("创建临时目录失败: %v", err)
			}
			t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(workDir) })
			config.SetExeDir(workDir)

			appDir := filepath.Join(workDir, "ApplicationFolder")
			if err := os.MkdirAll(appDir, 0755); err != nil {
				t.Fatalf("创建 appDir 失败: %v", err)
			}
			c.setup(t, appDir)

			fc := &FullConfig{MainFolder: appDir}
			relPath := "post.bat"
			if os.PathSeparator != '\\' {
				relPath = "post.sh"
			}
			runPostApplyScript(fc, relPath, "2.0.0")

			marker := filepath.Join(appDir, ".updator", "after_apply_2.0.0.done")
			if _, statErr := os.Stat(marker); statErr == nil {
				t.Fatalf("脚本失败时不得写 marker（B9），实际存在: %s", marker)
			}
			logData, logErr := ioutil.ReadFile(filepath.Join(workDir, "update.log"))
			if logErr == nil && !strings.Contains(string(logData), "not marked as done") {
				t.Errorf("应留痕“脚本失败、marker 未写”，实际日志: %s", logData)
			}
		})
	}
}
