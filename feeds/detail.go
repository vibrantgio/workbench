// detail.go renders the selected article in the right-hand pane of the
// articles/detail SplitPane (see feedsShellLayer for the layout choice and
// FEEDBACK-G5.2.md for the rationale). The pane is a header (title + meta)
// above a patterns/tabs strip with three tabs: Reader (paragraph-wrapped
// body), Raw (the same body in the theme's Code style — the mono face), and
// Comments (a static placeholder list).
//
// The tabs instance is constructed ONCE; its Tab.Content closures are static
// per patterns/tabs' contract, so they read the selected article and theme
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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Detail pane tab indices, in tab-strip order.
const (
	tabReader = iota
	tabRaw
	tabComments
)

const detailPadDp = 16

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
	selectedArticleObs rx.Observable[ArticleID],
	selectedTabObs rx.Observable[int],
) rx.Observable[layout.Widget] {
	// Token mirror for the static tab Content closures and the header,
	// which run outside any rx.Defer scope (see articlesMain's mirror
	// for the pattern rationale).
	loadTokens := mirrorTokens(th)

	// Selected-article cell. patterns/tabs captures Tab.Content widgets at
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
			{Label: "Reader", Content: readerTab(loadTokens, loadArticle)},
			{Label: "Raw", Content: rawTab(loadTokens, loadArticle)},
			{Label: "Comments", Content: commentsTab(loadTokens)},
		},
		Selected: selectedTabObs,
		OnSelect: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: SelectTab{Idx: idx}}.Add(gtx.Ops)
		},
	})

	return rx.Map(
		rx.CombineLatest2(tabsObs, selectedArticleObs),
		func(t rx.Tuple2[layout.Widget, ArticleID]) layout.Widget {
			a, ok := articleByID(t.Second)
			articleCell.Store(detailArticle{a: a, ok: ok})
			tabsW := t.First
			return func(gtx layout.Context) layout.Dimensions {
				return drawDetail(gtx, loadTokens(), loadArticle(), tabsW)
			}
		},
	)
}

// drawDetail lays the pane: placeholder when nothing is selected, otherwise
// title + meta header above the tab strip, which flexes to the remaining
// height. Primary text sits on the Neutral ramp's 900 step, the meta line
// and the placeholder on the low-contrast 700 step (ADR-007).
func drawDetail(
	gtx layout.Context,
	tok themeTokens,
	sel detailArticle,
	tabsW layout.Widget,
) layout.Dimensions {
	size := gtx.Constraints.Max
	if !sel.ok {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawLabel(gtx, tok.shaper, "Select an article", tok.typ.BodyLarge, tok.col.Ramps.Neutral.Step(700))
		})
	}
	layout.UniformInset(unit.Dp(detailPadDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, sel.a.Title, tok.typ.TitleLarge, tok.col.Ramps.Neutral.Step(900))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				meta := sel.a.Author + " · " + sel.a.Published.Format("Jan 2 2006")
				return drawLabel(gtx, tok.shaper, meta, tok.typ.BodySmall, tok.col.Ramps.Neutral.Step(700))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(1, tabsW),
		)
	})
	return layout.Dimensions{Size: size}
}

// readerTab renders the article body paragraph-wrapped in the theme's
// BodyMedium role. The closure is static (tabs captures it once) and reads
// the selected article + tokens from the cells at frame time.
func readerTab(loadTokens func() themeTokens, loadArticle func() detailArticle) layout.Widget {
	return bodyTab(loadTokens, loadArticle, func(typ tokens.Typography) tokens.TextStyle {
		return typ.BodyMedium
	})
}

// rawTab renders the SAME body bytes as readerTab in the theme's Code style
// — BodyMedium's metrics on the mono face (Roboto Mono, G-F0). Per the
// G5.2c spec the two tabs differ only in font.
func rawTab(loadTokens func() themeTokens, loadArticle func() detailArticle) layout.Widget {
	return bodyTab(loadTokens, loadArticle, func(typ tokens.Typography) tokens.TextStyle {
		return typ.Code
	})
}

func bodyTab(loadTokens func() themeTokens, loadArticle func() detailArticle, pick func(tokens.Typography) tokens.TextStyle) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		sel := loadArticle()
		if !sel.ok {
			return layout.Dimensions{Size: size}
		}
		tok := loadTokens()
		layout.UniformInset(unit.Dp(detailPadDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawWrappedText(gtx, tok.shaper, hardCodedBody(sel.a), pick(tok.typ), tok.col.Ramps.Neutral.Step(900))
		})
		return layout.Dimensions{Size: size}
	}
}

// commentsTab renders the static placeholder comment list. The rows are
// fixture data shared across all articles (per the G5.2c spec).
func commentsTab(loadTokens func() themeTokens) layout.Widget {
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
						return drawLabel(gtx, tok.shaper, c.Author, tok.typ.LabelLarge, tok.col.Primary)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawWrappedText(gtx, tok.shaper, c.Text, tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(900))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
		return layout.Dimensions{Size: size}
	}
}

// drawWrappedText lays a multi-line label (MaxLines 0 = unlimited) in one
// Typography role, wrapped at the current Max.X. The single-line drawLabel
// in app.go truncates; body text needs wrapping, which is the Reader tab's
// one formatting promise.
func drawWrappedText(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	style tokens.TextStyle,
	c color.NRGBA,
) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	gtx.Constraints.Min = image.Point{}
	return typeset.Layout(gtx, shaper, typeset.Label(style, 0),
		typeset.Font(style, font.Normal), unit.Sp(style.Size), msg, material)
}
