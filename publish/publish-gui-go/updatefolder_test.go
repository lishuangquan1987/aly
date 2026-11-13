package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateUpdateFolderWritesClientJSON(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "App")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainExe := filepath.Join(projectDir, "demo.exe")
	if err := os.WriteFile(mainExe, []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := createUpdateFolder(projectDir, mainExe, ""); err != nil {
		t.Fatalf("createUpdateFolder: %v", err)
	}

	clientJSON := filepath.Join(base, "UpdateFolder", "client.json")
	raw, err := os.ReadFile(clientJSON)
	if err != nil {
		t.Fatal(err)
	}
	var cfg updateClientConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	wantRel, err := filepath.Rel(filepath.Join(base, "UpdateFolder"), mainExe)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MainExeRelativePath != wantRel {
		t.Fatalf("main_exe_relative_path=%q want=%q", cfg.MainExeRelativePath, wantRel)
	}
	if len(cfg.MustCloseProcessName) != 1 || cfg.MustCloseProcessName[0] != "demo" {
		t.Fatalf("must_close_process_name=%v", cfg.MustCloseProcessName)
	}
}

func TestRefreshUpdatersKeepsClientJSON(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "App")
	updateDir := filepath.Join(base, "UpdateFolder")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(updateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	clientPath := filepath.Join(updateDir, "client.json")
	original := []byte(`{"main_exe_relative_path":"custom","must_close_process_name":["custom"]}`)
	if err := os.WriteFile(clientPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := refreshUpdaters(projectDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("client.json changed: %s", got)
	}
}

func TestUpdaterNames(t *testing.T) {
	names := updaterNames()
	if len(names) != 2 || names[0] == "" || names[0] == names[1] {
		t.Fatalf("updaterNames=%v", names)
	}
}

func TestRefreshUpdatersWithoutFolderIsNoop(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "App")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	warning, err := refreshUpdaters(projectDir)
	if err != nil || warning != "" {
		t.Fatalf("refreshUpdaters=%q, %v", warning, err)
	}
}
