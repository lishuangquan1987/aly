package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Detail page metrics: the side column mirrors the C# GridSplitter column,
// the changes pane keeps its own minimum, and the split divider is the hairline
// ui.Split draws between the panes.
const (
	detailPadding      float32 = 16
	detailDefaultSide  float32 = 380
	detailMinSide      float32 = 300
	detailMaxSide      float32 = 540
	detailMinChanges   float32 = 300
	detailSplitDivider float32 = 1
	detailSidebarWidth float32 = 288
)

// detailView is the right-hand project page: header, changes above/below
// split, publish panel and version history.
func (a *app) detailView(c *ui.Context, d *detailState) {
	root := ui.Column(c).Grow(1).FillHeight().Gap(12).Padding(detailPadding)
	root.Children(func() {
		a.detailHeader(c, d)
		if d.busy {
			a.busyRow(c, a.detailBusyText(d))
		}
		a.detailSplit(c, d, root)
	})
}

// detailBusyText names the detail work underway, with a generic caption when
// the operation did not set one.
func (a *app) detailBusyText(d *detailState) string {
	if d != nil && d.busyText != "" {
		return d.busyText
	}
	return a.t("changes.reading")
}

// detailHeader shows the project identity and the header actions.
func (a *app) detailHeader(c *ui.Context, d *detailState) {
	p := a.pal(c)
	busy := d.busy || d.uploading
	ui.Row(c).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
		a.iconTile(c, iconFolderOpen, tonePrimary, 42)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, d.project.DisplayName).FontSize(fontTitle).FontWeight(650).
					TextColor(p.text).SingleLine()
				a.versionTag(c, d.currentVersion)
				if d.busy {
					a.busyRow(c, a.detailBusyText(d))
				}
			})
			ui.Text(c, d.project.ProjectPath).FontSize(fontCaption).TextColor(p.textThird).
				SingleLine().Selectable().Tooltip(d.project.ProjectPath)
		})
		if a.iconButton(c, a.t("actions.folder"), iconFolder, btnGhost, false) {
			a.openProjectFolder()
		}
		if a.iconButton(c, a.t("actions.launch"), iconPlay, btnGhost, false) {
			a.launchMain()
		}
		a.toolbarDivider(c)
		if a.iconButton(c, a.t("detail.refreshTip"), iconRefresh, btnGhost, busy) {
			a.refreshDetail()
		}
		if a.buttonSized(c, a.t("toolbar.edit"), a.t("detail.settingsTip"), iconSettings,
			btnSecondary, d.busy, controlHeightS) {
			a.openEdit()
		}
	})
}

// detailSplit puts the changes area left and the publish/history side right.
func (a *app) detailSplit(c *ui.Context, d *detailState, root ui.Element) {
	total := root.Bounds().W - 2*detailPadding
	if total <= 0 {
		width, _ := c.Size()
		total = float32(width) - detailSidebarWidth - detailSplitDivider - 2*detailPadding
	}
	if total < detailMinSide+detailMinChanges+detailSplitDivider {
		total = detailMinSide + detailMinChanges + detailSplitDivider
	}
	d.splitTotal = total
	side := clampSideWidth(total, d.splitW)
	d.changesWidth = total - side - detailSplitDivider
	d.splitW = side
	first := total - side - detailSplitDivider
	if first < 10 {
		first = 10
	}
	ui.Split(c, &first, func() {
		a.changesView(c, d)
	}, func() {
		a.sidePanel(c, d)
	}).Grow(1)
	// A divider drag wrote the first pane's width back: store it again as the
	// side column's width so the clamp holds from frame to frame.
	if total > detailSplitDivider {
		d.splitW = clampSideWidth(total, total-detailSplitDivider-first)
	}
}

// clampSideWidth keeps the side column between its minimum and maximum and
// leaves the changes pane its own minimum room.
func clampSideWidth(total, side float32) float32 {
	if total <= 0 {
		return detailDefaultSide
	}
	if side <= 0 {
		side = detailDefaultSide
	}
	if side < detailMinSide {
		side = detailMinSide
	}
	if side > detailMaxSide {
		side = detailMaxSide
	}
	if limit := total - detailSplitDivider - detailMinChanges; side > limit && limit >= detailMinSide {
		side = limit
	}
	return side
}

// sidePanel stacks the publish card over the growing history card.
func (a *app) sidePanel(c *ui.Context, d *detailState) {
	ui.Column(c).Grow(1).Shrink(0).FillHeight().Gap(12).Children(func() {
		a.publishView(c, d)
		a.historyView(c, d)
	})
}

// changesView is the card holding the unstaged and staged file tables, with
// the first-load skeleton over it until the first refresh attempt ends.
func (a *app) changesView(c *ui.Context, d *detailState) {
	p := a.pal(c)
	ui.Column(c).Grow(1).FillHeight().Background(p.card).Border(1, p.border).
		Radius(12).Shadow(0, 1, 3, 0, p.shadow).Clip().Children(func() {
		ui.SplitVerticalPercent(c, &d.splitPercent,
			func() { a.filesPanel(c, d, false) },
			func() { a.filesPanel(c, d, true) },
		).Grow(1)
		if d.initialLoading {
			a.detailSkeleton(c)
		}
	})
}

// detailSkeleton is the first-load overlay: a translucent cover with a spinner
// and the "reading project status" caption.
func (a *app) detailSkeleton(c *ui.Context) {
	p := a.pal(c)
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
		Background(p.bg.Alpha(0.88)).Center().Children(func() {
		ui.Column(c).Gap(10).Center().Children(func() {
			ui.Spinner(c).Label(a.t("changes.reading"))
			ui.Text(c, a.t("changes.reading")).FontSize(fontCaption).TextColor(p.textSecond)
		})
	})
}

// filesPanel is one half of the changes card: header actions plus the file
// table, or the empty state when nothing is listed.
func (a *app) filesPanel(c *ui.Context, d *detailState, staged bool) {
	p := a.pal(c)
	files := d.unstaged
	title := a.t("changes.unstaged")
	empty := a.t("changes.emptyUnstaged")
	emptyIcon := iconCheck
	headerIcon := iconFile
	headerTone := toneNeutral
	if staged {
		files = d.staged
		title = a.t("changes.staged")
		empty = a.t("changes.emptyStaged")
		emptyIcon = iconInbox
		headerIcon = iconCheckSquare
		headerTone = toneSuccess
	}
	disabled := d.busy || d.uploading || len(files) == 0
	hasSelected := len(selectedFiles(files)) > 0
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Row(c).Wrap().Gap(6).Padding(8, 12).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(p.divider).Children(func() {
			a.iconTile(c, headerIcon, headerTone, 24)
			ui.Text(c, title).FontSize(fontSection).FontWeight(600).TextColor(p.text)
			a.countTag(c, len(files), toneNeutral)
			ui.Box(c).Grow(1)
			label := a.t("changes.selectAll")
			if allSelected(files) {
				label = a.t("changes.unselectAll")
			}
			if a.buttonSized(c, label, label, iconCheckSquare, btnGhost, disabled, 26) {
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
				if a.buttonSized(c, a.t("changes.unstage"), a.t("changes.unstage"), iconMinusCircle,
					btnSecondary, d.busy || d.uploading || !hasSelected, 26) {
					a.stageSelected(false)
				}
				if a.buttonSized(c, a.t("changes.clear"), a.t("changes.clear"), iconTrash,
					btnDangerGhost, d.busy || d.uploading || len(files) == 0, 26) {
					a.clearOpen = true
				}
			} else {
				if a.buttonSized(c, a.t("changes.stageSelected"), a.t("changes.stageSelected"),
					iconCheck, btnSecondary, d.busy || d.uploading || !hasSelected, 26) {
					a.stageSelected(true)
				}
				if a.buttonSized(c, a.t("changes.stageAll"), a.t("changes.stageAll"),
					iconPlusCircle, btnPrimary, d.busy || d.uploading || len(files) == 0, 26) {
					a.stageAll(true)
				}
			}
		})
		if len(files) == 0 {
			a.emptyState(c, emptyIcon, empty, a.t("changes.emptyHint"), nil)
			return
		}
		narrow := d.changesWidth > 0 && d.changesWidth < 560
		cols := []ui.TableColumn{
			{Title: a.t("table.select"), Width: 56, Fixed: true},
			{Title: a.t("table.file"), Width: 0, MinWidth: 90},
			{Title: a.t("table.status"), Width: 88},
			{Title: a.t("table.local"), Width: 76, Align: ui.End},
			{Title: a.t("table.server"), Width: 76, Align: ui.End},
			{Title: "", Width: 32, Fixed: true},
		}
		if narrow {
			// A narrow changes pane keeps one size column and the open action.
			cols = []ui.TableColumn{
				{Title: a.t("table.select"), Width: 56, Fixed: true},
				{Title: a.t("table.file"), Width: 0, MinWidth: 70},
				{Title: a.t("table.status"), Width: 84},
				{Title: a.t("table.local"), Width: 64, Align: ui.End},
				{Title: "", Width: 30, Fixed: true},
			}
		}
		ui.Table(c, nil, cols, len(files), func(row, col int) {
			f := &files[row]
			switch col {
			case 0:
				ui.Checkbox(c, &f.Selected, "").Label(a.t("table.select"))
			case 1:
				ui.Text(c, f.RelativePath).FontSize(fontCaption).TextColor(p.text).
					SingleLine().Tooltip(f.RelativePath)
			case 2:
				a.statusTag(c, f.Status)
			case 3:
				ui.Text(c, formatSize(f.LocalSize)).FontSize(fontCaption).TextColor(p.textSecond)
			case 4:
				if narrow {
					if a.iconButtonSize(c, a.t("table.open"), iconFolderOpen, btnGhost, false, 22) {
						a.openFileLocation(*f)
					}
				} else {
					ui.Text(c, formatSize(f.ServerSize)).FontSize(fontCaption).TextColor(p.textSecond)
				}
			case 5:
				if a.iconButtonSize(c, a.t("table.open"), iconFolderOpen, btnGhost, false, 22) {
					a.openFileLocation(*f)
				}
			}
		}).Grow(1)
	})
}

// publishHint is the reason the publish button is disabled, or empty when the
// form is complete (C# PublishHint).
func (a *app) publishHint(d *detailState) string {
	switch {
	case len(d.staged) == 0:
		return a.t("publish.stageFirst")
	case strings.TrimSpace(d.newVersion) == "":
		return a.t("publish.needVersion")
	case strings.TrimSpace(d.notes) == "":
		return a.t("publish.needNotes")
	}
	return ""
}

// publishButtonText is "Publish" with no staged files, else the count form.
func (a *app) publishButtonText(d *detailState) string {
	if len(d.staged) == 0 {
		return a.t("publish.publish")
	}
	if len(d.staged) == 1 {
		return a.tf("publish.publishCountOne", "count", "1")
	}
	return a.tf("publish.publishCount", "count", fmt.Sprint(len(d.staged)))
}

// publishView is the release form: version, notes, advanced options, the
// publish button with its hint, and the upload progress card.
func (a *app) publishView(c *ui.Context, d *detailState) {
	canPublish := !d.busy && !d.uploading && len(d.staged) > 0 &&
		strings.TrimSpace(d.newVersion) != "" && strings.TrimSpace(d.notes) != ""
	hint := a.publishHint(d)
	buttonText := a.publishButtonText(d)
	nextVersion := suggestVersion(d.currentVersion)
	ui.Column(c).Shrink(0).Children(func() {
		a.card(c, func() {
			a.cardHeader(c, iconUpload, a.t("publish.title"), len(d.staged), func() {
				a.versionTag(c, d.currentVersion)
			})
			ui.Column(c).Gap(12).Children(func() {
				a.field(c, a.t("publish.version"), func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						in := ui.TextInput(c, &d.newVersion).Placeholder(a.t("publish.versionHint")).
							Label(a.t("publish.version")).Grow(1).Height(controlHeight)
						if in.Submitted() && canPublish {
							a.publish()
						}
						if a.iconButtonSize(c, a.t("publish.nextVersion"), iconSparkles, btnGhost,
							d.busy || d.uploading || nextVersion == "", controlHeight) {
							d.newVersion = nextVersion
						}
					})
				})
				a.field(c, a.t("publish.notes"), func() {
					ui.TextArea(c, &d.notes).Placeholder(a.t("publish.notesHint")).
						Label(a.t("publish.notes")).MinHeight(88).Radius(8)
				})
				ui.Collapsible(c, a.t("publish.advanced"), &d.advancedOpen, func() {
					ui.Column(c).Gap(10).PaddingY(2).Children(func() {
						a.field(c, a.t("publish.script"), func() {
							ui.TextInput(c, &d.script).Placeholder(a.t("publish.scriptHint")).
								Label(a.t("publish.script")).Height(controlHeight)
						})
						ui.Checkbox(c, &d.forceUpdate, a.t("publish.force"))
					})
				})
			})
			tooltip := buttonText
			if hint != "" {
				tooltip = hint
			}
			if a.buttonSized(c, buttonText, tooltip, iconUpload, btnPrimary, !canPublish, 38) {
				a.publish()
			}
			if hint != "" {
				a.inlineNotice(c, iconInfo, hint, toneWarning)
			}
			if d.uploading || len(d.uploadItems) > 0 {
				a.uploadCard(c, d)
			}
		})
	})
}

// uploadCard shows the streamed push progress: caption, bar, summary and the
// per-file rows.
func (a *app) uploadCard(c *ui.Context, d *detailState) {
	p := a.pal(c)
	ui.Column(c).Gap(8).Padding(12).Background(p.fill).Border(1, p.border).
		Radius(10).Children(func() {
		if d.uploadText != "" {
			a.busyRow(c, d.uploadText)
		}
		label := a.t("publish.uploading")
		if d.uploadTotal > 0 {
			ui.Progress(c, float64(d.uploadDone+d.uploadFailed)/float64(d.uploadTotal)).Label(label)
		} else {
			ui.Progress(c, -1).Label(label)
		}
		if d.uploadSummary != "" {
			ui.Text(c, d.uploadSummary).FontSize(fontMicro).TextColor(p.textSecond).SingleLine()
		}
		items := d.uploadItems
		if len(items) > 0 {
			cols := []ui.TableColumn{
				{Title: a.t("table.file"), Width: 0, MinWidth: 90},
				{Title: a.t("table.local"), Width: 64, Align: ui.End},
				{Title: a.t("table.status"), Width: 96},
			}
			ui.Table(c, nil, cols, len(items), func(row, col int) {
				item := items[row]
				switch col {
				case 0:
					ui.Text(c, item.File).FontSize(fontMicro).TextColor(p.text).
						SingleLine().Tooltip(item.File)
				case 1:
					ui.Text(c, formatSize(item.FileSize)).FontSize(fontMicro).TextColor(p.textSecond)
				case 2:
					t := toneNeutral
					switch item.Status {
					case "DONE":
						t = toneSuccess
					case "FAIL":
						t = toneDanger
					}
					a.tag(c, uploadStatusText(item, a.lang), t)
				}
			}).MaxHeight(140).Shrink(0)
		}
		if d.uploading {
			if a.buttonSized(c, a.t("publish.cancel"), a.t("publish.cancel"), iconX, btnDangerGhost, false, 26) {
				a.cancelPublish()
			}
		}
	})
}

// historyView lists releases; the newest version tag uses the accent colour.
func (a *app) historyView(c *ui.Context, d *detailState) {
	p := a.pal(c)
	ui.Column(c).Grow(1).FillHeight().Background(p.card).Border(1, p.border).
		Radius(12).Shadow(0, 1, 3, 0, p.shadow).Clip().Children(func() {
		ui.Row(c).Gap(8).Padding(8, 12).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(p.divider).Children(func() {
			a.iconTile(c, iconHistory, toneInfo, 24)
			ui.Text(c, a.t("history.title")).FontSize(fontSection).FontWeight(600).TextColor(p.text)
			a.countTag(c, len(d.logs), toneNeutral)
			ui.Box(c).Grow(1)
			a.tag(c, a.t("history.latest"), toneNeutral)
			if a.iconButtonSize(c, a.t("detail.refreshTip"), iconRefresh, btnGhost, d.busy || d.uploading, 24) {
				a.refreshDetail()
			}
		})
		if len(d.logs) == 0 {
			a.emptyState(c, iconHistory, a.t("history.empty"), a.t("history.emptyHint"), nil)
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("history.version"), Width: 96},
			{Title: a.t("history.time"), Width: 116},
			{Title: a.t("history.notes"), Width: 0, MinWidth: 80},
		}
		if d.splitW > 0 && d.splitW < 360 {
			cols = []ui.TableColumn{
				{Title: a.t("history.version"), Width: 82},
				{Title: a.t("history.time"), Width: 104},
				{Title: a.t("history.notes"), Width: 0, MinWidth: 60},
			}
		}
		logs := d.logs
		ui.Table(c, nil, cols, len(logs), func(row, col int) {
			log := logs[row]
			switch col {
			case 0:
				if row == 0 {
					a.tagIcon(c, log.Version, iconTag, tonePrimary)
				} else {
					a.tag(c, log.Version, toneNeutral)
				}
			case 1:
				ui.Text(c, log.Time).FontSize(fontMicro).TextColor(p.textSecond).SingleLine()
			case 2:
				ui.Column(c).Gap(2).Children(func() {
					for _, line := range log.Logs {
						ui.Text(c, line).FontSize(fontMicro).TextColor(p.textSecond).Wrap()
					}
				})
			}
		}).Grow(1)
	})
}

// uploadStatusText names a streamed upload row's state.
func uploadStatusText(item UploadProgress, lang string) string {
	switch item.Status {
	case "START":
		if lang == "en" {
			return "Uploading"
		}
		return "上传中"
	case "DONE":
		if lang == "en" {
			return "Done"
		}
		return "完成"
	case "FAIL":
		if item.Error != "" {
			return item.Error
		}
		if lang == "en" {
			return "Failed"
		}
		return "失败"
	}
	return item.Status
}
