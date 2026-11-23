package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The app's design tokens. Colours are not taken from MyGo's default palette
// alone: every surface, border and text level is derived here, so the widgets
// built on them gain hover, pressed and disabled states that read as one
// system in both appearances and for every accent.
type palette struct {
	dark bool

	// Surfaces, from the window's canvas to the face of a card.
	bg        ui.Color
	bgAlt     ui.Color
	card      ui.Color
	cardHover ui.Color
	fill      ui.Color
	fillHover ui.Color
	fillPress ui.Color

	border       ui.Color
	borderStrong ui.Color
	divider      ui.Color

	text       ui.Color
	textSecond ui.Color
	textThird  ui.Color

	primary        ui.Color
	primaryHover   ui.Color
	primaryPressed ui.Color
	onPrimary      ui.Color
	primarySoft    ui.Color
	primarySoftHov ui.Color
	primaryBorder  ui.Color
	primaryText    ui.Color

	success       ui.Color
	successSoft   ui.Color
	successBorder ui.Color
	warning       ui.Color
	warningSoft   ui.Color
	warningBorder ui.Color
	danger        ui.Color
	dangerSoft    ui.Color
	dangerBorder  ui.Color
	info          ui.Color
	infoSoft      ui.Color
	infoBorder    ui.Color

	shadow       ui.Color
	shadowStrong ui.Color
	scrollbar    ui.Color
	focusRing    ui.Color
	inverse      ui.Color
	inverseText  ui.Color
}

// Typography scale, in DIPs. The theme's FontSize is fontBody.
const (
	fontTitle   float32 = 18
	fontSection float32 = 14
	fontBody    float32 = 13
	fontLabel   float32 = 12
	fontCaption float32 = 11
	fontMicro   float32 = 10
)

// Control heights: the toolbar and dialog buttons, inputs and the footer.
const (
	controlHeight  float32 = 34
	controlHeightS float32 = 30
	toolbarHeight  float32 = 56
	statusHeight   float32 = 34
)

func newPalette(dark bool, accent ui.Color) palette {
	if dark {
		return newDarkPalette(accent)
	}
	return newLightPalette(accent)
}

func newLightPalette(accent ui.Color) palette {
	text := ui.Hex("#161A22")
	primaryText := accent.MixOklab(ui.Hex("#0B1220"), 0.12)
	return palette{
		bg:           ui.Hex("#F1F3F7"),
		bgAlt:        ui.Hex("#FBFCFE"),
		card:         ui.Hex("#FFFFFF"),
		cardHover:    ui.Hex("#F8FAFD"),
		fill:         ui.Hex("#F2F4F8"),
		fillHover:    ui.Hex("#E9EDF4"),
		fillPress:    ui.Hex("#DFE5EE"),
		border:       ui.Hex("#E3E8EF"),
		borderStrong: ui.Hex("#C8D1DE"),
		divider:      ui.Hex("#ECF0F5"),
		text:         text,
		textSecond:   ui.Hex("#55606F"),
		textThird:    ui.Hex("#8792A4"),

		primary:        accent,
		primaryHover:   accent.MixOklab(ui.Hex("#000000"), 0.10),
		primaryPressed: accent.MixOklab(ui.Hex("#000000"), 0.20),
		onPrimary:      ui.Hex("#FFFFFF"),
		primarySoft:    accent.Alpha(0.10),
		primarySoftHov: accent.Alpha(0.17),
		primaryBorder:  accent.Alpha(0.28),
		primaryText:    primaryText,

		success:       ui.Hex("#15803D"),
		successSoft:   ui.Hex("#E7F6EC"),
		successBorder: ui.Hex("#BFE5CB"),
		warning:       ui.Hex("#B45309"),
		warningSoft:   ui.Hex("#FFF4E3"),
		warningBorder: ui.Hex("#F3D8A6"),
		danger:        ui.Hex("#DC2626"),
		dangerSoft:    ui.Hex("#FDECEC"),
		dangerBorder:  ui.Hex("#F4B9B9"),
		info:          ui.Hex("#0369A1"),
		infoSoft:      ui.Hex("#E8F3FB"),
		infoBorder:    ui.Hex("#B9D9EE"),

		shadow:       ui.RGBA(18, 26, 45, 0.06),
		shadowStrong: ui.RGBA(18, 26, 45, 0.18),
		scrollbar:    ui.RGBA(18, 26, 45, 0.22),
		focusRing:    accent.Alpha(0.42),
		inverse:      ui.Hex("#1F2937"),
		inverseText:  ui.Hex("#F8FAFC"),
	}
}

func newDarkPalette(accent ui.Color) palette {
	primaryText := accent.MixOklab(ui.Hex("#FFFFFF"), 0.34)
	return palette{
		bg:           ui.Hex("#121317"),
		bgAlt:        ui.Hex("#181A1F"),
		card:         ui.Hex("#1E2026"),
		cardHover:    ui.Hex("#23262D"),
		fill:         ui.Hex("#262932"),
		fillHover:    ui.Hex("#2D313B"),
		fillPress:    ui.Hex("#363B47"),
		border:       ui.Hex("#2F333D"),
		borderStrong: ui.Hex("#404553"),
		divider:      ui.Hex("#262A32"),
		text:         ui.Hex("#E7E9EF"),
		textSecond:   ui.Hex("#A2A9B8"),
		textThird:    ui.Hex("#747C8C"),

		primary:        accent,
		primaryHover:   accent.MixOklab(ui.Hex("#FFFFFF"), 0.16),
		primaryPressed: accent.MixOklab(ui.Hex("#FFFFFF"), 0.06),
		onPrimary:      ui.Hex("#FFFFFF"),
		primarySoft:    accent.Alpha(0.18),
		primarySoftHov: accent.Alpha(0.28),
		primaryBorder:  accent.Alpha(0.40),
		primaryText:    primaryText,

		success:       ui.Hex("#4ADE80"),
		successSoft:   ui.RGBA(74, 222, 128, 0.14),
		successBorder: ui.RGBA(74, 222, 128, 0.34),
		warning:       ui.Hex("#FBBF24"),
		warningSoft:   ui.RGBA(251, 191, 36, 0.14),
		warningBorder: ui.RGBA(251, 191, 36, 0.34),
		danger:        ui.Hex("#F87171"),
		dangerSoft:    ui.RGBA(248, 113, 113, 0.14),
		dangerBorder:  ui.RGBA(248, 113, 113, 0.34),
		info:          ui.Hex("#38BDF8"),
		infoSoft:      ui.RGBA(56, 189, 248, 0.14),
		infoBorder:    ui.RGBA(56, 189, 248, 0.34),

		shadow:       ui.RGBA(0, 0, 0, 0.36),
		shadowStrong: ui.RGBA(0, 0, 0, 0.55),
		scrollbar:    ui.RGBA(255, 255, 255, 0.22),
		focusRing:    accent.Alpha(0.50),
		inverse:      ui.Hex("#F2F4F7"),
		inverseText:  ui.Hex("#1B1F26"),
	}
}

// pal resolves the current tokens. The appearance comes from the context's
// theme, which follows the system when the appearance setting is "system".
func (a *app) pal(c *ui.Context) palette {
	return newPalette(c.Theme().Dark, ui.Hex(accentHex(normalizeAccent(a.accent))))
}

// applyTokens puts the palette into the theme the widgets are built with, so
// built-in controls (buttons, inputs, tables, scroll bars, focus rings) share
// the app's colours instead of MyGo's defaults.
func (a *app) applyTokens(c *ui.Context) {
	base := *c.Theme()
	p := a.pal(c)
	base.Background = p.bg
	base.Surface = p.fill
	base.SurfaceHover = p.fillHover
	base.SurfacePressed = p.fillPress
	base.Border = p.border
	base.Text = p.text
	base.TextMuted = p.textSecond
	base.Accent = p.primary
	base.AccentHover = p.primaryHover
	base.AccentPressed = p.primaryPressed
	base.AccentText = p.onPrimary
	base.InactiveAccent = p.primary
	base.Danger = p.danger
	base.Warning = p.warning
	base.Success = p.success
	base.Selection = p.primary.Alpha(0.26)
	base.Focus = p.focusRing
	base.FocusRingWidth = 2
	base.FocusRingOffset = 1
	base.Inverse = p.inverse
	base.InverseText = p.inverseText
	base.Scrollbar = p.scrollbar
	base.Radius = 8
	base.Spacing = 4
	base.FontSize = fontBody
	base.TooltipDelay = 450 * time.Millisecond
	c.SetTheme(&base)
}

// tone is a semantic accent of a tag, a notice or an icon tile.
type tone int

const (
	toneNeutral tone = iota
	tonePrimary
	toneSuccess
	toneWarning
	toneDanger
	toneInfo
)

func (t tone) String() string {
	switch t {
	case tonePrimary:
		return "primary"
	case toneSuccess:
		return "success"
	case toneWarning:
		return "warning"
	case toneDanger:
		return "danger"
	case toneInfo:
		return "info"
	default:
		return "neutral"
	}
}

// toneColors resolves a tone's soft background, readable text and border.
func toneColors(p palette, t tone) (bg, fg, border ui.Color) {
	switch t {
	case tonePrimary:
		return p.primarySoft, p.primaryText, p.primaryBorder
	case toneSuccess:
		return p.successSoft, p.success, p.successBorder
	case toneWarning:
		return p.warningSoft, p.warning, p.warningBorder
	case toneDanger:
		return p.dangerSoft, p.danger, p.dangerBorder
	case toneInfo:
		return p.infoSoft, p.info, p.infoBorder
	default:
		return p.fill, p.textSecond, p.border
	}
}
