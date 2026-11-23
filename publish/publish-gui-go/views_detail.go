package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Detail page metrics. The side column mirrors the C# GridSplitter column
// (380 DIP default, kept between 260 and 520), and the split divider is the
// 1 DIP line ui.Split draws between its panes.
const (
	detailPadding      float32 = 14
	detailDefaultSide  float32 = 380
	detailMinSide      float32 = 260
	detailMaxSide      float32 = 520
	detailMinChanges   float32 = 240
	detailSplitDivider float32 = 1
	// detailSidebarWidth is the fixed project list width in views.go; it only
	// feeds the fallback split measurement on the very first frame.
	detailSidebarWidth float32 = 300
)

// detailView is the right-hand project page: header, changes above/below
// split, publish panel and version history.
func (a *app) detailView(c *ui.Context, d *detailState) {
	root := ui.Column(c).Grow(1).FillHeight().Gap(8).Padding(detailPadding)
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

// detailHeader shows the project identity and the header actions the C#
// ProjectTabView has: folder, launch, refresh (F5) and project settings.
func (a *app) detailHeader(c *ui.Context, d *detailState) {
	t := c.Theme()
	busy := d.busy || d.uploading
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconFolderOpen).FontSize(20).TextColor(t.Accent)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, d.project.DisplayName).FontSize(17).Bold().SingleLine()
			ui.Text(c, d.project.ProjectPath).FontSize(10).TextColor(t.TextMuted).
				SingleLine().Tooltip(d.project.ProjectPath)
		})
		if d.currentVersion != "" {
			a.versionPill(c, d.currentVersion)
		} else {
			ui.Text(c, a.t("history.none")).FontSize(11).TextColor(t.TextMuted).SingleLine()
		}
		a.toolbarSeparator(c)
		if a.ghostIconButton(c, a.t("actions.folder"), iconFolder, false) {
			a.openProjectFolder()
		}
		if a.ghostIconButton(c, a.t("actions.launch"), iconPlay, false) {
			a.launchMain()
		}
		if a.actionButtonLabel(c, a.t("toolbar.refresh"), a.t("detail.refreshTip"), iconRefresh, false, busy) {
			a.refreshDetail()
		}
		if a.actionButtonLabel(c, a.t("toolbar.edit"), a.t("detail.settingsTip"), iconSettings, false, d.busy) {
			a.openEdit()
		}
	})
}

// detailSplit puts the changes area left and the publish/history side right.
// d.splitW stores the side width (the C# GridSplitter column), while ui.Split
// wants the first pane's width, so the two are converted around the build.
func (a *app) detailSplit(c *ui.Context, d *detailState, root ui.Element) {
	total := root.Bounds().W - 2*detailPadding
	if total <= 0 {
		// First frame: the root has no layout yet, so estimate from the
		// window less the sidebar and the page padding.
		width, _ := c.Size()
		total = float32(width) - detailSidebarWidth - detailSplitDivider - 2*detailPadding
	}
	if total < detailMinSide+detailMinChanges+detailSplitDivider {
		// An unreasonably small window still gets a usable split; ui.Split
		// keeps the panes positive.
		total = detailMinSide + detailMinChanges + detailSplitDivider
	}
	d.splitTotal = total
	side := clampSideWidth(total, d.splitW)
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

// clampSideWidth keeps the side column between the C# GridSplitter's minimum
// and maximum and leaves the changes pane its own minimum room.
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
	ui.Column(c).Grow(1).Shrink(0).FillHeight().Gap(10).Children(func() {
		a.publishView(c, d)
		a.historyView(c, d)
	})
}

// changesView is the card holding the unstaged and staged file tables, with
// the first-load skeleton over it until the first refresh attempt ends.
func (a *app) changesView(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Background(t.Surface).Border(1, t.Border).
		Radius(10).Shadow(0, 1, 3, 0, cardShadow).Clip().Children(func() {
		ui.SplitVerticalPercent(c, &d.splitPercent,
			func() { a.filesPanel(c, d, false) },
			func() { a.filesPanel(c, d, true) },
		).Grow(1)
		if d.initialLoading {
			a.detailSkeleton(c)
		}
	})
}

// detailSkeleton is the C# first-load overlay: a translucent card with a
// spinner and the "reading project status" caption.
func (a *app) detailSkeleton(c *ui.Context) {
	t := c.Theme()
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
		Background(t.Background.Alpha(0.9)).Center().Children(func() {
		ui.Column(c).Gap(10).Center().Children(func() {
			ui.Spinner(c).Label(a.t("changes.reading"))
			ui.Text(c, a.t("changes.reading")).FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// filesPanel is one half of the changes card: header actions plus the file
// table, or the empty state when nothing is listed.
func (a *app) filesPanel(c *ui.Context, d *detailState, staged bool) {
	t := c.Theme()
	files := d.unstaged
	title := a.t("changes.unstaged")
	empty := a.t("changes.emptyUnstaged")
	emptyIcon := iconCheck
	headerIcon := iconFile
	iconColor := t.TextMuted
	if staged {
		files = d.staged
		title = a.t("changes.staged")
		empty = a.t("changes.emptyStaged")
		emptyIcon = iconInbox
		headerIcon = iconCheckSquare
		iconColor = t.Success
	}
	disabled := d.busy || d.uploading || len(files) == 0
	hasSelected := len(selectedFiles(files)) > 0
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Row(c).Wrap().Gap(6).Padding(10, 6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Icon(c, headerIcon).FontSize(14).TextColor(iconColor)
			ui.Text(c, title).FontSize(12).Bold()
			a.countPill(c, len(files))
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
				if a.dangerButton(c, a.t("changes.clear"), iconTrash, d.busy || d.uploading || len(files) == 0) {
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
			a.emptyState(c, emptyIcon, empty, a.t("changes.emptyHint"))
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("table.select"), Width: 62},
			{Title: a.t("table.file"), Width: 0, MinWidth: 120},
			{Title: a.t("table.status"), Width: 92},
			{Title: a.t("table.local"), Width: 82, Align: ui.End},
			{Title: a.t("table.server"), Width: 82, Align: ui.End},
			{Title: "", Width: 44},
		}
		ui.Table(c, nil, cols, len(files), func(row, col int) {
			f := &files[row]
			switch col {
			case 0:
				ui.Checkbox(c, &f.Selected, "").Label(a.t("table.select"))
			case 1:
				ui.Text(c, f.RelativePath).FontSize(11).SingleLine().Tooltip(f.RelativePath)
			case 2:
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					a.statusDot(c, statusColor(t, f.Status))
					ui.Text(c, statusDisplay(f.Status, a.lang)).FontSize(11).
						TextColor(statusColor(t, f.Status)).SingleLine()
				})
			case 3:
				ui.Text(c, formatSize(f.LocalSize)).FontSize(11).TextColor(t.TextMuted)
			case 4:
				ui.Text(c, formatSize(f.ServerSize)).FontSize(11).TextColor(t.TextMuted)
			case 5:
				if a.ghostIconButtonSize(c, a.t("table.open"), iconFolderOpen, false, 24, 24) {
					a.openFileLocation(*f)
				}
			}
		}).Grow(1)
	})
}

// dangerButton is the "clear staging" action: a secondary button whose text
// and icon read as destructive (the C# button's Danger style class).
func (a *app) dangerButton(c *ui.Context, label string, ic *ui.SVG, disabled bool) bool {
	t := c.Theme()
	b := ui.Button(c, "").Label(label).Tooltip(label).Disabled(disabled)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(14).TextColor(t.Danger)
		ui.Text(c, label).FontSize(12).TextColor(t.Danger).SingleLine()
	})
	return b.Clicked()
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
	return a.tf("publish.publishCount", "count", fmt.Sprint(len(d.staged)))
}

// publishView is the release form: version, notes, advanced options, the
// publish button with its hint, and the upload progress card.
func (a *app) publishView(c *ui.Context, d *detailState) {
	t := c.Theme()
	canPublish := !d.busy && !d.uploading && len(d.staged) > 0 &&
		strings.TrimSpace(d.newVersion) != "" && strings.TrimSpace(d.notes) != ""
	hint := a.publishHint(d)
	buttonText := a.publishButtonText(d)
	nextVersion := suggestVersion(d.currentVersion)
	ui.Column(c).Gap(8).Padding(14, 10).Background(t.Surface).Border(1, t.Border).
		Radius(10).Shadow(0, 1, 3, 0, cardShadow).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.t("publish.title")).FontSize(13).Bold()
			a.countPill(c, len(d.staged))
			ui.Box(c).Grow(1)
			if d.currentVersion != "" {
				a.versionPill(c, d.currentVersion)
			} else {
				ui.Text(c, a.t("history.none")).FontSize(10).TextColor(t.TextMuted).SingleLine()
			}
		})
		a.fieldLabel(c, a.t("publish.version"))
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &d.newVersion).Placeholder(a.t("publish.versionHint")).
				Label(a.t("publish.version")).Grow(1)
			if a.ghostIconButton(c, a.t("publish.nextVersion"), iconPlus, d.busy || d.uploading || nextVersion == "") {
				d.newVersion = nextVersion
			}
		})
		a.fieldLabel(c, a.t("publish.notes"))
		ui.TextArea(c, &d.notes).Placeholder(a.t("publish.notesHint")).
			Label(a.t("publish.notes")).MinHeight(80)
		ui.Collapsible(c, a.t("publish.advanced"), &d.advancedOpen, func() {
			ui.Column(c).Gap(8).Children(func() {
				a.fieldLabel(c, a.t("publish.script"))
				ui.TextInput(c, &d.script).Placeholder(a.t("publish.scriptHint")).
					Label(a.t("publish.script"))
				ui.Checkbox(c, &d.forceUpdate, a.t("publish.force"))
			})
		})
		tooltip := buttonText
		if hint != "" {
			tooltip = hint
		}
		pub := ui.PrimaryButton(c, "").Label(buttonText).Tooltip(tooltip).
			Disabled(!canPublish).Height(36).FillWidth()
		pub.Children(func() {
			ui.Icon(c, iconUpload).FontSize(15)
			ui.Text(c, buttonText).FontSize(13).Bold().SingleLine()
		})
		if pub.Clicked() {
			a.publish()
		}
		if hint != "" {
			ui.Row(c).Gap(5).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconInfo).FontSize(12).TextColor(t.Warning)
				ui.Text(c, hint).FontSize(11).TextColor(t.Warning).Wrap()
			})
		}
		if d.uploading || len(d.uploadItems) > 0 {
			a.uploadCard(c, d)
		}
	})
}

// uploadCard shows the streamed push progress: caption, bar, summary and the
// per-file rows (C# upload progress border).
func (a *app) uploadCard(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Gap(6).Padding(10, 8).Background(t.Background).Border(1, t.Border).
		Radius(6).Children(func() {
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
			ui.Text(c, d.uploadSummary).FontSize(10).TextColor(t.TextMuted).SingleLine()
		}
		items := d.uploadItems
		if len(items) > 0 {
			cols := []ui.TableColumn{
				{Title: a.t("table.file"), Width: 0, MinWidth: 90},
				{Title: a.t("table.local"), Width: 66, Align: ui.End},
				{Title: a.t("table.status"), Width: 104},
			}
			ui.Table(c, nil, cols, len(items), func(row, col int) {
				item := items[row]
				switch col {
				case 0:
					ui.Text(c, item.File).FontSize(10).SingleLine().Tooltip(item.File)
				case 1:
					ui.Text(c, formatSize(item.FileSize)).FontSize(10).TextColor(t.TextMuted)
				case 2:
					ui.Text(c, uploadStatusText(item, a.lang)).FontSize(10).
						TextColor(uploadStatusColor(t, item.Status)).SingleLine()
				}
			}).MaxHeight(150).Shrink(0)
		}
		if d.uploading {
			if a.actionButton(c, a.t("publish.cancel"), iconX, false, false) {
				a.cancelPublish()
			}
		}
	})
}

// historyView lists releases; the newest version pill uses the accent colour.
func (a *app) historyView(c *ui.Context, d *detailState) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Background(t.Surface).Border(1, t.Border).
		Radius(10).Shadow(0, 1, 3, 0, cardShadow).Clip().Children(func() {
		ui.Row(c).Gap(6).Padding(12, 8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Icon(c, iconHistory).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, a.t("history.title")).FontSize(13).Bold()
			a.countPill(c, len(d.logs))
			ui.Box(c).Grow(1)
			ui.Text(c, a.t("history.latest")).FontSize(10).TextColor(t.TextMuted).SingleLine()
			if a.ghostIconButtonSize(c, a.t("detail.refreshTip"), iconRefresh, d.busy || d.uploading, 24, 24) {
				a.refreshDetail()
			}
		})
		if len(d.logs) == 0 {
			a.emptyState(c, iconHistory, a.t("history.empty"), a.t("history.emptyHint"))
			return
		}
		cols := []ui.TableColumn{
			{Title: a.t("history.version"), Width: 92},
			{Title: a.t("history.time"), Width: 118},
			{Title: a.t("history.notes"), Width: 0, MinWidth: 80},
		}
		logs := d.logs
		ui.Table(c, nil, cols, len(logs), func(row, col int) {
			log := logs[row]
			switch col {
			case 0:
				pill := ui.Row(c).Gap(4).Padding(8, 2).Radius(999).AlignItems(ui.Center)
				if row == 0 {
					pill.Background(t.Accent)
					pill.Children(func() {
						ui.Text(c, log.Version).FontSize(10).Bold().TextColor(t.AccentText).SingleLine()
					})
				} else {
					pill.Background(t.Background).Border(1, t.Border)
					pill.Children(func() {
						ui.Text(c, log.Version).FontSize(10).TextColor(t.TextMuted).SingleLine()
					})
				}
			case 1:
				ui.Text(c, log.Time).FontSize(10).TextColor(t.TextMuted).SingleLine()
			case 2:
				ui.Column(c).Gap(2).Children(func() {
					for _, line := range log.Logs {
						ui.Text(c, line).FontSize(10).TextColor(t.TextMuted).Wrap()
					}
				})
			}
		}).Grow(1)
	})
}

func statusColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case "new":
		return t.Success
	case "modified":
		return t.Warning
	case "deleted":
		return t.Danger
	default:
		return t.TextMuted
	}
}

func uploadStatusText(p UploadProgress, lang string) string {
	switch p.Status {
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
		if p.Error != "" {
			return p.Error
		}
		if lang == "en" {
			return "Failed"
		}
		return "失败"
	}
	return p.Status
}

func uploadStatusColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case "DONE":
		return t.Success
	case "FAIL":
		return t.Danger
	default:
		return t.TextMuted
	}
}
