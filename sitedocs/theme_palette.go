// theme_palette.go is what this window brings to the colour board: the
// board itself is the shared section that stands beside the published
// inventory — components/gallery/palette — and every application that shows
// the platform's set draws it from there, so what is on screen cannot drift
// from the set it is describing or from what another window says about it.
//
// What is kept here is what belongs to this window: the colours and type
// roles it draws the board's own frame in, and the conversions that hand the
// window's set and typography over in the shape the board asks for.
//
// The one difference from the shared board's own view of typography:
// TypeFrom takes the shaper explicitly, so goldens can hand in the
// deterministic one instead of the theme's cached system shaper.

package main

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// paletteChrome is the board's own frame, drawn in the set the board is
// describing: the band a section heading fills is chrome capping the content
// the board's rows stand on, so it wears the chrome material and parts itself
// from what is under it with the platform's seam.
//
// The two foregrounds are the platform's labels at two strengths, flattened
// onto the fill each actually lands on — the board's rows stand on the
// content plane, which is what the shared section fills its bodies with.
func paletteChrome(c tokens.PlatformColors) palette.Chrome {
	return palette.Chrome{
		Surface: c.SidebarMaterial,
		Seam:    vgcolor.Flatten(c.Separator, c.SidebarMaterial),
		Text:    vgcolor.Flatten(c.Label, c.ControlBackground),
		Muted:   vgcolor.Flatten(c.SecondaryLabel, c.ControlBackground),
	}
}

// Type is this window's view of the theme's Typography: the roles it
// draws directly, as textdraw styles, plus the shaper it draws them with.
type Type struct {
	Shaper *text.Shaper
	Head   textdraw.TextStyle // TitleSmall: the board's column heads
	Label  textdraw.TextStyle // LabelLarge: section labels
	Body   textdraw.TextStyle // BodyMedium: a row's name
	Small  textdraw.TextStyle // BodySmall: values and captions
}

// TypeFrom resolves the roles from the typography, drawing with the shaper
// handed in — the theme's cached one in the app, the deterministic one in
// goldens.
func TypeFrom(shaper *text.Shaper, t tokens.Typography) Type {
	return Type{
		Shaper: shaper,
		Head:   palTextStyle(t.TitleSmall),
		Label:  palTextStyle(t.LabelLarge),
		Body:   palTextStyle(t.BodyMedium),
		Small:  palTextStyle(t.BodySmall),
	}
}

// story is the window's typography as the shared section takes it: the four
// roles it sets text in, with the shaper this window resolved.
func (t Type) story() palette.Type {
	return palette.Type{
		Shaper: t.Shaper,
		Head:   t.Head,
		Label:  t.Label,
		Body:   t.Body,
		Small:  t.Small,
	}
}

// Ellipsis is the mark a run of text wears when it was cut short.
const Ellipsis = "…"

// palTextStyle converts one Typography role to a single-line textdraw style.
func palTextStyle(ts tokens.TextStyle) textdraw.TextStyle {
	f := font.Font{Typeface: font.Typeface(ts.Typeface)}
	if ts.Weight != 0 {
		f.Weight = tokens.FontWeight(ts.Weight)
	}
	return textdraw.TextStyle{Font: f, Alignment: textdraw.Start, Size: unit.Sp(ts.Size), MaxLines: 1, Truncator: Ellipsis}
}

// PaletteSectionRows is how many rows PaletteRows returns: the board's
// heading and its body.
const PaletteSectionRows = 2

// PaletteRows is the colour board as rows of this window's column, framed in
// the colours and type roles this window resolved.
func PaletteRows(c tokens.PlatformColors, ty Type) []layout.Widget {
	return palette.Rows(paletteChrome(c), c, ty.story())
}
