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

// SchemeFor resolves the colour tokens the whole window draws in: one side of
// the pair [Model.Pair] resolves, chosen by the model's answer to the
// light/dark question.
//
// Which side applies starts as the OS's decision, read off the palette the OS
// handed over rather than asked for a second time — a scheme whose background
// is dark is a dark scheme — and becomes the window's as soon as its own
// switch is pressed. A seed has two sides and both have to be reachable to
// judge it.
//
// It is the one place the window's own colours come from. Every surface the
// window is themed in is drawn from what this returns — the switch that
// changes the side included, which is what stops a control from wearing one
// theme while the page it stands on wears another.
func SchemeFor(os tokens.ColorTokens, m Model) tokens.ColorTokens {
	shown, _ := SchemePair(os, m)
	return shown
}

// SchemePair is [SchemeFor] and the side it did not return: the palette the
// window draws in, and the one on the other side of its switch.
//
// Both come out of one derivation, which is the only reason this exists. The
// window has one place that wants the side it is not showing — the inverse pair,
// which is by definition the counterpart scheme's surface and text — and asking
// [SchemeFor] and then asking again for the other side would derive the whole
// palette twice per emission to learn something the first derivation already
// knew. A pick costs one derivation, and it stays one.
func SchemePair(os tokens.ColorTokens, m Model) (shown, other tokens.ColorTokens) {
	light, dark := m.Pair(os)
	if m.Dark(os) {
		return dark, light
	}
	return light, dark
}

// Pair is both sides of the theme on screen, resolved from one seed so that
// the two sides are two sides of one thing rather than two answers.
//
// With a candidate chosen the seed is that candidate and the pair is what it
// generates — the point of the application being that a seed is judged by what
// it does to a window, not by a swatch.
//
// With nothing chosen the window is wearing the theme its own stream was
// built with, and the desktop hands over one side of it per frame. That side
// is returned exactly as it arrived, so following the desktop stays following
// the desktop, down to whatever the accessibility preferences did to it. The
// other side is the side the desktop never sends, and it is derived from the
// seed the stream was built from: the brand kept when the window opened, or —
// with nothing kept, the stream following the desktop's accent — the brand
// base the palette on screen pins, which is the seed itself on the light side
// and the nearest thing to it a dark palette says about itself.
//
// Deriving it rather than reaching for the theme's default pair is the whole
// of it: a default pair is a different colour, and reaching for it put the
// window's own controls in one theme and its page in another on the first
// press of the switch.
func (m Model) Pair(os tokens.ColorTokens) (light, dark tokens.ColorTokens) {
	if seed, ok := m.Seed(); ok {
		return tokens.FromSeed(seed)
	}
	seed := m.Opened
	if seed.A == 0 {
		seed = os.Primary
	}
	light, dark = tokens.FromSeed(seed)
	if isDark(os) {
		return light, os
	}
	return os, dark
}

// isDark reports whether a palette is the dark side of its pair, by the
// luminance of the surface everything else is drawn on.
func isDark(c tokens.ColorTokens) bool {
	return vgcolor.RelativeLuminance(c.Background) < 0.5
}

// edgeFloor is WCAG 1.4.11's contrast floor for a graphic that carries
// meaning without being text — 3:1. A swatch's frame is exactly that: it is
// the whole of what says where a pale colour ends and the card behind it
// begins, so it is not decoration and owes its ground this much.
const edgeFloor = 3.0

// Palette is the application's view of the colour tokens: every colour it
// draws with, named for what it draws.
type Palette struct {
	Backdrop  stdcolor.NRGBA // window background
	Surface   stdcolor.NRGBA // the picture's mat and the candidate cards
	Divider   stdcolor.NRGBA // the drop zone's outline at rest
	CardEdge  stdcolor.NRGBA // a candidate card's outline at rest
	Edge      stdcolor.NRGBA // the frame round a swatch, the heaviest edge in the window
	Text      stdcolor.NRGBA // headings and the chosen candidate's label
	Muted     stdcolor.NRGBA // hints, hex values, unchosen labels
	Accent    stdcolor.NRGBA // the chosen candidate's ring, the hover highlight
	OnAccent  stdcolor.NRGBA // text over Accent
	Selection stdcolor.NRGBA // the chosen candidate's card fill
	Problem   stdcolor.NRGBA // a drop that produced nothing
}

// PaletteFrom resolves the palette in the token vocabulary: the pinned
// Background, Primary and Error, the neutral ramp's surface, border and
// text steps, and the primary ramp's container step for the selection fill.
//
// The container step is read off the side the palette is on, and it is the
// one place a single step number will not do. Every ramp runs dark to light
// in both schemes, and the ground moves with them: on the light side step 100
// is a pale tint standing just off a near-white page, while on the dark side
// the same step is very nearly the dark page itself — a selection fill nobody
// can see, on the one thing in the window that has to be seen.
func PaletteFrom(c tokens.ColorTokens) Palette {
	container := c.Ramps.Primary.Step(100)
	if isDark(c) {
		container = c.Ramps.Primary.Step(300)
	}
	return Palette{
		Backdrop: c.Background,
		// The mat and the candidate cards: filled insets lying on the
		// window's page, and the raised storey is lighter than that page in
		// both schemes — #F8F8F8 on paper, #222222 on slate. A ramp index is
		// not a storey: neutral 200 is #E8E8E8 under a #F6F6F6 page, which
		// sinks a card into the desk instead of raising it.
		//
		// On paper the storey is a whisper the fill cannot carry alone, and
		// this palette is built for that: CardEdge is what makes a card an
		// object, and Edge is derived against this very fill so a near-white
		// swatch on a near-white card still has a boundary.
		Surface: c.SurfaceAt(tokens.Level1),
		Divider: c.Ramps.Neutral.Step(300),
		// A card sits on a surface only a shade off the window's own — the
		// neutral steps are close together by design — so the card's own
		// edge, not its fill, is what makes it an object. It is drawn a rung
		// stronger than the page's dividers for exactly that reason.
		CardEdge: c.Ramps.Neutral.Step(400),
		// The heaviest edge, and it is on swatches rather than on cards for
		// one reason: a swatch can be any colour a style or a photograph
		// contains, and plenty of both are near-white. A near-white swatch on
		// a near-white card has no boundary of its own, and without one it
		// does not read as a pale colour somebody chose — it reads as a card
		// that failed to finish drawing. The weight has to beat the two
		// near-whites it stands between, which the card weight does not.
		//
		// So it is derived rather than named: the neutral rung the ramp
		// measures as reaching 3:1 against the card the swatches lie on,
		// which is Surface — the level-1 storey. Named at step 500 it
		// measured 2.35:1 there in the light scheme and 5.94:1 in the dark,
		// one line of code meaning two different weights.
		Edge: c.MarkOn(tokens.RoleNeutral, c.SurfaceAt(tokens.Level1), edgeFloor),
		Text: c.Text,
		// The quiet register: hints and hex values, and the chrome in the
		// title row that stands under the window's own name. It is a step
		// short of the text ink rather than a wash — measured, it clears the
		// body-text floor against either page by a margin, which is what lets
		// a control wear it and still be read.
		Muted:     c.Ramps.Neutral.Step(700),
		Accent:    c.Primary,
		OnAccent:  c.OnPrimary,
		Selection: container,
		Problem:   c.Error,
	}
}

// Type is the application's view of the theme's Typography: the roles it
// draws directly, as textdraw styles, plus the theme's cached shaper. The
// application builds no shaper and bundles no font of its own.
type Type struct {
	Shaper *text.Shaper
	Title  textdraw.TextStyle // TitleLarge: the drop zone's invitation
	Head   textdraw.TextStyle // TitleSmall: the window's own name in the title row
	Label  textdraw.TextStyle // LabelLarge: section labels, the pair's "Aa"
	Body   textdraw.TextStyle // BodyMedium: the file name, the hint line
	Small  textdraw.TextStyle // BodySmall: hex values and shares
	// Role is LabelLarge as the theme states it, for the one control here
	// drawn by a published component rather than by this application: the
	// component lays the role out itself, in the line box the role names.
	Role tokens.TextStyle
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper: t.Shaper(),
		Title:  textStyle(t.TitleLarge),
		Head:   textStyle(t.TitleSmall),
		Label:  textStyle(t.LabelLarge),
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
