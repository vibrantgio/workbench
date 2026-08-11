// docs_sidebar.go builds the accordion-grouped docs sidebar. The Patterns
// shell pattern accepts a sidebar.Props (flat Items + toggle); since the
// G5.1c milestone calls for phase-grouped sections with nested links, this
// file composes the sidebar itself from patterns/accordion — bypassing
// patterns/sidebar. The entry point is docsSidebar, which folds the
// accordion's open-state stream into the returned layer observable so a
// header click repaints on the same frame.

package main

import (
	"image"
	"image/color"

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

	"github.com/vibrantgio/patterns/accordion"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
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

// docsSidebarSections returns the static sidebar shape. Every link
// routes to a distinct page; docsPages (docs_content.go) is the
// authority on which pages exist.
func docsSidebarSections() []docsSidebarSection {
	return []docsSidebarSection{
		{
			Title: "Components",
			Links: []docsSidebarLink{
				{Label: "Getting started", Page: pageComponentsGettingStarted},
				{Label: "Tokens & theme", Page: pageComponentsTokens},
				{Label: "Primitives", Page: pageComponentsPrimitives},
			},
		},
		{
			Title: "Patterns",
			Links: []docsSidebarLink{
				{Label: "Patterns", Page: pagePatternsPatterns},
				{Label: "Shells", Page: pagePatternsShells},
			},
		},
		{
			Title: "Theme",
			Links: []docsSidebarLink{
				{Label: "Window & system", Page: pageThemeWindow},
				{Label: "Live theme", Page: pageThemeTheme},
			},
		},
		{
			Title: "Effects",
			Links: []docsSidebarLink{
				{Label: "Motion", Page: pageEffectsMotion},
				{Label: "Effects", Page: pageEffectsEffects},
			},
		},
		{
			Title: "MVU",
			Links: []docsSidebarLink{
				{Label: "The loop", Page: pageMVULoop},
				{Label: "Reactive window", Page: pageMVUWindow},
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
// ToggleAccordion mvu.MessageOp so the model updates on the same frame. No
// Shaper prop is passed — the accordion titles and the link rows shape with
// the theme's Typography.Shaper().
func docsSidebar(
	th rx.Observable[theme.Theme],
	openSectionsObs rx.Observable[map[int]bool],
) rx.Observable[layout.Widget] {
	sections := docsSidebarSections()

	accSections := make([]accordion.Section, len(sections))
	for i, sec := range sections {
		accSections[i] = accordion.Section{
			Title: sec.Title,
			Body:  linkListBody(th, sec.Links),
		}
	}

	// SingleOpen is false: the patterns accordion emits exactly one
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
	})

	// Fold the accordion widget stream into the returned layer observable via
	// CombineLatest. accObs re-emits whenever the open-section map (driven by
	// the MVU model) or a theme token changes, so a click that lands a
	// ToggleAccordion message re-emits this layer on the next frame — which is
	// what drives theme/window's Invalidate() and the same-frame repaint.
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
// frame as the click. The rows read the theme snapshot through the shared
// mirrorTokens adapter: BodySmall on the body-text ramp step, shaped with
// the theme's shaper.
func linkListBody(
	th rx.Observable[theme.Theme],
	links []docsSidebarLink,
) layout.Widget {
	loadTok := mirrorTokens(th)

	clicks := make([]widget.Clickable, len(links))
	return func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		for i := range links {
			if clicks[i].Clicked(gtx) {
				mvu.MessageOp{Message: SetRoute{Page: links[i].Page}}.Add(gtx.Ops)
			}
		}
		return drawLinkList(gtx, s, links, clicks)
	}
}

func drawLinkList(
	gtx layout.Context,
	tok themeTokens,
	links []docsSidebarLink,
	clicks []widget.Clickable,
) layout.Dimensions {
	size := gtx.Constraints.Max
	rowH := gtx.Dp(unit.Dp(docsLinkRowHDp))
	indent := gtx.Dp(unit.Dp(docsLinkIndentDp))

	for i, l := range links {
		off := image.Pt(indent, i*rowH)
		stk := op.Offset(off).Push(gtx.Ops)
		rowGtx := gtx
		rowGtx.Constraints = layout.Exact(image.Pt(size.X-indent, rowH))
		drawSidebarLink(rowGtx, tok.shaper, l.Label, clickForLink(clicks, i), tok.col.Ramps.Neutral.Step(900), tok.typ.BodySmall)
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
	style tokens.TextStyle,
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
		labelDims := typeset.Layout(labelGtx, shaper, typeset.Label(style, 1),
			typeset.Font(style, font.Normal), unit.Sp(style.Size), label, material)
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
