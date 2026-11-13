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
	for _, text := range []string{"Project settings", "Ignored folders", "Folders not to copy", "nlog1.txt"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
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
