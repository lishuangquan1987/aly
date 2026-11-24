package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeEditShared(t *testing.T, body string) string {
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

func TestApplyEditFormWritesOnlyDiffsInOrder(t *testing.T) {
	dir := writeEditShared(t, `{
		"server_url":"http://old","project_name":"old-project",
		"ignore_folders":["keep","drop"],"ignore_files":["same"],
		"un_copy_folders":["old-copy"],"un_copy_files":["same-copy"]
	}`)
	cli, logPath := newHelperCLI(t, "success")
	t.Setenv("ALY_GUI_TEST_PROJECT_NAME", "new-project")
	a := testApp()
	syncWarning, updaterWarning, storedMain, err := a.applyEditForm(
		context.Background(), cli, "en", dir,
		"http://new", "new-project", "",
		[]string{"keep", "add"}, []string{"same"}, nil, []string{"same-copy"},
	)
	if err != nil || syncWarning != "" || updaterWarning != "" || storedMain != "" {
		t.Fatalf("applyEditForm = sync %q updater %q main %q err %v", syncWarning, updaterWarning, storedMain, err)
	}
	calls := readHelperCalls(t, logPath)
	got := make([]string, len(calls))
	for i := range calls {
		got[i] = strings.Join(calls[i].Args, " ")
	}
	want := []string{
		"config set server.url http://new",
		"config set-array ignore.folders --remove drop",
		"config set-array ignore.folders --add add",
		"config set-array un_copy.folders --remove old-copy",
		"config set project.name new-project",
		"project list --server http://new",
		"project update --server http://new --name new-project --title Server title --force-update --ignore-folders keep,add --ignore-files same",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CLI calls:\n got %q\nwant %q", got, want)
	}
}

func TestApplyEditFormNoChangesSkipsCLI(t *testing.T) {
	dir := writeEditShared(t, `{"server_url":"http://same","project_name":"demo","ignore_folders":["keep"],"ignore_files":["same"]}`)
	cli, logPath := newHelperCLI(t, "success")
	a := testApp()
	syncWarning, updaterWarning, _, err := a.applyEditForm(
		context.Background(), cli, "en", dir,
		"http://same", "demo", "",
		[]string{"keep"}, []string{"same"}, nil, nil,
	)
	if err != nil || syncWarning != "" || updaterWarning != "" {
		t.Fatalf("applyEditForm = %q %q %v", syncWarning, updaterWarning, err)
	}
	if calls := readHelperCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("unchanged form invoked CLI: %+v", calls)
	}
}

func TestApplyEditFormStopsAtFailedDifference(t *testing.T) {
	dir := writeEditShared(t, `{"server_url":"http://same","project_name":"demo","ignore_folders":["drop"]}`)
	cli, logPath := newHelperCLI(t, "success")
	t.Setenv("ALY_GUI_TEST_FAIL_ON_CALL", "2")
	a := testApp()
	_, _, _, err := a.applyEditForm(
		context.Background(), cli, "en", dir,
		"http://same", "demo", "",
		[]string{"add"}, nil, nil, nil,
	)
	if err == nil || !strings.Contains(err.Error(), "injected call failure") {
		t.Fatalf("error = %v", err)
	}
	calls := readHelperCalls(t, logPath)
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want remove then failed add only", len(calls))
	}
	if got := strings.Join(calls[0].Args, " "); got != "config set-array ignore.folders --remove drop" {
		t.Fatalf("first call = %q", got)
	}
	if got := strings.Join(calls[1].Args, " "); got != "config set-array ignore.folders --add add" {
		t.Fatalf("second call = %q", got)
	}
}

func TestApplyEditFormMissingServerProjectIsWarning(t *testing.T) {
	dir := writeEditShared(t, `{"server_url":"http://same","project_name":"missing","ignore_folders":["drop"]}`)
	cli, logPath := newHelperCLI(t, "success")
	t.Setenv("ALY_GUI_TEST_PROJECT_NAME", "some-other-project")
	a := testApp()
	syncWarning, _, _, err := a.applyEditForm(
		context.Background(), cli, "en", dir,
		"http://same", "missing", "",
		[]string{"add"}, nil, nil, nil,
	)
	if err != nil || !strings.Contains(syncWarning, "Server project not found") {
		t.Fatalf("syncWarning = %q err = %v", syncWarning, err)
	}
	calls := readHelperCalls(t, logPath)
	for _, call := range calls {
		if len(call.Args) > 1 && call.Args[0] == "project" && call.Args[1] == "update" {
			t.Fatalf("missing server project was updated: %+v", call.Args)
		}
	}
}

func TestCleanRuleList(t *testing.T) {
	got := cleanRuleList([]string{"  alpha  ", "", "alpha", "beta", "Alpha", " beta "})
	want := []string{"alpha", "beta", "Alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanRuleList = %v, want %v", got, want)
	}
}

func prepareSaveEditApp(t *testing.T, dir string) *app {
	t.Helper()
	a := testApp()
	a.projects[0].ProjectPath = dir
	a.projects[0].DisplayName = "demo"
	a.detail.project = a.projects[0]
	a.editOpen = true
	a.editing = &editState{
		project:     a.projects[0],
		displayName: "Renamed",
		serverURL:   "http://same",
		projectName: "demo",
	}
	return a
}

func TestSaveEditSuccessPersistsAndCloses(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	dir := writeEditShared(t, `{"server_url":"http://same","project_name":"demo"}`)
	cli, _ := newHelperCLI(t, "success")
	a := prepareSaveEditApp(t, dir)
	a.cli = cli
	e := a.editing
	a.saveEdit()
	refreshed := a.tf("status.refreshed", "unstaged", "0", "staged", "0")
	waitForTestCondition(t, "save edit and refresh", func() bool {
		return !e.busy && !a.editOpen && a.detail.loaded && !a.detail.busy &&
			a.projects[0].DisplayName == "Renamed" && a.detail.status == refreshed
	})
	if a.projects[0].DisplayName != "Renamed" || a.detail.project.DisplayName != "Renamed" {
		t.Fatalf("saved project state = %+v detail=%+v", a.projects[0], a.detail.project)
	}
	if got := loadProjects(); len(got) != 1 || got[0].DisplayName != "Renamed" {
		t.Fatalf("persisted projects = %+v", got)
	}
	if a.status != a.t("edit.saved") || a.statusTone != toneSuccess {
		t.Fatalf("save status = %q tone=%v", a.status, a.statusTone)
	}
}

func TestSaveEditCLIFailureStaysOpen(t *testing.T) {
	t.Setenv("ALYPUBLISH_CONFIG_DIR", t.TempDir())
	dir := writeEditShared(t, `{"server_url":"http://same","project_name":"demo","ignore_folders":["drop"]}`)
	cli, _ := newHelperCLI(t, "business-failure")
	a := prepareSaveEditApp(t, dir)
	a.cli = cli
	a.editing.ignoreFolders = []string{"add"}
	e := a.editing
	a.saveEdit()
	waitForTestCondition(t, "save edit CLI failure", func() bool {
		return !e.busy && e.error != ""
	})
	if !a.editOpen || a.editing != e || a.projects[0].DisplayName != "demo" {
		t.Fatalf("CLI failure changed dialog/project state: open=%v project=%+v", a.editOpen, a.projects[0])
	}
	if !strings.Contains(e.error, "business rejected") {
		t.Fatalf("error = %q", e.error)
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Fatalf("CLI failure wrote bookmarks: %v", err)
	}
}

func TestSaveEditBookmarkFailureKeepsProjectState(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(configFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALYPUBLISH_CONFIG_DIR", configFile)
	dir := writeEditShared(t, `{"server_url":"http://same","project_name":"demo"}`)
	cli, _ := newHelperCLI(t, "success")
	a := prepareSaveEditApp(t, dir)
	a.cli = cli
	e := a.editing
	a.saveEdit()
	waitForTestCondition(t, "save edit bookmark failure", func() bool {
		return !e.busy && e.error != ""
	})
	if !a.editOpen || a.editing != e {
		t.Fatalf("bookmark failure closed dialog: open=%v", a.editOpen)
	}
	if a.projects[0].DisplayName != "demo" || a.detail.project.DisplayName != "demo" {
		t.Fatalf("bookmark failure changed in-memory project: projects=%+v detail=%+v", a.projects[0], a.detail.project)
	}
}
