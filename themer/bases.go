// The syntax-base chooser: which palette the sample's fenced code is coloured
// from.
//
// # Which names are on it
//
// One half at a time, following the scheme switch at the top of the window:
// the sun's list is the bases fitted to a light background and the moon's
// those fitted to a dark one, measured off each style's own background rather
// than read off its name. It is one state and not a second control — a filter
// with a switch of its own could be set to disagree with the appearance on
// screen, which would mean offering a list of styles for the scheme nobody is
// looking at.
//
// # What a press means
//
// The appearance on screen is what a name is chosen for. A theme carries a
// base per appearance, because a palette somebody balanced against a
// near-white page is not the one they would balance against a near-black one,
// so the sun's list sets the light base and the moon's the dark one. Flipping
// the scheme therefore swaps the list, the marked row and the colour under
// the code together, in the frame the switch is pressed; it changes neither
// choice.
//
// # Why a scrolled column
//
// There are seventy-four palettes before anybody adds one, which rules out
// the idioms a smaller set would take: a row of cards is the swatch row's
// shape and holds six, and a menu drawing its full height would run off the
// bottom of the window long before it ran out of names. So the names are a
// scrolled column with the chosen one marked, beside the sample they colour,
// inside the group's own box.
package main

import (
	"fmt"
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

// The chooser's dimensions.
const (
	BaseW    unit.Dp = 190 // the column, beside the code sample
	BaseRow  unit.Dp = 26  // one name
	BasePad  unit.Dp = 8   // a row's leading edge to the name on it
	BaseMark unit.Dp = 6   // the chosen row's marker, at the row's leading edge
)

// baseLead is how many names the column keeps above the applied one when it
// brings it into view. Landing the applied base on the first line would make
// a list that starts at G look like a list of bases that start at G; a couple
// of rows above it show the cut for what it is.
const baseLead = 2

// What heads the group and what marks a row.
const (
	BaseLabel  = "Syntax base"
	BaseAdded  = "added"  // a style read from the styles folder
	BaseEither = "either" // a style that named no background, so it is on both lists
)

// BaseHintFor is what the syntax base group's title row says beside its
// title: the base the code on screen is coloured from, and how much of the
// set the column under it is offering. The base named is the applied one
// rather than the kept one, because what is on screen is what is being judged.
func BaseHintFor(m Model, dark bool, n int) string {
	return m.Base(dark) + ", " + BaseCountFor(dark, n)
}

// BaseCountFor says how long the list on screen is and which half of the set
// it is. A column showing half the styles there are, with nothing saying so,
// reads as one that failed to load the rest.
func BaseCountFor(dark bool, n int) string {
	if dark {
		return fmt.Sprintf("one of %d fitted to a dark background.", n)
	}
	return fmt.Sprintf("one of %d fitted to a light background.", n)
}

// baseChooser is what the column keeps across emissions: where it is
// scrolled, one click handler per base, whether the chosen name has been
// brought into view yet, and which half of the list was on screen last time.
//
// The handlers are one per base and not one per visible row, which is what
// makes the filter cost nothing: a name keeps its handler when half the list
// goes, so a press in flight cannot be handed to another style by the scheme
// changing under it. The set of names does not change after startup, so the
// slice is allocated once, on the first emission that knows how long it has
// to be.
type baseChooser struct {
	st     *list.State
	clicks []gesture.Click
	shown  bool
	dark   bool
}

func newBaseChooser() *baseChooser { return &baseChooser{st: list.NewState()} }

// handlers returns n click handlers, allocating on the first call.
func (b *baseChooser) handlers(n int) []gesture.Click {
	if len(b.clicks) < n {
		b.clicks = make([]gesture.Click, n)
	}
	return b.clicks
}

// reveal brings the applied base into view when the list it is in has just
// been replaced — the first emission, and every flip of the scheme, which
// swaps one half of the names for the other and leaves a scroll offset that
// measured a list nobody is looking at any more. Between those, the column is
// left exactly where the reader put it: scrolling it on every emission would
// drag it back under whoever is reading it.
func (b *baseChooser) reveal(dark bool, row int) {
	if b.shown && b.dark == dark {
		return
	}
	b.shown, b.dark = true, dark
	if row >= 0 {
		b.st.ScrollTo(max(0, row-baseLead))
	}
}

// BasePanel draws the base chooser: the names in a scrolling column with the
// chosen one marked, standing in the group's box.
//
// Only the half of the list the scheme switch is showing — see
// [Model.VisibleBases] — and the marked row is that appearance's own choice.
// A row carries the index of the base it names in the whole list, not its
// place in the visible half, so what a press means does not depend on which
// half is on screen.
//
// The column draws no plate of its own and no heading. The group's box is the
// plate and the group's title row is the heading, which is where the count and
// the scheme switch stand.
func BasePanel(p Palette, c tokens.PlatformColors, ty Type, m Model, dark bool, sel *baseChooser) layout.Widget {
	if sel == nil {
		sel = newBaseChooser()
	}
	visible := m.VisibleBases(dark)
	clicks := sel.handlers(len(m.Bases))
	rows := make([]layout.Widget, len(visible))
	applied := -1
	chosen := m.BaseAt(dark)
	for row, i := range visible {
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
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		gtx.Constraints = layout.Exact(size)
		// The bar takes a gutter rather than floating over the names. The
		// gutter costs a few points of a column that is narrow already and
		// buys the one thing an overlay cannot on a list this dense: the
		// marked row ends where the row ends rather than running under the
		// track.
		list.LayoutScrollbar(gtx, st, scrollbar.FromTokens(c, p.Surface), list.Occupy, rows,
			func(gtx layout.Context, row layout.Widget) layout.Dimensions {
				return row(gtx)
			})
		return layout.Dimensions{Size: size}
	}
}

// BaseRowWidget draws one name and makes it clickable.
func BaseRowWidget(gtx layout.Context, p Palette, ty Type, opt BaseOption, index int, dark, chosen bool, click *gesture.Click) layout.Dimensions {
	dims := ChoiceRow(gtx, p, ty, opt.Name, baseTag(opt), chosen, click)
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: SelectBase{Index: index, Dark: dark}}.Add(gtx.Ops)
		}
	}
	return dims
}

// ChoiceRow draws one row of either chooser — a name, an optional word at the
// far end, and the state it is in — and hands the row's own area to click.
// Pumping that handler is the caller's, because what a press means is the
// caller's.
//
// The chosen row wears the platform's own selection under the label the
// platform pairs with it, which is how a list says which of its rows is the
// one in force, and carries a bar at its leading edge in that same label
// colour: the selection fill is the accent, so a marker drawn in the accent
// would be a marker nobody can see. Under the pointer a row takes the
// platform's hover overlay.
//
// An unchosen row takes the platform's label colour, and not the muted step
// under it. Read cold, a column of choices set in that muted step read as a
// column of choices that were not available — "a 36-item list of available
// choices reads as one live row and four dead ones" — and the platform's own
// lists carry the label colour on every row, marking the chosen one by
// inverting it.
func ChoiceRow(gtx layout.Context, p Palette, ty Type, name, tag string, chosen bool, click *gesture.Click) layout.Dimensions {
	h := gtx.Dp(BaseRow)
	size := image.Pt(gtx.Constraints.Max.X, h)
	r := image.Rectangle{Max: size}
	switch {
	case chosen:
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Selection)
	case click.Hovered():
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Hover)
	}
	pad := gtx.Dp(BasePad)
	foreground := p.CardText
	if chosen {
		foreground = p.AccentForeground
		// Nearly the row's full height: a stub a third of the row tall reads
		// as a stray mark rather than as a marker.
		mark := image.Rect(0, h/8, gtx.Dp(BaseMark), h-h/8)
		fillRRect(gtx, mark, gtx.Dp(BaseMark)/2, p.AccentForeground)
	}
	text := image.Rect(pad+gtx.Dp(BaseMark), 0, size.X-pad, h)
	// A word at the far end for the two things about a name that are not
	// obvious from it: that somebody put this style in the folder themselves,
	// and that this one named no background of its own and is therefore on
	// the light list and the dark one both.
	if tag != "" {
		box := image.Rect(size.X-pad-gtx.Dp(40), 0, size.X-pad, h)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, box, 1, 0.5, foreground, tag)
		text.Max.X = box.Min.X - pad/2
	}
	textdraw.FillText(gtx, ty.Shaper, ty.Small, text, 0, 0.5, foreground, name)

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	return layout.Dimensions{Size: size}
}

// baseTag is the word marking one row, or none. Where a style came from wins
// when a style is both, being the more surprising fact.
func baseTag(o BaseOption) string {
	switch {
	case o.Added:
		return BaseAdded
	case o.Light && o.Dark:
		return BaseEither
	}
	return ""
}
