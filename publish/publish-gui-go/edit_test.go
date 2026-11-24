package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// editTestProjectDir creates a project folder whose .updator/shared.json is
// body, the shape the settings and add-local dialogs read.
func editTestProjectDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".updator"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".updator", "shared.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// editTestAppAt points the test app's visible project (bookmark and detail) at
// dir, as loading config.json would.
func editTestAppAt(t *testing.T, dir string) *app {
	t.Helper()
	a := testApp()
	a.projects[0].ProjectPath = dir
	a.detail.project.ProjectPath = dir
	return a
}

// accessOfName fails the test when no element with the role and accessible name
// exists, and returns what assistive technology sees of it.
func accessOfName(t *testing.T, tt *ui.Tester, role ui.Role, name string) ui.AccessInfo {
	t.Helper()
	info, ok := tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("no %v named %q; texts=%q", role, name, tt.Texts())
	}
	return info
}

// TestEditDialogSuggestionChipAddsRule extends the dialog's rule editing:
// the C# dialog offers one-click suggestions above the rule lists, and a click
// must append that rule to the matching list only.
func TestEditDialogSuggestionChipAddsRule(t *testing.T) {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{
		project:     a.detail.project,
		displayName: "demo",
		serverURL:   "http://127.0.0.1:2000",
		projectName: "demo",
	}
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("node_modules"); err != nil {
		t.Fatalf("clicking the folder suggestion failed: %v", err)
	}
	if len(a.editing.ignoreFolders) != 1 || a.editing.ignoreFolders[0] != "node_modules" {
		t.Fatalf("ignore folders = %v, want [node_modules]", a.editing.ignoreFolders)
	}
	if len(a.editing.ignoreFiles) != 0 {
		t.Fatalf("a folder suggestion leaked into the file list: %v", a.editing.ignoreFiles)
	}
	if err := tt.Click("*.log"); err != nil {
		t.Fatalf("clicking the file suggestion failed: %v", err)
	}
	if len(a.editing.ignoreFiles) != 1 || a.editing.ignoreFiles[0] != "*.log" {
		t.Fatalf("ignore files = %v, want [*.log]", a.editing.ignoreFiles)
	}
}

// TestEditDialogRuleInputSubmits extends dialog_test.go's "Add" button test to
// the keyboard path: Enter in a rule input appends the typed rule and clears
// the draft, as the C# TagInput does.
func TestEditDialogRuleInputSubmits(t *testing.T) {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{project: a.detail.project, displayName: "demo"}
	tt := ui.NewTester(a.view, 1440, 900)

	add, ok := tt.Find("Add")
	if !ok {
		t.Fatalf("the rule add button is missing; texts=%q", tt.Texts())
	}
	label, ok := tt.Find("Ignored folders")
	if !ok {
		t.Fatalf("the rule group label is missing; texts=%q", tt.Texts())
	}
	// The input shares its row with the Add button; the group's label marks the
	// left edge of the column, so this lands inside the input.
	tt.ClickAt(label.X+16, add.Y+add.H/2)
	tt.Type("obj")
	if a.editing.newIgnoreFolder != "obj" {
		t.Fatalf("typing did not reach the rule input: draft %q", a.editing.newIgnoreFolder)
	}
	tt.Key(0, ui.KeyEnter)
	if len(a.editing.ignoreFolders) != 1 || a.editing.ignoreFolders[0] != "obj" {
		t.Fatalf("Enter did not append the rule: %v", a.editing.ignoreFolders)
	}
	if a.editing.newIgnoreFolder != "" {
		t.Fatalf("the rule input was not cleared: %q", a.editing.newIgnoreFolder)
	}
}

// TestEditSaveDisabledUntilServersFieldsFilled checks the Save gating the C#
// dialog has: the server URL, server project and display name are required.
func TestEditSaveDisabledUntilServersFieldsFilled(t *testing.T) {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{project: a.detail.project}
	tt := ui.NewTester(a.view, 1440, 900)

	if save := accessOfName(t, tt, ui.RoleButton, "Save"); !save.Disabled {
		t.Fatal("Save is enabled with every required field empty")
	}
	if !tt.HasText("Server URL is required") {
		t.Fatalf("the missing server URL is not explained; texts=%q", tt.Texts())
	}

	a.editing.serverURL = "http://127.0.0.1:2000"
	a.editing.projectName = "demo"
	a.editing.displayName = "demo"
	tt.Frame()
	if save := accessOfName(t, tt, ui.RoleButton, "Save"); save.Disabled {
		t.Fatal("Save is disabled with a complete server target and display name")
	}

	a.editing.displayName = ""
	tt.Frame()
	if save := accessOfName(t, tt, ui.RoleButton, "Save"); !save.Disabled {
		t.Fatal("Save is enabled without a display name")
	}
}

// TestEditDialogLoadErrorBanner checks that a malformed shared.json opens the
// dialog with a banner instead of a toast, and keeps Save disabled.
func TestEditDialogLoadErrorBanner(t *testing.T) {
	dir := editTestProjectDir(t, `{"server_url": oops`)
	a := editTestAppAt(t, dir)
	a.openEdit()
	if !a.editOpen || a.editing == nil {
		t.Fatal("a malformed shared.json should still open the dialog")
	}
	if a.editing.loadError == "" {
		t.Fatal("the load error was not recorded")
	}
	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText(".updator/shared.json") {
		t.Fatalf("the load error banner is missing; texts=%q", tt.Texts())
	}
	if save := accessOfName(t, tt, ui.RoleButton, "Save"); !save.Disabled {
		t.Fatal("Save is enabled although the config could not be loaded")
	}
	if a.editing.busy {
		t.Fatal("the dialog is busy after a failed load")
	}
}

// TestEditMainExeHintText covers the three states of the C# MainExeHint: empty,
// a path that will be stored relative, and one that stays absolute because it
// is outside the project.
func TestEditMainExeHintText(t *testing.T) {
	dir := t.TempDir()
	a := testApp()
	e := &editState{project: ProjectConfig{ProjectPath: dir}}

	if got, want := a.mainExeHintText(e), a.t("edit.mainExeHint"); got != want {
		t.Fatalf("empty hint = %q, want %q", got, want)
	}

	e.mainExe = filepath.Join(dir, "bin", "app.exe")
	want := a.tf("edit.mainExeStored", "path", "bin/app.exe")
	if got := a.mainExeHintText(e); got != want {
		t.Fatalf("relative hint = %q, want %q", got, want)
	}

	e.mainExe = filepath.Join(filepath.Dir(dir), "elsewhere", "app.exe")
	if got, want := a.mainExeHintText(e), a.t("edit.mainExeOutside"); got != want {
		t.Fatalf("outside hint = %q, want %q", got, want)
	}
}

// TestEditKnownServerChipsFillField checks the one-click chips built from the
// other bookmarks' shared.json, as ProjectConfigService.GetKnownServerUrls does.
func TestEditKnownServerChipsFillField(t *testing.T) {
	dir := editTestProjectDir(t, `{"server_url":"http://server-a:2000","project_name":"demo"}`)
	a := editTestAppAt(t, dir)
	a.openEdit()
	if len(a.editing.knownServers) != 1 || a.editing.knownServers[0] != "http://server-a:2000" {
		t.Fatalf("known servers = %v", a.editing.knownServers)
	}
	a.editing.serverURL = ""
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("http://server-a:2000"); err != nil {
		t.Fatalf("clicking the known server chip failed: %v", err)
	}
	if a.editing.serverURL != "http://server-a:2000" {
		t.Fatalf("server URL = %q after choosing the chip", a.editing.serverURL)
	}
}

// TestAddLocalDialogEmptyPath checks the new entry point: the dialog opens with
// no folder chosen, explains what to pick and keeps Add disabled.
func TestAddLocalDialogEmptyPath(t *testing.T) {
	a := testApp()
	a.addLocalProject()
	if !a.localPreviewOpen || a.localPreview == nil {
		t.Fatal("addLocalProject did not open the dialog")
	}
	if a.localPreview.path != "" {
		t.Fatalf("the dialog started with a folder: %q", a.localPreview.path)
	}
	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText("Choose an initialized project folder") {
		t.Fatalf("the empty-folder hint is missing; texts=%q", tt.Texts())
	}
	if add := accessOfName(t, tt, ui.RoleButton, "Add project"); !add.Disabled {
		t.Fatal("Add project is enabled without a folder")
	}
}

// TestAddLocalDialogMissingFolder checks the live state line for a folder that
// does not exist.
func TestAddLocalDialogMissingFolder(t *testing.T) {
	a := testApp()
	a.addLocalProject()
	a.localPreview.path = filepath.Join(t.TempDir(), "missing")
	a.refreshLocalPreview(a.localPreview)
	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText("does not exist") {
		t.Fatalf("the missing-folder state is not shown; texts=%q", tt.Texts())
	}
	if add := accessOfName(t, tt, ui.RoleButton, "Add project"); !add.Disabled {
		t.Fatal("Add project is enabled for a missing folder")
	}
}

// TestAddLocalDialogInitializedFolderPreviews checks that a folder with
// .updator/shared.json is previewed and can be added, including the ignore-rule
// counts from both the ignore and the not-copy lists.
func TestAddLocalDialogInitializedFolderPreviews(t *testing.T) {
	body := `{"server_url":"http://server:2000","project_name":"demo",` +
		`"ignore_folders":["Log"],"ignore_files":["a.log"],` +
		`"un_copy_folders":["TestData"],"un_copy_files":["b.tmp"]}`
	dir := editTestProjectDir(t, body)
	a := testApp()
	a.addLocalProject()
	a.localPreview.path = dir
	a.refreshLocalPreview(a.localPreview)

	if !a.localPreview.loaded || a.localPreview.shared.ProjectName != "demo" {
		t.Fatalf("the config was not loaded: %+v", a.localPreview.shared)
	}
	if a.localPreview.displayName != "demo" {
		t.Fatalf("display name = %q, want the server project name", a.localPreview.displayName)
	}
	tt := ui.NewTester(a.view, 1440, 900)
	for _, text := range []string{"Configuration found", "http://server:2000", "Ignore rules", "2 folders · 2 files"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
	if add := accessOfName(t, tt, ui.RoleButton, "Add project"); add.Disabled {
		t.Fatal("Add project is disabled for a valid initialized folder")
	}
}

// TestAddLocalDialogDuplicateStaysDisabled checks that a folder that is already
// bookmarked is reported as such and cannot be added twice.
func TestAddLocalDialogDuplicateStaysDisabled(t *testing.T) {
	dir := editTestProjectDir(t, `{"server_url":"http://server:2000","project_name":"demo"}`)
	a := testApp()
	a.projects = append(a.projects, ProjectConfig{DisplayName: "demo", ProjectPath: dir})
	a.addLocalProject()
	a.localPreview.path = dir
	a.refreshLocalPreview(a.localPreview)
	if a.localPreview.loaded {
		t.Fatal("a bookmarked folder was loaded as a new project")
	}
	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText("already in the list") {
		t.Fatalf("the duplicate state is not shown; texts=%q", tt.Texts())
	}
	if add := accessOfName(t, tt, ui.RoleButton, "Add project"); !add.Disabled {
		t.Fatal("Add project is enabled for an already bookmarked folder")
	}
}

// TestAddLocalConfirmAddsProject checks the confirm path: the bookmark is
// written, the project list grows, and the dialog closes on the new project.
func TestAddLocalConfirmAddsProject(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ALYPUBLISH_CONFIG_DIR", configDir)
	dir := editTestProjectDir(t, `{"server_url":"http://server:2000","project_name":"demo"}`)
	a := testApp()
	a.cli = &cliClient{} // no CLI: adding must not spawn work through openSelected
	a.projects = nil
	a.detail = nil
	a.selected = -1
	a.addLocalProject()
	a.localPreview.path = dir
	a.localPreview.displayName = "demo"
	a.refreshLocalPreview(a.localPreview)

	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("Add project"); err != nil {
		t.Fatalf("clicking Add project failed: %v", err)
	}
	if a.localPreviewOpen || a.localPreview != nil {
		t.Fatal("the dialog stayed open after adding the project")
	}
	if len(a.projects) != 1 {
		t.Fatalf("projects = %v, want the new bookmark", a.projects)
	}
	if got, want := a.projects[0].ProjectPath, normalizeForCompare(dir); got != want {
		t.Fatalf("bookmarked path = %q, want %q", got, want)
	}
	if a.projects[0].DisplayName != "demo" {
		t.Fatalf("bookmarked name = %q", a.projects[0].DisplayName)
	}
	raw, err := os.ReadFile(filepath.Join(configDir, "config.json"))
	if err != nil {
		t.Fatalf("the bookmark file was not written: %v", err)
	}
	if !strings.Contains(string(raw), "demo") {
		t.Fatalf("the bookmark file does not hold the project: %s", raw)
	}
}

// TestDialogsFitMinimumWindow renders both dialogs at the smallest supported
// window (see main.go's MinWidth/MinHeight) and checks that the dialog and its
// footer actions stay inside it.
func TestDialogsFitMinimumWindow(t *testing.T) {
	const winW, winH = 1080, 640
	inside := func(r ui.Rect) bool {
		const slack = 0.5
		return r.X >= -slack && r.Y >= -slack &&
			r.X+r.W <= winW+slack && r.Y+r.H <= winH+slack
	}
	check := func(t *testing.T, tt *ui.Tester, labels ...string) {
		t.Helper()
		dialog := accessOfName(t, tt, ui.RoleDialog, "")
		if !inside(dialog.Bounds) {
			t.Errorf("dialog at %+v does not fit %vx%v", dialog.Bounds, winW, winH)
		}
		for _, label := range labels {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%q is missing; texts=%q", label, tt.Texts())
			}
			if !inside(r) {
				t.Errorf("%q at %+v does not fit %vx%v", label, r, winW, winH)
			}
		}
	}

	t.Run("project settings", func(t *testing.T) {
		a := testApp()
		a.editOpen = true
		a.editing = &editState{
			project:     a.detail.project,
			displayName: "demo",
			serverURL:   "http://127.0.0.1:2000",
			projectName: "demo",
		}
		tt := ui.NewTester(a.view, winW, winH)
		check(t, tt, "Project settings", "Cancel", "Save")
	})

	t.Run("add local project", func(t *testing.T) {
		dir := editTestProjectDir(t, `{"server_url":"http://server:2000","project_name":"demo"}`)
		a := testApp()
		a.addLocalProject()
		a.localPreview.path = dir
		a.refreshLocalPreview(a.localPreview)
		tt := ui.NewTester(a.view, winW, winH)
		check(t, tt, "Add local project", "Cancel", "Add project")
	})
}

func TestConfirmLocalProjectRejectsMalformedConfigWithoutBookmark(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	dir := editTestProjectDir(t, `{"server_url":broken`)
	a := testApp()
	a.cli = &cliClient{}
	a.projects = nil
	a.detail = nil
	a.selected = -1
	a.addLocalProject()
	a.localPreview.path = dir
	a.refreshLocalPreview(a.localPreview)
	a.confirmLocalProject()
	if len(a.projects) != 0 || a.selected != -1 {
		t.Fatalf("malformed config changed bookmarks: %+v selected=%d", a.projects, a.selected)
	}
	if !a.localPreviewOpen || a.localPreview.error == "" {
		t.Fatalf("dialog did not retain the error: open=%v state=%+v", a.localPreviewOpen, a.localPreview)
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Fatalf("bookmark file exists after rejected config: %v", err)
	}
}

func TestConfirmLocalProjectSaveFailureDoesNotChangeBookmarks(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(configFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALYPUBLISH_CONFIG_DIR", configFile)
	dir := editTestProjectDir(t, `{"server_url":"http://server:2000","project_name":"demo"}`)
	a := testApp()
	a.cli = &cliClient{}
	a.projects = nil
	a.detail = nil
	a.selected = -1
	a.addLocalProject()
	a.localPreview.path = dir
	a.localPreview.displayName = "demo"
	a.refreshLocalPreview(a.localPreview)
	a.confirmLocalProject()
	if len(a.projects) != 0 || a.selected != -1 {
		t.Fatalf("failed save changed bookmarks: %+v selected=%d", a.projects, a.selected)
	}
	if !a.localPreviewOpen || a.localPreview.error == "" {
		t.Fatalf("save failure was not retained in dialog: open=%v state=%+v", a.localPreviewOpen, a.localPreview)
	}
}
