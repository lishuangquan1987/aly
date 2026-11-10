//go:build !windows

package alyclient

import "os/exec"

// hideWindow 在非 Windows 平台无需处理。
func hideWindow(cmd *exec.Cmd) {}
