package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// Palette is the app's view of the theme colour tokens, resolved fresh on
// every theme emission so an OS light/dark switch restyles the whole app.
// The page is the content ground (level 0); nothing in it is raised, so the
// only fills are the components' own. The three readouts wear the three
// accent roles as inks — derived through InkOn so a pale seed can never put
// an unreadable pin on the page.
type Palette struct {
	Backdrop color.NRGBA // the window ground: the Background pin, level 0
	Text     color.NRGBA // body ink: neutral 900
	Label    color.NRGBA // captions and secondary ink: neutral 700
	Volt     color.NRGBA // the voltage readout: Primary as ink
	Amp      color.NRGBA // the current readout: Secondary as ink
	Watt     color.NRGBA // the power readout: Tertiary as ink
	Danger   color.NRGBA // protection trips and errors: Error as ink
	Panel    color.NRGBA // a chart panel: raised one storey off the paper
	Hairline color.NRGBA // panel borders and the recessive chart grid
	TipFill  color.NRGBA // a hint bubble: the inverse surface
	TipInk   color.NRGBA // its text
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	ground := c.SurfaceAt(tokens.Level0)
	return Palette{
		Backdrop: ground,
		Text:     c.Ramps.Neutral.Step(900),
		Label:    c.Ramps.Neutral.Step(700),
		Volt:     c.InkOn(tokens.RolePrimary, ground, tokens.TextFloor),
		Amp:      c.InkOn(tokens.RoleSecondary, ground, tokens.TextFloor),
		Watt:     c.InkOn(tokens.RoleTertiary, ground, tokens.TextFloor),
		Danger:   c.InkOn(tokens.RoleError, ground, tokens.TextFloor),
		Panel:    c.RaisedOn(c.SurfaceAt(tokens.Level0)).Fill,
		Hairline: c.Divider,
		TipFill:  c.InverseSurface,
		TipInk:   c.OnInverseSurface,
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
