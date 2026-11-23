package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// localPreviewModal is the add-local-project dialog. It opens before a folder
// is chosen: the folder field is the first thing the user touches, and the
// config found under it is previewed live.
func (a *app) localPreviewModal(c *ui.Context) {
	p := a.localPreview
	if p == nil {
		return
	}
	w, h := dialogSize(c, 640, 480)
	a.formModal(c, &a.localPreviewOpen, func() {
		a.dialogShell(c, iconFolderPlus, a.t("addLocal.previewTitle"), a.t("addLocal.subtitle"),
			w, h, p.busy, func() {
				if !p.busy {
					a.closeLocalPreview()
				}
			}, func() {
				changed := false
				a.field(c, a.t("addProject.path"), func() {
					row := ui.Row(c).Gap(6)
					row.Children(func() {
						if ui.TextInput(c, &p.path).Placeholder(a.t("addLocal.folderPlaceholder")).
							Label(a.t("addProject.path")).Grow(1).Height(controlHeight).
							Radius(8).Disabled(p.busy).Changed() {
							changed = true
						}
						if a.buttonSized(c, a.t("edit.browse"), a.t("edit.browse"), iconFolder,
							btnSecondary, p.busy, controlHeight) {
							a.pickFolder(false, a.t("addLocal.title"), func(paths []string) {
								if len(paths) == 0 {
									return
								}
								p.path = paths[0]
								a.refreshLocalPreview(p)
							})
						}
					})
					if dropped := row.DroppedFiles(); len(dropped) > 0 {
						path := dropped[0]
						if info, err := os.Stat(path); err == nil && !info.IsDir() {
							path = filepath.Dir(path)
						}
						p.path = path
						changed = true
					}
				}).Description(a.t("addLocal.dropHint"))
				if changed {
					a.refreshLocalPreview(p)
				}
				status, tone := a.localPreviewStatus(p)
				a.notice(c, previewStatusIcon(tone), status, tone)
				if p.hasPreview() {
					a.localPreviewCard(c, p)
				}
				a.field(c, a.t("edit.displayName"), func() {
					ui.TextInput(c, &p.displayName).Placeholder(a.t("addProject.displayNameHint")).
						Height(controlHeight).Radius(8).Disabled(p.busy)
				})
				if p.error != "" {
					a.inlineNotice(c, iconErrorCircle, p.error, toneDanger)
				}
				if p.busy {
					a.busyRow(c, a.t("addLocal.adding"))
				}
			}, func() {
				ui.Box(c).Grow(1)
				if a.buttonSized(c, a.t("edit.cancel"), a.t("edit.cancel"), iconX, btnSecondary, p.busy, controlHeight) {
					a.closeLocalPreview()
				}
				if a.buttonSized(c, a.t("addLocal.confirm"), a.t("addLocal.confirm"), iconCheck,
					btnPrimary, !a.localPreviewReady(p), controlHeight) {
					a.confirmLocalProject()
				}
			})
	})
}

// previewStatusIcon is the icon a preview state wears: a problem, or the
// neutral hint of an empty field.
func previewStatusIcon(t tone) *ui.SVG {
	switch t {
	case toneDanger:
		return iconWarning
	case toneSuccess:
		return iconCheckCircle
	default:
		return iconInfoCircle
	}
}

// localPreviewCard previews what shared.json holds, so the user confirms the
// project they meant before it enters the sidebar.
func (a *app) localPreviewCard(c *ui.Context, p *localPreviewState) {
	pal := a.pal(c)
	folderRules := len(p.shared.IgnoreFolders) + len(p.shared.UnCopyFolders)
	fileRules := len(p.shared.IgnoreFiles) + len(p.shared.UnCopyFiles)
	ui.Column(c).Gap(8).Padding(12).Background(pal.fill).Border(1, pal.border).
		Radius(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			a.iconTile(c, iconFileText, toneNeutral, 22)
			a.sectionTitle(c, a.t("addLocal.configTitle"))
		})
		a.kv(c, a.t("addLocal.server"), p.shared.ServerURL)
		a.kv(c, a.t("addLocal.project"), p.shared.ProjectName)
		a.kv(c, a.t("addLocal.ignored"), a.tf("addLocal.previewCount",
			"folders", fmt.Sprint(folderRules),
			"files", fmt.Sprint(fileRules)))
	})
}
