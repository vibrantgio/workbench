// detail.go renders the selected article in the right-hand pane of the
// articles/detail SplitPane (see feedsShellLayer for the layout choice and
// FEEDBACK-G5.2.md for the rationale). The pane is a header (title + meta)
// above a cadence/tabs strip with three tabs: Reader (paragraph-wrapped
// body), Raw (the same body in the Go Mono face), and Comments (a static
// placeholder list).
//
// The tabs instance is constructed ONCE; its Tab.Content closures are static
// per cadence/tabs' contract, so they read the selected article and theme
// tokens from atomic cells at frame time (the same layer-boundary adapter
// pattern as mainCell in app.go). Selection (which article, which tab) is
// model-derived: SelectArticle and SelectTab messages re-emit the layer via
// the observables this pane folds together, which is what repaints the
// window on the same frame as the click.
package main

import (
	"image"
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

	"github.com/vibrantgio/cadence/tabs"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// Detail pane tab indices, in tab-strip order.
const (
	tabReader = iota
	tabRaw
	tabComments
)

const detailPadDp = 16

// detailTokens is the colour/typography snapshot the static tab Content
// closures read at frame time.
type detailTokens struct {
	col tokens.ColorTokens
	typ tokens.TypeScale
}

// detailArticle is the selected-article snapshot stored in the article cell.
// ok=false renders the "select an article" placeholder.
type detailArticle struct {
	a  article
	ok bool
}

// detailPane composes the article detail view as an
// rx.Observable[layout.Widget] suitable for folding onto the shell's
// sidebar-driven stream. selectedArticleObs and selectedTabObs are derived
// from the MVU model; the tab-strip click lands a SelectTab mvu.MessageOp so
// the model — and this layer — advance on the same frame.
func detailPane(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	selectedArticleObs rx.Observable[ArticleID],
	selectedTabObs rx.Observable[int],
) rx.Observable[layout.Widget] {
	// Token mirror for the static tab Content closures and the header,
	// which run outside any rx.Defer scope (see articlesMain's tokenCell
	// for the pattern rationale).
	var tokenCell atomic.Value
	tokenCell.Store(detailTokens{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typeObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	_ = rx.CombineLatest2(colorObs, typeObs).Subscribe(rx.GoroutineContext(), func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			tokenCell.Store(detailTokens{col: t.First, typ: t.Second})
		}
	})
	loadTokens := func() detailTokens { return tokenCell.Load().(detailTokens) }

	// Selected-article cell. cadence/tabs captures Tab.Content widgets at
	// construction (a static slice, not an observable), so the closures
	// cannot receive the article in-band; they read this cell instead. The
	// cell is stored synchronously in the combined map below, BEFORE the
	// emitted widget can be laid out, so a frame never renders tabs for a
	// stale article. (Friction logged in FEEDBACK-G5.2.md.)
	var articleCell atomic.Value
	articleCell.Store(detailArticle{})
	loadArticle := func() detailArticle { return articleCell.Load().(detailArticle) }

	tabsObs := tabs.Tabs(th, tabs.Props{
		Tabs: []tabs.Tab{
			{Label: "Reader", Content: readerTab(shaper, loadTokens, loadArticle)},
			{Label: "Raw", Content: rawTab(shaper, loadTokens, loadArticle)},
			{Label: "Comments", Content: commentsTab(shaper, loadTokens)},
		},
		Selected: selectedTabObs,
		OnSelect: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: SelectTab{Idx: idx}}.Add(gtx.Ops)
		},
		Shaper: shaper,
	})

	return rx.Map(
		rx.CombineLatest2(tabsObs, selectedArticleObs),
		func(t rx.Tuple2[layout.Widget, ArticleID]) layout.Widget {
			a, ok := articleByID(t.Second)
			articleCell.Store(detailArticle{a: a, ok: ok})
			tabsW := t.First
			return func(gtx layout.Context) layout.Dimensions {
				return drawDetail(gtx, shaper, loadTokens(), loadArticle(), tabsW)
			}
		},
	)
}

// drawDetail lays the pane: placeholder when nothing is selected, otherwise
// title + meta header above the tab strip, which flexes to the remaining
// height.
func drawDetail(
	gtx layout.Context,
	shaper *text.Shaper,
	tok detailTokens,
	sel detailArticle,
	tabsW layout.Widget,
) layout.Dimensions {
	size := gtx.Constraints.Max
	if !sel.ok {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawLabel(gtx, shaper, "Select an article", unit.Sp(tok.typ.BodyLarge), mutedColor(tok.col.OnSurface))
		})
	}
	layout.UniformInset(unit.Dp(detailPadDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, shaper, sel.a.Title, unit.Sp(tok.typ.TitleLarge), tok.col.OnSurface)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				meta := sel.a.Author + " · " + sel.a.Published.Format("Jan 2 2006")
				return drawLabel(gtx, shaper, meta, unit.Sp(tok.typ.BodySmall), mutedColor(tok.col.OnSurface))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(1, tabsW),
		)
	})
	return layout.Dimensions{Size: size}
}

// readerTab renders the article body paragraph-wrapped in the proportional
// Go face. The closure is static (tabs captures it once) and reads the
// selected article + tokens from the cells at frame time.
func readerTab(shaper *text.Shaper, loadTokens func() detailTokens, loadArticle func() detailArticle) layout.Widget {
	return bodyTab(shaper, loadTokens, loadArticle, font.Font{})
}

// rawTab renders the SAME body bytes as readerTab in the Go Mono face —
// per the G5.2c spec the two tabs differ only in font.
func rawTab(shaper *text.Shaper, loadTokens func() detailTokens, loadArticle func() detailArticle) layout.Widget {
	return bodyTab(shaper, loadTokens, loadArticle, font.Font{Typeface: "Go Mono"})
}

func bodyTab(shaper *text.Shaper, loadTokens func() detailTokens, loadArticle func() detailArticle, f font.Font) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		sel := loadArticle()
		if !sel.ok {
			return layout.Dimensions{Size: size}
		}
		tok := loadTokens()
		layout.UniformInset(unit.Dp(detailPadDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawWrappedText(gtx, shaper, hardCodedBody(sel.a), f, unit.Sp(tok.typ.BodyMedium), tok.col.OnSurface)
		})
		return layout.Dimensions{Size: size}
	}
}

// commentsTab renders the static placeholder comment list. The rows are
// fixture data shared across all articles (per the G5.2c spec).
func commentsTab(shaper *text.Shaper, loadTokens func() detailTokens) layout.Widget {
	comments := hardCodedComments()
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		tok := loadTokens()
		layout.UniformInset(unit.Dp(detailPadDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2*len(comments))
			for _, c := range comments {
				c := c
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawLabel(gtx, shaper, c.Author, unit.Sp(tok.typ.LabelLarge), tok.col.Primary)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawWrappedText(gtx, shaper, c.Text, font.Font{}, unit.Sp(tok.typ.BodyMedium), tok.col.OnSurface)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
		return layout.Dimensions{Size: size}
	}
}

// drawWrappedText lays a multi-line label (MaxLines 0 = unlimited) wrapped
// at the current Max.X. The single-line drawLabel in app.go truncates; body
// text needs wrapping, which is the Reader tab's one formatting promise.
func drawWrappedText(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	f font.Font,
	size unit.Sp,
	c color.NRGBA,
) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	gtx.Constraints.Min = image.Point{}
	wl := widget.Label{}
	return wl.Layout(gtx, shaper, f, size, msg, material)
}

// mutedColor halves the alpha of c for secondary text (meta lines, the
// empty-pane placeholder).
func mutedColor(c color.NRGBA) color.NRGBA {
	c.A /= 2
	return c
}
