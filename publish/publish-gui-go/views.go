package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// formModal draws a form dialog over the window: a scrim that keeps the
// pointer above the window, so a press beside the panel goes nowhere, while
// Escape closes it.
//
// Never give the scrim HandleInput: MyGo routes a press to the innermost
// element that handles input (ui/handler.go, engine.pointerDown), so a handler
// on this ancestor would take every press before the dialog's own buttons
// could, leaving the dialog's controls unclickable.
func (a *app) formModal(c *ui.Context, open *bool, build func()) {
	a.formModalEsc(c, open, nil, build)
}

// formModalEsc is formModal with a say in what Escape does: onEscape nil
// closes the dialog, and a wizard's sub-page sets it to go back one step.
func (a *app) formModalEsc(c *ui.Context, open *bool, onEscape func(), build func()) {
	if !*open {
		return
	}
	p := a.pal(c)
	scrim := ui.RGBA(9, 11, 17, 0.42)
	if p.dark {
		scrim = ui.RGBA(0, 0, 0, 0.62)
	}
	ui.Overlay(c, func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().
			Background(scrim).Modal().FocusRing(false)
		if backdrop.OverlayShortcut(0, ui.KeyEscape) {
			if onEscape != nil {
				onEscape()
			} else {
				*open = false
			}
			return
		}
		backdrop.Children(build)
	})
}

// dialogSize clamps a dialog's wanted size to the window, so a form still
// fits a small window and its content scrolls instead of overflowing.
func dialogSize(c *ui.Context, wantW, wantH float32) (w, h float32) {
	winW, winH := c.Size()
	return max(min(wantW, winW-56), 360), max(min(wantH, winH-56), 280)
}

// versionTag shows a version as a neutral tag, or the "no version" caption.
func (a *app) versionTag(c *ui.Context, version string) {
	if strings.TrimSpace(version) == "" {
		a.tag(c, a.t("history.none"), toneNeutral)
		return
	}
	a.tagIcon(c, version, iconTag, toneNeutral)
}

func (a *app) view(c *ui.Context) {
	a.applyTokens(c)
	p := a.pal(c)
	ui.Column(c).Fill().Background(p.bg).Children(func() {
		a.toolbar(c)
		if a.busyText != "" {
			ui.Progress(c, -1).Height(2).Shrink(0)
		}
		if !a.cli.available() {
			a.cliBanner(c)
		}
		ui.Row(c).Grow(1).Shrink(1).FillWidth().Children(func() {
			a.sidebar(c)
			a.content(c)
		})
		a.statusBar(c)
		if a.editOpen && a.editing != nil {
			a.editModal(c)
		}
		if a.addOpen && a.adding != nil {
			a.addProjectModal(c)
		}
		if a.localPreviewOpen && a.localPreview != nil {
			a.localPreviewModal(c)
		}
		if a.removeOpen {
			if a.detail == nil {
				a.removeOpen = false
			} else {
				switch ui.AlertDialog(c, &a.removeOpen,
					a.t("remove.title"),
					a.tf("remove.message", "name", a.detail.project.DisplayName),
					a.t("common.cancel"),
					a.t("common.remove"),
				) {
				case 1:
					a.removeSelected()
				}
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
		// One-shot messages set by the actions become queued toasts, drawn in
		// the window's lower right by toastOverlay.
		if a.toastMsg != "" {
			a.notify(a.toastKind, a.toastMsg)
			a.toastMsg, a.toastKind = "", toastInfo
		}
		a.toastOverlay(c)
	})
	// Declared after the tree is built, so inputs/buttons claim their keys
	// first and these window-level shortcuts never steal typing.
	a.handleShortcuts(c)
}

// handleShortcuts mirrors the C# MainWindow KeyBindings: F5 refreshes,
// Ctrl+Enter publishes, Ctrl+E opens project settings, and Ctrl+F puts the
// caret in the project search. While a modal is open its dialog owns the
// keyboard, so the shortcuts stay inactive.
func (a *app) handleShortcuts(c *ui.Context) {
	if a.editOpen || a.addOpen || a.localPreviewOpen || a.removeOpen || a.clearOpen {
		return
	}
	if c.Shortcut(0, ui.KeyF5) {
		a.refreshDetail()
	}
	if c.Shortcut(ui.Cmd, ui.KeyEnter) {
		a.publish()
	}
	if c.Shortcut(ui.Cmd, ui.KeyE) {
		a.openEdit()
	}
	if c.Shortcut(ui.Cmd, ui.KeyF) {
		a.focusSearch = true
	}
}

func (a *app) toolbarDivider(c *ui.Context) {
	p := a.pal(c)
	ui.Box(c).Width(1).Height(22).Background(p.border)
}

func (a *app) toolbar(c *ui.Context) {
	p := a.pal(c)
	ui.Row(c).Height(toolbarHeight).Shrink(0).FillWidth().Gap(8).PaddingX(14).
		Background(p.bgAlt).BorderWidth(0, 0, 1, 0).BorderColor(p.border).
		AlignItems(ui.Center).Children(func() {
		a.brandTile(c, 28)
		ui.Text(c, a.t("app.title")).FontSize(14).FontWeight(650).TextColor(p.text).SingleLine()
		ui.Box(c).Width(10)
		a.toolbarDivider(c)
		ui.Box(c).Width(4)
		if a.button(c, a.t("toolbar.addServer"), iconCloud, btnPrimary, false) {
			a.openAddProject()
		}
		if a.button(c, a.t("toolbar.addLocal"), iconFolderPlus, btnSecondary, false) {
			a.addLocalProject()
		}
		ui.Box(c).Width(2)
		a.toolbarDivider(c)
		ui.Box(c).Width(2)
		if a.iconButton(c, a.t("toolbar.edit"), iconSettings, btnGhost, a.detail == nil) {
			a.openEdit()
		}
		if a.iconButton(c, a.t("toolbar.remove"), iconTrash, btnDangerGhost, a.detail == nil) {
			a.removeOpen = true
		}
		ui.Box(c).Grow(1)
		if a.iconButtonSize(c, a.tf("theme.tooltip", "theme", a.themeLabel()), a.themeIcon(),
			btnGhost, false, controlHeightS) {
			a.cycleTheme()
		}
		swatch := a.accentSwatchElement(c, a.accentOpen)
		if swatch.Clicked() {
			a.accentOpen = !a.accentOpen
		}
		ui.PopoverBase(c, swatch, &a.accentOpen, func(panel ui.Element) {
			panel.Width(300).Padding(14).Background(p.card).Border(1, p.border).
				Radius(12).Shadow(0, 12, 30, 0, p.shadowStrong)
			a.accentPanel(c)
		})
		if a.iconButtonSize(c, a.t("toolbar.openCLI"), iconTerminal, btnGhost, false, controlHeightS) {
			a.openCLIFolder()
		}
		if a.iconButtonSize(c, a.t("toolbar.openLog"), iconFileText, btnGhost, false, controlHeightS) {
			a.openLogFolder()
		}
		a.toolbarDivider(c)
		langLabel := "EN"
		if a.lang == "en" {
			langLabel = "中文"
		}
		if a.langButton(c, langLabel, a.t("toolbar.languageTip")) {
			a.toggleLang()
		}
	})
}

// langButton is the compact language switch: a globe and the other language.
func (a *app) langButton(c *ui.Context, label, tip string) bool {
	p := a.pal(c)
	b := ui.ButtonBase(c).Label(tip).Tooltip(tip).Height(controlHeightS).
		PaddingX(9).Gap(5).Radius(8)
	bg, fg, _ := a.buttonFaces(p, btnGhost, b.Hovered(), b.Pressed())
	b.Background(bg).TextColor(fg)
	b.Children(func() {
		ui.Icon(c, iconGlobe).FontSize(14).TextColor(fg)
		ui.Text(c, label).FontSize(fontCaption).FontWeight(500).TextColor(fg).SingleLine()
	})
	return b.Clicked()
}

// accentSwatchElement is the toolbar button that opens the accent palette: a
// small disc of the accent colour inside the usual icon button.
func (a *app) accentSwatchElement(c *ui.Context, open bool) ui.Element {
	p := a.pal(c)
	b := ui.ButtonBase(c).Label(a.t("accent.title")).Tooltip(a.t("accent.title")).
		Size(controlHeightS, controlHeightS).Radius(8)
	bg, fg, border := a.buttonFaces(p, btnGhost, b.Hovered() || open, b.Pressed())
	if open {
		bg, border = p.primarySoft, p.primaryBorder
	}
	b.Background(bg).Border(1, border)
	b.Children(func() {
		ring := ui.Box(c).Size(14, 14).Radius(7).Background(p.primary)
		if open {
			ring.Border(2, p.card)
		}
		_ = fg
	})
	return b
}

func (a *app) accentPanel(c *ui.Context) {
	p := a.pal(c)
	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, a.t("accent.title")).FontSize(fontCaption).FontWeight(600).TextColor(p.textSecond)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, opt := range accentPalette {
				selected := normalizeAccent(a.accent) == opt.ID
				label := a.t(opt.Label)
				sw := ui.ButtonBase(c).Label(label).Tooltip(label).Size(26, 26).
					Radius(13).Background(ui.Hex(opt.Hex))
				if selected {
					sw.Border(2, p.text)
					sw.Children(func() {
						ui.Icon(c, iconCheck).FontSize(12).TextColor(ui.Hex("#ffffff"))
					})
				} else if sw.Hovered() {
					sw.Border(2, ui.Hex(opt.Hex).MixOklab(p.text, 0.25))
				}
				if sw.Clicked() {
					a.accent = opt.ID
					a.savePrefs()
					a.post(func() {})
				}
			}
		})
	})
}

func (a *app) cliBanner(c *ui.Context) {
	p := a.pal(c)
	bg, fg, border := toneColors(p, toneWarning)
	ui.Row(c).Gap(10).Padding(9, 14).Shrink(0).FillWidth().Background(bg).
		BorderWidth(0, 0, 1, 0).BorderColor(border).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconWarning).FontSize(15).TextColor(fg)
		ui.Text(c, a.t("cli.banner")).FontSize(fontLabel).TextColor(fg).Grow(1).Wrap()
		if a.buttonSized(c, a.t("toolbar.retry"), a.t("toolbar.retry"), iconRefresh, btnSecondary, false, controlHeightS) {
			a.cli = &cliClient{exe: findCLI()}
			if a.cli.available() {
				a.status = a.tf("cli.found", "path", a.cli.exe)
				a.statusTone = toneSuccess
				a.refreshDetail()
			}
		}
		if a.buttonSized(c, a.t("cli.openFolder"), a.t("cli.openFolder"), iconFolder, btnGhost, false, controlHeightS) {
			a.openCLIFolder()
		}
	})
}

func (a *app) sidebar(c *ui.Context) {
	p := a.pal(c)
	list := a.filteredProjects()
	if a.selected >= len(list) {
		a.selected = len(list) - 1
	}
	if a.selected < 0 && len(list) > 0 {
		a.selected = 0
	}
	if len(list) == 0 && a.detail != nil {
		// A filter with no matches clears the selection, as the C# view's
		// ApplyProjectFilter leaves no SelectedTab. The tab state stays
		// cached in a.details for when a project matches again.
		a.detail = nil
	}
	a.list.Selected = nil
	ui.Column(c).Width(detailSidebarWidth).Shrink(0).FillHeight().
		Background(p.bgAlt).BorderWidth(0, 1, 0, 0).BorderColor(p.border).
		Children(func() {
			ui.Row(c).Height(48).PaddingX(16).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, a.t("projects.title")).FontSize(fontSection).FontWeight(600).TextColor(p.text)
				a.countTag(c, len(a.projects), toneNeutral)
				ui.Box(c).Grow(1)
				if a.iconButtonSize(c, a.t("toolbar.addServer"), iconPlus, btnGhost, false, 26) {
					a.openAddProject()
				}
			})
			ui.Box(c).Padding(2, 12).Children(func() {
				field := ui.SearchField(c, &a.filter).
					Placeholder(a.t("projects.search")).Label(a.t("projects.search")).
					Height(controlHeight).Radius(9)
				if a.focusSearch {
					field.Focus()
					a.focusSearch = false
				}
				if field.Changed() {
					a.selected = 0
					a.openSelected()
				}
			})
			switch {
			case len(a.projects) > 0 && len(list) == 0 && strings.TrimSpace(a.filter) != "":
				a.emptyState(c, iconInbox, a.t("projects.noMatch"), "", func() {
					if a.buttonSized(c, a.t("projects.clearSearch"), a.t("projects.clearSearch"), iconX, btnSecondary, false, controlHeightS) {
						a.filter = ""
						a.selected = 0
						a.openSelected()
					}
				})
			case len(list) == 0:
				a.emptyState(c, iconFolderPlus, a.t("projects.emptyTitle"), a.t("projects.emptyHint"), func() {
					ui.Row(c).Wrap().Gap(6).Justify(ui.Center).Children(func() {
						if a.buttonSized(c, a.t("toolbar.addServer"), a.t("toolbar.addServer"), iconCloud, btnPrimary, false, controlHeightS) {
							a.openAddProject()
						}
						if a.buttonSized(c, a.t("toolbar.addLocal"), a.t("toolbar.addLocal"), iconFolderPlus, btnSecondary, false, controlHeightS) {
							a.addLocalProject()
						}
					})
				})
			default:
				a.list.Key = func(i int) any { return list[i].ProjectPath }
				ui.List(c, &a.list, len(list), func(i int) {
					a.projectRow(c, list[i], i == a.selected, i)
				}).Grow(1)
			}
			a.divider(c)
			ui.Column(c).Children(func() {
				ui.Row(c).Padding(6, 14).AlignItems(ui.Center).Gap(6).Children(func() {
					dot := p.success
					label := a.t("sidebar.cliReady")
					if !a.cli.available() {
						dot, label = p.danger, a.t("sidebar.cliMissing")
					}
					ui.Box(c).Size(6, 6).Radius(3).Background(dot)
					ui.Text(c, label).FontSize(fontMicro).TextColor(p.textThird).SingleLine()
					ui.Box(c).Grow(1)
					ui.Text(c, a.tf("sidebar.projectCount", "count", fmt.Sprint(len(a.projects)))).
						FontSize(fontMicro).TextColor(p.textThird).SingleLine()
				})
				ui.Row(c).Padding(6, 14).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Icon(c, iconArrowRight).FontSize(11).TextColor(p.textThird)
					ui.Text(c, a.t("projects.searchKey")).FontSize(fontMicro).
						TextColor(p.textThird).SingleLine()
				})
			})
		})
}

// projectRow is one project in the sidebar: an inset card with the project's
// name and folder path. The chosen row takes the accent tint and a hairline,
// as a navigation list on the web does, instead of a full accent fill.
func (a *app) projectRow(c *ui.Context, project ProjectConfig, selected bool, index int) {
	p := a.pal(c)
	b := ui.ButtonBase(c).Label(project.DisplayName).Tooltip(project.ProjectPath).
		Margin(2, 10).Padding(6, 8).Gap(10).Radius(10)
	bg, border := ui.Transparent, ui.Color{}
	name, path := p.text, p.textThird
	iconBg, iconFg := p.fill, p.textSecond
	switch {
	case selected:
		bg, border = p.primarySoft, p.primaryBorder
		name, path = p.primaryText, p.textSecond
		iconBg, iconFg = p.primaryBorder, p.primaryText
	case b.Hovered():
		bg = p.fillHover
	}
	b.Background(bg)
	if border.A > 0 {
		b.Border(1, border)
	}
	b.Children(func() {
		ui.Box(c).Size(30, 30).Radius(9).Background(iconBg).Center().Children(func() {
			ui.Icon(c, iconFolderOpen).FontSize(15).TextColor(iconFg)
		})
		ui.Column(c).Grow(1).Shrink(1).Gap(1).Children(func() {
			ui.Text(c, project.DisplayName).FontSize(fontBody).FontWeight(550).
				TextColor(name).SingleLine()
			ui.Text(c, project.ProjectPath).FontSize(fontMicro).TextColor(path).SingleLine()
		})
	})
	if b.Clicked() {
		a.selected = index
		a.openSelected()
	}
}

func (a *app) content(c *ui.Context) {
	ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
		if a.detail == nil {
			p := a.pal(c)
			_ = p
			a.emptyState(c, iconFolder, a.t("content.emptyTitle"), a.t("content.emptyHint"), func() {
				ui.Row(c).Wrap().Gap(6).Justify(ui.Center).Children(func() {
					if a.buttonSized(c, a.t("toolbar.addServer"), a.t("toolbar.addServer"),
						iconCloud, btnPrimary, false, controlHeightS) {
						a.openAddProject()
					}
					if a.buttonSized(c, a.t("toolbar.addLocal"), a.t("toolbar.addLocal"),
						iconFolderPlus, btnSecondary, false, controlHeightS) {
						a.addLocalProject()
					}
				})
			})
			return
		}
		a.detailView(c, a.detail)
	})
}

func (a *app) statusBar(c *ui.Context) {
	p := a.pal(c)
	status := a.status
	if a.detail != nil && a.detail.status != "" {
		status = a.detail.status
	}
	col := p.success
	switch a.statusTone {
	case toneWarning:
		col = p.warning
	case toneDanger:
		col = p.danger
	}
	if !a.cli.available() {
		col = p.danger
	}
	ui.Row(c).Height(statusHeight).Shrink(0).FillWidth().Gap(8).PaddingX(14).
		Background(p.bgAlt).BorderWidth(1, 0, 0, 0).BorderColor(p.border).
		AlignItems(ui.Center).Children(func() {
		if a.busyText != "" {
			ui.Spinner(c).Label(a.busyText)
			ui.Text(c, a.busyText).FontSize(fontCaption).TextColor(p.textSecond).SingleLine().Grow(1)
		} else {
			ui.Box(c).Size(7, 7).Radius(4).Background(col)
			ui.Text(c, status).FontSize(fontCaption).TextColor(p.textSecond).SingleLine().Grow(1)
		}
		if a.detail != nil {
			ui.Text(c, a.detail.project.ProjectPath).FontSize(fontMicro).TextColor(p.textThird).
				SingleLine().Selectable().MaxWidth(420).Tooltip(a.detail.project.ProjectPath)
		}
		if a.iconButtonSize(c, a.t("status.logs"), iconFileText, btnGhost, false, 26) {
			a.openLogFolder()
		}
	})
}

// editGroup edits one rule list: chips, an input with Add, and a folder/file
// picker. Shared by the project settings dialog and the wizard's rules page.
func (a *app) editGroup(c *ui.Context, label string, list *[]string, input *string, disabled bool, pick func()) {
	p := a.pal(c)
	add := func() {
		if v := strings.TrimSpace(*input); v != "" {
			*list = appendUnique(*list, v)
			*input = ""
		}
	}
	ui.Column(c).Gap(7).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			a.fieldLabel(c, label)
			a.countTag(c, len(*list), toneNeutral)
		})
		items := append([]string(nil), (*list)...)
		if len(items) > 0 {
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, value := range items {
					value := value
					if a.closeTag(c, value, toneNeutral, a.t("edit.remove"), disabled) {
						*list = removeValue(*list, value)
					}
				}
			})
		}
		ui.Row(c).Gap(6).Children(func() {
			in := ui.TextInput(c, input).Placeholder(label).Grow(1).
				Height(controlHeightS).Radius(8).Disabled(disabled)
			if in.Submitted() {
				add()
			}
			if a.buttonSized(c, a.t("edit.add"), a.t("edit.add"), iconPlus, btnSecondary, disabled, controlHeightS) {
				add()
			}
			if a.iconButtonSize(c, a.t("edit.pick"), iconFolder, btnGhost, disabled, controlHeightS) {
				pick()
			}
		})
		_ = p
	})
}

// stepBadge numbers a wizard step; active marks the step the user is on.
func (a *app) stepBadge(c *ui.Context, n int, active bool) {
	p := a.pal(c)
	bg, border, fg := p.card, p.border, p.textThird
	if active {
		bg, border, fg = p.primary, p.primaryBorder, p.onPrimary
	}
	ui.Box(c).Size(26, 26).Radius(9).Background(bg).Border(1, border).Center().Children(func() {
		ui.Text(c, fmt.Sprint(n)).FontSize(fontLabel).FontWeight(600).TextColor(fg)
	})
}
