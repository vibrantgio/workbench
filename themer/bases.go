// The base selector: which syntax palette the code on the embedded page is
// coloured from.
//
// # Where it sits
//
// Beside the code, inside the page's last section, and nowhere else. The
// syntax palette is one choice among the dozens this window is for, and a
// column standing the whole height of the window gave it a permanence the rest
// of the theming does not get — a page about theming a design system with a
// syntax chooser pinned to the side of it is a page that says code is half the
// subject. Seated in the section it drives, it is exactly as prominent as the
// thing it changes: reached by scrolling to the specimen, and once there, on
// screen at the same time as the fence it colours.
//
// That seat is also what makes a list of this size workable. There are
// seventy-four palettes before anybody adds one, which rules out the idioms a
// smaller set would take: a row of chips is the candidate row's shape and holds
// six, and a menu drawing its full height would run off the bottom of the
// window long before it ran out of names. So the names are a scrolled column
// with the chosen one marked, a hundred and ninety points wide, taking the
// margin beside a code plate that is measured to its own longest line and does
// not want the rest of the row anyway.
//
// # Which names are on it
//
// One half at a time, following the scheme control at the top: the sun's list
// is the bases fitted to a light ground and the moon's the bases fitted to a
// dark one, measured off each style's own background rather than read off its
// name. It is one state and not a second control — a filter with a switch of
// its own could be set to disagree with the appearance on screen, which would
// mean offering a list of styles for the scheme nobody is looking at.
//
// # What a press means
//
// The appearance on screen is what a name is chosen for. A theme carries a
// base per appearance, because a palette somebody balanced against a near-white
// page is not the one they would balance against a near-black one, so the sun's
// list sets the light base and the moon's the dark one. Flipping the scheme
// therefore swaps the list, the applied row and the ink under the code together,
// in the frame the switch is pressed; it changes neither choice.
package main

import (
	"fmt"
	"image"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// The selector's dimensions.
const (
	BaseW    unit.Dp = 190 // the column, beside the code specimen
	BaseRow  unit.Dp = 26  // one name
	BasePad  unit.Dp = 8   // panel edge to the names inside it
	BaseInk  unit.Dp = 6   // the chosen row's marker, at the row's left edge
	BaseHead unit.Dp = 38  // the two lines heading the column, inside its panel
)

// baseLead is how many names the column keeps above the applied one when it
// brings it into view. Landing the applied base on the first line would make a
// list that starts at G look like a list of bases that start at G; a couple of
// rows above it show the cut for what it is.
const baseLead = 2

// What heads the column and what marks a row.
const (
	BaseLabel  = "Syntax base"
	BaseInvite = "Click one to apply."
	BaseAdded  = "added"  // a style read from the styles folder
	BaseEither = "either" // a style that named no ground, so it is on both lists
)

// BaseCountFor says how long the list on screen is and which half of the set it
// is. A column showing half the styles there are, with nothing saying so, reads
// as one that failed to load the rest.
func BaseCountFor(dark bool, n int) string {
	if dark {
		return fmt.Sprintf("%d dark bases", n)
	}
	return fmt.Sprintf("%d light bases", n)
}

// baseSelector is what the column keeps across emissions: where it is
// scrolled, one click handler per base, whether the chosen name has been
// brought into view yet, and which half of the list was on screen last time.
//
// The handlers are one per base and not one per visible row, which is what
// makes the filter cost nothing: a name keeps its handler when half the list
// goes, so a press in flight cannot be handed to another style by the scheme
// changing under it. The set of names does not change after startup, so the
// slice is allocated once, on the first emission that knows how long it has to
// be.
type baseSelector struct {
	st     *list.State
	clicks []gesture.Click
	shown  bool
	dark   bool
}

func newBaseSelector() *baseSelector { return &baseSelector{st: list.NewState()} }

// handlers returns n click handlers, allocating on the first call.
func (b *baseSelector) handlers(n int) []gesture.Click {
	if len(b.clicks) < n {
		b.clicks = make([]gesture.Click, n)
	}
	return b.clicks
}

// reveal brings the applied base into view when the list it is in has just been
// replaced — the first emission, and every flip of the scheme, which swaps one
// half of the names for the other and leaves a scroll offset that measured a
// list nobody is looking at any more. Between those, the column is left exactly
// where the reader put it: scrolling it on every emission would drag it back
// under whoever is reading it.
func (b *baseSelector) reveal(dark bool, row int) {
	if b.shown && b.dark == dark {
		return
	}
	b.shown, b.dark = true, dark
	if row >= 0 {
		b.st.ScrollTo(max(0, row-baseLead))
	}
}

// BesideTheCode seats the selector to the left of the code specimen's own row,
// and hands back the row with both in it.
//
// The two share a row rather than stacking because they are one act: the names
// change what the fence beside them is coloured with, and a name chosen from
// behind the thing it changes is chosen blind. The specimen's row is where they
// can share one, being the tallest on the page and the only one whose whole
// subject is the choice.
//
// The code keeps the margin it had. It is laid out in what is left of the row
// after the column, and it loses nothing by it: the plate is measured to its own
// longest line, and the width the column takes is width the specimen was
// leaving empty on the right.
func BesideTheCode(p Palette, c tokens.ColorTokens, ty Type, m Model, dark bool, sel *baseSelector, code layout.Widget) layout.Widget {
	panel := BasePanel(p, c, ty, m, dark, sel)
	return func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		padX, padY := gtx.Dp(inventory.SectionPadX), gtx.Dp(inventory.SectionPadY)
		col := min(gtx.Dp(BaseW), max(0, width-2*padX))

		// The specimen first, because its height is the row's and the column
		// is cut to it. Its own left margin is the gutter between the two, so
		// nothing is inserted here that the row does not already space with.
		beside := gtx
		beside.Constraints.Max.X = max(0, width-padX-col)
		beside.Constraints.Min.X = beside.Constraints.Max.X
		var dims layout.Dimensions
		at(beside, image.Pt(padX+col, 0), func(gtx layout.Context) {
			dims = code(gtx)
		})

		// The column stands in the same margin the specimen is inset by, top
		// and bottom, so the two read as one pair of plates rather than as a
		// list that happens to be near some code.
		if h := dims.Size.Y - 2*padY; h > 0 && col > 0 {
			inner := gtx
			inner.Constraints = layout.Exact(image.Pt(col, h))
			at(inner, image.Pt(padX, padY), func(gtx layout.Context) {
				panel(gtx)
			})
		}
		return layout.Dimensions{Size: image.Pt(width, dims.Size.Y)}
	}
}

// BasePanel draws the base selector: one plate carrying a two-line heading and,
// under it, the names in a scrolling list with the chosen one marked.
//
// Only the half of the list the scheme control is showing — see
// [Model.VisibleBases] — and the marked row is that appearance's own choice. A
// row carries the index of the base it names in the whole list, not its place
// in the visible half, so what a press means does not depend on which half is
// on screen.
func BasePanel(p Palette, c tokens.ColorTokens, ty Type, m Model, dark bool, sel *baseSelector) layout.Widget {
	visible := m.VisibleBases(dark)
	clicks := sel.handlers(len(m.Bases))
	rows := make([]layout.Widget, len(visible))
	applied := -1
	chosen := m.BaseAt(dark)
	for row, i := range visible {
		i := i
		if i == chosen {
			applied = row
		}
		rows[row] = func(gtx layout.Context) layout.Dimensions {
			return BaseRowWidget(gtx, p, ty, m.Bases[i], i, dark, i == chosen, &clicks[i])
		}
	}
	st := sel.st
	// The window opens on the base that was kept, which may be sixty names
	// down a sorted list, and a flip of the scheme replaces the list under it.
	sel.reveal(dark, applied)
	count := BaseCountFor(dark, len(visible))
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		panel := image.Rectangle{Max: size}
		fillRRect(gtx, panel, gtx.Dp(Radius), p.Surface)
		defer clip.UniformRRect(panel, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(size)
		layout.UniformInset(BasePad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			w, headH := gtx.Constraints.Max.X, gtx.Dp(BaseHead)
			// The heading is inside the panel rather than above it, so the
			// column's plate starts on the same line as the code plate beside
			// it and the two read as a pair. Two lines: what the list is, how
			// much of it there is, and that it can be pressed — a column of
			// plain names with one row tinted is a legend until something says
			// otherwise.
			line := headH / 2
			name := image.Rect(0, 0, w, line)
			hint := image.Rect(0, line, w, headH)
			textdraw.FillText(gtx, ty.Shaper, ty.Label, name, 0, 0.5, p.Text, BaseLabel)
			textdraw.FillText(gtx, ty.Shaper, ty.Small, name, 1, 0.5, p.Muted, count)
			textdraw.FillText(gtx, ty.Shaper, ty.Small, hint, 0, 0.5, p.Muted, BaseInvite)

			if headH >= gtx.Constraints.Max.Y {
				return layout.Dimensions{Size: size}
			}
			at(gtx, image.Pt(0, headH), func(gtx layout.Context) {
				gtx.Constraints = layout.Exact(image.Pt(w, gtx.Constraints.Max.Y-headH))
				// The bar takes a gutter rather than floating over the names,
				// and it never fades. The gutter costs ten points of a column
				// that is narrow already and buys the one thing an overlay
				// cannot on a list this dense: the marked row ends where the
				// row ends rather than running under the track. Standing
				// rather than fading buys the other: the plate cuts the list
				// off a row short of its end, and a cut that says nothing
				// reads as the list finishing there.
				bar := scrollbar.FromTokens(c)
				bar.FadeDelay = 0
				list.LayoutScrollbar(gtx, st, bar, list.Occupy, rows,
					func(gtx layout.Context, row layout.Widget) layout.Dimensions {
						return row(gtx)
					})
			})
			return layout.Dimensions{Size: size}
		})
		strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline), p.Divider)
		return layout.Dimensions{Size: size}
	}
}

// BaseRowWidget draws one name and makes it clickable. The chosen row carries
// a bar in the accent at its leading edge and its name in the page's own text
// colour; every other row is muted, so the column reads as one choice among
// many rather than as a list of equal things.
func BaseRowWidget(gtx layout.Context, p Palette, ty Type, opt BaseOption, index int, dark, chosen bool, click *gesture.Click) layout.Dimensions {
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
		// Nearly the row's full height. On a dark scheme the fill under a
		// chosen row is a deep primary against a deep panel and carries almost
		// none of the signal, so this bar is what says "this one"; a stub a
		// third of the row tall reads as a stray mark rather than a marker.
		mark := image.Rect(0, h/8, gtx.Dp(BaseInk), h-h/8)
		fillRRect(gtx, mark, gtx.Dp(BaseInk)/2, p.Accent)
	}
	tone := p.Muted
	if chosen {
		tone = p.Text
	}
	text := image.Rect(pad+gtx.Dp(BaseInk), 0, size.X-pad, h)
	// A word at the far end for the two things about a name that are not
	// obvious from it: that somebody put this style in the folder themselves,
	// and that this one named no ground of its own and is therefore on the light
	// list and the dark one both. Where it came from wins when a style is both,
	// being the more surprising fact. It is muted and not accented because the
	// accent belongs to the one row that is the answer, and a coloured word on
	// four other rows outshouts the state it is there to annotate.
	if tag := baseTag(opt); tag != "" {
		box := image.Rect(size.X-pad-gtx.Dp(40), 0, size.X-pad, h)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, box, 1, 0.5, p.Muted, tag)
		text.Max.X = box.Min.X - pad/2
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
			mvu.MessageOp{Message: SelectBase{Index: index, Dark: dark}}.Add(gtx.Ops)
		}
	}
	return layout.Dimensions{Size: size}
}

// baseTag is the word marking one row, or none. A style that came out of the
// folder says so; one that named no ground of its own says that instead, which
// is the only explanation a reader gets for a name reading "dark" sitting on
// the light list.
func baseTag(o BaseOption) string {
	switch {
	case o.Added:
		return BaseAdded
	case o.Light && o.Dark:
		return BaseEither
	}
	return ""
}
