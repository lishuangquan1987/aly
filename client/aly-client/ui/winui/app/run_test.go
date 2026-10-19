// +build windows

package app

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func mkTemp(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "alyapp")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func touch(t *testing.T, p string) {
	if err := ioutil.WriteFile(p, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", p, err)
	}
}

// 候选 A：aly-client.exe 与 UI 同目录（真实部署：都在 UpdateFolder）。
func TestFindAlyClientSameDir(t *testing.T) {
	dir, cleanup := mkTemp(t)
	defer cleanup()

	touch(t, filepath.Join(dir, "aly-client.exe"))

	got, err := FindAlyClient(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "aly-client.exe")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// 候选 B：由 client.json 的 main_exe_relative_path 推导主程序目录。
func TestFindAlyClientViaClientJSON(t *testing.T) {
	dir, cleanup := mkTemp(t)
	defer cleanup()

	// <dir>/UpdateFolder 作为 UI 目录
	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// 主程序目录里放 aly-client.exe，UpdateFolder 里不放
	touch(t, filepath.Join(main, "aly-client.exe"))
	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../win-x64/App.exe"}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := FindAlyClient(update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(main, "aly-client.exe")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// 同目录优先于 client.json 推导。
func TestFindAlyClientPrefersSameDir(t *testing.T) {
	dir, cleanup := mkTemp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	touch(t, filepath.Join(update, "aly-client.exe"))
	touch(t, filepath.Join(main, "aly-client.exe"))
	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../win-x64/App.exe"}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := FindAlyClient(update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join(update, "aly-client.exe") {
		t.Fatalf("same-dir candidate must win, got %q", got)
	}
}

// 两处都找不到：必须报错且信息可读。
func TestFindAlyClientNotFound(t *testing.T) {
	dir, cleanup := mkTemp(t)
	defer cleanup()

	if _, err := FindAlyClient(dir); err == nil {
		t.Fatal("expected error when aly-client.exe is absent everywhere")
	}
}

// SelfDir 必须返回可执行文件所在目录。
func TestSelfDir(t *testing.T) {
	dir, cleanup := mkTemp(t)
	defer cleanup()

	p := filepath.Join(dir, "aly-client-ui.exe")
	touch(t, p)

	got, err := SelfDirOf(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Fatalf("got %q want %q", got, dir)
	}
}

// 模式分派：未知模式不得 panic，且应落到安全默认值。
func TestNormalizeMode(t *testing.T) {
	cases := map[string]string{
		"":         "auto",
		"auto":     "auto",
		"AUTO":     "auto",
		" check ":  "check",
		"silent":   "silent",
		"update":   "update",
		"rollback": "rollback",
		"bogus":    "auto",
	}
	for in, want := range cases {
		if got := NormalizeMode(in); got != want {
			t.Errorf("NormalizeMode(%q)=%q want %q", in, got, want)
		}
	}
}

func TestVersionLabel(t *testing.T) {
	cases := map[string]string{
		"":       "",
		"1.0.1":  "v1.0.1",
		"v1.0.1": "v1.0.1",
		"0.9":    "v0.9",
	}
	for in, want := range cases {
		if got := versionLabel(in); got != want {
			t.Errorf("versionLabel(%q)=%q want %q", in, got, want)
		}
	}
}

