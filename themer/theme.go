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

// SchemeFor resolves the colour tokens the whole window draws in. Until a
// candidate is chosen that is the OS palette unchanged; from then on it is
// the palette the chosen candidate generates — the point of the application
// being that a seed is judged by what it does to a window, not by a swatch.
//
// Which side of the generated pair applies starts as the OS's decision, read
// off the palette the OS handed over rather than asked for a second time — a
// scheme whose background is dark is a dark scheme — and becomes the window's
// as soon as its own switch is pressed. A seed has two sides and both have to
// be reachable to judge it.
//
// The one awkward corner is an override with nothing chosen: there is no
// generated pair to take the other side of, so the theme's own default pair
// stands in. It is the only other pair there is, and it is what the window
// would show a moment later anyway.
func SchemeFor(os tokens.ColorTokens, m Model) tokens.ColorTokens {
	dark := m.Dark(os)
	seed, ok := m.Seed()
	if !ok {
		switch {
		case m.Scheme == FollowOS:
			return os
		case dark:
			return tokens.DefaultDark
		default:
			return tokens.DefaultLight
		}
	}
	light, darkTokens := tokens.FromSeed(seed)
	if dark {
		return darkTokens
	}
	return light
}

// isDark reports whether a palette is the dark side of its pair, by the
// luminance of the surface everything else is drawn on.
func isDark(c tokens.ColorTokens) bool {
	return vgcolor.RelativeLuminance(c.Background) < 0.5
}

// Palette is the application's view of the colour tokens: every colour it
// draws with, named for what it draws.
type Palette struct {
	Backdrop  stdcolor.NRGBA // window background
	Surface   stdcolor.NRGBA // the picture's mat and the candidate cards
	Divider   stdcolor.NRGBA // the drop zone's outline at rest
	CardEdge  stdcolor.NRGBA // a candidate card's outline at rest
	Edge      stdcolor.NRGBA // the frame round a swatch, at the strong border weight
	Text      stdcolor.NRGBA // headings and the chosen candidate's label
	Muted     stdcolor.NRGBA // hints, hex values, unchosen labels
	Accent    stdcolor.NRGBA // the chosen candidate's ring, the hover highlight
	OnAccent  stdcolor.NRGBA // text over Accent
	Selection stdcolor.NRGBA // the chosen candidate's card fill
	Outline   stdcolor.NRGBA // the boundary of a control standing on the page
	Problem   stdcolor.NRGBA // a drop that produced nothing
}

// EdgeContrast is what an outline has to reach against the surface behind it
// before it reads as the boundary of an object rather than as a tint. It is
// the standing floor for the visual boundary of a control, which is what the
// outline this file resolves is drawn round.
const EdgeContrast = 3.0

// outlineOn is the step of a ramp an outline is drawn at: the first one
// standing EdgeContrast clear of the ground behind it.
//
// The step is measured rather than pinned, because the same step number is not
// the same distance from the ground in the two schemes. The primary ramp's mid
// step stands better than six to one off a dark page and under three to one off
// a light one — so an outline pinned to it is a boundary under the moon and a
// tint under the sun, which is precisely how the way back came out: an object
// on one side of the switch and a floating label on the other. Choosing by
// measurement is what gives the two schemes the same treatment instead of the
// same number.
//
// The walk runs from the tinted end upward and stops at the first step that
// clears, so the outline is the quietest one that is still a boundary. The last
// step is the fallback and cannot fail to be reached in practice: a ramp whose
// darkest end does not stand three to one off its own ground is not a ramp this
// application could draw anything on.
func outlineOn(ramp tokens.Ramp, ground stdcolor.NRGBA) stdcolor.NRGBA {
	for n := 100; n <= 900; n += 100 {
		if step := ramp.Step(n); vgcolor.ContrastRatio(step, ground) >= EdgeContrast {
			return step
		}
	}
	return ramp.Step(900)
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
		Surface:  c.Ramps.Neutral.Step(200),
		Divider:  c.Ramps.Neutral.Step(300),
		// A card sits on a surface only a shade off the window's own — the
		// neutral steps are close together by design — so the card's own
		// edge, not its fill, is what makes it an object. It is drawn a rung
		// stronger than the page's dividers for exactly that reason.
		CardEdge: c.Ramps.Neutral.Step(400),
		// The strong border step, and it is on swatches rather than on cards
		// for one reason: a swatch can be any colour a style or a photograph
		// contains, and plenty of both are near-white. A near-white swatch on
		// a near-white card has no boundary of its own, and without one it
		// does not read as a pale colour somebody chose — it reads as a card
		// that failed to finish drawing. The weight has to beat the two
		// near-whites it stands between, which the card weight does not.
		Edge:      c.Ramps.Neutral.Step(500),
		Text:      c.Text,
		Muted:     c.Ramps.Neutral.Step(700),
		Accent:    c.Primary,
		OnAccent:  c.OnPrimary,
		Selection: container,
		Outline:   outlineOn(c.Ramps.Primary, c.Background),
		Problem:   c.Error,
	}
}

// Type is the application's view of the theme's Typography: the roles it
// draws directly, as textdraw styles, plus the theme's cached shaper. The
// application builds no shaper and bundles no font of its own.
type Type struct {
	Shaper *text.Shaper
	Title  textdraw.TextStyle // TitleLarge: the drop zone's invitation
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
		Label:  textStyle(t.LabelLarge),
		Body:   textStyle(t.BodyMedium),
		Small:  textStyle(t.BodySmall),
		Role:   t.LabelLarge,
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
