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

// textButton is a compact, left-aligned secondary action for dialogs.
func (a *app) textButton(c *ui.Context, label string, ic *ui.SVG, disabled bool) bool {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Disabled(disabled).Width(150).Height(28)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(13)
		ui.Text(c, label).FontSize(11).SingleLine()
	})
	return b.Clicked()
}

// formModal draws a form dialog over the window. The backdrop keeps the
// pointer above the window, so a press beside the form goes nowhere, while
// Escape still closes it.
//
// Never give the backdrop HandleInput: MyGo routes a press to the innermost
// element that handles input (ui/handler.go, engine.pointerDown), so a
// handler on this ancestor would take every press before the dialog's own
// buttons could, leaving the dialog's controls unclickable.
func (a *app) formModal(c *ui.Context, open *bool, build func()) {
	a.formModalEsc(c, open, nil, build)
}

// formModalEsc is formModal with a say in what Escape does: onEscape nil
// closes the dialog, and a wizard's sub-page sets it to go back one step.
func (a *app) formModalEsc(c *ui.Context, open *bool, onEscape func(), build func()) {
	if !*open {
		return
	}
	ui.Overlay(c, func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().
			Background(ui.RGBA(0, 0, 0, 0.4)).Modal()
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

var cardShadow = ui.RGBA(0, 0, 0, 0.06)

// ghostIconButton is the borderless variant for table rows, header utilities
// and chips, where a full button face on every item looks heavy.
func (a *app) ghostIconButton(c *ui.Context, label string, ic *ui.SVG, disabled bool) bool {
	return a.ghostIconButtonElement(c, label, ic, disabled).Clicked()
}

func (a *app) ghostIconButtonElement(c *ui.Context, label string, ic *ui.SVG, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Disabled(disabled).Width(28).Height(28)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(15)
	})
	return b
}

func (a *app) ghostIconButtonSize(c *ui.Context, label string, ic *ui.SVG, disabled bool, w, h float32) bool {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Disabled(disabled).Width(w).Height(h)
	size := float32(15)
	if w < 24 {
		size = 12
	}
	b.Children(func() {
		ui.Icon(c, ic).FontSize(size)
	})
	return b.Clicked()
}

func (a *app) toolbarSeparator(c *ui.Context) {
	ui.Box(c).Width(1).Height(20).Background(c.Theme().Border).Margin(2, 0)
}

func (a *app) countPill(c *ui.Context, n int) {
	ui.Badge(c, fmt.Sprint(n))
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
	// The pill sets its own text colors: a selected list row tints the text
	// of its children, which would make this pill's label vanish on its own
	// background.
	ui.Row(c).Gap(4).Padding(8, 3).Background(t.Background).Border(1, t.Border).
		Radius(999).AlignItems(ui.Center).TextColor(t.Text).Children(func() {
		ui.Icon(c, iconTag).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, version).FontSize(11).Bold().TextColor(t.Text)
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

// emptyStateActions is emptyState with a row of calls to action under it.
func (a *app) emptyStateActions(c *ui.Context, ic *ui.SVG, title, hint string, actions func()) {
	t := c.Theme()
	ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
		ui.Icon(c, ic).FontSize(30).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(13).TextColor(t.TextMuted)
		if hint != "" {
			ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).MaxWidth(340).Wrap()
		}
		actions()
	})
}

// dialogSize clamps a dialog's wanted size to the window, so a form still
// fits a small window and its content scrolls instead of overflowing.
func dialogSize(c *ui.Context, wantW, wantH float32) (w, h float32) {
	winW, winH := c.Size()
	return max(min(wantW, winW-48), 360), max(min(wantH, winH-48), 260)
}

// busyRow shows work of unknown length: a spinner and what it is.
func (a *app) busyRow(c *ui.Context, text string) {
	t := c.Theme()
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c).Label(text)
		ui.Text(c, text).FontSize(11).TextColor(t.TextMuted).SingleLine()
	})
}

// stepBadge numbers a wizard step; active marks the step the user is on.
func (a *app) stepBadge(c *ui.Context, n int, active bool) {
	t := c.Theme()
	background, border, text := t.Background, t.Border, t.TextMuted
	if active {
		background, border, text = t.Accent, t.Accent, t.AccentText
	}
	ui.Box(c).Size(24, 24).Radius(12).Background(background).Border(1, border).Center().Children(func() {
		ui.Text(c, fmt.Sprint(n)).FontSize(11).Bold().TextColor(text)
	})
}

// suggestionChips offers one-click values for a rule list, as the C# dialogs do.
func (a *app) suggestionChips(c *ui.Context, values []string, pick func(string)) {
	t := c.Theme()
	ui.Row(c).Wrap().Gap(4).Children(func() {
		for _, value := range values {
			value := value
			b := ui.ButtonBase(c).Label(value).Tooltip(value).Height(20).Padding(6, 0).
				Border(1, t.Border).Radius(4)
			b.Children(func() {
				ui.Text(c, value).FontSize(10).TextColor(t.TextMuted).SingleLine()
			})
			if b.Clicked() {
				pick(value)
			}
		}
	})
}

// noticeBox shows a hint, a warning or an error under a dialog's header.
func (a *app) noticeBox(c *ui.Context, ic *ui.SVG, text string, col ui.Color, wrap bool) {
	t := c.Theme()
	ui.Row(c).Gap(6).Padding(10, 8).Background(t.Surface).Border(1, col).Radius(6).
		AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(13).TextColor(col)
		label := ui.Text(c, text).FontSize(11).TextColor(col)
		if wrap {
			label.Grow(1).Wrap()
		}
	})
}

func (a *app) view(c *ui.Context) {
	a.applyAccent(c)
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.toolbar(c)
		if a.busyText != "" {
			ui.Progress(c, -1).Height(2).Shrink(0)
		}
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
		if a.addOpen && a.adding != nil {
			a.addProjectModal(c)
		}
		if a.localPreviewOpen && a.localPreview != nil {
			a.localPreviewModal(c)
		}
		if a.removeOpen {
			if a.detail == nil {
				// The dialog has no project to name; drop it instead of
				// showing a message with an empty name.
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
		if a.toastMsg != "" {
			msg := a.toastMsg
			a.toastMsg = ""
			c.Toast(msg)
		}
	})
	// Declared after the tree is built, so inputs/buttons claim their keys
	// first and these window-level shortcuts never steal typing.
	a.handleShortcuts(c)
}

// handleShortcuts mirrors the C# MainWindow KeyBindings: F5 refreshes,
// Ctrl+Enter publishes, Ctrl+E opens project settings. ui.Cmd is Control on
// Windows/Linux and Command on macOS. While a modal is open its dialog owns
// the keyboard, so the shortcuts stay inactive.
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
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	detailBusy := a.detail != nil && (a.detail.busy || a.detail.uploading)
	ui.Row(c).Height(56).Shrink(0).Gap(8).Padding(14, 10).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		if a.actionButton(c, a.t("toolbar.addServer"), iconCloud, true, false) {
			a.openAddProject()
		}
		if a.actionButton(c, a.t("toolbar.addLocal"), iconFolderPlus, false, false) {
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
		if a.ghostIconButton(c, a.tf("theme.tooltip", "theme", a.themeLabel()), a.themeIcon(), false) {
			a.cycleTheme()
		}
		palette := a.ghostIconButtonElement(c, a.t("accent.title"), iconPalette, false)
		if palette.Clicked() {
			a.accentOpen = !a.accentOpen
		}
		ui.PopoverBase(c, palette, &a.accentOpen, func(panel ui.Element) {
			panel.Width(280).Padding(12).Background(t.Surface).Border(1, t.Border).Radius(12).
				Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
			a.accentPanel(c)
		})
		a.toolbarSeparator(c)
		if a.ghostIconButton(c, a.t("toolbar.openCLI"), iconTerminal, false) {
			a.openCLIFolder()
		}
		if a.ghostIconButton(c, a.t("toolbar.openLog"), iconFileText, false) {
			a.openLogFolder()
		}
		a.toolbarSeparator(c)
		langLabel := "EN"
		if a.lang == "en" {
			langLabel = "中文"
		}
		if a.actionButtonLabel(c, langLabel, a.t("toolbar.languageTip"), iconGlobe, false, false) {
			a.toggleLang()
		}
	})
}

func (a *app) accentPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, a.t("accent.title")).FontSize(11).Bold().TextColor(t.TextMuted)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, opt := range accentPalette {
				selected := normalizeAccent(a.accent) == opt.ID
				label := a.t(opt.Label)
				sw := ui.ButtonBase(c).Label(label).Tooltip(label).Width(28).Height(28).
					Radius(14).Background(ui.Hex(opt.Hex))
				if selected {
					sw.Border(2, t.Text)
				} else {
					sw.Border(1, t.Border)
				}
				if selected {
					sw.Children(func() {
						ui.Icon(c, iconCheck).FontSize(12).TextColor(ui.Hex("#ffffff"))
					})
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
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(14, 8).Shrink(0).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Warning).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconWarning).FontSize(15).TextColor(t.Warning)
		ui.Text(c, a.t("cli.banner")).FontSize(11).TextColor(t.Warning).Grow(1).Wrap()
		if a.actionButton(c, a.t("toolbar.retry"), iconRefresh, false, false) {
			a.cli = &cliClient{exe: findCLI()}
			if a.cli.available() {
				a.status = a.tf("cli.found", "path", a.cli.exe)
				a.refreshDetail()
			}
		}
		if a.actionButton(c, a.t("cli.openFolder"), iconFolder, false, false) {
			a.openCLIFolder()
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
	if len(list) == 0 && a.detail != nil {
		// A filter with no matches clears the selection, as the C# view's
		// ApplyProjectFilter leaves no SelectedTab. The tab state stays
		// cached in a.details for when a project matches again.
		a.detail = nil
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
			a.emptyStateActions(c, iconInbox, a.t("projects.noMatch"), "", func() {
				ui.Row(c).Wrap().Gap(6).Justify(ui.Center).Children(func() {
					if a.actionButton(c, a.t("projects.clearSearch"), iconX, false, false) {
						a.filter = ""
						a.selected = 0
						a.openSelected()
					}
				})
			})
		case len(list) == 0:
			a.emptyStateActions(c, iconFolderPlus, a.t("projects.emptyTitle"), a.t("projects.emptyHint"), func() {
				ui.Row(c).Wrap().Gap(6).Justify(ui.Center).Children(func() {
					if a.actionButton(c, a.t("toolbar.addServer"), iconCloud, true, false) {
						a.openAddProject()
					}
					if a.actionButton(c, a.t("toolbar.addLocal"), iconFolderPlus, false, false) {
						a.addLocalProject()
					}
				})
			})
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
	ui.Row(c).Height(40).Shrink(0).Gap(6).Padding(14, 0).Background(t.Surface).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		if a.busyText != "" {
			ui.Spinner(c).Label(a.busyText)
			ui.Text(c, a.busyText).FontSize(11).TextColor(t.TextMuted).SingleLine().Grow(1)
		} else {
			a.statusDot(c, col)
			ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).SingleLine().Grow(1)
		}
		if a.detail != nil {
			ui.Text(c, a.detail.project.ProjectPath).FontSize(10).TextColor(t.TextMuted).
				SingleLine().Selectable().MaxWidth(420).Tooltip(a.detail.project.ProjectPath)
		}
		if a.ghostIconButton(c, a.t("status.logs"), iconFileText, false) {
			a.openLogFolder()
		}
	})
}

func (a *app) editGroup(c *ui.Context, label string, list *[]string, input *string, pick func()) {
	t := c.Theme()
	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(11).Bold().TextColor(t.TextMuted)
			a.countPill(c, len(*list))
		})
		items := append([]string(nil), (*list)...)
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for _, value := range items {
				value := value
				ui.Row(c).Gap(1).AlignItems(ui.Center).Height(18).Padding(5, 0).Background(t.Text.Alpha(0.06)).
					Border(1, t.Border).Radius(4).MaxWidth(280).Children(func() {
					ui.Text(c, value).FontSize(10).SingleLine().Tooltip(value)
					if a.ghostIconButtonSize(c, a.t("edit.remove"), iconX, false, 14, 14) {
						*list = removeValue(*list, value)
					}
				})
			}
		})
		ui.Row(c).Gap(5).Children(func() {
			if ui.TextInput(c, input).Placeholder(label).Grow(1).Submitted() {
				if v := strings.TrimSpace(*input); v != "" {
					*list = appendUnique(*list, v)
					*input = ""
				}
			}
			if a.actionButton(c, a.t("edit.add"), iconPlus, false, false) {
				if v := strings.TrimSpace(*input); v != "" {
					*list = appendUnique(*list, v)
					*input = ""
				}
			}
			if a.actionButton(c, a.t("edit.pick"), iconFolder, false, false) {
				pick()
			}
		})
	})
}
