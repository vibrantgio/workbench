// detail.go renders the selected article as the window's content column —
// the document the list beside it chooses. It is a header (title + meta)
// above a patterns/tabs strip with three tabs: Reader (paragraph-wrapped
// body), Raw (the same body in the theme's Code style — the mono face), and
// Comments (a static placeholder list).
//
// The tabs instance is constructed ONCE; its Tab.Content closures are static
// per patterns/tabs' contract, so they read the selected article and theme
// tokens from atomic cells at frame time. Selection (which article, which
// tab) is model-derived: SelectArticle and SelectTab messages re-emit the
// layer via the observables this pane folds together, which is what repaints
// the window on the same frame as the click.
package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/tabs"
	vgcolor "github.com/vibrantgio/theme/color"
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

// detailPadDp is the content column's own inset, and so the one leading
// edge everything the column draws stands on: the headline, the byline, the
// tab strip's first title and the prose under it.
const detailPadDp = 16

// tabTitlePadDp is the padding patterns/tabs lays either side of a tab's
// title inside its cell, which is the spacing scale's S3; tabStripOutdentDp
// is what the strip's own box therefore stands leading of the column's
// inset, so that the first title lands on it. Insetting the tab pane's
// content by tabTitlePadDp brings the prose back to the same edge.
var (
	tabTitlePadDp     = unit.Dp(tokens.Spacing.S3)
	tabStripOutdentDp = unit.Dp(detailPadDp) - tabTitlePadDp
)

// detailArticle is the selected-article snapshot stored in the article cell.
// ok=false draws the column's empty state instead of a document.
type detailArticle struct {
	a  article
	ok bool
}

// detailPane composes the article detail view as an
// rx.Observable[layout.Widget] suitable for folding onto the shell's
// sidebar-driven stream. selectedArticleObs and selectedTabObs are derived
// from the MVU model; the tab-strip click lands a SelectTab mvu.MessageOp so
// the model — and this layer — advance on the same frame. bars is the
// window's "Show scroll bars" setting, which the column's bar is built from.
func detailPane(
	th rx.Observable[theme.Theme],
	selectedArticleObs rx.Observable[ArticleID],
	selectedTabObs rx.Observable[int],
	bars desktop.ScrollBarSetting,
) rx.Observable[layout.Widget] {
	// Token mirror for the static tab Content closures and the header, which
	// run outside any rx.Defer scope.
	loadTokens := mirrorTokens(th)

	// Selected-article cell. patterns/tabs captures each Tab.Content
	// layout.Widget at construction (a static slice, not an observable), so the
	// closures cannot receive the article in-band; they read this cell instead.
	// The cell is stored synchronously in the combined map below, BEFORE the
	// emitted layout.Widget can be laid out, so a frame never renders tabs for
	// a stale article.
	var articleCell atomic.Value
	articleCell.Store(detailArticle{})
	loadArticle := func() detailArticle { return articleCell.Load().(detailArticle) }

	tabsObs := tabs.Tabs(th, tabs.Props{
		Tabs: []tabs.Tab{
			{Title: "Reader", Content: readerTab(loadTokens, loadArticle, bars)},
			{Title: "Raw", Content: rawTab(loadTokens, loadArticle, bars)},
			{Title: "Comments", Content: commentsTab(loadTokens, bars)},
		},
		Selected: selectedTabObs,
		OnSelect: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: SelectTab{Idx: idx}}.Add(gtx.Ops)
		},
		// The strip has no fill of its own: it stands on the fill this
		// column paints, which drawDetail states below.
		Fill: detailFill,
	})

	return rx.Map(
		rx.CombineLatest2(tabsObs, selectedArticleObs),
		func(t rx.Tuple2[layout.Widget, ArticleID]) layout.Widget {
			a, ok := articleByID(t.Second)
			articleCell.Store(detailArticle{a: a, ok: ok})
			tabsW := t.First
			return func(gtx layout.Context) layout.Dimensions {
				return drawDetail(gtx, loadTokens(), bars, loadArticle(), tabsW)
			}
		},
	)
}

// drawDetail lays the pane: the column's empty state when nothing is
// selected, otherwise title + meta header above the tab strip, which flexes
// to the remaining height. The title and the body are the platform's label
// and the meta line its secondary label; the empty state spends what
// patterns/shell reads off the platform.
//
// The headline, the byline and the tab strip stay where they are; the
// active tab's content below the strip is the region that scrolls. The
// strip is not content and the active tab's content is, and a header above
// what scrolls spans the column's full width with no lane beside it. The
// strip's box therefore runs to the column's trailing edge, so the region
// under it does too and its bar is at that edge.
//
// The column paints its OWN fill first. It is the reading surface, so it
// wears ControlBackground — the same fill the arrangement paints the column
// with, stated here so the document is legible wherever it is laid out.
func drawDetail(
	gtx layout.Context,
	tok themeTokens,
	bars desktop.ScrollBarSetting,
	sel detailArticle,
	tabsW layout.Widget,
) layout.Dimensions {
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, tok.col.ControlBackground, clip.Rect{Max: size}.Op())
	if !sel.ok {
		return detailEmptyState(gtx, tok, detailBar(tok, bars), "No Article Selected")
	}
	// The column's parts are inset one by one rather than together, because
	// the tab strip's own cell padding stands its first title further in
	// than the box it is laid out in: the strip takes the column's inset
	// less that padding, and the four parts then share one leading edge.
	column := layout.Inset{Left: unit.Dp(detailPadDp), Right: unit.Dp(detailPadDp)}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(layout.Spacer{Height: unit.Dp(detailPadDp)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return column.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return typeset.Text(gtx, tok.shaper, sel.a.Title, tok.typ.TitleLarge,
					vgcolor.Flatten(tok.col.Label, tok.col.ControlBackground), 1)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return column.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				meta := sel.a.Author + " · " + sel.a.Published.Format("Jan 2 2006")
				return typeset.Text(gtx, tok.shaper, meta, tok.typ.BodySmall,
					vgcolor.Flatten(tok.col.SecondaryLabel, tok.col.ControlBackground), 1)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: tabStripOutdentDp}.Layout(gtx, tabsW)
		}),
	)
	return layout.Dimensions{Size: size}
}

// detailEmptyState draws the platform's empty state in the content column's
// FULL frame: the main slot's own box grown upward by the toolbar's share,
// which is the frame the line centres in rather than the row below the
// toolbar. The pattern carries the reading; the fill is the column's own.
//
// The slot is recorded and replayed under the toolbar's own ops — the window
// passes the arrangement MainFirst — so what stands in the toolbar paints
// over the strip this frame reaches up into. The frame is reduced by
// shell.EmptyStateFrame, so where the column's bar takes a lane the line
// centres in what the content beside the lane would occupy.
func detailEmptyState(gtx layout.Context, tok themeTokens, bar scrollbar.Style, msg string) layout.Dimensions {
	row := gtx.Constraints.Max
	toolbar := gtx.Dp(unit.Dp(shell.ToolbarDepth))
	full := gtx
	full.Constraints = layout.Exact(shell.EmptyStateFrame(gtx, image.Pt(row.X, row.Y+toolbar), bar))
	defer op.Offset(image.Pt(0, -toolbar)).Push(gtx.Ops).Pop()
	shell.EmptyState(full, tok.shaper, tok.col, tok.typ, tok.col.ControlBackground,
		shell.EmptyStateProps{Line: msg})
	return layout.Dimensions{Size: row}
}

// detailFill is the content column's own fill, for the scheme being drawn:
// the reading surface this column paints and everything laid out in it
// stands on, the tab strip included.
func detailFill(c tokens.PlatformColors) color.NRGBA { return c.ControlBackground }

// The tab pane needs no fill of its own: patterns/tabs draws its pane on the
// content fill, which is the fill this column already paints.

// detailBar is the content column's scrollbar: drawn over the column's own
// fill, and standing without fading where the reader asked for scroll bars
// always.
func detailBar(tok themeTokens, bars desktop.ScrollBarSetting) scrollbar.Style {
	return scrollBarStyle(scrollbar.FromTokens(tok.col, tok.col.ControlBackground), bars)
}

// tabPaneInset is what a tab's content is inset by inside the region that
// scrolls. On the leading edge it is the column's own inset less what the
// strip is outdented by, so the prose lands on the same leading edge as the
// headline above it. The region runs to the column's trailing edge, so the
// trailing inset is the column's own; top and bottom are the column's inset
// too. The inset is inside the region, so it scrolls with the content.
func tabPaneInset() layout.Inset {
	return layout.Inset{
		Top:    unit.Dp(detailPadDp),
		Bottom: unit.Dp(detailPadDp),
		Left:   tabTitlePadDp,
		Right:  unit.Dp(detailPadDp),
	}
}

// scrollingTab lays content out as the one block of a region that scrolls
// across the whole box gtx gives, with the column's bar at the region's
// trailing edge. The bar's anchor is what list.AnchorFor returns: a standing bar
// takes a lane and the content stops short of it, a fading bar lies over
// the content's trailing edge. content is inset by tabPaneInset inside the
// region, and its height is its own, so a tab taller than the region
// scrolls to its last line.
func scrollingTab(gtx layout.Context, bar scrollbar.Style, scroll *list.State, content layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	list.LayoutScrollbar(gtx, scroll, bar, list.AnchorFor(bar), []struct{}{{}},
		func(gtx layout.Context, _ struct{}) layout.Dimensions {
			gtx.Constraints.Min.Y = 0
			return tabPaneInset().Layout(gtx, content)
		})
	return layout.Dimensions{Size: size}
}

// readerTab renders the article body paragraph-wrapped in the theme's
// BodyMedium role. The closure is static (tabs captures it once) and reads
// the selected article + tokens from the cells at frame time.
func readerTab(loadTokens func() themeTokens, loadArticle func() detailArticle, bars desktop.ScrollBarSetting) layout.Widget {
	return bodyTab(loadTokens, loadArticle, bars, func(typ tokens.Typography) tokens.TextStyle {
		return typ.BodyMedium
	})
}

// rawTab renders the SAME body bytes as readerTab in the theme's Code style
// — BodyMedium's metrics on the mono face (Roboto Mono). The two tabs differ
// only in font.
func rawTab(loadTokens func() themeTokens, loadArticle func() detailArticle, bars desktop.ScrollBarSetting) layout.Widget {
	return bodyTab(loadTokens, loadArticle, bars, func(typ tokens.Typography) tokens.TextStyle {
		return typ.Code
	})
}

// bodyTab is the article's body in one style, in a region that scrolls.
// Each tab keeps its own scroll position, and a newly chosen article opens
// at its first line.
func bodyTab(loadTokens func() themeTokens, loadArticle func() detailArticle, bars desktop.ScrollBarSetting, pick func(tokens.Typography) tokens.TextStyle) layout.Widget {
	scroll := list.NewState()
	var shown ArticleID
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		tok := loadTokens()
		sel := loadArticle()
		if !sel.ok {
			return layout.Dimensions{Size: size}
		}
		if sel.a.ID != shown {
			shown = sel.a.ID
			scroll.ScrollToStart()
		}
		return drawBody(gtx, tok, detailBar(tok, bars), scroll, sel.a, pick(tok.typ))
	}
}

// drawBody lays a's body out in style in a region that scrolls under bar.
func drawBody(gtx layout.Context, tok themeTokens, bar scrollbar.Style, scroll *list.State, a article, style tokens.TextStyle) layout.Dimensions {
	return scrollingTab(gtx, bar, scroll, func(gtx layout.Context) layout.Dimensions {
		return drawWrappedText(gtx, tok.shaper, hardCodedBody(a), style,
			vgcolor.Flatten(tok.col.Label, tok.col.ControlBackground))
	})
}

// commentsTab renders the static placeholder comment list. The rows are
// fixture data shared across all articles, in a region that scrolls.
func commentsTab(loadTokens func() themeTokens, bars desktop.ScrollBarSetting) layout.Widget {
	comments := hardCodedComments()
	scroll := list.NewState()
	return func(gtx layout.Context) layout.Dimensions {
		tok := loadTokens()
		return scrollingTab(gtx, detailBar(tok, bars), scroll, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2*len(comments))
			for _, c := range comments {
				c := c
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return typeset.Text(gtx, tok.shaper, c.Author, tok.typ.LabelLarge,
							vgcolor.Flatten(tok.col.Label, tok.col.ControlBackground), 1)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawWrappedText(gtx, tok.shaper, c.Text, tok.typ.BodyMedium,
							vgcolor.Flatten(tok.col.SecondaryLabel, tok.col.ControlBackground))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	}
}

// drawWrappedText lays text out in one Typography role wrapped at the current
// Max.X, on as many lines as it takes. The minimum is relaxed first, so the
// result is the text's own size rather than the slot it was offered.
func drawWrappedText(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	style tokens.TextStyle,
	c color.NRGBA,
) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	return typeset.Text(gtx, shaper, msg, style, c, 0)
}
