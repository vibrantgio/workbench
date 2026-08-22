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
	"github.com/vibrantgio/markdown/highlight"
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
	bases  highlight.BasePair
	code   int
	// swapAt is the row of the inventory's own list where its palette sections
	// begin and swapRows how many rows they take, or -1 before the first call
	// has looked. They are the rows this window replaces with its own — see
	// [embed.items].
	swapAt, swapRows int
}

func newEmbed() *embed { return &embed{st: list.NewState(), code: -1, swapAt: -1} }

// swapped names the inventory's own palette sections, which this window shows
// its own two in the place of.
//
// The window has one palette story and it is the one with the provenance in it.
// Rendering both would put the same token names in front of a reader twice
// within a screen, with nothing saying which of the two answers the question
// they came with — and the second telling, being a definition rather than a
// specimen of this seed, is the one that answers nothing this window is for.
//
// Which is a decision about what this window shows and not an edit to what it
// shows it from: the inventory is asked for its sections and this window chooses
// which of them to lay out, exactly as it already chooses to put a base selector
// beside one of them. Every other section is drawn as the inventory built it.
var swapped = []string{"foundations-roles", "foundations-ramps"}

// items returns the scrolling column: the inventory in the given palette, with
// its code drawn in the given syntax bases — the pair, so the appearance the
// palette is on picks its own member and a flip of the scheme puts the other
// one's whole plate on the specimen, its ground included — and with the palette
// rows standing where the inventory's own palette sections were.
//
// The inventory itself is built on the first call — before anything has been
// dropped, so the parse is behind us by the time a pick has to be quick. A
// code-face change restyles the parsed document; it does not rebuild it.
//
// Changing the base moves nothing. It used to scroll the specimen into view,
// because a base chosen from the far end of a column several screens tall
// changed nothing anybody could see; now the only place a base can be chosen
// from is beside the specimen, so whoever picked one is already looking at what
// it did, and a column that jumped under them would be taking a view they had
// set themselves.
//
// The palette rows are the window's own and are handed in rather than built
// here, because they are a function of the derived pair and of the side of it
// on screen, which this type knows nothing about.
func (e *embed) items(shaper *text.Shaper, typ tokens.Typography, c tokens.ColorTokens, bases highlight.BasePair, palette []layout.Widget) []layout.Widget {
	if e.inv == nil {
		e.inv, e.shaper = inventory.New(shaper), shaper
		e.bases, e.code, e.swapAt = highlight.BasePair{}, -1, -1
	} else if e.shaper != shaper {
		// A code-face change needs the matching collection. The parsed
		// document stays; only Code and the extra faces change.
		e.inv.SetShaper(shaper)
		e.shaper = shaper
	}
	e.inv.SetTypography(typ)
	if e.bases != bases {
		e.bases = bases
		e.inv.SetCodeBases(bases)
	}
	if e.code < 0 {
		// Once per inventory: the row a section lands on is a fact about the
		// column's shape, which no palette moves.
		if row := e.inv.ItemIndex(c, inventory.CodeSectionName()); row >= 0 {
			e.code = row + 1 // the heading's row, then the body's
		}
		e.swapAt, e.swapRows = e.paletteSpan(c)
	}
	items := e.inv.Items(c)
	if e.swapAt < 0 {
		// Nothing there to stand in the place of, which no build ships: the
		// window still says what its palette is, at the head of the column.
		return append(palette, items...)
	}
	out := make([]layout.Widget, 0, len(items)-e.swapRows+len(palette))
	out = append(out, items[:e.swapAt]...)
	out = append(out, palette...)
	return append(out, items[e.swapAt+e.swapRows:]...)
}

// paletteSpan is where the inventory's own palette sections stand in its list:
// the row the first of them begins on, and how many rows the run of them takes,
// counting a heading and a body each. It answers (-1, 0) if none of them is
// there, which is what a rename upstream looks like from here.
func (e *embed) paletteSpan(c tokens.ColorTokens) (at, rows int) {
	first, last := -1, -1
	for _, name := range swapped {
		row := e.inv.ItemIndex(c, name)
		if row < 0 {
			continue
		}
		if first < 0 || row < first {
			first = row
		}
		if row > last {
			last = row
		}
	}
	if first < 0 {
		return -1, 0
	}
	return first, last + 2 - first
}

// codeRow is the row of [embed.items] the code specimen's body is drawn on, or
// -1 before the first call has built the column.
//
// It indexes the inventory's own list and nothing else. The column is that list
// with rows taken out of it and rows put in — see [embed.items] — so anything
// addressing the column asks [embed.codeColumnRow] instead.
func (e *embed) codeRow() int { return e.code }

// codeColumnRow is the row of the scrolling column the code specimen's body is
// drawn on: its row in the inventory's own list, moved by whatever the palette
// swap did to the rows in front of it.
//
// It is the number anything addressing the column uses, [embed.codeRow] being
// an index into a list the column is not.
func (e *embed) codeColumnRow() int {
	if e.code < 0 {
		return -1
	}
	if e.swapAt < 0 {
		return e.code + PaletteSectionRows
	}
	return e.code + PaletteSectionRows - e.swapRows
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

// GalleryHintFor writes what the page is rendered from: the chosen colour and
// the syntax base its code is coloured with under the appearance on screen, or
// the standing invitation while there is none.
//
// The base is named here as well as marked in the column because the column is
// at the far end of a page several screens tall. Everywhere else on that page,
// this line is the only thing saying what the code is coloured with — and it
// stays on screen, where the column does not. It names one base and not the
// pair for the same reason: it says what is on screen, and what is on screen is
// one appearance's.
func GalleryHintFor(m Model, dark bool) string {
	if seed, ok := m.Seed(); ok {
		return "rendered from " + hexOf(seed) + " · syntax base " + m.Base(dark)
	}
	return GalleryHint
}
