package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// addProjectModal dispatches the wizard's current page. Every page is a form
// modal: a backdrop that owns the keyboard (Escape) but never the pointer, so
// the page's own buttons stay clickable.
func (a *app) addProjectModal(c *ui.Context) {
	s := a.adding
	if s == nil {
		return
	}
	switch s.stage {
	case addStageLocal:
		a.addProjectLocalModal(c, s)
	case addStageRules:
		a.addProjectRulesModal(c, s)
	case addStageCreate:
		a.addProjectCreateModal(c, s)
	default:
		a.addProjectServerModal(c, s)
	}
}

// addProjectPanel is the dialog frame shared by the wizard pages.
func (a *app) addProjectPanel(c *ui.Context, w, h float32, build func()) {
	t := c.Theme()
	ui.Column(c).Role(ui.RoleDialog).Size(w, h).Gap(10).Padding(18).
		Background(t.Surface).Radius(12).Border(1, t.Border).
		Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.08)).Children(build)
}

func (a *app) addProjectHeader(c *ui.Context, ic *ui.SVG, title string, busy bool) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(18).TextColor(t.Accent)
		ui.Text(c, title).FontSize(16).Bold()
		ui.Box(c).Grow(1)
		if a.ghostIconButton(c, a.t("edit.cancel"), iconX, busy) {
			a.closeAddProject()
		}
	})
}

// addProjectSteps numbers the wizard's two main steps, as the C# header does.
func (a *app) addProjectSteps(c *ui.Context, stage addStage) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		a.stepBadge(c, 1, stage == addStageServer)
		ui.Text(c, a.t("addProject.stepServer")).FontSize(13)
		ui.Box(c).Width(28).Height(1).Background(t.Border).Margin(12, 0)
		a.stepBadge(c, 2, stage != addStageServer)
		ui.Text(c, a.t("addProject.stepLocal")).FontSize(13)
	})
}

// addProjectFooter puts the page's validation caption on the left and its
// buttons on the right.
func (a *app) addProjectFooter(c *ui.Context, caption string, captionColor ui.Color, buttons func()) {
	ui.Row(c).Gap(8).Shrink(0).Children(func() {
		if caption != "" {
			ui.Icon(c, iconWarning).FontSize(12).TextColor(captionColor)
			ui.Text(c, caption).FontSize(11).TextColor(captionColor).Wrap().Grow(1)
		} else {
			ui.Spacer(c)
		}
		buttons()
	})
}

// addProjectServerModal is the C# server step: enter a server URL, fetch the
// project list, pick or create a project, then go to the local step.
func (a *app) addProjectServerModal(c *ui.Context, s *addProjectState) {
	t := c.Theme()
	a.formModalEsc(c, &a.addOpen, func() {
		a.closeAddProject()
	}, func() {
		w, h := dialogSize(c, 820, 640)
		a.addProjectPanel(c, w, h, func() {
			busy := s.busy || s.creating
			a.addProjectHeader(c, iconCloud, a.t("addProject.title"), busy)
			a.addProjectSteps(c, addStageServer)
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				a.fieldLabel(c, a.t("addProject.server"))
				urlChanged, urlSubmitted := false, false
				ui.Row(c).Gap(6).Children(func() {
					in := ui.TextInput(c, &s.serverURL).Placeholder(a.t("addProject.serverHint")).
						Label(a.t("addProject.server")).Grow(1).AutoFocus()
					urlChanged = in.Changed()
					urlSubmitted = in.Submitted()
					if a.actionButton(c, a.t("addProject.fetch"), iconRefresh, true,
						s.fetching || busy || strings.TrimSpace(s.serverURL) == "") {
						urlSubmitted = true
					}
				})
				if len(s.knownURLs) > 0 {
					a.suggestionChips(c, s.knownURLs, func(value string) {
						s.serverURL = value
						s.clearServerProjects()
					})
				}
				if urlChanged {
					s.clearServerProjects()
				}
				if urlSubmitted {
					a.fetchServerProjects()
				}
				if s.fetching {
					a.busyRow(c, a.t("addProject.fetching"))
				}
				if s.fetchError != "" {
					a.noticeBox(c, iconWarning, s.fetchError, t.Warning, true)
				}
				if len(s.allProjects) > 0 {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if ui.SearchField(c, &s.filter).Placeholder(a.t("addProject.searchHint")).
							Label(a.t("addProject.searchHint")).Changed() {
							s.applyServerFilter()
						}
						ui.Text(c, a.t("addProject.selectHint")).FontSize(11).
							TextColor(t.TextMuted).SingleLine()
					})
				}
				filtered := s.filteredProjects()
				if len(filtered) > 0 {
					s.clampSelection()
					s.list.Selected = &s.selected
					s.list.Key = func(i int) any { return filtered[i].Name }
					list := ui.List(c, &s.list, len(filtered), func(i int) {
						a.addProjectRow(c, filtered[i], i == s.selected)
					}).Height(210)
					if list.Changed() {
						if p, ok := s.selectedProject(); ok {
							s.chooseProject(p)
						}
					}
				} else if len(s.allProjects) > 0 {
					ui.Text(c, a.t("addProject.noMatch")).FontSize(11).TextColor(t.TextMuted)
				} else if s.fetchError == "" && !s.fetching {
					a.addProjectEmptyBox(c, iconServer, a.t("addProject.emptyTitle"), a.t("addProject.emptyHint"))
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					if a.actionButton(c, a.t("addProject.create"), iconCloud, false,
						s.fetching || busy || strings.TrimSpace(s.serverURL) == "") {
						a.openAddProjectCreate()
					}
					ui.Text(c, a.t("addProject.createHint")).FontSize(11).
						TextColor(t.TextMuted).Wrap().Grow(1)
				})
			})
			validation := a.serverValidation(s)
			s.validation = validation
			a.addProjectFooter(c, validation, t.Warning, func() {
				if a.actionButton(c, a.t("edit.cancel"), iconX, false, busy) {
					a.closeAddProject()
				}
				if a.actionButton(c, a.t("addProject.next"), iconArrowRight, true, !s.canNext()) {
					a.addProjectNextStep()
				}
			})
		})
	})
}

// addProjectRow is one fetched server project: name and title, its ID, the
// version pill and the force-update badge, as the C# ListBox item template.
func (a *app) addProjectRow(c *ui.Context, p ProjectInfo, selected bool) {
	t := c.Theme()
	nameColor, mutedColor := t.Text, t.TextMuted
	if selected {
		nameColor, mutedColor = t.AccentText, t.AccentText
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(9, 6).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Name).FontSize(12).Bold().TextColor(nameColor).SingleLine()
				if p.Title != "" && p.Title != p.Name {
					ui.Text(c, p.Title).FontSize(10).TextColor(mutedColor).SingleLine()
				}
			})
			ui.Text(c, fmt.Sprintf("ID %d", p.ID)).FontSize(9).TextColor(mutedColor).SingleLine()
		})
		if p.ForceUpdate {
			ui.Row(c).Padding(6, 2).Radius(3).Background(t.Warning).Children(func() {
				ui.Text(c, a.t("addProject.forceUpdate")).FontSize(9).TextColor(ui.Hex("#ffffff"))
			})
		}
		version := strings.TrimSpace(p.Version)
		if version == "" {
			version = a.t("addProject.versionNone")
		}
		a.versionPill(c, version)
	})
}

// addProjectEmptyBox is the C# server step's empty state.
func (a *app) addProjectEmptyBox(c *ui.Context, ic *ui.SVG, title, hint string) {
	t := c.Theme()
	ui.Column(c).Gap(6).Padding(20, 18).Background(t.Background).
		Border(1, t.Border).Radius(6).Center().Children(func() {
		ui.Icon(c, ic).FontSize(26).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(12).Bold().TextColor(t.TextMuted)
		ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).MaxWidth(360).Wrap()
	})
}

// addProjectSelectedCard shows the server project the local step will add.
func (a *app) addProjectSelectedCard(c *ui.Context, p ProjectInfo) {
	t := c.Theme()
	version := strings.TrimSpace(p.Version)
	if version == "" {
		version = a.t("addProject.versionNone")
	}
	ui.Row(c).Gap(8).Padding(12, 10).Shrink(0).Background(t.Background).
		Border(1, t.Border).Radius(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconServer).FontSize(16).TextColor(t.Accent)
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, p.Name).FontSize(13).Bold().SingleLine()
			if p.Title != "" {
				ui.Text(c, p.Title).FontSize(10).TextColor(t.TextMuted).SingleLine()
			}
		})
		a.versionPill(c, version)
	})
}

// addProjectLocalModal is the C# local step: the project folder, the display
// name and the optional executable/UpdateFolder inputs.
func (a *app) addProjectLocalModal(c *ui.Context, s *addProjectState) {
	t := c.Theme()
	a.formModalEsc(c, &a.addOpen, func() {
		if s.busy {
			return
		}
		a.addProjectStage(addStageServer)
	}, func() {
		w, h := dialogSize(c, 760, 640)
		a.addProjectPanel(c, w, h, func() {
			a.addProjectHeader(c, iconCloud, a.t("addProject.title"), s.busy)
			a.addProjectSteps(c, addStageLocal)
			project, _ := s.selectedProject()
			a.addProjectSelectedCard(c, project)
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				pathState := projectPathState(s.path, a.projects)
				ui.Form(c, func() {
					ui.Field(c, a.t("addProject.path"), func() {
						changed := false
						row := ui.Row(c).Gap(6)
						row.Children(func() {
							if ui.TextInput(c, &s.path).Placeholder(a.t("addProject.pathHint")).
								Grow(1).Changed() {
								changed = true
							}
							if a.actionButton(c, a.t("edit.browse"), iconFolder, false, false) {
								a.pickFolder(false, a.t("addProject.path"), func(paths []string) {
									if len(paths) == 0 {
										return
									}
									s.path = normalizePath(paths[0])
									a.refreshAddProjectLocal(s)
								})
							}
						})
						if dropped := row.DroppedFiles(); len(dropped) > 0 {
							s.path = normalizePath(dropped[0])
							changed = true
						}
						if changed {
							a.refreshAddProjectLocal(s)
							pathState = projectPathState(s.path, a.projects)
						}
					}).Description(pathStateText(a.lang, pathState))
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconFolderOpen).FontSize(12).TextColor(t.TextMuted)
						ui.Text(c, a.t("addProject.dropHint")).FontSize(10).TextColor(t.TextMuted).SingleLine()
					})
					ui.Field(c, a.t("addProject.displayName"), func() {
						ui.TextInput(c, &s.displayName).Placeholder(a.t("addProject.displayNameHint"))
					})
					ui.Field(c, a.t("edit.mainExe"), func() {
						if len(s.mainExeCandidates) > 0 {
							if ui.Select(c, &s.mainExe, s.mainExeCandidates).Changed() {
								s.syncCloseProcess()
							}
						}
						mainChanged := false
						ui.Row(c).Gap(6).Children(func() {
							if ui.TextInput(c, &s.mainExe).Placeholder(a.t("addProject.mainExeHint")).
								Grow(1).Changed() {
								mainChanged = true
							}
							if a.actionButton(c, a.t("edit.browse"), iconFolder, false, false) {
								a.pickExecutable(a.t("edit.mainExe"), func(path string) {
									s.mainExe = path
									s.syncCloseProcess()
								})
							}
						})
						if mainChanged {
							s.syncCloseProcess()
						}
					})
					ui.Field(c, a.t("addProject.closeProcess"), func() {
						ui.TextInput(c, &s.closeProcessName).
							Placeholder(a.t("addProject.closeProcessHint"))
					})
				})
				if a.textButton(c, a.t("addProject.advanced"), iconSettings, s.busy) {
					a.addProjectStage(addStageRules)
				}
			})
			validation := a.localValidation(s)
			s.validation = validation
			if s.busy {
				a.busyRow(c, a.t("addProject.initializing"))
			}
			a.addProjectFooter(c, validation, t.Warning, func() {
				if a.actionButton(c, a.t("edit.cancel"), iconX, false, s.busy) {
					a.closeAddProject()
				}
				if a.actionButton(c, a.t("addProject.back"), iconArrowLeft, false, s.busy) {
					a.addProjectStage(addStageServer)
				}
				if a.actionButton(c, a.t("addProject.initialize"), iconCheck, true,
					s.busy || validation != "") {
					a.initializeAddProject()
				}
			})
		})
	})
}

// addProjectRulesModal is the C# Expander of the local step, promoted to a
// sub-page that goes back to the local step instead of closing the wizard.
func (a *app) addProjectRulesModal(c *ui.Context, s *addProjectState) {
	t := c.Theme()
	a.formModalEsc(c, &a.addOpen, func() {
		if s.busy {
			return
		}
		a.addProjectStage(addStageLocal)
	}, func() {
		w, h := dialogSize(c, 820, 420)
		a.addProjectPanel(c, w, h, func() {
			a.addProjectHeader(c, iconSettings, a.t("addProject.ignoreTitle"), s.busy)
			ui.Text(c, a.t("addProject.ignoreHint")).FontSize(11).TextColor(t.TextMuted).Wrap()
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				ui.Row(c).Gap(14).Children(func() {
					ui.Column(c).Grow(1).Gap(10).Children(func() {
						a.suggestionChips(c, addProjectIgnoreFolderSuggestions, func(value string) {
							s.ignoreFolders = appendUnique(s.ignoreFolders, value)
						})
						a.editGroup(c, a.t("edit.ignoreFolders"), &s.ignoreFolders, &s.newIgnoreFolder, func() {
							a.addProjectRulesPickFolders(&s.ignoreFolders, a.t("edit.ignoreFolders"))
						})
						a.suggestionChips(c, addProjectIgnoreFileSuggestions, func(value string) {
							s.ignoreFiles = appendUnique(s.ignoreFiles, value)
						})
						a.editGroup(c, a.t("edit.ignoreFiles"), &s.ignoreFiles, &s.newIgnoreFile, func() {
							a.addProjectRulesPickFiles(&s.ignoreFiles, a.t("edit.ignoreFiles"))
						})
					})
					ui.Column(c).Grow(1).Gap(10).Children(func() {
						a.editGroup(c, a.t("edit.uncopyFolders"), &s.uncopyFolders, &s.newUncopyFolder, func() {
							a.addProjectRulesPickFolders(&s.uncopyFolders, a.t("edit.uncopyFolders"))
						})
						a.editGroup(c, a.t("edit.uncopyFiles"), &s.uncopyFiles, &s.newUncopyFile, func() {
							a.addProjectRulesPickFiles(&s.uncopyFiles, a.t("edit.uncopyFiles"))
						})
					})
				})
			})
			a.addProjectFooter(c, "", t.Warning, func() {
				if a.actionButton(c, a.t("addProject.back"), iconArrowLeft, false, s.busy) {
					a.addProjectStage(addStageLocal)
				}
				if a.actionButton(c, a.t("addProject.done"), iconCheck, true, s.busy) {
					a.addProjectStage(addStageLocal)
				}
			})
		})
	})
}

func (a *app) addProjectRulesPickFolders(list *[]string, title string) {
	s := a.adding
	if s == nil {
		return
	}
	if strings.TrimSpace(s.path) == "" {
		a.toastMsg = a.t("addProject.pathSelect")
		return
	}
	a.pickFolder(true, title, func(paths []string) {
		if a.adding != s {
			return
		}
		for _, p := range paths {
			a.addProjectPath(list, s.path, p)
		}
	})
}

func (a *app) addProjectRulesPickFiles(list *[]string, title string) {
	s := a.adding
	if s == nil {
		return
	}
	if strings.TrimSpace(s.path) == "" {
		a.toastMsg = a.t("addProject.pathSelect")
		return
	}
	a.pickFiles(title, func(paths []string) {
		if a.adding != s {
			return
		}
		for _, p := range paths {
			a.addProjectPath(list, s.path, p)
		}
	})
}

// addProjectCreateModal is the C# CreateProjectDialog: create a server project
// without leaving the wizard. The identifier follows the title until the user
// edits it, as in the C# view model.
func (a *app) addProjectCreateModal(c *ui.Context, s *addProjectState) {
	t := c.Theme()
	a.formModalEsc(c, &a.addOpen, func() {
		if s.creating {
			return
		}
		a.addProjectStage(addStageServer)
	}, func() {
		w, h := dialogSize(c, 560, 620)
		a.addProjectPanel(c, w, h, func() {
			a.addProjectHeader(c, iconCloud, a.t("addProject.createHeading"), s.creating)
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				a.fieldLabel(c, a.t("addProject.server"))
				ui.Text(c, strings.TrimSpace(s.serverURL)).FontSize(12).
					TextColor(t.TextMuted).SingleLine()
				a.fieldLabel(c, a.t("addProject.titleField"))
				titleBefore := s.createTitle
				ui.TextInput(c, &s.createTitle).Placeholder(a.t("addProject.titleHint")).AutoFocus()
				titleTyped := s.createTitle != titleBefore
				if titleTyped && !s.createNameEdited {
					s.createName = strings.TrimSpace(s.createTitle)
				}
				a.fieldLabel(c, a.t("addProject.nameLabel"))
				nameBefore := s.createName
				ui.TextInput(c, &s.createName).Placeholder(a.t("addProject.projectHint"))
				if !titleTyped && s.createName != nameBefore {
					s.createNameEdited = true
				}
				ui.Text(c, a.t("addProject.createNameHint")).FontSize(11).
					TextColor(t.TextMuted).Wrap()
				a.fieldLabel(c, a.t("addProject.updatePolicy"))
				ui.Checkbox(c, &s.createForceUpdate, a.t("addProject.forceUpdate"))
				ui.Text(c, a.t("addProject.forceUpdateHint")).FontSize(11).
					TextColor(t.Warning).Wrap()
				a.fieldLabel(c, a.t("addProject.suggestions"))
				a.suggestionChips(c, addProjectIgnoreFolderSuggestions, func(value string) {
					s.createIgnoreFolders = appendUnique(s.createIgnoreFolders, value)
				})
				a.editGroup(c, a.t("edit.ignoreFolders"), &s.createIgnoreFolders,
					&s.createNewIgnoreFolder, func() {
						a.addProjectCreatePickFolders(&s.createIgnoreFolders, a.t("edit.ignoreFolders"))
					})
				a.suggestionChips(c, addProjectIgnoreFileSuggestions, func(value string) {
					s.createIgnoreFiles = appendUnique(s.createIgnoreFiles, value)
				})
				a.editGroup(c, a.t("edit.ignoreFiles"), &s.createIgnoreFiles,
					&s.createNewIgnoreFile, func() {
						a.addProjectCreatePickFiles(&s.createIgnoreFiles, a.t("edit.ignoreFiles"))
					})
			})
			validation := a.createValidation(s)
			s.validation = validation
			if s.creating {
				a.busyRow(c, a.t("addProject.creating"))
			}
			caption, captionColor := validation, t.Warning
			if s.actionError != "" {
				caption, captionColor = s.actionError, t.Danger
			}
			a.addProjectFooter(c, caption, captionColor, func() {
				if a.actionButton(c, a.t("edit.cancel"), iconX, false, s.creating) {
					a.addProjectStage(addStageServer)
				}
				if a.actionButton(c, a.t("addProject.create"), iconCloud, true, s.creating) {
					a.createServerProject()
				}
			})
		})
	})
}

// addProjectCreatePickFolders picks folders for the create page's ignore
// rules; before a local folder is known it falls back to the folder name.
func (a *app) addProjectCreatePickFolders(list *[]string, title string) {
	s := a.adding
	if s == nil {
		return
	}
	a.pickFolder(true, title, func(paths []string) {
		if a.adding != s {
			return
		}
		for _, p := range paths {
			if strings.TrimSpace(s.path) != "" {
				a.addProjectPath(list, s.path, p)
				continue
			}
			if base := filepath.Base(normalizePath(p)); base != "" && base != "." {
				*list = appendUnique(*list, base)
			}
		}
	})
}

func (a *app) addProjectCreatePickFiles(list *[]string, title string) {
	s := a.adding
	if s == nil {
		return
	}
	a.pickFiles(title, func(paths []string) {
		if a.adding != s {
			return
		}
		for _, p := range paths {
			if strings.TrimSpace(s.path) != "" {
				a.addProjectPath(list, s.path, p)
				continue
			}
			if base := filepath.Base(normalizePath(p)); base != "" && base != "." {
				*list = appendUnique(*list, base)
			}
		}
	})
}

// fieldLabelBox is the fixed-width label cell shared with the add-local
// preview dialog.
func (a *app) fieldLabelBox(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(11).TextColor(c.Theme().TextMuted).Width(104).SingleLine()
}
