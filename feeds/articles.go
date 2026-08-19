package main

import (
	"image"
	"sort"
	"strings"
	"sync/atomic"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/components/keyed"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/pagination"
	"github.com/vibrantgio/patterns/table"
	"github.com/vibrantgio/patterns/tooltip"
	"github.com/vibrantgio/theme/theme"
)

// defaultRowsPerPage is the seed row count per pagination page. It is no
// longer fixed: Model.rowsPerPage carries it and the Preferences panel edits
// it live (see preferences.go).
const defaultRowsPerPage = 10

// rowsPerPageChoices are the page sizes the Preferences panel offers. A
// short closed set is what makes the preference a row of buttons rather
// than a number field with a validation story.
var rowsPerPageChoices = []int{5, 10, 25}

// Sortable column indices for the articles table.
const (
	colTitle     = 0
	colAuthor    = 1
	colPublished = 2
	colUnread    = 3
)

// Geometry shared by the Unread tooltip overlay. unreadColWDp matches the
// Unread column's pinned Width; tableHeaderHDp mirrors the header band's
// height at Comfortable density (Density.ControlHeight, the E1.4 row rule) —
// the table draws its header internally and exposes no per-header widget
// hook, so the tooltip hit area is positioned by arithmetic over these
// constants (friction logged in FEEDBACK-G5.2.md).
const (
	unreadColWDp   = 96
	tableHeaderHDp = 36
)

// cellPadDp is the horizontal cell padding themedTextCell applies — 12 dp,
// mirroring patterns/table's own cell padding so app-built cells sit flush
// with the table's stock geometry.
const cellPadDp = 12

// filterAndSortArticles is the pure transform composed inside the table's
// Items pipeline. Filtering matches the lower-cased query against Title and
// Author. Sort handles only the Sortable columns (Title, Published); other
// column indices leave the slice in its scan order.
func filterAndSortArticles(all []article, feed FeedID, query string, sk table.Sort) []article {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]article, 0, len(all))
	for _, a := range all {
		if feed != "" && a.FeedID != feed {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(a.Title), q) &&
			!strings.Contains(strings.ToLower(a.Author), q) {
			continue
		}
		out = append(out, a)
	}
	switch sk.Column {
	case colTitle:
		sort.SliceStable(out, func(i, j int) bool {
			if sk.Asc {
				return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
			}
			return strings.ToLower(out[i].Title) > strings.ToLower(out[j].Title)
		})
	case colPublished:
		sort.SliceStable(out, func(i, j int) bool {
			if sk.Asc {
				return out[i].Published.Before(out[j].Published)
			}
			return out[i].Published.After(out[j].Published)
		})
	}
	return out
}

// unreadOnlyArticles drops the read articles when the unread-only reading
// preference is on, and returns arts untouched when it is off. It is a
// separate pass rather than a fifth parameter to filterAndSortArticles
// because it is a PREFERENCE, applied to whatever that query-and-sort
// transform produced, and the two have different lifetimes: the filter text
// is per-keystroke UI state, the preference outlives the session.
func unreadOnlyArticles(arts []article, unreadOnly bool) []article {
	if !unreadOnly {
		return arts
	}
	out := make([]article, 0, len(arts))
	for _, a := range arts {
		if a.Unread {
			out = append(out, a)
		}
	}
	return out
}

// pageSlice returns the [start, end) window of arts corresponding to
// 1-indexed page at the given page size. Out-of-range pages return an
// empty slice; the consumer is responsible for clamping its page state.
func pageSlice(arts []article, page, size int) []article {
	if page < 1 || size < 1 {
		return nil
	}
	start := (page - 1) * size
	if start >= len(arts) {
		return nil
	}
	end := start + size
	if end > len(arts) {
		end = len(arts)
	}
	return arts[start:end]
}

// pageCountFor returns the number of pages required to display arts at the
// given page size, with a minimum of 1 so pagination always shows page 1.
func pageCountFor(arts []article, size int) int {
	if size < 1 {
		return 1
	}
	n := (len(arts) + size - 1) / size
	if n < 1 {
		return 1
	}
	return n
}

// articlesMain composes the textfield filter, articles table, and pagination
// row into an rx.Observable[layout.Widget] suitable for folding onto the
// shell's sidebar-driven stream. Selection (selectedFeedObs), paging
// (currentPageObs), sort (sortObs) and the filter text (filterObs) are all
// derived from the MVU model; every interactive callback lands an
// mvu.MessageOp so a click re-emits this layer — and the shell — on the same
// frame.
//
// tipArb is this window's tooltip arbitration set (ADR-008): the Unread
// header tooltip joins it so that at most one tooltip in the window is up.
func articlesMain(
	th rx.Observable[theme.Theme],
	selectedFeedObs rx.Observable[FeedID],
	currentPageObs rx.Observable[int],
	sortObs rx.Observable[table.Sort],
	rowsPerPageObs rx.Observable[int],
	unreadOnlyObs rx.Observable[bool],
	filterObs rx.Observable[string],
	tipArb *tooltip.Arbiter,
) rx.Observable[layout.Widget] {
	all := hardCodedArticles()

	// Token mirror so the table column Cell closures (which run outside any
	// rx.Defer scope) can read current colours/typography — and the theme's
	// shaper — on each frame without crossing scheduler boundaries.
	loadTok := mirrorTokens(th)

	// Sort mirror so onSort can read the current sort (from the model) to
	// decide whether to flip the Asc bit or start a fresh Asc on a different
	// column. An atomic.Value fed by the model-derived sortObs — the click
	// callback reads the latest model value, then emits a SetSort message that
	// re-emits this layer.
	var sortCell atomic.Value
	sortCell.Store(table.Sort{Column: colPublished, Asc: false})
	_ = sortObs.Subscribe(rx.GoroutineContext(), func(s table.Sort, _ error, done bool) {
		if !done {
			sortCell.Store(s)
		}
	})

	// Per-article widget.Clickable registry. The Deferred lives for the
	// program lifetime — feeds is a single-window app and articlesMain is
	// called once from feedsShellLayer, so no rx.Defer scope is required.
	rowClicks := keyed.Defer(func(_ ArticleID) *widget.Clickable {
		return &widget.Clickable{}
	})

	columns := articleColumns(loadTok, rowClicks)

	onSort := func(gtx layout.Context, col int) {
		cur, _ := sortCell.Load().(table.Sort)
		if cur.Column == col {
			mvu.MessageOp{Message: SetSort{Sort: table.Sort{Column: col, Asc: !cur.Asc}}}.Add(gtx.Ops)
			return
		}
		mvu.MessageOp{Message: SetSort{Sort: table.Sort{Column: col, Asc: true}}}.Add(gtx.Ops)
	}

	// The reading preferences join the pipeline as ordinary model-derived
	// streams: unread-only narrows what `filtered` holds, rows-per-page
	// resizes the window `paged` cuts out of it and the count the pagination
	// row draws. Because they are model state like any other, changing one in
	// the Preferences panel repaginates the table on the same frame, with the
	// panel still open over it — which is what "applies live" means here and
	// why the panel needs no Save.
	filtered := rx.Map(
		rx.CombineLatest4(selectedFeedObs, filterObs, sortObs, unreadOnlyObs),
		func(t rx.Tuple4[FeedID, string, table.Sort, bool]) []article {
			return unreadOnlyArticles(filterAndSortArticles(all, t.First, t.Second, t.Third), t.Fourth)
		},
	)
	paged := rx.Map(
		rx.CombineLatest3(filtered, currentPageObs, rowsPerPageObs),
		func(t rx.Tuple3[[]article, int, int]) []article {
			return pageSlice(t.First, t.Second, t.Third)
		},
	)
	pageCountObs := rx.Map(
		rx.CombineLatest2(filtered, rowsPerPageObs),
		func(t rx.Tuple2[[]article, int]) int {
			return pageCountFor(t.First, t.Second)
		},
	)

	// Hover tooltip for the icon-only Unread ("•") column header. The
	// trigger fills whatever canvas it is given; the overlay wrapper in
	// articlesLayout positions that canvas over the header cell, since the
	// table draws its headers internally and offers no widget slot there.
	unreadTipObs := tooltip.Tooltip(th, tooltip.Props{
		Text:      "Unread",
		Placement: tooltip.Bottom,
		Arbiter:   tipArb,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
	})

	filterWidgetObs := input.TextField(th, input.TextFieldProps{
		Placeholder: "Filter articles",
		Description: "Filter articles by title or author",
		OnChange: func(gtx layout.Context, s string) {
			// One message carries both halves: the reducer stores the text
			// and resets to page 1, because narrowing the filter shrinks the
			// result set and would otherwise strand the user on an
			// out-of-range slice.
			mvu.MessageOp{Message: SetFilter{Text: s}}.Add(gtx.Ops)
		},
	})
	tableWidgetObs := table.Table(th, table.Props[article]{
		Columns: columns,
		Items:   paged,
		Sort:    sortObs,
		OnSort:  onSort,
	})
	// pagination.Props takes Page/PageCount as static ints; CombineLatest holds
	// the latest page + page count from the model-derived streams and rebuilds
	// the row each emission, so the active highlight tracks model state. The
	// OnSelect callback lands a SetPage message — no re-subscription SwitchMap
	// and no captured-at-construction static ints (the FEEDBACK-G5.2 friction).
	paginationWidgetObs := rx.SwitchMap(
		rx.CombineLatest2(currentPageObs, pageCountObs),
		func(t rx.Tuple2[int, int]) rx.Observable[layout.Widget] {
			return pagination.Pagination(th, pagination.Props{
				Page:      t.First,
				PageCount: t.Second,
				OnSelect:  func(gtx layout.Context, p int) { mvu.MessageOp{Message: SetPage{Page: p}}.Add(gtx.Ops) },
			})
		},
	)

	return rx.Map(
		rx.CombineLatest4(filterWidgetObs, tableWidgetObs, paginationWidgetObs, unreadTipObs),
		func(t rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			return articlesLayout(t.First, t.Second, t.Third, t.Fourth)
		},
	)
}

// themedTextCell renders a single line of Text-coloured cell text in the
// theme's BodyMedium role — typeface, weight, size and line height from the
// Typography, the shaper the theme's own — within the cell's allocated
// rectangle, with the table's stock 12 dp horizontal padding. It is the
// theme-driven successor to the static table.RenderTextCell form
// (F1.2); the golden tests keep the static form, which is its documented
// remit.
func themedTextCell(tok themeTokens, s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		padH := gtx.Dp(unit.Dp(cellPadDp))
		labelMaxW := size.X - 2*padH
		if labelMaxW <= 0 {
			return layout.Dimensions{Size: size}
		}
		labelGtx := gtx
		labelGtx.Constraints.Min = image.Point{}
		labelGtx.Constraints.Max.X = labelMaxW
		labelGtx.Constraints.Max.Y = size.Y

		mLabel := op.Record(gtx.Ops)
		labelDims := drawLabel(labelGtx, tok.shaper, s, tok.typ.BodyMedium, tok.col.Text)
		labelCall := mLabel.Stop()

		offY := (size.Y - labelDims.Size.Y) / 2
		if offY < 0 {
			offY = 0
		}
		st := op.Offset(image.Pt(padH, offY)).Push(gtx.Ops)
		labelCall.Add(gtx.Ops)
		st.Pop()
		return layout.Dimensions{Size: size}
	}
}

// articleColumns builds the four table columns. Title is sortable and hosts
// the per-row click registration (patterns/table has no whole-row click
// affordance; see FEEDBACK-G5.2.md). A row click lands a SelectArticle
// message. Published is sortable. Author and Unread are static.
func articleColumns(
	loadTok func() themeTokens,
	rowClicks *keyed.Deferred[ArticleID, *widget.Clickable],
) []table.Column[article] {
	cellText := func(get func(a article) string) func(article) layout.Widget {
		return func(a article) layout.Widget {
			s := get(a)
			return func(gtx layout.Context) layout.Dimensions {
				return themedTextCell(loadTok(), s)(gtx)
			}
		}
	}
	titleCell := func(a article) layout.Widget {
		click := rowClicks.For(a.ID)
		return func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				mvu.MessageOp{Message: SelectArticle{Article: a.ID}}.Add(gtx.Ops)
			}
			body := themedTextCell(loadTok(), a.Title)
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(a.Title).Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				return body(gtx)
			})
		}
	}
	return []table.Column[article]{
		{Header: "Title", Sortable: true, Cell: titleCell},
		{Header: "Author", Width: unit.Dp(160), Cell: cellText(func(a article) string { return a.Author })},
		{Header: "Published", Width: unit.Dp(140), Sortable: true, Cell: cellText(func(a article) string {
			return a.Published.Format("Jan 2 2006")
		})},
		// Icon-only header: the bullet mirrors the cell glyph; the
		// column's meaning is carried by the hover tooltip ("Unread")
		// overlaid in articlesLayout.
		{Header: "•", Width: unit.Dp(unreadColWDp), Cell: cellText(func(a article) string {
			if a.Unread {
				return "•"
			}
			return ""
		})},
	}
}

// articlesLayout vertically stacks the three composed widgets with a
// uniform inset and a small gap between rows. The table flexes to
// consume vertical space the filter and pagination rows leave behind.
// unreadTip is overlaid on the table's Unread header cell — see
// overlayUnreadTooltip.
func articlesLayout(filter, table, pag, unreadTip layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(filter),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, overlayUnreadTooltip(table, unreadTip)),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(pag),
			)
		})
	}
}

// overlayUnreadTooltip draws the table, then lays the tooltip's
// trigger-sized canvas exactly over the Unread header cell (the trailing
// pinned-width column, header row height). The tooltip registers its hover
// hit area inside that canvas and paints its surface below it, over the
// table body. Positioning is arithmetic over unreadColWDp/tableHeaderHDp
// because patterns/table exposes no per-header widget slot.
func overlayUnreadTooltip(table, tip layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dims := table(gtx)
		w := gtx.Dp(unit.Dp(unreadColWDp))
		h := gtx.Dp(unit.Dp(tableHeaderHDp))
		if w > dims.Size.X {
			w = dims.Size.X
		}
		if h > dims.Size.Y {
			h = dims.Size.Y
		}
		st := op.Offset(image.Pt(dims.Size.X-w, 0)).Push(gtx.Ops)
		tipGtx := gtx
		tipGtx.Constraints = layout.Exact(image.Pt(w, h))
		tip(tipGtx)
		st.Pop()
		return dims
	}
}
