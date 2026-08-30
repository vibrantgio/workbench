// The seed section: the colour the rest of the palette was grown from, at the
// head of the palette story it explains.
//
// The row itself is the shared story's — [palette.SeedCells] is the two-cell
// rule, [palette.SeedRows] draws the band and the cells, and the names and
// rules they carry are that package's constants, written to be cut without
// ever shedding a claim. What is here is what this window knows and the story
// cannot: whether anything is picked at all, and whether the palette on screen
// is the pick's.
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
// # Why the row checks before it names
//
// A dark palette does not carry its seed. The light Primary base is the seed
// realized and the dark side is a projection through it, re-toned to the dark
// scale's own depth, so in a dark scheme the realized colour is a colour the
// palette on screen draws nowhere. The story says so in the same clause that
// names it, not after a comma a narrow window could take off.
//
// And what the row shows is the pick, checked against the palette it is
// standing on rather than assumed: this window derives its pair from the
// chosen candidate, so a seed's pair is reproducible from the seed, and
// [grownFrom] reproduces it before anything is named. Where the check fails
// the row falls back to the one thing it knows first-hand — which colour was
// clicked — and claims nothing about what grew from it.
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
	stdcolor "image/color"

	"gioui.org/layout"

	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/theme/tokens"
)

// The two states the shared row has no words for, because neither is a fact
// about a derivation: this window can be standing on a palette its pick did
// not make, and it can be standing there before anything is picked at all.
//
// Both are one clause, like every line the row draws — [palette.FitLine] cuts
// at a comma and marks nothing when it does, so a line with no seam in it can
// only ever be cut the marked way.
const (
	// SeedHintNone is the caption with nothing picked: what this bar will hold,
	// said where it will hold it, rather than a description of a derivation
	// that is not on screen.
	SeedHintNone = "the colour a palette here is grown from"

	// SeedNoneName is the row with nothing picked. It is a name for the state
	// and not for a colour, because there is no colour.
	SeedNoneName = "No seed picked"
	// SeedNoneRule is what will be here, and no claim about what is on screen
	// now.
	SeedNoneRule = "this row names the colour once one is picked"

	// SeedUnprovenRule is the pick the palette on screen did not check out
	// against — a state no build ships, since this window derives its pair from
	// the pick. It says the one thing the window knows first-hand and denies
	// the rest, rather than crediting a palette to a colour that did not make
	// it.
	SeedUnprovenRule = "the colour picked and not the seed of the palette on screen"
)

// SeedRows is the head of the palette story: the band, and under it the colour
// the palette grew from — with the colour picked beside it wherever those are
// two colours, and a line saying so wherever there is neither.
//
// picked is whether a candidate is chosen, which this window knows directly:
// the seed is the swatch that was clicked, and no colour has to be inferred
// from the palette to name it.
func SeedRows(p Palette, c tokens.ColorTokens, ty Type, seed stdcolor.NRGBA, picked bool) []layout.Widget {
	cells := seedCells(c, seed, picked)
	return palette.SeedRows(p.story(), c, ty.story(), cells, seedHint(cells))
}

// seedHint is the caption for the cells actually drawn. The derivation is only
// described where there is a derivation on screen to describe; the rest of the
// sizing is the story's own.
func seedHint(cells []palette.SeedCell) string {
	if len(cells) == 1 && cells[0].WordsOnly {
		return SeedHintNone
	}
	return palette.SeedHint(cells)
}

// seedCells is what the row has to say, as cells. seed is the chosen
// candidate; the palette on screen is asked whether it is that seed's before
// the row credits it with anything, and only a palette that checks out is
// handed to the story's own rule.
func seedCells(c tokens.ColorTokens, seed stdcolor.NRGBA, picked bool) []palette.SeedCell {
	if !picked {
		return []palette.SeedCell{{Name: SeedNoneName, Rule: SeedNoneRule, WordsOnly: true}}
	}
	grown, ok := grownFrom(c, seed)
	if !ok {
		return []palette.SeedCell{{Col: seed, Name: palette.SeedName, Rule: SeedUnprovenRule}}
	}
	return palette.SeedCells(seed, grown, isDark(c), palette.SeedLiftedNameDark)
}

// grownFrom is the colour a palette grew from when the palette is this seed's,
// checked rather than assumed: [Model.Pair] derives both sides from one seed,
// so c is that seed's only if it is one of the two token sets byte for byte.
// The colour returned is the light scheme's pinned base — the seed realized,
// which is what every ramp and every base downstream was measured off.
//
// It stays here rather than in the story because the sets to check are a fact
// about this window: two, because this window derives exactly two, and no
// alpha to normalize because the candidate is a colour read out of an image.
// A window inferring a seed through a theme it did not derive has a different
// answer to both, and a shared check parameterised over them would be a
// function whose whole body is its parameters.
func grownFrom(c tokens.ColorTokens, seed stdcolor.NRGBA) (stdcolor.NRGBA, bool) {
	light, dark := tokens.FromSeed(seed)
	if c == light || c == dark {
		return light.Primary, true
	}
	return stdcolor.NRGBA{}, false
}
