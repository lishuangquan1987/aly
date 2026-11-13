package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALYPUBLISH_CONFIG_DIR", dir)
	projectPath := filepath.Join(t.TempDir(), "demo")
	projects := []ProjectConfig{
		{DisplayName: "demo", ProjectPath: projectPath, MainExePath: "demo.exe"},
	}
	if err := saveProjects(projects); err != nil {
		t.Fatal(err)
	}
	// Saving again must replace the existing config file (Windows included).
	if err := saveProjects(projects); err != nil {
		t.Fatal(err)
	}
	got := loadProjects()
	if len(got) != 1 || got[0].DisplayName != "demo" || got[0].ProjectPath != projectPath {
		t.Fatalf("got=%+v", got)
	}
	if _, err := os.Stat(configPath()); err != nil {
		t.Fatal(err)
	}
}

func TestAppearanceSurvivesCSharpSettingsRewrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALYPUBLISH_CONFIG_DIR", dir)
	want := appearanceSettings{Theme: "dark", Accent: "violet"}
	if err := saveAppearance(want); err != nil {
		t.Fatal(err)
	}
	// C# LanguageManager writes exactly this shape when it switches language.
	if err := os.WriteFile(settingsPath(), []byte(`{"language":"en-US"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadAppearance(); got != want {
		t.Fatalf("appearance=%+v want=%+v", got, want)
	}
}

func TestLoadSharedRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	updator := filepath.Join(dir, ".updator")
	if err := os.MkdirAll(updator, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(updator, "shared.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadShared(dir); err == nil {
		t.Fatal("malformed shared.json was accepted")
	}
}

func TestLoadShared(t *testing.T) {
	dir := t.TempDir()
	updator := filepath.Join(dir, ".updator")
	if err := os.MkdirAll(updator, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"server_url":"http://127.0.0.1:2000","project_name":"demo","ignore_folders":["node_modules"],"ignore_files":["*.log"],"un_copy_folders":["cache"],"un_copy_files":["a.tmp"]}`
	if err := os.WriteFile(filepath.Join(updator, "shared.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	shared, err := loadShared(dir)
	if err != nil {
		t.Fatal(err)
	}
	if shared.ProjectName != "demo" || len(shared.IgnoreFolders) != 1 || len(shared.UnCopyFiles) != 1 {
		t.Fatalf("shared=%+v", shared)
	}
	if !isInitialized(dir) {
		t.Fatal("expected initialized")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	want := appSettings{Language: "zh-CN"}
	if err := saveSettings(want); err != nil {
		t.Fatal(err)
	}
	got := loadSettings()
	if got != want {
		t.Fatalf("settings=%+v want=%+v", got, want)
	}
	appearance := appearanceSettings{Theme: "dark", Accent: "violet"}
	if err := saveAppearance(appearance); err != nil {
		t.Fatal(err)
	}
	if got := loadAppearance(); got != appearance {
		t.Fatalf("appearance=%+v want=%+v", got, appearance)
	}
}
