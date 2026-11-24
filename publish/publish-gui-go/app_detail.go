package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type fileItem struct {
	RelativePath string
	Status       string
	LocalSize    int64
	ServerSize   int64
	Selected     bool
}

type detailState struct {
	project ProjectConfig
	shared  SharedConfig

	unstaged []fileItem
	staged   []fileItem
	// ignoredDeleted keeps legacy deleted staged entries out of the view while
	// retaining their paths so publish can unstage them before calling the CLI.
	ignoredDeleted []string
	logs           []ChangeLog

	currentVersion string
	newVersion     string
	notes          string
	script         string
	forceUpdate    bool
	forceKnown     bool
	advancedOpen   bool
	splitPercent   float32
	busy           bool
	status         string

	uploading     bool
	uploadItems   []UploadProgress
	uploadTotal   int
	uploadDone    int
	uploadFailed  int
	uploadText    string
	uploadSummary string
	cancel        context.CancelFunc

	// busyText labels the detail work underway (refresh, stage, publish);
	// the page shows it next to a spinner where C# shows a ProgressBar.
	busyText string
	// initialLoading draws the first-load skeleton until the first refresh
	// attempt of this tab finishes; loaded remembers that attempt (the C#
	// view model's _hasLoaded).
	initialLoading bool
	loaded         bool
	// splitW is the side column's width, like the C# GridSplitter column
	// (380 default, clamped to 300..540). ui.Split wants the first pane's
	// width, so the view converts it while building.
	splitW float32
	// splitTotal is the split's width as last laid out. The view measures it
	// from the previous frame so the clamp stays exact after a window resize.
	splitTotal float32
	// changesWidth is the changes card's width in the last layout, so its
	// tables can drop columns when the window is narrow.
	changesWidth float32
	// publishPercent is the share of the side column the publish card takes
	// above the version history, like the C# GridSplitter running across it.
	// Both splits keep their ratio as files change and grow the panes
	// proportionally as the window resizes.
	publishPercent float32
}

func (a *app) refreshDetail() {
	d := a.detail
	if d == nil {
		return
	}
	if d.busy || d.uploading {
		return
	}
	if !a.cli.available() {
		// Nothing can load, but the first attempt is over: drop the skeleton
		// so the tab does not look permanently busy.
		d.busy = false
		d.busyText = ""
		d.initialLoading = false
		d.loaded = true
		d.status = a.t("toolbar.cliMissing")
		return
	}
	d.busy = true
	d.busyText = a.t("changes.reading")
	d.status = d.busyText
	d.initialLoading = !d.loaded
	busyLabel := d.busyText
	a.beginBusy(busyLabel)

	path := d.project.ProjectPath
	shared, _ := loadShared(path)
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		status, err := cli.status(ctx, path)
		logs, logErr := cli.logs(ctx, path, 20)
		var info ProjectInfo
		var infoErr error
		if shared.ServerURL != "" && shared.ProjectName != "" {
			info, infoErr = cli.projectInfo(ctx, shared.ServerURL, shared.ProjectName)
		}
		a.post(func() {
			// Whatever happened, the detail is no longer busy and the first
			// load is over, even when the user switched to another tab.
			d.busy = false
			d.busyText = ""
			d.initialLoading = false
			d.loaded = true
			a.endDetailBusy(busyLabel)
			if a.detail != d {
				return
			}
			if err != nil {
				d.status = a.tf("status.refreshFailed", "error", err.Error())
				a.statusTone = toneDanger
				a.toastMsg = d.status
				return
			}
			// Keep the order the CLI returned: the file tables mirror it. Deleted
			// entries are retained server-side, but legacy staged deletions must
			// be remembered so a later push can unstage rather than publish them.
			d.ignoredDeleted = d.ignoredDeleted[:0]
			for _, item := range status.Staged {
				if item.Status == "deleted" {
					d.ignoredDeleted = append(d.ignoredDeleted, item.RelativePath)
				}
			}
			d.unstaged = toFileItems(status.Unstaged)
			d.staged = toFileItems(status.Staged)
			if logErr == nil {
				d.logs = logs
				d.currentVersion = ""
				if len(logs) > 0 {
					d.currentVersion = logs[0].Version
				}
			}
			d.shared = shared
			if infoErr == nil {
				d.forceUpdate = info.ForceUpdate
				d.forceKnown = true
			}
			a.statusTone = toneSuccess
			d.status = a.tf("status.refreshed",
				"unstaged", fmt.Sprint(len(d.unstaged)),
				"staged", fmt.Sprint(len(d.staged)))
			if len(d.ignoredDeleted) > 0 {
				d.status = a.t("status.deletedIgnored")
				a.statusTone = toneWarning
				a.toastMsg = d.status
			}
		})
	}()
}

func toFileItems(items []FileStatusItem) []fileItem {
	out := make([]fileItem, 0, len(items))
	for _, item := range items {
		if item.Status == "deleted" {
			continue
		}
		out = append(out, fileItem{
			RelativePath: item.RelativePath,
			Status:       item.Status,
			LocalSize:    item.LocalSize,
			ServerSize:   item.ServerSize,
		})
	}
	return out
}

func selectedFiles(files []fileItem) []string {
	var out []string
	for _, f := range files {
		if f.Selected {
			out = append(out, f.RelativePath)
		}
	}
	return out
}

func allSelected(files []fileItem) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !f.Selected {
			return false
		}
	}
	return true
}

func (a *app) stageAll(staged bool) {
	d := a.detail
	if d == nil || d.busy || d.uploading || !a.cli.available() {
		return
	}
	d.busy = true
	d.busyText = a.stageBusyText(staged)
	d.status = d.busyText
	busyLabel := d.busyText
	a.beginBusy(busyLabel)

	path := d.project.ProjectPath
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		var err error
		if staged {
			err = cli.addAll(ctx, path)
		} else {
			err = cli.resetAll(ctx, path)
		}
		a.post(func() {
			if a.detail == d {
				d.busy = false
				d.busyText = ""
			}
			a.endDetailBusy(busyLabel)
			if a.detail != d {
				return
			}
			if err != nil {
				key := "status.unstageFailed"
				if staged {
					key = "status.stageFailed"
				}
				d.status = a.tf(key, "error", err.Error())
				a.statusTone = toneDanger
				a.toastMsg = d.status
				return
			}
			a.statusTone = toneSuccess
			if staged {
				a.status = a.t("status.staged")
			} else {
				a.status = a.t("status.unstaged")
			}
			a.refreshDetail()
		})
	}()
}

func (a *app) stageSelected(staged bool) {
	d := a.detail
	if d == nil || d.busy || d.uploading || !a.cli.available() {
		return
	}
	files := selectedFiles(d.unstaged)
	if !staged {
		files = selectedFiles(d.staged)
	}
	if len(files) == 0 {
		return
	}
	d.busy = true
	d.busyText = a.stageBusyText(staged)
	d.status = d.busyText
	busyLabel := d.busyText
	a.beginBusy(busyLabel)

	path := d.project.ProjectPath
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		var err error
		if staged {
			err = cli.addFiles(ctx, path, files)
		} else {
			err = cli.resetFiles(ctx, path, files)
		}
		a.post(func() {
			if a.detail == d {
				d.busy = false
				d.busyText = ""
			}
			a.endDetailBusy(busyLabel)
			if a.detail != d {
				return
			}
			if err != nil {
				key := "status.unstageFailed"
				if staged {
					key = "status.stageFailed"
				}
				d.status = a.tf(key, "error", err.Error())
				a.statusTone = toneDanger
				a.toastMsg = d.status
				return
			}
			a.refreshDetail()
		})
	}()
}

// stageBusyText is the spinner caption for staging or unstaging work.
func (a *app) stageBusyText(staged bool) string {
	if staged {
		return a.t("changes.staging")
	}
	return a.t("changes.unstaging")
}

func (a *app) publish() {
	d := a.detail
	if d == nil || d.busy || d.uploading || !a.cli.available() {
		return
	}
	deleted := append([]string(nil), d.ignoredDeleted...)
	staged := make([]fileItem, 0, len(d.staged))
	for _, f := range d.staged {
		if f.Status == "deleted" {
			deleted = appendUnique(deleted, f.RelativePath)
			continue
		}
		staged = append(staged, f)
	}
	if len(staged) == 0 {
		if len(deleted) > 0 {
			d.status = a.t("status.deletedIgnored")
			a.statusTone = toneWarning
			a.toastMsg = d.status
		}
		return
	}
	d.staged = staged
	version := strings.TrimSpace(d.newVersion)
	notes := strings.TrimSpace(d.notes)
	if version == "" || notes == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	d.cancel = cancel
	d.uploading = true
	d.uploadItems = nil
	d.uploadTotal = len(d.staged)
	d.uploadDone = 0
	d.uploadFailed = 0
	d.uploadText = a.tf("publish.preparingCount", "count", fmt.Sprint(d.uploadTotal))
	d.uploadSummary = ""
	busyLabel := d.uploadText
	a.beginBusy(busyLabel)

	path := d.project.ProjectPath
	script := strings.TrimSpace(d.script)
	// 每行一条变更记录：多行说明拆成多条 --message（与 CLI 语义一致）。
	var messages []string
	for _, line := range strings.Split(strings.ReplaceAll(d.notes, "\r\n", "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			messages = append(messages, trimmed)
		}
	}
	var force *bool
	if d.forceKnown {
		value := d.forceUpdate
		force = &value
	}
	cli := a.cli

	go func() {
		failure := ""
		sawResult := false
		var err error
		if len(deleted) > 0 {
			// The CLI owns the staging file. Removing legacy deleted entries from
			// the Go view alone would still let push delete them on the server.
			err = cli.resetFiles(ctx, path, deleted)
		}
		if err == nil {
			err = cli.push(ctx, path, version, messages, script, force, func(line string) {
				ev := parsePushLine(line)
				switch ev.Kind {
				case pushFailure:
					if failure == "" {
						failure = ev.Error
					}
				case pushSuccess:
					sawResult = true
				}
				if ev.Kind != pushProgress {
					return
				}
				a.post(func() {
					if a.detail != d {
						return
					}
					applyUploadProgress(a, d, ev.Progress)
				})
			})
		}
		cancel()
		a.post(func() {
			if a.detail == d {
				d.uploading = false
				d.cancel = nil
				d.uploadItems = nil
				d.uploadSummary = ""
				d.uploadText = ""
				d.uploadTotal = 0
				d.uploadDone = 0
				d.uploadFailed = 0
			}
			a.endDetailBusy(busyLabel)
			if a.detail != d {
				return
			}
			switch {
			case errors.Is(err, context.Canceled):
				d.status = a.t("publish.cancelled")
				a.statusTone = toneWarning
				a.toastMsg = d.status
			case errors.Is(err, context.DeadlineExceeded):
				d.status = a.t("publish.timeout")
				a.statusTone = toneWarning
				a.toastMsg = d.status
			case err != nil:
				d.status = a.t("publish.failedStatus")
				a.statusTone = toneDanger
				a.toastMsg = d.status + ": " + err.Error()
			case failure != "":
				d.status = a.t("publish.failedStatus")
				a.statusTone = toneDanger
				a.toastMsg = failure
			case !sawResult:
				d.status = a.t("publish.noResult")
				a.statusTone = toneDanger
				a.toastMsg = d.status
			default:
				d.newVersion = ""
				d.notes = ""
				d.script = ""
				d.ignoredDeleted = nil
				d.status = a.tf("publish.published", "version", version)
				a.statusTone = toneSuccess
				a.status = d.status
				a.toastMsg = d.status
				a.refreshDetail()
			}
		})
	}()
}

// applyUploadProgress folds one streamed upload event into the detail state:
// the per-file rows, the done/failed counters and the status captions with
// their running counts (the C# OnUploadProgress behaviour).
func applyUploadProgress(a *app, d *detailState, p UploadProgress) {
	if d == nil {
		return
	}
	if p.Total > 0 {
		d.uploadTotal = p.Total
	}
	idx := -1
	for i := range d.uploadItems {
		if d.uploadItems[i].Index == p.Index {
			idx = i
			break
		}
	}
	previousStatus := ""
	if idx >= 0 {
		previousStatus = d.uploadItems[idx].Status
		d.uploadItems[idx] = p
	} else {
		d.uploadItems = append(d.uploadItems, p)
	}
	// A replayed terminal event is idempotent; a changed status moves the file
	// between counters instead of counting the same index once per status.
	if previousStatus != p.Status {
		switch previousStatus {
		case "DONE":
			d.uploadDone--
		case "FAIL":
			d.uploadFailed--
		}
		switch p.Status {
		case "DONE":
			d.uploadDone++
		case "FAIL":
			d.uploadFailed++
		}
	}
	total := fmt.Sprint(d.uploadTotal)
	switch p.Status {
	case "START":
		d.uploadText = a.tf("publish.uploadingCount",
			"file", p.File,
			"count", fmt.Sprint(d.uploadDone+d.uploadFailed),
			"total", total)
	case "DONE":
		d.uploadText = a.tf("publish.doneCount",
			"file", p.File,
			"count", fmt.Sprint(d.uploadDone+d.uploadFailed),
			"total", total)
		d.uploadSummary = a.tf("publish.summary",
			"done", fmt.Sprint(d.uploadDone),
			"failed", fmt.Sprint(d.uploadFailed),
			"total", total)
	case "FAIL":
		d.uploadText = a.tf("publish.failedCount",
			"file", p.File,
			"count", fmt.Sprint(d.uploadDone+d.uploadFailed),
			"total", total,
			"error", p.Error)
		d.uploadSummary = a.tf("publish.summary",
			"done", fmt.Sprint(d.uploadDone),
			"failed", fmt.Sprint(d.uploadFailed),
			"total", total)
	}
}

// endDetailBusy clears the window busy label only when it still belongs to
// the operation that is finishing, so a stale refresh on a switched-away tab
// cannot hide the new tab's work.
func (a *app) endDetailBusy(label string) {
	if label != "" && a.busyText == label {
		a.endBusy()
	}
}

func (a *app) cancelPublish() {
	d := a.detail
	if d == nil || !d.uploading || d.cancel == nil {
		return
	}
	d.cancel()
	d.status = a.t("publish.cancelled")
}
