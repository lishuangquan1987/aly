package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// addProjectModal dispatches the wizard's current page. Every page is a form
// modal with the shared dialog chrome.
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

// addProjectStepper numbers the wizard's two main steps, as the C# header does.
func (a *app) addProjectStepper(c *ui.Context, stage addStage) {
	p := a.pal(c)
	active := stage != addStageServer
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		a.stepBadge(c, 1, !active)
		weight(ui.Text(c, a.t("addProject.stepServer")).FontSize(fontLabel), !active, 600).
			TextColor(condColor(p, !active, p.text, p.textThird))
		ui.Box(c).Width(28).Height(1).Background(p.border).MarginX(6)
		a.stepBadge(c, 2, active)
		weight(ui.Text(c, a.t("addProject.stepLocal")).FontSize(fontLabel), active, 600).
			TextColor(condColor(p, active, p.text, p.textThird))
	})
}

// weight is the font weight for a boolean state on one line.
func weight(e ui.Element, on bool, w int) ui.Element {
	if on {
		return e.FontWeight(w)
	}
	return e.FontWeight(400)
}

func condColor(p palette, on bool, yes, no ui.Color) ui.Color {
	if on {
		return yes
	}
	return no
}

// addProjectServerModal is the server step: enter a server URL, fetch the
// project list, pick or create a project, then go to the local step.
func (a *app) addProjectServerModal(c *ui.Context, s *addProjectState) {
	p := a.pal(c)
	a.formModalEsc(c, &a.addOpen, func() {
		a.closeAddProject()
	}, func() {
		w, h := dialogSize(c, 840, 560)
		busy := s.busy || s.creating
		a.dialogShell(c, iconServer, a.t("addProject.title"), a.t("addProject.serverSubtitle"),
			w, h, busy, func() { a.closeAddProject() }, func() {
				a.addProjectStepper(c, addStageServer)
				urlChanged, urlSubmitted := false, false
				a.field(c, a.t("addProject.server"), func() {
					ui.Row(c).Gap(6).Children(func() {
						in := ui.TextInput(c, &s.serverURL).Placeholder(a.t("addProject.serverHint")).
							Label(a.t("addProject.server")).Grow(1).Height(controlHeight).
							Radius(8).AutoFocus()
						urlChanged = in.Changed()
						urlSubmitted = in.Submitted()
						if a.buttonSized(c, a.t("addProject.fetch"), a.t("addProject.fetch"), iconRefresh,
							btnPrimary, s.fetching || busy || strings.TrimSpace(s.serverURL) == "", controlHeight) {
							urlSubmitted = true
						}
					})
				})
				if len(s.knownURLs) > 0 {
					a.suggestionTags(c, s.knownURLs, busy, func(value string) {
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
					a.notice(c, iconWarning, s.fetchError, toneWarning)
				}
				if len(s.allProjects) > 0 {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if ui.SearchField(c, &s.filter).Placeholder(a.t("addProject.searchHint")).
							Label(a.t("addProject.searchHint")).Height(controlHeightS).
							Radius(8).Changed() {
							s.applyServerFilter()
						}
						ui.Text(c, a.t("addProject.selectHint")).FontSize(fontCaption).
							TextColor(p.textThird).SingleLine()
					})
				}
				filtered := s.filteredProjects()
				if len(filtered) > 0 {
					s.clampSelection()
					s.list.Selected = &s.selected
					s.list.Key = func(i int) any { return filtered[i].Name }
					list := ui.List(c, &s.list, len(filtered), func(i int) {
						a.addProjectRow(c, filtered[i], i == s.selected)
					}).Height(180)
					if list.Changed() {
						if project, ok := s.selectedProject(); ok {
							s.chooseProject(project)
						}
					}
				} else if len(s.allProjects) > 0 {
					ui.Text(c, a.t("addProject.noMatch")).FontSize(fontCaption).TextColor(p.textThird)
				} else if s.fetchError == "" && !s.fetching {
					a.addProjectEmptyBox(c, iconServer, a.t("addProject.emptyTitle"), a.t("addProject.emptyHint"))
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					if a.buttonSized(c, a.t("addProject.create"), a.t("addProject.create"), iconPlus,
						btnSecondary, s.fetching || busy || strings.TrimSpace(s.serverURL) == "", controlHeightS) {
						a.openAddProjectCreate()
					}
					ui.Text(c, a.t("addProject.createHint")).FontSize(fontCaption).
						TextColor(p.textThird).Wrap().Grow(1)
				})
			}, func() {
				validation := a.serverValidation(s)
				s.validation = validation
				if validation != "" {
					a.inlineNotice(c, iconInfo, validation, toneWarning)
				} else {
					ui.Box(c).Grow(1)
				}
				if a.buttonSized(c, a.t("edit.cancel"), a.t("edit.cancel"), iconX, btnSecondary, busy, controlHeight) {
					a.closeAddProject()
				}
				if a.buttonSized(c, a.t("addProject.next"), a.t("addProject.next"), iconArrowRight,
					btnPrimary, !s.canNext(), controlHeight) {
					a.addProjectNextStep()
				}
			})
	})
}

// addProjectRow is one fetched server project: name and title, its ID, the
// version tag and the force-update badge.
func (a *app) addProjectRow(c *ui.Context, project ProjectInfo, selected bool) {
	p := a.pal(c)
	nameColor, mutedColor := p.text, p.textThird
	iconBg, iconFg := p.fill, p.textSecond
	versionTone := toneNeutral
	if selected {
		nameColor, mutedColor = p.onPrimary, p.onPrimary.Alpha(0.75)
		iconBg, iconFg = p.onPrimary.Alpha(0.18), p.onPrimary
		versionTone = tonePrimary
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(7, 10).Children(func() {
		ui.Box(c).Size(26, 26).Radius(8).Background(iconBg).Center().Children(func() {
			ui.Icon(c, iconServer).FontSize(13).TextColor(iconFg)
		})
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, project.Name).FontSize(fontBody).FontWeight(600).
					TextColor(nameColor).SingleLine()
				if project.Title != "" && project.Title != project.Name {
					ui.Text(c, project.Title).FontSize(fontCaption).TextColor(mutedColor).SingleLine()
				}
			})
			ui.Text(c, fmt.Sprintf("ID %d", project.ID)).FontSize(fontMicro).
				TextColor(mutedColor).SingleLine()
		})
		version := strings.TrimSpace(project.Version)
		if version == "" {
			version = a.t("addProject.versionNone")
		}
		if selected {
			if project.ForceUpdate {
				a.tagOnAccent(c, a.t("addProject.forceUpdate"), nil)
			}
			a.tagOnAccent(c, version, iconTag)
			return
		}
		if project.ForceUpdate {
			a.tag(c, a.t("addProject.forceUpdate"), toneWarning)
		}
		a.tagIcon(c, version, iconTag, versionTone)
	})
}

// addProjectEmptyBox is the server step's empty state.
func (a *app) addProjectEmptyBox(c *ui.Context, ic *ui.SVG, title, hint string) {
	p := a.pal(c)
	bg, _, border := toneColors(p, toneNeutral)
	ui.Column(c).Gap(6).Padding(18, 20).Background(bg).Border(1, border).Radius(10).
		Center().Children(func() {
		ui.Icon(c, ic).FontSize(26).TextColor(p.textThird)
		ui.Text(c, title).FontSize(fontLabel).FontWeight(600).TextColor(p.textSecond)
		ui.Text(c, hint).FontSize(fontCaption).TextColor(p.textThird).MaxWidth(380).Wrap()
	})
}

// addProjectSelectedCard shows the server project the local step will add.
func (a *app) addProjectSelectedCard(c *ui.Context, project ProjectInfo) {
	p := a.pal(c)
	bg, _, border := toneColors(p, tonePrimary)
	version := strings.TrimSpace(project.Version)
	if version == "" {
		version = a.t("addProject.versionNone")
	}
	ui.Row(c).Gap(10).Padding(12).Shrink(0).Background(bg).Border(1, border).
		Radius(10).AlignItems(ui.Center).Children(func() {
		a.iconTile(c, iconServer, tonePrimary, 30)
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, project.Name).FontSize(fontBody).FontWeight(600).TextColor(p.text).SingleLine()
			if project.Title != "" {
				ui.Text(c, project.Title).FontSize(fontCaption).TextColor(p.textSecond).SingleLine()
			}
		})
		a.tagIcon(c, version, iconTag, toneNeutral)
	})
}

// addProjectLocalModal is the local step: the project folder, the display name
// and the optional executable/UpdateFolder inputs.
func (a *app) addProjectLocalModal(c *ui.Context, s *addProjectState) {
	a.formModalEsc(c, &a.addOpen, func() {
		if s.busy {
			return
		}
		a.addProjectStage(addStageServer)
	}, func() {
		w, h := dialogSize(c, 800, 700)
		a.dialogShell(c, iconFolderPlus, a.t("addProject.title"), a.t("addProject.localSubtitle"),
			w, h, s.busy, func() {
				if !s.busy {
					a.closeAddProject()
				}
			}, func() {
				a.addProjectStepper(c, addStageLocal)
				project, _ := s.selectedProject()
				a.addProjectSelectedCard(c, project)
				stateText := pathStateText(a.lang, projectPathState(s.path, a.projects))
				pathField := a.field(c, a.t("addProject.path"), func() {
					changed := false
					row := ui.Row(c).Gap(6)
					row.Children(func() {
						if ui.TextInput(c, &s.path).Placeholder(a.t("addProject.pathHint")).
							Label(a.t("addProject.path")).Grow(1).Height(controlHeight).
							Radius(8).Changed() {
							changed = true
						}
						if a.buttonSized(c, a.t("edit.browse"), a.t("edit.browse"), iconFolder,
							btnSecondary, s.busy, controlHeight) {
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
						stateText = pathStateText(a.lang, projectPathState(s.path, a.projects))
					}
				})
				switch state := projectPathState(s.path, a.projects); state {
				case pathMissing, pathListed:
					// A problem shows on the field itself, in the danger tone.
					pathField.Error(stateText)
				default:
					pathField.Description(stateText)
				}
				if projectPathState(s.path, a.projects) == pathEmpty {
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconFolderOpen).FontSize(12).TextColor(a.pal(c).textThird)
						ui.Text(c, a.t("addProject.dropHint")).FontSize(fontMicro).
							TextColor(a.pal(c).textThird).SingleLine()
					})
				}
				a.field(c, a.t("addProject.displayName"), func() {
					ui.TextInput(c, &s.displayName).Placeholder(a.t("addProject.displayNameHint")).
						Height(controlHeight).Radius(8)
				})
				a.field(c, a.t("edit.mainExe"), func() {
					ui.Column(c).Gap(6).Children(func() {
						if len(s.mainExeCandidates) > 0 {
							sel := ui.Select(c, &s.mainExe, s.mainExeCandidates).
								Height(controlHeight).Radius(8)
							if sel.Changed() {
								s.syncCloseProcess()
							}
						}
						mainChanged := false
						ui.Row(c).Gap(6).Children(func() {
							if ui.TextInput(c, &s.mainExe).Placeholder(a.t("edit.mainExePlaceholder")).
								Height(controlHeight).Radius(8).Grow(1).Changed() {
								mainChanged = true
							}
							if a.buttonSized(c, a.t("edit.browse"), a.t("edit.browse"), iconFolder,
								btnSecondary, s.busy, controlHeight) {
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
				})
				a.field(c, a.t("addProject.closeProcess"), func() {
					ui.TextInput(c, &s.closeProcessName).
						Placeholder(a.t("addProject.closeProcessHint")).
						Height(controlHeight).Radius(8)
				})
				ui.Row(c).Children(func() {
					if a.buttonSized(c, a.t("addProject.advanced"), a.t("addProject.advanced"),
						iconSettings, btnGhost, s.busy, controlHeightS) {
						a.addProjectStage(addStageRules)
					}
				})
				if s.busy {
					a.busyRow(c, a.t("addProject.initializing"))
				}
			}, func() {
				validation := a.localValidation(s)
				s.validation = validation
				// The state under the folder field already says what is wrong
				// with the path, so the footer repeats only other problems.
				if validation != "" && validation != pathStateText(a.lang, projectPathState(s.path, a.projects)) {
					a.inlineNotice(c, iconInfo, validation, toneWarning)
				} else {
					ui.Box(c).Grow(1)
				}
				if a.buttonSized(c, a.t("edit.cancel"), a.t("edit.cancel"), iconX, btnSecondary, s.busy, controlHeight) {
					a.closeAddProject()
				}
				if a.buttonSized(c, a.t("addProject.back"), a.t("addProject.back"), iconArrowLeft,
					btnGhost, s.busy, controlHeight) {
					a.addProjectStage(addStageServer)
				}
				if a.buttonSized(c, a.t("addProject.initialize"), a.t("addProject.initialize"), iconCheck,
					btnPrimary, s.busy || validation != "", controlHeight) {
					a.initializeAddProject()
				}
			})
	})
}

// addProjectRulesModal is the local step's rule editor, a sub-page that goes
// back to the local step instead of closing the wizard.
func (a *app) addProjectRulesModal(c *ui.Context, s *addProjectState) {
	a.formModalEsc(c, &a.addOpen, func() {
		if s.busy {
			return
		}
		a.addProjectStage(addStageLocal)
	}, func() {
		w, h := dialogSize(c, 840, 420)
		a.dialogShellTone(c, iconSettings, toneInfo, a.t("addProject.ignoreTitle"),
			a.t("addProject.ignoreHint"), w, h, s.busy, func() {
				if !s.busy {
					a.addProjectStage(addStageLocal)
				}
			}, func() {
				a.ruleGrid(c, s.busy,
					ruleGroup{a.t("edit.ignoreFolders"), &s.ignoreFolders, &s.newIgnoreFolder,
						addProjectIgnoreFolderSuggestions,
						func() { a.addProjectRulesPickFolders(&s.ignoreFolders, a.t("edit.ignoreFolders")) }},
					ruleGroup{a.t("edit.uncopyFolders"), &s.uncopyFolders, &s.newUncopyFolder,
						addProjectIgnoreFolderSuggestions,
						func() { a.addProjectRulesPickFolders(&s.uncopyFolders, a.t("edit.uncopyFolders")) }},
					ruleGroup{a.t("edit.ignoreFiles"), &s.ignoreFiles, &s.newIgnoreFile,
						addProjectIgnoreFileSuggestions,
						func() { a.addProjectRulesPickFiles(&s.ignoreFiles, a.t("edit.ignoreFiles")) }},
					ruleGroup{a.t("edit.uncopyFiles"), &s.uncopyFiles, &s.newUncopyFile,
						addProjectIgnoreFileSuggestions,
						func() { a.addProjectRulesPickFiles(&s.uncopyFiles, a.t("edit.uncopyFiles")) }},
				)
			}, func() {
				ui.Box(c).Grow(1)
				if a.buttonSized(c, a.t("addProject.back"), a.t("addProject.back"), iconArrowLeft,
					btnGhost, s.busy, controlHeight) {
					a.addProjectStage(addStageLocal)
				}
				if a.buttonSized(c, a.t("addProject.done"), a.t("addProject.done"), iconCheck,
					btnPrimary, s.busy, controlHeight) {
					a.addProjectStage(addStageLocal)
				}
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
		for _, picked := range paths {
			a.addProjectPath(list, s.path, picked)
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
		for _, picked := range paths {
			a.addProjectPath(list, s.path, picked)
		}
	})
}

// addProjectCreateModal is the C# CreateProjectDialog: create a server project
// without leaving the wizard. The identifier follows the title until the user
// edits it, as in the C# view model.
func (a *app) addProjectCreateModal(c *ui.Context, s *addProjectState) {
	p := a.pal(c)
	a.formModalEsc(c, &a.addOpen, func() {
		if s.creating {
			return
		}
		a.addProjectStage(addStageServer)
	}, func() {
		w, h := dialogSize(c, 800, 620)
		a.dialogShell(c, iconCloud, a.t("addProject.createHeading"), a.t("addProject.createSubtitle"),
			w, h, s.creating, func() {
				if !s.creating {
					a.addProjectStage(addStageServer)
				}
			}, func() {
				a.field(c, a.t("addProject.server"), func() {
					ui.Text(c, strings.TrimSpace(s.serverURL)).FontSize(fontLabel).
						TextColor(p.textSecond).SingleLine()
				})
				a.field(c, a.t("addProject.titleField"), func() {
					before := s.createTitle
					ui.TextInput(c, &s.createTitle).Placeholder(a.t("addProject.titleHint")).
						Height(controlHeight).Radius(8).AutoFocus()
					if s.createTitle != before && !s.createNameEdited {
						s.createName = strings.TrimSpace(s.createTitle)
					}
				})
				a.field(c, a.t("addProject.nameLabel"), func() {
					before := s.createName
					ui.TextInput(c, &s.createName).Placeholder(a.t("addProject.projectHint")).
						Height(controlHeight).Radius(8)
					if s.createName != before {
						// The user typed the identifier: it stops following
						// the title, as the C# view model does.
						s.createNameEdited = true
					}
				}).Description(a.t("addProject.createNameHint"))
				ui.Column(c).Gap(5).Children(func() {
					ui.Checkbox(c, &s.createForceUpdate, a.t("addProject.forceUpdate"))
					ui.Text(c, a.t("addProject.forceUpdateHint")).FontSize(fontCaption).
						TextColor(p.textThird).Wrap()
				})
				a.ruleGrid(c, s.creating,
					ruleGroup{a.t("edit.ignoreFolders"), &s.createIgnoreFolders, &s.createNewIgnoreFolder,
						addProjectIgnoreFolderSuggestions,
						func() { a.addProjectCreatePickFolders(&s.createIgnoreFolders, a.t("edit.ignoreFolders")) }},
					ruleGroup{a.t("edit.ignoreFiles"), &s.createIgnoreFiles, &s.createNewIgnoreFile,
						addProjectIgnoreFileSuggestions,
						func() { a.addProjectCreatePickFiles(&s.createIgnoreFiles, a.t("edit.ignoreFiles")) }},
				)
				if s.creating {
					a.busyRow(c, a.t("addProject.creating"))
				}
			}, func() {
				validation := a.createValidation(s)
				s.validation = validation
				caption, t := validation, toneWarning
				if s.actionError != "" {
					caption, t = s.actionError, toneDanger
				}
				if caption != "" {
					a.inlineNotice(c, iconInfo, caption, t)
				} else {
					ui.Box(c).Grow(1)
				}
				if a.buttonSized(c, a.t("edit.cancel"), a.t("edit.cancel"), iconX, btnSecondary, s.creating, controlHeight) {
					a.addProjectStage(addStageServer)
				}
				if a.buttonSized(c, a.t("addProject.create"), a.t("addProject.create"), iconPlus,
					btnPrimary, s.creating, controlHeight) {
					a.createServerProject()
				}
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
		for _, picked := range paths {
			if strings.TrimSpace(s.path) != "" {
				a.addProjectPath(list, s.path, picked)
				continue
			}
			if base := filepath.Base(normalizePath(picked)); base != "" && base != "." {
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
		for _, picked := range paths {
			if strings.TrimSpace(s.path) != "" {
				a.addProjectPath(list, s.path, picked)
				continue
			}
			if base := filepath.Base(normalizePath(picked)); base != "" && base != "." {
				*list = appendUnique(*list, base)
			}
		}
	})
}
