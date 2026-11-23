package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// ignoreFolderSuggestions and ignoreFileSuggestions mirror the C# GUI's
// AppConstants lists offered as one-click rule chips in the project settings
// dialog and the wizard.
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
	disabled := e.loadError != ""
	validation := a.editValidation(e)
	e.validation = validation
	w, h := dialogSize(c, 880, 780)
	a.formModal(c, &a.editOpen, func() {
		a.dialogShell(c, iconSettings, a.t("edit.title"), a.tf("edit.subtitle",
			"project", e.project.DisplayName), w, h, e.busy, func() {
			a.editOpen = false
		}, func() {
			if disabled {
				a.notice(c, iconWarning, e.loadError, toneDanger)
			}
			submitted := a.editLocalCard(c, e, disabled)
			if a.editConfigCard(c, e, disabled) {
				submitted = true
			}
			// Enter in the server, project or display name field saves, as
			// the C# dialog's default button does.
			if submitted && !disabled && !e.busy && validation == "" {
				a.saveEdit()
			}
		}, func() {
			if e.error != "" {
				a.inlineNotice(c, iconErrorCircle, e.error, toneDanger)
			} else if validation != "" && !disabled {
				a.inlineNotice(c, iconWarning, validation, toneWarning)
			} else {
				ui.Box(c).Grow(1)
			}
			if a.buttonSized(c, a.t("edit.cancel"), a.t("edit.cancel"), iconX, btnSecondary, e.busy, controlHeight) {
				a.editOpen = false
			}
			if a.buttonSized(c, a.t("edit.save"), a.t("edit.save"), iconCheck, btnPrimary,
				e.busy || disabled || validation != "", controlHeight) {
				a.saveEdit()
			}
		})
	})
}

// editLocalCard holds the bookmark fields: the list name and the main
// executable the "Launch" action uses. Neither lives in shared.json.
func (a *app) editLocalCard(c *ui.Context, e *editState, disabled bool) bool {
	submitted := false
	ui.Column(c).Gap(8).Disabled(disabled).Children(func() {
		a.cardSectionTitle(c, iconMonitor, toneInfo, a.t("edit.localDisplay"))
		a.field(c, a.t("edit.displayName"), func() {
			in := ui.TextInput(c, &e.displayName).Height(controlHeight).Radius(8).
				Tooltip(a.t("edit.displayNameHint"))
			if in.Submitted() {
				submitted = true
			}
		}).Error(requiredText(a.t("edit.requireDisplayName"), e.displayName))
		a.field(c, a.t("edit.mainExe"), func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.TextInput(c, &e.mainExe).Placeholder(a.t("edit.mainExePlaceholder")).
					Height(controlHeight).Radius(8).Grow(1).
					Tooltip(a.mainExeHintText(e))
				if a.buttonSized(c, a.t("edit.browse"), a.t("edit.browse"), iconFolder,
					btnSecondary, e.busy || disabled, controlHeight) {
					a.pickExecutable(a.t("edit.mainExe"), func(path string) {
						e.mainExe = path
					})
				}
			})
		})
	})
	return submitted
}

// editConfigCard holds the fields that live in .updator/shared.json: the
// server target and the four ignore/not-copy rule lists.
func (a *app) editConfigCard(c *ui.Context, e *editState, disabled bool) bool {
	submitted := false
	ui.Column(c).Gap(8).Disabled(disabled).Children(func() {
		a.cardSectionTitle(c, iconServer, tonePrimary, a.t("edit.configSection"))
		a.field(c, a.t("edit.serverURL"), func() {
			in := ui.TextInput(c, &e.serverURL).Placeholder(a.t("addProject.serverHint")).
				Height(controlHeight).Radius(8)
			if !disabled {
				// The C# dialog focuses the server field when it opens.
				in.AutoFocus()
			}
			if in.Submitted() {
				submitted = true
			}
		}).Error(requiredText(a.t("edit.requireServer"), e.serverURL))
		if chips := a.editKnownServers(e); len(chips) > 0 {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				a.fieldLabel(c, a.t("edit.knownServers"))
				a.suggestionTags(c, chips, disabled, func(value string) {
					e.serverURL = value
				})
			})
		}
		a.field(c, a.t("edit.projectName"), func() {
			in := ui.TextInput(c, &e.projectName).Placeholder(a.t("addProject.projectHint")).
				Height(controlHeight).Radius(8)
			if in.Submitted() {
				submitted = true
			}
		}).Description(a.t("edit.projectNameHint")).
			Error(requiredText(a.t("edit.requireProject"), e.projectName))
		a.ruleGrid(c, disabled,
			ruleGroup{a.t("edit.ignoreFolders"), &e.ignoreFolders, &e.newIgnoreFolder,
				ignoreFolderSuggestions,
				func() { a.pickFolderRules(&e.ignoreFolders, a.t("edit.ignoreFolders"), e.project.ProjectPath) }},
			ruleGroup{a.t("edit.uncopyFolders"), &e.uncopyFolders, &e.newUncopyFolder,
				ignoreFolderSuggestions,
				func() { a.pickFolderRules(&e.uncopyFolders, a.t("edit.uncopyFolders"), e.project.ProjectPath) }},
			ruleGroup{a.t("edit.ignoreFiles"), &e.ignoreFiles, &e.newIgnoreFile,
				ignoreFileSuggestions,
				func() { a.pickFileRules(&e.ignoreFiles, a.t("edit.ignoreFiles"), e.project.ProjectPath) }},
			ruleGroup{a.t("edit.uncopyFiles"), &e.uncopyFiles, &e.newUncopyFile,
				ignoreFileSuggestions,
				func() { a.pickFileRules(&e.uncopyFiles, a.t("edit.uncopyFiles"), e.project.ProjectPath) }},
		)
	})
	return submitted
}

// ruleGroup is one editable rule list for ruleGrid.
type ruleGroup struct {
	label       string
	list        *[]string
	input       *string
	suggestions []string
	pick        func()
}

// ruleGrid lays out rule groups in two aligned columns: folder rules in the
// first row, file rules in the second. Every cell keeps the same structure
// (label, suggestion chips, input) so the rows line up.
func (a *app) ruleGrid(c *ui.Context, disabled bool, groups ...ruleGroup) {
	ui.Column(c).Gap(8).Children(func() {
		for row := 0; row+1 < len(groups); row += 2 {
			left, right := groups[row], groups[row+1]
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				a.ruleCell(c, disabled, left)
				a.ruleCell(c, disabled, right)
			})
		}
	})
}

func (a *app) ruleCell(c *ui.Context, disabled bool, g ruleGroup) {
	ui.Column(c).Grow(1).Basis(0).Gap(5).Children(func() {
		if len(g.suggestions) > 0 {
			a.suggestionTags(c, g.suggestions, disabled, func(value string) {
				*g.list = appendUnique(*g.list, value)
			})
		}
		a.editGroup(c, g.label, g.list, g.input, disabled, g.pick)
	})
}

// cardSectionTitle heads a block inside a dialog body with a tinted icon.
func (a *app) cardSectionTitle(c *ui.Context, ic *ui.SVG, t tone, title string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		a.plainIcon(c, ic, t, 16)
		a.sectionTitle(c, title)
	})
}

// pickFolderRules opens the folder picker for one rule list, constraining the
// choice to the project folder.
func (a *app) pickFolderRules(list *[]string, title, projectPath string) {
	a.pickFolder(true, title, func(paths []string) {
		for _, path := range paths {
			a.addProjectPath(list, projectPath, path)
		}
	})
}

// pickFileRules opens the file picker for one rule list.
func (a *app) pickFileRules(list *[]string, title, projectPath string) {
	a.pickFiles(title, func(paths []string) {
		for _, path := range paths {
			a.addProjectPath(list, projectPath, path)
		}
	})
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
