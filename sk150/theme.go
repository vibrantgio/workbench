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

// The meter's own display, measured off the brightest lit-segment pixels of
// the photograph in the plan's reference/sk150-display.png. These are the
// device's values, not the platform's: the readout panel is a picture of the
// meter, so it is the same in both colour schemes — the meter has one panel.
var (
	displayVolt  = color.NRGBA{R: 0x2b, G: 0xf4, B: 0x2f, A: 0xff}
	displayAmp   = color.NRGBA{R: 0xfe, G: 0xfb, B: 0x43, A: 0xff}
	displayWatt  = color.NRGBA{R: 0xf9, G: 0x28, B: 0xfa, A: 0xff}
	displayPanel = color.NRGBA{R: 0x09, G: 0x09, B: 0x05, A: 0xff}

	// The panel's titles carry no hue. Over the photograph's lit area
	// every lit pixel falls in the green, the yellow or the magenta above,
	// and six pixels in twenty-one thousand are unsaturated, so the
	// display has no fourth colour to spend on a word. This is the panel's
	// own black lifted two fifths of the way to white, a brightness rather
	// than a colour, which leaves the three-hue code the readings teach
	// intact. The OWON photograph the boxes come from writes its own box
	// titles in a light blue, #80C6F6 over 337 glyph-core pixels at
	// x 660-1000 and x 1680-1990, y 1835-1905, upright, with the blown
	// pixels dropped — that device's fourth colour, and not one this
	// display holds.
	displayCaption = color.NRGBA{R: 0x6b, G: 0x6b, B: 0x69, A: 0xff}

	// displayRim draws the edges of the two boxes at the panel's foot. It
	// is the one value the panel takes from the OWON SPE6103 photograph
	// the boxes themselves come from, reference/sk150-display-2026-09-23.jpeg
	// read upright: the median of 41139 pixels on both boxes' four edges
	// with every pixel the camera blew out dropped — the left box's left
	// edge at x 378-394 y 1930-2030, its bottom at y 2042-2052, the right
	// box's right edge at x 2222-2238 and its bottom, and the two top
	// edges. The photograph's exposure falls off across the display, so
	// the same edge reads #2B83EF at the bright left and #303BB1 at the
	// dim right; the median stands between them. The edge is a line and
	// not a lit value, so the three-hue code the readings teach is
	// untouched.
	displayRim = color.NRGBA{R: 0x1d, G: 0x58, B: 0xe9, A: 0xff}
)

// Palette is the app's view of the platform's colour set, resolved fresh on
// every theme emission so an appearance switch restyles the whole app, plus
// the four device values the readout panel is lit in. The page stands on the
// window's own plane and paints no fill of its own; the readout panel and the
// chart panels are the things standing on it.
//
// Every alpha-carrying platform name is flattened onto the fill it lands on
// here, so what reaches Gio is opaque.
type Palette struct {
	Backdrop       color.NRGBA // the window's own plane
	Label          color.NRGBA // body text
	SecondaryLabel color.NRGBA // captions and text at the second strength
	Dim            color.NRGBA // a disabled glyph and an idle badge's label
	// Accent is every on-and-active mark outside the readout panel: the lit
	// bolt, the power glyph, a closed switch, the active memory group and the
	// preset badge.
	Accent     color.NRGBA
	VoltSeries color.NRGBA // the output-voltage history's stroke
	AmpSeries  color.NRGBA // the output-current history's stroke
	Danger     color.NRGBA // protection trips and errors
	FilledText color.NRGBA // a label drawn on a platform fill
	ChartPanel color.NRGBA // a chart panel's fill
	Grid       color.NRGBA // the chart's recessive grid lines
	Seam       color.NRGBA // a switch's track while it is off
	Hover      color.NRGBA // a header button under the pointer
	Press      color.NRGBA // a header button held down

	// The readout panel, off the photograph and the same in both schemes.
	DisplayVolt  color.NRGBA // the voltage readout, its unit and the CV badge
	DisplayAmp   color.NRGBA // the current readout, its unit, the CC badge and the ON badge
	DisplayWatt  color.NRGBA // the power readout and its unit
	DisplayPanel color.NRGBA // the panel the readouts are lit on, and the label cut out of a lit badge

	// DisplayCaption is what the panel writes a word with: the Set and
	// Limit titles inside the two boxes at its foot.
	DisplayCaption color.NRGBA
	// DisplayRim is the edge of each of those two boxes.
	DisplayRim color.NRGBA
}

// PaletteFrom reads the page off the platform's set and hands the readout
// panel the meter's own values.
//
// The window's chrome, its controls and its labels are the platform's
// throughout. The three readouts are not: the owner asked for the colours of
// the device's display, so the voltage line is its green, the current line
// and the ON badge its yellow, the power line its magenta, all lit on its
// black. The two history charts stand outside that panel and keep the
// platform's names — the voltage series takes the accent, the current series
// one more of the platform's named colours, neither of them one of the four
// the platform reserves for a status, so a series can never be mistaken for a
// warning.
func PaletteFrom(c tokens.PlatformColors) Palette {
	return Palette{
		Backdrop:       c.WindowBackground,
		Label:          vgcolor.Flatten(c.Label, c.WindowBackground),
		SecondaryLabel: vgcolor.Flatten(c.SecondaryLabel, c.WindowBackground),
		Dim:            vgcolor.Flatten(c.DisabledControlText, c.WindowBackground),
		Accent:         c.ControlAccent,
		VoltSeries:     c.ControlAccent,
		AmpSeries:      c.SystemTeal,
		Danger:         c.SystemRed,
		FilledText:     c.AlternateSelectedControlText,
		ChartPanel:     c.CardFill,
		Grid:           c.Grid,
		Seam:           vgcolor.Flatten(c.Separator, c.WindowBackground),
		Hover:          vgcolor.Flatten(c.HoverOverlay, c.WindowBackground),
		Press:          vgcolor.Flatten(c.PressOverlay, c.WindowBackground),

		DisplayVolt:  displayVolt,
		DisplayAmp:   displayAmp,
		DisplayWatt:  displayWatt,
		DisplayPanel: displayPanel,

		DisplayCaption: displayCaption,
		DisplayRim:     displayRim,
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
	Set    textdraw.TextStyle // the Set and Limit lines under the readouts
	Stack  textdraw.TextStyle // the CV/CC and ON/OFF badge labels
	Title  textdraw.TextStyle // section headings and the header title
	Body   textdraw.TextStyle // status lines and field captions
	Small  textdraw.TextStyle // the notice and connection lines
	Mono   textdraw.TextStyle // aligned digit columns at body size
	Table  textdraw.TextStyle // the preset table: compact aligned columns
}

// appShaper hands the app's typography its shaper. The window draws with the
// platform's fonts as fallback, which varies by machine; a stored render
// replaces this with the pinned collection so the same text shapes to the same
// pixels everywhere.
var appShaper = func(t tokens.Typography) *text.Shaper { return t.Shaper() }

func TypeFrom(t tokens.Typography) Type {
	digits := t.Code
	digits.Size = 56
	digits.Weight = 700
	unit := t.Code
	unit.Size = 24
	unit.Weight = 700
	// Off the OWON photograph the set and limit digits stand about a fifth
	// of the reading digits' height (80 px against 426 px). A fifth of 56 sp
	// is 11 sp, under what this window reads at 1x, so the two lines take
	// 16 sp — a little over a quarter of the readings.
	set := t.Code
	set.Size = 16
	set.Weight = 700
	stack := t.Code
	stack.Size = 14
	stack.Weight = 700
	table := t.Code
	table.Size = 12
	return Type{
		Shaper: appShaper(t),
		Digits: textStyle(digits),
		Unit:   textStyle(unit),
		Set:    textStyle(set),
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
