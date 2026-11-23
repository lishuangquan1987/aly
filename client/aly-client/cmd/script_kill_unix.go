// +build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// setScriptProcAttr Unix 侧让脚本进程独立成进程组（Setpgid），
// 超时后可整组终止（含孙进程）。
func setScriptProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killScriptTree 终止脚本进程树：向整个进程组发 SIGKILL（负 PID）。
// Setpgid 依赖 start 时已设置（runScript 在 Start 前统一调用 setScriptProcAttr）；
// 组信号失败时兜底 Kill 直接子进程。
func killScriptTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err == nil {
		return
	}
	cmd.Process.Kill()
}
