// The palette section: what the seed on screen actually generated, said in
// two parts, in the place on the embedded page where the palette was already
// spoken about.
//
// The section itself is not written here. It is the shared palette story that
// stands beside the published inventory, and both the applications that show
// a palette draw it from there — the band vocabulary, the ramps grid and its
// marks, the picks board and the rules under it, and the honesty tooling that
// cuts a line at its own boundaries rather than mid-word. What this file
// keeps is what belongs to this window: why the window carries the story at
// all, where it stands in the column, and the two conversions that hand the
// window's own palette and typography over in the shape the story asks for.
//
// # Why the window shows this at all
//
// Everything else in this window answers "what does this colour look like".
// The row of candidates offers colours, the page below draws the whole design
// system in the one that was chosen, and between those two there is a step
// nobody sees: a seed does not become a window, it becomes a palette, and the
// palette is what the window is drawn from. A person judging a seed by the page
// alone can tell that something is wrong without being able to say what — the
// dividers are too faint, the warning is too close to the error — because the
// thing that decided both is not on screen anywhere.
//
// So it is put on screen, and in the order the derivation works in. The seed
// itself comes first, one section above this one, because it is the input
// everything here is a function of. Then the ramps, which are what there is to
// pick from. Then the picks, which are the colours the theme actually names,
// each beside the rule that chose it.
//
// # Where it stands
//
// In the embedded page's own foundations, and the catalogue's own two palette
// sections are on no tab of this window: a section of provenance beside a
// section showing the same tokens again is the same page saying one thing
// twice, with no way of telling which of the two is the answer.
//
// It is a section of that column rather than a band of the window. The page
// under the candidate row is what a seed is judged on and is the biggest thing
// in the window; a fixed band above it would come out of that page, and this
// section is a third of a screen tall. In the column it costs the page nothing
// and lands in the causal order: the seed, the ramps, the picks, then
// everything drawn from them.
package main

import (
	"gioui.org/layout"

	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/theme/tokens"
)

// story is the window's palette as the shared section takes it: the four
// colours that section draws its own furniture with, and no more.
//
// The window's palette names more than that — a selection fill, a card edge,
// an accent and the ink over it — and none of them is a colour the story has
// any business reading. Handing over four fields rather than the whole value
// is what keeps that true by construction: a colour the window adds later
// cannot silently start appearing inside a section that never asked for it.
func (p Palette) story() palette.Chrome {
	return palette.Chrome{
		Surface: p.Surface,
		Divider: p.Divider,
		Text:    p.Text,
		Muted:   p.Muted,
	}
}

// story is the window's typography as the shared section takes it: the four
// roles it sets text in, with the theme's cached shaper. The window's own
// title role and the line box it hands to a published component stay here,
// the section drawing neither.
func (t Type) story() palette.Type {
	return palette.Type{
		Shaper: t.Shaper,
		Head:   t.Head,
		Label:  t.Label,
		Body:   t.Body,
		Small:  t.Small,
	}
}

// PaletteRows is the palette story as rows of the embedded page's column,
// drawn in this window's palette and typography.
func PaletteRows(p Palette, c, other tokens.ColorTokens, ty Type, dark bool) []layout.Widget {
	return palette.Rows(p.story(), c, other, ty.story(), dark)
}

// paletteHeading is the section's heading band, which the seed row above the
// story and the type ladder below it wear as well: one band for the whole
// column, so a reader meets one kind of heading between the seed and the
// ladder rather than three.
func paletteHeading(p Palette, c tokens.ColorTokens, ty Type, title, hint string) layout.Widget {
	return palette.Heading(p.story(), c, ty.story(), title, hint)
}

// fitHint cuts a heading band's caption to the room the title left it, at the
// clause boundaries the caption is written in.
func fitHint(gtx layout.Context, ty Type, hint string, room int) string {
	return palette.FitHint(gtx, ty.story(), hint, room)
}
