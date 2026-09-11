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

// Palette is the app's view of the platform's colour set, resolved fresh on
// every theme emission so an appearance switch restyles the whole app. The
// page stands on the window's own plane and paints no fill of its own; the
// chart panels are the one thing standing on it, and they wear the
// platform's box fill.
//
// Every alpha-carrying platform name is flattened onto the fill it lands on
// here, so what reaches Gio is opaque.
type Palette struct {
	Backdrop   color.NRGBA // the window's own plane
	Label      color.NRGBA // body text
	Secondary  color.NRGBA // captions and secondary text
	Dim        color.NRGBA // a disabled glyph and an idle badge's label
	Volt       color.NRGBA // the voltage readout, and every on-and-active mark
	Amp        color.NRGBA // the current readout
	Watt       color.NRGBA // the power readout
	Danger     color.NRGBA // protection trips and errors
	FilledText color.NRGBA // a label drawn on a fill one of the four paints
	Panel      color.NRGBA // a chart panel's fill
	Grid       color.NRGBA // the chart's recessive grid lines
	Seam       color.NRGBA // a switch's track while it is off
	Hover      color.NRGBA // a header button under the pointer
	Press      color.NRGBA // a header button held down
}

// PaletteFrom reads the page off the platform's set.
//
// The three readouts are three quantities the reader has to tell apart at a
// glance, so each takes a colour the platform publishes by name. Voltage
// takes the accent, which is also what every on-and-active mark in this
// window wears — the lit bolt, the ON badge, a closed switch, the active
// memory slot — so the panel and the controls agree without a second
// colour. Current and power take two more of the platform's named colours,
// picked as far from the accent and from each other as the catalogue
// allows; neither is one of the four the platform reserves for a status, so
// a readout can never be mistaken for a warning.
func PaletteFrom(c tokens.PlatformColors) Palette {
	return Palette{
		Backdrop:   c.WindowBackground,
		Label:      vgcolor.Flatten(c.Label, c.WindowBackground),
		Secondary:  vgcolor.Flatten(c.SecondaryLabel, c.WindowBackground),
		Dim:        vgcolor.Flatten(c.DisabledControlText, c.WindowBackground),
		Volt:       c.ControlAccent,
		Amp:        c.SystemTeal,
		Watt:       c.SystemPurple,
		Danger:     c.SystemRed,
		FilledText: c.AlternateSelectedControlText,
		Panel:      c.CardFill,
		Grid:       c.Grid,
		Seam:       vgcolor.Flatten(c.Separator, c.WindowBackground),
		Hover:      vgcolor.Flatten(c.HoverOverlay, c.WindowBackground),
		Press:      vgcolor.Flatten(c.PressOverlay, c.WindowBackground),
	}
}

// Type is the app's view of the theme's Typography: the roles the app draws
// directly with textdraw, plus the theme's cached shaper. Digits is the Code
// role blown up to panel size — the mono face keeps the three readouts'
// digit columns aligned, the way the device's own seven-segment panel does.
type Type struct {
	Shaper *text.Shaper
	Digits textdraw.TextStyle // the V/A/W readouts: Code face at 56 sp
	Unit   textdraw.TextStyle // the unit letters: half the digit size
	Stack  textdraw.TextStyle // the CV/CC and ON/OFF badge labels
	Title  textdraw.TextStyle // section headings and the header title
	Body   textdraw.TextStyle // status lines and field captions
	Small  textdraw.TextStyle // the notice and connection lines
	Mono   textdraw.TextStyle // aligned digit columns at body size
	Table  textdraw.TextStyle // the preset table: compact aligned columns
}

func TypeFrom(t tokens.Typography) Type {
	digits := t.Code
	digits.Size = 56
	digits.Weight = 700
	unit := t.Code
	unit.Size = 24
	unit.Weight = 700
	stack := t.Code
	stack.Size = 14
	stack.Weight = 700
	table := t.Code
	table.Size = 12
	return Type{
		Shaper: t.Shaper(),
		Digits: textStyle(digits),
		Unit:   textStyle(unit),
		Stack:  textStyle(stack),
		Title:  textStyle(t.TitleLarge),
		Body:   textStyle(t.BodyLarge),
		Small:  textStyle(t.BodyMedium),
		Mono:   textStyle(t.Code),
		Table:  textStyle(table),
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
	Padding    unit.Dp = 16
	RowGap     unit.Dp = 8
	FieldWidth unit.Dp = 140
	LabelWidth unit.Dp = 190
)
