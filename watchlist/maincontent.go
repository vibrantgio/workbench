// maincontent.go draws the Main content area: above an "Add symbol" button,
// a cadence/table of the selected watchlist's symbols (a leading checkbox
// column for bulk-select, then Symbol, Exchange, Timeframe, Notes, then a
// trailing trash column for row delete). A row click in the Symbol column
// reopens the add/edit modal; the trash icon opens a per-row cadence/popover
// delete confirm; cadence/tooltip header overlays explain each column; a
// cadence/pagination row appears below the table only when the watchlist has
// more than pageSize symbols. When no watchlist is selected it prompts to pick.
//
// G5.3c additions over G5.3b:
//   - bulk-select checkbox column (ToggleSelect{absolute row}),
//   - per-row trash → delete-confirm popover (DeleteSymbol{absolute row}),
//   - per-header tooltip overlays (column-width arithmetic, header 36dp),
//   - conditional pagination (rendered only when pageCount > 1).
//
// Selection + paging come from the model (selectionObs, pageObs); the visible
// rows are the page slice, but the checkbox/trash messages carry ABSOLUTE
// indices (pageOffset + page-relative row) so they survive pagination.

package main

import (
	"image"
	"sync/atomic"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/pagination"
	"github.com/vibrantgio/cadence/table"
	"github.com/vibrantgio/cadence/tooltip"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/keyed"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

const wlMainPadDp = 24

// cellPadDp is the horizontal cell padding themedTextCell applies — 12 dp,
// mirroring cadence/table's own cell padding so app-built cells sit flush
// with the table's stock geometry (the feeds F1.2 idiom).
const cellPadDp = 12

// Column geometry. Symbol flexes; the rest are pinned. The checkbox + trash
// gutters are fixed leading/trailing columns. tableHeaderHDp mirrors
// cadence/table's private header height — Density.ControlHeight at
// Comfortable (36 dp, the E1.4 row rule) — because the table draws headers
// internally with no per-header widget slot, so the tooltip overlays are
// positioned by arithmetic over these widths (friction logged in
// FEEDBACK-G5.3.md).
const (
	selColWDp      = 48
	exchColWDp     = 140
	tfColWDp       = 120
	notesColWDp    = 220
	trashColWDp    = 48
	tableHeaderHDp = 36
)

// symbolRow pairs a symbol with its ABSOLUTE 0-based index in the watchlist so
// row interactions (edit / delete / select) land messages keyed on the stable
// index even when the table shows only a page slice.
type symbolRow struct {
	idx      int
	sym      Symbol
	selected bool
}

// watchlistMain returns the Main-pane observable. selectedObs streams the
// selected watchlist name; symbolsObs the full symbol slice; selectionObs the
// bulk-select set; pageObs the current page. A model mutation re-emits this
// stream and the folded shell repaints.
func watchlistMain(
	th rx.Observable[theme.Theme],
	selectedObs rx.Observable[string],
	symbolsObs rx.Observable[[]Symbol],
	selectionObs rx.Observable[map[int]bool],
	pageObs rx.Observable[int],
	storePath string,
	modelMirrorObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	loadTok := mirrorTokens(th)

	var selCell atomic.Value
	selCell.Store("")

	// ONE eager model mirror for the whole pane, subscribed in the function body
	// (NOT inside a keyed.Defer). A per-row mirror would subscribe LAZILY during
	// the first layout frame — after the seed emission already
	// fired — so it would never be seeded and would write an empty Document on a
	// pre-interaction delete (data loss). Sharing one eager mirror keeps the
	// modelObs consumer count STATIC (independent of row count) and seed-correct.
	// (Invariant logged in FEEDBACK-G5.3.md.)
	var modelCell atomic.Value
	modelCell.Store(Model{editIndex: -1})
	_ = modelMirrorObs.Subscribe(rx.GoroutineContext(), func(m Model, _ error, done bool) {
		if !done {
			modelCell.Store(m)
		}
	})
	loadModel := func() Model { m, _ := modelCell.Load().(Model); return m }

	// Per-index widget state, stable across list mutation.
	rowClicks := keyed.Defer(func(int) *widget.Clickable { return &widget.Clickable{} })
	checkClicks := keyed.Defer(func(int) *widget.Clickable { return &widget.Clickable{} })
	// Per-row delete-confirm popovers: ephemeral interaction state (per-row
	// rx.Subject open flag), NOT model state — the feeds idiom. Keyed by the
	// absolute row index. They read the model via loadModel (the shared eager
	// mirror), never their own modelObs subscription. (Choice logged in
	// FEEDBACK-G5.3.md.)
	trashClicks := keyed.Defer(func(int) *widget.Clickable { return &widget.Clickable{} })
	confirmClicks := keyed.Defer(func(int) *widget.Clickable { return &widget.Clickable{} })
	rowPopovers := keyed.Defer(func(idx int) *rowDeleteConfirm {
		return newRowDeleteConfirm(th, idx, storePath,
			trashClicks.For(idx), confirmClicks.For(idx), loadModel)
	})

	columns := symbolColumns(loadTok, rowClicks, checkClicks, rowPopovers)

	// rowsObs maps (symbols, selection, page) into the PAGE slice as indexed
	// rows. The idx field is the ABSOLUTE index so messages survive pagination.
	rowsObs := rx.Map(
		rx.CombineLatest3(symbolsObs, selectionObs, pageObs),
		func(t rx.Tuple3[[]Symbol, map[int]bool, int]) []symbolRow {
			syms, sel, page := t.First, t.Second, t.Third
			if page < 1 {
				page = 1
			}
			start := (page - 1) * pageSize
			if start > len(syms) {
				start = len(syms)
			}
			end := start + pageSize
			if end > len(syms) {
				end = len(syms)
			}
			rows := make([]symbolRow, 0, end-start)
			for i := start; i < end; i++ {
				rows = append(rows, symbolRow{idx: i, sym: syms[i], selected: sel[i]})
			}
			return rows
		},
	)

	tableObs := table.Table(th, table.Props[symbolRow]{
		Columns: columns,
		Items:   rowsObs,
	})

	// pageCountObs derives the page count from the symbol slice. When it is 1
	// the pagination slot renders nothing (the ">25 symbols" conditional).
	pageCountObs := rx.Map(symbolsObs, func(syms []Symbol) int {
		n := (len(syms) + pageSize - 1) / pageSize
		if n < 1 {
			return 1
		}
		return n
	})

	// pagination.Props.Page/PageCount are static ints; rebuild the row via
	// SwitchMap on (page, pageCount) so the active highlight tracks model state
	// (the FEEDBACK-G5.2b workaround). A nil widget for pageCount==1.
	paginationObs := rx.SwitchMap(
		rx.CombineLatest2(pageObs, pageCountObs),
		func(t rx.Tuple2[int, int]) rx.Observable[layout.Widget] {
			page, count := t.First, t.Second
			if count <= 1 {
				return rx.Of[layout.Widget](nil)
			}
			return pagination.Pagination(th, pagination.Props{
				Page:      page,
				PageCount: count,
				OnSelect:  func(gtx layout.Context, p int) { mvu.MessageOp{Message: SetPage{Page: p}}.Add(gtx.Ops) },
			})
		},
	)

	// One tooltip per column header, overlaid by column-width arithmetic.
	colTips := columnTooltips(th)

	// "Add symbol" button (a plain clickable, no extra layer-boundary cell).
	var addClick widget.Clickable
	addButton := func(gtx layout.Context) layout.Dimensions {
		if addClick.Clicked(gtx) {
			mvu.MessageOp{Message: OpenAddSymbol{}}.Add(gtx.Ops)
		}
		s := loadTok()
		return addClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Add symbol").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, s.shaper, "+ Add symbol", s.typ.LabelLarge, s.col.Primary)
		})
	}

	// mainW composes the header, Add button, table (with header tooltip
	// overlay), and the conditional pagination row.
	mainW := func(tableW, pagW layout.Widget, tips []layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			s := loadTok()
			selected, _ := selCell.Load().(string)
			size := gtx.Constraints.Max
			paint.FillShape(gtx.Ops, s.col.Background, clip.Rect{Max: size}.Op())

			if selected == "" {
				pad := gtx.Dp(unit.Dp(wlMainPadDp))
				drawLabelAt(gtx, s.shaper, "Select a watchlist",
					s.typ.BodyLarge, s.col.Ramps.Neutral.Step(700), image.Pt(pad, pad))
				return layout.Dimensions{Size: size}
			}

			return layout.UniformInset(unit.Dp(wlMainPadDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawLabel(gtx, s.shaper, selected, s.typ.HeadlineSmall, s.col.Text)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(addButton),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Flexed(1, overlayHeaderTooltips(tableW, tips)),
				}
				if pagW != nil {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(pagW),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}
	}

	// rx.CombineLatest is the variadic form yielding []T — collapses the four
	// per-column tooltip streams into one slice (rx tops out at CombineLatest5,
	// logged in FEEDBACK-G5.3.md).
	tipsObs := rx.CombineLatest(colTips...)

	return rx.Map(
		rx.CombineLatest4(selectedObs, tableObs, paginationObs, tipsObs),
		func(n rx.Tuple4[string, layout.Widget, layout.Widget, []layout.Widget]) layout.Widget {
			selCell.Store(n.First)
			return mainW(n.Second, n.Third, n.Fourth)
		},
	)
}

// themedTextCell renders a single line of Text-coloured cell text in the
// theme's BodyMedium role — typeface, weight, size and line height from the
// Typography, the shaper the theme's own — within the cell's allocated
// rectangle, with the table's stock 12 dp horizontal padding. It is the
// theme-driven successor to the frozen table.RenderTextCell(TypeScale) form
// (F1.2/F1.3); the golden tests keep the static form, which is its documented
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

// symbolColumns builds the six table columns: a leading checkbox, the four text
// columns (Symbol hosts the row-click edit), and a trailing trash gutter.
func symbolColumns(
	loadTok func() themeTokens,
	rowClicks *keyed.Deferred[int, *widget.Clickable],
	checkClicks *keyed.Deferred[int, *widget.Clickable],
	rowPopovers *keyed.Deferred[int, *rowDeleteConfirm],
) []table.Column[symbolRow] {
	cellText := func(get func(r symbolRow) string) func(symbolRow) layout.Widget {
		return func(r symbolRow) layout.Widget {
			s := get(r)
			return func(gtx layout.Context) layout.Dimensions {
				return themedTextCell(loadTok(), s)(gtx)
			}
		}
	}
	checkboxCell := func(r symbolRow) layout.Widget {
		click := checkClicks.For(r.idx)
		return func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				mvu.MessageOp{Message: ToggleSelect{Row: r.idx}}.Add(gtx.Ops)
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp("Select row").Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				drawCheckbox(gtx, r.selected, loadTok().col)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}
	}
	symbolCell := func(r symbolRow) layout.Widget {
		click := rowClicks.For(r.idx)
		return func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				mvu.MessageOp{Message: OpenEditSymbol{Row: r.idx}}.Add(gtx.Ops)
			}
			body := themedTextCell(loadTok(), r.sym.Symbol)
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(r.sym.Symbol).Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				return body(gtx)
			})
		}
	}
	trashCell := func(r symbolRow) layout.Widget {
		dc := rowPopovers.For(r.idx)
		return func(gtx layout.Context) layout.Dimensions {
			return dc.layout(gtx)
		}
	}
	return []table.Column[symbolRow]{
		{Header: "", Width: unit.Dp(selColWDp), Cell: checkboxCell},
		{Header: "Symbol", Cell: symbolCell},
		{Header: "Exchange", Width: unit.Dp(exchColWDp), Cell: cellText(func(r symbolRow) string { return r.sym.Exchange })},
		{Header: "Timeframe", Width: unit.Dp(tfColWDp), Cell: cellText(func(r symbolRow) string { return r.sym.Timeframe })},
		{Header: "Notes", Width: unit.Dp(notesColWDp), Cell: cellText(func(r symbolRow) string { return r.sym.Notes })},
		{Header: "", Width: unit.Dp(trashColWDp), Cell: trashCell},
	}
}

// drawCheckbox paints a small square, filled when selected. clip/paint only so
// it stays golden-deterministic. The unchecked outline is the strong-border
// ramp step (Neutral 500 — the Outline alias's resolution).
func drawCheckbox(gtx layout.Context, checked bool, col tokens.ColorTokens) {
	box := gtx.Constraints.Max
	side := gtx.Dp(unit.Dp(16))
	x0 := (box.X - side) / 2
	y0 := (box.Y - side) / 2
	r := image.Rect(x0, y0, x0+side, y0+side)
	if checked {
		paint.FillShape(gtx.Ops, col.Primary, clip.Rect(r).Op())
	} else {
		// Outline (four thin rects).
		border := col.Ramps.Neutral.Step(500)
		s := gtx.Dp(unit.Dp(1))
		if s < 1 {
			s = 1
		}
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+s)).Op())
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(r.Min.X, r.Max.Y-s, r.Max.X, r.Max.Y)).Op())
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(r.Min.X, r.Min.Y, r.Min.X+s, r.Max.Y)).Op())
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(r.Max.X-s, r.Min.Y, r.Max.X, r.Max.Y)).Op())
	}
}

// columnTooltips builds one tooltip per labelled header column (skipping the
// icon-only checkbox/trash gutters). Each Trigger fills its incoming canvas;
// overlayHeaderTooltips positions that canvas over the matching header cell.
func columnTooltips(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	mk := func(textStr string) rx.Observable[layout.Widget] {
		return tooltip.Tooltip(th, tooltip.Props{
			Text:      textStr,
			Placement: tooltip.Bottom,
			Trigger: func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			},
		})
	}
	return []rx.Observable[layout.Widget]{
		mk("Instrument symbol, e.g. BTC/USD"),
		mk("Exchange the symbol trades on"),
		mk("Chart timeframe, e.g. 1h"),
		mk("Free-form notes"),
	}
}

// overlayHeaderTooltips draws the table, then lays each tooltip's
// trigger-sized canvas exactly over its header cell. The x offsets accumulate
// in column order: checkbox gutter, then the flexing Symbol column, then the
// pinned Exchange/Timeframe/Notes columns. Header height is tableHeaderHDp.
// Positioning is arithmetic because cadence/table exposes no per-header slot.
func overlayHeaderTooltips(tbl layout.Widget, tips []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dims := tbl(gtx)
		h := gtx.Dp(unit.Dp(tableHeaderHDp))
		if h > dims.Size.Y {
			h = dims.Size.Y
		}
		selW := gtx.Dp(unit.Dp(selColWDp))
		exchW := gtx.Dp(unit.Dp(exchColWDp))
		tfW := gtx.Dp(unit.Dp(tfColWDp))
		notesW := gtx.Dp(unit.Dp(notesColWDp))
		trashW := gtx.Dp(unit.Dp(trashColWDp))
		// Symbol flexes to fill the remaining width.
		symW := dims.Size.X - selW - exchW - tfW - notesW - trashW
		if symW < 0 {
			symW = 0
		}
		// (column-header text, x-offset, width) in render order.
		type span struct {
			x, w int
		}
		spans := []span{
			{selW, symW},                        // Symbol header
			{selW + symW, exchW},                // Exchange header
			{selW + symW + exchW, tfW},          // Timeframe header
			{selW + symW + exchW + tfW, notesW}, // Notes header
		}
		for i, sp := range spans {
			if i >= len(tips) || tips[i] == nil {
				continue
			}
			w := sp.w
			if sp.x+w > dims.Size.X {
				w = dims.Size.X - sp.x
			}
			if w <= 0 {
				continue
			}
			st := op.Offset(image.Pt(sp.x, 0)).Push(gtx.Ops)
			tgtx := gtx
			tgtx.Constraints = layout.Exact(image.Pt(w, h))
			tips[i](tgtx)
			st.Pop()
		}
		return dims
	}
}
