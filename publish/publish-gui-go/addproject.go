package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// addStage is the page the add-project wizard shows. The wizard mirrors the
// C# AddProjectDialog: a server step, a local step, and two sub-pages (ignore
// rules and create-project) that temporarily replace the server/local body.
type addStage int

const (
	addStageServer addStage = iota
	addStageLocal
	addStageRules
	addStageCreate
)

// addProjectState is the Go counterpart of the C# AddProjectDialogViewModel
// plus the C# CreateProjectDialogViewModel: pick or create a server project,
// then point the wizard at the local build folder and initialize it.
type addProjectState struct {
	stage addStage

	// Server step.
	serverURL   string
	knownURLs   []string
	allProjects []ProjectInfo
	filter      string
	selected    int
	list        ui.ListState
	fetching    bool
	fetchError  string

	// The project chosen on the server step.
	projectName string
	title       string
	forceUpdate bool

	// Local step.
	path              string
	displayName       string
	mainExe           string
	mainExeCandidates []string
	closeProcessName  string
	lastMainExe       string

	ignoreFolders   []string
	ignoreFiles     []string
	uncopyFolders   []string
	uncopyFiles     []string
	newIgnoreFolder string
	newIgnoreFile   string
	newUncopyFolder string
	newUncopyFile   string

	// Create sub-page.
	createTitle           string
	createName            string
	createForceUpdate     bool
	createIgnoreFolders   []string
	createIgnoreFiles     []string
	createNewIgnoreFolder string
	createNewIgnoreFile   string
	createNameEdited      bool
	creating              bool

	busy        bool
	validation  string
	actionError string
}

// addProjectIgnoreFolderSuggestions and addProjectIgnoreFileSuggestions are
// the C# AppConstants lists; the wizard offers them as one-click rule chips.
var (
	addProjectIgnoreFolderSuggestions = []string{"node_modules", ".git", ".vs", "bin", "obj", "dist", "logs"}
	addProjectIgnoreFileSuggestions   = []string{"*.log", "*.tmp", "*.pdb", "Thumbs.db", ".DS_Store"}
)

// addProjectFetchStartedHook lets a test observe the state right after a fetch
// starts, before its goroutine can clear the flag; nil in production.
var addProjectFetchStartedHook func(*addProjectState)

func (a *app) openAddProject() {
	known := knownServerURLs(a.projects)
	s := &addProjectState{selected: -1, knownURLs: known}
	if len(known) > 0 {
		s.serverURL = known[0]
	}
	a.adding = s
	a.addOpen = true
}

func (a *app) closeAddProject() {
	a.addOpen = false
	a.adding = nil
}

// addProjectStage moves the wizard to another page and clears the errors that
// belonged to the page it leaves.
func (a *app) addProjectStage(stage addStage) {
	s := a.adding
	if s == nil {
		return
	}
	s.stage = stage
	s.actionError = ""
	s.validation = ""
}

// filteredProjects applies the filter box to the fetched server projects.
func (s *addProjectState) filteredProjects() []ProjectInfo {
	q := strings.ToLower(strings.TrimSpace(s.filter))
	if q == "" {
		return s.allProjects
	}
	out := make([]ProjectInfo, 0, len(s.allProjects))
	for _, p := range s.allProjects {
		if strings.Contains(strings.ToLower(p.Name+" "+p.Title), q) {
			out = append(out, p)
		}
	}
	return out
}

func (s *addProjectState) selectedProject() (ProjectInfo, bool) {
	list := s.filteredProjects()
	if s.selected < 0 || s.selected >= len(list) {
		return ProjectInfo{}, false
	}
	return list[s.selected], true
}

func (s *addProjectState) clampSelection() {
	n := len(s.filteredProjects())
	if n == 0 {
		s.selected = -1
		return
	}
	if s.selected >= n {
		s.selected = n - 1
	}
}

// chooseProject copies the row the user picked into the fields the local step
// and the initialization use, as the C# SelectedServerProject binding does.
func (s *addProjectState) chooseProject(p ProjectInfo) {
	s.projectName = p.Name
	s.title = p.Title
	s.forceUpdate = p.ForceUpdate
	s.displayName = p.Name
}

// clearServerProjects drops the fetched list when the server URL changes.
func (s *addProjectState) clearServerProjects() {
	s.allProjects = nil
	s.selected = -1
	s.fetchError = ""
	s.actionError = ""
}

// applyServerFilter re-selects the row of the project already chosen (after a
// create) or the only row, as the C# ApplyServerFilter does.
func (s *addProjectState) applyServerFilter() {
	list := s.filteredProjects()
	s.selected = -1
	name := strings.TrimSpace(s.projectName)
	for i, p := range list {
		if strings.EqualFold(p.Name, name) {
			s.selected = i
			break
		}
	}
	if s.selected < 0 && len(list) == 1 {
		s.selected = 0
	}
	if p, ok := s.selectedProject(); ok {
		s.chooseProject(p)
	}
}

func (s *addProjectState) hasServerProjectName(name string) bool {
	for _, p := range s.allProjects {
		if strings.EqualFold(strings.TrimSpace(p.Name), name) {
			return true
		}
	}
	return false
}

func (s *addProjectState) canNext() bool {
	_, ok := s.selectedProject()
	return ok && !s.busy && !s.fetching && !s.creating && strings.TrimSpace(s.serverURL) != ""
}

// syncCloseProcess fills the close process name from the main executable, as
// the C# OnMainExePathChanged does, whenever the executable changed.
func (s *addProjectState) syncCloseProcess() {
	mainExe := strings.TrimSpace(s.mainExe)
	if mainExe == s.lastMainExe {
		return
	}
	s.lastMainExe = mainExe
	if mainExe != "" {
		s.closeProcessName = strings.TrimSuffix(filepath.Base(mainExe), filepath.Ext(mainExe))
	}
}

// refreshAddProjectLocal re-reads the folder-dependent suggestions when the
// local folder changes: main-executable candidates, their default selection,
// the close process name and the display name fallback.
func (a *app) refreshAddProjectLocal(s *addProjectState) {
	path := strings.TrimSpace(s.path)
	s.mainExeCandidates = nil
	if path != "" {
		s.mainExeCandidates = mainExeCandidates(normalizePath(path))
	}
	if strings.TrimSpace(s.mainExe) == "" && len(s.mainExeCandidates) == 1 {
		s.mainExe = s.mainExeCandidates[0]
	}
	s.syncCloseProcess()
	if strings.TrimSpace(s.displayName) == "" {
		s.displayName = strings.TrimSpace(s.projectName)
	}
	if strings.TrimSpace(s.displayName) == "" && path != "" {
		s.displayName = filepath.Base(normalizePath(path))
	}
}

func (a *app) addProjectNextStep() {
	s := a.adding
	if s == nil || !s.canNext() {
		return
	}
	s.stage = addStageLocal
	s.actionError = ""
	a.refreshAddProjectLocal(s)
}

// fetchServerProjects fetches the server project list. The fetching flag is
// set synchronously, before the goroutine starts, so the page can show its
// spinner and disable the fetch button right away.
func (a *app) fetchServerProjects() {
	s := a.adding
	if s == nil || s.fetching || s.busy || s.creating {
		return
	}
	serverURL := strings.TrimSpace(s.serverURL)
	if serverURL == "" {
		s.fetchError = a.t("addProject.serverRequired")
		return
	}
	s.fetching = true
	s.fetchError = ""
	if addProjectFetchStartedHook != nil {
		addProjectFetchStartedHook(s)
	}
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		projects, err := cli.projectList(ctx, serverURL)
		a.post(func() {
			if a.adding != s {
				return
			}
			s.fetching = false
			if err != nil {
				s.allProjects = nil
				s.selected = -1
				s.fetchError = a.tf("addProject.fetchFailed", "error", err.Error())
				return
			}
			s.allProjects = projects
			s.applyServerFilter()
			a.refreshAddProjectLocal(s)
		})
	}()
}

// openAddProjectCreate opens the create sub-page seeded with the local step's
// ignore rules, as the C# dialog passes its rule lists to the create dialog.
func (a *app) openAddProjectCreate() {
	s := a.adding
	if s == nil || s.fetching || s.busy {
		return
	}
	s.stage = addStageCreate
	s.actionError = ""
	s.validation = ""
	// The create page starts from the local step's rule lists, as the C#
	// CreateServerProjectRequest is built from the add dialog's lists.
	s.createIgnoreFolders = append([]string(nil), s.ignoreFolders...)
	s.createIgnoreFiles = append([]string(nil), s.ignoreFiles...)
}

func (a *app) createValidation(s *addProjectState) string {
	if strings.TrimSpace(s.serverURL) == "" {
		return a.t("addProject.serverRequired")
	}
	name := strings.TrimSpace(s.createName)
	if name == "" {
		return a.t("addProject.createEmptyName")
	}
	if s.hasServerProjectName(name) {
		return a.t("addProject.createDuplicate")
	}
	return ""
}

// createServerProject creates the server project; the title defaults to the
// name when empty, matching the C# dialog. Validation failures show on the
// create page and start no work.
func (a *app) createServerProject() {
	s := a.adding
	if s == nil || s.busy || s.creating || s.fetching {
		return
	}
	serverURL := strings.TrimSpace(s.serverURL)
	name := strings.TrimSpace(s.createName)
	title := strings.TrimSpace(s.createTitle)
	if title == "" {
		title = name
	}
	if msg := a.createValidation(s); msg != "" {
		s.actionError = msg
		s.validation = msg
		return
	}
	forceUpdate := s.createForceUpdate
	ignoreFolders := append([]string(nil), s.createIgnoreFolders...)
	ignoreFiles := append([]string(nil), s.createIgnoreFiles...)
	s.creating = true
	s.actionError = ""
	s.validation = ""
	cli := a.cli
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		info, err := cli.projectCreate(ctx, serverURL, name, title, forceUpdate, ignoreFolders, ignoreFiles)
		a.post(func() {
			if a.adding != s {
				return
			}
			s.creating = false
			if err != nil {
				s.actionError = a.tf("addProject.createFailed", "error", err.Error())
				return
			}
			createdName := strings.TrimSpace(info.Name)
			if createdName == "" {
				createdName = name
			}
			createdTitle := info.Title
			if createdTitle == "" {
				createdTitle = title
			}
			s.createName = createdName
			s.projectName = createdName
			s.title = createdTitle
			s.forceUpdate = info.ForceUpdate
			s.displayName = createdName
			s.stage = addStageServer
			s.fetchError = ""
			s.actionError = ""
			s.validation = ""
			// Pick the new project up on the server step: the refreshed list
			// selects it by name and fills the local step's defaults.
			a.fetchServerProjects()
		})
	}()
}

// addProjectMainExeValidation checks the optional main executable: it must
// exist and live inside the project folder, as the C# ProjectSetupService
// requires before it creates UpdateFolder.
func addProjectMainExeValidation(lang, path, mainExe string) string {
	mainExe = strings.TrimSpace(mainExe)
	if mainExe == "" {
		return ""
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return translate(lang, "addProject.pathSelect")
	}
	full := resolveProjectPath(path, mainExe)
	if full == "" || !fileExists(full) {
		return translate(lang, "addProject.mainMissing")
	}
	rel, err := filepath.Rel(normalizePath(path), normalizePath(full))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return translate(lang, "addProject.mainOutside")
	}
	return ""
}

// localValidation is the C# BuildValidationMessage for the local step.
func (a *app) localValidation(s *addProjectState) string {
	if strings.TrimSpace(s.serverURL) == "" {
		return a.t("addProject.serverRequired")
	}
	if _, ok := s.selectedProject(); !ok {
		return a.t("addProject.selectProject")
	}
	state := projectPathState(s.path, a.projects)
	if state != pathUsable {
		return pathStateText(a.lang, state)
	}
	if strings.TrimSpace(s.displayName) == "" {
		return a.t("addProject.displayNameRequired")
	}
	return addProjectMainExeValidation(a.lang, s.path, s.mainExe)
}

// serverValidation is the C# BuildValidationMessage for the server step.
func (a *app) serverValidation(s *addProjectState) string {
	if _, ok := s.selectedProject(); !ok {
		return a.t("addProject.selectProject")
	}
	if strings.TrimSpace(s.serverURL) == "" {
		return a.t("addProject.serverRequired")
	}
	return ""
}

// initializeAddProject runs the C# ProjectSetupService flow: CLI config init,
// an optional UpdateFolder, the bookmark, then the updater refresh.
func (a *app) initializeAddProject() {
	s := a.adding
	if s == nil || s.busy || s.fetching || s.creating {
		return
	}
	project, ok := s.selectedProject()
	if !ok {
		s.validation = a.t("addProject.selectProject")
		return
	}
	if msg := a.localValidation(s); msg != "" {
		s.validation = msg
		return
	}
	serverURL := strings.TrimSpace(s.serverURL)
	path := normalizePath(strings.TrimSpace(s.path))
	displayName := strings.TrimSpace(s.displayName)
	mainExe := strings.TrimSpace(s.mainExe)
	closeProcessName := strings.TrimSpace(s.closeProcessName)
	ignoreFolders := append([]string(nil), s.ignoreFolders...)
	ignoreFiles := append([]string(nil), s.ignoreFiles...)
	uncopyFolders := append([]string(nil), s.uncopyFolders...)
	uncopyFiles := append([]string(nil), s.uncopyFiles...)
	s.busy = true
	s.validation = ""
	s.actionError = ""
	cli := a.cli
	baseProjects := append([]ProjectConfig(nil), a.projects...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		warnings := ""
		var err error
		if !cli.available() {
			err = errCLIMissing
		} else if err = cli.initProject(ctx, path, serverURL, project.Name,
			ignoreFolders, ignoreFiles, uncopyFolders, uncopyFiles); err == nil && mainExe != "" {
			warning, createErr := createUpdateFolder(path, resolveProjectPath(path, mainExe), closeProcessName)
			if createErr != nil {
				err = createErr
			} else {
				warnings = warning
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
		if err == nil {
			// Refreshing an existing sibling UpdateFolder is best effort, as
			// the C# RefreshUpdaters call: a warning only goes to the toast.
			if warning, _ := refreshUpdaters(path); warning != "" {
				warnings = warning
			}
		}
		a.post(func() {
			if a.adding != s {
				return
			}
			s.busy = false
			if err != nil {
				s.actionError = a.tf("addProject.initFailed", "error", err.Error())
				return
			}
			a.projects = next
			a.addOpen = false
			a.adding = nil
			a.filter = ""
			a.selected = len(a.projects) - 1
			a.status = a.tf("addProject.added", "name", displayName)
			a.toastMsg = a.status
			if warnings != "" {
				a.toastMsg = warnings
			}
			a.openSelected()
		})
	}()
}
