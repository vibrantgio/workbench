// theme_seed.go opens the Theme tab's palette story with the one colour
// everything under it is grown from. The ramps, the picks and the bases
// are all derivations; until this row the tab showed the derivations and
// never the input.
//
// The row itself is the shared story's — [palette.SeedCells] is the
// two-cell rule, [palette.SeedRows] draws the band and the cells, and the
// names and rules they carry are that package's constants, written so that
// no line a narrow window cuts can shed a claim or invert one. The design
// rationale for all of it is in that package's comments. What is kept here
// is what belongs to this window: whether the palette on screen is a seed's
// at all, and what the row says where it is not.
//
// # Why the row checks before it names
//
// A dark palette does not carry its seed: the light primary base is the
// seed realized, and the derivation is a projection through it, which is
// what lets theme/export recover a whole pair from one colour; a dark base
// is re-toned to the dark scale's step-700 depth and the seed's own depth
// is gone. So the row is handed a candidate — the brand this user kept,
// else the palette default — and checks it against the palette on screen
// before naming it: [grownFrom] of a true seed reproduces the very tokens
// being drawn.
//
// Where the check fails, which is a window wearing an OS accent nobody told
// this app about, the row says what it can prove and no more. A light
// scheme pins the colour it grew from, so the row reads it off the base and
// claims nothing about which colour was picked to get it; a dark scheme
// pins its accent at a fixed depth rather than at the seed's, so there is
// nothing to read off and the rule says so out loud.
//
// Both of those cells are named for the token on screen rather than for a
// pick, because "Seed" is a name for a colour somebody chose and this
// window cannot prove one was.
package main

import (
	stdcolor "image/color"

	"gioui.org/layout"

	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/theme/tokens"
)

// What the row says where it cannot prove a seed. Every one of these is one
// clause — no comma, no " ·", no " /" — so [palette.FitLine] has no
// unmarked cut to make on any of them, and every cut a reader is shown ends
// in an ellipsis.
const (
	// SeedPinName is what the cell is called on both unproven paths: the
	// token it is actually showing, named for itself.
	SeedPinName = "Primary"

	// SeedFromBase: no candidate matched, but a light scheme pins the colour
	// it grew from, so the row reads it off the base. Whether the dial moved
	// the pick is not recoverable, and the rule claims nothing about it.
	SeedFromBase = palette.SeedGrewFrom + " read off the Primary base it pins"

	// SeedNotHeld: no candidate matched and the scheme is dark, which pins
	// its accent at a fixed depth rather than at the seed's. The swatch is
	// that pin, and the rule says so and denies the rest — the one rule this
	// window draws with no claim to what the palette grew from, and the only
	// one that has to say out loud that it has none.
	SeedNotHeld = "the Primary base this scheme pins and not the colour the pair grew from"
)

// seedRows is the head of the palette story: the band, and under it the
// colour the palette grew from — with the colour picked beside it wherever
// those are two colours.
func seedRows(p Palette, c tokens.ColorTokens, ty Type, seed stdcolor.NRGBA) []layout.Widget {
	cells := seedCells(c, seed)
	return palette.SeedRows(p.story(), c, ty.story(), cells, palette.SeedHint(cells))
}

// seedCells is what the row has to say, as cells. seed is a candidate, not
// an assertion — the palette on screen is asked whether it is that seed's
// before the row names it one, and only a palette that checks out is handed
// to the story's own rule.
func seedCells(c tokens.ColorTokens, seed stdcolor.NRGBA) []palette.SeedCell {
	dark := isDark(c)
	if grown, ok := grownFrom(c, seed); ok {
		return palette.SeedCells(seed, grown, dark)
	}
	if dark {
		return []palette.SeedCell{{Col: c.Primary, Name: SeedPinName, Rule: SeedNotHeld}}
	}
	// Named for the token and not for the seed: this is the colour the
	// palette grew from, but which colour was picked to get it is not
	// recoverable, and "Seed" over here would be the pick's name over a
	// colour that may never have been picked.
	return []palette.SeedCell{{Col: c.Primary, Name: SeedPinName, Rule: SeedFromBase}}
}

// grownFrom reports the colour a palette grew from when the palette is this
// seed's, checked rather than assumed: a seed derives one pair per variant,
// and c is that seed's only if it is one of the four token sets byte for
// byte. The colour returned is the light scheme's pinned base — the seed
// realized, which is what every accent ramp and base downstream was
// measured off.
//
// It stays here rather than in the story because both halves of it are
// facts about this window: four sets, because an application theme can hand
// this window a high-contrast pair it did not derive itself, and an alpha
// normalized first, because the candidate arrives from a kept brand rather
// than from a swatch this window painted. A window that derives its own
// pair from its own pick has a different answer to both, and a shared check
// parameterised over them would be a function whose whole body is its
// parameters.
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
