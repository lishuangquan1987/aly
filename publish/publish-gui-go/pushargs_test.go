package main

import (
	"reflect"
	"testing"
)

func boolPtr(v bool) *bool { return &v }

// TestPushArgsMultiMessage pins the changelog contract: a multi-line release
// note becomes one --message per non-empty line (the CLI repeats --message),
// a note without usable lines keeps one empty --message so the CLI's
// required-check explains itself, and the force flag stays tri-state.
func TestPushArgsMultiMessage(t *testing.T) {
	got := pushArgs("V1.0.2", []string{"修复登录超时", "", "  加快下载 "}, "scripts\\done.cmd", boolPtr(true))
	want := []string{"--json", "push", "--version", "V1.0.2",
		"--message", "修复登录超时", "--message", "加快下载",
		"--after-apply-update-script", "scripts\\done.cmd",
		"--set-force-update"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pushArgs = %q, want %q", got, want)
	}

	got = pushArgs("V1.0.2", nil, "", boolPtr(false))
	want = []string{"--json", "push", "--version", "V1.0.2", "--message", "", "--clear-force-update"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pushArgs empty = %q, want %q", got, want)
	}

	got = pushArgs("V1.0.2", []string{"only"}, "", nil)
	want = []string{"--json", "push", "--version", "V1.0.2", "--message", "only"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pushArgs nil force = %q, want %q", got, want)
	}
}
