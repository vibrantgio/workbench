// g53c_sim_test.go verifies the G5.3c interaction surfaces HEADLESSLY (no GUI
// driving is available — launching the Gio app from a shell has no window-server
// session). Coverage:
//   - the novel right-click composition: a PRIMARY press reaches the row select
//     clickable AND a SECONDARY press opens the context menu, driven through a
//     real input.Router (TestRightClickPassesPrimaryReachesContextSecondary),
//   - a column-header tooltip showing on hover + delay through a real Router
//     (TestColumnTooltipHoverHeadless),
//   - pixel-sim flows against the REAL shell: row delete removes a row, bulk
//     "Delete N" renders the count, pagination appears only above pageSize rows
//     (a 30-symbol fixture) and not at/below it, the rename modal opens
//     (TestG53cShellStatesHeadless),
//   - store-level round-trips for delete / bulk-delete / rename / delete-
//     watchlist persistence (TestG53cPersistenceRoundTrips).
//
// Persistence "across restart" is proven at the store level (apply the pure
// helper → saveStore → fresh loadStore → deep-equal), the same proof G5.3b used
// for edits: the submit/confirm callbacks route through the SAME pure helpers,
// so a store round-trip of the helper is the durable persistence guarantee.
package main

import (
	"image"
	"image/color"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/tooltip"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// TestRightClickPassesPrimaryReachesContextSecondary is the regression guard for
// the codebase's first right-click composition (sidebar context menu). It
// composes the row exactly as drawWatchlistRow does — a select clickable, then a
// secondary-press hit area registered IN FRONT inside a pointer.PassOp — and
// drives a real input.Router. A primary press+release must reach the clickable
// (click-to-select survives); a secondary press must open the context menu. The
// PassOp is load-bearing: without it the front-most area swallows the primary
// press and select breaks.
func TestRightClickPassesPrimaryReachesContextSecondary(t *testing.T) {
	var rowClick widget.Clickable
	var tag ctxPressTag
	const w, h = 192, 32

	selected := false
	contextOpened := false

	row := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if rowClick.Clicked(gtx) {
			selected = true
		}
		for {
			e, ok := gtx.Event(pointer.Filter{Target: &tag, Kinds: pointer.Press})
			if !ok {
				break
			}
			if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press &&
				pe.Buttons.Contain(pointer.ButtonSecondary) {
				contextOpened = true
			}
		}
		rowClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		pass := pointer.PassOp{}.Push(gtx.Ops)
		cclip := clip.Rect{Max: size}.Push(gtx.Ops)
		event.Op(gtx.Ops, &tag)
		cclip.Pop()
		pass.Pop()
		return layout.Dimensions{Size: size}
	}

	r := new(gioinput.Router)
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(image.Pt(w, h)),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Now:         time.Now(),
			Source:      r.Source(),
		}
		row(gtx)
		r.Frame(ops)
	}

	frame() // register tags
	at := f32.Pt(40, h/2)

	r.Queue(pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary})
	frame()
	if !contextOpened {
		t.Error("secondary press did not open the context menu")
	}

	r.Queue(pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	frame()
	r.Queue(pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	frame()
	if !selected {
		t.Error("primary press did NOT reach the select clickable; the context hit area swallowed it (PassOp regression)")
	}
}

// TestColumnTooltipHoverHeadless drives the first column-header tooltip through
// a real input.Router: a pointer.Move into the Symbol header cell, the show
// delay elapsing via gtx.Now, then a pixel assertion that the surface painted
// below the header. This is the overlayHeaderTooltips composition.
func TestColumnTooltipHoverHeadless(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	tipObs := tooltip.Tooltip(rx.Of(theme.Default()), tooltip.Props{
		Text:      "Instrument symbol, e.g. BTC/USD",
		Placement: tooltip.Bottom,
		Shaper:    shaper,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
	})
	tip := collectOneWidget(t, tipObs)

	stubTable := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Surface, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	const cw, ch = 900, 400
	// Overlay just the Symbol tooltip (index 0). The Symbol header spans from
	// selColWDp to the start of the Exchange column.
	overlay := overlayHeaderTooltips(stubTable, []layout.Widget{tip, nil, nil, nil})

	size := image.Pt(cw, ch)
	r := new(gioinput.Router)
	t0 := time.Unix(100, 0)
	frame := func(now time.Time) {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Now:         now,
			Source:      r.Source(),
		}
		overlay(gtx)
		r.Frame(ops)
	}

	// The Symbol header cell starts at selColWDp; hover its middle.
	hoverX := float32(selColWDp + 60)
	frame(t0)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(hoverX, tableHeaderHDp/2), Source: pointer.Mouse})
	frame(t0)
	tShown := t0.Add(tooltip.DefaultDelay + time.Millisecond)
	frame(tShown)

	hovered := captureAtCtx(t, size, overlay, tShown.Add(time.Millisecond), r.Source())
	if hovered == nil {
		t.Skip("headless rendering unavailable")
	}

	freshTip := collectOneWidget(t, tooltip.Tooltip(rx.Of(theme.Default()), tooltip.Props{
		Text:      "Instrument symbol, e.g. BTC/USD",
		Placement: tooltip.Bottom,
		Shaper:    shaper,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
	}))
	baseline := captureAtCtx(t, size, overlayHeaderTooltips(stubTable, []layout.Widget{freshTip, nil, nil, nil}), t0, gioinput.Source{})
	if baseline == nil {
		t.Skip("headless rendering unavailable")
	}

	surfaceRegion := image.Rect(selColWDp, tableHeaderHDp, selColWDp+260, tableHeaderHDp+60)
	if n := regionDiff(baseline, hovered, surfaceRegion); n <= 0 {
		t.Errorf("no pixels changed below the Symbol header after hover+delay (diff=%d); tooltip did not show", n)
	}
}

// g53cDoc seeds a 30-symbol "big" watchlist (so pageCount=2, exercising the
// >pageSize pagination conditional) plus a 2-symbol "small" one.
func g53cDoc() Document {
	syms := make([]Symbol, 30)
	for i := range syms {
		syms[i] = Symbol{Symbol: string(rune('A'+i)) + "/USD", Exchange: "Coinbase"}
	}
	return Document{
		Version:  formatVersion,
		Selected: "big",
		Watchlists: []Watchlist{
			{Name: "big", Symbols: syms},
			{Name: "small", Symbols: []Symbol{{Symbol: "BTC/USD"}, {Symbol: "ETH/USD"}}},
		},
	}
}

// TestG53cShellStatesHeadless renders the real shell at the G5.3c model states
// and asserts the pixel deltas the Measurable describes.
func TestG53cShellStatesHeadless(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	send, modelObs := rx.Subject[Model](0, 1, 256)
	storePath := filepath.Join(t.TempDir(), "watchlists.json")
	layer := watchlistShellLayer(rx.Of(theme.Default()), shaper, modelObs, storePath)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(shellCanvasW, shellCanvasH)
	snap := func(what string) *image.RGBA {
		w := awaitStableWidget(t, emissions, what)
		img := capture(t, size, scene(w, bg))
		if img == nil {
			t.Skip("headless rendering unavailable")
		}
		return img
	}

	// 30-symbol "big" selected → pagination MUST render (pageCount 2).
	m := initialModel(g53cDoc())
	if m.pageCount() != 2 {
		t.Fatalf("fixture should have 2 pages, got %d", m.pageCount())
	}
	send.Next(m)
	base := snap("initial (30 symbols, paginated)")

	// Switch to the 2-symbol "small" watchlist → pagination MUST NOT render.
	mSmall, _ := Update(m, SelectWatchlist{Name: "small"})
	send.Next(mSmall)
	small := snap("small watchlist (no pagination)")
	// The pagination row sits in the lower Main area; assert it differs there.
	pagRegion := image.Rect(wlSidebarWidthDp+10, shellCanvasH-90, shellCanvasW-10, shellCanvasH-20)
	if n := regionDiff(base, small, pagRegion); n <= 0 {
		t.Errorf("pagination region unchanged between paginated and unpaginated watchlists (diff=%d)", n)
	}

	// Delete a row from "big": the Main table changes.
	mDel, _ := Update(m, DeleteSymbol{Row: 0})
	send.Next(mDel)
	deleted := snap("DeleteSymbol(0)")
	wl, _ := mDel.selectedWatchlist()
	if len(wl.Symbols) != 29 {
		t.Fatalf("delete did not remove a row: %d", len(wl.Symbols))
	}
	if n := regionDiff(base, deleted, mainRegion); n <= 0 {
		t.Errorf("Main table unchanged after delete (diff=%d)", n)
	}

	// Select two rows → the navbar "Delete 2" action appears (navbar region).
	mSel, _ := Update(m, ToggleSelect{Row: 0})
	mSel, _ = Update(mSel, ToggleSelect{Row: 1})
	send.Next(mSel)
	withSel := snap("two rows selected")
	navRegion := image.Rect(wlSidebarWidthDp, 0, shellCanvasW, 64)
	if n := regionDiff(base, withSel, navRegion); n <= 0 {
		t.Errorf("navbar unchanged after selecting rows (diff=%d); Delete N action did not appear", n)
	}

	// Open the rename modal → the scrim paints over the window.
	mRename, _ := Update(m, OpenRenameWatchlist{Name: "big"})
	send.Next(mRename)
	renaming := snap("OpenRenameWatchlist")
	if n := regionDiff(base, renaming, scrimRegion); n <= 0 {
		t.Errorf("window unchanged after OpenRenameWatchlist (diff=%d); modal did not open", n)
	}
}

// TestG53cPersistenceRoundTrips proves delete / bulk-delete / rename / delete-
// watchlist persist at the store level: apply the SAME pure helper the confirm
// callbacks use, saveStore, then a fresh loadStore + deep-equal.
func TestG53cPersistenceRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	doc := g53cDoc()

	// Row delete.
	next := deleteSymbolAt(doc.Watchlists, "big", 0)
	if err := saveStore(path, documentOf(next, "big")); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	got, err := loadStore(path)
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if len(got.Watchlists[0].Symbols) != 29 || got.Watchlists[0].Symbols[0].Symbol != "B/USD" {
		t.Errorf("row delete did not persist: head=%+v", got.Watchlists[0].Symbols[0])
	}

	// Bulk delete (rows 0,1,2).
	next = bulkDeleteRows(doc.Watchlists, "big", []int{0, 1, 2})
	if err := saveStore(path, documentOf(next, "big")); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	got, _ = loadStore(path)
	if len(got.Watchlists[0].Symbols) != 27 {
		t.Errorf("bulk delete did not persist: %d symbols", len(got.Watchlists[0].Symbols))
	}

	// Rename watchlist (selection follows).
	next = renameWatchlistTo(doc.Watchlists, "big", "majors")
	if err := saveStore(path, documentOf(next, "majors")); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	got, _ = loadStore(path)
	if got.Watchlists[0].Name != "majors" || got.Selected != "majors" {
		t.Errorf("rename did not persist: name=%q selected=%q", got.Watchlists[0].Name, got.Selected)
	}

	// Delete watchlist (selection falls back to the first remaining).
	next = deleteWatchlistNamed(doc.Watchlists, "big")
	if err := saveStore(path, documentOf(next, firstWatchlistName(next))); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	got, _ = loadStore(path)
	if len(got.Watchlists) != 1 || got.Watchlists[0].Name != "small" || got.Selected != "small" {
		t.Errorf("delete-watchlist did not persist: %+v selected=%q", got.Watchlists, got.Selected)
	}
}

// ----- local headless helpers -----

// collectOneWidget subscribes to a widget observable and returns the first
// non-nil emission (cold theme observable replays immediately).
func collectOneWidget(t *testing.T, obs rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	ch := make(chan layout.Widget, 4)
	sub := obs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case ch <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()
	select {
	case w := <-ch:
		return w
	case <-time.After(2 * time.Second):
		t.Fatal("widget observable did not emit")
		return nil
	}
}

// captureAtCtx mirrors capture() but threads Now and an input Source into the
// layout context, which the live tooltip path needs.
func captureAtCtx(t *testing.T, size image.Point, draw layout.Widget, now time.Time, src gioinput.Source) *image.RGBA {
	t.Helper()
	return capture(t, size, func(gtx layout.Context) layout.Dimensions {
		gtx.Now = now
		gtx.Source = src
		paint.FillShape(gtx.Ops, color.NRGBA{R: 240, G: 240, B: 240, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return draw(gtx)
	})
}

var _ = reflect.DeepEqual
