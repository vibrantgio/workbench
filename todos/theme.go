package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// Palette is the app's view of the theme colour tokens: a handful of named
// roles derived from tokens.ColorTokens on every theme emission. Because the
// theme window feeds a live OS theme, an OS light/dark switch re-emits the
// tokens and restyles the whole app with no imperative wiring.
type Palette struct {
	Backdrop color.NRGBA // the window ground: the Background pin, level 0
	Dialog   color.NRGBA // the modal's surface, level 2
	Edit     color.NRGBA // the dialog's text-entry fill, level 3
	Select   color.NRGBA // placeholder text and editor selection
	Text     color.NRGBA // primary text
	Icon     color.NRGBA // accent glyphs and dialog border
	Cover    color.NRGBA // modal scrim over the disabled page
}

// PaletteFrom resolves the palette in ADR-007's vocabulary, assigned by
// ADR-021's window grammar. The list is the thing this window exists to show,
// so it is the content ground: it wears level 0 and paints no surface of its
// own. Backdrop is that ground — the Background pin, filled once underneath
// everything — and nothing in the page is raised above it. The only fills over
// it belong to the modal, and a modal is transient: the dialog takes level 2,
// the rung the pattern library reserves for a dialog, and its text-entry field
// walks one rung on from the dialog it lies in rather than from the window —
// level 3 — because a rung is counted from the surface a thing is lying on.
// Primary is the pinned accent, and the remaining Neutral steps are inks: 700
// the low-contrast text step, 900 the body-text step.
func PaletteFrom(c tokens.ColorTokens) Palette {
	return Palette{
		Backdrop: c.SurfaceAt(tokens.Level0),
		Dialog:   c.SurfaceAt(tokens.Level2),
		Edit:     c.SurfaceAt(tokens.Level3),
		Select:   c.Ramps.Neutral.Step(700),
		Text:     c.Ramps.Neutral.Step(900),
		Icon:     c.Primary,
		// A scrim darkens regardless of scheme, so it is black-based
		// rather than token-based.
		Cover: color.NRGBA{A: 153},
	}
}

// Type is the app's view of the theme's Typography (ADR-003): the two roles
// the app draws directly, converted to textdraw styles, plus the theme's
// cached shaper. The app builds no shaper and bundles no font of its own —
// the typeface arrives through the theme.
type Type struct {
	Shaper   *text.Shaper
	Headline textdraw.TextStyle // HeadlineSmall: the dialog's editor text
	Title    textdraw.TextStyle // TitleLarge: list rows and placeholder text
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper:   t.Shaper(),
		Headline: textStyle(t.HeadlineSmall),
		Title:    textStyle(t.TitleLarge),
	}
}

// textStyle converts one Typography role to a single-line textdraw style.
func textStyle(ts tokens.TextStyle) textdraw.TextStyle {
	f := font.Font{Typeface: font.Typeface(ts.Typeface)}
	if ts.Weight != 0 {
		f.Weight = tokens.FontWeight(ts.Weight)
	}
	return textdraw.TextStyle{Font: f, Alignment: textdraw.Start, Size: unit.Sp(ts.Size), MaxLines: 1, Truncator: "…"}
}

// Static layout dimensions; these do not vary with the colour scheme.
const (
	ModalWidth   unit.Dp = 650
	ModalHeight  unit.Dp = 200
	BorderRadius unit.Dp = 5
	BorderWidth  unit.Dp = 2
	Padding      unit.Dp = 12
)
