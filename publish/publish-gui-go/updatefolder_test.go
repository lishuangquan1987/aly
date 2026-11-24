package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestCreateUpdateFolderTrailingSeparatorAndExplicitProcess(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "App")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainExe := filepath.Join(projectDir, "demo.exe")
	if err := os.WriteFile(mainExe, []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := createUpdateFolder(projectDir+string(filepath.Separator), mainExe, "CustomProcess")
	if err != nil {
		t.Fatal(err)
	}
	updateDir := filepath.Join(base, "UpdateFolder")
	raw, err := os.ReadFile(filepath.Join(updateDir, "client.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg updateClientConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	wantRel, err := filepath.Rel(updateDir, mainExe)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MainExeRelativePath != wantRel || len(cfg.MustCloseProcessName) != 1 || cfg.MustCloseProcessName[0] != "CustomProcess" {
		t.Fatalf("client config = %+v, want rel %q and CustomProcess", cfg, wantRel)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "UpdateFolder")); !os.IsNotExist(err) {
		t.Fatalf("UpdateFolder was created inside project: %v", err)
	}
}

func TestEnsureUpdatersForRepairsMissingClientJSON(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "App")
	updateDir := filepath.Join(base, "UpdateFolder")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(updateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "demo.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = ensureUpdatersFor(projectDir, "demo.exe")
	clientPath := filepath.Join(updateDir, "client.json")
	if _, err := os.Stat(clientPath); err != nil {
		t.Fatalf("half-written UpdateFolder was not repaired: %v", err)
	}
	custom := []byte(`{"main_exe_relative_path":"custom","must_close_process_name":["custom"]}`)
	if err := os.WriteFile(clientPath, custom, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = ensureUpdatersFor(projectDir, "demo.exe")
	got, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Fatalf("existing client.json was overwritten: %s", got)
	}
}

func TestWriteFileAtomicFailureLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(target, []byte("new")); err == nil {
		t.Fatal("rename onto a directory unexpectedly succeeded")
	}
	matches, err := filepath.Glob(filepath.Join(dir, "config.json.*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("failed atomic write left temp files: %v", matches)
	}
}

func TestCopyUpdatersInCopiesBothPayloadFiles(t *testing.T) {
	appDir := t.TempDir()
	updateDir := t.TempDir()
	writeManifest(t, appDir, `{"client_rid":"probe-client"}`)
	for _, name := range updaterNames() {
		path := filepath.Join(appDir, "client", "probe-client", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("payload-"+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if warning := copyUpdatersIn(appDir, updateDir); warning != "" {
		t.Fatalf("copyUpdatersIn warning = %q", warning)
	}
	for _, name := range updaterNames() {
		got, err := os.ReadFile(filepath.Join(updateDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "payload-"+name {
			t.Fatalf("%s content = %q", name, got)
		}
	}
}

func TestCopyUpdatersInWarnsAndKeepsAvailableFile(t *testing.T) {
	appDir := t.TempDir()
	updateDir := t.TempDir()
	writeManifest(t, appDir, `{"client_rid":"probe-client"}`)
	names := updaterNames()
	path := filepath.Join(appDir, "client", "probe-client", names[0])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("available"), 0o755); err != nil {
		t.Fatal(err)
	}
	warning := copyUpdatersIn(appDir, updateDir)
	if !strings.Contains(warning, names[1]) {
		t.Fatalf("warning %q does not name missing %s", warning, names[1])
	}
	got, err := os.ReadFile(filepath.Join(updateDir, names[0]))
	if err != nil || string(got) != "available" {
		t.Fatalf("available updater = %q, %v", got, err)
	}
}
