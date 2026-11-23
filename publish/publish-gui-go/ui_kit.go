package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// This file is the app's small design system: buttons, tags, cards, dialog
// chrome and the typography helpers every view builds with. Views stay
// readable by naming intent (a primary button, a warning tag) instead of
// colour literals, and every control gains hover, pressed and disabled states
// from the palette in theme.go.

// ---------------------------------------------------------------- buttons --

type buttonKind int

const (
	btnPrimary buttonKind = iota
	btnPrimarySoft
	btnSecondary
	btnSubtle
	btnGhost
	btnDanger
	btnDangerGhost
)

// hoverColor lifts a saturated colour on hover: darker in a light appearance,
// lighter in a dark one, as filled buttons do on the web.
func (p palette) hoverColor(c ui.Color) ui.Color {
	if p.dark {
		return c.MixOklab(ui.Hex("#FFFFFF"), 0.14)
	}
	return c.MixOklab(ui.Hex("#000000"), 0.10)
}

// pressColor deepens a saturated colour while pressed.
func (p palette) pressColor(c ui.Color) ui.Color {
	if p.dark {
		return c.MixOklab(ui.Hex("#FFFFFF"), 0.04)
	}
	return c.MixOklab(ui.Hex("#000000"), 0.20)
}

// buttonFaces resolves a kind's background, text and border in its state.
func (a *app) buttonFaces(p palette, kind buttonKind, hovered, pressed bool) (bg, fg, border ui.Color) {
	switch kind {
	case btnPrimary:
		bg, fg = p.primary, p.onPrimary
		if pressed {
			bg = p.primaryPressed
		} else if hovered {
			bg = p.primaryHover
		}
	case btnPrimarySoft:
		bg, fg, border = p.primarySoft, p.primaryText, p.primaryBorder
		if pressed || hovered {
			bg = p.primarySoftHov
		}
	case btnSecondary:
		bg, fg, border = p.card, p.text, p.borderStrong
		if pressed {
			bg = p.fillPress
		} else if hovered {
			bg = p.fillHover
		}
	case btnSubtle:
		bg, fg = p.fill, p.text
		if pressed {
			bg = p.fillPress
		} else if hovered {
			bg = p.fillHover
		}
	case btnDanger:
		bg, fg = p.danger, p.onPrimary
		if pressed {
			bg = p.pressColor(p.danger)
		} else if hovered {
			bg = p.hoverColor(p.danger)
		}
	case btnDangerGhost:
		fg = p.danger
		if pressed || hovered {
			bg = p.dangerSoft
			border = p.dangerBorder
		}
	default: // btnGhost
		fg = p.textSecond
		if pressed {
			bg, fg = p.fillPress, p.text
		} else if hovered {
			bg, fg = p.fillHover, p.text
		}
	}
	return bg, fg, border
}

// button builds a labelled, icon-led action. The label is its accessible name
// and tooltip, and the kind and disabled flag carry the intent.
func (a *app) button(c *ui.Context, label string, ic *ui.SVG, kind buttonKind, disabled bool) bool {
	return a.buttonTip(c, label, label, ic, kind, disabled)
}

func (a *app) buttonTip(c *ui.Context, label, tip string, ic *ui.SVG, kind buttonKind, disabled bool) bool {
	return a.buttonSized(c, label, tip, ic, kind, disabled, controlHeight)
}

func (a *app) buttonSized(c *ui.Context, label, tip string, ic *ui.SVG, kind buttonKind, disabled bool, h float32) bool {
	p := a.pal(c)
	b := ui.ButtonBase(c).Label(tip).Tooltip(tip).Disabled(disabled).
		DimDisabled(false).
		Height(h).PaddingX(h * 0.38).Gap(6).Radius(8)
	hovered, pressed := b.Hovered(), b.Pressed()
	if disabled {
		hovered, pressed = false, false
		bg, border := p.fill, p.border
		if kind == btnGhost || kind == btnDangerGhost {
			bg, border = ui.Transparent, ui.Color{}
		}
		b.Background(bg).TextColor(p.textThird)
		if border.A > 0 {
			b.Border(1, border)
		}
		b.Children(func() {
			if ic != nil {
				ui.Icon(c, ic).FontSize(h * 0.42).TextColor(p.textThird)
			}
			ui.Text(c, label).FontSize(fontBody).FontWeight(500).TextColor(p.textThird).SingleLine()
		})
		return b.Clicked()
	}
	bg, fg, border := a.buttonFaces(p, kind, hovered, pressed)
	b.Background(bg).TextColor(fg)
	if border.A > 0 {
		b.Border(1, border)
	}
	if kind == btnPrimary || kind == btnDanger {
		b.Shadow(0, 1, 2, 0, p.shadow)
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(h * 0.42).TextColor(fg)
		}
		ui.Text(c, label).FontSize(fontBody).FontWeight(500).TextColor(fg).SingleLine()
	})
	return b.Clicked()
}

// iconButton is the square, label-less action used in toolbars, card headers
// and dialog titles.
func (a *app) iconButton(c *ui.Context, label string, ic *ui.SVG, kind buttonKind, disabled bool) bool {
	return a.iconButtonSize(c, label, ic, kind, disabled, controlHeightS)
}

func (a *app) iconButtonSize(c *ui.Context, label string, ic *ui.SVG, kind buttonKind, disabled bool, size float32) bool {
	p := a.pal(c)
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Disabled(disabled).
		DimDisabled(false).Size(size, size).Radius(8)
	hovered, pressed := b.Hovered(), b.Pressed()
	if disabled {
		hovered, pressed = false, false
		b.Background(ui.Transparent)
		b.Children(func() {
			ui.Icon(c, ic).FontSize(size * 0.5).TextColor(p.textThird)
		})
		return b.Clicked()
	}
	bg, fg, border := a.buttonFaces(p, kind, hovered, pressed)
	b.Background(bg).TextColor(fg)
	if border.A > 0 {
		b.Border(1, border)
	}
	b.Children(func() {
		ui.Icon(c, ic).FontSize(size * 0.5).TextColor(fg)
	})
	return b.Clicked()
}

// ------------------------------------------------------------------- tags --

// tagHeight and tagRadius make tags read as the web's: a soft fill, a hairline
// of the same hue and a pill or rounded-rect body.
const (
	tagHeight float32 = 22
	tagRadius float32 = 6
)

// tag is a static label, as a rule or a status.
func (a *app) tag(c *ui.Context, text string, t tone) ui.Element {
	return a.tagIcon(c, text, nil, t)
}

func (a *app) tagIcon(c *ui.Context, text string, ic *ui.SVG, t tone) ui.Element {
	p := a.pal(c)
	bg, fg, border := toneColors(p, t)
	e := ui.Row(c).Height(tagHeight).AlignItems(ui.Center).Gap(4).
		PaddingX(8).Radius(tagRadius).Background(bg).Border(1, border).
		TextColor(fg)
	e.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(fg)
		}
		ui.Text(c, text).FontSize(fontCaption).FontWeight(500).TextColor(fg).SingleLine()
	})
	return e
}

// closeTag is a tag with an X: it reports whether the X was clicked. The X
// gains its own hover fill, as a closable tag on the web does.
func (a *app) closeTag(c *ui.Context, text string, t tone, tip string, disabled bool) bool {
	p := a.pal(c)
	bg, fg, border := toneColors(p, t)
	closed := false
	ui.Row(c).Height(tagHeight).AlignItems(ui.Center).Gap(3).
		PaddingX(7).Radius(tagRadius).Background(bg).Border(1, border).
		Children(func() {
			ui.Text(c, text).FontSize(fontCaption).FontWeight(500).TextColor(fg).SingleLine()
			x := ui.ButtonBase(c).Label(tip).Tooltip(tip).Disabled(disabled).Size(15, 15).Radius(4)
			xbg, xfg := ui.Transparent, fg
			if !disabled {
				switch {
				case x.Pressed():
					xbg, xfg = fg.Alpha(0.22), p.danger
				case x.Hovered():
					xbg, xfg = fg.Alpha(0.14), p.danger
				}
			}
			x.Background(xbg)
			x.Children(func() {
				ui.Icon(c, iconX).FontSize(10).TextColor(xfg)
			})
			if x.Clicked() {
				closed = true
			}
		})
	return closed
}

// addTag is the dashed suggestion chip: one click adds that rule.
func (a *app) addTag(c *ui.Context, text, tip string, disabled bool) bool {
	p := a.pal(c)
	b := ui.ButtonBase(c).Label(tip).Tooltip(tip).Disabled(disabled).
		Height(tagHeight).PaddingX(7).Gap(3).Radius(tagRadius)
	bg, fg, border := p.card, p.textSecond, p.borderStrong
	if !disabled {
		switch {
		case b.Pressed():
			bg, fg, border = p.primarySoftHov, p.primaryText, p.primaryBorder
		case b.Hovered():
			bg, fg, border = p.primarySoft, p.primaryText, p.primaryBorder
		}
	}
	b.Background(bg).Border(1, border).BorderStyle(ui.BorderDashed).TextColor(fg)
	b.Children(func() {
		ui.Text(c, text).FontSize(fontCaption).FontWeight(500).TextColor(fg).SingleLine()
	})
	return b.Clicked()
}

// suggestionTags lays out the one-click rule chips.
func (a *app) suggestionTags(c *ui.Context, values []string, disabled bool, pick func(string)) {
	ui.Row(c).Wrap().Gap(5).Children(func() {
		for _, value := range values {
			value := value
			if a.addTag(c, value, value, disabled) {
				pick(value)
			}
		}
	})
}

// tagOnAccent is a tag drawn on the accent fill, as a fetched project row
// selected in a list: a translucent white pill with the accent text colour.
func (a *app) tagOnAccent(c *ui.Context, text string, ic *ui.SVG) {
	p := a.pal(c)
	fg := p.onPrimary
	ui.Row(c).Height(tagHeight).AlignItems(ui.Center).Gap(4).
		PaddingX(8).Radius(tagRadius).
		Background(fg.Alpha(0.16)).Border(1, fg.Alpha(0.24)).TextColor(fg).
		Children(func() {
			if ic != nil {
				ui.Icon(c, ic).FontSize(11).TextColor(fg)
			}
			ui.Text(c, text).FontSize(fontCaption).FontWeight(500).TextColor(fg).SingleLine()
		})
}

// countTag is the small pill beside a section title.
func (a *app) countTag(c *ui.Context, n int, t tone) {
	if n < 0 {
		return
	}
	p := a.pal(c)
	bg, fg, border := toneColors(p, t)
	ui.Box(c).MinWidth(18).Height(18).PaddingX(6).Radius(9).
		Background(bg).Border(1, border).Center().Children(func() {
		ui.Text(c, strconv.Itoa(n)).FontSize(fontMicro).FontWeight(600).TextColor(fg)
	})
}

// statusTag colours a file status with the semantic tones.
func (a *app) statusTag(c *ui.Context, status string) ui.Element {
	t := toneNeutral
	switch status {
	case "new":
		t = toneSuccess
	case "modified":
		t = toneWarning
	case "deleted":
		t = toneDanger
	}
	return a.tag(c, statusDisplay(status, a.lang), t)
}

// -------------------------------------------------------------- containers --

// card is the raised surface sections live on.
func (a *app) card(c *ui.Context, build func()) ui.Element {
	p := a.pal(c)
	e := ui.Column(c).Gap(10).Padding(14).Background(p.card).Border(1, p.border).
		Radius(12).Shadow(0, 1, 3, 0, p.shadow)
	e.Children(build)
	return e
}

// iconTile is the tinted rounded square inside an interactive row (a sidebar
// project, a fetched server project). Decorative headers use plainIcon, which
// never changes under the pointer: only icon buttons answer hover.
func (a *app) iconTile(c *ui.Context, ic *ui.SVG, t tone, size float32) {
	p := a.pal(c)
	bg, fg, _ := toneColors(p, t)
	ui.Box(c).Size(size, size).Radius(size * 0.28).Background(bg).Center().Children(func() {
		ui.Icon(c, ic).FontSize(size * 0.5).TextColor(fg)
	})
}

// plainIcon is a decorative icon: a colour of its own, no face, and no hover
// state, so it does not read as a button.
func (a *app) plainIcon(c *ui.Context, ic *ui.SVG, t tone, size float32) {
	p := a.pal(c)
	fg := p.primary
	if t != tonePrimary {
		_, fg, _ = toneColors(p, t)
	}
	ui.Icon(c, ic).FontSize(size).TextColor(fg)
}

// brandTile is the app mark in the toolbar.
func (a *app) brandTile(c *ui.Context, size float32) {
	a.plainIcon(c, iconCloud, tonePrimary, size*0.94)
}

// cardHeader is the title row of a card: icon, title, count, actions.
func (a *app) cardHeader(c *ui.Context, ic *ui.SVG, title string, count int, actions func()) {
	p := a.pal(c)
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		a.plainIcon(c, ic, tonePrimary, 18)
		ui.Text(c, title).FontSize(fontSection).FontWeight(600).TextColor(p.text).SingleLine()
		if count >= 0 {
			a.countTag(c, count, toneNeutral)
		}
		ui.Box(c).Grow(1)
		if actions != nil {
			actions()
		}
	})
}

// divider is the hairline between sections.
func (a *app) divider(c *ui.Context) {
	p := a.pal(c)
	ui.Box(c).Height(1).FillWidth().Background(p.divider)
}

// ------------------------------------------------------------- typography --

func (a *app) text(c *ui.Context, s string, size float32, weight int, col ui.Color) ui.Element {
	return ui.Text(c, s).FontSize(size).FontWeight(weight).TextColor(col)
}

// sectionTitle heads a block inside a page or dialog.
func (a *app) sectionTitle(c *ui.Context, s string) ui.Element {
	return a.text(c, s, fontSection, 600, a.pal(c).text)
}

// fieldLabel is the name of a form control.
func (a *app) fieldLabel(c *ui.Context, s string) ui.Element {
	return a.text(c, s, fontLabel, 550, a.pal(c).textSecond)
}

// field is a labelled form control: the label above, the description below,
// wired for assistive technology by ui.Field.
func (a *app) field(c *ui.Context, label string, control func()) ui.Element {
	return ui.Field(c, label, control)
}

// kv is a preview row: a muted key and its value.
func (a *app) kv(c *ui.Context, key, value string) {
	p := a.pal(c)
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, key).FontSize(fontCaption).TextColor(p.textThird).Width(84).SingleLine()
		ui.Text(c, value).FontSize(fontLabel).TextColor(p.text).Grow(1).SingleLine()
	})
}

// ------------------------------------------------- notices, busy, empties --

// notice is a soft, bordered line that explains a state.
func (a *app) notice(c *ui.Context, ic *ui.SVG, s string, t tone) {
	p := a.pal(c)
	bg, fg, border := toneColors(p, t)
	ui.Row(c).Gap(7).Padding(8, 10).Background(bg).Border(1, border).Radius(8).
		AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(13).TextColor(fg)
		ui.Text(c, s).FontSize(fontLabel).TextColor(fg).Grow(1).Wrap()
	})
}

// inlineNotice is the border-less form of notice, for footer validation.
func (a *app) inlineNotice(c *ui.Context, ic *ui.SVG, s string, t tone) {
	p := a.pal(c)
	_, fg, _ := toneColors(p, t)
	ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, ic).FontSize(12).TextColor(fg)
		ui.Text(c, s).FontSize(fontCaption).TextColor(fg).Grow(1).Wrap()
	})
}

// busyRow shows work of unknown length: a spinner and what it is.
func (a *app) busyRow(c *ui.Context, s string) {
	p := a.pal(c)
	ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c).Label(s)
		ui.Text(c, s).FontSize(fontCaption).TextColor(p.textSecond).SingleLine()
	})
}

// emptyState is the centred prompt of a list or page with nothing to show.
func (a *app) emptyState(c *ui.Context, ic *ui.SVG, title, hint string, actions func()) {
	p := a.pal(c)
	ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
		ui.Icon(c, ic).FontSize(34).TextColor(p.textThird)
		ui.Text(c, title).FontSize(fontSection).FontWeight(600).TextColor(p.textSecond)
		if hint != "" {
			ui.Text(c, hint).FontSize(fontCaption).TextColor(p.textThird).MaxWidth(340).Wrap()
		}
		if actions != nil {
			ui.Box(c).Height(4)
			actions()
		}
	})
}

// -------------------------------------------------------------- dialog shell --

// dialogShell is the frame every modal shares: heading, a scrollable body and
// a footer separated by hairlines.
func (a *app) dialogShell(c *ui.Context, ic *ui.SVG, title, subtitle string,
	width, height float32, busy bool, onClose func(), body, footer func()) {
	a.dialogChrome(c, ic, tonePrimary, title, subtitle, width, height, busy, onClose, body, footer, false)
}

func (a *app) dialogShellTone(c *ui.Context, ic *ui.SVG, t tone, title, subtitle string,
	width, height float32, busy bool, onClose func(), body, footer func()) {
	a.dialogChrome(c, ic, t, title, subtitle, width, height, busy, onClose, body, footer, false)
}

// dialogShellFill is the variant whose body hands its remaining room to the
// content (a list, a preview), so the page uses the dialog's height instead of
// leaving a gap above the footer.
func (a *app) dialogShellFill(c *ui.Context, ic *ui.SVG, title, subtitle string,
	width, height float32, busy bool, onClose func(), body, footer func()) {
	a.dialogChrome(c, ic, tonePrimary, title, subtitle, width, height, busy, onClose, body, footer, true)
}

func (a *app) dialogChrome(c *ui.Context, ic *ui.SVG, t tone, title, subtitle string,
	width, height float32, busy bool, onClose func(), body, footer func(), fill bool) {
	p := a.pal(c)
	ui.Column(c).Role(ui.RoleDialog).Width(width).Height(height).Background(p.card).
		Border(1, p.borderStrong).Radius(14).Shadow(0, 22, 52, 0, p.shadowStrong).
		Children(func() {
			// Title row: a decorative icon, the heading and its caption.
			ui.Row(c).Padding(12, 20).Shrink(0).AlignItems(ui.Center).Gap(10).Children(func() {
				a.plainIcon(c, ic, t, 24)
				ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
					ui.Text(c, title).FontSize(fontTitle).FontWeight(650).TextColor(p.text).SingleLine()
					if subtitle != "" {
						ui.Text(c, subtitle).FontSize(fontCaption).TextColor(p.textThird).SingleLine()
					}
				})
				if a.iconButton(c, a.t("common.close"), iconX, btnGhost, busy) {
					if onClose != nil {
						onClose()
					}
				}
			})
			a.divider(c)
			// Body.
			if fill {
				ui.Column(c).Grow(1).MinHeight(0).Shrink(1).Padding(12, 20).Gap(12).Children(body)
			} else {
				ui.Scroll(c).Grow(1).MinHeight(0).Shrink(1).KeepScrollbars(true).Children(func() {
					ui.Column(c).Padding(12, 20).Gap(12).Children(body)
				})
			}
			a.divider(c)
			// Footer.
			ui.Row(c).Padding(10, 16).Shrink(0).Background(p.card).AlignItems(ui.Center).Gap(8).Children(footer)
		})
}

// ------------------------------------------------------------------ toasts --

type toastKind int

const (
	toastInfo toastKind = iota
	toastSuccess
	toastWarning
	toastError
)

type toast struct {
	text string
	kind toastKind
	at   time.Time
}

const toastDuration = 4 * time.Second

// notify queues a toast. It is shown by toastOverlay on the next frame and
// retires itself after toastDuration.
func (a *app) notify(kind toastKind, text string) {
	if text == "" {
		return
	}
	a.toasts = append(a.toasts, toast{text: text, kind: kind, at: time.Now()})
	if len(a.toasts) > 3 {
		a.toasts = a.toasts[len(a.toasts)-3:]
	}
}

func toastTone(k toastKind) tone {
	switch k {
	case toastSuccess:
		return toneSuccess
	case toastWarning:
		return toneWarning
	case toastError:
		return toneDanger
	default:
		return toneInfo
	}
}

func toastIcon(k toastKind) *ui.SVG {
	switch k {
	case toastSuccess:
		return iconCheckCircle
	case toastWarning:
		return iconWarning
	case toastError:
		return iconErrorCircle
	default:
		return iconInfo
	}
}

// toastOverlay draws the stack in the window's lower right, newest at the
// bottom, and prunes what expired.
func (a *app) toastOverlay(c *ui.Context) {
	if len(a.toasts) == 0 {
		return
	}
	p := a.pal(c)
	now := c.Now()
	if len(a.toasts) > 0 {
		// Ask for the frame that retires the oldest toast.
		if left := toastDuration - now.Sub(a.toasts[0].at); left > 0 {
			c.After(left)
		}
	}
	ui.Overlay(c, func() {
		ui.Column(c).Absolute().Right(18).Bottom(statusHeight + 18).Width(380).Gap(8).Children(func() {
			kept := a.toasts[:0]
			remove := -1
			for _, item := range a.toasts {
				if now.Sub(item.at) >= toastDuration {
					continue
				}
				bg, fg, border := toneColors(p, toastTone(item.kind))
				ui.Row(c).Padding(10, 12).Background(p.card).Border(1, border).Radius(10).
					Shadow(0, 8, 20, 0, p.shadowStrong).AlignItems(ui.Center).Gap(9).Children(func() {
					ui.Icon(c, toastIcon(item.kind)).FontSize(16).TextColor(fg)
					ui.Text(c, item.text).FontSize(fontLabel).TextColor(p.text).Grow(1).Wrap()
					if a.iconButtonSize(c, a.t("common.close"), iconX, btnGhost, false, 22) {
						remove = len(kept)
					}
				})
				_ = bg
				kept = append(kept, item)
			}
			a.toasts = kept
			if remove >= 0 && remove < len(a.toasts) {
				a.toasts = append(a.toasts[:remove], a.toasts[remove+1:]...)
			}
		})
	})
}
