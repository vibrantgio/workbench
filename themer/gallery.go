// The embedded page: the whole published surface, drawn in the palette the
// chosen candidate generates.
//
// This is what the application is for. A swatch says what a colour is; three
// dozen families drawn in it say what it does, which is the only question a
// brand colour actually raises. The page below the candidate row is the
// answer, and picking another swatch redraws all of it on the next frame.
package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/text"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// The embedded page's own furniture.
const (
	GalleryLabel = "Everything in this theme"
	GalleryHint  = "the whole published surface, scroll for the rest"
)

// embed is what the page keeps across frames and across picks: where each of
// the inventory's own scrolling sections is scrolled to, the rasterised icon,
// and the reading sample parsed from its source.
//
// It is built once and never rebuilt for a palette, which is what makes a
// pick cheap. Parsing the reading sample is the expensive half of this page
// and colour is no part of it: the document takes its style at layout time,
// so choosing another candidate re-styles a document already parsed rather
// than reading one again. What a pick costs is one palette derivation, one
// pass building the section values, and the frame that draws them.
type embed struct {
	st     *list.State
	shaper *text.Shaper
	inv    *inventory.Inventory
}

func newEmbed() *embed { return &embed{st: list.NewState()} }

// items returns the whole inventory as the rows of the scrolling column, in
// the given palette. The inventory itself is built on the first call — before
// anything has been dropped, so the parse is behind us by the time a pick has
// to be quick — and again only if the typography under it is replaced.
func (e *embed) items(shaper *text.Shaper, c tokens.ColorTokens) []layout.Widget {
	if e.inv == nil || e.shaper != shaper {
		e.inv, e.shaper = inventory.New(shaper), shaper
	}
	return e.inv.Items(c)
}

// Gallery draws the embedded page: a label saying what it is and which colour
// it is rendered from, and under it the inventory on its own panel, scrolled
// rather than cut off.
//
// The panel is filled with the embedded palette's own background, not the
// window's — for a moment after a pick those are the same thing, and that is
// the point: the page is not a preview beside the theme, it is the theme.
func Gallery(p Palette, c tokens.ColorTokens, ty Type, seed string, st *list.State, items []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, size.X, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, GalleryLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, seed)

		top := labelH + gtx.Dp(RowTop)
		if top >= size.Y {
			return layout.Dimensions{Size: size}
		}
		panel := image.Rectangle{Max: image.Pt(size.X, size.Y-top)}
		at(gtx, image.Pt(0, top), func(gtx layout.Context) {
			fillRRect(gtx, panel, gtx.Dp(Radius), c.Background)
			// Clipped to the panel: the column is taller than the window
			// several times over, and a row drawn past the bottom edge would
			// land on the candidate row above it on the next frame.
			defer clip.UniformRRect(panel, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
			gtx.Constraints = layout.Exact(panel.Max)
			// The bar floats over the rows rather than reserving a gutter:
			// the column below is the design system at its own widths, and a
			// gutter cut out of it would be the application editing what it
			// is showing. It is drawn from the embedded palette, because it
			// is furniture of the page and not of the window.
			list.LayoutScrollbar(gtx, st, scrollbar.FromTokens(c), list.Overlay, items,
				func(gtx layout.Context, w layout.Widget) layout.Dimensions {
					return w(gtx)
				})
			strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline), p.Divider)
		})
		return layout.Dimensions{Size: size}
	}
}

// GalleryHintFor writes what the page is rendered from: the chosen colour, or
// the standing invitation while there is none.
func GalleryHintFor(m Model) string {
	if seed, ok := m.Seed(); ok {
		return "rendered from " + hexOf(seed)
	}
	return GalleryHint
}
