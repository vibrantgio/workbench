// The seed section: the colour the rest of the palette was grown from, at the
// head of the palette story it explains.
//
// # Why the window shows this at all
//
// Everything under this row is a derivation. The ramps are generated, the
// picks are taken off the ramps, the bases are pinned against them, and the
// whole page below is drawn in what came out. Until this row the tab showed
// every one of those and never the one colour they were all a function of —
// which is the colour the person at this window actually chose, and the only
// thing on screen they have any say over.
//
// It is drawn in the tab's own vocabulary: the same heading band the ramps and
// the picks stand under, the same body ground, a swatch the size of a picks
// cell's with the same corner and the same frame, and the same name-over-rules
// block beside it. It is the head of one story rather than a second design.
//
// # Why two colours and not one sentence
//
// A palette is not always grown from the colour somebody picked. The
// derivation realizes the light Primary base at the picked colour's own hue
// and depth with the palette's accent chroma dial applied, so a pick comes
// back more chromatic than it was handed over, and it is the realized colour —
// not the pick — that every ramp and every base below is measured off. The
// picks board already has a word for it: it calls the light Primary "the seed,
// lifted".
//
// So the row has two colours to tell apart, and it tells them apart as two
// cells rather than inside one sentence. A sentence relating them has its only
// clause boundary between the two, so a narrow window cuts it to a line naming
// one colour and claiming the other's fact, with nothing marking the cut —
// which is the one claim this file exists never to make. Two cells, each with
// its own swatch and its own name, and no line carrying a relation a cut can
// invert.
//
// Every line here is written as one clause for the same reason. [fitLine] cuts
// a line at its commas and at " ·" and " /" and marks nothing when it does,
// and falls back to a word boundary with an ellipsis; a line with no clause
// seam in it can therefore only ever be cut the marked way. That is a
// structural guard rather than an editorial one, and the price is that these
// strings say what they say without a comma, which is why they lean on "and".
//
// The claim leads. [SeedGrewFrom] is the sentence this section is named after,
// and every rule entitled to make it opens with it: a cut takes words off the
// tail, so a claim at the front survives every cut a reader is shown a mark
// for.
//
// # Why the pair is told apart by size as well
//
// The two colours are one hue at two chromas, which is a difference a
// reduced-chroma reader does not have — measured, the pair the default seed
// makes stands at 1.00:1 luminance and four greyscale levels apart, which is
// one swatch drawn twice. So the colour the palette only took in is drawn
// inside the slot the realized colour fills whole, smaller by
// [SeedPickedInset]. It is the grid's own device — [RampPinInset], which stops
// a pinned chip reading as a tenth step — turned on the one distinction this
// row exists to draw. Size is a channel no display setting takes away.
//
// # Why the row checks before it names
//
// A dark palette does not carry its seed. The light Primary base is the seed
// realized and the dark side is a projection through it, re-toned to the dark
// scale's own depth, so in a dark scheme the realized colour is a colour the
// palette on screen draws nowhere. The row says so in the same clause that
// names it, not after a comma a narrow window could take off.
//
// And what the row shows is the pick, checked against the palette it is
// standing on rather than assumed: this window derives its pair from the
// chosen candidate, so a seed's pair is reproducible from the seed, and the
// row reproduces it before naming anything. Where the check fails the row
// falls back to the one thing it knows first-hand — which colour was clicked —
// and claims nothing about what grew from it.
//
// # Why there is a row before anything is picked
//
// Because a section that vanishes is a section nobody learns, and because a
// row with nothing to show has two answers and only one of them is honest.
// With no candidate chosen the window is not wearing a seed's palette at all,
// so there is no seed to show and the row says exactly that, in the place it
// will say something else the moment a swatch is clicked. Showing a colour
// here would be the window naming a seed it does not have.
//
// The window as it stands does not reach that state: the page this section is
// on is drawn only where there are candidates, and a candidate row always has
// one of them chosen. It is written the way [GalleryHintFor] writes its own
// standing case — the state is the model's to have, so the row answers for it
// rather than assuming a fact about the model that this file does not own.
package main

import (
	"image"
	stdcolor "image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// SeedSectionRows is how many rows [SeedRows] returns: a heading and a body,
// the shape every section of this column has.
const SeedSectionRows = 2

// SeedPickedInset is how far inside its slot the swatch of the colour the
// palette only took in is drawn, leaving the full slot to the colour the
// palette realized. Four rather than the grid's three, because the shape it
// insets is the larger one and the ring it leaves has to be a ring at 1x.
const SeedPickedInset unit.Dp = 4

// What the section says about itself, as [HintSep] clauses so a narrow bar
// drops it a clause at a time — see [fitHint]. A caption is a legend and not a
// claim under a swatch, and each of these stands alone.
//
// The derivation clauses say the hue and the chroma come from different places
// because they do, and the two cells under the caption are the proof. They say
// "the accent" rather than "every role" for the same reason: the neutral ramp
// carries no hue at all, and the four status roles are anchored to fixed hues
// the seed may only tint. They stop there rather than going on about ramps and
// bases, which are two bands further down under captions of their own.
const (
	SeedLabel = "Palette Seed"
	// SeedHintPair is the legend for the two sizes, and leads the caption
	// wherever there are two cells to size — the ramps' caption leads with the
	// legend for its dot the same way.
	SeedHintPair   = "the smaller swatch is the colour picked"
	SeedHintHue    = "the seed sets the accent hue"
	SeedHintChroma = "the palette's own dial sets its chroma"
	SeedHintStatus = "it only tints the status hues"
	// SeedHintNone is the caption with nothing picked: what this bar will hold,
	// said where it will hold it.
	SeedHintNone = "the colour a palette here is grown from"
)

// The names over the cells, in the vocabulary the picks board already uses: it
// calls the realized colour "the seed, lifted", so the two cells are the seed
// and the seed lifted. The board's comma is not carried across — a name is cut
// by the same [fitLine] a rule is, and "Seed, lifted" cuts to "Seed", which is
// the other cell's name over the other cell's colour.
//
// The name is also where the dark scheme's disclosure lives, and for a reason
// a rule cannot serve. A cell draws three lines and each is cut on its own, so
// a fact on the name line and a fact on the rule line are shed at two
// different widths while two facts on one line are shed together. This row has
// two facts that must both survive — which colour the palette grew from, and
// that a dark scheme does not draw it — so they stand on two lines: the rule
// opens with the claim, the name carries the scheme. The name is the shorter
// line in the larger face, so it is the one that survives furthest.
const (
	SeedName       = "Seed"
	SeedLiftedName = "Lifted seed"
	// SeedLiftedNameDark names the same colour in the scheme that does not draw
	// it, in the words the picks board names a colour from across the pair in:
	// the other side's, and pinned there rather than here.
	//
	// It is a participle and not a relative clause. The draft read "Lifted
	// seed the light scheme pins", which is grammatical with the "that"
	// dropped and which a fresh-eyes pass read as a run-on with a clause
	// missing its punctuation — and punctuation is the one repair not
	// available here, since a comma in a name is a seam [fitLine] cuts at
	// without marking. Recast as one noun phrase it needs no seam and reads
	// as one thing.
	SeedLiftedNameDark = "Lifted seed pinned in the light scheme"
	// SeedNoneName is the row with nothing picked. It is a name for the state
	// and not for a colour, because there is no colour.
	SeedNoneName = "No seed picked"
)

// The rules under the names. Every one of them is one clause — no comma, no
// " ·", no " /" — so [fitLine] has no unmarked cut to make on any of them and
// every cut a reader is shown ends in an ellipsis.
const (
	// SeedGrewFrom is the claim the section is named after. Every rule that can
	// prove it opens with it, so no cut can take it off.
	SeedGrewFrom = "the colour this palette grew from"

	// SeedPickRule sits under the colour handed to the derivation, in the cell
	// whose neighbour carries what the derivation made of it. It is the one
	// rule here that may not open with SeedGrewFrom: this is the colour the
	// palette did not grow from.
	SeedPickRule = "the colour picked"

	// SeedLiftedRule and SeedLiftedRuleDark sit under the realized colour. They
	// differ because the fact differs: a light scheme pins this exact colour as
	// its Primary base — the chip at the end of the Primary row below is these
	// very bytes — while a dark scheme pins a re-toned one, so in dark the
	// swatch is a colour the palette on screen draws nowhere.
	SeedLiftedRule     = SeedGrewFrom + " and pins as its Primary base"
	SeedLiftedRuleDark = SeedGrewFrom + " before this scheme re-toned it"

	// SeedKeptRule and SeedKeptRuleDark are the one-cell case: the dial left
	// the pick alone, so the colour picked and the colour the palette grew from
	// are one colour and there is nothing to tell apart.
	SeedKeptRule = SeedGrewFrom + " and the colour picked and its Primary base"
	// The dark one names no scheme where the light one names none either: it is
	// already the longest line the row draws, and the room a narrow window
	// leaves is the budget every line here is written to.
	SeedKeptRuleDark = SeedGrewFrom + " and the colour picked before it was re-toned"

	// SeedUnprovenRule is the pick the palette on screen did not check out
	// against — a state no build ships, since this window derives its pair from
	// the pick. It says the one thing the window knows first-hand and denies
	// the rest, rather than crediting a palette to a colour that did not make
	// it.
	SeedUnprovenRule = "the colour picked and not the seed of the palette on screen"

	// SeedNoneRule is the row with nothing picked: what will be here, and no
	// claim about what is on screen now.
	SeedNoneRule = "this row names the colour once one is picked"
)

// seedCell is one thing the row shows: a colour, what to call it, what it is,
// and whether the palette only took it in — which is drawn, as the smaller
// swatch. The hex is written out from the colour itself.
//
// wordsOnly is the cell with no colour at all, which is the row before
// anything is picked: the name and the rule, and no swatch to stand for a seed
// that is not there.
type seedCell struct {
	col        stdcolor.NRGBA
	name, rule string
	handedIn   bool
	wordsOnly  bool
}

// height is the slot this cell takes: three lines where there is a colour to
// write out, two where the cell is words.
func (c seedCell) height() unit.Dp {
	if c.wordsOnly {
		return PickCellH
	}
	return PickPairH
}

// SeedRows is the head of the palette story: the band, and under it the colour
// the palette grew from — with the colour picked beside it wherever those are
// two colours, and a line saying so wherever there is neither.
//
// picked is whether a candidate is chosen, which this window knows directly:
// the seed is the swatch that was clicked, and no colour has to be inferred
// from the palette to name it.
func SeedRows(p Palette, c tokens.ColorTokens, ty Type, seed stdcolor.NRGBA, picked bool) []layout.Widget {
	cells := seedCells(c, seed, picked)
	return []layout.Widget{
		paletteHeading(p, c, ty, SeedLabel, seedHint(cells)),
		paletteBody(c, seedBody(p, c, ty, cells)),
	}
}

// seedHint is the caption for the cells actually drawn. The legend for the two
// sizes is only true where two cells are drawn, so it is only said there, and
// the derivation is only described where there is a derivation on screen to
// describe.
func seedHint(cells []seedCell) string {
	if len(cells) == 1 && cells[0].wordsOnly {
		return SeedHintNone
	}
	clauses := []string{SeedHintHue, SeedHintChroma, SeedHintStatus}
	if len(cells) > 1 {
		clauses = append([]string{SeedHintPair}, clauses...)
	}
	return strings.Join(clauses, HintSep)
}

// seedCells is what the row has to say, as cells. seed is the chosen
// candidate; the palette on screen is asked whether it is that seed's before
// the row credits it with anything.
func seedCells(c tokens.ColorTokens, seed stdcolor.NRGBA, picked bool) []seedCell {
	if !picked {
		return []seedCell{{name: SeedNoneName, rule: SeedNoneRule, wordsOnly: true}}
	}
	dark := isDark(c)
	grown, ok := grownFrom(c, seed)
	switch {
	case !ok:
		return []seedCell{{col: seed, name: SeedName, rule: SeedUnprovenRule}}
	case grown == seed:
		return []seedCell{{col: grown, name: SeedName,
			rule: seedScheme(SeedKeptRule, SeedKeptRuleDark, dark)}}
	}
	return []seedCell{
		{col: seed, name: SeedName, rule: SeedPickRule, handedIn: true},
		{col: grown, name: seedScheme(SeedLiftedName, SeedLiftedNameDark, dark),
			rule: seedScheme(SeedLiftedRule, SeedLiftedRuleDark, dark)},
	}
}

// seedScheme chooses the wording for the side of the pair on screen. It is not
// called pick: on this tab a pick is a colour the theme names, and the word is
// spoken for.
func seedScheme(light, dark string, isDark bool) string {
	if isDark {
		return dark
	}
	return light
}

// grownFrom is the colour a palette grew from when the palette is this seed's,
// checked rather than assumed: [Model.Pair] derives both sides from one seed,
// so c is that seed's only if it is one of the two token sets byte for byte.
// The colour returned is the light scheme's pinned base — the seed realized,
// which is what every ramp and every base downstream was measured off.
func grownFrom(c tokens.ColorTokens, seed stdcolor.NRGBA) (stdcolor.NRGBA, bool) {
	light, dark := tokens.FromSeed(seed)
	if c == light || c == dark {
		return light.Primary, true
	}
	return stdcolor.NRGBA{}, false
}

// seedBody draws the cells stacked: each colour, and beside it the name over
// its value over its rule. The geometry is a paired picks cell's, taken from
// the same constants, so the head of the story lines up with the story.
func seedBody(p Palette, c tokens.ColorTokens, ty Type, cells []seedCell) func(gtx layout.Context, width int) int {
	return func(gtx layout.Context, width int) int {
		y := 0
		for _, cell := range cells {
			h := gtx.Dp(cell.height())
			if width > 0 {
				drawSeedCell(gtx, p, c, ty, cell, image.Rect(0, y, width, y+h))
			}
			y += h
		}
		return y
	}
}

// drawSeedCell draws one cell in the slot it was given, the way [drawCell]
// draws a paired one: the colour, and a block of lines beside it.
//
// The colour sits in a slot the width of a picks swatch whichever cell this
// is, and the lines start off the slot rather than off the colour, so two
// cells keep one text column however their swatches are drawn. A cell that is
// words takes no slot at all — an empty box where a swatch belongs reads as a
// swatch that failed to draw.
func drawSeedCell(gtx layout.Context, p Palette, c tokens.ColorTokens, ty Type, cell seedCell, r image.Rectangle) {
	if r.Dx() <= 0 {
		return
	}
	lines := r.Min.X
	body := []string{cell.rule}
	if !cell.wordsOnly {
		sw, sh := min(gtx.Dp(PickSwatchW), r.Dx()), min(gtx.Dp(PickSwatchH), r.Dy())
		top := r.Min.Y + (r.Dy()-sh)/2
		slot := image.Rect(r.Min.X, top, r.Min.X+sw, top+sh)
		box := slot
		// The second channel: a colour the palette only took in is drawn inside
		// the slot the colour it realized fills whole.
		if cell.handedIn {
			if in := slot.Inset(gtx.Dp(SeedPickedInset)); !in.Empty() {
				box = in
			}
		}
		radius := gtx.Dp(InnerR) / 2
		fillRRect(gtx, box, radius, cell.col)
		// The same frame every swatch in this column wears — see [edgeIn] — and
		// here for the reason it is there: a colour near the tone of the ground
		// it stands on has no boundary of its own.
		strokeRRect(gtx, box, radius, gtx.Dp(Hairline), edgeIn(c))
		lines = slot.Max.X + gtx.Dp(PickGap)
		body = []string{hexOf(cell.col), cell.rule}
	}
	if lines >= r.Max.X {
		return
	}
	room := r.Max.X - lines
	title, rule := gtx.Dp(PickTitleH), gtx.Dp(PickRuleH)
	block := min(title+len(body)*rule, r.Dy())
	y := r.Min.Y + (r.Dy()-block)/2
	textdraw.FillText(gtx, ty.Shaper, ty.Body,
		image.Rect(lines, y, r.Max.X, y+title), 0, 0.5, p.Text,
		fitLine(gtx, ty.Shaper, ty.Body, cell.name, room))
	y += title
	for _, line := range body {
		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(lines, y, r.Max.X, y+rule), 0, 0.5, p.Muted,
			fitLine(gtx, ty.Shaper, ty.Small, line, room))
		y += rule
	}
}
