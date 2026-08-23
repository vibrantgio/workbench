// theme_seed.go opens the Theme tab's palette story with the one colour
// everything under it is grown from. The ramps, the picks and the bases
// are all derivations; until this row the tab showed the derivations and
// never the input.
//
// The row is drawn in the palette section's own vocabulary — the same
// heading band, the same body ground, cells the size of a picks-board
// cell with the same swatch, the same hairline frame and the same
// name-over-rules block — so it reads as the first cells of the story
// rather than as a second design.
//
// Honesty is the whole of the difficulty, and it shaped the layout.
//
// A palette is not always grown from the colour somebody picked.
// tokens.liftSeed realizes the light primary base at the picked colour's
// own hue and depth with the accent chroma dial applied, so a pick under
// the dial comes back more chromatic than it was handed over — the
// canonical #6750A4 grows a palette from #723AD4 — and it is the
// realized colour, not the pick, that every accent ramp and base is
// measured off. Which means the row has two colours to tell apart, and
// the first draft told them apart inside one sentence. A fresh-eyes pass
// killed that draft: the sentence's only clause boundary sat between the
// two colours, so a narrow window cut it to "the colour this palette
// grew from — #6750A4 picked" with nothing marking the cut, which is the
// one claim this file exists to never make. Two colours are therefore
// two cells, each with its own swatch and its own name, and no line
// carries a relation a cut can invert.
//
// The other half is the scheme. A dark palette does not carry its seed:
// the light primary base is the seed realized, and the derivation is a
// projection through it, which is what lets theme/export recover a whole
// pair from one colour; a dark base is re-toned to the dark scale's
// step-700 depth and the seed's own depth is gone. So the row is handed
// a candidate — the brand this user kept, else the palette default — and
// checks it against the palette on screen before naming it: FromSeed of
// a true seed reproduces the very tokens being drawn. Where the check
// fails, which is a window wearing an OS accent nobody told this app
// about, the row says what it can prove and no more.
//
// A dark scheme therefore shows a colour that is nowhere in the ramps
// below it, and the rules say so rather than letting a reader hunt for
// it: in a light scheme the lifted seed is the base that scheme pins,
// and in a dark one it is the base the *other* scheme pins.
package main

import (
	"fmt"
	"image"
	stdcolor "image/color"

	"gioui.org/layout"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// SeedSectionRows is how many rows seedRows returns: a heading and a body,
// the shape every section of this story has.
const SeedSectionRows = 2

// What the section says about itself. SeedHint is the derivation in one
// sentence, written as HintSep clauses like the other two captions so a
// narrow window drops it a clause at a time (see fitHint).
//
// It says the hue and the chroma come from different places because they
// do, and a caption claiming otherwise is disproved by the cell under it:
// #6750A4 and #723AD4 measure the same OKLCh hue to within 0.03° and
// differ in chroma by 68%. It says "the accent" and not "every role"
// for the same reason — the neutral ramp carries no hue at all and the
// four status roles are anchored to fixed hues the seed may only tint.
// And it stops there rather than going on about ramps and bases: the two
// bands under this one caption themselves, and a caption that summarizes
// its neighbours is both redundant and, for the bases, wrong — they are
// pinned at several depths, not one.
const (
	SeedLabel = "Palette Seed"
	SeedHint  = "the seed sets the accent hue · the palette's own dial sets its chroma · the status hues it only tints"
)

// The names over the cells, in the vocabulary the picks board below
// already uses: it calls the realized colour "the seed, lifted", so the
// two cells are the seed and the seed lifted, and one word covers both
// tellings of the same operation.
const (
	SeedName       = "Seed"
	SeedLiftedName = "Seed, lifted"
	// SeedPinName is what the cell is called where the row could not prove
	// a seed at all: the token it is actually showing, named for itself.
	SeedPinName = "Primary"
)

// The rules under the names. Every one of them is written so that its
// leading clause is true on its own, because fitLine cuts a rule at its
// commas and marks nothing when it does.
const (
	// SeedPickRule sits under the colour handed to the derivation, in the
	// cell whose neighbour carries what the derivation made of it.
	SeedPickRule = "the colour picked"
	// SeedLiftedRule and SeedLiftedRuleDark sit under the realized colour.
	// They differ because the fact differs: a light scheme pins this exact
	// colour as its Primary base — the chip at the end of the Primary row
	// below is these very bytes — while a dark scheme pins a re-toned one,
	// so in dark the swatch is a colour the palette on screen never draws.
	SeedLiftedRule     = "the pick at the palette's accent chroma, and the base this scheme pins"
	SeedLiftedRuleDark = "the pick at the palette's accent chroma, pinned by the light scheme and re-toned by this one"
	// SeedKeptRule and SeedKeptRuleDark are the one-cell case: the dial
	// left the pick alone, so the colour picked and the colour the palette
	// grew from are one colour and there is nothing to tell apart.
	SeedKeptRule     = "the colour picked, and the base this scheme pins"
	SeedKeptRuleDark = "the colour picked, pinned by the light scheme and re-toned by this one"
	// SeedFromBase: no candidate matched, but a light scheme pins the
	// colour it grew from, so the row reads it off the base. Whether the
	// dial moved the pick is not recoverable, and the rule claims nothing
	// about it.
	SeedFromBase = "the colour this palette grew from, read off the base it pins"
	// SeedNotHeld: no candidate matched and the scheme is dark, which pins
	// its accent at a fixed depth rather than at the seed's. The swatch is
	// that pin, and the rule says so rather than calling it the seed.
	SeedNotHeld = "the accent this scheme pins, not the colour the pair grew from — a dark palette does not carry it"
)

// seedCell is one colour the row shows: the colour, what to call it, and
// what it is. The value is written out from the colour itself.
type seedCell struct {
	col  stdcolor.NRGBA
	name string
	rule string
}

// seedRows is the head of the palette story: the band, and under it the
// colour the palette grew from — with the colour picked beside it
// wherever those are two colours.
func seedRows(p Palette, c tokens.ColorTokens, ty Type, seed stdcolor.NRGBA) []layout.Widget {
	cells := seedCells(c, seed)
	return []layout.Widget{
		paletteHeading(p, c, ty, SeedLabel, SeedHint),
		paletteBody(c, seedBody(p, c, ty, cells)),
	}
}

// seedCells is what the row has to say, as cells. seed is a candidate,
// not an assertion — the palette on screen is asked whether it is that
// seed's before the row names it one.
func seedCells(c tokens.ColorTokens, seed stdcolor.NRGBA) []seedCell {
	dark := isDark(c)
	if grown, ok := grownFrom(c, seed); ok {
		if grown == seed {
			return []seedCell{{grown, SeedName, schemeRule(SeedKeptRule, SeedKeptRuleDark, dark)}}
		}
		return []seedCell{
			{seed, SeedName, SeedPickRule},
			{grown, SeedLiftedName, schemeRule(SeedLiftedRule, SeedLiftedRuleDark, dark)},
		}
	}
	if dark {
		return []seedCell{{c.Primary, SeedPinName, SeedNotHeld}}
	}
	return []seedCell{{c.Primary, SeedName, SeedFromBase}}
}

// schemeRule chooses the rule for the side of the pair on screen. It is
// not called pick: on this tab a pick is a colour the theme names, and
// the word is spoken for.
func schemeRule(light, dark string, isDark bool) string {
	if isDark {
		return dark
	}
	return light
}

// grownFrom reports the colour a palette grew from when the palette is
// this seed's, checked rather than assumed: a seed derives one pair per
// variant, and c is that seed's only if it is one of the four token sets
// byte for byte. The colour returned is the light scheme's pinned base —
// the seed realized, which is what every accent ramp and base downstream
// was measured off.
func grownFrom(c tokens.ColorTokens, seed stdcolor.NRGBA) (stdcolor.NRGBA, bool) {
	seed.A = 0xff // a palette is derived from an opaque colour, whatever alpha the candidate carried
	light, dark := tokens.FromSeed(seed)
	if c == light || c == dark {
		return light.Primary, true
	}
	hcLight, hcDark := tokens.FromSeedHighContrast(seed)
	if c == hcLight || c == hcDark {
		return hcLight.Primary, true
	}
	return stdcolor.NRGBA{}, false
}

// seedBody draws the cells stacked: each colour, and beside it the name
// over its value over its rule. The geometry is a paired picks-board
// cell's, taken from the same constants, so the head of the story lines
// up with the story.
func seedBody(p Palette, c tokens.ColorTokens, ty Type, cells []seedCell) func(gtx layout.Context, width int) int {
	return func(gtx layout.Context, width int) int {
		h := gtx.Dp(PickPairH)
		total := len(cells) * h
		if width <= 0 {
			return total
		}
		for i, cell := range cells {
			drawSeedCell(gtx, p, c, ty, cell, image.Rect(0, i*h, width, i*h+h))
		}
		return total
	}
}

// drawSeedCell draws one cell in the slot it was given, the way drawCell
// draws a paired one: the colour, and a block of three lines beside it —
// the name, the value, and what the colour is.
func drawSeedCell(gtx layout.Context, p Palette, c tokens.ColorTokens, ty Type, cell seedCell, r image.Rectangle) {
	if r.Dx() <= 0 {
		return
	}
	sw, sh := min(gtx.Dp(PickSwatchW), r.Dx()), min(gtx.Dp(PickSwatchH), r.Dy())
	top := r.Min.Y + (r.Dy()-sh)/2
	box := image.Rect(r.Min.X, top, r.Min.X+sw, top+sh)
	radius := gtx.Dp(InnerR) / 2
	fillRRect(gtx, box, radius, cell.col)
	// The same frame every swatch on this page wears: a colour near the
	// tone of the ground it stands on has no boundary of its own.
	strokeRRect(gtx, box, radius, gtx.Dp(Hairline), edgeIn(c))

	lines := box.Max.X + gtx.Dp(PickGap)
	if lines >= r.Max.X {
		return
	}
	room := r.Max.X - lines
	title, rule := gtx.Dp(PickTitleH), gtx.Dp(PickRuleH)
	block := min(title+2*rule, r.Dy())
	y := r.Min.Y + (r.Dy()-block)/2
	textdraw.FillText(gtx, ty.Shaper, ty.Body,
		image.Rect(lines, y, r.Max.X, y+title), 0, 0.5, p.Text,
		fitLine(gtx, ty.Shaper, ty.Body, cell.name, room))
	y += title
	for _, line := range []string{hexOf(cell.col), cell.rule} {
		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(lines, y, r.Max.X, y+rule), 0, 0.5, p.Muted,
			fitLine(gtx, ty.Shaper, ty.Small, line, room))
		y += rule
	}
}

// hexOf writes a colour the way this section's original does — the
// themer's own hexOf, uppercase, the way a stylesheet writes one. Every
// colour here is opaque, so the alpha is not written.
func hexOf(c stdcolor.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
