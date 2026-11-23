package main

import (
	"context"
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
	logs     []ChangeLog

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
	// (380 default, clamped to 260..520). ui.Split wants the first pane's
	// width, so the view converts it while building.
	splitW float32
	// splitTotal is the split's width as last laid out. The view measures it
	// from the previous frame so the clamp stays exact after a window resize.
	splitTotal float32
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
				a.toastMsg = d.status
				return
			}
			// Keep the order the CLI returned: the file tables mirror it.
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
			d.status = a.tf("status.refreshed",
				"unstaged", fmt.Sprint(len(d.unstaged)),
				"staged", fmt.Sprint(len(d.staged)))
		})
	}()
}

func toFileItems(items []FileStatusItem) []fileItem {
	out := make([]fileItem, 0, len(items))
	for _, item := range items {
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
				a.toastMsg = d.status
				return
			}
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
	version := strings.TrimSpace(d.newVersion)
	notes := strings.TrimSpace(d.notes)
	if len(d.staged) == 0 || version == "" || notes == "" {
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
	var force *bool
	if d.forceKnown {
		value := d.forceUpdate
		force = &value
	}
	cli := a.cli

	go func() {
		failure := ""
		sawResult := false
		err := cli.push(ctx, path, version, notes, script, force, func(line string) {
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
			case err == context.Canceled:
				d.status = a.t("publish.cancelled")
				a.toastMsg = d.status
			case err == context.DeadlineExceeded:
				d.status = a.t("publish.timeout")
				a.toastMsg = d.status
			case err != nil:
				d.status = a.t("publish.failedStatus")
				a.toastMsg = d.status + ": " + err.Error()
			case failure != "":
				d.status = a.t("publish.failedStatus")
				a.toastMsg = failure
			case !sawResult:
				d.status = a.t("publish.noResult")
				a.toastMsg = d.status
			default:
				d.newVersion = ""
				d.notes = ""
				d.script = ""
				d.status = a.tf("publish.published", "version", version)
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
	if idx >= 0 {
		d.uploadItems[idx] = p
	} else {
		d.uploadItems = append(d.uploadItems, p)
	}
	total := fmt.Sprint(d.uploadTotal)
	switch p.Status {
	case "START":
		d.uploadText = a.tf("publish.uploadingCount",
			"file", p.File,
			"count", fmt.Sprint(d.uploadDone+d.uploadFailed),
			"total", total)
	case "DONE":
		d.uploadDone++
		d.uploadText = a.tf("publish.doneCount",
			"file", p.File,
			"count", fmt.Sprint(d.uploadDone+d.uploadFailed),
			"total", total)
		d.uploadSummary = a.tf("publish.summary",
			"done", fmt.Sprint(d.uploadDone),
			"failed", fmt.Sprint(d.uploadFailed),
			"total", total)
	case "FAIL":
		d.uploadFailed++
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
