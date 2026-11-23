// +build windows

package cmd

import (
	"fmt"
	"os/exec"
)

// setScriptProcAttr Windows 侧无需进程组设置（taskkill /T 按父子树遍历）。
func setScriptProcAttr(cmd *exec.Cmd) {}

// killScriptTree 终止脚本进程树：
// cmd /c 脚本常派生孙进程（如 .bat 里再启 exe），只 Kill 直接子进程会留下孤儿，
// 持锁/占端口继续运行。taskkill /T /F 自 XP 起系统自带，按父子树强制结束。
// taskkill 失败时兜底 Kill 直接子进程（尽力而为）。
func killScriptTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	tk := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
	if tk.Run() == nil {
		return
	}
	// 兜底：taskkill 不可用/失败时至少杀掉直接子进程
	cmd.Process.Kill()
}
