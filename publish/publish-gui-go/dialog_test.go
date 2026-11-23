package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// newEditDialogApp returns a test app with the project settings dialog open and
// its server fields filled in, so its footer actions are enabled.
func newEditDialogApp() *app {
	a := testApp()
	a.editOpen = true
	a.editing = &editState{
		project:     a.detail.project,
		shared:      SharedConfig{ServerURL: "http://127.0.0.1:2000", ProjectName: "demo"},
		serverURL:   "http://127.0.0.1:2000",
		projectName: "demo",
		displayName: "demo",
	}
	return a
}

// A form dialog's own buttons must receive the pointer: the backdrop is not
// allowed to take input on behalf of its children (see formModal).
func TestEditDialogCancelButtonCloses(t *testing.T) {
	a := newEditDialogApp()
	tt := ui.NewTester(a.view, 1320, 840)
	if err := tt.Click("Cancel"); err != nil {
		t.Fatalf("clicking the dialog's close button failed: %v", err)
	}
	if a.editOpen {
		t.Fatal("the dialog stayed open after its close button was clicked")
	}
}

// The rule chips' remove and add buttons live inside the dialog and must work
// too; this is the same input routing regression as the footer buttons.
func TestEditDialogRuleAddButtonResponds(t *testing.T) {
	a := newEditDialogApp()
	a.editing.newIgnoreFolder = "Log"
	tt := ui.NewTester(a.view, 1320, 840)
	if err := tt.Click("Add"); err != nil {
		t.Fatalf("clicking the add button failed: %v", err)
	}
	if len(a.editing.ignoreFolders) != 1 || a.editing.ignoreFolders[0] != "Log" {
		t.Fatalf("ignore folder was not added: %v", a.editing.ignoreFolders)
	}
	if a.editing.newIgnoreFolder != "" {
		t.Fatalf("the rule input was not cleared: %q", a.editing.newIgnoreFolder)
	}
}

// Typing into a field of a form dialog must keep working: the buttons were
// broken while text fields still took the keyboard.
func TestEditDialogFieldAcceptsTyping(t *testing.T) {
	a := newEditDialogApp()
	tt := ui.NewTester(a.view, 1320, 840)
	// The server URL field is the dialog's AutoFocus target.
	tt.Type("x")
	if a.editing.serverURL != "http://127.0.0.1:2000x" {
		t.Fatalf("typed text did not reach the first field: %q", a.editing.serverURL)
	}
}
