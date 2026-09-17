package main

import (
	stdcolor "image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// isDark reports which side of the platform's pair a set is, by the luminance
// of the window's own plane.
func isDark(c tokens.PlatformColors) bool {
	return vgcolor.RelativeLuminance(c.WindowBackground) < 0.5
}

// SchemeColors is the platform's set for one appearance: the live set where
// the appearance asked for is the one the desktop is on, and the recorded set
// for the other side, which cannot be read live while the desktop is on this
// one — and which is what every platform but macOS carries anyway.
//
// The recorded set is given the live accent, so a window flipped to the
// appearance the desktop is not on keeps the accent colour the machine is
// actually set to rather than falling back to the one that was recorded.
func SchemeColors(live tokens.PlatformColors, dark bool) tokens.PlatformColors {
	if dark == isDark(live) {
		return live
	}
	recorded := tokens.PlatformLight
	if dark {
		recorded = tokens.PlatformDark
	}
	return recorded.WithAccent(live.ControlAccent)
}

// WindowSet is what this window itself wears: the platform's set for the
// appearance the switch at the top of the window is on.
//
// The window draws in the appearance being chosen and not in the desktop's.
// Every choice below the switch is made for one appearance — a theme colour is
// judged on the plane it lands on, a highlighter style is fitted to a
// background — so the window shows the one being settled, and the switch moves
// the whole of it.
func WindowSet(live tokens.PlatformColors, m Model) tokens.PlatformColors {
	return SchemeColors(live, m.Dark(live))
}

// PreviewSet is what the window previews: [SchemeColors] with the theme
// colour standing in where the platform uses its accent — the default button,
// the selection, the focus ring. With nothing chosen, and on a desktop that
// reports no accent, it is the platform's set as it stands.
func PreviewSet(live tokens.PlatformColors, m Model, dark bool) tokens.PlatformColors {
	set := SchemeColors(live, dark)
	if col, ok := m.Color(); ok {
		return set.WithAccent(col)
	}
	return set
}

// Palette is the window's own colours: the platform's name for what each one
// draws, flattened onto the fill it lands on.
//
// The window draws in [WindowSet] — the platform's set for the appearance the
// switch at the top of the window is on — so the page a reader is looking at
// is the appearance they are settling.
type Palette struct {
	Backdrop         stdcolor.NRGBA // the window's own plane, and the picture's mat
	Surface          stdcolor.NRGBA // a group's box
	Edge             stdcolor.NRGBA // a hairline in a box: the frame round a swatch
	Text             stdcolor.NRGBA // a group's title, on the plane
	Muted            stdcolor.NRGBA // a title row's hint, on the plane
	CardText         stdcolor.NRGBA // a row's own words, in the box
	CardMuted        stdcolor.NRGBA // a caption and a swatch's label, in the box
	Hover            stdcolor.NRGBA // a row or a card under the pointer
	Accent           stdcolor.NRGBA // the theme colour in force
	AccentForeground stdcolor.NRGBA // what reads on the selection
	Selection        stdcolor.NRGBA // the chosen row's or card's fill
	Problem          stdcolor.NRGBA // a drop that produced nothing
}

// PaletteFrom reads the window's colours off the platform's set.
//
// A group's box is the platform's box on the window's plane, so it takes the
// box's fill and a seam for the hairline anything inside it wears. A swatch is
// a graphic the pointer operates, which is what the platform lays its hover
// overlay over; a list row and a push button, which do not tint, are neither.
func PaletteFrom(c tokens.PlatformColors) Palette {
	return Palette{
		Backdrop:         c.WindowBackground,
		Surface:          c.CardFill,
		Edge:             vgcolor.Flatten(c.Separator, c.CardFill),
		Text:             vgcolor.Flatten(c.Label, c.WindowBackground),
		Muted:            vgcolor.Flatten(c.SecondaryLabel, c.WindowBackground),
		CardText:         vgcolor.Flatten(c.Label, c.CardFill),
		CardMuted:        vgcolor.Flatten(c.SecondaryLabel, c.CardFill),
		Hover:            vgcolor.Flatten(c.HoverOverlay, c.CardFill),
		Accent:           c.ControlAccent,
		AccentForeground: c.AlternateSelectedControlText,
		Selection:        c.SelectedContentBackground,
		Problem:          c.SystemRed,
	}
}

// Type is the application's view of the theme's Typography: the roles it
// draws directly, as textdraw styles, plus the theme's cached shaper. The
// application builds no shaper and bundles no font of its own.
//
// Four tiers, and they differ from each other in size as well as in weight:
// the window's own name, a group's title, a row's words, a caption. The
// platform sets a group's heading in semibold where a row's label is regular,
// and reading this window cold against it found our two — one role at medium
// and one at regular, both 14 — did not separate: "the headings are not
// headings", one tier where the platform has three.
type Type struct {
	Shaper *text.Shaper
	Head   textdraw.TextStyle // TitleLarge: the window's own name in the title row
	Label  textdraw.TextStyle // TitleMedium: a group's title
	Body   textdraw.TextStyle // BodyMedium: a row's own words
	Small  textdraw.TextStyle // BodySmall: hints, hex values and shares
	// Role is LabelLarge as the theme states it, for the controls here drawn
	// by a published component rather than by this application: the
	// component lays the role out itself, in the line box the role names.
	Role tokens.TextStyle
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper: t.Shaper(),
		Head:   textStyle(t.TitleLarge),
		Label:  textStyle(t.TitleMedium),
		Body:   textStyle(t.BodyMedium),
		Small:  textStyle(t.BodySmall),
		Role:   t.LabelLarge,
	}
}

// Ellipsis is the mark a run of text wears when it was cut short: the
// shaper's own truncator, and the one this window appends when it does the
// cutting itself rather than leaving it to the shaper. One mark, so a reader
// meets one sign for one fact wherever a line stopped early.
const Ellipsis = "…"

// textStyle converts one Typography role to a single-line textdraw style.
func textStyle(ts tokens.TextStyle) textdraw.TextStyle {
	f := font.Font{Typeface: font.Typeface(ts.Typeface)}
	if ts.Weight != 0 {
		f.Weight = tokens.FontWeight(ts.Weight)
	}
	return textdraw.TextStyle{Font: f, Alignment: textdraw.Start, Size: unit.Sp(ts.Size), MaxLines: 1, Truncator: Ellipsis}
}
