package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// Palette is the app's view of the platform's colour set: the named places
// this window paints, each resolved from tokens.PlatformColors on every theme
// emission. Because the theme window feeds a live OS theme, an OS light/dark
// switch re-emits the set and restyles the whole app with no imperative
// wiring.
//
// Which platform name each place takes is what this window IS on macOS.
// Nothing here is chrome: the window is one plane with a catalogue drawn on
// it, so the fill is the window's own, and the symbols and their captions are
// drawn in the platform's label colour over it. No symbol here is selected, so
// none is drawn in the accent. Every alpha-carrying name is flattened onto the fill it
// lands on, in encoded sRGB, so what is passed to Gio is opaque.
type Palette struct {
	Backdrop color.NRGBA // the window's own plane, full-bleed
	Text     color.NRGBA // the symbols, their captions and the section labels
	Muted    color.NRGBA // section notes
}

// PaletteFrom resolves the window's places from the platform's set.
func PaletteFrom(c tokens.PlatformColors) Palette {
	plane := c.WindowBackground
	return Palette{
		Backdrop: plane,
		Text:     vgcolor.Flatten(c.Label, plane),
		Muted:    vgcolor.Flatten(c.SecondaryLabel, plane),
	}
}

// Type is the app's view of the theme's Typography: the roles the app draws
// directly, converted to textdraw styles, plus the theme's cached shaper. The
// app builds no shaper and bundles no font of its own — the typeface arrives
// through the theme.
type Type struct {
	Shaper  *text.Shaper
	Caption textdraw.TextStyle // BodySmall: icon captions
	Section textdraw.TextStyle // TitleSmall: the labels over the two sets
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper:  t.Shaper(),
		Caption: textStyle(t.BodySmall),
		Section: textStyle(t.TitleSmall),
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
	Padding  unit.Dp = 12
	CellW    unit.Dp = 160 // grid cell width: symbol + caption both fit
	CellH    unit.Dp = 84  // 40 dp symbol, gap, one caption line, padding
	IconSize unit.Dp = 40
	HeadingH unit.Dp = 28 // one section label on its own line
)
