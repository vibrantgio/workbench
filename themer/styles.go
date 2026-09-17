// The syntax highlighter style chooser: which of chroma's styles a fenced
// code block is drawn in.
//
// # Which names are on it
//
// One half at a time, following the appearance switch at the top of the
// window: under the sun the styles fitted to a light background, under the
// moon those fitted to a dark one, measured off each style's own background
// rather than read off its name. The four that name no background of their
// own are on both lists and say so. It is one state and not a second control
// — a filter with a switch of its own could be set to disagree with the
// appearance on screen, which would mean offering a list of styles for the
// scheme nobody is looking at.
//
// # What a press means
//
// The appearance on screen is what a name is chosen for. A theme carries a
// style per appearance, because a set of colours somebody balanced against a
// near-white page is not the set they would balance against a near-black one,
// so the sun's list sets the light style and the moon's the dark one.
// Flipping the appearance therefore swaps the list, the marked row and the
// colours under the code together, in the frame the switch is pressed; it
// changes neither choice.
//
// # Why a scrolled column
//
// Chroma ships seventy-four styles — thirty-six fitted to a light background
// and forty-two to a dark one — before anybody adds one of their own, which
// rules out the idioms a smaller set would take: a row of cards is the swatch
// row's shape and holds six, and a menu drawing its full height would run off
// the bottom of the window long before it ran out of names. So the names are
// a column of list rows at the platform's own row height, with the platform's
// scrollbar in a gutter beside them, filling the group's box from the top of
// the choices to the footer.
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

// The chooser's dimensions.
const (
	// StyleW is the group's box, standing beside the preview.
	StyleW unit.Dp = 300
	// StyleRowH is one name: the platform's own list row height, which is
	// what puts as much of the set on screen at once as the column has room
	// for.
	StyleRowH unit.Dp = unit.Dp(tokens.ComfortableRowHeight)
	// StylePad is a row's leading edge to the name on it, and StyleIndent the
	// further run the name is set in from, so a row reads as a list's row
	// rather than as a word against the box's edge.
	StylePad    unit.Dp = 8
	StyleIndent unit.Dp = 6
)

// styleLead is how many names the column keeps above the applied one when it
// brings it into view. Landing the applied style on the first line would make
// a list that starts at G look like a list of styles that start at G; a couple
// of rows above it show the cut for what it is.
const styleLead = 2

// What heads the group and what marks a row.
//
// The group's title row carries no caption. The title names the thing and the
// group is a list of its names, which is the whole of what there is to say;
// the run beside it is 300 points wide, and a sentence cut short after four
// words says less than nothing.
const (
	StyleLabel  = "Syntax highlighter style"
	StyleHint   = ""
	StyleAdded  = "added"  // a style read from the styles folder
	StyleEither = "either" // a style that named no background, so it is on both lists
)

// styleChooser is what the column keeps across emissions: where it is
// scrolled, one click handler per style, whether the chosen name has been
// brought into view yet, and which half of the list was on screen last time.
//
// The handlers are one per style and not one per visible row, which is what
// makes the filter cost nothing: a name keeps its handler when half the list
// goes, so a press in flight cannot be handed to another style by the
// appearance changing under it. The set of names does not change after
// startup, so the slice is allocated once, on the first emission that knows
// how long it has to be.
type styleChooser struct {
	st     *list.State
	clicks []gesture.Click
	shown  bool
	dark   bool
}

func newStyleChooser() *styleChooser { return &styleChooser{st: list.NewState()} }

// handlers returns n click handlers, allocating on the first call.
func (b *styleChooser) handlers(n int) []gesture.Click {
	if len(b.clicks) < n {
		b.clicks = make([]gesture.Click, n)
	}
	return b.clicks
}

// reveal brings the applied style into view when the list it is in has just
// been replaced — the first emission, and every flip of the appearance, which
// swaps one half of the names for the other and leaves a scroll offset that
// measured a list nobody is looking at any more. Between those, the column is
// left exactly where the reader put it: scrolling it on every emission would
// drag it back under whoever is reading it.
func (b *styleChooser) reveal(dark bool, row int) {
	if b.shown && b.dark == dark {
		return
	}
	b.shown, b.dark = true, dark
	if row >= 0 {
		b.st.ScrollTo(max(0, row-styleLead))
	}
}

// StylePanel draws the chooser: the names in a scrolling column with the
// chosen one marked, filling the group's box.
//
// Only the half of the list the appearance switch is showing — see
// [Model.VisibleStyles] — and the marked row is that appearance's own choice.
// A row carries the index of the style it names in the whole list, not its
// place in the visible half, so what a press means does not depend on which
// half is on screen.
//
// The column draws no plate of its own and no heading. The group's box is the
// plate and the group's title row is the heading.
func StylePanel(p Palette, c tokens.PlatformColors, ty Type, m Model, dark bool, sel *styleChooser) layout.Widget {
	if sel == nil {
		sel = newStyleChooser()
	}
	visible := m.VisibleStyles(dark)
	clicks := sel.handlers(len(m.Styles))
	rows := make([]layout.Widget, len(visible))
	applied := -1
	chosen := m.StyleAt(dark)
	for row, i := range visible {
		if i == chosen {
			applied = row
		}
		rows[row] = func(gtx layout.Context) layout.Dimensions {
			return StyleRowWidget(gtx, p, ty, m.Styles[i], i, dark, i == chosen, &clicks[i])
		}
	}
	st := sel.st
	// The window opens on the style that was kept, which may be sixty names
	// down a sorted list, and a flip of the appearance replaces the list under
	// it.
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

// StyleRowWidget draws one name and makes it clickable.
func StyleRowWidget(gtx layout.Context, p Palette, ty Type, opt StyleOption, index int, dark, chosen bool, click *gesture.Click) layout.Dimensions {
	dims := ChoiceRow(gtx, p, ty, opt.Name, styleTag(opt), StyleRowH, chosen, click)
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: SelectStyle{Index: index, Dark: dark}}.Add(gtx.Ops)
		}
	}
	return dims
}

// ChoiceRow draws one row of either chooser — a name, an optional word at the
// far end, and the state it is in — h tall, and hands the row's own area to
// click. Pumping that handler is the caller's, because what a press means is
// the caller's.
//
// The chosen row wears the platform's own selection under the label the
// platform pairs with it, and nothing else. That is how the platform's own
// lists say which row is in force; a second mark inside the fill — a bar at
// the row's leading edge — read cold as "a flat white square notch stuck to
// the left cap, breaking the rounded end", because a marker drawn inside a
// rounded fill's own corner is a bite out of that corner. Under the pointer
// a row takes the platform's hover overlay.
//
// An unchosen row takes the platform's label colour, and not the muted step
// under it. Read cold, a column of choices set in that muted step read as a
// column of choices that were not available — "a 36-item list of available
// choices reads as one live row and four dead ones" — and the platform's own
// lists carry the label colour on every row, marking the chosen one by
// inverting it.
func ChoiceRow(gtx layout.Context, p Palette, ty Type, name, tag string, height unit.Dp, chosen bool, click *gesture.Click) layout.Dimensions {
	h := gtx.Dp(height)
	size := image.Pt(gtx.Constraints.Max.X, h)
	r := image.Rectangle{Max: size}
	switch {
	case chosen:
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Selection)
	case click.Hovered():
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Hover)
	}
	pad := gtx.Dp(StylePad)
	foreground := p.CardText
	if chosen {
		foreground = p.AccentForeground
	}
	text := image.Rect(pad+gtx.Dp(StyleIndent), 0, size.X-pad, h)
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

// styleTag is the word marking one row, or none. Where a style came from wins
// when a style is both, being the more surprising fact.
func styleTag(o StyleOption) string {
	switch {
	case o.Added:
		return StyleAdded
	case o.Light && o.Dark:
		return StyleEither
	}
	return ""
}
