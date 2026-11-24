package main

import (
	"os"
	"path/filepath"
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
	tt := ui.NewTester(a.view, 1080, 640)
	for _, text := range []string{"Projects", "Unstaged", "Publish"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q at minimum size", text)
		}
	}
	if img := tt.Image(); img == nil {
		t.Error("no rendered image at minimum size")
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

func TestEditModalEscapeStillCloses(t *testing.T) {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{
		project:     a.detail.project,
		serverURL:   "http://127.0.0.1:2000",
		projectName: "demo",
		displayName: "demo",
	}
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Key(0, ui.KeyEscape)
	if a.editOpen {
		t.Fatal("Escape did not close the edit dialog")
	}
}

func TestAddProjectModalRenders(t *testing.T) {
	a := testApp()
	a.addOpen = true
	a.adding = &addProjectState{
		serverURL:   "http://127.0.0.1:2000",
		projectName: "demo",
		title:       "Demo",
		displayName: "demo",
		allProjects: []ProjectInfo{
			{ID: 1, Name: "demo", Title: "Demo"},
		},
		selected: 0,
	}
	tt := ui.NewTester(a.view, 1320, 840)
	for _, text := range []string{"Add server project", "Server URL", "Create project", "Fetch projects", "demo"} {
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
		stage:         addStageRules,
		serverURL:     "http://127.0.0.1:2000",
		projectName:   "demo",
		path:          `C:\demo`,
		ignoreFolders: []string{"Log", "Config"},
		ignoreFiles:   []string{"a.log"},
		uncopyFolders: []string{"TestData"},
	}
	tt := ui.NewTester(a.view, 1320, 840)
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

// TestSidebarClearSearch covers the no-match empty state's call to action:
// clicking it drops the filter and the project comes back, as the C# view's
// ClearProjectFilterCommand does.
func TestSidebarClearSearch(t *testing.T) {
	a := testApp()
	a.cli = &cliClient{} // unavailable, so the refresh path starts no goroutine
	a.filter = "zzz"
	tt := ui.NewTester(a.view, 1320, 840)
	for _, text := range []string{"No matching projects", "Clear search"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q; texts=%q", text, tt.Texts())
		}
	}
	if err := tt.Click("Clear search"); err != nil {
		t.Fatalf("clicking Clear search failed: %v", err)
	}
	if a.filter != "" {
		t.Fatalf("filter = %q after Clear search, want empty", a.filter)
	}
	if !tt.HasText("demo") {
		t.Fatalf("the project did not come back; texts=%q", tt.Texts())
	}
}

// TestEmptyProjectListShowsActions checks the C# empty state's two calls to
// action: initialize a server project or add an initialized local one.
func TestEmptyProjectListShowsActions(t *testing.T) {
	a := testApp()
	a.projects = nil
	a.detail = nil
	a.selected = -1
	tt := ui.NewTester(a.view, 1320, 840)
	for _, text := range []string{"No projects yet", "Add server project", "Add local project"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

// TestWindowBusyCaption checks the window-level busy hint: the status bar shows
// a spinner and the label while a background operation runs, and neither after
// it finishes.
func TestWindowBusyCaption(t *testing.T) {
	a := testApp()
	a.beginBusy("Updating the updater package…")
	tt := ui.NewTester(a.view, 1320, 840)
	if !tt.HasText("Updating the updater package…") {
		t.Fatalf("the busy caption is not shown; texts=%q", tt.Texts())
	}
	a.endBusy()
	tt.Frame()
	if tt.HasText("Updating the updater package…") {
		t.Fatalf("the busy caption stayed after endBusy; texts=%q", tt.Texts())
	}
	if !tt.HasText("Ready") {
		t.Errorf("the status message did not come back; texts=%q", tt.Texts())
	}
}

// TestShortcutsMatchCSharpGUI verifies the window-level shortcuts introduced
// for feature parity with the C# GUI (F5 / Ctrl+Enter / Ctrl+E). The CLI is
// intentionally unavailable so every action takes its no-CLI path: no
// goroutines are started and a.win (nil in the tester) is never touched.
func TestShortcutsMatchCSharpGUI(t *testing.T) {
	t.Setenv("ALY_PUBLISH", "")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".updator"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".updator", "shared.json"),
		[]byte(`{"server_url":"http://localhost:2000","project_name":"demo"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	fresh := func() *app {
		a := testApp()
		a.cli = &cliClient{}
		a.projects[0].ProjectPath = dir
		a.detail.project.ProjectPath = dir
		return a
	}

	t.Run("F5 refreshes", func(t *testing.T) {
		a := fresh()
		tt := ui.NewTester(a.view, 1440, 900)
		tt.Key(0, ui.KeyF5)
		if a.detail.status != a.t("toolbar.cliMissing") {
			t.Fatalf("F5 did not trigger refresh: status=%q", a.detail.status)
		}
	})

	t.Run("Ctrl+E opens project settings", func(t *testing.T) {
		a := fresh()
		tt := ui.NewTester(a.view, 1440, 900)
		tt.Key(ui.Cmd, ui.KeyE)
		if !a.editOpen {
			t.Fatal("Ctrl+E did not open project settings")
		}
	})

	t.Run("Ctrl+Enter does not publish an incomplete form", func(t *testing.T) {
		a := fresh()
		tt := ui.NewTester(a.view, 1440, 900)
		tt.Key(ui.Cmd, ui.KeyEnter)
		if a.detail.uploading {
			t.Fatal("Ctrl+Enter started a publish without version/notes")
		}
	})

	t.Run("modal owns the keyboard", func(t *testing.T) {
		a := fresh()
		a.editOpen = true
		tt := ui.NewTester(a.view, 1440, 900)
		tt.Key(ui.Cmd, ui.KeyE)
		if a.editing != nil {
			t.Fatal("shortcut fired while a modal was open")
		}
	})
}

func TestCtrlFFocusesProjectSearch(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.view, 1320, 840)
	tt.Key(ui.Cmd, ui.KeyF)
	if !tt.Focused(a.t("projects.search")) {
		t.Fatal("Ctrl+F did not focus the project search field")
	}
	tt.Type("needle")
	if a.filter != "needle" {
		t.Fatalf("focused search did not receive input: filter=%q", a.filter)
	}
}

func TestViewRendersChinese(t *testing.T) {
	a := testApp()
	a.lang = "zh"
	tt := ui.NewTester(a.view, 1320, 840)
	for _, text := range []string{"项目", "未暂存", "已暂存", "发布", "版本历史"} {
		if !tt.HasText(text) {
			t.Errorf("Chinese view missing %q; texts=%q", text, tt.Texts())
		}
	}
}
