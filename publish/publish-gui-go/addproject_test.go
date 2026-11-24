package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// newAddProjectApp opens the add-project wizard on its server step with a
// server URL already typed, so its fetch and create actions are enabled.
func newAddProjectApp() *app {
	a := testApp()
	a.addOpen = true
	a.adding = &addProjectState{serverURL: "http://127.0.0.1:2000", selected: -1}
	return a
}

// TestAddProjectWizardFetchStartsSynchronously is the regression test for the
// fetching flag: it is set before the goroutine starts, so the page can show
// its spinner and disable the button without waiting for a frame.
func TestAddProjectWizardFetchStartsSynchronously(t *testing.T) {
	a := newAddProjectApp()
	a.cli = &cliClient{} // unavailable: the fetch goroutine fails at once
	fetchingSeen := false
	addProjectFetchStartedHook = func(s *addProjectState) { fetchingSeen = s.fetching }
	t.Cleanup(func() { addProjectFetchStartedHook = nil })

	tt := ui.NewTester(a.view, 1320, 840)
	if err := tt.Click("Fetch projects"); err != nil {
		t.Fatalf("clicking Fetch projects failed: %v", err)
	}
	if !fetchingSeen {
		t.Fatal("fetching was not set synchronously, before the fetch goroutine started")
	}
}

// TestAddProjectWizardTwoStepNavigation walks the C# flow: no project selected
// means Next stays disabled; selecting a row enables it and Next shows the
// local step fields.
func TestAddProjectWizardTwoStepNavigation(t *testing.T) {
	a := newAddProjectApp()
	a.adding.allProjects = []ProjectInfo{
		{ID: 7, Name: "alpha", Title: "Alpha", Version: "V1.0.2"},
	}
	tt := ui.NewTester(a.view, 1320, 840)

	if a.adding.canNext() {
		t.Fatal("Next must stay disabled without a selected project")
	}
	// A disabled Next must not advance the wizard even if the click lands.
	_ = tt.Click("Next")
	if a.adding.stage != addStageServer {
		t.Fatal("Next advanced the wizard without a selection")
	}
	if !tt.HasText("ID 7") {
		t.Errorf("the project row does not show its ID; texts=%q", tt.Texts())
	}

	if err := tt.Click("alpha"); err != nil {
		t.Fatalf("clicking the project row failed: %v", err)
	}
	if a.adding.selected != 0 {
		t.Fatalf("selected = %d after clicking the row, want 0", a.adding.selected)
	}
	if a.adding.projectName != "alpha" || a.adding.displayName != "alpha" {
		t.Fatalf("row click did not fill the project: name=%q displayName=%q",
			a.adding.projectName, a.adding.displayName)
	}
	if !a.adding.canNext() {
		t.Fatal("Next is still disabled after selecting a project")
	}

	if err := tt.Click("Next"); err != nil {
		t.Fatalf("clicking Next failed: %v", err)
	}
	if a.adding.stage != addStageLocal {
		t.Fatalf("stage = %v after Next, want addStageLocal", a.adding.stage)
	}
	for _, text := range []string{"Local folder", "Close process name", "Initialize and add", "Back", "alpha"} {
		if !tt.HasText(text) {
			t.Errorf("local step is missing %q; texts=%q", text, tt.Texts())
		}
	}
}

// TestAddProjectWizardCreateRequiresName checks the create sub-page: an empty
// name shows a validation error and clicking Create starts no work.
func TestAddProjectWizardCreateRequiresName(t *testing.T) {
	a := newAddProjectApp()
	a.adding.stage = addStageCreate
	a.adding.allProjects = []ProjectInfo{{ID: 1, Name: "alpha", Title: "Alpha"}}
	tt := ui.NewTester(a.view, 1320, 840)

	if !tt.HasText("Enter a project name") {
		t.Errorf("the create page does not show its validation; texts=%q", tt.Texts())
	}
	if err := tt.Click("Create project"); err != nil {
		t.Fatalf("clicking Create project failed: %v", err)
	}
	if a.adding.creating {
		t.Fatal("Create started a CLI call with an empty project name")
	}
	if a.adding.actionError == "" || a.adding.validation == "" {
		t.Fatalf("Create set no error: actionError=%q validation=%q",
			a.adding.actionError, a.adding.validation)
	}
	// A duplicate name is rejected too, as the C# dialog validates.
	a.adding.createTitle = "Alpha"
	a.adding.createName = "alpha"
	if msg := a.createValidation(a.adding); msg != a.t("addProject.createDuplicate") {
		t.Fatalf("duplicate validation = %q", msg)
	}
}

// TestAddProjectWizardCreateNameFollowsTitle reproduces the C# "identifier
// follows the title until the user edits it" rule.
func TestAddProjectWizardCreateNameFollowsTitle(t *testing.T) {
	a := newAddProjectApp()
	a.adding.stage = addStageCreate
	tt := ui.NewTester(a.view, 1320, 840)
	// The title input is auto-focused on the create page.
	tt.Type("Daily build")
	if a.adding.createTitle != "Daily build" || a.adding.createName != "Daily build" {
		t.Fatalf("title=%q name=%q, want both to follow typing",
			a.adding.createTitle, a.adding.createName)
	}
	if a.adding.createNameEdited {
		t.Fatal("auto-filling the name must not mark it as edited")
	}
}

// TestAddProjectWizardRulesAddChip adds a rule on the rules sub-page and
// checks that Done returns to the local step.
func TestAddProjectWizardRulesAddChip(t *testing.T) {
	a := newAddProjectApp()
	a.adding.stage = addStageRules
	a.adding.newIgnoreFolder = "Log"
	tt := ui.NewTester(a.view, 1320, 840)

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("clicking the rule Add button failed: %v", err)
	}
	if len(a.adding.ignoreFolders) != 1 || a.adding.ignoreFolders[0] != "Log" {
		t.Fatalf("ignore folders = %v, want [Log]", a.adding.ignoreFolders)
	}
	if a.adding.newIgnoreFolder != "" {
		t.Fatalf("the rule input was not cleared: %q", a.adding.newIgnoreFolder)
	}
	if err := tt.Click("Done"); err != nil {
		t.Fatalf("clicking Done failed: %v", err)
	}
	if a.adding.stage != addStageLocal {
		t.Fatalf("stage = %v after Done, want addStageLocal", a.adding.stage)
	}
}

// TestAddProjectWizardPathStates covers the five folder verdicts: each one is
// shown live under the folder field and blocks initialization.
func TestAddProjectWizardPathStates(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	initialized := filepath.Join(root, "initialized")
	usable := filepath.Join(root, "usable")
	listed := filepath.Join(root, "listed")
	for _, dir := range []string{initialized, usable, listed} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeShared(t, initialized, "http://server:2000", "initialized")
	projects := []ProjectConfig{{DisplayName: "listed", ProjectPath: listed}}

	cases := []struct {
		name string
		path string
		want pathState
	}{
		{"empty", "", pathEmpty},
		{"missing", missing, pathMissing},
		{"initialized", initialized, pathInitialized},
		{"listed", listed, pathListed},
		{"usable", usable, pathUsable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := projectPathState(tc.path, projects); got != tc.want {
				t.Fatalf("projectPathState(%q) = %v, want %v", tc.path, got, tc.want)
			}
			a := testApp()
			a.projects = projects
			a.addOpen = true
			a.adding = &addProjectState{
				stage:       addStageLocal,
				serverURL:   "http://127.0.0.1:2000",
				allProjects: []ProjectInfo{{ID: 1, Name: "alpha", Title: "Alpha"}},
				selected:    0,
				path:        tc.path,
				displayName: "alpha",
			}
			tt := ui.NewTester(a.view, 1080, 640)
			want := pathStateText("en", tc.want)
			if !tt.HasText(want) {
				t.Errorf("local step does not show %q; texts=%q", want, tt.Texts())
			}
			// A usable folder is the only state without a validation error.
			wantValidation := want
			if tc.want == pathUsable {
				wantValidation = ""
			}
			if got := a.localValidation(a.adding); got != wantValidation {
				t.Errorf("localValidation = %q, want %q", got, wantValidation)
			}
		})
	}
}

// TestAddProjectWizardInitializeValidation checks that a usable folder with a
// display name and an inside executable passes validation, and that an outside
// executable is refused.
func TestAddProjectWizardInitializeValidation(t *testing.T) {
	root := t.TempDir()
	usable := filepath.Join(root, "app")
	if err := os.MkdirAll(usable, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(usable, "demo.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.exe")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := testApp()
	a.projects = nil
	a.addOpen = true
	a.adding = &addProjectState{
		stage:       addStageLocal,
		serverURL:   "http://127.0.0.1:2000",
		allProjects: []ProjectInfo{{ID: 1, Name: "alpha", Title: "Alpha"}},
		selected:    0,
		path:        usable,
		displayName: "alpha",
		mainExe:     "demo.exe",
	}
	if got := a.localValidation(a.adding); got != "" {
		t.Fatalf("usable folder with an inside executable failed validation: %q", got)
	}
	a.adding.mainExe = outside
	if got := a.localValidation(a.adding); got != a.t("addProject.mainOutside") {
		t.Fatalf("outside executable validation = %q", got)
	}
}

// TestAddProjectWizardEscapeGoesBackOneStep checks the wizard's Escape rule:
// a sub-page goes back, the first page closes the dialog.
func TestAddProjectWizardEscapeGoesBackOneStep(t *testing.T) {
	a := newAddProjectApp()
	a.adding.allProjects = []ProjectInfo{{ID: 1, Name: "alpha"}}
	a.adding.selected = 0
	tt := ui.NewTester(a.view, 1320, 840)

	a.adding.stage = addStageLocal
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if !a.addOpen || a.adding == nil || a.adding.stage != addStageServer {
		t.Fatal("Escape on the local step did not go back to the server step")
	}

	a.adding.stage = addStageRules
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if !a.addOpen || a.adding.stage != addStageLocal {
		t.Fatal("Escape on the rules page did not go back to the local step")
	}

	a.adding.stage = addStageCreate
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if !a.addOpen || a.adding.stage != addStageServer {
		t.Fatal("Escape on the create page did not go back to the server step")
	}

	tt.Key(0, ui.KeyEscape)
	if a.addOpen {
		t.Fatal("Escape on the server step did not close the wizard")
	}
}

// TestAddProjectWizardBackdropKeepsOpen is the pointer regression from the
// dialog backdrop: a press beside the panel must not close the dialog.
func TestAddProjectWizardBackdropKeepsOpen(t *testing.T) {
	a := newAddProjectApp()
	tt := ui.NewTester(a.view, 1320, 840)
	tt.ClickAt(4, 4)
	if !a.addOpen || a.adding == nil {
		t.Fatal("clicking the backdrop closed the add-project wizard")
	}
}

// TestAddProjectWizardFitsSmallWindow checks the wizard on the 1080x640
// minimum window: the panel renders inside the window on both steps.
func TestAddProjectWizardFitsSmallWindow(t *testing.T) {
	const width, height = 1080, 640
	a := newAddProjectApp()
	a.adding.allProjects = []ProjectInfo{
		{ID: 3, Name: "alpha", Title: "Alpha", Version: "V1.0.2", ForceUpdate: true},
	}
	tt := ui.NewTester(a.view, width, height)
	for _, text := range []string{"Add server project", "Server URL", "Fetch projects", "Create project", "alpha", "V1.0.2"} {
		if !tt.HasText(text) {
			t.Errorf("server step is missing %q at 1080x640; texts=%q", text, tt.Texts())
		}
	}
	r, ok := tt.Find("Add server project")
	if !ok {
		t.Fatal("the wizard's title is not rendered")
	}
	if r.X < 0 || r.Y < 0 || r.X+r.W > width || r.Y+r.H > height {
		t.Fatalf("the wizard title is outside the 1080x640 window: %+v", r)
	}
	if img := tt.Image(); img == nil {
		t.Fatal("no rendered image at 1080x640")
	}

	a.adding.stage = addStageLocal
	a.adding.selected = 0
	tt.Frame()
	for _, text := range []string{"Local folder", "Display name", "Initialize and add"} {
		if !tt.HasText(text) {
			t.Errorf("local step is missing %q at 1080x640; texts=%q", text, tt.Texts())
		}
	}
}

func TestFetchServerProjectsSuccessAndSelectionClamp(t *testing.T) {
	a := newAddProjectApp()
	cli, logPath := newHelperCLI(t, "success")
	a.cli = cli
	t.Setenv("ALY_GUI_TEST_PROJECT_LIST_JSON", `[
		{"id":1,"name":"alpha","title":"Alpha","version":"V1.0.0"},
		{"id":2,"name":"beta","title":"Beta","version":"V1.1.0","force_update":true}
	]`)
	a.adding.projectName = "beta"
	s := a.adding
	a.fetchServerProjects()
	waitForTestCondition(t, "server project fetch", func() bool {
		return !s.fetching && len(s.allProjects) == 2 && s.selected == 1 &&
			s.projectName == "beta" && s.displayName == "beta" && s.forceUpdate
	})
	if s.selected != 1 || s.projectName != "beta" || s.displayName != "beta" || !s.forceUpdate {
		t.Fatalf("selected project was not restored: %+v", s)
	}
	s.selected = 99
	s.clampSelection()
	if s.selected != 1 {
		t.Fatalf("selection above list = %d, want 1", s.selected)
	}
	s.filter = "no-match"
	s.clampSelection()
	if s.selected != -1 {
		t.Fatalf("empty filtered selection = %d, want -1", s.selected)
	}
	calls := readHelperCalls(t, logPath)
	if len(calls) != 1 || strings.Join(calls[0].Args, " ") != "project list --server http://127.0.0.1:2000" {
		t.Fatalf("fetch args = %+v", calls)
	}
}

func TestFetchServerProjectsFailureClearsSelection(t *testing.T) {
	a := newAddProjectApp()
	cli, _ := newHelperCLI(t, "business-failure")
	a.cli = cli
	s := a.adding
	s.allProjects = []ProjectInfo{{ID: 1, Name: "stale"}}
	s.selected = 0
	a.fetchServerProjects()
	waitForTestCondition(t, "server project fetch failure", func() bool {
		return !s.fetching && s.fetchError != ""
	})
	if s.allProjects != nil || s.selected != -1 || !strings.Contains(s.fetchError, "business rejected") {
		t.Fatalf("failed fetch retained stale state: %+v", s)
	}
}

func TestFetchServerProjectsEmptyList(t *testing.T) {
	a := newAddProjectApp()
	cli, _ := newHelperCLI(t, "success")
	a.cli = cli
	t.Setenv("ALY_GUI_TEST_PROJECT_LIST_JSON", `[]`)
	s := a.adding
	s.selected = 0
	a.fetchServerProjects()
	waitForTestCondition(t, "empty server project fetch", func() bool {
		return !s.fetching
	})
	if len(s.allProjects) != 0 || s.selected != -1 || s.canNext() {
		t.Fatalf("empty fetch state: projects=%v selected=%d", s.allProjects, s.selected)
	}
}

func TestCreateServerProjectSuccessFetchesAndSelects(t *testing.T) {
	a := newAddProjectApp()
	cli, logPath := newHelperCLI(t, "success")
	a.cli = cli
	t.Setenv("ALY_GUI_TEST_PROJECT_NAME", "new-project")
	t.Setenv("ALY_GUI_TEST_PROJECT_LIST_JSON", `[{"id":7,"name":"new-project","title":"new-project","version":"V1.0.0","force_update":true}]`)
	s := a.adding
	s.stage = addStageCreate
	s.createName = " new-project "
	s.createForceUpdate = true
	s.createIgnoreFolders = []string{"bin"}
	s.createIgnoreFiles = []string{"*.log"}
	a.createServerProject()
	waitForTestCondition(t, "create and subsequent fetch", func() bool {
		return !s.creating && !s.fetching && s.stage == addStageServer && len(s.allProjects) == 1
	})
	if s.projectName != "new-project" || s.title != "new-project" || s.displayName != "new-project" || s.selected != 0 || !s.forceUpdate {
		t.Fatalf("created project was not selected: %+v", s)
	}
	calls := readHelperCalls(t, logPath)
	if len(calls) != 2 {
		t.Fatalf("create calls = %+v, want create then list", calls)
	}
	want := "project create --server http://127.0.0.1:2000 --name new-project --title new-project --force-update --ignore-folders bin --ignore-files *.log"
	if got := strings.Join(calls[0].Args, " "); got != want {
		t.Fatalf("create args = %q, want %q", got, want)
	}
}

func TestCreateServerProjectFailureStaysOnCreatePage(t *testing.T) {
	a := newAddProjectApp()
	cli, logPath := newHelperCLI(t, "business-failure")
	a.cli = cli
	s := a.adding
	s.stage = addStageCreate
	s.createName = "new-project"
	a.createServerProject()
	waitForTestCondition(t, "create failure", func() bool {
		return !s.creating && s.actionError != ""
	})
	if s.stage != addStageCreate || !strings.Contains(s.actionError, "business rejected") {
		t.Fatalf("create failure state = %+v", s)
	}
	if calls := readHelperCalls(t, logPath); len(calls) != 1 {
		t.Fatalf("failed create triggered a fetch: %+v", calls)
	}
}

func prepareInitializeAddProject(a *app, path string) {
	s := a.adding
	s.stage = addStageLocal
	s.path = path
	s.displayName = "alpha"
	s.allProjects = []ProjectInfo{{ID: 1, Name: "alpha", Title: "Alpha"}}
	s.selected = 0
}

func TestInitializeAddProjectSuccess(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	a := newAddProjectApp()
	a.projects = nil
	a.detail = nil
	a.selected = -1
	cli, logPath := newHelperCLI(t, "success")
	a.cli = cli
	projectDir := t.TempDir()
	prepareInitializeAddProject(a, projectDir)
	s := a.adding
	a.initializeAddProject()
	waitForTestCondition(t, "initialize and initial refresh", func() bool {
		return !s.busy && !a.addOpen && a.detail != nil && a.detail.loaded && !a.detail.busy
	})
	if len(a.projects) != 1 || a.projects[0].ProjectPath != projectDir || a.selected != 0 || a.adding != nil {
		t.Fatalf("initialized state: projects=%+v selected=%d adding=%+v", a.projects, a.selected, a.adding)
	}
	if got := loadProjects(); len(got) != 1 || got[0].DisplayName != "alpha" {
		t.Fatalf("persisted bookmarks = %+v", got)
	}
	calls := readHelperCalls(t, logPath)
	if len(calls) == 0 || !strings.HasPrefix(strings.Join(calls[0].Args, " "), "config init --server http://127.0.0.1:2000 --project alpha") {
		t.Fatalf("initialize calls = %+v", calls)
	}
}

func TestInitializeAddProjectCLIFailureDoesNotAddBookmark(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	a := newAddProjectApp()
	cli, _ := newHelperCLI(t, "business-failure")
	a.cli = cli
	prepareInitializeAddProject(a, t.TempDir())
	s := a.adding
	a.initializeAddProject()
	waitForTestCondition(t, "initialize CLI failure", func() bool {
		return !s.busy && s.actionError != ""
	})
	if len(a.projects) != 1 || a.projects[0].DisplayName != "demo" || !a.addOpen || a.adding != s {
		t.Fatalf("CLI failure changed project state: projects=%+v open=%v", a.projects, a.addOpen)
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Fatalf("CLI failure wrote bookmarks: %v", err)
	}
}

func TestInitializeAddProjectUpdateFolderFailureDoesNotAddBookmark(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	a := newAddProjectApp()
	cli, _ := newHelperCLI(t, "success")
	a.cli = cli
	base := t.TempDir()
	projectDir := filepath.Join(base, "App")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "demo.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "UpdateFolder"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepareInitializeAddProject(a, projectDir)
	a.adding.mainExe = "demo.exe"
	s := a.adding
	a.initializeAddProject()
	waitForTestCondition(t, "initialize UpdateFolder failure", func() bool {
		return !s.busy && s.actionError != ""
	})
	if len(a.projects) != 1 || !a.addOpen || a.adding != s {
		t.Fatalf("UpdateFolder failure changed project state: projects=%+v open=%v", a.projects, a.addOpen)
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Fatalf("UpdateFolder failure wrote bookmarks: %v", err)
	}
}

func TestInitializeAddProjectBookmarkFailureKeepsState(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(configFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALYPUBLISH_CONFIG_DIR", configFile)
	a := newAddProjectApp()
	cli, _ := newHelperCLI(t, "success")
	a.cli = cli
	prepareInitializeAddProject(a, t.TempDir())
	s := a.adding
	a.initializeAddProject()
	waitForTestCondition(t, "initialize bookmark failure", func() bool {
		return !s.busy && s.actionError != ""
	})
	if len(a.projects) != 1 || a.projects[0].DisplayName != "demo" || a.selected != 0 || !a.addOpen || a.adding != s {
		t.Fatalf("bookmark failure changed project state: projects=%+v selected=%d open=%v", a.projects, a.selected, a.addOpen)
	}
}
