package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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
	split          float32
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
}

type editState struct {
	project  ProjectConfig
	shared   SharedConfig
	original SharedConfig

	displayName string
	mainExe     string

	ignoreFolders []string
	ignoreFiles   []string
	uncopyFolders []string
	uncopyFiles   []string

	newIgnoreFolder string
	newIgnoreFile   string
	newUncopyFolder string
	newUncopyFile   string

	busy  bool
	error string
}

type app struct {
	win  *mygo.Window
	lang string
	cli  *cliClient

	projects []ProjectConfig
	filter   string
	selected int
	list     ui.ListState

	detail *detailState

	editOpen   bool
	editing    *editState
	removeOpen bool
	clearOpen  bool

	status   string
	toastMsg string
}

func newApp() *app {
	lang := "zh"
	if strings.HasPrefix(strings.ToLower(loadSettings().Language), "en") {
		lang = "en"
	}
	a := &app{
		lang:     lang,
		projects: loadProjects(),
		selected: -1,
	}
	a.cli = &cliClient{exe: findCLI()}
	if a.cli.available() {
		a.status = a.tf("cli.found", "path", a.cli.exe)
	} else {
		a.status = a.t("toolbar.cliMissing")
	}
	return a
}

func (a *app) t(key string) string { return translate(a.lang, key) }

func (a *app) tf(key string, pairs ...string) string {
	s := a.t(key)
	for i := 0; i+1 < len(pairs); i += 2 {
		s = strings.ReplaceAll(s, "{"+pairs[i]+"}", pairs[i+1])
	}
	return s
}

func (a *app) localeTag() string {
	if a.lang == "zh" {
		return "zh-Hans"
	}
	return "en"
}

func (a *app) toggleLang() {
	if a.lang == "zh" {
		a.lang = "en"
	} else {
		a.lang = "zh"
	}
	mygo.App.SetLocale(a.localeTag())
	_ = saveSettings(appSettings{Language: map[bool]string{true: "zh-CN", false: "en-US"}[a.lang == "zh"]})
	if a.win != nil {
		a.win.SetTitle(a.t("app.title"))
	}
	a.status = a.t("status.ready")
}

func (a *app) start() {
	if len(a.projects) == 0 {
		a.status = a.t("projects.emptyTitle")
		return
	}
	if a.selected < 0 || a.selected >= len(a.projects) {
		a.selected = 0
	}
	a.openSelected()
}

func (a *app) filteredProjects() []ProjectConfig {
	q := strings.ToLower(strings.TrimSpace(a.filter))
	if q == "" {
		return a.projects
	}
	out := make([]ProjectConfig, 0, len(a.projects))
	for _, p := range a.projects {
		if strings.Contains(strings.ToLower(p.DisplayName+" "+p.ProjectPath), q) {
			out = append(out, p)
		}
	}
	return out
}

func (a *app) openSelected() {
	list := a.filteredProjects()
	if a.selected < 0 || a.selected >= len(list) {
		return
	}
	p := list[a.selected]
	if a.detail != nil && normalizePath(a.detail.project.ProjectPath) == normalizePath(p.ProjectPath) {
		return
	}
	if a.detail != nil && a.detail.cancel != nil {
		a.detail.cancel()
	}
	a.detail = &detailState{project: p, split: 280, forceUpdate: false}
	a.refreshDetail()
}

func (a *app) refreshDetail() {
	d := a.detail
	if d == nil || d.busy || d.uploading || !a.cli.available() {
		if d != nil && !a.cli.available() {
			d.status = a.t("toolbar.cliMissing")
		}
		return
	}
	d.busy = true
	d.status = a.t("changes.reading")
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
		a.win.Update(func() {
			if a.detail != d {
				return
			}
			d.busy = false
			if err != nil {
				d.status = a.tf("status.refreshFailed", "error", err.Error())
				a.toastMsg = d.status
				return
			}
			d.unstaged = toFileItems(status.Unstaged)
			d.staged = toFileItems(status.Staged)
			if logErr == nil {
				d.logs = logs
				if len(logs) > 0 {
					d.currentVersion = logs[0].Version
				}
			}
			d.shared = shared
			if infoErr == nil {
				d.forceUpdate = info.ForceUpdate
				d.forceKnown = true
			}
			d.status = a.tf("status.refreshed", "unstaged", fmt.Sprint(len(d.unstaged)), "staged", fmt.Sprint(len(d.staged)))
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
	d.status = a.t("changes.reading")
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
		a.win.Update(func() {
			if a.detail != d {
				return
			}
			d.busy = false
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
		a.win.Update(func() {
			if a.detail != d {
				return
			}
			d.busy = false
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
	d.uploadTotal = 0
	d.uploadDone = 0
	d.uploadFailed = 0
	d.uploadText = a.t("changes.reading")
	d.uploadSummary = ""
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
			a.win.Update(func() {
				if a.detail != d {
					return
				}
				applyUploadProgress(a, d, ev.Progress)
			})
		})
		cancel()
		a.win.Update(func() {
			if a.detail != d {
				return
			}
			d.uploading = false
			d.cancel = nil
			d.uploadItems = nil
			d.uploadSummary = ""
			d.uploadText = ""
			d.uploadTotal = 0
			d.uploadDone = 0
			d.uploadFailed = 0
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

func applyUploadProgress(a *app, d *detailState, p UploadProgress) {
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
	switch p.Status {
	case "START":
		d.uploadText = a.tf("publish.uploading", "file", p.File)
	case "DONE":
		d.uploadDone++
		d.uploadText = a.tf("publish.done", "file", p.File)
	case "FAIL":
		d.uploadFailed++
		d.uploadText = a.tf("publish.failed", "file", p.File, "error", p.Error)
	}
	d.uploadSummary = a.tf("publish.summary",
		"done", fmt.Sprint(d.uploadDone),
		"failed", fmt.Sprint(d.uploadFailed),
		"total", fmt.Sprint(d.uploadTotal))
}

func (a *app) cancelPublish() {
	d := a.detail
	if d == nil || !d.uploading || d.cancel == nil {
		return
	}
	d.cancel()
	d.status = a.t("publish.cancelled")
}

func (a *app) openProjectFolder() {
	d := a.detail
	if d == nil {
		return
	}
	path := d.project.ProjectPath
	go func() { _ = mygo.Shell.OpenPath(path) }()
}

func (a *app) launchMain() {
	d := a.detail
	if d == nil {
		return
	}
	exe := resolveProjectPath(d.project.ProjectPath, d.project.MainExePath)
	if exe == "" {
		a.toastMsg = a.t("edit.mainExeHint")
		return
	}
	go func() {
		cmd := exec.Command(exe)
		cmd.Dir = filepath.Dir(exe)
		if err := cmd.Start(); err != nil {
			a.win.Update(func() { a.toastMsg = err.Error() })
		}
	}()
}

func (a *app) openFileLocation(f fileItem) {
	d := a.detail
	if d == nil {
		return
	}
	full := resolveProjectPath(d.project.ProjectPath, f.RelativePath)
	dir := filepath.Dir(full)
	go func() { _ = mygo.Shell.OpenPath(dir) }()
}

func (a *app) openEdit() {
	d := a.detail
	if d == nil {
		return
	}
	shared, err := loadShared(d.project.ProjectPath)
	if err != nil {
		a.toastMsg = a.t("edit.sharedFailed")
		return
	}
	a.editing = &editState{
		project:       d.project,
		shared:        shared,
		original:      shared,
		displayName:   d.project.DisplayName,
		mainExe:       resolveProjectPath(d.project.ProjectPath, d.project.MainExePath),
		ignoreFolders: append([]string(nil), shared.IgnoreFolders...),
		ignoreFiles:   append([]string(nil), shared.IgnoreFiles...),
		uncopyFolders: append([]string(nil), shared.UnCopyFolders...),
		uncopyFiles:   append([]string(nil), shared.UnCopyFiles...),
	}
	a.editOpen = true
}

func (a *app) saveEdit() {
	e := a.editing
	d := a.detail
	if e == nil || d == nil || e.busy {
		return
	}
	e.busy = true
	e.error = ""
	path := d.project.ProjectPath

	// Snapshot the form before the goroutine starts: the view may still
	// rebuild while the CLI calls run.
	displayName := strings.TrimSpace(e.displayName)
	mainExe := e.mainExe
	nextIgnoreFolders := append([]string(nil), e.ignoreFolders...)
	nextIgnoreFiles := append([]string(nil), e.ignoreFiles...)
	nextUncopyFolders := append([]string(nil), e.uncopyFolders...)
	nextUncopyFiles := append([]string(nil), e.uncopyFiles...)
	cli := a.cli

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		var err error
		if !cli.available() {
			err = errCLIMissing
		} else {
			for _, group := range []struct {
				key     string
				current []string
				next    []string
			}{
				{"ignore.folders", e.original.IgnoreFolders, nextIgnoreFolders},
				{"ignore.files", e.original.IgnoreFiles, nextIgnoreFiles},
				{"un_copy.folders", e.original.UnCopyFolders, nextUncopyFolders},
				{"un_copy.files", e.original.UnCopyFiles, nextUncopyFiles},
			} {
				added, removed := diffList(group.current, group.next)
				for _, item := range added {
					if err = cli.configSetArray(ctx, path, group.key, true, item); err != nil {
						break
					}
				}
				if err != nil {
					break
				}
				for _, item := range removed {
					if err = cli.configSetArray(ctx, path, group.key, false, item); err != nil {
						break
					}
				}
				if err != nil {
					break
				}
			}
		}
		a.win.Update(func() {
			if a.editing != e {
				return
			}
			e.busy = false
			if err != nil {
				e.error = err.Error()
				a.toastMsg = a.tf("status.configFailed", "error", err.Error())
				return
			}
			// Update the bookmark and write it atomically.
			newProject := e.project
			newProject.DisplayName = displayName
			if newProject.DisplayName == "" {
				newProject.DisplayName = filepath.Base(newProject.ProjectPath)
			}
			newProject.MainExePath = relativeToProject(newProject.ProjectPath, mainExe)
			for i := range a.projects {
				if normalizePath(a.projects[i].ProjectPath) == normalizePath(newProject.ProjectPath) {
					a.projects[i] = newProject
					break
				}
			}
			sameDetail := a.detail == d && normalizePath(d.project.ProjectPath) == normalizePath(newProject.ProjectPath)
			if sameDetail {
				d.project = newProject
			}
			if saveErr := saveProjects(a.projects); saveErr != nil {
				e.error = saveErr.Error()
				a.toastMsg = saveErr.Error()
				return
			}
			a.editOpen = false
			a.status = a.t("edit.saved")
			a.toastMsg = a.status
			if sameDetail {
				a.refreshDetail()
			}
		})
	}()
}

func (a *app) removeSelected() {
	d := a.detail
	if d == nil {
		return
	}
	next := make([]ProjectConfig, 0, len(a.projects))
	for _, item := range a.projects {
		if normalizePath(item.ProjectPath) != normalizePath(d.project.ProjectPath) {
			next = append(next, item)
		}
	}
	if err := saveProjects(next); err != nil {
		a.toastMsg = err.Error()
		return
	}
	a.projects = next
	if a.detail != nil && a.detail.cancel != nil {
		a.detail.cancel()
	}
	a.detail = nil
	if a.selected >= len(a.filteredProjects()) {
		a.selected = len(a.filteredProjects()) - 1
	}
	if a.selected < 0 && len(a.projects) > 0 {
		a.selected = 0
	}
	a.status = a.t("remove.done")
	a.toastMsg = a.status
	a.openSelected()
}

func (a *app) addLocalProject() {
	a.pickFolder(false, a.t("addLocal.title"), func(paths []string) {
		if len(paths) == 0 {
			return
		}
		path := normalizePath(paths[0])
		for _, p := range a.projects {
			if normalizePath(p.ProjectPath) == path {
				a.toastMsg = a.t("addLocal.duplicate")
				return
			}
		}
		if !isInitialized(path) {
			a.toastMsg = a.t("addLocal.notInit")
			return
		}
		shared, _ := loadShared(path)
		name := strings.TrimSpace(shared.ProjectName)
		if name == "" {
			name = filepath.Base(path)
		}
		next := append(append([]ProjectConfig(nil), a.projects...), ProjectConfig{DisplayName: name, ProjectPath: path})
		if err := saveProjects(next); err != nil {
			a.toastMsg = err.Error()
			return
		}
		a.projects = next
		a.filter = ""
		a.selected = len(a.projects) - 1
		a.status = a.t("addLocal.added")
		a.toastMsg = a.status
		a.openSelected()
	})
}

func (a *app) pickFolder(multiple bool, title string, cb func([]string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.win,
			Title:     title,
			Directory: true,
			Multiple:  multiple,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.win.Update(func() { cb(paths) })
	}()
}

func (a *app) pickFiles(title string, cb func([]string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    title,
			Multiple: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.win.Update(func() { cb(paths) })
	}()
}

func (a *app) pickExecutable(title string, cb func(string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    title,
			Multiple: false,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.win.Update(func() { cb(paths[0]) })
	}()
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.toolbar(c)
		if !a.cli.available() {
			a.cliBanner(c)
		}
		ui.Row(c).Grow(1).Children(func() {
			a.sidebar(c)
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			a.content(c)
		})
		a.statusBar(c)
		if a.editOpen && a.editing != nil {
			a.editModal(c)
		}
		if a.removeOpen {
			switch ui.AlertDialog(c, &a.removeOpen,
				a.t("remove.title"),
				a.t("remove.message"),
				a.t("common.cancel"),
				a.t("common.remove"),
			) {
			case 1:
				a.removeSelected()
			}
		}
		if a.clearOpen {
			switch ui.AlertDialog(c, &a.clearOpen,
				a.t("changes.clearTitle"),
				a.t("changes.clearMessage"),
				a.t("common.cancel"),
				a.t("changes.clear"),
			) {
			case 1:
				a.stageAll(false)
			}
		}
		if a.toastMsg != "" {
			msg := a.toastMsg
			a.toastMsg = ""
			c.Toast(msg)
		}
	})
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(12, 8).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		if ui.PrimaryButton(c, a.t("toolbar.addLocal")).Clicked() {
			a.addLocalProject()
		}
		if ui.Button(c, a.t("toolbar.edit")).Disabled(a.detail == nil).Clicked() {
			a.openEdit()
		}
		if ui.Button(c, a.t("toolbar.remove")).Disabled(a.detail == nil).Clicked() {
			a.removeOpen = true
		}
		if ui.Button(c, a.t("toolbar.refresh")).Disabled(a.detail == nil || a.detail.busy || a.detail.uploading).Clicked() {
			a.refreshDetail()
		}
		ui.Box(c).Grow(1)
		if a.cli.available() {
			ui.Text(c, a.t("toolbar.cliReady")).FontSize(11).TextColor(t.Success)
		} else {
			ui.Text(c, a.t("toolbar.cliMissing")).FontSize(11).TextColor(t.Danger)
		}
		if ui.Button(c, a.t("toolbar.language")).Clicked() {
			a.toggleLang()
		}
	})
}

func (a *app) cliBanner(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(12, 8).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Danger).Children(func() {
		ui.Text(c, a.t("cli.banner")).FontSize(12).TextColor(t.Danger).Grow(1).Wrap()
		if ui.Button(c, a.t("toolbar.retry")).Clicked() {
			a.cli = &cliClient{exe: findCLI()}
			if a.cli.available() {
				a.status = a.tf("cli.found", "path", a.cli.exe)
				a.refreshDetail()
			}
		}
	})
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	list := a.filteredProjects()
	if a.selected >= len(list) {
		a.selected = len(list) - 1
	}
	if a.selected < 0 && len(list) > 0 {
		a.selected = 0
	}
	a.list.Selected = &a.selected
	ui.Column(c).Width(260).FillHeight().Gap(8).Padding(12).Background(t.Surface).Children(func() {
		ui.Text(c, a.t("projects.title")).Bold().FontSize(13)
		if ui.TextInput(c, &a.filter).Placeholder(a.t("projects.search")).Changed() {
			a.selected = 0
			a.openSelected()
		}
		switch {
		case len(a.projects) > 0 && len(list) == 0 && strings.TrimSpace(a.filter) != "":
			ui.Text(c, a.t("projects.noMatch")).FontSize(12).TextColor(t.TextMuted).Wrap()
		case len(list) == 0:
			ui.Text(c, a.t("projects.emptyTitle")).FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, a.t("projects.emptyHint")).FontSize(11).TextColor(t.TextMuted).Wrap()
		default:
			if ui.List(c, &a.list, len(list), func(i int) {
				p := list[i]
				ui.Column(c).Gap(2).Padding(6, 4).Children(func() {
					ui.Text(c, p.DisplayName).FontSize(13).Bold().SingleLine()
					ui.Text(c, p.ProjectPath).FontSize(10).TextColor(t.TextMuted).SingleLine().Tooltip(p.ProjectPath)
				})
			}).Grow(1).Changed() {
				a.openSelected()
			}
		}
	})
}

func (a *app) content(c *ui.Context) {
	t := c.Theme()
	if a.detail == nil {
		ui.Column(c).Grow(1).FillHeight().Center().Gap(8).Children(func() {
			ui.Text(c, a.t("content.emptyTitle")).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, a.t("content.emptyHint")).FontSize(11).TextColor(t.TextMuted).MaxWidth(360).Wrap()
		})
		return
	}
	a.detailView(c, a.detail)
}

func (a *app) detailView(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Gap(8).Padding(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, d.project.DisplayName).FontSize(16).Bold()
			ui.Text(c, d.project.ProjectPath).FontSize(10).TextColor(t.TextMuted).SingleLine().Tooltip(d.project.ProjectPath).Grow(1)
			if ui.Button(c, a.t("actions.folder")).Clicked() {
				a.openProjectFolder()
			}
			if ui.Button(c, a.t("actions.launch")).Clicked() {
				a.launchMain()
			}
			if d.currentVersion != "" {
				ui.Text(c, d.currentVersion).FontSize(11).Padding(8, 2).Background(t.Surface).Radius(10)
			} else {
				ui.Text(c, a.t("history.none")).FontSize(11).TextColor(t.TextMuted)
			}
			if ui.Button(c, a.t("toolbar.refresh")).Disabled(d.busy || d.uploading).Clicked() {
				a.refreshDetail()
			}
			if ui.Button(c, a.t("toolbar.edit")).Clicked() {
				a.openEdit()
			}
			if ui.Button(c, a.t("toolbar.remove")).Clicked() {
				a.removeOpen = true
			}
		})
		if d.busy {
			ui.Progress(c, -1).Width(160)
		}
		ui.Row(c).Grow(1).Gap(12).Children(func() {
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				a.changesView(c, d)
			})
			ui.Column(c).Width(380).FillHeight().Gap(8).Children(func() {
				a.publishView(c, d)
				a.historyView(c, d)
			})
		})
	})
}

func (a *app) changesView(c *ui.Context, d *detailState) {
	ui.Column(c).Grow(1).FillHeight().Gap(8).Children(func() {
		ui.SplitVertical(c, &d.split,
			func() { a.filesPanel(c, d, false) },
			func() { a.filesPanel(c, d, true) },
		).Grow(1)
	})
}

func (a *app) filesPanel(c *ui.Context, d *detailState, staged bool) {
	t := c.Theme()
	files := d.unstaged
	title := a.t("changes.unstaged")
	empty := a.t("changes.emptyUnstaged")
	if staged {
		files = d.staged
		title = a.t("changes.staged")
		empty = a.t("changes.emptyStaged")
	}
	ui.Column(c).Grow(1).FillHeight().Gap(6).Padding(10).Background(t.Surface).
		Border(1, t.Border).Radius(6).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(13).Bold()
			ui.Text(c, fmt.Sprintf("%d", len(files))).FontSize(11).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			label := a.t("changes.selectAll")
			if allSelected(files) {
				label = a.t("changes.unselectAll")
			}
			disabled := d.busy || d.uploading || len(files) == 0
			if ui.Button(c, label).Disabled(disabled).Clicked() {
				selectAll := !allSelected(files)
				for i := range files {
					files[i].Selected = selectAll
				}
				if staged {
					d.staged = files
				} else {
					d.unstaged = files
				}
			}
			hasSelected := len(selectedFiles(files)) > 0
			if staged {
				if ui.Button(c, a.t("changes.unstage")).Disabled(d.busy || d.uploading || !hasSelected).Clicked() {
					a.stageSelected(false)
				}
				if ui.Button(c, a.t("changes.clear")).Disabled(d.busy || d.uploading || len(files) == 0).Clicked() {
					a.clearOpen = true
				}
			} else {
				if ui.Button(c, a.t("changes.stageSelected")).Disabled(d.busy || d.uploading || !hasSelected).Clicked() {
					a.stageSelected(true)
				}
				if ui.PrimaryButton(c, a.t("changes.stageAll")).Disabled(d.busy || d.uploading || len(files) == 0).Clicked() {
					a.stageAll(true)
				}
			}
		})
		if len(files) == 0 {
			ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
				ui.Text(c, empty).FontSize(13).TextColor(t.TextMuted)
				ui.Text(c, a.t("changes.emptyHint")).FontSize(11).TextColor(t.TextMuted)
			})
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("table.select"), Width: 76},
			{Title: a.t("table.file"), Width: 0},
			{Title: a.t("table.status"), Width: 86},
			{Title: a.t("table.local"), Width: 86, Align: ui.End},
			{Title: a.t("table.server"), Width: 86, Align: ui.End},
			{Title: "", Width: 44},
		}
		ui.Table(c, nil, cols, len(files), func(row, col int) {
			f := &files[row]
			switch col {
			case 0:
				ui.Checkbox(c, &f.Selected, "")
			case 1:
				ui.Text(c, f.RelativePath).FontSize(12).SingleLine().Tooltip(f.RelativePath)
			case 2:
				ui.Text(c, statusDisplay(f.Status, a.lang)).FontSize(11).TextColor(statusColor(t, f.Status))
			case 3:
				ui.Text(c, formatSize(f.LocalSize)).FontSize(11).TextColor(t.TextMuted)
			case 4:
				ui.Text(c, formatSize(f.ServerSize)).FontSize(11).TextColor(t.TextMuted)
			case 5:
				if ui.Button(c, "…").Width(28).Height(24).Clicked() {
					a.openFileLocation(*f)
				}
			}
		}).Grow(1)
	})
}

func (a *app) publishView(c *ui.Context, d *detailState) {
	t := c.Theme()
	canPublish := !d.busy && !d.uploading && len(d.staged) > 0 &&
		strings.TrimSpace(d.newVersion) != "" && strings.TrimSpace(d.notes) != ""
	ui.Column(c).Gap(8).Padding(12).Background(t.Surface).
		Border(1, t.Border).Radius(6).Children(func() {
		ui.Text(c, a.t("publish.title")).FontSize(13).Bold()
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.t("publish.version")).FontSize(11).TextColor(t.TextMuted)
			ui.TextInput(c, &d.newVersion).Placeholder(a.t("publish.versionHint")).Grow(1)
			nextVersion := suggestVersion(d.currentVersion)
			if ui.Button(c, "+1").Disabled(d.busy || nextVersion == "").Clicked() {
				d.newVersion = nextVersion
			}
		})
		ui.TextArea(c, &d.notes).Placeholder(a.t("publish.notesHint")).MinHeight(64)
		ui.Collapsible(c, a.t("publish.advanced"), &d.advancedOpen, func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.TextInput(c, &d.script).Placeholder(a.t("publish.scriptHint"))
				ui.Checkbox(c, &d.forceUpdate, a.t("publish.force"))
			})
		})
		if ui.PrimaryButton(c, a.t("publish.publish")).Disabled(!canPublish).Clicked() {
			a.publish()
		}
		hint := ""
		switch {
		case len(d.staged) == 0:
			hint = a.t("publish.stageFirst")
		case strings.TrimSpace(d.newVersion) == "":
			hint = a.t("publish.needVersion")
		case strings.TrimSpace(d.notes) == "":
			hint = a.t("publish.needNotes")
		}
		if hint != "" {
			ui.Text(c, hint).FontSize(11).TextColor(t.Warning)
		}
		if d.uploading || len(d.uploadItems) > 0 {
			ui.Box(c).Height(1).Background(t.Border)
		}
		if d.uploading || len(d.uploadItems) > 0 {
			ui.Text(c, d.uploadText).FontSize(11).TextColor(t.TextMuted).SingleLine()
			progress := -1.0
			if d.uploadTotal > 0 {
				progress = float64(d.uploadDone+d.uploadFailed) / float64(d.uploadTotal)
			}
			ui.Progress(c, progress)
			if d.uploadSummary != "" {
				ui.Text(c, d.uploadSummary).FontSize(11).TextColor(t.TextMuted)
			}
			uploadCols := []ui.TableColumn{
				{Title: a.t("table.file"), Width: 0},
				{Title: a.t("table.local"), Width: 70, Align: ui.End},
				{Title: a.t("table.status"), Width: 120},
			}
			items := d.uploadItems
			ui.Table(c, nil, uploadCols, len(items), func(row, col int) {
				item := items[row]
				switch col {
				case 0:
					ui.Text(c, item.File).FontSize(11).SingleLine().Tooltip(item.File)
				case 1:
					ui.Text(c, formatSize(item.FileSize)).FontSize(11).TextColor(t.TextMuted)
				case 2:
					ui.Text(c, uploadStatusText(item, a.lang)).FontSize(11).
						TextColor(uploadStatusColor(t, item.Status))
				}
			}).MaxHeight(150)
			if d.uploading {
				if ui.Button(c, a.t("publish.cancel")).Clicked() {
					a.cancelPublish()
				}
			}
		}
	})
}

func (a *app) historyView(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Gap(6).Padding(12).Background(t.Surface).
		Border(1, t.Border).Radius(6).Children(func() {
		ui.Text(c, a.t("history.title")).FontSize(13).Bold()
		if len(d.logs) == 0 {
			ui.Text(c, a.t("history.empty")).FontSize(11).TextColor(t.TextMuted)
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("history.version"), Width: 76},
			{Title: a.t("history.time"), Width: 128},
			{Title: a.t("history.notes"), Width: 0},
		}
		logs := d.logs
		ui.Table(c, nil, cols, len(logs), func(row, col int) {
			log := logs[row]
			switch col {
			case 0:
				ui.Text(c, log.Version).FontSize(11).Bold()
			case 1:
				ui.Text(c, log.Time).FontSize(10).TextColor(t.TextMuted)
			case 2:
				ui.Column(c).Gap(2).Children(func() {
					for _, line := range log.Logs {
						ui.Text(c, line).FontSize(11).TextColor(t.TextMuted).Wrap()
					}
				})
			}
		}).Grow(1)
	})
}

func (a *app) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(28).Padding(12, 0).Background(t.Surface).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		status := a.status
		if a.detail != nil && a.detail.status != "" {
			status = a.detail.status
		}
		ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).SingleLine().Grow(1)
		if a.detail != nil {
			ui.Text(c, a.detail.project.ProjectPath).FontSize(10).TextColor(t.TextMuted).SingleLine().
				MaxWidth(420).Tooltip(a.detail.project.ProjectPath)
		}
	})
}

func statusColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case "new":
		return t.Success
	case "modified":
		return t.Warning
	case "deleted":
		return t.Danger
	default:
		return t.TextMuted
	}
}

func uploadStatusText(p UploadProgress, lang string) string {
	switch p.Status {
	case "START":
		if lang == "en" {
			return "Uploading"
		}
		return "上传中"
	case "DONE":
		if lang == "en" {
			return "Done"
		}
		return "完成"
	case "FAIL":
		if p.Error != "" {
			return p.Error
		}
		if lang == "en" {
			return "Failed"
		}
		return "失败"
	}
	return p.Status
}

func uploadStatusColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case "DONE":
		return t.Success
	case "FAIL":
		return t.Danger
	default:
		return t.TextMuted
	}
}

func (a *app) editModal(c *ui.Context) {
	t := c.Theme()
	ui.Modal(c, &a.editOpen, func() {
		e := a.editing
		if e == nil {
			return
		}
		ui.Column(c).Width(760).MaxHeight(620).Gap(8).Padding(16).
			Background(t.Surface).Radius(8).Border(1, t.Border).Children(func() {
			ui.Text(c, a.t("edit.title")).FontSize(16).Bold()
			ui.Text(c, fmt.Sprintf("%s: %s / %s: %s", a.t("edit.server"), e.shared.ServerURL, a.t("edit.project"), e.shared.ProjectName)).
				FontSize(11).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.t("edit.displayName")).FontSize(11).TextColor(t.TextMuted).Width(96)
				ui.TextInput(c, &e.displayName).Grow(1)
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.t("edit.mainExe")).FontSize(11).TextColor(t.TextMuted).Width(96)
				ui.TextInput(c, &e.mainExe).Placeholder(a.t("edit.mainExeHint")).Grow(1)
				if ui.Button(c, a.t("edit.browse")).Clicked() {
					a.pickExecutable(a.t("edit.mainExe"), func(path string) {
						e.mainExe = path
					})
				}
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Row(c).Gap(12).Children(func() {
					ui.Column(c).Grow(1).Gap(8).Children(func() {
						a.editGroup(c, a.t("edit.ignoreFolders"), &e.ignoreFolders, &e.newIgnoreFolder, func() {
							a.pickFolder(true, a.t("edit.ignoreFolders"), func(paths []string) {
								for _, p := range paths {
									a.addProjectPath(&e.ignoreFolders, e.project.ProjectPath, p)
								}
							})
						})
						a.editGroup(c, a.t("edit.ignoreFiles"), &e.ignoreFiles, &e.newIgnoreFile, func() {
							a.pickFiles(a.t("edit.ignoreFiles"), func(paths []string) {
								for _, p := range paths {
									a.addProjectPath(&e.ignoreFiles, e.project.ProjectPath, p)
								}
							})
						})
					})
					ui.Column(c).Grow(1).Gap(8).Children(func() {
						a.editGroup(c, a.t("edit.uncopyFolders"), &e.uncopyFolders, &e.newUncopyFolder, func() {
							a.pickFolder(true, a.t("edit.uncopyFolders"), func(paths []string) {
								for _, p := range paths {
									a.addProjectPath(&e.uncopyFolders, e.project.ProjectPath, p)
								}
							})
						})
						a.editGroup(c, a.t("edit.uncopyFiles"), &e.uncopyFiles, &e.newUncopyFile, func() {
							a.pickFiles(a.t("edit.uncopyFiles"), func(paths []string) {
								for _, p := range paths {
									a.addProjectPath(&e.uncopyFiles, e.project.ProjectPath, p)
								}
							})
						})
					})
				})
			})
			if e.error != "" {
				ui.Text(c, e.error).FontSize(11).TextColor(t.Danger).Wrap()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, a.t("edit.cancel")).Disabled(e.busy).Clicked() {
					a.editOpen = false
				}
				if ui.PrimaryButton(c, a.t("edit.save")).Disabled(e.busy).Clicked() {
					a.saveEdit()
				}
			})
		})
	})
}

func (a *app) editGroup(c *ui.Context, label string, list *[]string, input *string, pick func()) {
	t := c.Theme()
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(11).Bold().TextColor(t.TextMuted)
		items := append([]string(nil), (*list)...)
		for _, value := range items {
			value := value
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, value).FontSize(11).Grow(1).SingleLine().Tooltip(value)
				if ui.Button(c, "×").Width(24).Height(22).Clicked() {
					*list = removeValue(*list, value)
				}
			})
		}
		ui.Row(c).Gap(4).Children(func() {
			if ui.TextInput(c, input).Placeholder(label).Grow(1).Submitted() {
				if v := strings.TrimSpace(*input); v != "" {
					*list = appendUnique(*list, v)
					*input = ""
				}
			}
			if ui.Button(c, a.t("edit.add")).Clicked() {
				if v := strings.TrimSpace(*input); v != "" {
					*list = appendUnique(*list, v)
					*input = ""
				}
			}
			if ui.Button(c, a.t("edit.pick")).Clicked() {
				pick()
			}
		})
	})
}

func removeValue(list []string, value string) []string {
	for i, item := range list {
		if item == value {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func (a *app) addProjectPath(list *[]string, projectPath, path string) {
	rel, ok := projectRelativePath(projectPath, path)
	if !ok {
		a.toastMsg = a.t("edit.outside")
		return
	}
	*list = appendUnique(*list, rel)
}

func projectRelativePath(projectPath, path string) (string, bool) {
	rel := relativeToProject(projectPath, path)
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	return rel, true
}

func appendUnique(list []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return list
	}
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}
