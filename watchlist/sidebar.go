// sidebar.go draws the watchlists sidebar: one clickable row per watchlist
// name loaded from disk, with the selected row tinted, and an empty-state
// message when no watchlists are present.
//
// Rows are drawn MANUALLY (rather than via cadence/sidebar) because the list
// is data-driven: cadence/sidebar.Props.Items is a static slice fixed at
// construction, so a dynamic, on-disk-loaded name list cannot drive it — the
// same reason feeds/sidebar.go hand-draws its rows. Per-name widget.Clickable
// state is keyed by name via prism/keyed so a future add/rename never re-binds
// a clickable to the wrong row. Row clicks land SelectWatchlist messages.
// (cadence/sidebar friction logged in FEEDBACK-G5.3.md.)
//
// G5.3c adds a RIGHT-CLICK context menu (Rename / Delete) per row. This is the
// codebase's first right-click composition (logged in FEEDBACK-G5.3.md): a raw
// pointer.Press hit area is registered IN FRONT of the row's select clickable,
// inside a pointer.PassOp so the press ALSO reaches the clickable behind it; the
// drain ignores everything but ButtonSecondary. Without the PassOp the
// front-most area swallows the PRIMARY press and click-to-select breaks (proven
// by TestRightClickPassesPrimaryReachesContextSecondary). cadence/popover
// anchors to the row and centres in its canvas — it cannot open at the cursor
// position; the menu opens centred on the row (logged).

package main

import (
	"image"
	"sync/atomic"

	"gioui.org/io/event"
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

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/keyed"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

const (
	wlSidebarWidthDp = 192
	wlRowHDp         = 32
	wlRowPadXDp      = 12
	// wlSidebarTopDp offsets the sidebar content down by the navbar height so
	// the first watchlist row aligns with the top of the Main content area
	// (the navbar spans only the Main column, not the full-height sidebar).
	wlSidebarTopDp = 64
)

// watchlistSidebar returns the sidebar observable. watchlistsObs streams the
// ordered watchlist list from the model; selectedObs streams the selected
// name. The widget reads both live each frame (via atomic cells updated on
// every emission), so a SelectWatchlist re-emits this stream — driving the
// shell re-emission and same-frame repaint.
func watchlistSidebar(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	watchlistsObs rx.Observable[[]Watchlist],
	selectedObs rx.Observable[string],
	storePath string,
	modelMirrorObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	var tokenCell atomic.Value
	tokenCell.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	_ = rx.CombineLatest2(colObs, typObs).Subscribe(rx.GoroutineContext(), func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			tokenCell.Store(tokenState{col: t.First, typ: t.Second})
		}
	})

	// Live model cells, read at frame time.
	var listCell atomic.Value
	listCell.Store([]Watchlist(nil))
	var selCell atomic.Value
	selCell.Store("")

	// ONE eager model mirror (subscribed here, NOT inside a keyed.Defer — a lazy
	// per-row subscription joins the hot stream after the seed and writes an
	// empty Document on a pre-interaction delete; invariant logged in
	// FEEDBACK-G5.3.md). The per-name context menus read it via loadModel.
	var modelCell atomic.Value
	modelCell.Store(Model{editIndex: -1})
	_ = modelMirrorObs.Subscribe(rx.GoroutineContext(), func(m Model, _ error, done bool) {
		if !done {
			modelCell.Store(m)
		}
	})
	loadModel := func() Model { m, _ := modelCell.Load().(Model); return m }

	// Per-name row clickables, stable across list mutation.
	rowClicks := keyed.Defer(func(string) *widget.Clickable { return &widget.Clickable{} })
	// Per-name right-click context menu (Rename / Delete) — ephemeral per-row
	// interaction state (feeds idiom). Its inner clickables are owned by the
	// sidebarContext. The secondary-press hit area uses a per-name tag.
	renameClicks := keyed.Defer(func(string) *widget.Clickable { return &widget.Clickable{} })
	deleteClicks := keyed.Defer(func(string) *widget.Clickable { return &widget.Clickable{} })
	confirmClicks := keyed.Defer(func(string) *widget.Clickable { return &widget.Clickable{} })
	ctxTags := keyed.Defer(func(string) *ctxPressTag { return &ctxPressTag{} })
	contexts := keyed.Defer(func(name string) *sidebarContext {
		return newSidebarContext(th, shaper, name, storePath,
			renameClicks.For(name), deleteClicks.For(name), confirmClicks.For(name), loadModel)
	})

	sidebarW := func(gtx layout.Context) layout.Dimensions {
		s := tokenCell.Load().(tokenState)
		lists, _ := listCell.Load().([]Watchlist)
		selected, _ := selCell.Load().(string)

		w := gtx.Dp(unit.Dp(wlSidebarWidthDp))
		h := gtx.Constraints.Max.Y
		size := image.Pt(w, h)
		paint.FillShape(gtx.Ops, s.col.Surface, clip.Rect{Max: size}.Op())

		top := gtx.Dp(unit.Dp(wlSidebarTopDp))
		padX := gtx.Dp(unit.Dp(wlRowPadXDp))

		// Empty state: no watchlists loaded (absent or empty file).
		if len(lists) == 0 {
			drawLabelAt(gtx, shaper, "No watchlists yet",
				unit.Sp(s.typ.BodySmall), s.col.OnSurfaceVariant,
				image.Pt(padX, top))
			return layout.Dimensions{Size: size}
		}

		rowH := gtx.Dp(unit.Dp(wlRowHDp))

		// Process clicks first so a selection on this frame is observed.
		for _, wl := range lists {
			rc := rowClicks.For(wl.Name)
			if rc.Clicked(gtx) {
				mvu.MessageOp{Message: SelectWatchlist{Name: wl.Name}}.Add(gtx.Ops)
			}
			// Drain the per-row secondary-press tag: a right-click opens the
			// context menu. The tag is registered in front of the row (below)
			// inside a PassOp, so the PRIMARY press still reaches the select
			// clickable; only ButtonSecondary opens the menu.
			tag := ctxTags.For(wl.Name)
			for {
				e, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
				if !ok {
					break
				}
				if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press &&
					pe.Buttons.Contain(pointer.ButtonSecondary) {
					contexts.For(wl.Name).openMenu()
				}
			}
		}

		for i, wl := range lists {
			off := op.Offset(image.Pt(0, top+i*rowH)).Push(gtx.Ops)
			rowGtx := gtx
			rowGtx.Constraints = layout.Exact(image.Pt(w, rowH))
			drawWatchlistRow(rowGtx, shaper, wl.Name, wl.Name == selected,
				rowClicks.For(wl.Name), ctxTags.For(wl.Name), contexts.For(wl.Name),
				padX, s.col, s.typ)
			off.Pop()
		}
		return layout.Dimensions{Size: size}
	}

	colorsObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	// Fold the list, selection, and a theme token together so the widget
	// re-emits on any model OR theme change; store the live values into the
	// cells the widget reads.
	return rx.Map(
		rx.CombineLatest3(watchlistsObs, selectedObs, colorsObs),
		func(n rx.Tuple3[[]Watchlist, string, tokens.ColorTokens]) layout.Widget {
			listCell.Store(n.First)
			selCell.Store(n.Second)
			return sidebarW
		},
	)
}

// ctxPressTag is a non-zero-size type so its address is a unique per-row event
// tag for the right-click (secondary-press) hit area.
type ctxPressTag struct{ _ byte }

// drawWatchlistRow paints one watchlist row: the name label, with a Primary
// background tint when selected. The whole row is the SelectWatchlist click
// target; a right-click (secondary press) opens the context menu, registered in
// front of the select clickable inside a PassOp so the primary press still
// reaches it. The context popover is drawn over the row while open.
func drawWatchlistRow(
	gtx layout.Context,
	shaper *text.Shaper,
	name string,
	selected bool,
	click *widget.Clickable,
	ctxTag *ctxPressTag,
	ctx *sidebarContext,
	padX int,
	col tokens.ColorTokens,
	ts tokens.TypeScale,
) layout.Dimensions {
	size := gtx.Constraints.Max
	if selected {
		paint.FillShape(gtx.Ops, col.Primary, clip.Rect{Max: size}.Op())
	}
	fg := col.OnSurface
	if selected {
		fg = col.OnPrimary
	}
	gtx.Constraints = layout.Exact(size)
	click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(name).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		// Vertically centre the label within the row.
		off := op.Offset(image.Pt(padX, (size.Y-gtx.Sp(unit.Sp(ts.BodyMedium)))/2)).Push(gtx.Ops)
		drawLabel(gtx, shaper, name, unit.Sp(ts.BodyMedium), fg)
		off.Pop()
		return layout.Dimensions{Size: size}
	})

	// Right-click hit area, registered IN FRONT of the select clickable inside a
	// PassOp so the press reaches BOTH (the clickable for primary, this tag for
	// secondary). The drain in sidebarW filters to ButtonSecondary.
	pass := pointer.PassOp{}.Push(gtx.Ops)
	cclip := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, ctxTag)
	cclip.Pop()
	pass.Pop()

	// The context popover (anchor + menu surface while open) draws over the row.
	if ctx != nil {
		ctx.layout(gtx)
	}
	return layout.Dimensions{Size: size}
}
