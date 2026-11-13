package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func testApp() *app {
	project := ProjectConfig{DisplayName: "demo", ProjectPath: `C:\demo`, MainExePath: "demo.exe"}
	return &app{
		lang:     "en",
		theme:    "light",
		accent:   "violet",
		status:   "Ready",
		projects: []ProjectConfig{project},
		selected: 0,
		cli:      &cliClient{exe: "test"},
		detail: &detailState{
			project:        project,
			currentVersion: "V1.0.0",
			forceKnown:     true,
			splitPercent:   62,
			unstaged: []fileItem{
				{RelativePath: "app.exe", Status: "modified", LocalSize: 2048, ServerSize: 1024},
			},
			staged: []fileItem{
				{RelativePath: "old.dll", Status: "deleted", LocalSize: 0, ServerSize: 512},
			},
			logs: []ChangeLog{
				{Version: "V1.0.0", Time: "2026-11-13 10:00:00", Logs: []string{"first release"}},
			},
		},
	}
}

func TestViewRendersInLightAndDark(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := testApp()
		tt := ui.NewTesterWith(a.view, 1440, 900, ui.TesterOptions{Dark: dark})
		for _, text := range []string{"Projects", "Unstaged", "Staged", "Publish", "Version history"} {
			if !tt.HasText(text) {
				t.Errorf("dark=%v: missing %q; texts=%q", dark, text, tt.Texts())
			}
		}
		if img := tt.Image(); img == nil {
			t.Errorf("dark=%v: no rendered image", dark)
		}
	}
}

func TestViewRendersAtMinimumSize(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.view, 1280, 720)
	for _, text := range []string{"Projects", "Unstaged", "Publish"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q at minimum size", text)
		}
	}
}

func TestEditModalRendersChips(t *testing.T) {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{
		project:       a.detail.project,
		displayName:   "demo",
		ignoreFolders: []string{"Log", "Config"},
		ignoreFiles:   []string{"nlog1.txt"},
		uncopyFolders: []string{"TestData"},
	}
	tt := ui.NewTester(a.view, 1440, 900)
	for _, text := range []string{"Project settings", "Server URL", "Server project", "Ignored folders", "Folders not to copy", "nlog1.txt"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

func TestEditModalBackdropKeepsOpen(t *testing.T) {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{
		project:     a.detail.project,
		serverURL:   "http://127.0.0.1:2000",
		projectName: "demo",
		displayName: "demo",
	}
	tt := ui.NewTester(a.view, 1440, 900)
	tt.ClickAt(4, 4)
	if !a.editOpen {
		t.Fatal("clicking the backdrop closed the edit dialog")
	}
}

func TestAddProjectModalRenders(t *testing.T) {
	a := testApp()
	a.addOpen = true
	a.adding = &addProjectState{
		serverURL:   "http://127.0.0.1:2000",
		projectName: "demo",
		title:       "Demo",
		path:        `C:\demo`,
		displayName: "demo",
		projects: []ProjectInfo{
			{Name: "demo", Title: "Demo"},
		},
	}
	tt := ui.NewTester(a.view, 1440, 900)
	for _, text := range []string{"Add server project", "Server URL", "Create project", "Initialize and add", "demo"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

func TestProjectStateCacheSurvivesSwitch(t *testing.T) {
	a := &app{
		projects: []ProjectConfig{
			{DisplayName: "one", ProjectPath: `C:\one`},
			{DisplayName: "two", ProjectPath: `C:\two`},
		},
		selected: 0,
		details:  map[string]*detailState{},
		cli:      &cliClient{},
	}
	a.openSelected()
	first := a.detail
	if first == nil {
		t.Fatal("first detail missing")
	}
	first.newVersion = "V9.9.9"
	a.selected = 1
	a.openSelected()
	if a.detail == first {
		t.Fatal("switching projects reused the first detail")
	}
	a.selected = 0
	a.openSelected()
	if a.detail != first || a.detail.newVersion != "V9.9.9" {
		t.Fatalf("cached detail not restored: %+v", a.detail)
	}
}

func TestAddProjectIgnoreModalRenders(t *testing.T) {
	a := testApp()
	a.addOpen = true
	a.adding = &addProjectState{
		serverURL:     "http://127.0.0.1:2000",
		projectName:   "demo",
		path:          `C:\demo`,
		advancedOpen:  true,
		ignoreFolders: []string{"Log", "Config"},
		ignoreFiles:   []string{"a.log"},
		uncopyFolders: []string{"TestData"},
	}
	tt := ui.NewTester(a.view, 1440, 900)
	for _, text := range []string{"Ignore rules", "Ignored folders", "Folders not to copy", "Back"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

func TestLocalPreviewModalRenders(t *testing.T) {
	a := testApp()
	a.localPreviewOpen = true
	a.localPreview = &localPreviewState{
		path: `C:\demo`,
		shared: SharedConfig{
			ServerURL:     "http://127.0.0.1:2000",
			ProjectName:   "demo",
			IgnoreFolders: []string{"Log"},
			IgnoreFiles:   []string{"a.log"},
		},
		displayName: "demo",
	}
	tt := ui.NewTester(a.view, 1440, 900)
	for _, text := range []string{"Add local project", "Server", "Ignore rules", "Add project", "demo"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

func TestAccentPalette(t *testing.T) {
	if normalizeAccent("rose") != "rose" || accentHex("rose") != "#e11d48" {
		t.Fatal("rose accent lookup failed")
	}
	if normalizeAccent("bogus") != "blue" || accentHex("bogus") != "#2563eb" {
		t.Fatal("invalid accent should fall back to blue")
	}
}

func TestThemeCycle(t *testing.T) {
	got := []string{}
	theme := "system"
	for i := 0; i < 3; i++ {
		theme = nextTheme(theme)
		got = append(got, theme)
	}
	want := []string{"light", "dark", "system"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cycle=%v want=%v", got, want)
		}
	}
	if normalizeTheme("bogus") != "system" {
		t.Fatal("invalid theme should normalize to system")
	}
}
