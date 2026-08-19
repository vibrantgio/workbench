// The base selector: which syntax palette the code on the embedded page is
// coloured from.
//
// # Why a standing column and not a menu
//
// There are seventy-four palettes before anybody adds one, which rules out
// the two idioms this window already had. A row of chips is the candidate
// row's shape and holds six; a menu that draws its full height would run off
// the bottom of the window long before it ran out of names, and one that
// scrolls inside a popup puts a scrolling region over a scrolling page, which
// is the arrangement every review of it has called rough.
//
// So the names stand in a column of their own beside the page, scrolled, with
// the chosen one marked. It costs a hundred and ninety points of width that
// the page does not miss — the widest thing on it lays out at six hundred and
// forty — and it buys the thing a menu cannot: the list is on screen at the
// same time as the code it changes, so choosing is looking at a name, looking
// at the fence, and looking at the next name. That is the whole judgment, and
// a popup would put a lid on it between every step.
package main

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// The selector's dimensions.
const (
	BaseW   unit.Dp = 190 // the column, beside the embedded page
	BaseRow unit.Dp = 26  // one name
	BasePad unit.Dp = 8   // panel edge to the names inside it
	BaseInk unit.Dp = 6   // the chosen row's marker, at the row's left edge
)

// What heads the column. The hint says what the list is for rather than what
// it contains, because what it contains is obvious and why it is there is not.
const (
	BaseLabel = "Syntax base"
	BaseHint  = "colours the code"
	BaseAdded = "added" // marks a style read from the styles folder
)

// baseSelector is what the column keeps across emissions: where it is
// scrolled, one click handler per row, and whether the chosen name has been
// brought into view yet.
//
// The handlers are per row and not per name for the reason the candidate
// row's are: a handler rebuilt while a press is in flight loses the press.
// The list of names does not change after startup, so the slice is allocated
// once, on the first emission that knows how long it has to be.
type baseSelector struct {
	st     *list.State
	clicks []gesture.Click
	shown  bool
}

func newBaseSelector() *baseSelector { return &baseSelector{st: list.NewState()} }

// handlers returns n click handlers, allocating on the first call.
func (b *baseSelector) handlers(n int) []gesture.Click {
	if len(b.clicks) < n {
		b.clicks = make([]gesture.Click, n)
	}
	return b.clicks
}

// BasePanel draws the base selector: a label, and under it the names in a
// scrolling panel with the chosen one marked.
//
// The palette it is drawn from is the window's own, not the embedded page's:
// it is furniture of the application, standing beside the page rather than on
// it, and the page's own ground is what it is standing against.
func BasePanel(p Palette, c tokens.ColorTokens, ty Type, m Model, sel *baseSelector) layout.Widget {
	clicks := sel.handlers(len(m.Bases))
	rows := make([]layout.Widget, len(m.Bases))
	for i := range m.Bases {
		i := i
		rows[i] = func(gtx layout.Context) layout.Dimensions {
			return BaseRowWidget(gtx, p, ty, m.Bases[i], i, i == m.BaseAt, &clicks[i])
		}
	}
	st := sel.st
	if !sel.shown && len(rows) > 0 {
		// The window opens on the base that was kept, which may be sixty
		// names down a sorted list. Once, at the start: scrolling the column
		// on every emission would drag it back under whoever is reading it.
		sel.shown = true
		st.ScrollTo(m.BaseAt)
	}
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, size.X, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, BaseLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, BaseHint)

		top := labelH + gtx.Dp(RowTop)
		if top >= size.Y {
			return layout.Dimensions{Size: size}
		}
		panel := image.Rectangle{Max: image.Pt(size.X, size.Y-top)}
		at(gtx, image.Pt(0, top), func(gtx layout.Context) {
			fillRRect(gtx, panel, gtx.Dp(Radius), p.Surface)
			defer clip.UniformRRect(panel, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
			gtx.Constraints = layout.Exact(panel.Max)
			layout.UniformInset(BasePad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				// The bar floats over the names rather than reserving a
				// gutter: the column is narrow to begin with, and a gutter
				// cut out of it would take the width a long style name
				// needs. It is drawn from the window's palette, like the
				// panel it is in.
				return list.LayoutScrollbar(gtx, st, scrollbar.FromTokens(c), list.Overlay, rows,
					func(gtx layout.Context, w layout.Widget) layout.Dimensions {
						return w(gtx)
					})
			})
			strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline), p.Divider)
		})
		return layout.Dimensions{Size: size}
	}
}

// BaseRowWidget draws one name and makes it clickable. The chosen row carries
// a bar in the accent at its leading edge and its name in the page's own text
// colour; every other row is muted, so the column reads as one choice among
// many rather than as a list of seventy-four equal things.
func BaseRowWidget(gtx layout.Context, p Palette, ty Type, opt BaseOption, index int, chosen bool, click *gesture.Click) layout.Dimensions {
	h := gtx.Dp(BaseRow)
	size := image.Pt(gtx.Constraints.Max.X, h)
	r := image.Rectangle{Max: size}
	switch {
	case chosen:
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Selection)
	case click.Hovered():
		// A rung off the panel it is drawn on rather than the panel's own
		// fill: the rows sit on Surface, so a hover painted in Surface is a
		// hover nobody can see.
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Divider)
	}
	pad := gtx.Dp(BasePad)
	if chosen {
		mark := image.Rect(0, h/4, gtx.Dp(BaseInk), h-h/4)
		fillRRect(gtx, mark, gtx.Dp(BaseInk)/2, p.Accent)
	}
	tone := p.Muted
	if chosen {
		tone = p.Text
	}
	text := image.Rect(pad+gtx.Dp(BaseInk), 0, size.X-pad, h)
	if opt.Added {
		// The mark for a style somebody put in the folder themselves, at the
		// far end so the names still start on one line.
		tag := image.Rect(size.X-pad-gtx.Dp(40), 0, size.X-pad, h)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, tag, 1, 0.5, p.Accent, BaseAdded)
		text.Max.X = tag.Min.X - pad/2
	}
	textdraw.FillText(gtx, ty.Shaper, ty.Small, text, 0, 0.5, tone, opt.Name)

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: SelectBase{Index: index}}.Add(gtx.Ops)
		}
	}
	return layout.Dimensions{Size: size}
}
