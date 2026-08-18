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
		Surface:  c.Ramps.Neutral.Step(200),
		Divider:  c.Ramps.Neutral.Step(300),
		// A card sits on a surface only a shade off the window's own — the
		// neutral steps are close together by design — so the card's own
		// edge, not its fill, is what makes it an object. It is drawn a rung
		// stronger than the page's dividers for exactly that reason.
		CardEdge:  c.Ramps.Neutral.Step(400),
		Text:      c.Text,
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
	Label  textdraw.TextStyle // LabelLarge: section labels, the pair's "Aa"
	Body   textdraw.TextStyle // BodyMedium: the file name, the hint line
	Small  textdraw.TextStyle // BodySmall: hex values and shares
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper: t.Shaper(),
		Title:  textStyle(t.TitleLarge),
		Label:  textStyle(t.LabelLarge),
		Body:   textStyle(t.BodyMedium),
		Small:  textStyle(t.BodySmall),
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
