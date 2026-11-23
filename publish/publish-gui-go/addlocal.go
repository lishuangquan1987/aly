package main

import (
	"os"
	"path/filepath"
	"strings"
)

// localPreviewState is the add-local-project dialog: the folder field, the
// preview of the .updator/shared.json found there, and the display name the
// project is bookmarked under. It mirrors the C# AddLocalProjectDialog: the
// folder is chosen inside the dialog and only the confirm step bookmarks it,
// so an invalid pick never leaves a broken entry behind.
type localPreviewState struct {
	path        string
	shared      SharedConfig
	displayName string

	// loaded reports that shared.json was read for the current folder, so the
	// preview card may show its values.
	loaded bool
	// loadErr explains why an initialized folder's shared.json could not be
	// read (invalid JSON, permission denied).
	loadErr string

	busy  bool
	error string
}

// addLocalProject opens the add-local dialog with an empty folder; the folder
// is chosen, typed or dropped inside the dialog where its config is previewed.
func (a *app) addLocalProject() {
	a.localPreview = &localPreviewState{}
	a.localPreviewOpen = true
}

func (a *app) closeLocalPreview() {
	a.localPreviewOpen = false
	a.localPreview = nil
}

// hasPreview reports whether the preview card has values to show. A state
// built directly (tests, a restored session) still previews its config even
// when its folder could not be examined here.
func (p *localPreviewState) hasPreview() bool {
	if p.loaded {
		return true
	}
	s := p.shared
	return strings.TrimSpace(s.ServerURL) != "" || strings.TrimSpace(s.ProjectName) != "" ||
		len(s.IgnoreFolders) > 0 || len(s.IgnoreFiles) > 0 ||
		len(s.UnCopyFolders) > 0 || len(s.UnCopyFiles) > 0
}

// suggestedLocalDisplayName is the initial display name, as
// ProjectInputHelper.SuggestDisplayName in the C# GUI: the server project name
// when the config names one, else the folder name.
func suggestedLocalDisplayName(path string, shared SharedConfig) string {
	if name := strings.TrimSpace(shared.ProjectName); name != "" {
		return name
	}
	path = strings.TrimRight(filepath.Clean(path), string(filepath.Separator))
	return filepath.Base(path)
}

// bookmarked reports whether path already is one of a's projects. The C#
// dialog compares full normalized paths ignoring case.
func (a *app) bookmarked(path string) bool {
	path = normalizeForCompare(path)
	if path == "" {
		return false
	}
	for _, project := range a.projects {
		if strings.EqualFold(normalizeForCompare(project.ProjectPath), path) {
			return true
		}
	}
	return false
}

// localPreviewStateText is the live verdict under the folder field: empty,
// missing, already bookmarked, not initialized, unreadable, or found. It reuses
// projectPathState for the folder checks and adds the add-local specifics, as
// the C# dialog's OnProjectPathChanged does. ok reports that the preview is
// valid and the project can be added.
func (a *app) localPreviewStateText(p *localPreviewState) (text string, ok bool) {
	path := normalizeForCompare(p.path)
	if a.bookmarked(path) {
		return a.t("addLocal.duplicate"), false
	}
	switch projectPathState(p.path, a.projects) {
	case pathEmpty:
		return a.t("addLocal.folderHint"), false
	case pathMissing:
		return a.t("addLocal.dirMissing"), false
	case pathUsable:
		// The folder exists but carries no .updator/shared.json yet.
		return a.t("addLocal.notInit"), false
	}
	// pathInitialized: the folder may be added once its config loads.
	if p.loadErr != "" {
		return p.loadErr, false
	}
	if !p.loaded {
		return a.t("addLocal.notInit"), false
	}
	return a.t("addLocal.configFound"), true
}

// localPreviewReady reports whether the confirm button may add the project.
func (a *app) localPreviewReady(p *localPreviewState) bool {
	if p == nil || p.busy || !p.hasPreview() {
		return false
	}
	_, ok := a.localPreviewStateText(p)
	return ok
}

// refreshLocalPreview re-reads .updator/shared.json for the dialog's folder and
// suggests a display name while the user has not typed one. Called whenever the
// folder changes (typed, browsed or dropped).
func (a *app) refreshLocalPreview(p *localPreviewState) {
	if p == nil {
		return
	}
	p.loadErr = ""
	p.loaded = false
	p.shared = SharedConfig{}
	path := normalizeForCompare(p.path)
	if path == "" {
		return
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return
	}
	if a.bookmarked(path) {
		return
	}
	if !isInitialized(path) {
		return
	}
	shared, err := loadShared(path)
	if err != nil {
		p.loadErr = a.t("addLocal.invalidConfig")
		return
	}
	p.shared = shared
	p.loaded = true
	if strings.TrimSpace(p.displayName) == "" {
		p.displayName = suggestedLocalDisplayName(path, shared)
	}
}

// confirmLocalProject bookmarks the previewed folder. The bookmark write is
// synchronous so the list cannot show a project whose config was never saved;
// the sibling UpdateFolder refresh follows it (in the background when there is
// a window, so the dialog can show its busy row).
func (a *app) confirmLocalProject() {
	p := a.localPreview
	if p == nil || p.busy {
		return
	}
	path := normalizeForCompare(p.path)
	if path == "" {
		p.error = a.t("addLocal.folderHint")
		return
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		p.error = a.t("addLocal.dirMissing")
		return
	}
	if a.bookmarked(path) {
		p.error = a.t("addLocal.duplicate")
		return
	}
	if !isInitialized(path) {
		p.error = a.t("addLocal.notInit")
		return
	}
	shared, err := loadShared(path)
	if err != nil {
		p.error = a.t("addLocal.invalidConfig")
		return
	}
	// The config was re-read above, so a file changed while the dialog was
	// open cannot be bookmarked with stale values.
	p.shared = shared
	p.loaded = true
	p.loadErr = ""
	name := strings.TrimSpace(p.displayName)
	if name == "" {
		name = suggestedLocalDisplayName(path, shared)
	}
	next := append(append([]ProjectConfig(nil), a.projects...), ProjectConfig{
		DisplayName: name,
		ProjectPath: path,
	})
	if err := saveProjects(next); err != nil {
		p.error = err.Error()
		return
	}
	a.projects = next
	a.filter = ""
	a.selected = len(a.projects) - 1
	a.status = a.t("addLocal.added")
	a.toastMsg = a.status
	p.error = ""
	p.busy = true

	finish := func(warning string) {
		if a.localPreview != p {
			return
		}
		p.busy = false
		if warning != "" {
			a.status = warning
			a.toastMsg = warning
		}
		a.closeLocalPreview()
		a.openSelected()
	}
	if a.win == nil {
		// No frame loop (tests, headless): the copy runs inline so the dialog
		// state has settled before the next frame reads it.
		warning, err := refreshUpdaters(path)
		if err != nil {
			warning = err.Error()
		}
		finish(warning)
		return
	}
	go func() {
		warning, err := refreshUpdaters(path)
		if err != nil {
			warning = err.Error()
		}
		a.post(func() { finish(warning) })
	}()
}
