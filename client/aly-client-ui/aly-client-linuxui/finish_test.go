//go:build linux

package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParseFinishArgs(t *testing.T) {
	dir, err := ioutil.TempDir("", "finish-args")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target := filepath.Join(dir, "aly-client-ui")
	newPath := target + ".new"
	writeFile(t, newPath, "NEW")
	opt, err := parseFinishArgs([]string{
		"--target", target, "--pid", "1234", "--created", "99",
		"--new", newPath, "--", "-mode", "check",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opt.target != target || opt.newPath != newPath || opt.pid != 1234 || opt.created != "99" {
		t.Fatalf("bad opts: %+v", opt)
	}
	if len(opt.restart) != 2 || opt.restart[0] != "-mode" {
		t.Fatalf("bad restart: %#v", opt.restart)
	}
	for _, bad := range [][]string{
		{"--target", target, "--new", newPath},
		{"--target", "rel", "--pid", "1", "--new", newPath},
		{"--target", target, "--pid", "1", "--new", target},
	} {
		if _, err := parseFinishArgs(bad); err == nil {
			t.Fatalf("expected error: %#v", bad)
		}
	}
}

func TestReplaceWithBackup(t *testing.T) {
	dir, err := ioutil.TempDir("", "finish-replace")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target := filepath.Join(dir, "aly-client-ui")
	newPath := target + ".new"
	writeFile(t, target, "OLD")
	writeFile(t, newPath, "NEW")
	if err := replaceWithBackup(newPath, target); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "NEW" {
		t.Fatalf("target=%q", got)
	}
	if got := readFile(t, target+".old"); got != "OLD" {
		t.Fatalf(".old=%q", got)
	}
}

func TestProcessGoneAndDeadPID(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if processGone(pid) {
		t.Fatal("running process should not be gone")
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if !processGone(pid) {
		t.Fatalf("killed/reaped pid %d should be gone", pid)
	}
	if !waitForOldProcess(999999, "0", filepath.Join(t.TempDir(), "ui")) {
		t.Fatal("nonexistent pid should be treated as gone")
	}
}

func TestFinishSelfUpdateFlow(t *testing.T) {
	dir, err := ioutil.TempDir("", "finish-flow")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target := filepath.Join(dir, "aly-client-ui")
	newPath := target + ".new"
	writeFile(t, target, "OLD")
	writeFile(t, newPath, "NEW")

	var started string
	var args []string
	orig := startNewUIFn
	startNewUIFn = func(t string, a []string) error {
		started, args = t, append([]string(nil), a...)
		return nil
	}
	defer func() { startNewUIFn = orig }()

	code := finishSelfUpdate([]string{
		"--target", target, "--pid", "999999", "--created", "0",
		"--new", newPath, "--", "-mode", "check",
	})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if got := readFile(t, target); got != "NEW" {
		t.Fatalf("target=%q", got)
	}
	if got := readFile(t, target+".old"); got != "OLD" {
		t.Fatalf(".old=%q", got)
	}
	if started != target || len(args) != 2 || args[0] != "-mode" {
		t.Fatalf("started=%q args=%#v", started, args)
	}
	logData := readFile(t, target+".update.log")
	if !strings.Contains(logData, "finish OK") {
		t.Fatalf("log:\n%s", logData)
	}
	if opts, _ := parseFinishArgs([]string{"--target", target, "--pid", "999999", "--new", newPath, "--"}); opts.pid != 999999 {
		t.Fatal(strconv.Itoa(opts.pid))
	}
}
