// docs_sidebar.go builds the accordion-grouped docs sidebar. The Cadence
// shell pattern accepts a sidebar.Props (flat Items + toggle); since the
// G5.1c milestone calls for phase-grouped sections with nested links, this
// file composes the sidebar itself from cadence/accordion — bypassing
// cadence/sidebar. The entry point is docsSidebar, which folds the
// accordion's open-state stream into the returned layer observable so a
// header click repaints on the same frame.

package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/accordion"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// Sidebar layout constants.
const (
	docsSidebarWidthDp = 192
	docsLinkRowHDp     = 28
	docsLinkIndentDp   = 24
)

// docsSidebarLink is one navigable entry inside a phase section.
type docsSidebarLink struct {
	Label string
	Page  string
}

// docsSidebarSection is one accordion-grouped phase section.
type docsSidebarSection struct {
	Title string
	Links []docsSidebarLink
}

// docsSidebarSections returns the static sidebar shape.
func docsSidebarSections() []docsSidebarSection {
	return []docsSidebarSection{
		{
			Title: "Prism",
			Links: []docsSidebarLink{
				{Label: "Getting started", Page: pageDocsGettingStarted},
				{Label: "Tokens & theme", Page: pageDocsPhasesOverview},
				{Label: "Components", Page: pageDocsComponentRef},
			},
		},
		{
			Title: "Cadence",
			Links: []docsSidebarLink{
				{Label: "Patterns overview", Page: pageDocsPhasesOverview},
				{Label: "Pattern reference", Page: pageDocsComponentRef},
			},
		},
		{
			Title: "Spectrum",
			Links: []docsSidebarLink{
				{Label: "System glue", Page: pageDocsPhasesOverview},
				{Label: "Live theme", Page: pageDocsComponentRef},
			},
		},
		{
			Title: "Pulse",
			Links: []docsSidebarLink{
				{Label: "Motion overview", Page: pageDocsPhasesOverview},
				{Label: "Effects reference", Page: pageDocsComponentRef},
			},
		},
	}
}

// copyOpenMap returns a shallow copy of m.
func copyOpenMap(m map[int]bool) map[int]bool {
	cp := make(map[int]bool, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// docsSidebar returns the route-ready docs sidebar observable. openSectionsObs
// streams the current open-section map from the MVU model; OnToggle emits a
// ToggleAccordion mvu.MessageOp so the model updates on the same frame.
func docsSidebar(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	openSectionsObs rx.Observable[map[int]bool],
) rx.Observable[layout.Widget] {
	sections := docsSidebarSections()

	accSections := make([]accordion.Section, len(sections))
	for i, sec := range sections {
		accSections[i] = accordion.Section{
			Title: sec.Title,
			Body:  linkListBody(th, shaper, sec.Links),
		}
	}

	// SingleOpen is false: the cadence accordion emits exactly one
	// ToggleAccordion per click, and sitedocs.Update owns the single-open
	// invariant (opening a section closes its peers). One message per click
	// keeps the model update — and the same-frame repaint it drives — to a
	// single hop, rather than the N+1 OnToggle calls SingleOpen mode fires.
	accObs := accordion.Accordion(th, accordion.Props{
		Sections: accSections,
		Open:     openSectionsObs,
		OnToggle: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: ToggleAccordion{Idx: idx}}.Add(gtx.Ops)
		},
		SingleOpen: false,
		Shaper:     shaper,
	})

	// Fold the accordion widget stream into the returned layer observable via
	// CombineLatest. accObs re-emits whenever the open-section map (driven by
	// the MVU model) or a theme token changes, so a click that lands a
	// ToggleAccordion message re-emits this layer on the next frame — which is
	// what drives spectrum/window's Invalidate() and the same-frame repaint.
	// The former atomic.Value mirror severed accObs from the layer chain, so
	// open-state changes never reached Invalidate (the FEEDBACK-G5.1 lag bug).
	colorsObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(rx.CombineLatest2(accObs, colorsObs), func(n rx.Tuple2[layout.Widget, tokens.ColorTokens]) layout.Widget {
		accW, c := n.First, n.Second
		return func(gtx layout.Context) layout.Dimensions {
			return drawDocsSidebar(gtx, accW, c)
		}
	})
}

// drawDocsSidebar paints the sidebar column: a Surface background plus the
// accordion widget supplied by the combined layer observable.
func drawDocsSidebar(
	gtx layout.Context,
	accW layout.Widget,
	colors tokens.ColorTokens,
) layout.Dimensions {
	w := gtx.Dp(unit.Dp(docsSidebarWidthDp))
	h := gtx.Constraints.Max.Y
	size := image.Pt(w, h)
	paint.FillShape(gtx.Ops, colors.Surface, clip.Rect{Max: size}.Op())

	gtx.Constraints = layout.Exact(size)
	if accW != nil {
		accW(gtx)
	}
	return layout.Dimensions{Size: size}
}

// linkListBody returns the body widget for a single accordion section.
// Clicks emit mvu.MessageOp{SetRoute{...}} so navigation fires on the same
// frame as the click.
func linkListBody(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	links []docsSidebarLink,
) layout.Widget {
	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })

	var state atomic.Value
	state.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	_ = rx.CombineLatest2(colObs, typObs).Subscribe(func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			state.Store(tokenState{col: t.First, typ: t.Second})
		}
	}, rx.Goroutine)

	clicks := make([]widget.Clickable, len(links))
	return func(gtx layout.Context) layout.Dimensions {
		s := state.Load().(tokenState)
		for i := range links {
			if clicks[i].Clicked(gtx) {
				mvu.MessageOp{Message: SetRoute{Page: links[i].Page}}.Add(gtx.Ops)
			}
		}
		return drawLinkList(gtx, shaper, links, clicks, s.col, s.typ)
	}
}

func drawLinkList(
	gtx layout.Context,
	shaper *text.Shaper,
	links []docsSidebarLink,
	clicks []widget.Clickable,
	colors tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Dimensions {
	size := gtx.Constraints.Max
	rowH := gtx.Dp(unit.Dp(docsLinkRowHDp))
	indent := gtx.Dp(unit.Dp(docsLinkIndentDp))

	for i, l := range links {
		off := image.Pt(indent, i*rowH)
		stk := op.Offset(off).Push(gtx.Ops)
		rowGtx := gtx
		rowGtx.Constraints = layout.Exact(image.Pt(size.X-indent, rowH))
		drawSidebarLink(rowGtx, shaper, l.Label, clickForLink(clicks, i), colors.OnSurface, ts)
		stk.Pop()
	}
	return layout.Dimensions{Size: size}
}

func clickForLink(clicks []widget.Clickable, i int) *widget.Clickable {
	if clicks == nil || i >= len(clicks) {
		return nil
	}
	return &clicks[i]
}

func drawSidebarLink(
	gtx layout.Context,
	shaper *text.Shaper,
	label string,
	click *widget.Clickable,
	fg color.NRGBA,
	ts tokens.TypeScale,
) layout.Dimensions {
	size := gtx.Constraints.Max
	inner := func(gtx layout.Context) layout.Dimensions {
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := mColor.Stop()

		labelGtx := gtx
		labelGtx.Constraints.Min = image.Point{}
		labelGtx.Constraints.Max = size

		mLabel := op.Record(gtx.Ops)
		wl := widget.Label{MaxLines: 1}
		labelDims := wl.Layout(labelGtx, shaper, font.Font{}, unit.Sp(ts.BodySmall), label, material)
		labelCall := mLabel.Stop()

		offY := (size.Y - labelDims.Size.Y) / 2
		if offY < 0 {
			offY = 0
		}
		stk := op.Offset(image.Pt(0, offY)).Push(gtx.Ops)
		labelCall.Add(gtx.Ops)
		stk.Pop()

		return layout.Dimensions{Size: size}
	}
	if click == nil {
		return inner(gtx)
	}
	gtx.Constraints = layout.Exact(size)
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		return inner(gtx)
	})
}
