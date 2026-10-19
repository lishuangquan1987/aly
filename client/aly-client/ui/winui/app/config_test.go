// +build windows

package app

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// writeTempClientJSON 在临时目录写一个 client.json 并返回目录。
func writeTempClientJSON(t *testing.T, content string) string {
	dir, err := ioutil.TempDir("", "alyui")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "client.json"), []byte(content), 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("WriteFile: %v", err)
	}
	return dir
}

func TestReadMainExeRelativePath(t *testing.T) {
	dir := writeTempClientJSON(t, `{
  "main_exe_relative_path": "../win-x64/YOFC.OTDR3001.exe",
  "must_close_process_name": ["YOFC.OTDR3001"]
}`)
	defer os.RemoveAll(dir)

	got, err := ReadMainExeRelativePath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "../win-x64/YOFC.OTDR3001.exe" {
		t.Fatalf("got %q", got)
	}
}

func TestReadMainExeRelativePathMissingFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "alyui")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	if _, err := ReadMainExeRelativePath(dir); err == nil {
		t.Fatal("expected error when client.json is missing")
	}
}

func TestReadMainExeRelativePathEmptyField(t *testing.T) {
	dir := writeTempClientJSON(t, `{"main_exe_relative_path":""}`)
	defer os.RemoveAll(dir)

	if _, err := ReadMainExeRelativePath(dir); err == nil {
		t.Fatal("expected error when main_exe_relative_path is empty")
	}
}

func TestReadMainExeRelativePathBadJSON(t *testing.T) {
	dir := writeTempClientJSON(t, `{not json`)
	defer os.RemoveAll(dir)

	if _, err := ReadMainExeRelativePath(dir); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestMainFolderFromUpdateFolder(t *testing.T) {
	// UpdateFolder 与 win-x64 同级
	update := filepath.Join("C:", "app", "UpdateFolder")
	got, err := MainFolder(update, "../win-x64/YOFC.OTDR3001.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("C:", "app", "win-x64")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMainFolderNestedPath(t *testing.T) {
	// 主程序在更深一层目录时，仍取 exe 所在目录
	update := filepath.Join("D:", "pkg", "UpdateFolder")
	got, err := MainFolder(update, "../app/bin/MyApp.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("D:", "pkg", "app", "bin")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMainFolderEmptyArg(t *testing.T) {
	if _, err := MainFolder(filepath.Join("C:", "app"), ""); err == nil {
		t.Fatal("expected error for empty mainExeRelativePath")
	}
}
