package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

// editState is the project settings dialog: the bookmark fields plus the
// .updator/shared.json values, the load/validation messages, and the four rule
// lists edited in place.
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

	// knownServers are the server URLs of the other bookmarks offered as
	// one-click chips; serversLoaded keeps the view from re-reading every
	// bookmark's shared.json on each frame.
	knownServers  []string
	serversLoaded bool

	busy  bool
	error string

	// loadError says shared.json could not be read: the dialog stays open with
	// its fields disabled and this banner instead of a toast that hides it.
	loadError string
	// validation is the message under the fields while a required field is
	// empty; Save stays disabled.
	validation string
}

// openEdit opens the project settings dialog for the visible project. A
// malformed .updator/shared.json still opens it, disabled, with the reason in a
// banner (as the C# EditProjectDialog does) rather than as a toast.
func (a *app) openEdit() {
	d := a.detail
	if d == nil {
		return
	}
	path := d.project.ProjectPath
	e := &editState{
		project:       d.project,
		displayName:   d.project.DisplayName,
		mainExe:       resolveProjectPath(path, d.project.MainExePath),
		knownServers:  knownServerURLs(a.projects),
		serversLoaded: true,
	}
	if shared, err := loadShared(path); err != nil {
		e.loadError = a.tf("edit.loadFailed", "error", err.Error())
	} else {
		e.shared = shared
		e.serverURL = shared.ServerURL
		e.projectName = shared.ProjectName
		e.ignoreFolders = append([]string(nil), shared.IgnoreFolders...)
		e.ignoreFiles = append([]string(nil), shared.IgnoreFiles...)
		e.uncopyFolders = append([]string(nil), shared.UnCopyFolders...)
		e.uncopyFiles = append([]string(nil), shared.UnCopyFiles...)
	}
	e.validation = a.editValidation(e)
	a.editing = e
	a.editOpen = true
}

// editValidation is the form's invalid reason: the server target and the list
// name are required before anything can be saved. Mirrors
// EditProjectDialogViewModel.UpdateValidation, with the display name added
// because the Go bookmark requires one.
func (a *app) editValidation(e *editState) string {
	switch {
	case strings.TrimSpace(e.serverURL) == "":
		return a.t("edit.requireServer")
	case strings.TrimSpace(e.projectName) == "":
		return a.t("edit.requireProject")
	case strings.TrimSpace(e.displayName) == "":
		return a.t("edit.requireDisplayName")
	}
	return ""
}

// mainExeHintText says what saving the main executable field will store, as
// EditProjectDialogViewModel.MainExeHint does: nothing, a project-relative
// path, or an absolute path outside the project.
func (a *app) mainExeHintText(e *editState) string {
	value := strings.TrimSpace(e.mainExe)
	if value == "" {
		return a.t("edit.mainExeHint")
	}
	stored := relativeToProject(e.project.ProjectPath, value)
	if stored == "" || filepath.IsAbs(stored) {
		return a.t("edit.mainExeOutside")
	}
	return a.tf("edit.mainExeStored", "path", stored)
}

// syncIgnoreRules pushes the local ignore rules to the server project, keeping
// its title and force-update flag. Soft failures (no server target, missing
// project, CLI error) are returned as warnings so the save itself is not
// reported as failed, like ProjectConfigService.SyncToServerAsync.
func (a *app) syncIgnoreRules(ctx context.Context, cli *cliClient, lang, serverURL, projectName string, ignoreFolders, ignoreFiles []string) string {
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(projectName) == "" {
		// A project can be saved without a server target; the rules then only
		// live in shared.json, which the user must be told about.
		return tfLang(lang, "edit.syncNoServer")
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

// saveEdit persists the form: the .updator/shared.json values first (only the
// real differences, removals before additions, as
// ProjectConfigService.SaveAsync does), then the bookmark. A server sync
// failure is a warning; a CLI failure keeps the dialog open with the error.
func (a *app) saveEdit() {
	e := a.editing
	d := a.detail
	if e == nil || d == nil || e.busy {
		return
	}
	if e.loadError != "" {
		e.error = e.loadError
		a.toastMsg = e.loadError
		return
	}
	if msg := a.editValidation(e); msg != "" {
		e.error = msg
		a.toastMsg = msg
		return
	}
	e.busy = true
	e.error = ""
	path := d.project.ProjectPath

	// Snapshot the form before the goroutine starts: the view may still
	// rebuild while the CLI calls run.
	displayName := strings.TrimSpace(e.displayName)
	mainExe := strings.TrimSpace(e.mainExe)
	serverURL := strings.TrimSpace(e.serverURL)
	projectName := strings.TrimSpace(e.projectName)
	nextIgnoreFolders := cleanRuleList(e.ignoreFolders)
	nextIgnoreFiles := cleanRuleList(e.ignoreFiles)
	nextUncopyFolders := cleanRuleList(e.uncopyFolders)
	nextUncopyFiles := cleanRuleList(e.uncopyFiles)
	cli := a.cli
	lang := a.lang

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		syncWarning, updaterWarning, storedMainExe, err := a.applyEditForm(
			ctx, cli, lang, path, serverURL, projectName, mainExe,
			nextIgnoreFolders, nextIgnoreFiles, nextUncopyFolders, nextUncopyFiles,
		)
		a.post(func() {
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
			newProject.MainExePath = storedMainExe
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
			a.statusTone = toneSuccess
			switch {
			case syncWarning != "":
				a.status = syncWarning
				a.statusTone = toneWarning
				a.toastMsg = syncWarning
			case updaterWarning != "":
				a.status = updaterWarning
				a.statusTone = toneWarning
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

// applyEditForm writes only the changed shared.json values and returns the
// warnings plus the stored main executable path. The current file is re-read
// first so an old dialog snapshot can never overwrite changes made elsewhere;
// removals run before additions for every list, as ApplyListAsync does.
func (a *app) applyEditForm(
	ctx context.Context,
	cli *cliClient,
	lang, path, serverURL, projectName, mainExe string,
	ignoreFolders, ignoreFiles, uncopyFolders, uncopyFiles []string,
) (syncWarning, updaterWarning, storedMainExe string, err error) {
	if !cli.available() {
		return "", "", "", errCLIMissing
	}
	current, loadErr := loadShared(path)
	if loadErr != nil {
		return "", "", "", loadErr
	}
	changed := false
	if current.ServerURL != serverURL {
		if err = cli.configSet(ctx, path, "server.url", serverURL); err != nil {
			return "", "", "", err
		}
		changed = true
	}
	for _, group := range []struct {
		key     string
		current []string
		next    []string
	}{
		{"ignore.folders", current.IgnoreFolders, ignoreFolders},
		{"ignore.files", current.IgnoreFiles, ignoreFiles},
		{"un_copy.folders", current.UnCopyFolders, uncopyFolders},
		{"un_copy.files", current.UnCopyFiles, uncopyFiles},
	} {
		added, removed := diffList(group.current, group.next)
		for _, item := range removed {
			if err = cli.configSetArray(ctx, path, group.key, false, item); err != nil {
				return "", "", "", err
			}
			changed = true
		}
		for _, item := range added {
			if err = cli.configSetArray(ctx, path, group.key, true, item); err != nil {
				return "", "", "", err
			}
			changed = true
		}
	}
	if current.ProjectName != projectName {
		if err = cli.configSet(ctx, path, "project.name", projectName); err != nil {
			return "", "", "", err
		}
		changed = true
	}
	if changed {
		syncWarning = a.syncIgnoreRules(ctx, cli, lang, serverURL, projectName, ignoreFolders, ignoreFiles)
	}
	storedMainExe = relativeToProject(path, mainExe)
	updaterWarning = ensureUpdatersFor(path, storedMainExe)
	return syncWarning, updaterWarning, storedMainExe, nil
}

// cleanRuleList trims each rule and drops blanks and duplicates, as the C#
// dialog's Clean does before saving.
func cleanRuleList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
