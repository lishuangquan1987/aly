//go:build windows

package main

import (
	"os/exec"
	"testing"
)

func TestHideConsoleUsesNoWindow(t *testing.T) {
	cmd := exec.Command("aly-publish.exe")
	hideConsole(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow is false")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags=%#x missing CREATE_NO_WINDOW", cmd.SysProcAttr.CreationFlags)
	}
}
