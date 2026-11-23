package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

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

	status      string
	statusTone  tone
	focusSearch bool
	toastMsg    string
	toastKind   toastKind
	toasts      []toast
	theme       string
	accent      string
	accentOpen  bool
	addOpen     bool
	adding      *addProjectState

	localPreviewOpen bool
	localPreview     *localPreviewState

	// busyText labels the background operation the detail page is running
	// (refresh, staging or publishing), shown under the toolbar and in the
	// status bar.
	busyText string

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
		a.statusTone = toneSuccess
	} else {
		a.status = a.t("toolbar.cliMissing")
		a.statusTone = toneDanger
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

func (a *app) cycleTheme() {
	a.theme = nextTheme(a.theme)
	a.applyTheme()
	a.savePrefs()
	a.post(func() {})
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
	a.detail = &detailState{project: p, splitPercent: 50, publishPercent: 50, forceUpdate: false}
	a.details[key] = a.detail
	a.refreshDetail()
}

func (a *app) removeSelected() {
	d := a.detail
	if d == nil {
		return
	}
	removed := normalizePath(d.project.ProjectPath)
	next := make([]ProjectConfig, 0, len(a.projects))
	for _, item := range a.projects {
		if normalizePath(item.ProjectPath) != removed {
			next = append(next, item)
		}
	}
	if err := saveProjects(next); err != nil {
		a.toastMsg = err.Error()
		return
	}
	if d.cancel != nil {
		d.cancel()
	}
	a.projects = next
	delete(a.details, removed)
	a.detail = nil
	// Keep the position the removed row had: the row that took its place,
	// or the last one when it was the last.
	if list := a.filteredProjects(); len(list) == 0 {
		a.selected = -1
	} else if a.selected >= len(list) {
		a.selected = len(list) - 1
	} else if a.selected < 0 {
		a.selected = 0
	}
	a.status = a.t("remove.done")
	a.statusTone = toneSuccess
	a.toastMsg = a.status
	a.openSelected()
}

// post runs fn on the UI thread: through the window's Update while it runs,
// and directly in tests, which have no window.
func (a *app) post(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
		return
	}
	fn()
}

// beginBusy shows work of unknown length in the window: the label under the
// toolbar and the spinner in the status bar.
func (a *app) beginBusy(text string) { a.busyText = text }

// endBusy clears the window-level busy label.
func (a *app) endBusy() { a.busyText = "" }
