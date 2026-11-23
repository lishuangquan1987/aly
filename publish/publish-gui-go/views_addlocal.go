package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// localPreviewModal is the add-local-project dialog. Unlike the old flow it
// opens before a folder is chosen: the folder field is the first thing the
// user touches, and the config found under it is previewed live, as the C#
// AddLocalProjectDialog does.
func (a *app) localPreviewModal(c *ui.Context) {
	p := a.localPreview
	if p == nil {
		return
	}
	t := c.Theme()
	w, h := dialogSize(c, 620, 560)
	a.formModal(c, &a.localPreviewOpen, func() {
		ui.Column(c).Role(ui.RoleDialog).Width(w).MaxHeight(h).Gap(10).Padding(18).
			Background(t.Surface).Radius(12).Border(1, t.Border).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
			a.localPreviewHeader(c, p)
			a.localPreviewFolderField(c, p)
			a.localPreviewStatus(c, p)
			if p.hasPreview() {
				a.localPreviewCard(c, p)
			}
			ui.Field(c, a.t("edit.displayName"), func() {
				ui.TextInput(c, &p.displayName).Placeholder(a.t("addProject.displayNameHint")).
					Grow(1).Disabled(p.busy)
			})
			if p.error != "" {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconWarning).FontSize(12).TextColor(t.Danger)
					ui.Text(c, p.error).FontSize(11).TextColor(t.Danger).Wrap().Grow(1)
				})
			}
			if p.busy {
				a.busyRow(c, a.t("addLocal.adding"))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if a.actionButton(c, a.t("edit.cancel"), iconX, false, p.busy) {
					a.closeLocalPreview()
				}
				if a.actionButton(c, a.t("addLocal.confirm"), iconCheck, true, !a.localPreviewReady(p)) {
					a.confirmLocalProject()
				}
			})
		})
	})
}

func (a *app) localPreviewHeader(c *ui.Context, p *localPreviewState) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconFolderPlus).FontSize(18).TextColor(t.Accent)
		ui.Text(c, a.t("addLocal.previewTitle")).FontSize(16).Bold()
		ui.Box(c).Grow(1)
		if a.ghostIconButton(c, a.t("edit.cancel"), iconX, p.busy) {
			a.closeLocalPreview()
		}
	})
}

// localPreviewFolderField is the folder text field with its Browse action. The
// field itself accepts a dropped folder, as the C# view's drop handler did.
func (a *app) localPreviewFolderField(c *ui.Context, p *localPreviewState) {
	t := c.Theme()
	field := ui.Field(c, a.t("addProject.path"), func() {
		ui.Row(c).Gap(6).Children(func() {
			in := ui.TextInput(c, &p.path).Placeholder(a.t("addLocal.folderPlaceholder")).
				Grow(1).AutoFocus().Disabled(p.busy)
			if in.Changed() {
				a.refreshLocalPreview(p)
			}
			if a.actionButton(c, a.t("edit.browse"), iconFolder, false, p.busy) {
				a.pickFolder(false, a.t("addLocal.title"), func(paths []string) {
					if len(paths) == 0 {
						return
					}
					p.path = paths[0]
					a.refreshLocalPreview(p)
				})
			}
		})
	})
	if dropped := field.DroppedFiles(); len(dropped) > 0 {
		path := dropped[0]
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// A file was dropped: its folder is what the dialog wants.
			path = filepath.Dir(path)
		}
		p.path = path
		a.refreshLocalPreview(p)
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconFolderOpen).FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, a.t("addLocal.dropHint")).FontSize(10).TextColor(t.TextMuted)
	})
}

// localPreviewStatus shows the live verdict of the folder in the same box the
// C# dialog gives its InitStatusMessage.
func (a *app) localPreviewStatus(c *ui.Context, p *localPreviewState) {
	t := c.Theme()
	text, ready := a.localPreviewStateText(p)
	ic, col := iconInfo, t.TextMuted
	switch {
	case ready:
		ic, col = iconCheck, t.Success
	case normalizeForCompare(p.path) == "":
		ic, col = iconInfo, t.TextMuted
	default:
		ic, col = iconWarning, t.Danger
	}
	a.noticeBox(c, ic, text, col, true)
}

// localPreviewCard previews what shared.json holds, so the user confirms the
// project they meant before it enters the sidebar.
func (a *app) localPreviewCard(c *ui.Context, p *localPreviewState) {
	t := c.Theme()
	folders := len(p.shared.IgnoreFolders) + len(p.shared.UnCopyFolders)
	files := len(p.shared.IgnoreFiles) + len(p.shared.UnCopyFiles)
	ui.Column(c).Gap(6).Padding(12, 10).Background(t.Background).
		Border(1, t.Border).Radius(8).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			a.fieldLabelBox(c, a.t("addLocal.server"))
			ui.Text(c, p.shared.ServerURL).FontSize(12).Grow(1).SingleLine()
		})
		ui.Row(c).Gap(8).Children(func() {
			a.fieldLabelBox(c, a.t("addLocal.project"))
			ui.Text(c, p.shared.ProjectName).FontSize(12).Grow(1).SingleLine()
		})
		ui.Row(c).Gap(8).Children(func() {
			a.fieldLabelBox(c, a.t("addLocal.ignored"))
			ui.Text(c, a.tf("addLocal.previewCount",
				"folders", fmt.Sprint(folders),
				"files", fmt.Sprint(files))).
				FontSize(12).TextColor(t.TextMuted).Grow(1)
		})
	})
}
