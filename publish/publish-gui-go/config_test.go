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

func TestLoadProjectsMissingReturnsEmpty(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	if got := loadProjects(); len(got) != 0 {
		t.Fatalf("projects=%+v", got)
	}
}

func TestIsInitialized(t *testing.T) {
	dir := t.TempDir()
	if isInitialized(dir) {
		t.Fatal("empty folder reported initialized")
	}
	updator := filepath.Join(dir, ".updator")
	if err := os.MkdirAll(updator, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(updator, "shared.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isInitialized(dir) {
		t.Fatal("shared.json not detected")
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

// TestConfigLoadToleratesBOM covers files an editor saved with a UTF-8 byte
// order mark: the C# GUI reads them with File.ReadAllText, which strips the
// mark, so the Go GUI must strip it too instead of reading an empty config.
func TestConfigLoadToleratesBOM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALYPUBLISH_CONFIG_DIR", dir)
	bom := []byte{0xEF, 0xBB, 0xBF}

	projects := append(append([]byte{}, bom...),
		[]byte("[{\"DisplayName\":\"demo\",\"ProjectPath\":\"C:\\\\demo\"}]")...)
	if err := os.WriteFile(configPath(), projects, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadProjects(); len(got) != 1 || got[0].DisplayName != "demo" {
		t.Fatalf("projects with a BOM = %+v", got)
	}

	settings := append(append([]byte{}, bom...), []byte("{\"language\":\"en-US\"}")...)
	if err := os.WriteFile(settingsPath(), settings, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(); got.Language != "en-US" {
		t.Fatalf("settings with a BOM = %+v", got)
	}

	project := t.TempDir()
	updator := filepath.Join(project, ".updator")
	if err := os.MkdirAll(updator, 0o755); err != nil {
		t.Fatal(err)
	}
	shared := append(append([]byte{}, bom...),
		[]byte("{\"server_url\":\"http://server:2000\",\"project_name\":\"demo\"}")...)
	if err := os.WriteFile(filepath.Join(updator, "shared.json"), shared, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := loadShared(project); err != nil || got.ServerURL != "http://server:2000" {
		t.Fatalf("shared.json with a BOM = %+v, %v", got, err)
	}

	manifest := append(append([]byte{}, bom...),
		[]byte("{\"cli_rid\":\"windows-amd64\",\"client_rid\":\"windows-x86\"}")...)
	if err := os.WriteFile(filepath.Join(dir, "payload.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPayloadManifest(dir); got.CliRid != "windows-amd64" {
		t.Fatalf("payload.json with a BOM = %+v", got)
	}
}
