// +build windows

package app

import (
	"os/exec"
	"syscall"
)

const (
	// CREATE_NO_WINDOW：GUI 进程无控制台，这里主要是避免任何控制台闪窗。
	createNoWindowFlag = 0x08000000
	// CREATE_BREAKAWAY_FROM_JOB：宿主若用 kill-on-close Job Object，收尾进程
	// 必须在 UI 退出后继续活到替换完成；Job 不允许 breakaway 时回落。
	breakawayFromJob = 0x01000000
)

// spawnFinishSelfUpdate 启动 .new 的隐藏收尾进程（-finish-self-update）。
//
// 与旧的 cmd 延迟脚本不同：参数由 Go exec 直接转义后传给 .new，不经过
// cmd.exe / ping / move，因此没有固定延时、PATH、引号与 % 展开问题。
//
// 先尝试 CREATE_BREAKAWAY_FROM_JOB；若宿主 Job 不允许 breakaway（或权限
// 不足），回退到不带 breakaway 的启动——此时收尾进程仍受宿主 Job 生命周期
// 影响，但绝大多数部署没有 kill-on-close Job。
func spawnFinishSelfUpdate(exePath string, args []string) error {
	if err := startFinishProcess(exePath, args,
		createNoWindowFlag|breakawayFromJob); err == nil {
		return nil
	}
	return startFinishProcess(exePath, args, createNoWindowFlag)
}

func startFinishProcess(exePath string, args []string, flags uint32) error {
	c := exec.Command(exePath, args...)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: flags,
	}
	return c.Start()
}
