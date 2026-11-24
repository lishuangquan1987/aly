package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLanguageTogglePersists checks the language button: toggling writes the
// choice to settings.json and a fresh app instance starts in the toggled
// language, exactly like the C# GUI's settings file contract.
func TestLanguageTogglePersists(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())

	a := testApp()
	a.lang = "zh"
	a.toggleLang()
	if a.lang != "en" {
		t.Fatalf("lang after toggle = %q, want en", a.lang)
	}
	if got := loadSettings().Language; got != "en-US" {
		t.Fatalf("settings.json language = %q, want en-US", got)
	}

	a.toggleLang()
	if a.lang != "zh" {
		t.Fatalf("lang after second toggle = %q, want zh", a.lang)
	}
	if got := loadSettings().Language; got != "zh-CN" {
		t.Fatalf("settings.json language = %q, want zh-CN", got)
	}
}

// TestAppearancePersists checks that theme and accent choices survive a save
// and reload through appearance.json, independently of settings.json.
func TestAppearancePersists(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())

	a := testApp()
	a.theme = "dark"
	a.accent = "rose"
	a.savePrefs()

	got := loadAppearance()
	if got.Theme != "dark" || got.Accent != "rose" {
		t.Fatalf("appearance after savePrefs = %+v, want dark/rose", got)
	}

	// A settings.json rewrite (what the historical C# build does when it
	// switches language) must not touch the appearance file.
	if err := saveSettings(appSettings{Language: "zh-CN"}); err != nil {
		t.Fatal(err)
	}
	got = loadAppearance()
	if got.Theme != "dark" || got.Accent != "rose" {
		t.Fatalf("appearance after settings rewrite = %+v, want dark/rose", got)
	}
}

// TestBookmarkRemoveRoundtrip checks the remove flow end to end: removing the
// selected project drops only that bookmark, writes config.json immediately,
// and reselects a neighbour — the project's files and .updator/ stay untouched.
func TestBookmarkRemoveRoundtrip(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	projects := []ProjectConfig{
		{DisplayName: "first", ProjectPath: first},
		{DisplayName: "second", ProjectPath: second},
	}
	if err := saveProjects(projects); err != nil {
		t.Fatal(err)
	}

	a := testApp()
	a.projects = projects
	a.selected = 1
	a.detail = &detailState{project: projects[1]}

	a.removeSelected()

	if len(a.projects) != 1 || a.projects[0].DisplayName != "first" {
		t.Fatalf("projects after remove = %+v, want only first", a.projects)
	}
	// removeSelected re-selects the remaining project, so the detail view
	// switches to it instead of staying nil.
	if a.detail == nil || a.detail.project.DisplayName != "first" {
		t.Fatalf("detail after remove = %+v, want the remaining first project", a.detail)
	}
	got := loadProjects()
	if len(got) != 1 || got[0].DisplayName != "first" {
		t.Fatalf("config.json after remove = %+v, want only first", got)
	}
	// The removed project's directory itself must survive.
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("removed project's files must stay on disk: %v", err)
	}
}

// TestI18nKeyParity checks that every translation table defines exactly the
// same key set for Chinese and English, so a missing key shows up in tests
// instead of as an untranslated label at runtime.
func TestI18nKeyParity(t *testing.T) {
	zhKeys := translations["zh"]
	enKeys := translations["en"]
	if len(zhKeys) == 0 || len(enKeys) == 0 {
		t.Fatalf("translation tables are empty: zh=%d en=%d", len(zhKeys), len(enKeys))
	}
	var missingInEN, missingInZH []string
	for key := range zhKeys {
		if _, ok := enKeys[key]; !ok {
			missingInEN = append(missingInEN, key)
		}
	}
	for key := range enKeys {
		if _, ok := zhKeys[key]; !ok {
			missingInZH = append(missingInZH, key)
		}
	}
	if len(missingInEN) > 0 || len(missingInZH) > 0 {
		t.Fatalf("i18n key sets differ: missing in en=%v, missing in zh=%v",
			missingInEN, missingInZH)
	}
}

func TestBookmarkRemovePreservesActualProjectFiles(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	projectDir := t.TempDir()
	appPath := filepath.Join(projectDir, "app.exe")
	sharedPath := filepath.Join(projectDir, ".updator", "shared.json")
	if err := os.MkdirAll(filepath.Dir(sharedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	appBody := []byte("application-bytes")
	sharedBody := []byte(`{"server_url":"http://server:2000","project_name":"demo"}`)
	if err := os.WriteFile(appPath, appBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sharedPath, sharedBody, 0o644); err != nil {
		t.Fatal(err)
	}
	project := ProjectConfig{DisplayName: "demo", ProjectPath: projectDir}
	a := testApp()
	a.projects = []ProjectConfig{project}
	a.selected = 0
	a.detail = &detailState{project: project}
	a.removeSelected()
	if len(a.projects) != 0 || a.detail != nil || a.selected != -1 {
		t.Fatalf("bookmark was not removed: projects=%v detail=%+v selected=%d", a.projects, a.detail, a.selected)
	}
	gotApp, err := os.ReadFile(appPath)
	if err != nil || string(gotApp) != string(appBody) {
		t.Fatalf("application changed: %q, %v", gotApp, err)
	}
	gotShared, err := os.ReadFile(sharedPath)
	if err != nil || string(gotShared) != string(sharedBody) {
		t.Fatalf("shared config changed: %q, %v", gotShared, err)
	}
	if got := loadProjects(); len(got) != 0 {
		t.Fatalf("persisted bookmarks = %+v", got)
	}
}
