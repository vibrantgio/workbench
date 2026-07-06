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
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/pagination"
	"github.com/vibrantgio/cadence/table"
	"github.com/vibrantgio/cadence/tooltip"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/keyed"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// articlesPageSize is the fixed row count per pagination page.
const articlesPageSize = 10

// Sortable column indices for the articles table.
const (
	colTitle     = 0
	colAuthor    = 1
	colPublished = 2
	colUnread    = 3
)

// Geometry shared by the Unread tooltip overlay. unreadColWDp matches the
// Unread column's pinned Width; tableHeaderHDp mirrors cadence/table's
// private headerHDp — the table draws its header internally and exposes no
// per-header widget hook, so the tooltip hit area is positioned by
// arithmetic over these constants (friction logged in FEEDBACK-G5.2.md).
const (
	unreadColWDp   = 96
	tableHeaderHDp = 44
)

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
// (currentPageObs), and sort (sortObs) are derived from the MVU model; every
// interactive callback lands an mvu.MessageOp so a click re-emits this layer
// — and the shell — on the same frame. The filter text is local UI state
// (it never needed to be model-derived) and is held in a small rx.Subject.
func articlesMain(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	selectedFeedObs rx.Observable[FeedID],
	currentPageObs rx.Observable[int],
	sortObs rx.Observable[table.Sort],
) rx.Observable[layout.Widget] {
	all := hardCodedArticles()

	// Token mirror so the table column Cell closures (which run outside any
	// rx.Defer scope) can read current colours/typography on each frame
	// without crossing scheduler boundaries. An atomic.Value, not the typed
	// atomic mirror the GX.10 cleanup removed — this mirror feeds cell
	// rendering and is not a substitute for a layer re-emission.
	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	var tokenCell atomic.Value
	tokenCell.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typeObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	_ = rx.CombineLatest2(colorObs, typeObs).Subscribe(func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			tokenCell.Store(tokenState{col: t.First, typ: t.Second})
		}
	}, rx.Goroutine)
	loadColor := func() tokens.ColorTokens { return tokenCell.Load().(tokenState).col }
	loadType := func() tokens.TypeScale { return tokenCell.Load().(tokenState).typ }

	// Sort mirror so onSort can read the current sort (from the model) to
	// decide whether to flip the Asc bit or start a fresh Asc on a different
	// column. An atomic.Value fed by the model-derived sortObs — the click
	// callback reads the latest model value, then emits a SetSort message that
	// re-emits this layer.
	var sortCell atomic.Value
	sortCell.Store(table.Sort{Column: colPublished, Asc: false})
	_ = sortObs.Subscribe(func(s table.Sort, _ error, done bool) {
		if !done {
			sortCell.Store(s)
		}
	}, rx.Goroutine)

	// Per-article widget.Clickable registry. The Deferred lives for the
	// program lifetime — feeds is a single-window app and articlesMain is
	// called once from feedsShellLayer, so no rx.Defer scope is required.
	rowClicks := keyed.Defer(func(_ ArticleID) *widget.Clickable {
		return &widget.Clickable{}
	})

	// Filter text is local UI state, not part of the persisted model. A small
	// Subject feeds the filter into the Items pipeline.
	filterSend, filterObs := rx.Subject[string](0, 1)
	filterSend.Next("")

	columns := articleColumns(shaper, loadColor, loadType, rowClicks)

	onSort := func(gtx layout.Context, col int) {
		cur, _ := sortCell.Load().(table.Sort)
		if cur.Column == col {
			mvu.MessageOp{Message: SetSort{Sort: table.Sort{Column: col, Asc: !cur.Asc}}}.Add(gtx.Ops)
			return
		}
		mvu.MessageOp{Message: SetSort{Sort: table.Sort{Column: col, Asc: true}}}.Add(gtx.Ops)
	}

	filtered := rx.Map(
		rx.CombineLatest3(selectedFeedObs, filterObs, sortObs),
		func(t rx.Tuple3[FeedID, string, table.Sort]) []article {
			return filterAndSortArticles(all, t.First, t.Second, t.Third)
		},
	)
	paged := rx.Map(
		rx.CombineLatest2(filtered, currentPageObs),
		func(t rx.Tuple2[[]article, int]) []article {
			return pageSlice(t.First, t.Second, articlesPageSize)
		},
	)
	pageCountObs := rx.Map(filtered, func(arts []article) int {
		return pageCountFor(arts, articlesPageSize)
	})

	// Hover tooltip for the icon-only Unread ("•") column header. The
	// trigger fills whatever canvas it is given; the overlay wrapper in
	// articlesLayout positions that canvas over the header cell, since the
	// table draws its headers internally and offers no widget slot there.
	unreadTipObs := tooltip.Tooltip(th, tooltip.Props{
		Text:      "Unread",
		Placement: tooltip.Bottom,
		Shaper:    shaper,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
	})

	filterWidgetObs := input.TextField(th, input.TextFieldProps{
		Placeholder: "Filter articles",
		Description: "Filter articles by title or author",
		OnChange: func(gtx layout.Context, s string) {
			filterSend.Next(s)
			// Narrowing the filter shrinks the result set, so reset to page 1
			// to avoid stranding the user on an out-of-range slice. SetPage is
			// idempotent when already on page 1.
			mvu.MessageOp{Message: SetPage{Page: 1}}.Add(gtx.Ops)
		},
		Shaper: shaper,
	})
	tableWidgetObs := table.Table(th, table.Props[article]{
		Columns: columns,
		Items:   paged,
		Sort:    sortObs,
		OnSort:  onSort,
		Shaper:  shaper,
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
				Shaper:    shaper,
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

// articleColumns builds the four table columns. Title is sortable and hosts
// the per-row click registration (cadence/table has no whole-row click
// affordance; see FEEDBACK-G5.2.md). A row click lands a SelectArticle
// message. Published is sortable. Author and Unread are static.
func articleColumns(
	shaper *text.Shaper,
	loadColor func() tokens.ColorTokens,
	loadType func() tokens.TypeScale,
	rowClicks *keyed.Deferred[ArticleID, *widget.Clickable],
) []table.Column[article] {
	cellText := func(get func(a article) string) func(article) layout.Widget {
		return func(a article) layout.Widget {
			s := get(a)
			return func(gtx layout.Context) layout.Dimensions {
				return table.RenderTextCell(shaper, loadColor(), loadType(), s)(gtx)
			}
		}
	}
	titleCell := func(a article) layout.Widget {
		click := rowClicks.For(a.ID)
		return func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				mvu.MessageOp{Message: SelectArticle{Article: a.ID}}.Add(gtx.Ops)
			}
			body := table.RenderTextCell(shaper, loadColor(), loadType(), a.Title)
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
// because cadence/table exposes no per-header widget slot.
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

// staticArticleColumns mirrors articleColumns for the golden-render path.
// Tokens are passed in directly; rows are not clickable.
func staticArticleColumns(shaper *text.Shaper, colors tokens.ColorTokens, ts tokens.TypeScale) []table.Column[article] {
	cellText := func(get func(a article) string) func(article) layout.Widget {
		return func(a article) layout.Widget {
			return table.RenderTextCell(shaper, colors, ts, get(a))
		}
	}
	return []table.Column[article]{
		{Header: "Title", Sortable: true, Cell: cellText(func(a article) string { return a.Title })},
		{Header: "Author", Width: unit.Dp(160), Cell: cellText(func(a article) string { return a.Author })},
		{Header: "Published", Width: unit.Dp(140), Sortable: true, Cell: cellText(func(a article) string {
			return a.Published.Format("Jan 2 2006")
		})},
		{Header: "•", Width: unit.Dp(unreadColWDp), Cell: cellText(func(a article) string {
			if a.Unread {
				return "•"
			}
			return ""
		})},
	}
}
