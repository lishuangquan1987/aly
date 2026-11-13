package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// actionButton is a labelled button with a leading icon. The label doubles as
// the accessible name and tooltip so every icon action is discoverable.
func (a *app) actionButton(c *ui.Context, label string, ic *ui.SVG, primary, disabled bool) bool {
	return a.actionButtonLabel(c, label, label, ic, primary, disabled)
}

func (a *app) actionButtonLabel(c *ui.Context, visible, tooltip string, ic *ui.SVG, primary, disabled bool) bool {
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c, "")
	} else {
		b = ui.Button(c, "")
	}
	b = b.Label(tooltip).Tooltip(tooltip).Disabled(disabled)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(14)
		ui.Text(c, visible).FontSize(12).SingleLine()
	})
	return b.Clicked()
}

// iconButton is a compact icon-only action. It always gets a label and tooltip
// so it remains usable with a screen reader.
func (a *app) iconButton(c *ui.Context, label string, ic *ui.SVG, disabled bool) bool {
	b := ui.Button(c, "").Label(label).Tooltip(label).Disabled(disabled).Width(30).Height(30)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(15)
	})
	return b.Clicked()
}

func (a *app) toolbarSeparator(c *ui.Context) {
	ui.Box(c).Width(1).Height(20).Background(c.Theme().Border).Margin(2, 0)
}

func (a *app) countPill(c *ui.Context, n int) {
	t := c.Theme()
	ui.Text(c, fmt.Sprint(n)).FontSize(10).Padding(6, 1).Background(t.Background).
		Radius(8).TextColor(t.TextMuted)
}

func (a *app) sectionHeader(c *ui.Context, ic *ui.SVG, title string, count int) {
	t := c.Theme()
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(15).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(13).Bold()
		a.countPill(c, count)
	})
}

func (a *app) fieldLabel(c *ui.Context, text string) {
	t := c.Theme()
	ui.Text(c, text).FontSize(10).Bold().TextColor(t.TextMuted)
}

func (a *app) statusDot(c *ui.Context, col ui.Color) {
	ui.Box(c).Width(7).Height(7).Radius(4).Background(col)
}

func (a *app) versionPill(c *ui.Context, version string) {
	t := c.Theme()
	ui.Row(c).Gap(4).Padding(8, 3).Background(t.Background).Border(1, t.Border).
		Radius(999).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconTag).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, version).FontSize(11).Bold()
	})
}

func (a *app) emptyState(c *ui.Context, ic *ui.SVG, title, hint string) {
	t := c.Theme()
	ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
		ui.Icon(c, ic).FontSize(30).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(13).TextColor(t.TextMuted)
		if hint != "" {
			ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).MaxWidth(320).Wrap()
		}
	})
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.toolbar(c)
		if !a.cli.available() {
			a.cliBanner(c)
		}
		ui.Row(c).Grow(1).Shrink(1).Children(func() {
			a.sidebar(c)
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			a.content(c)
		})
		a.statusBar(c)
		if a.editOpen && a.editing != nil {
			a.editModal(c)
		}
		if a.removeOpen {
			switch ui.AlertDialog(c, &a.removeOpen,
				a.t("remove.title"),
				a.t("remove.message"),
				a.t("common.cancel"),
				a.t("common.remove"),
			) {
			case 1:
				a.removeSelected()
			}
		}
		if a.clearOpen {
			switch ui.AlertDialog(c, &a.clearOpen,
				a.t("changes.clearTitle"),
				a.t("changes.clearMessage"),
				a.t("common.cancel"),
				a.t("changes.clear"),
			) {
			case 1:
				a.stageAll(false)
			}
		}
		if a.toastMsg != "" {
			msg := a.toastMsg
			a.toastMsg = ""
			c.Toast(msg)
		}
	})
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	detailBusy := a.detail != nil && (a.detail.busy || a.detail.uploading)
	ui.Row(c).Height(56).Shrink(0).Gap(8).Padding(14, 10).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		if a.actionButton(c, a.t("toolbar.addLocal"), iconFolderPlus, true, false) {
			a.addLocalProject()
		}
		a.toolbarSeparator(c)
		if a.actionButton(c, a.t("toolbar.edit"), iconSettings, false, a.detail == nil) {
			a.openEdit()
		}
		if a.actionButton(c, a.t("toolbar.remove"), iconTrash, false, a.detail == nil) {
			a.removeOpen = true
		}
		if a.actionButton(c, a.t("toolbar.refresh"), iconRefresh, false, a.detail == nil || detailBusy) {
			a.refreshDetail()
		}
		ui.Box(c).Grow(1)
		ui.Row(c).Gap(6).Padding(8, 4).Background(t.Background).Border(1, t.Border).
			Radius(999).AlignItems(ui.Center).Children(func() {
			col := t.Success
			label := a.t("toolbar.cliReady")
			if !a.cli.available() {
				col = t.Danger
				label = a.t("toolbar.cliMissing")
			}
			a.statusDot(c, col)
			ui.Text(c, label).FontSize(10).TextColor(col)
		})
		a.toolbarSeparator(c)
		langLabel := "EN"
		if a.lang == "en" {
			langLabel = "中文"
		}
		if a.actionButtonLabel(c, langLabel, a.t("toolbar.language"), iconGlobe, false, false) {
			a.toggleLang()
		}
	})
}

func (a *app) cliBanner(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(14, 8).Shrink(0).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Danger).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconWarning).FontSize(15).TextColor(t.Danger)
		ui.Text(c, a.t("cli.banner")).FontSize(11).TextColor(t.Danger).Grow(1).Wrap()
		if a.actionButton(c, a.t("toolbar.retry"), iconRefresh, false, false) {
			a.cli = &cliClient{exe: findCLI()}
			if a.cli.available() {
				a.status = a.tf("cli.found", "path", a.cli.exe)
				a.refreshDetail()
			}
		}
	})
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	list := a.filteredProjects()
	if a.selected >= len(list) {
		a.selected = len(list) - 1
	}
	if a.selected < 0 && len(list) > 0 {
		a.selected = 0
	}
	a.list.Selected = &a.selected
	ui.Column(c).Width(300).Shrink(0).FillHeight().Gap(10).Padding(14).
		Background(t.Surface).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.t("projects.title")).Bold().FontSize(13)
			a.countPill(c, len(a.projects))
		})
		if ui.SearchField(c, &a.filter).Placeholder(a.t("projects.search")).
			Label(a.t("projects.search")).Changed() {
			a.selected = 0
			a.openSelected()
		}
		switch {
		case len(a.projects) > 0 && len(list) == 0 && strings.TrimSpace(a.filter) != "":
			a.emptyState(c, iconInbox, a.t("projects.noMatch"), "")
		case len(list) == 0:
			a.emptyState(c, iconFolderPlus, a.t("projects.emptyTitle"), a.t("projects.emptyHint"))
		default:
			if ui.List(c, &a.list, len(list), func(i int) {
				p := list[i]
				selected := i == a.selected
				nameColor, pathColor, iconColor := t.Text, t.TextMuted, t.TextMuted
				if selected {
					nameColor, pathColor, iconColor = t.AccentText, t.AccentText, t.AccentText
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(7, 5).Children(func() {
					ui.Icon(c, iconFolder).FontSize(16).TextColor(iconColor)
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Text(c, p.DisplayName).FontSize(13).Bold().TextColor(nameColor).SingleLine()
						ui.Text(c, p.ProjectPath).FontSize(10).TextColor(pathColor).
							SingleLine().Tooltip(p.ProjectPath)
					})
				})
			}).Grow(1).Changed() {
				a.openSelected()
			}
		}
	})
}

func (a *app) content(c *ui.Context) {
	if a.detail == nil {
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.emptyState(c, iconFolder, a.t("content.emptyTitle"), a.t("content.emptyHint"))
		})
		return
	}
	a.detailView(c, a.detail)
}

func (a *app) detailView(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Gap(12).Padding(16).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFolderOpen).FontSize(20).TextColor(t.Accent)
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, d.project.DisplayName).FontSize(17).Bold().SingleLine()
				ui.Text(c, d.project.ProjectPath).FontSize(10).TextColor(t.TextMuted).
					SingleLine().Tooltip(d.project.ProjectPath)
			})
			if d.currentVersion != "" {
				a.versionPill(c, d.currentVersion)
			} else {
				ui.Text(c, a.t("history.none")).FontSize(11).TextColor(t.TextMuted)
			}
			if a.iconButton(c, a.t("actions.folder"), iconFolder, false) {
				a.openProjectFolder()
			}
			if a.iconButton(c, a.t("actions.launch"), iconPlay, false) {
				a.launchMain()
			}
		})
		if d.busy {
			ui.Progress(c, -1).Width(160)
		}
		ui.Row(c).Grow(1).Gap(14).Children(func() {
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				a.changesView(c, d)
			})
			ui.Column(c).Width(420).Shrink(0).FillHeight().Gap(10).Children(func() {
				a.publishView(c, d)
				a.historyView(c, d)
			})
		})
	})
}

func (a *app) changesView(c *ui.Context, d *detailState) {
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.SplitVerticalPercent(c, &d.splitPercent,
			func() { a.filesPanel(c, d, false) },
			func() { a.filesPanel(c, d, true) },
		).Grow(1)
	})
}

func (a *app) filesPanel(c *ui.Context, d *detailState, staged bool) {
	t := c.Theme()
	files := d.unstaged
	title := a.t("changes.unstaged")
	empty := a.t("changes.emptyUnstaged")
	icon := iconFile
	if staged {
		files = d.staged
		title = a.t("changes.staged")
		empty = a.t("changes.emptyStaged")
		icon = iconCheckSquare
	}
	disabled := d.busy || d.uploading || len(files) == 0
	hasSelected := len(selectedFiles(files)) > 0
	ui.Column(c).Grow(1).FillHeight().Gap(8).Padding(12).Background(t.Surface).
		Border(1, t.Border).Radius(8).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			a.sectionHeader(c, icon, title, len(files))
			ui.Box(c).Grow(1)
			label := a.t("changes.selectAll")
			if allSelected(files) {
				label = a.t("changes.unselectAll")
			}
			if a.actionButton(c, label, iconCheckSquare, false, disabled) {
				selectAll := !allSelected(files)
				for i := range files {
					files[i].Selected = selectAll
				}
				if staged {
					d.staged = files
				} else {
					d.unstaged = files
				}
			}
			if staged {
				if a.actionButton(c, a.t("changes.unstage"), iconMinusCircle, false, d.busy || d.uploading || !hasSelected) {
					a.stageSelected(false)
				}
				if a.actionButton(c, a.t("changes.clear"), iconTrash, false, d.busy || d.uploading || len(files) == 0) {
					a.clearOpen = true
				}
			} else {
				if a.actionButton(c, a.t("changes.stageSelected"), iconCheck, false, d.busy || d.uploading || !hasSelected) {
					a.stageSelected(true)
				}
				if a.actionButton(c, a.t("changes.stageAll"), iconPlusCircle, true, d.busy || d.uploading || len(files) == 0) {
					a.stageAll(true)
				}
			}
		})
		if len(files) == 0 {
			a.emptyState(c, iconInbox, empty, a.t("changes.emptyHint"))
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("table.select"), Width: 84},
			{Title: a.t("table.file"), Width: 0},
			{Title: a.t("table.status"), Width: 92},
			{Title: a.t("table.local"), Width: 84, Align: ui.End},
			{Title: a.t("table.server"), Width: 84, Align: ui.End},
			{Title: "", Width: 56},
		}
		ui.Table(c, nil, cols, len(files), func(row, col int) {
			f := &files[row]
			switch col {
			case 0:
				ui.Checkbox(c, &f.Selected, "").Label(a.t("table.select"))
			case 1:
				ui.Text(c, f.RelativePath).FontSize(12).SingleLine().Tooltip(f.RelativePath)
			case 2:
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					a.statusDot(c, statusColor(t, f.Status))
					ui.Text(c, statusDisplay(f.Status, a.lang)).FontSize(11).
						TextColor(statusColor(t, f.Status))
				})
			case 3:
				ui.Text(c, formatSize(f.LocalSize)).FontSize(11).TextColor(t.TextMuted)
			case 4:
				ui.Text(c, formatSize(f.ServerSize)).FontSize(11).TextColor(t.TextMuted)
			case 5:
				if a.iconButton(c, a.t("table.open"), iconExternal, false) {
					a.openFileLocation(*f)
				}
			}
		}).Grow(1)
	})
}

func (a *app) publishView(c *ui.Context, d *detailState) {
	t := c.Theme()
	canPublish := !d.busy && !d.uploading && len(d.staged) > 0 &&
		strings.TrimSpace(d.newVersion) != "" && strings.TrimSpace(d.notes) != ""
	ui.Column(c).Gap(10).Padding(14).Background(t.Surface).
		Border(1, t.Border).Radius(8).Children(func() {
		a.sectionHeader(c, iconUpload, a.t("publish.title"), len(d.staged))
		a.fieldLabel(c, a.t("publish.version"))
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &d.newVersion).Placeholder(a.t("publish.versionHint")).Grow(1)
			nextVersion := suggestVersion(d.currentVersion)
			if a.iconButton(c, a.t("publish.nextVersion"), iconPlus, d.busy || nextVersion == "") {
				d.newVersion = nextVersion
			}
		})
		a.fieldLabel(c, a.t("publish.notes"))
		ui.TextArea(c, &d.notes).Placeholder(a.t("publish.notesHint")).MinHeight(80)
		ui.Collapsible(c, a.t("publish.advanced"), &d.advancedOpen, func() {
			ui.Column(c).Gap(8).Children(func() {
				a.fieldLabel(c, a.t("publish.script"))
				ui.TextInput(c, &d.script).Placeholder(a.t("publish.scriptHint"))
				ui.Checkbox(c, &d.forceUpdate, a.t("publish.force"))
			})
		})
		pub := ui.PrimaryButton(c, "").Label(a.t("publish.publish")).Tooltip(a.t("publish.publish")).
			Disabled(!canPublish).Height(36).FillWidth()
		pub.Children(func() {
			ui.Icon(c, iconUpload).FontSize(15)
			ui.Text(c, a.t("publish.publish")).FontSize(12).SingleLine()
		})
		if pub.Clicked() {
			a.publish()
		}
		hint := ""
		switch {
		case len(d.staged) == 0:
			hint = a.t("publish.stageFirst")
		case strings.TrimSpace(d.newVersion) == "":
			hint = a.t("publish.needVersion")
		case strings.TrimSpace(d.notes) == "":
			hint = a.t("publish.needNotes")
		}
		if hint != "" {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconInfo).FontSize(12).TextColor(t.Warning)
				ui.Text(c, hint).FontSize(11).TextColor(t.Warning).Wrap()
			})
		}
		if d.uploading || len(d.uploadItems) > 0 {
			ui.Box(c).Height(1).Background(t.Border)
		}
		if d.uploading || len(d.uploadItems) > 0 {
			ui.Text(c, d.uploadText).FontSize(11).TextColor(t.TextMuted).SingleLine()
			progress := -1.0
			if d.uploadTotal > 0 {
				progress = float64(d.uploadDone+d.uploadFailed) / float64(d.uploadTotal)
			}
			ui.Progress(c, progress)
			if d.uploadSummary != "" {
				ui.Text(c, d.uploadSummary).FontSize(11).TextColor(t.TextMuted)
			}
			uploadCols := []ui.TableColumn{
				{Title: a.t("table.file"), Width: 0},
				{Title: a.t("table.local"), Width: 70, Align: ui.End},
				{Title: a.t("table.status"), Width: 112},
			}
			items := d.uploadItems
			ui.Table(c, nil, uploadCols, len(items), func(row, col int) {
				item := items[row]
				switch col {
				case 0:
					ui.Text(c, item.File).FontSize(11).SingleLine().Tooltip(item.File)
				case 1:
					ui.Text(c, formatSize(item.FileSize)).FontSize(11).TextColor(t.TextMuted)
				case 2:
					ui.Text(c, uploadStatusText(item, a.lang)).FontSize(11).
						TextColor(uploadStatusColor(t, item.Status))
				}
			}).MaxHeight(150)
			if d.uploading {
				if a.actionButton(c, a.t("publish.cancel"), iconX, false, false) {
					a.cancelPublish()
				}
			}
		}
	})
}

func (a *app) historyView(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Gap(8).Padding(12).Background(t.Surface).
		Border(1, t.Border).Radius(8).Children(func() {
		a.sectionHeader(c, iconHistory, a.t("history.title"), len(d.logs))
		if len(d.logs) == 0 {
			a.emptyState(c, iconHistory, a.t("history.empty"), "")
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("history.version"), Width: 82},
			{Title: a.t("history.time"), Width: 126},
			{Title: a.t("history.notes"), Width: 0},
		}
		logs := d.logs
		ui.Table(c, nil, cols, len(logs), func(row, col int) {
			log := logs[row]
			switch col {
			case 0:
				ui.Text(c, log.Version).FontSize(11).Bold()
			case 1:
				ui.Text(c, log.Time).FontSize(10).TextColor(t.TextMuted)
			case 2:
				ui.Column(c).Gap(2).Children(func() {
					for _, line := range log.Logs {
						ui.Text(c, line).FontSize(11).TextColor(t.TextMuted).Wrap()
					}
				})
			}
		}).Grow(1)
	})
}

func (a *app) statusBar(c *ui.Context) {
	t := c.Theme()
	status := a.status
	if a.detail != nil && a.detail.status != "" {
		status = a.detail.status
	}
	col := t.Success
	if !a.cli.available() {
		col = t.Danger
	}
	ui.Row(c).Height(30).Shrink(0).Padding(14, 0).Background(t.Surface).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		a.statusDot(c, col)
		ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).SingleLine().Grow(1)
		if a.detail != nil {
			ui.Text(c, a.detail.project.ProjectPath).FontSize(10).TextColor(t.TextMuted).SingleLine().
				MaxWidth(460).Tooltip(a.detail.project.ProjectPath)
		}
	})
}
