// docs.go composes the three docs pages — Getting started, Phases
// overview, Component reference. Each page stacks: a cadence/breadcrumb
// row at the top, a vertically scrollable prose section in the middle,
// and a sequence of cadence/card-wrapped code samples beneath. The
// runtime entry point is docsPage; the static counterpart renderDocsPage
// is used by goldens.

package main

import (
	"image/color"
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/breadcrumb"
	"github.com/vibrantgio/cadence/card"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"

	"github.com/vibrantgio/mvu"
)

// Page layout constants. The outer inset matches the marketing-pattern
// inset (S6 = 24 dp) so the docs page reads at the same canvas inset as
// the landing page.
const (
	docsOuterInsetDp = 24
	docsProseGapDp   = 12
	docsCardGapDp    = 16
	docsCodeRowHDp   = 18
	docsCardHeightDp = 120
)

// docsPageContent is the raw material for a single docs page.
type docsPageContent struct {
	// Layer names the ecosystem layer the page documents (Prism,
	// Cadence, Spectrum, Pulse, MVU); it becomes the middle breadcrumb.
	Layer      string
	Title      string
	Paragraphs []string
	Codes      []docsCodeSample

	// BreadcrumbLabels, when set to exactly three entries, overrides the
	// default Home / Layer / Title labels used by the renderDocsPage path.
	BreadcrumbLabels []string
}

type docsCodeSample struct {
	Caption string
	Lines   []string
}

// docsPage returns the runtime observable for the named page. All child
// observables (breadcrumb, cards) are combined via CombineLatest so the
// page re-emits on any theme change. The returned observable stays alive
// for the program's lifetime so scroll position is preserved.
func docsPage(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	content docsPageContent,
) rx.Observable[layout.Widget] {
	bcItems := docsBreadcrumb(content)
	bcObs := breadcrumb.Breadcrumb(th, breadcrumb.Props{
		Items:  bcItems,
		Shaper: shaper,
	})

	// Build card observables. Each card is a separate observable that re-emits
	// on theme change.
	cardObss := make([]rx.Observable[layout.Widget], len(content.Codes))
	for i, code := range content.Codes {
		cs := code
		cardObss[i] = card.Card(th, card.Props{
			Header: codeCaptionWidget(th, shaper, cs.Caption),
			Body:   codeBodyWidget(th, shaper, cs.Lines),
		})
	}

	// Resolve color and type tokens for prose rendering.
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })

	list := &layout.List{Axis: layout.Vertical}

	// Combine all child observables: breadcrumb + cards + color tokens + type tokens.
	// CombineLatest re-emits whenever any input changes (e.g. theme change).
	allObs := make([]rx.Observable[layout.Widget], 0, 1+len(cardObss))
	allObs = append(allObs, bcObs)
	allObs = append(allObs, cardObss...)

	combined := rx.CombineLatest(allObs...)
	combined2 := rx.CombineLatest2(colObs, typObs)

	// Merge both combined streams. When either changes, rebuild the widget.
	// We need all four: bcWidget, cardWidgets, colors, typeScale.
	// Use a two-level approach: combine layout widgets first, then combine with tokens.
	type widgetState struct {
		bc    layout.Widget
		cards []layout.Widget
	}
	widgetObs := rx.Map(combined, func(ws []layout.Widget) widgetState {
		return widgetState{
			bc:    ws[0],
			cards: ws[1:],
		}
	})

	type fullState struct {
		ws  widgetState
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	type tokenPair struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	tokenObs := rx.Map(combined2, func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale]) tokenPair {
		return tokenPair{col: t.First, typ: t.Second}
	})

	// CombineLatest2 over widget state and token pair.
	full := rx.CombineLatest2(widgetObs, tokenObs)
	return rx.Map(full, func(t rx.Tuple2[widgetState, tokenPair]) layout.Widget {
		ws := t.First
		tok := t.Second
		bcW := ws.bc
		cardWs := ws.cards
		c := tok.col
		ts := tok.typ
		return func(gtx layout.Context) layout.Dimensions {
			return drawDocsPage(gtx, list, bcW, cardWs, content, shaper, c, ts)
		}
	})
}

// renderDocsPage is the static counterpart of docsPage used by goldens.
func renderDocsPage(
	shaper *text.Shaper,
	content docsPageContent,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	ts tokens.TypeScale,
) layout.Widget {
	bcItems := renderBreadcrumbItems(content)
	bcW := breadcrumb.Render(shaper, breadcrumb.Props{Items: bcItems, Shaper: shaper}, colors, sp, ts)

	cardWs := make([]layout.Widget, len(content.Codes))
	for i, cs := range content.Codes {
		cardWs[i] = card.Render(card.Props{
			Header: renderCodeCaption(shaper, cs.Caption, colors, ts),
			Body:   renderCodeBody(shaper, cs.Lines, colors, ts),
		}, colors, sp, rad)
	}
	return func(gtx layout.Context) layout.Dimensions {
		return drawDocsPageStatic(gtx, bcW, cardWs, content, shaper, colors, ts)
	}
}

// docsBreadcrumb returns the breadcrumb trail for a docs page: Home
// (clickable) / layer / title. Callbacks emit mvu.MessageOp so
// navigation fires on the same frame as the click.
func docsBreadcrumb(content docsPageContent) []breadcrumb.Item {
	layer := content.Layer
	if layer == "" {
		layer = "Docs"
	}
	return []breadcrumb.Item{
		{Label: "Home", OnClick: func(gtx layout.Context) {
			mvu.MessageOp{Message: SetRoute{Page: pageHome}}.Add(gtx.Ops)
		}},
		{Label: layer},
		{Label: content.Title},
	}
}

// renderBreadcrumbItems is the golden-safe counterpart of docsBreadcrumb.
func renderBreadcrumbItems(content docsPageContent) []breadcrumb.Item {
	if labels := content.BreadcrumbLabels; len(labels) == 3 {
		return []breadcrumb.Item{
			{Label: labels[0]},
			{Label: labels[1]},
			{Label: labels[2]},
		}
	}
	layer := content.Layer
	if layer == "" {
		layer = "Docs"
	}
	return []breadcrumb.Item{
		{Label: "Home"},
		{Label: layer},
		{Label: content.Title},
	}
}

func drawDocsPage(
	gtx layout.Context,
	list *layout.List,
	bcW layout.Widget,
	cardWs []layout.Widget,
	content docsPageContent,
	shaper *text.Shaper,
	colors tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Dimensions {
	inset := pllayout.Inset(docsOuterInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if bcW != nil {
					return bcW(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(pllayout.VSpacer(docsCardGapDp)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return docsScrollBody(gtx, list, cardWs, content, shaper, colors, ts)
			}),
		)
	})
}

func drawDocsPageStatic(
	gtx layout.Context,
	bcW layout.Widget,
	cardWs []layout.Widget,
	content docsPageContent,
	shaper *text.Shaper,
	colors tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Dimensions {
	inset := pllayout.Inset(docsOuterInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(bcW),
			layout.Rigid(pllayout.VSpacer(docsCardGapDp)),
		}
		for _, p := range content.Paragraphs {
			children = append(children,
				layout.Rigid(paragraphWidget(shaper, p, colors.OnSurface, ts)),
				layout.Rigid(pllayout.VSpacer(docsProseGapDp)),
			)
		}
		for _, cardW := range cardWs {
			children = append(children,
				layout.Rigid(fixedHeight(docsCardHeightDp, cardW)),
				layout.Rigid(pllayout.VSpacer(docsCardGapDp)),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// docsScrollBody renders the prose + cards as a scrollable layout.List.
func docsScrollBody(
	gtx layout.Context,
	list *layout.List,
	cardWs []layout.Widget,
	content docsPageContent,
	shaper *text.Shaper,
	colors tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Dimensions {
	type itemKind int
	const (
		kindParagraph itemKind = iota
		kindCard
	)
	type item struct {
		kind itemKind
		text string
		card layout.Widget
	}
	items := make([]item, 0, len(content.Paragraphs)+len(content.Codes))
	for _, p := range content.Paragraphs {
		items = append(items, item{kind: kindParagraph, text: p})
	}
	for _, cw := range cardWs {
		items = append(items, item{kind: kindCard, card: cw})
	}

	return list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		it := items[i]
		switch it.kind {
		case kindParagraph:
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(paragraphWidget(shaper, it.text, colors.OnSurface, ts)),
				layout.Rigid(pllayout.VSpacer(docsProseGapDp)),
			)
		default:
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if it.card == nil {
						return layout.Dimensions{Size: gtx.Constraints.Max}
					}
					return fixedHeight(docsCardHeightDp, it.card)(gtx)
				}),
				layout.Rigid(pllayout.VSpacer(docsCardGapDp)),
			)
		}
	})
}

// fixedHeight wraps a widget so its layout is forced to a fixed dp height.
func fixedHeight(dp float32, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(unit.Dp(dp))
		gtx.Constraints.Min.Y = h
		gtx.Constraints.Max.Y = h
		return w(gtx)
	}
}

// paragraphWidget renders one body-text paragraph at BodyMedium font size.
func paragraphWidget(
	shaper *text.Shaper,
	textBody string,
	fg color.NRGBA,
	ts tokens.TypeScale,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := mColor.Stop()
		wl := widget.Label{Alignment: text.Start}
		return wl.Layout(gtx, shaper, font.Font{}, unit.Sp(ts.BodyMedium), textBody, material)
	}
}

// codeCaptionWidget returns the rx-driven Header for a code card.
func codeCaptionWidget(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	caption string,
) layout.Widget {
	if caption == "" {
		return nil
	}
	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	combined := rx.CombineLatest2(colObs, typObs)

	var state atomic.Value
	state.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	_ = combined.Subscribe(func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			state.Store(tokenState{col: t.First, typ: t.Second})
		}
	}, rx.Goroutine)
	return func(gtx layout.Context) layout.Dimensions {
		s := state.Load().(tokenState)
		return renderCodeCaption(shaper, caption, s.col, s.typ)(gtx)
	}
}

func renderCodeCaption(
	shaper *text.Shaper,
	caption string,
	colors tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Widget {
	if caption == "" {
		return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	return func(gtx layout.Context) layout.Dimensions {
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: colors.OnSurfaceVariant}.Add(gtx.Ops)
		material := mColor.Stop()
		wl := widget.Label{MaxLines: 1}
		return wl.Layout(gtx, shaper, font.Font{}, unit.Sp(ts.LabelLarge), caption, material)
	}
}

// codeBodyWidget returns the rx-driven Body for a code card.
func codeBodyWidget(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	lines []string,
) layout.Widget {
	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	combined := rx.CombineLatest2(colObs, typObs)

	var state atomic.Value
	state.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	_ = combined.Subscribe(func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			state.Store(tokenState{col: t.First, typ: t.Second})
		}
	}, rx.Goroutine)
	return func(gtx layout.Context) layout.Dimensions {
		s := state.Load().(tokenState)
		return renderCodeBody(shaper, lines, s.col, s.typ)(gtx)
	}
}

func renderCodeBody(
	shaper *text.Shaper,
	lines []string,
	colors tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(lines))
		for _, line := range lines {
			l := line
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return codeLineWidget(gtx, shaper, l, colors.OnSurface, ts)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

func codeLineWidget(
	gtx layout.Context,
	shaper *text.Shaper,
	line string,
	fg color.NRGBA,
	ts tokens.TypeScale,
) layout.Dimensions {
	mColor := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := mColor.Stop()
	wl := widget.Label{MaxLines: 1}
	monoFont := font.Font{Typeface: "Go Mono"}
	return wl.Layout(gtx, shaper, monoFont, unit.Sp(ts.BodySmall), line, material)
}
