package main

import (
	"path/filepath"
	"strings"
)

// localPreviewState mirrors the C# AddLocalProject dialog: parse the project
// once, show the server/project/ignore preview, and only bookmark on confirm.
type localPreviewState struct {
	path        string
	shared      SharedConfig
	displayName string
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
		shared, err := loadShared(path)
		if err != nil {
			a.toastMsg = a.t("addLocal.invalidConfig")
			return
		}
		name := strings.TrimSpace(shared.ProjectName)
		if name == "" {
			name = filepath.Base(path)
		}
		a.localPreviewOpen = true
		a.localPreview = &localPreviewState{path: path, shared: shared, displayName: name}
	})
}

func (a *app) closeLocalPreview() {
	a.localPreviewOpen = false
	a.localPreview = nil
}

func (a *app) confirmLocalProject() {
	p := a.localPreview
	if p == nil {
		return
	}
	name := strings.TrimSpace(p.displayName)
	if name == "" {
		name = filepath.Base(p.path)
	}
	for _, existing := range a.projects {
		if normalizePath(existing.ProjectPath) == p.path {
			a.toastMsg = a.t("addLocal.duplicate")
			return
		}
	}
	next := append(append([]ProjectConfig(nil), a.projects...), ProjectConfig{
		DisplayName: name,
		ProjectPath: p.path,
	})
	if err := saveProjects(next); err != nil {
		a.toastMsg = err.Error()
		return
	}
	a.projects = next
	a.filter = ""
	a.selected = len(a.projects) - 1
	a.status = a.t("addLocal.added")
	a.toastMsg = a.status
	if warning, _ := refreshUpdaters(p.path); warning != "" {
		a.toastMsg = warning
	}
	a.closeLocalPreview()
	a.openSelected()
}
