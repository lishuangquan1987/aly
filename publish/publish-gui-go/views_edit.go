package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// ignoreFolderSuggestions and ignoreFileSuggestions mirror the C# GUI's
// AppConstants lists offered as one-click rule chips in the project settings
// dialog.
var ignoreFolderSuggestions = []string{"node_modules", ".git", ".vs", "bin", "obj", "dist", "logs"}
var ignoreFileSuggestions = []string{"*.log", "*.tmp", "*.pdb", "Thumbs.db", ".DS_Store"}

// editModal is the project settings dialog, aligned with the C# EditProjectDialog:
// a local bookmark card and a .updator/shared.json card, validation in the
// footer, and a scrollable body so the dialog fits the smallest window.
func (a *app) editModal(c *ui.Context) {
	e := a.editing
	if e == nil {
		return
	}
	t := c.Theme()
	disabled := e.loadError != ""
	validation := a.editValidation(e)
	e.validation = validation
	w, h := dialogSize(c, 820, 700)
	a.formModal(c, &a.editOpen, func() {
		ui.Column(c).Role(ui.RoleDialog).Width(w).Height(h).Gap(10).Padding(18).
			Background(t.Surface).Radius(12).Border(1, t.Border).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
			a.editHeader(c, e)
			if disabled {
				a.noticeBox(c, iconWarning, e.loadError, t.Danger, true)
			}
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Gap(12).Disabled(disabled).Children(func() {
					submitted := a.editLocalCard(c, e)
					if a.editConfigCard(c, e, disabled) {
						submitted = true
					}
					// Enter in the server, project or display name field saves,
					// as the C# dialog's default button does.
					if submitted && !disabled && !e.busy && a.editValidation(e) == "" {
						a.saveEdit()
					}
				})
			})
			if e.error != "" {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconWarning).FontSize(12).TextColor(t.Danger)
					ui.Text(c, e.error).FontSize(11).TextColor(t.Danger).Wrap().Grow(1)
				})
			}
			if e.busy {
				a.busyRow(c, a.t("edit.saving"))
			}
			ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Children(func() {
				if validation != "" && !disabled {
					ui.Icon(c, iconWarning).FontSize(12).TextColor(t.Warning)
					ui.Text(c, validation).FontSize(11).TextColor(t.Warning).Wrap().Grow(1)
				} else {
					ui.Box(c).Grow(1)
				}
				if a.actionButton(c, a.t("edit.cancel"), iconX, false, e.busy) {
					a.editOpen = false
				}
				if a.actionButton(c, a.t("edit.save"), iconCheck, true, e.busy || disabled || validation != "") {
					a.saveEdit()
				}
			})
		})
	})
}

func (a *app) editHeader(c *ui.Context, e *editState) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconSettings).FontSize(18).TextColor(t.Accent)
		ui.Text(c, a.t("edit.title")).FontSize(16).Bold()
		ui.Box(c).Grow(1)
		if a.ghostIconButton(c, a.t("edit.cancel"), iconX, e.busy) {
			a.editOpen = false
		}
	})
}

// editLocalCard holds the bookmark fields: the list name and the main
// executable the "Launch" action uses. Neither lives in shared.json.
func (a *app) editLocalCard(c *ui.Context, e *editState) bool {
	t := c.Theme()
	submitted := false
	ui.Column(c).Gap(10).Padding(14, 12).Background(t.Background).
		Border(1, t.Border).Radius(8).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconMonitor).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, a.t("edit.localDisplay")).FontSize(13).Bold()
		})
		ui.Form(c, func() {
			ui.Field(c, a.t("edit.displayName"), func() {
				in := ui.TextInput(c, &e.displayName).Grow(1)
				if in.Submitted() {
					submitted = true
				}
			}).Description(a.t("edit.displayNameHint")).
				Error(requiredText(a.t("edit.requireDisplayName"), e.displayName))
			ui.Field(c, a.t("edit.mainExe"), func() {
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &e.mainExe).Placeholder(a.t("edit.mainExeHint")).Grow(1)
					if a.actionButton(c, a.t("edit.browse"), iconFolder, false, e.busy) {
						a.pickExecutable(a.t("edit.mainExe"), func(path string) {
							e.mainExe = path
						})
					}
				})
			}).Description(a.mainExeHintText(e))
		})
	})
	return submitted
}

// editConfigCard holds the fields that live in .updator/shared.json: the
// server target and the four ignore/not-copy rule lists.
func (a *app) editConfigCard(c *ui.Context, e *editState, disabled bool) bool {
	t := c.Theme()
	submitted := false
	ui.Column(c).Gap(10).Padding(14, 12).Background(t.Background).
		Border(1, t.Border).Radius(8).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconSettings).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, a.t("edit.configSection")).FontSize(13).Bold()
		})
		ui.Form(c, func() {
			ui.Field(c, a.t("edit.serverURL"), func() {
				in := ui.TextInput(c, &e.serverURL).Placeholder(a.t("addProject.serverHint")).Grow(1)
				if !disabled {
					// The C# dialog focuses this field when it opens.
					in.AutoFocus()
				}
				if in.Submitted() {
					submitted = true
				}
				if chips := a.editKnownServers(e); len(chips) > 0 {
					a.suggestionChips(c, chips, func(value string) {
						e.serverURL = value
					})
				}
			}).Error(requiredText(a.t("edit.requireServer"), e.serverURL))
			ui.Field(c, a.t("edit.projectName"), func() {
				in := ui.TextInput(c, &e.projectName).Placeholder(a.t("addProject.projectHint")).Grow(1)
				if in.Submitted() {
					submitted = true
				}
			}).Description(a.t("edit.projectNameHint")).
				Error(requiredText(a.t("edit.requireProject"), e.projectName))
		})
		ui.Row(c).Gap(14).Children(func() {
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				a.suggestionChips(c, ignoreFolderSuggestions, func(value string) {
					e.ignoreFolders = appendUnique(e.ignoreFolders, value)
				})
				a.editGroup(c, a.t("edit.ignoreFolders"), &e.ignoreFolders, &e.newIgnoreFolder, func() {
					a.pickFolder(true, a.t("edit.ignoreFolders"), func(paths []string) {
						for _, path := range paths {
							a.addProjectPath(&e.ignoreFolders, e.project.ProjectPath, path)
						}
					})
				})
				a.suggestionChips(c, ignoreFileSuggestions, func(value string) {
					e.ignoreFiles = appendUnique(e.ignoreFiles, value)
				})
				a.editGroup(c, a.t("edit.ignoreFiles"), &e.ignoreFiles, &e.newIgnoreFile, func() {
					a.pickFiles(a.t("edit.ignoreFiles"), func(paths []string) {
						for _, path := range paths {
							a.addProjectPath(&e.ignoreFiles, e.project.ProjectPath, path)
						}
					})
				})
			})
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				a.editGroup(c, a.t("edit.uncopyFolders"), &e.uncopyFolders, &e.newUncopyFolder, func() {
					a.pickFolder(true, a.t("edit.uncopyFolders"), func(paths []string) {
						for _, path := range paths {
							a.addProjectPath(&e.uncopyFolders, e.project.ProjectPath, path)
						}
					})
				})
				a.editGroup(c, a.t("edit.uncopyFiles"), &e.uncopyFiles, &e.newUncopyFile, func() {
					a.pickFiles(a.t("edit.uncopyFiles"), func(paths []string) {
						for _, path := range paths {
							a.addProjectPath(&e.uncopyFiles, e.project.ProjectPath, path)
						}
					})
				})
			})
		})
	})
	return submitted
}

// editKnownServers are the one-click server suggestions: computed once when the
// dialog opens, so the view does not re-read every bookmark's shared.json on
// each frame.
func (a *app) editKnownServers(e *editState) []string {
	if e.serversLoaded {
		return e.knownServers
	}
	return knownServerURLs(a.projects)
}

func requiredText(message, value string) string {
	if strings.TrimSpace(value) == "" {
		return message
	}
	return ""
}
