package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// addProjectState is the Go counterpart of the C# AddProject wizard's
// server-project step. It intentionally keeps the first slice small: pick or
// create a server project, choose its local folder, then run config init.
type addProjectState struct {
	serverURL   string
	projectName string
	title       string
	path        string
	displayName string
	mainExe     string
	forceUpdate bool

	projects []ProjectInfo
	list     ui.ListState
	selected int

	fetching bool
	busy     bool
	error    string
}

func knownServerURL(projects []ProjectConfig) string {
	seen := map[string]bool{}
	for _, p := range projects {
		shared, err := loadShared(p.ProjectPath)
		if err != nil || shared.ServerURL == "" || seen[shared.ServerURL] {
			continue
		}
		seen[shared.ServerURL] = true
		return shared.ServerURL
	}
	return ""
}

func (a *app) openAddProject() {
	a.adding = &addProjectState{
		serverURL: knownServerURL(a.projects),
		selected:  -1,
	}
	a.addOpen = true
}

func (a *app) closeAddProject() {
	a.addOpen = false
	a.adding = nil
}

func (a *app) fetchServerProjects() {
	s := a.adding
	if s == nil || s.fetching || s.busy {
		return
	}
	if strings.TrimSpace(s.serverURL) == "" {
		s.error = a.t("addProject.serverRequired")
		return
	}
	s.fetching = true
	s.error = ""
	serverURL := strings.TrimSpace(s.serverURL)
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		projects, err := cli.projectList(ctx, serverURL)
		a.win.Update(func() {
			if a.adding != s {
				return
			}
			s.fetching = false
			if err != nil {
				s.error = a.tf("addProject.fetchFailed", "error", err.Error())
				return
			}
			s.projects = projects
			s.selected = -1
			// Keep the project the user just created/typed when it is still
			// in the refreshed list; otherwise fall back to the first row.
			for i, p := range projects {
				if strings.EqualFold(p.Name, strings.TrimSpace(s.projectName)) {
					s.selected = i
					break
				}
			}
			if s.selected < 0 && len(projects) > 0 {
				s.selected = 0
			}
			if s.selected >= 0 {
				p := projects[s.selected]
				s.projectName = p.Name
				s.title = p.Title
				s.forceUpdate = p.ForceUpdate
				if s.displayName == "" {
					s.displayName = p.Name
				}
			}
		})
	}()
}

func (a *app) createServerProject() {
	s := a.adding
	if s == nil || s.busy || s.fetching {
		return
	}
	serverURL := strings.TrimSpace(s.serverURL)
	name := strings.TrimSpace(s.projectName)
	title := strings.TrimSpace(s.title)
	forceUpdate := s.forceUpdate
	if serverURL == "" || name == "" {
		s.error = a.t("addProject.required")
		return
	}
	if title == "" {
		title = name
	}
	s.busy = true
	s.error = ""
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		info, err := cli.projectCreate(ctx, serverURL, name, title, forceUpdate, nil, nil)
		a.win.Update(func() {
			if a.adding != s {
				return
			}
			s.busy = false
			if err != nil {
				s.error = a.tf("addProject.createFailed", "error", err.Error())
				return
			}
			s.projectName = info.Name
			if info.Name == "" {
				s.projectName = name
			}
			s.title = info.Title
			if s.title == "" {
				s.title = title
			}
			if s.displayName == "" {
				s.displayName = s.projectName
			}
			a.fetchServerProjects()
		})
	}()
}

func (a *app) initializeAddProject() {
	s := a.adding
	if s == nil || s.busy || s.fetching {
		return
	}
	serverURL := strings.TrimSpace(s.serverURL)
	name := strings.TrimSpace(s.projectName)
	path := normalizePath(strings.TrimSpace(s.path))
	displayName := strings.TrimSpace(s.displayName)
	mainExe := s.mainExe
	if serverURL == "" || name == "" || path == "" {
		s.error = a.t("addProject.required")
		return
	}
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		s.error = a.t("folder.missing")
		return
	}
	if isInitialized(path) {
		s.error = a.t("addProject.alreadyInitialized")
		return
	}
	if strings.TrimSpace(mainExe) != "" && !fileExists(resolveProjectPath(path, mainExe)) {
		s.error = a.t("addProject.mainMissing")
		return
	}
	if displayName == "" {
		displayName = name
	}
	s.busy = true
	s.error = ""
	cli := a.cli
	baseProjects := append([]ProjectConfig(nil), a.projects...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		warning := ""
		var err error
		if !cli.available() {
			err = errCLIMissing
		} else if err = cli.initProject(ctx, path, serverURL, name, nil, nil, nil, nil); err == nil {
			if strings.TrimSpace(mainExe) != "" {
				w, createErr := createUpdateFolder(path, mainExe, "")
				if createErr != nil {
					err = createErr
				} else {
					warning = w
				}
			}
		}
		var next []ProjectConfig
		if err == nil {
			next = append(baseProjects, ProjectConfig{
				DisplayName: displayName,
				ProjectPath: path,
				MainExePath: relativeToProject(path, mainExe),
			})
			if saveErr := saveProjects(next); saveErr != nil {
				err = saveErr
			}
		}
		a.win.Update(func() {
			if a.adding != s {
				return
			}
			s.busy = false
			if err != nil {
				s.error = a.tf("addProject.initFailed", "error", err.Error())
				return
			}
			a.projects = next
			a.addOpen = false
			a.adding = nil
			a.filter = ""
			a.selected = len(a.projects) - 1
			a.status = a.tf("addProject.added", "name", displayName)
			a.toastMsg = a.status
			if warning != "" {
				a.toastMsg = warning
			}
			a.openSelected()
		})
	}()
}
