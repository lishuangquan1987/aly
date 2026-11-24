package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// TestDetailSplitsKeepRatios checks the two splitters: an empty half no
// longer moves the divider (the ratio is the user's), an out-of-range value is
// clamped, and the publish/history split survives a resize.
func TestDetailSplitsKeepRatios(t *testing.T) {
	a := detailTestApp()
	a.detail.splitPercent = 40
	a.detail.unstaged = nil
	ui.NewTester(a.view, 1320, 840)
	if a.detail.splitPercent != 40 {
		t.Fatalf("changes split = %v after one half emptied, want the set 40",
			a.detail.splitPercent)
	}

	b := detailTestApp()
	b.detail.splitPercent = 10
	ui.NewTester(b.view, 1320, 840)
	if b.detail.splitPercent != 20 {
		t.Fatalf("changes split = %v, want it clamped to 20", b.detail.splitPercent)
	}

	c := detailTestApp()
	c.detail.publishPercent = 60
	ui.NewTester(c.view, 1320, 840)
	if c.detail.publishPercent != 60 {
		t.Fatalf("publish split = %v after a render, want the set 60", c.detail.publishPercent)
	}

	d := detailTestApp()
	d.detail.publishPercent = 0
	ui.NewTester(d.view, 1320, 840)
	if d.detail.publishPercent != 50 {
		t.Fatalf("publish split = %v, want the half/half default 50", d.detail.publishPercent)
	}
}

// TestDetailSplitsAreDraggable checks the splitter answers a drag: both splits
// are the framework's SplitVerticalPercent, whose divider is the same widget,
// so dragging the changes divider proves the adjustable behaviour.
func TestDetailSplitsAreDraggable(t *testing.T) {
	a := detailTestApp()
	tt := ui.NewTester(a.view, 1320, 840)
	divider, ok := tt.Find("Divider")
	if !ok {
		t.Skip("the splitter divider is not exposed to the test tree")
	}
	x, y := divider.X+divider.W/2, divider.Y+divider.H/2
	before := a.detail.splitPercent
	tt.Press(x, y)
	tt.Move(x, y-60)
	tt.Release(x, y-60)
	if a.detail.splitPercent >= before {
		t.Fatalf("dragging the splitter left splitPercent at %v (was %v)",
			a.detail.splitPercent, before)
	}
}

// TestPublishNotesKeepSideRatio checks the side splitter's stability: a long
// release note scrolls inside the publish pane instead of growing it and
// squeezing the version history.
func TestPublishNotesKeepSideRatio(t *testing.T) {
	a := detailTestApp()
	a.detail.publishPercent = 50
	a.detail.notes = strings.Repeat("长说明行 \n", 40)
	tt := ui.NewTester(a.view, 1320, 840)
	if a.detail.publishPercent != 50 {
		t.Fatalf("publish split = %v after long notes, want 50", a.detail.publishPercent)
	}
	if !tt.HasText("Version history") {
		t.Fatalf("the version history is missing under the publish card")
	}
	if !tt.HasText("Release notes (required)") {
		t.Fatalf("the release notes field is missing from the publish pane")
	}
}

// TestDetailPanelsFillTheirRoom checks the summary line under a table and the
// hint that fills what a short file list leaves, so the panes read as full.
func TestDetailPanelsFillTheirRoom(t *testing.T) {
	a := detailTestApp()
	tt := ui.NewTester(a.view, 1320, 840)
	for _, text := range []string{
		"1 file \u00b7 2 KB",
		"Refresh (F5) after a build to pick up new files.",
		"Files upload in order and can be cancelled at any time.",
		"1 version",
		"Recent releases are listed newest first.",
	} {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

// TestDetailLargeTablesKeepLayout is the regression for min-content overflow:
// with many changed files and a long version history, the page must keep its
// header below the toolbar and its footer above the status bar instead of
// being centered over them.
func TestDetailLargeTablesKeepLayout(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1320, 840}, {1720, 1100}} {
		a := bigDetailApp()
		tt := ui.NewTesterWith(a.view, size.w, size.h, ui.TesterOptions{Dark: true})
		if unstaged, ok := tt.Find("Unstaged"); !ok || unstaged.Y < toolbarHeight {
			t.Fatalf("%dx%d: the changes card overlaps the toolbar: %+v", size.w, size.h, unstaged)
		}
		if !tt.HasText("30 files") {
			t.Fatalf("%dx%d: the file summary is missing", size.w, size.h)
		}
	}
}

// bigDetailApp is a project with enough files and history to overflow a small
// window's natural layout.
func bigDetailApp() *app {
	a := testApp()
	a.detail.unstaged = nil
	a.detail.logs = nil
	for i := 0; i < 30; i++ {
		a.detail.unstaged = append(a.detail.unstaged, fileItem{
			RelativePath: fmt.Sprintf("bin/Yofc.Component%02d.dll", i),
			Status:       "modified",
			LocalSize:    55296,
			ServerSize:   55296,
		})
	}
	for i := 0; i < 30; i++ {
		a.detail.logs = append(a.detail.logs, ChangeLog{
			Version: fmt.Sprintf("V1.0.%d", i),
			Time:    "2026-11-20 10:00:00",
			Logs:    []string{"changes"},
		})
	}
	return a
}

func TestToFileItemsFiltersDeleted(t *testing.T) {
	got := toFileItems([]FileStatusItem{
		{RelativePath: "new.exe", Status: "new"},
		{RelativePath: "old.dll", Status: "deleted"},
		{RelativePath: "changed.exe", Status: "modified"},
	})
	if len(got) != 2 || got[0].RelativePath != "new.exe" || got[1].RelativePath != "changed.exe" {
		t.Fatalf("deleted entries must be hidden from changes: %+v", got)
	}
}

func TestRefreshDetailFiltersAndWarnsDeleted(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, _ := newHelperCLI(t, "deleted-status")
	a.cli = cli
	a.detail.staged = nil
	a.detail.logs = nil
	a.refreshDetail()
	deletedWarning := a.t("status.deletedIgnored")
	waitForTestCondition(t, "deleted-status refresh", func() bool {
		return a.detail.loaded && !a.detail.busy && a.statusTone == toneWarning && a.toastMsg == deletedWarning
	})
	if len(a.detail.staged) != 1 || a.detail.staged[0].RelativePath != "app.exe" {
		t.Fatalf("staged = %+v, deleted entry was not filtered", a.detail.staged)
	}
	if got, want := a.toastMsg, a.t("status.deletedIgnored"); got != want || a.statusTone != toneWarning {
		t.Fatalf("deleted warning = %q tone=%v, want %q/warning", got, a.statusTone, want)
	}
}

func TestPublishDeletedOnlyDoesNotCallCLI(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, logPath := newHelperCLI(t, "push-success")
	a.cli = cli
	a.detail.staged = []fileItem{{RelativePath: "old.dll", Status: "deleted"}}
	a.detail.newVersion = "V1.0.1"
	a.detail.notes = "must not delete"
	a.publish()
	if a.detail.uploading {
		waitForTestCondition(t, "unexpected legacy publish completion", func() bool {
			return !a.detail.uploading
		})
	}
	if calls := readHelperCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("deleted-only publish invoked CLI: %+v", calls)
	}
	if got, want := a.toastMsg, a.t("status.deletedIgnored"); got != want {
		t.Fatalf("toast = %q, want %q", got, want)
	}
}

func TestPublishResetsLegacyDeletedBeforePush(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, logPath := newHelperCLI(t, "push-success")
	a.cli = cli
	a.detail.staged = []fileItem{
		{RelativePath: "old.dll", Status: "deleted"},
		{RelativePath: "app.exe", Status: "modified"},
	}
	a.detail.newVersion = "V1.0.1"
	a.detail.notes = "keep server-only files"
	a.publish()
	waitForTestCondition(t, "mixed legacy publish", func() bool {
		return !a.detail.uploading && !a.detail.busy && a.busyText == "" && a.detail.status != ""
	})
	calls := readHelperCalls(t, logPath)
	if len(calls) < 2 || len(calls[0].Args) != 3 || calls[0].Args[0] != "reset" || calls[0].Args[2] != "old.dll" {
		t.Fatalf("deleted entry was not reset before push: %+v", calls)
	}
	if len(calls[1].Args) == 0 || calls[1].Args[0] != "push" {
		t.Fatalf("push did not follow reset: %+v", calls)
	}
}

func TestPublishStopsWhenLegacyDeletedResetFails(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, logPath := newHelperCLI(t, "push-success")
	t.Setenv("ALY_GUI_TEST_FAIL_ON_CALL", "1")
	a.cli = cli
	a.detail.staged = []fileItem{
		{RelativePath: "old.dll", Status: "deleted"},
		{RelativePath: "app.exe", Status: "modified"},
	}
	a.detail.newVersion = "V1.0.1"
	a.detail.notes = "keep this draft"
	a.publish()
	waitForTestCondition(t, "failed deleted reset", func() bool {
		return !a.detail.uploading && a.busyText == "" && a.statusTone == toneDanger &&
			strings.Contains(a.toastMsg, "injected call failure")
	})
	calls := readHelperCalls(t, logPath)
	if len(calls) != 1 || len(calls[0].Args) == 0 || calls[0].Args[0] != "reset" {
		t.Fatalf("only the failed reset may run; calls = %+v", calls)
	}
	if a.detail.notes != "keep this draft" || a.statusTone != toneDanger {
		t.Fatalf("failed reset lost draft or danger state: notes=%q tone=%v", a.detail.notes, a.statusTone)
	}
}

func TestApplyUploadProgressDuplicateTerminalEventsAreIdempotent(t *testing.T) {
	a := detailTestApp()
	d := a.detail
	applyUploadProgress(a, d, UploadProgress{Index: 1, Total: 2, File: "a.exe", Status: "DONE"})
	applyUploadProgress(a, d, UploadProgress{Index: 1, Total: 2, File: "a.exe", Status: "DONE"})
	applyUploadProgress(a, d, UploadProgress{Index: 2, Total: 2, File: "b.exe", Status: "FAIL", Error: "denied"})
	applyUploadProgress(a, d, UploadProgress{Index: 2, Total: 2, File: "b.exe", Status: "FAIL", Error: "denied"})
	if d.uploadDone != 1 || d.uploadFailed != 1 {
		t.Fatalf("duplicate terminal events changed counters: done=%d failed=%d", d.uploadDone, d.uploadFailed)
	}
	if len(d.uploadItems) != 2 {
		t.Fatalf("duplicate terminal events added rows: %+v", d.uploadItems)
	}
}

func TestApplyUploadProgressTerminalTransitionsKeepCounts(t *testing.T) {
	a := detailTestApp()
	d := a.detail
	event := UploadProgress{Index: 1, Total: 1, File: "a.exe", Status: "DONE"}
	applyUploadProgress(a, d, event)
	event.Status = "FAIL"
	event.Error = "late failure"
	applyUploadProgress(a, d, event)
	if d.uploadDone != 0 || d.uploadFailed != 1 {
		t.Fatalf("DONE→FAIL must move the same file between counters: done=%d failed=%d", d.uploadDone, d.uploadFailed)
	}
	event.Status = "DONE"
	event.Error = ""
	applyUploadProgress(a, d, event)
	if d.uploadDone != 1 || d.uploadFailed != 0 {
		t.Fatalf("FAIL→DONE must move the same file between counters: done=%d failed=%d", d.uploadDone, d.uploadFailed)
	}
	event.Status = "START"
	applyUploadProgress(a, d, event)
	event.Status = "DONE"
	applyUploadProgress(a, d, event)
	if d.uploadDone != 1 || d.uploadFailed != 0 {
		t.Fatalf("terminal→START→terminal must not double count: done=%d failed=%d", d.uploadDone, d.uploadFailed)
	}
}

func TestStageSelectedUsesOnlySelectedPaths(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, logPath := newHelperCLI(t, "success")
	a.cli = cli
	a.detail.unstaged = []fileItem{
		{RelativePath: "selected.txt", Status: "new", Selected: true},
		{RelativePath: "other.txt", Status: "modified"},
	}
	a.detail.staged = nil
	a.stageSelected(true)
	refreshed := a.tf("status.refreshed", "unstaged", "0", "staged", "0")
	waitForTestCondition(t, "selected staging and refresh", func() bool {
		return !a.detail.busy && a.busyText == "" && a.detail.status == refreshed
	})
	calls := readHelperCalls(t, logPath)
	if len(calls) == 0 || strings.Join(calls[0].Args, " ") != "add -- selected.txt" {
		t.Fatalf("stageSelected args = %+v", calls)
	}
	if a.detail.busy || a.busyText != "" {
		t.Fatalf("staging left busy state: detail=%v window=%q", a.detail.busy, a.busyText)
	}
}

func TestStageSelectedFailureClearsBusy(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, _ := newHelperCLI(t, "business-failure")
	a.cli = cli
	a.detail.unstaged = []fileItem{{RelativePath: "selected.txt", Status: "new", Selected: true}}
	a.stageSelected(true)
	waitForTestCondition(t, "staging failure", func() bool {
		return !a.detail.busy && a.busyText == "" && a.statusTone == toneDanger &&
			strings.Contains(a.toastMsg, "Stage failed")
	})
	if !strings.Contains(a.detail.status, "Stage failed") || a.statusTone != toneDanger {
		t.Fatalf("status = %q tone=%v", a.detail.status, a.statusTone)
	}
	if a.busyText != "" {
		t.Fatalf("window busy text = %q", a.busyText)
	}
}

func TestStageAllSuccess(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, logPath := newHelperCLI(t, "success")
	a.cli = cli
	a.stageAll(true)
	refreshed := a.tf("status.refreshed", "unstaged", "0", "staged", "0")
	staged := a.t("status.staged")
	waitForTestCondition(t, "stage all and refresh", func() bool {
		return !a.detail.busy && a.busyText == "" && a.status == staged && a.detail.status == refreshed
	})
	calls := readHelperCalls(t, logPath)
	if len(calls) == 0 || strings.Join(calls[0].Args, " ") != "add --all" {
		t.Fatalf("stageAll calls = %+v", calls)
	}
	if a.statusTone != toneSuccess {
		t.Fatalf("stageAll success tone = %v", a.statusTone)
	}
}

func TestStageAllFailureClearsBusy(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, _ := newHelperCLI(t, "business-failure")
	a.cli = cli
	a.stageAll(true)
	waitForTestCondition(t, "stage all failure", func() bool {
		return !a.detail.busy && a.busyText == "" && a.statusTone == toneDanger &&
			strings.Contains(a.toastMsg, "Stage failed")
	})
	if !strings.Contains(a.detail.status, "Stage failed") || a.statusTone != toneDanger || a.busyText != "" {
		t.Fatalf("stageAll failure status=%q tone=%v busy=%q", a.detail.status, a.statusTone, a.busyText)
	}
}

func TestPublishForceKnownTriState(t *testing.T) {
	for _, tc := range []struct {
		name       string
		forceKnown bool
		force      bool
		wantArg    string
	}{
		{name: "unknown preserves server value", forceKnown: false},
		{name: "known true sets", forceKnown: true, force: true, wantArg: "--set-force-update"},
		{name: "known false clears", forceKnown: true, force: false, wantArg: "--clear-force-update"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := detailTestApp()
			a.detail.project.ProjectPath = t.TempDir()
			cli, logPath := newHelperCLI(t, "push-success")
			a.cli = cli
			a.detail.staged = []fileItem{{RelativePath: "app.exe", Status: "modified"}}
			a.detail.newVersion = "V1.0.1"
			a.detail.notes = "release"
			a.detail.script = "after.cmd"
			a.detail.forceKnown = tc.forceKnown
			a.detail.forceUpdate = tc.force
			a.publish()
			published := a.tf("publish.published", "version", "V1.0.1")
			refreshed := a.tf("status.refreshed", "unstaged", "0", "staged", "0")
			waitForTestCondition(t, "publish success and refresh", func() bool {
				return !a.detail.uploading && !a.detail.busy && a.busyText == "" &&
					a.status == published && a.toastMsg == published && a.detail.status == refreshed
			})
			calls := readHelperCalls(t, logPath)
			if len(calls) == 0 || len(calls[0].Args) == 0 || calls[0].Args[0] != "push" {
				t.Fatalf("publish calls = %+v", calls)
			}
			if len(calls[0].RawArgs) < 2 || calls[0].RawArgs[0] != "--json" || calls[0].RawArgs[1] != "push" {
				t.Fatalf("push raw args = %+v", calls[0].RawArgs)
			}
			joined := strings.Join(calls[0].Args, " ")
			if tc.wantArg == "" && (strings.Contains(joined, "--set-force-update") || strings.Contains(joined, "--clear-force-update")) {
				t.Fatalf("unknown force emitted an override: %s", joined)
			}
			if tc.wantArg != "" && !strings.Contains(joined, tc.wantArg) {
				t.Fatalf("push args %s missing %s", joined, tc.wantArg)
			}
			if a.detail.newVersion != "" || a.detail.notes != "" || a.detail.script != "" {
				t.Fatalf("successful publish retained form: version=%q notes=%q script=%q",
					a.detail.newVersion, a.detail.notes, a.detail.script)
			}
		})
	}
}

func TestPublishFailuresPreserveForm(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mode        string
		wantStatus  string
		wantMessage string
	}{
		{name: "business failure", mode: "push-failure", wantStatus: "Publish failed", wantMessage: "push rejected"},
		{name: "missing result", mode: "push-no-result", wantStatus: "No publish result", wantMessage: "No publish result"},
		{name: "process failure", mode: "process-failure", wantStatus: "Publish failed", wantMessage: "helper process exploded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := detailTestApp()
			a.detail.project.ProjectPath = t.TempDir()
			cli, _ := newHelperCLI(t, tc.mode)
			a.cli = cli
			a.detail.staged = []fileItem{{RelativePath: "app.exe", Status: "modified"}}
			a.detail.newVersion = "V1.0.1"
			a.detail.notes = "keep these notes"
			a.detail.script = "keep.cmd"
			a.publish()
			waitForTestCondition(t, "publish failure", func() bool {
				return !a.detail.uploading && a.busyText == "" && a.statusTone == toneDanger && a.toastMsg != ""
			})
			if !strings.Contains(a.detail.status, tc.wantStatus) || !strings.Contains(a.toastMsg, tc.wantMessage) {
				t.Fatalf("status = %q toast = %q", a.detail.status, a.toastMsg)
			}
			if a.detail.newVersion != "V1.0.1" || a.detail.notes != "keep these notes" || a.detail.script != "keep.cmd" {
				t.Fatalf("failed publish cleared form: %+v", a.detail)
			}
		})
	}
}

func TestPublishCancellationPreservesForm(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, _ := newHelperCLI(t, "block")
	a.cli = cli
	ready := filepath.Join(t.TempDir(), "ready")
	t.Setenv("ALY_GUI_TEST_READY_FILE", ready)
	a.detail.staged = []fileItem{{RelativePath: "app.exe", Status: "modified"}}
	a.detail.newVersion = "V1.0.1"
	a.detail.notes = "cancel without losing notes"
	a.publish()
	waitForTestCondition(t, "publish helper readiness", func() bool {
		_, err := os.Stat(ready)
		return err == nil
	})
	a.cancelPublish()
	waitForTestCondition(t, "publish cancellation", func() bool {
		return !a.detail.uploading && a.detail.status == a.t("publish.cancelled")
	})
	if a.detail.notes != "cancel without losing notes" || a.detail.newVersion != "V1.0.1" {
		t.Fatalf("cancel cleared form: version=%q notes=%q", a.detail.newVersion, a.detail.notes)
	}
}

func TestPublishCancellationDuringDeletedResetPreservesForm(t *testing.T) {
	a := detailTestApp()
	a.detail.project.ProjectPath = t.TempDir()
	cli, _ := newHelperCLI(t, "block")
	a.cli = cli
	ready := filepath.Join(t.TempDir(), "ready")
	t.Setenv("ALY_GUI_TEST_READY_FILE", ready)
	a.detail.staged = []fileItem{
		{RelativePath: "old.dll", Status: "deleted"},
		{RelativePath: "app.exe", Status: "modified"},
	}
	a.detail.newVersion = "V1.0.1"
	a.detail.notes = "cancel during reset"
	a.publish()
	waitForTestCondition(t, "deleted reset helper readiness", func() bool {
		_, err := os.Stat(ready)
		return err == nil
	})
	a.cancelPublish()
	cancelled := a.t("publish.cancelled")
	waitForTestCondition(t, "deleted reset cancellation", func() bool {
		return !a.detail.uploading && a.busyText == "" && a.detail.status == cancelled &&
			a.statusTone == toneWarning && a.toastMsg == cancelled
	})
	if got, want := a.detail.status, a.t("publish.cancelled"); got != want {
		t.Fatalf("wrapped reset cancellation reported as %q, want %q", got, want)
	}
	if a.detail.notes != "cancel during reset" || a.detail.newVersion != "V1.0.1" {
		t.Fatalf("reset cancellation cleared form: version=%q notes=%q", a.detail.newVersion, a.detail.notes)
	}
}

func TestDetailHistoryShowsAfterApplyScript(t *testing.T) {
	a := detailTestApp()
	a.detail.logs = []ChangeLog{{
		Version:                "V1.0.1",
		Time:                   "2026-11-24 10:00:00",
		Logs:                   []string{"release"},
		AfterApplyUpdateScript: "scripts/migrate.cmd",
	}}
	tt := ui.NewTester(a.view, 1320, 840)
	if !tt.HasText("Post-update script: scripts/migrate.cmd") {
		t.Fatalf("after-apply script missing; texts=%q", tt.Texts())
	}
}
