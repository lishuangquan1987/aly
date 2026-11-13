package main

import (
	"context"
	"errors"
	"fmt"
	"os"
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
}

type editState struct {
	project ProjectConfig
	shared  SharedConfig

	displayName string
	mainExe     string
	serverURL   string
	projectName string

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

	status     string
	toastMsg   string
	theme      string
	accent     string
	accentOpen bool
	addOpen    bool
	adding     *addProjectState

	localPreviewOpen bool
	localPreview     *localPreviewState

	// details keeps per-project tab state (C# ProjectTabViewModel cache).
	details map[string]*detailState
}

func newApp() *app {
	settings := loadSettings()
	appearance := loadAppearance()
	lang := "zh"
	if strings.HasPrefix(strings.ToLower(settings.Language), "en") {
		lang = "en"
	}
	theme := normalizeTheme(appearance.Theme)
	accent := normalizeAccent(appearance.Accent)
	a := &app{
		lang:     lang,
		theme:    theme,
		accent:   accent,
		projects: loadProjects(),
		selected: -1,
		details:  map[string]*detailState{},
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

func (a *app) tf(key string, pairs ...string) string { return tfLang(a.lang, key, pairs...) }

func tfLang(lang, key string, pairs ...string) string {
	s := translate(lang, key)
	for i := 0; i+1 < len(pairs); i += 2 {
		s = strings.ReplaceAll(s, "{"+pairs[i]+"}", pairs[i+1])
	}
	return s
}

func normalizeTheme(value string) string {
	switch value {
	case "light", "dark", "system":
		return value
	default:
		return "system"
	}
}

func nextTheme(current string) string {
	switch normalizeTheme(current) {
	case "system":
		return "light"
	case "light":
		return "dark"
	default:
		return "system"
	}
}

func (a *app) themeSource() mygo.ThemeSource {
	switch a.theme {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

func (a *app) applyTheme() {
	mygo.Theme.SetSource(a.themeSource())
}

func (a *app) savePrefs() {
	_ = saveSettings(appSettings{
		Language: map[bool]string{true: "zh-CN", false: "en-US"}[a.lang == "zh"],
	})
	_ = saveAppearance(appearanceSettings{Theme: a.theme, Accent: a.accent})
}

type accentOption struct {
	ID    string
	Hex   string
	Label string
}

var accentPalette = []accentOption{
	{"blue", "#2563eb", "accent.blue"},
	{"teal", "#0d9488", "accent.teal"},
	{"green", "#16a34a", "accent.green"},
	{"violet", "#7c3aed", "accent.violet"},
	{"orange", "#ea580c", "accent.orange"},
	{"rose", "#e11d48", "accent.rose"},
	{"indigo", "#4f46e5", "accent.indigo"},
}

func normalizeAccent(value string) string {
	for _, opt := range accentPalette {
		if opt.ID == value {
			return value
		}
	}
	return "blue"
}

func accentHex(value string) string {
	for _, opt := range accentPalette {
		if opt.ID == value {
			return opt.Hex
		}
	}
	return "#2563eb"
}

func (a *app) applyAccent(c *ui.Context) {
	base := *c.Theme()
	accent := ui.Hex(accentHex(normalizeAccent(a.accent)))
	base.Accent = accent
	base.AccentHover = accent.MixOklab(ui.Hex("#ffffff"), 0.14)
	base.AccentPressed = accent.MixOklab(ui.Hex("#000000"), 0.16)
	base.AccentText = ui.Hex("#ffffff")
	base.InactiveAccent = accent
	c.SetTheme(&base)
}

func (a *app) cycleTheme() {
	a.theme = nextTheme(a.theme)
	a.applyTheme()
	a.savePrefs()
	if a.win != nil {
		a.win.Update(func() {})
	}
}

func (a *app) themeLabel() string {
	switch normalizeTheme(a.theme) {
	case "light":
		return a.t("theme.light")
	case "dark":
		return a.t("theme.dark")
	default:
		return a.t("theme.system")
	}
}

func (a *app) themeIcon() *ui.SVG {
	switch normalizeTheme(a.theme) {
	case "light":
		return iconSun
	case "dark":
		return iconMoon
	default:
		return iconMonitor
	}
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
	a.savePrefs()
	if a.win != nil {
		a.win.SetTitle(a.t("app.title"))
	}
	a.status = a.t("status.ready")
	a.toastMsg = ""
	if a.detail != nil && !a.detail.busy && !a.detail.uploading {
		a.detail.status = a.tf("status.refreshed",
			"unstaged", fmt.Sprint(len(a.detail.unstaged)),
			"staged", fmt.Sprint(len(a.detail.staged)))
	}
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
	if a.details == nil {
		a.details = map[string]*detailState{}
	}
	key := normalizePath(p.ProjectPath)
	if a.detail != nil && normalizePath(a.detail.project.ProjectPath) == key {
		return
	}
	// Keep per-project form/version/selection state when switching, like the
	// C# ProjectTabViewModel cache. An in-flight upload is cancelled before
	// its tab is hidden, because the hidden state has no progress pump.
	if a.detail != nil && a.detail.uploading && a.detail.cancel != nil {
		a.detail.cancel()
		a.detail.uploading = false
		a.detail.cancel = nil
		a.detail.status = a.t("publish.cancelled")
	}
	// A refresh/stage callback ignores itself when the visible detail changed;
	// reset busy so the cached tab cannot come back permanently disabled.
	if a.detail != nil {
		a.detail.busy = false
	}
	if cached := a.details[key]; cached != nil {
		a.detail = cached
		return
	}
	a.detail = &detailState{project: p, splitPercent: 62, forceUpdate: false}
	a.details[key] = a.detail
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

func (a *app) openCLIFolder() {
	if !a.cli.available() {
		a.toastMsg = a.t("toolbar.cliMissing")
		return
	}
	dir := filepath.Dir(a.cli.exe)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		a.toastMsg = a.t("folder.missing")
		return
	}
	go func() { _ = mygo.Shell.OpenPath(dir) }()
}

func (a *app) openLogFolder() {
	dir := logDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.toastMsg = err.Error()
		return
	}
	go func() { _ = mygo.Shell.OpenPath(dir) }()
}

func (a *app) openProjectFolder() {
	d := a.detail
	if d == nil {
		return
	}
	path := d.project.ProjectPath
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		a.toastMsg = a.t("folder.missing")
		return
	}
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
	if !fileExists(exe) {
		a.toastMsg = a.tf("launch.missing", "path", exe)
		return
	}
	switch strings.ToLower(filepath.Ext(exe)) {
	case ".bat", ".cmd":
		go func() { _ = mygo.Shell.OpenPath(exe) }()
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
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		a.toastMsg = a.t("folder.missing")
		return
	}
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
		displayName:   d.project.DisplayName,
		mainExe:       resolveProjectPath(d.project.ProjectPath, d.project.MainExePath),
		serverURL:     shared.ServerURL,
		projectName:   shared.ProjectName,
		ignoreFolders: append([]string(nil), shared.IgnoreFolders...),
		ignoreFiles:   append([]string(nil), shared.IgnoreFiles...),
		uncopyFolders: append([]string(nil), shared.UnCopyFolders...),
		uncopyFiles:   append([]string(nil), shared.UnCopyFiles...),
	}
	a.editOpen = true
}

func (a *app) syncIgnoreRules(ctx context.Context, cli *cliClient, lang, serverURL, projectName string, ignoreFolders, ignoreFiles []string) string {
	if serverURL == "" || projectName == "" {
		return ""
	}
	projects, err := cli.projectList(ctx, serverURL)
	if err != nil {
		return tfLang(lang, "status.syncFailed", "error", err.Error())
	}
	var match ProjectInfo
	found := false
	for _, p := range projects {
		if strings.EqualFold(p.Name, projectName) {
			match, found = p, true
			break
		}
	}
	if !found {
		return tfLang(lang, "status.syncMissing", "project", projectName)
	}
	if err := cli.projectUpdate(ctx, serverURL, match.Name, match.Title, match.ForceUpdate, ignoreFolders, ignoreFiles); err != nil {
		return tfLang(lang, "status.syncFailed", "error", err.Error())
	}
	return ""
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
	serverURL := strings.TrimSpace(e.serverURL)
	projectName := strings.TrimSpace(e.projectName)
	nextIgnoreFolders := append([]string(nil), e.ignoreFolders...)
	nextIgnoreFiles := append([]string(nil), e.ignoreFiles...)
	nextUncopyFolders := append([]string(nil), e.uncopyFolders...)
	nextUncopyFiles := append([]string(nil), e.uncopyFiles...)
	cli := a.cli
	lang := a.lang

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		var err error
		syncWarning := ""
		updaterWarning := ""
		if serverURL == "" || projectName == "" {
			err = errors.New(translate(lang, "edit.required"))
		} else if !cli.available() {
			err = errCLIMissing
		} else {
			// Re-read the current file so an old dialog snapshot cannot
			// overwrite changes made elsewhere.
			current, loadErr := loadShared(path)
			if loadErr != nil {
				err = loadErr
			} else {
				changed := false
				if current.ServerURL != serverURL {
					if err = cli.configSet(ctx, path, "server.url", serverURL); err != nil {
						goto done
					}
					changed = true
				}
				for _, group := range []struct {
					key     string
					current []string
					next    []string
				}{
					{"ignore.folders", current.IgnoreFolders, nextIgnoreFolders},
					{"ignore.files", current.IgnoreFiles, nextIgnoreFiles},
					{"un_copy.folders", current.UnCopyFolders, nextUncopyFolders},
					{"un_copy.files", current.UnCopyFiles, nextUncopyFiles},
				} {
					added, removed := diffList(group.current, group.next)
					for _, item := range added {
						if err = cli.configSetArray(ctx, path, group.key, true, item); err != nil {
							goto done
						}
						changed = true
					}
					for _, item := range removed {
						if err = cli.configSetArray(ctx, path, group.key, false, item); err != nil {
							goto done
						}
						changed = true
					}
				}
				if current.ProjectName != projectName {
					if err = cli.configSet(ctx, path, "project.name", projectName); err != nil {
						goto done
					}
					changed = true
				}
				if changed {
					syncWarning = a.syncIgnoreRules(ctx, cli, lang, serverURL, projectName, nextIgnoreFolders, nextIgnoreFiles)
				}
				updaterWarning = ensureUpdatersFor(path, mainExe)
			}
		}
	done:
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
			switch {
			case syncWarning != "":
				a.status = syncWarning
				a.toastMsg = syncWarning
			case updaterWarning != "":
				a.status = updaterWarning
				a.toastMsg = updaterWarning
			default:
				a.toastMsg = a.status
			}
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
	if a.detail != nil {
		delete(a.details, normalizePath(a.detail.project.ProjectPath))
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
		ui.Column(c).Width(820).MaxHeight(680).Gap(12).Padding(18).
			Background(t.Surface).Radius(12).Border(1, t.Border).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconSettings).FontSize(18).TextColor(t.Accent)
				ui.Text(c, a.t("edit.title")).FontSize(16).Bold()
				ui.Box(c).Grow(1)
				if a.ghostIconButton(c, a.t("edit.cancel"), iconX, e.busy) {
					a.editOpen = false
				}
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconInfo).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("%s: %s   ·   %s: %s", a.t("edit.server"), e.shared.ServerURL, a.t("edit.project"), e.shared.ProjectName)).
					FontSize(11).TextColor(t.TextMuted).SingleLine().Grow(1)
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.t("edit.serverURL")).FontSize(11).TextColor(t.TextMuted).Width(104)
				ui.TextInput(c, &e.serverURL).Grow(1)
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.t("edit.projectName")).FontSize(11).TextColor(t.TextMuted).Width(104)
				ui.TextInput(c, &e.projectName).Grow(1)
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.t("edit.displayName")).FontSize(11).TextColor(t.TextMuted).Width(104)
				ui.TextInput(c, &e.displayName).Grow(1)
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.t("edit.mainExe")).FontSize(11).TextColor(t.TextMuted).Width(104)
				ui.TextInput(c, &e.mainExe).Placeholder(a.t("edit.mainExeHint")).Grow(1)
				if a.actionButton(c, a.t("edit.browse"), iconFolder, false, false) {
					a.pickExecutable(a.t("edit.mainExe"), func(path string) {
						e.mainExe = path
					})
				}
			})
			ui.Box(c).Height(1).Background(t.Border)
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Row(c).Gap(14).Children(func() {
					ui.Column(c).Grow(1).Gap(10).Children(func() {
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
					ui.Column(c).Grow(1).Gap(10).Children(func() {
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
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconWarning).FontSize(12).TextColor(t.Danger)
					ui.Text(c, e.error).FontSize(11).TextColor(t.Danger).Wrap()
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if a.actionButton(c, a.t("edit.cancel"), iconX, false, e.busy) {
					a.editOpen = false
				}
				if a.actionButton(c, a.t("edit.save"), iconCheck, true, e.busy || strings.TrimSpace(e.serverURL) == "" || strings.TrimSpace(e.projectName) == "") {
					a.saveEdit()
				}
			})
		})
	})
}

func (a *app) editGroup(c *ui.Context, label string, list *[]string, input *string, pick func()) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(11).Bold().TextColor(t.TextMuted)
			a.countPill(c, len(*list))
		})
		items := append([]string(nil), (*list)...)
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for _, value := range items {
				value := value
				ui.Row(c).Gap(2).AlignItems(ui.Center).Padding(6, 1).Background(t.Text.Alpha(0.06)).
					Border(1, t.Border).Radius(5).MaxWidth(320).Children(func() {
					ui.Text(c, value).FontSize(11).SingleLine().Tooltip(value)
					if a.ghostIconButtonSize(c, a.t("edit.remove"), iconX, false, 16, 16) {
						*list = removeValue(*list, value)
					}
				})
			}
		})
		ui.Row(c).Gap(5).Children(func() {
			if ui.TextInput(c, input).Placeholder(label).Grow(1).Submitted() {
				if v := strings.TrimSpace(*input); v != "" {
					*list = appendUnique(*list, v)
					*input = ""
				}
			}
			if a.actionButton(c, a.t("edit.add"), iconPlus, false, false) {
				if v := strings.TrimSpace(*input); v != "" {
					*list = appendUnique(*list, v)
					*input = ""
				}
			}
			if a.actionButton(c, a.t("edit.pick"), iconFolder, false, false) {
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
