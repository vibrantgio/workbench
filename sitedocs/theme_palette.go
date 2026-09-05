// theme_palette.go is what this window brings to the palette story: the
// ramps grid and the named picks with the rule that chose each one are the
// shared section that stands beside the published inventory —
// components/gallery/palette — and every application that shows a palette
// draws it from there, so the rules on screen cannot drift from the palette
// they describe or from each other.
//
// What is kept here is what belongs to this window: the colours and type
// roles it draws its own furniture in, which side of the scheme pair is on
// screen and what the other side is, and the conversions that hand the
// window's palette and typography over in the shape the story asks for.
//
// The one difference from the shared story's own view of typography:
// TypeFrom takes the shaper explicitly, so goldens can hand in the
// deterministic one instead of the theme's cached system shaper.

package main

import (
	"image"
	stdcolor "image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The window's own furniture measurements, which are the ones it draws the
// seed row's swatches with. The story keeps its own copies of these for the
// cells it draws itself; they are the same numbers, because a swatch above
// the board and a swatch on it are the same object shown twice.
const (
	Hairline unit.Dp = 1 // resting outlines
	InnerR   unit.Dp = 8 // swatch and chip corners
)

// edgeFloor is WCAG 1.4.11's contrast floor for a graphic that carries
// meaning without being text — 3:1. An edge is exactly that: it is the whole
// of what says where one plane ends and the next begins, so it is not
// decoration and owes its ground this much.
const edgeFloor = 3.0

// Palette is this window's view of the colour tokens: every colour it
// draws its own furniture with, named for what it draws.
type Palette struct {
	Backdrop stdcolor.NRGBA
	Surface  stdcolor.NRGBA
	Divider  stdcolor.NRGBA
	CardEdge stdcolor.NRGBA
	Edge     stdcolor.NRGBA
	Text     stdcolor.NRGBA
	Muted    stdcolor.NRGBA
	Accent   stdcolor.NRGBA
	OnAccent stdcolor.NRGBA
	Problem  stdcolor.NRGBA
}

// PaletteFrom resolves the palette in the token vocabulary.
func PaletteFrom(c tokens.ColorTokens) Palette {
	return Palette{
		Backdrop: c.Background,
		// The section headings' band: a filled strip lying on the page the
		// section is printed on, so it is the RAISE walked from that page,
		// lighter than it in both schemes. A ramp index is not a raise —
		// neutral 200 is #E8E8E8 UNDER a light page, a resting band darker
		// than what it lies on, which is the one arrangement elevation
		// forbids. The walk answers #FFFFFF on paper and #222222 on slate.
		Surface:  raisedOnPage(c),
		Divider:  c.Ramps.Neutral.Step(300),
		CardEdge: c.Ramps.Neutral.Step(400),
		// The heaviest edge, derived rather than named: the neutral step the
		// ramp measures as reaching the graphic floor against Surface — the
		// raise the headings above fill at. A named step 500 would mean two
		// different weights from a line that looks scheme-neutral: measured,
		// 2.35:1 in the light scheme against 5.94:1 in the dark.
		Edge:     c.MarkOn(tokens.RoleNeutral, raisedOnPage(c), edgeFloor),
		Text:     c.Text,
		Muted:    c.Ramps.Neutral.Step(700),
		Accent:   c.Primary,
		OnAccent: c.OnPrimary,
		Problem:  c.Error,
	}
}

// story is the window's palette as the shared section takes it: the four
// colours that section draws its own furniture with, and no more.
//
// The window's palette names more than that — a card edge, an accent and
// the ink over it, the colour a problem is said in — and none of them is a
// colour the story has any business reading. Handing over four fields
// rather than the whole value is what keeps that true by construction: a
// colour this window adds later cannot silently start appearing inside a
// section that never asked for it.
func (p Palette) story() palette.Chrome {
	return palette.Chrome{
		Surface: p.Surface,
		Divider: p.Divider,
		Text:    p.Text,
		Muted:   p.Muted,
	}
}

// Type is this window's view of the theme's Typography: the roles it
// draws directly, as textdraw styles, plus the shaper it draws them with.
type Type struct {
	Shaper *text.Shaper
	Head   textdraw.TextStyle // TitleSmall: family names on the picks board
	Label  textdraw.TextStyle // LabelLarge: section labels, the pair's "Aa"
	Body   textdraw.TextStyle // BodyMedium: a cell's names
	Small  textdraw.TextStyle // BodySmall: rules, step numbers, captions
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

// isDark reports whether a palette is the dark side of its pair, by the
// luminance of the surface everything else is drawn on.
func isDark(c tokens.ColorTokens) bool {
	return vgcolor.RelativeLuminance(c.Background) < 0.5
}

// schemeCounterpart is the side of the theme the window is not showing,
// derived with nothing chosen: from the brand base the palette on screen
// pins — c.Primary, which is the seed itself on the light side and the
// nearest thing to it a dark palette says about itself. The counterpart
// feeds only the inverse pair's rules.
func schemeCounterpart(c tokens.ColorTokens) tokens.ColorTokens {
	light, dark := tokens.FromSeed(c.Primary)
	if isDark(c) {
		return light
	}
	return dark
}

// PaletteSectionRows is how many rows PaletteRows returns: a heading and a
// body for the ramps, and the same pair for the picks.
const PaletteSectionRows = 4

// PaletteRows is the palette story as rows of this window's column, drawn
// in the window's own palette and typography.
func PaletteRows(p Palette, c, other tokens.ColorTokens, ty Type, dark bool) []layout.Widget {
	return palette.Rows(p.story(), c, other, ty.story(), dark)
}

// paletteHeading is a section's heading band, which the seed row above the
// story and the type ladder below it wear as well: one band for the whole
// column, so a reader meets one kind of heading between the seed and the
// ladder rather than three.
func paletteHeading(p Palette, c tokens.ColorTokens, ty Type, title, hint string) layout.Widget {
	return palette.Heading(p.story(), c, ty.story(), title, hint)
}

// fillRRect paints a rounded rectangle.
func fillRRect(gtx layout.Context, r image.Rectangle, radius int, c stdcolor.NRGBA) {
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// strokeRRect outlines a rounded rectangle, inset by half the stroke width
// so the whole line lands inside the rectangle rather than half outside it.
func strokeRRect(gtx layout.Context, r image.Rectangle, radius, width int, c stdcolor.NRGBA) {
	if width <= 0 {
		return
	}
	half := float32(width) / 2
	inner := image.Rect(r.Min.X+width/2, r.Min.Y+width/2, r.Max.X-width/2, r.Max.Y-width/2)
	path := clip.UniformRRect(inner, max(0, radius-width/2)).Path(gtx.Ops)
	defer clip.Stroke{Path: path, Width: half * 2}.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// raisedOnPage is the raise walked from the page a section stands on: the
// surface one step above the content, which is what a band, a card and a
// filled inset all fill with ([tokens.ColorTokens.RaisedOn]).
func raisedOnPage(c tokens.ColorTokens) stdcolor.NRGBA {
	return c.RaisedOn(c.SurfaceAt(tokens.Level0)).Fill
}
