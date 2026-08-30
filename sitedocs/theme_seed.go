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
// The second pass on that fix found it had shed the problem rather than
// solved it, and the rest of this file is what it asked for.
//
// Nothing this row draws carries a clause seam. [palette.FitLine] cuts
// a line at its commas and at " ·" and " /" and marks nothing when it
// does, and cuts at word boundaries with an ellipsis when it cannot; so a
// line written as one clause can only ever be cut the marked way. That is
// the whole guard, and it is structural rather than editorial: there is no
// wording of a comma that a reader can be relied on to supply back. The
// price is that every string here has to say what it says inside one
// clause, which is why they are short and why they lean on "and" where a
// comma would read better.
//
// The claim itself leads. SeedGrewFrom is the sentence the section is
// titled after, and every rule entitled to make it opens with it — a cut
// takes words off the tail, so a claim that leads is a claim that
// survives every cut the reader is shown a mark for. The previous draft
// left it unsaid on the path where it is provable, and a section called
// Palette Seed that identifies no seed is worth less than no section.
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
// below it, and the rules say so in the same clause that names it, not
// after a comma a narrow window could take off.
//
// And the pair is told apart twice over. The two colours are one hue at
// two chromas, which is a difference a reduced-chroma reader does not
// have: measured, the default pair stands at 1.00:1 luminance and four
// greyscale levels apart, which is one swatch drawn twice. So the colour
// the palette only took in is drawn inside the slot the realized colour
// fills, smaller by SeedHandedInset — the grid's own device, the one
// that stops a pinned chip reading as a tenth step, turned on the one
// distinction this row exists to draw. Size is a channel no display
// setting takes away.
package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// SeedSectionRows is how many rows seedRows returns: a heading and a body,
// the shape every section of this story has.
const SeedSectionRows = 2

// SeedHandedInset is how far inside its slot the swatch of a colour the
// palette only took in is drawn, leaving the full slot to the colour the
// palette realized. It is [palette.RampPinInset] doing the grid's job:
// there, keeping a pinned chip from reading as a tenth step; here,
// keeping two colours of one hue from reading as one swatch drawn twice.
// Four rather than the grid's three because the swatch it insets is the
// larger shape, and the ring has to be a ring at 1x.
const SeedHandedInset unit.Dp = 4

// What the section says about itself, as [palette.HintSep] clauses so a
// narrow window drops it a clause at a time. A caption is a legend, not a
// claim under a swatch: [palette.FitHint] drops whole clauses and each of
// these stands alone.
//
// The derivation clauses say the hue and the chroma come from different
// places because they do, and a caption claiming otherwise is disproved
// by the cell under it: #6750A4 and #723AD4 measure the same OKLCh hue to
// within 0.03° and differ in chroma by 68%. They say "the accent" and not
// "every role" for the same reason — the neutral ramp carries no hue at
// all and the four status roles are anchored to fixed hues the seed may
// only tint. And they stop there rather than going on about ramps and
// bases: the two bands under this one caption themselves, and a caption
// that summarizes its neighbours is both redundant and, for the bases,
// wrong — they are pinned at several depths, not one.
const (
	SeedLabel = "Palette Seed"
	// SeedHintPair is the legend for the size the cells are drawn at, and
	// leads the caption where there are two of them — the ramps' caption
	// leads with the legend for its dot the same way.
	SeedHintPair = "the smaller swatch is the colour picked"
	// The derivation, one clause a fact.
	SeedHintHue    = "the seed sets the accent hue"
	SeedHintChroma = "the palette's own dial sets its chroma"
	SeedHintStatus = "it only tints the status hues"
)

// The names over the cells, in the vocabulary the picks board below
// already uses: it calls the realized colour "the seed, lifted", so the
// two cells are the seed and the seed lifted. The board's comma is not
// carried across — a name is cut by the same [palette.FitLine] the rules
// are, and "Seed, lifted" cuts to "Seed", which is the other cell's name
// over the other cell's colour.
//
// The name is also where the dark scheme's disclosure lives, and it is
// there for a reason a rule cannot serve. A cell draws three lines, and
// [palette.FitLine] cuts each of them on its own; a fact on the name line
// and a fact on the rule line are therefore shed at two different widths,
// while two facts on one line are shed together. This row has two facts
// that must both survive — which colour the palette grew from, and that
// a dark scheme does not draw it — so they are put on two lines. The
// rule opens with the claim; the name carries the scheme. The name is
// the shorter line in the larger face, so it is the one that survives
// furthest: SeedLiftedNameDark holds to a window about 190px narrower
// than its rule does.
//
// SeedName is only ever over a colour somebody picked. Where the row
// cannot prove a pick it does not guess at one — both unproven cases are
// named for the token on screen instead.
const (
	SeedName       = "Seed"
	SeedLiftedName = "Lifted seed"
	// SeedLiftedNameDark names the same colour in the scheme that does not
	// draw it, in the words the picks board names a colour from across the
	// pair in: the other side's, and pinned there rather than here.
	SeedLiftedNameDark = "Lifted seed the light scheme pins"
	// SeedPinName is what the cell is called where the row could not prove
	// a seed at all: the token it is actually showing, named for itself.
	SeedPinName = "Primary"
)

// The rules under the names. Every one of them is one clause — no comma,
// no " ·", no " /" — so [palette.FitLine] has no unmarked cut to make on
// any of them, and every cut a reader is shown ends in an ellipsis.
const (
	// SeedGrewFrom is the claim the section is titled after. Every rule
	// that can prove it opens with it, so no cut can take it off.
	SeedGrewFrom = "the colour this palette grew from"

	// SeedPickRule sits under the colour handed to the derivation, in the
	// cell whose neighbour carries what the derivation made of it. It is
	// the one rule here that may not open with SeedGrewFrom: this is the
	// colour the palette did not grow from.
	SeedPickRule = "the colour picked"

	// SeedLiftedRule and SeedLiftedRuleDark sit under the realized colour.
	// They differ because the fact differs: a light scheme pins this exact
	// colour as its Primary base — the chip at the end of the Primary row
	// below is these very bytes — while a dark scheme pins a re-toned one,
	// so in dark the swatch is a colour the palette on screen never draws.
	// The dark one says so inside the clause that names it.
	SeedLiftedRule     = SeedGrewFrom + " and pins as its Primary base"
	SeedLiftedRuleDark = SeedGrewFrom + " before this scheme re-toned it"

	// SeedKeptRule and SeedKeptRuleDark are the one-cell case: the dial
	// left the pick alone, so the colour picked and the colour the palette
	// grew from are one colour and there is nothing to tell apart.
	SeedKeptRule = SeedGrewFrom + " and the colour picked and its Primary base"
	// The dark one names no scheme where the light one names none either:
	// it is already the longest line the row draws, and the room a narrow
	// window leaves is the budget every line here is written to.
	SeedKeptRuleDark = SeedGrewFrom + " and the colour picked before it was re-toned"

	// SeedFromBase: no candidate matched, but a light scheme pins the
	// colour it grew from, so the row reads it off the base. Whether the
	// dial moved the pick is not recoverable, and the rule claims nothing
	// about it.
	SeedFromBase = SeedGrewFrom + " read off the Primary base it pins"

	// SeedNotHeld: no candidate matched and the scheme is dark, which pins
	// its accent at a fixed depth rather than at the seed's. The swatch is
	// that pin, and the rule says so and denies the rest — the one rule
	// here with no claim to SeedGrewFrom and the only one that has to say
	// out loud that it has none.
	SeedNotHeld = "the Primary base this scheme pins and not the colour the pair grew from"
)

// seedCell is one colour the row shows: the colour, what to call it, what
// it is, and whether the palette only took it in — which is drawn, as the
// smaller swatch. The value is written out from the colour itself.
type seedCell struct {
	col      stdcolor.NRGBA
	name     string
	rule     string
	handedIn bool
}

// seedRows is the head of the palette story: the band, and under it the
// colour the palette grew from — with the colour picked beside it
// wherever those are two colours.
func seedRows(p Palette, c tokens.ColorTokens, ty Type, seed stdcolor.NRGBA) []layout.Widget {
	cells := seedCells(c, seed)
	return []layout.Widget{
		paletteHeading(p, c, ty, SeedLabel, seedHint(cells)),
		palette.Body(c, seedBody(p, c, ty, cells)),
	}
}

// seedHint is the caption for the cells actually drawn. The legend for
// the two sizes is only true where two cells are drawn, so it is only
// said there.
func seedHint(cells []seedCell) string {
	clauses := []string{SeedHintHue, SeedHintChroma, SeedHintStatus}
	if len(cells) > 1 {
		clauses = append([]string{SeedHintPair}, clauses...)
	}
	return strings.Join(clauses, palette.HintSep)
}

// seedCells is what the row has to say, as cells. seed is a candidate,
// not an assertion — the palette on screen is asked whether it is that
// seed's before the row names it one.
func seedCells(c tokens.ColorTokens, seed stdcolor.NRGBA) []seedCell {
	dark := isDark(c)
	if grown, ok := grownFrom(c, seed); ok {
		if grown == seed {
			return []seedCell{{col: grown, name: SeedName,
				rule: schemeRule(SeedKeptRule, SeedKeptRuleDark, dark)}}
		}
		return []seedCell{
			{col: seed, name: SeedName, rule: SeedPickRule, handedIn: true},
			{col: grown, name: schemeRule(SeedLiftedName, SeedLiftedNameDark, dark),
				rule: schemeRule(SeedLiftedRule, SeedLiftedRuleDark, dark)},
		}
	}
	if dark {
		return []seedCell{{col: c.Primary, name: SeedPinName, rule: SeedNotHeld}}
	}
	// Named for the token and not for the seed: this is the colour the
	// palette grew from, but which colour was picked to get it is not
	// recoverable, and "Seed" over here would be the pick's name over a
	// colour that may never have been picked.
	return []seedCell{{col: c.Primary, name: SeedPinName, rule: SeedFromBase}}
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
		h := gtx.Dp(palette.PickPairH)
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

// drawSeedCell draws one cell in the slot it was given, the way the picks
// board draws a paired one: the colour, and a block of three lines beside
// it — the name, the value, and what the colour is.
//
// The colour sits in a slot the width of a picks swatch whichever cell
// this is, and the lines start off the slot rather than off the colour,
// so two cells keep one text column however their swatches are drawn.
func drawSeedCell(gtx layout.Context, p Palette, c tokens.ColorTokens, ty Type, cell seedCell, r image.Rectangle) {
	if r.Dx() <= 0 {
		return
	}
	sw, sh := min(gtx.Dp(palette.PickSwatchW), r.Dx()), min(gtx.Dp(palette.PickSwatchH), r.Dy())
	top := r.Min.Y + (r.Dy()-sh)/2
	slot := image.Rect(r.Min.X, top, r.Min.X+sw, top+sh)
	box := slot
	// The second channel: a colour the palette only took in is drawn
	// inside the slot the colour it realized fills whole.
	if cell.handedIn {
		if in := slot.Inset(gtx.Dp(SeedHandedInset)); !in.Empty() {
			box = in
		}
	}
	radius := gtx.Dp(InnerR) / 2
	fillRRect(gtx, box, radius, cell.col)
	// The same frame every swatch on this page wears — see [palette.EdgeIn]
	// — and here for the reason it is there: a colour near the tone of the
	// ground it stands on has no boundary of its own.
	strokeRRect(gtx, box, radius, gtx.Dp(Hairline), palette.EdgeIn(c))

	lines := slot.Max.X + gtx.Dp(palette.PickGap)
	if lines >= r.Max.X {
		return
	}
	room := r.Max.X - lines
	title, rule := gtx.Dp(palette.PickTitleH), gtx.Dp(palette.PickRuleH)
	block := min(title+2*rule, r.Dy())
	y := r.Min.Y + (r.Dy()-block)/2
	textdraw.FillText(gtx, ty.Shaper, ty.Body,
		image.Rect(lines, y, r.Max.X, y+title), 0, 0.5, p.Text,
		palette.FitLine(gtx, ty.Shaper, ty.Body, cell.name, room))
	y += title
	for _, line := range []string{hexOf(cell.col), cell.rule} {
		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(lines, y, r.Max.X, y+rule), 0, 0.5, p.Muted,
			palette.FitLine(gtx, ty.Shaper, ty.Small, line, room))
		y += rule
	}
}

// hexOf writes a colour the way a stylesheet writes one: uppercase, and
// with no alpha, since every colour this row draws is opaque.
func hexOf(c stdcolor.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
