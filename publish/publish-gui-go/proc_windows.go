//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps a console-subsystem child (aly-publish.exe) from
// allocating a console at all. HideWindow alone can still flash a window.
const createNoWindow = 0x08000000

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
