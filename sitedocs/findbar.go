// findbar.go draws the platform's find bar over the guide column: the find
// bar field, the previous and next control and Done, on the column's own fill
// with the seam under it. The live bar and its static twin share one layout,
// so the goldens and the review captures show the bar the window draws.

package main

import (
	"image"
	stdcolor "image/color"
	"strconv"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/input"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// The find bar's height and the clear space around its parts.
//
// MEASURED, textedit-find-empty-{light,dark}.png: the bar's fill runs 31 rows
// with a 1 px hairline under it. At the bar's middle row, 7 clear columns lead
// the field, 7 stand before the previous and next control, 8 before Done and
// 9 trail it. Mail's bar (mail-find-bar-edge-{light,dark}.png) is 31 rows too,
// and its field starts 7 px after the column's first pixel.
const (
	findBarFillDp     = 31
	findBarLeadDp     = 7
	findBarFieldGapDp = 7
	findBarDoneGapDp  = 8
	findBarTrailDp    = 9
)

// findBarPrompt is the field's prompt, the platform's own.
const findBarPrompt = "Find"

// findBarSeam is the hairline under the bar: the platform's separator over
// the bar's fill. MEASURED, mail-find-bar-edge-{light,dark}.png: Mail's
// hairline under its bar is the separator over the column's fill to the byte.
func findBarSeam(c tokens.PlatformColors) stdcolor.NRGBA {
	return vgcolor.Flatten(c.Separator, pageFill(c))
}

// layoutFindBar lays the bar out across the width it is given: the fill and
// the seam under it, and the three parts leading to trailing. The field takes
// the width the other parts leave. Each part is centred on the fill's rows,
// with the odd row below the part, as the platform centres them: the 22 px
// field gets 5 rows above it and 4 below, a 20 px button 6 above and 5 below.
func layoutFindBar(gtx layout.Context, c tokens.PlatformColors, field, steps, done layout.Widget) layout.Dimensions {
	fill := image.Pt(gtx.Constraints.Max.X, gtx.Dp(findBarFillDp))
	hair := max(gtx.Dp(unit.Dp(1)), 1)
	paint.FillShape(gtx.Ops, pageFill(c), clip.Rect{Max: fill}.Op())
	paint.FillShape(gtx.Ops, findBarSeam(c), clip.Rect{
		Min: image.Pt(0, fill.Y), Max: image.Pt(fill.X, fill.Y+hair),
	}.Op())

	centre := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = 0
			gtx.Constraints.Max.Y = fill.Y
			m := op.Record(gtx.Ops)
			dims := w(gtx)
			call := m.Stop()
			defer op.Offset(image.Pt(0, (fill.Y-dims.Size.Y+1)/2)).Push(gtx.Ops).Pop()
			call.Add(gtx.Ops)
			return layout.Dimensions{Size: image.Pt(dims.Size.X, fill.Y)}
		}
	}
	row := gtx
	row.Constraints = layout.Exact(fill)
	layout.Flex{}.Layout(row,
		layout.Rigid(complayout.HSpacer(findBarLeadDp)),
		layout.Flexed(1, centre(field)),
		layout.Rigid(complayout.HSpacer(findBarFieldGapDp)),
		layout.Rigid(centre(steps)),
		layout.Rigid(complayout.HSpacer(findBarDoneGapDp)),
		layout.Rigid(centre(done)),
		layout.Rigid(complayout.HSpacer(findBarTrailDp)),
	)
	return layout.Dimensions{Size: image.Pt(fill.X, fill.Y+hair)}
}

// withFindBar stacks the bar over w when open is true. The bar takes its own
// rows and w takes the rest, so the bar covers no part of w.
func withFindBar(open bool, bar, w layout.Widget) layout.Widget {
	if !open {
		return w
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(bar),
			layout.Flexed(1, w),
		)
	}
}

// findBar is the live bar's three parts and the find they drive. The field is
// built once, so its text and its focus tag stay while the window is open.
// The two buttons take their surface as a colour, so they are built again
// when the scheme changes.
//
// The query and the current match are frame state: the field, the previous
// and next control and the page write them, all on the frame. Stepping needs
// the number of matches, and only the document holds it.
type findBar struct {
	// field is the field's focus tag. The page gives it the keyboard when
	// the bar opens.
	field event.Tag
	// doc is the guide that the find marks.
	doc *markdown.Document
	// query is the field's text.
	query string
	// current indexes the document's matches: the match that Enter and the
	// previous and next control step from.
	current int
	// inField is whether the field holds the keyboard on this frame. The
	// page sets it before anything is laid out. The marks and the count
	// show only while it is true.
	inField bool

	parts rx.Observable[rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]]
}

// newFindBar builds the bar's parts over doc. Done posts CloseFind through
// post.
func newFindBar(th rx.Observable[theme.Theme], doc *markdown.Document, post func(gtx layout.Context, msg mvu.Message)) *findBar {
	b := &findBar{doc: doc}
	fieldObs := input.SearchField(th, input.SearchFieldProps{
		Placeholder: findBarPrompt,
		Variant:     input.FindBar,
		Surface:     pageFill,
		FocusTag:    func(tag event.Tag) { b.field = tag },
		OnChange:    func(_ layout.Context, text string) { b.setQuery(text) },
		OnNext:      func(gtx layout.Context) { b.step(gtx, 1) },
		OnPrevious:  func(gtx layout.Context) { b.step(gtx, -1) },
		Count:       b.count,
	})
	colors := rx.DistinctUntilChanged(func(a, b tokens.PlatformColors) bool { return a == b })(
		rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform }))
	stepsObs := rx.SwitchMap(colors, func(c tokens.PlatformColors) rx.Observable[layout.Widget] {
		return button.FindBarSteps(th, button.FindBarStepsProps{
			HasQuery:            func() bool { return b.query != "" },
			Surface:             pageFill(c),
			PreviousDescription: "Previous",
			NextDescription:     "Next",
			OnPrevious:          func(gtx layout.Context) { b.step(gtx, -1) },
			OnNext:              func(gtx layout.Context) { b.step(gtx, 1) },
		})
	})
	doneObs := rx.SwitchMap(colors, func(c tokens.PlatformColors) rx.Observable[layout.Widget] {
		return button.Button(th, button.Props{
			Title:   "Done",
			Variant: button.FindBar,
			Surface: pageFill(c),
			OnClick: func(gtx layout.Context) { post(gtx, CloseFind{}) },
		})
	})
	b.parts = rx.CombineLatest3(fieldObs, stepsObs, doneObs)
	return b
}

// frame reads where the keyboard is and puts the marks on the guide or takes
// them off. The page calls it on each frame before the bar and the guide are
// laid out. The query and the current match stay when the marks go, so the
// marks come back when the keyboard goes back into the field.
func (b *findBar) frame(gtx layout.Context) {
	b.inField = b.field != nil && gtx.Focused(b.field)
	if b.inField && b.query != "" {
		b.doc.Find(b.query, b.current)
		return
	}
	b.doc.ClearFind()
}

// setQuery takes the field's new text. The first match becomes the current
// match and scrolls into view. An empty query takes every mark off.
func (b *findBar) setQuery(text string) {
	b.query = text
	b.current = 0
	if text == "" {
		b.doc.ClearFind()
		return
	}
	b.doc.Find(text, 0)
	b.doc.ScrollToMatch(0)
}

// step moves the current match by delta, from the last match to the first
// and from the first to the last, and scrolls it into view. It gives the
// keyboard to the field, because the marks show only while the field holds
// it.
func (b *findBar) step(gtx layout.Context, delta int) {
	if b.query == "" {
		return
	}
	b.doc.Find(b.query, b.current)
	n := len(b.doc.Matches())
	if n == 0 {
		return
	}
	b.current = ((b.current+delta)%n + n) % n
	b.doc.Find(b.query, b.current)
	b.doc.ScrollToMatch(b.current)
	if b.field != nil && !gtx.Focused(b.field) {
		gtx.Execute(key.FocusCmd{Tag: b.field})
	}
}

// count is the field's count: the number of matches, and nothing while the
// field does not hold the keyboard or holds no text. The reference measures
// the number alone, with no "1 of 8".
func (b *findBar) count() string {
	if !b.inField || b.query == "" {
		return ""
	}
	return strconv.Itoa(len(b.doc.Matches()))
}

// track puts the matches on the guide column's scroll bar: where each match
// lies, and which one is current. With no marks on the guide it puts none.
func track(bar scrollbar.Style, doc *markdown.Document, current int) scrollbar.Style {
	if places := doc.MatchPlaces(); len(places) > 0 {
		bar.Matches = places
		bar.Current = current
	}
	return bar
}

// renderFindBar is the static twin of the live bar, for goldens and review
// captures. field is the field's state; its variant and surface are set
// here. The previous and next control is enabled while the field holds text.
func renderFindBar(shaper *text.Shaper, c tokens.PlatformColors, typo tokens.Typography, field input.RenderState) layout.Widget {
	surface := pageFill(c)
	field.Variant = input.FindBar
	field.Surface = surface
	search := input.RenderSearch(shaper, findBarPrompt, c, tokens.Spacing, tokens.Radius, typo.SystemFont, tokens.Comfortable, field)
	seg := button.RenderState{Variant: button.FindBar, Surface: surface, Disabled: field.Text == ""}
	steps := func(gtx layout.Context) layout.Dimensions {
		return button.BorderedSegments(gtx, nil, c, tokens.Radius, tokens.TextStyle{}, tokens.Comfortable, []button.BorderedSegment{
			{Icon: icons.Mark(icons.ChevronLeft), State: seg},
			{Icon: icons.Mark(icons.ChevronRight), State: seg},
		})
	}
	done := button.Render(shaper, "Done", c, tokens.Spacing, tokens.Radius, typo.SmallSystemFont, tokens.Comfortable,
		button.RenderState{Variant: button.FindBar, Surface: surface})
	return func(gtx layout.Context) layout.Dimensions {
		return layoutFindBar(gtx, c, search, steps, done)
	}
}
