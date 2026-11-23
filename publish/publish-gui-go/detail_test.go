package main

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// detailTestApp is a window-less app whose CLI is unavailable, so the detail
// page tests never start a process or a goroutine.
func detailTestApp() *app {
	project := ProjectConfig{DisplayName: "demo", ProjectPath: `C:\demo`, MainExePath: "demo.exe"}
	a := &app{
		lang:     "en",
		theme:    "light",
		accent:   "violet",
		status:   "Ready",
		projects: []ProjectConfig{project},
		selected: 0,
		cli:      &cliClient{},
		details:  map[string]*detailState{},
	}
	a.detail = &detailState{
		project:        project,
		currentVersion: "V1.0.0",
		forceKnown:     true,
		splitPercent:   62,
		splitW:         detailDefaultSide,
		loaded:         true,
		unstaged: []fileItem{
			{RelativePath: "app.exe", Status: "modified", LocalSize: 2048, ServerSize: 1024},
		},
		staged: []fileItem{
			{RelativePath: "old.dll", Status: "deleted", LocalSize: 0, ServerSize: 512},
		},
		logs: []ChangeLog{
			{Version: "V1.0.0", Time: "2026-11-13 10:00:00", Logs: []string{"first release"}},
		},
	}
	a.details[normalizePath(project.ProjectPath)] = a.detail
	return a
}

// TestDetailInitialLoadingSkeleton checks the C# first-load overlay: the
// reading caption is on screen while initialLoading is set, and an attempted
// refresh with no CLI ends the skeleton instead of leaving it stuck.
func TestDetailInitialLoadingSkeleton(t *testing.T) {
	a := detailTestApp()
	a.detail.initialLoading = true
	a.detail.loaded = false
	tt := ui.NewTester(a.view, 1440, 900)

	reading := a.t("changes.reading")
	if !tt.HasText(reading) {
		t.Fatalf("first-load skeleton missing %q; texts=%q", reading, tt.Texts())
	}

	a.refreshDetail()
	if a.detail.status != a.t("toolbar.cliMissing") {
		t.Fatalf("status=%q want %q", a.detail.status, a.t("toolbar.cliMissing"))
	}
	if a.detail.initialLoading || !a.detail.loaded {
		t.Fatalf("refresh attempt left initialLoading=%v loaded=%v", a.detail.initialLoading, a.detail.loaded)
	}
	tt.Frame()
	if tt.HasText(reading) {
		t.Fatalf("skeleton still visible after the refresh attempt; texts=%q", tt.Texts())
	}
}

// TestDetailPublishButtonTextAndHint covers the C# PublishButtonText and
// PublishHint rules: the count form when files are staged and the missing
// release-notes hint.
func TestDetailPublishButtonTextAndHint(t *testing.T) {
	a := detailTestApp()
	a.detail.staged = []fileItem{
		{RelativePath: "one.dll", Status: "new"},
		{RelativePath: "two.dll", Status: "modified"},
	}
	a.detail.newVersion = "V1.0.2"
	a.detail.notes = ""
	tt := ui.NewTester(a.view, 1440, 900)

	want := a.tf("publish.publishCount", "count", "2")
	if !tt.HasText(want) {
		t.Fatalf("publish button text missing %q; texts=%q", want, tt.Texts())
	}
	if !tt.HasText(a.t("publish.needNotes")) {
		t.Fatalf("missing-notes hint absent; texts=%q", tt.Texts())
	}

	a.detail.notes = "fixed the crash"
	tt.Frame()
	if tt.HasText(a.t("publish.needNotes")) {
		t.Fatalf("hint still shown after notes were filled; texts=%q", tt.Texts())
	}

	a.detail.staged = nil
	tt.Frame()
	if !tt.HasText(a.t("publish.publish")) {
		t.Fatalf("plain publish label missing with no staged files; texts=%q", tt.Texts())
	}
	if !tt.HasText(a.t("publish.stageFirst")) {
		t.Fatalf("stage-first hint missing; texts=%q", tt.Texts())
	}
}

// TestDetailUploadProgressApplication folds streamed upload events into the
// state and checks the captions carry the running counts and the table rows
// render. It also checks cancelPublish is safe without and with a handler.
func TestDetailUploadProgressApplication(t *testing.T) {
	a := detailTestApp()
	d := a.detail
	d.uploading = true

	applyUploadProgress(a, d, UploadProgress{Index: 1, Total: 3, File: "pkg/app.exe", Status: "START"})
	applyUploadProgress(a, d, UploadProgress{Index: 1, Total: 3, File: "pkg/app.exe", Status: "DONE", FileSize: 2048})
	applyUploadProgress(a, d, UploadProgress{Index: 2, Total: 3, File: "pkg/lib.dll", Status: "START"})
	applyUploadProgress(a, d, UploadProgress{Index: 2, Total: 3, File: "pkg/lib.dll", Status: "FAIL", Error: "denied", FileSize: 512})

	if d.uploadDone != 1 || d.uploadFailed != 1 {
		t.Fatalf("counters done=%d failed=%d want 1/1", d.uploadDone, d.uploadFailed)
	}
	if !strings.Contains(d.uploadText, "2/3") {
		t.Fatalf("upload text %q missing running counts 2/3", d.uploadText)
	}
	if !strings.Contains(d.uploadSummary, "1 done") || !strings.Contains(d.uploadSummary, "1 failed") {
		t.Fatalf("upload summary %q missing counts", d.uploadSummary)
	}

	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText("pkg/lib.dll") {
		t.Fatalf("upload table row missing; texts=%q", tt.Texts())
	}
	if !tt.HasText(d.uploadSummary) {
		t.Fatalf("upload summary not rendered; texts=%q", tt.Texts())
	}

	// A cancel with no running publish must be a no-op, not a panic.
	d.cancel = nil
	a.cancelPublish()

	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx
	d.cancel = cancel
	a.cancelPublish()
	if d.status != a.t("publish.cancelled") {
		t.Fatalf("cancel status=%q want %q", d.status, a.t("publish.cancelled"))
	}
}

// TestDetailHistoryLatestVersion checks history rendering: every version row
// is present and the latest caption heads the card.
func TestDetailHistoryLatestVersion(t *testing.T) {
	a := detailTestApp()
	a.detail.logs = []ChangeLog{
		{Version: "V2.0.0", Time: "2026-11-20 09:00:00", Logs: []string{"second release"}},
		{Version: "V1.9.0", Time: "2026-11-13 10:00:00", Logs: []string{"first release"}},
	}
	a.detail.currentVersion = "V2.0.0"
	tt := ui.NewTester(a.view, 1440, 900)

	if !tt.HasText("V2.0.0") {
		t.Fatalf("latest version missing; texts=%q", tt.Texts())
	}
	if !tt.HasText("V1.9.0") {
		t.Fatalf("earlier version missing; texts=%q", tt.Texts())
	}
	if !tt.HasText(a.t("history.latest")) {
		t.Fatalf("latest caption missing; texts=%q", tt.Texts())
	}
	if !tt.HasText(a.t("history.title")) {
		t.Fatalf("history card title missing; texts=%q", tt.Texts())
	}
}

// TestDetailSplitWidthClamp mirrors the C# GridSplitter column: the side
// column stays within 260..520 DIPs whatever width is stored.
func TestDetailSplitWidthClamp(t *testing.T) {
	a := detailTestApp()
	tt := ui.NewTester(a.view, 1440, 900)

	a.detail.splitW = 10
	tt.Frame()
	if a.detail.splitW < detailMinSide {
		t.Fatalf("side width %v below minimum %v", a.detail.splitW, detailMinSide)
	}

	a.detail.splitW = 900
	tt.Frame()
	if a.detail.splitW > detailMaxSide {
		t.Fatalf("side width %v above maximum %v", a.detail.splitW, detailMaxSide)
	}
	if a.detail.splitW < detailMinSide {
		t.Fatalf("side width %v below minimum %v", a.detail.splitW, detailMinSide)
	}
}

// TestDetailHeaderTexts covers the texts the C# ProjectTabView always shows,
// at the reference size and at the smaller supported window.
func TestDetailHeaderTexts(t *testing.T) {
	for _, size := range []struct {
		width, height int
	}{
		{1440, 900},
		{1080, 640},
	} {
		a := detailTestApp()
		tt := ui.NewTester(a.view, size.width, size.height)
		for _, text := range []string{"Unstaged", "Staged", "Publish", "Version history"} {
			if !tt.HasText(text) {
				t.Errorf("%dx%d: missing %q; texts=%q", size.width, size.height, text, tt.Texts())
			}
		}
	}
}

// TestDetailBusyRow checks the detail work caption appears next to a spinner
// while an operation runs.
func TestDetailBusyRow(t *testing.T) {
	a := detailTestApp()
	a.detail.busy = true
	a.detail.busyText = a.t("changes.staging")
	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText(a.t("changes.staging")) {
		t.Fatalf("busy caption missing; texts=%q", tt.Texts())
	}
}

// TestDetailTablesAdaptToPaneWidth checks the responsive table: the wide
// changes pane lists both size columns, while a narrow one (the minimum
// window) drops the server size to keep the file name readable.
func TestDetailTablesAdaptToPaneWidth(t *testing.T) {
	wide := detailTestApp()
	wt := ui.NewTester(wide.view, 1440, 900)
	if !wt.HasText("Server") {
		t.Errorf("the wide changes table does not show the server column; texts=%q", wt.Texts())
	}
	narrow := detailTestApp()
	nt := ui.NewTester(narrow.view, 1080, 640)
	if nt.HasText("Server") {
		t.Errorf("the narrow changes table still shows the server column")
	}
	for _, text := range []string{"Local", "Status", "File"} {
		if !nt.HasText(text) {
			t.Errorf("the narrow changes table lost %q", text)
		}
	}
}
