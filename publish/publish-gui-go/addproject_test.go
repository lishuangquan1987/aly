package main

import (
	"os"
	"path/filepath"
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
