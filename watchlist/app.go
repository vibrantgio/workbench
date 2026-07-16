package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/font/gofont"
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

	"github.com/vibrantgio/cadence/navbar"
	"github.com/vibrantgio/cadence/shell"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// modelObsConsumers is the EXACT number of cold subscriptions that reach
// modelObs when watchlistShellLayer is subscribed once (as spectrum/window
// does). It is LOAD-BEARING and must be MEASURED, not hand-counted:
// mvuWin.Messages() drains a channel and rx.Publish() multicasts WITHOUT
// replay, so Publish().AutoConnect(modelObsConsumers) in run() connects the
// loop's upstream scan — and lets the seed emitted by mvu.Loop flow — only
// once the count-th subscription attaches. Too low and late consumers miss the seed (blank
// launch); too high and Connect never fires (frozen app).
//
// The count is MEASURED by TestModelObsConsumerCountMatchesConst (which fails
// if a topology edit changes it without updating this), not hand-counted — the
// measured G5.3c total is 22 (was 11 at G5.3b).
//
// CRITICAL INVARIANT (logged in FEEDBACK-G5.3.md): NEVER subscribe modelObs
// inside a keyed.Defer (per-row/per-name). A lazy subscription attaches during
// the first LAYOUT frame — AFTER the seed emission has already
// fired — so it (a) is invisible to the count test, which never lays out, and
// (b) never receives the seed, leaving its mirror at the zero Model; a
// pre-interaction delete then writes an EMPTY document over the user's file.
// All per-row/per-name surfaces (row delete confirm, sidebar context menu) read
// the model through ONE eager mirror their parent layer subscribes in its body
// and shares as a `func() Model`. That keeps the count STATIC (independent of
// the watchlist/symbol count) and seed-correct.
//
// The contributing fan-out (modelObs is passed BOTH directly as eager mirrors
// AND projected into the derived streams below):
//  - modelObs directly   → addSymbolModal + watchlistMain + watchlistSidebar
//                          + bulkDeletePopover eager mirrors              (4)
//  - watchlistsObs       → sidebar CombineLatest                         (1)
//  - selectedObs         → sidebar + Main                                (2)
//  - symbolsObs          → Main rowsObs + Main pageCountObs              (2)
//  - modalOpenObs        → symbol modal Open prop                        (1)
//  - modalErrorObs       → symbol modal errorCell mirror                 (1)
//  - editObs             → symbol modal per-field epoch SwitchMaps ×4    (4)
//  - selectionObs        → Main rowsObs                                  (1)
//  - pageObs             → Main rowsObs + Main paginationObs             (2)
//  - renameOpenObs       → rename modal Open prop                        (1)
//  - renameErrorObs      → rename modal errorCell mirror                 (1)
//  - renameEditObs       → rename modal name-field epoch SwitchMap       (1)
// (Trust the measured 22 over this breakdown if they ever disagree.)
const modelObsConsumers = 22

// buildLayers returns the spectrum/window build function: a Surface backdrop
// under the watchlist shell. storePath is the on-disk file the save callback
// writes back to atomically; tests inject a t.TempDir() path.
func buildLayers(modelObs rx.Observable[Model], storePath string) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			watchlistShellLayer(th, shaper, modelObs, storePath),
		}
	}
}

func backdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	return rx.Map(
		rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
			return t.Color
		}),
		func(c tokens.ColorTokens) layout.Widget {
			fill := c.Surface
			return func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
				return layout.Dimensions{Size: size}
			}
		},
	)
}

// watchlistShellLayer composes the watchlists sidebar, the navbar (brand +
// no-op "New watchlist" action), and the Main placeholder into a
// SidebarHeaderMain shell. The sidebar row list and active row, plus the Main
// placeholder, are all derived from modelObs; theme tokens flow independently
// through th.
//
// cadence/shell exposes Sidebar as an rx.Observable[layout.Widget] but Main
// (and navbar Actions) as static layout.Widget slots, and Shell re-emits only
// when its Sidebar or Navbar stream emits. So the live Main widget is folded
// onto the sidebar-driving observable and the latest is published into an
// atomic layer-boundary cell read by the static Main slot at frame time — the
// same hand-off feeds uses. Any model change therefore re-emits the sidebar
// stream, which makes Shell re-emit and the window repaint on the same frame.
func watchlistShellLayer(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	modelObs rx.Observable[Model],
	storePath string,
) rx.Observable[layout.Widget] {
	// Cold derivations of modelObs. Their fan-out is mirrored by
	// modelObsConsumers above — keep them in sync.
	watchlistsObs := rx.Map(modelObs, func(m Model) []Watchlist { return m.watchlists })
	selectedObs := rx.Map(modelObs, func(m Model) string { return m.selected })
	symbolsObs := rx.Map(modelObs, func(m Model) []Symbol {
		wl, _ := m.selectedWatchlist()
		return wl.Symbols
	})
	modalOpenObs := rx.Map(modelObs, func(m Model) bool { return m.modalOpen })
	modalErrorObs := rx.Map(modelObs, func(m Model) bool { return m.modalError })
	// editObs carries the modal's epoch + the seed row so the uncontrolled
	// TextFields rebuild fresh (and re-seed) on every open. Keyed on the epoch,
	// not editIndex, so reopening the SAME row after a cancel still re-emits.
	editObs := rx.Map(modelObs, func(m Model) editTarget {
		return editTarget{epoch: m.modalEpoch, seed: m.editSeed}
	})
	// G5.3c model-derived streams: the bulk-select set, the current page, and
	// the rename-modal state (same epoch-rebuild workaround as the symbol modal).
	selectionObs := rx.Map(modelObs, func(m Model) map[int]bool { return m.selection })
	pageObs := rx.Map(modelObs, func(m Model) int {
		if m.currentPage < 1 {
			return 1
		}
		return m.currentPage
	})
	renameOpenObs := rx.Map(modelObs, func(m Model) bool { return m.renameOpen })
	renameErrorObs := rx.Map(modelObs, func(m Model) bool { return m.renameError })
	renameEditObs := rx.Map(modelObs, func(m Model) renameTarget {
		return renameTarget{epoch: m.renameEpoch, target: m.renameTarget, seed: m.renameSeed}
	})

	sidebarObs := watchlistSidebar(th, shaper, watchlistsObs, selectedObs, storePath, modelObs)
	mainObs := watchlistMain(th, shaper, selectedObs, symbolsObs, selectionObs, pageObs, storePath, modelObs)
	modalObs := addSymbolModal(th, shaper, storePath, modelObs, modalOpenObs, modalErrorObs, editObs)
	renameModalObs := renameWatchlistModal(th, shaper, storePath, modelObs, renameOpenObs, renameErrorObs, renameEditObs)
	bulkDeleteObs := bulkDeletePopover(th, shaper, storePath, modelObs)
	toastObs := toast.Stack(th, toast.Props{Position: toast.TopRight, Shaper: shaper})

	// mainCell bridges the live Main widget stream into shell's static Main slot.
	var mainCell atomic.Value
	mainSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := mainCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	// bulkCell bridges the live "Delete N" navbar action (its anchor +
	// confirm popover) into shell's STATIC navbar Actions slot — the same
	// observable-over-static-slot hand-off as Main (logged in FEEDBACK-G5.3.md).
	// The action hides itself when the selection is empty (decided: hide, not
	// disable — a "Delete 0" affordance is meaningless; logged).
	var bulkCell atomic.Value
	bulkSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := bulkCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	// Fold the Main widget + bulk-delete action onto the sidebar stream: store
	// the latest into their cells and return the sidebar widget. Every model
	// change re-emits the sidebar stream → Shell re-emits → same-frame repaint.
	sidebarDriven := rx.Map(
		rx.CombineLatest3(sidebarObs, mainObs, bulkDeleteObs),
		func(n rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			mainCell.Store(n.Second)
			bulkCell.Store(n.Third)
			return n.First
		},
	)

	shellObs := shell.Shell(th, shell.Props{
		Layout:  shell.SidebarHeaderMain,
		Sidebar: sidebarDriven,
		Navbar:  watchlistNavbarProps(shaper, bulkSlot),
		Main:    mainSlot,
	})

	// Overlay composition (same hand-off feeds uses): the modal scrim and the
	// toast stack draw OVER the whole window. Fold them onto the shell stream
	// and draw them after the shell inside the returned widget, reporting the
	// shell's dims. Every model change still re-emits this stream.
	return rx.Map(
		rx.CombineLatest4(shellObs, modalObs, renameModalObs, toastObs),
		func(n rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			shellW, modalW, renameW, toastW := n.First, n.Second, n.Third, n.Fourth
			return func(gtx layout.Context) layout.Dimensions {
				dims := shellW(gtx)
				if modalW != nil {
					modalW(gtx)
				}
				if renameW != nil {
					renameW(gtx)
				}
				if toastW != nil {
					toastW(gtx)
				}
				return dims
			}
		},
	)
}

// watchlistNavbarProps builds the navbar: the "Watchlist editor" brand, the
// live "Delete N" bulk-delete action (bridged through bulkSlot), and a no-op
// "New watchlist" action (creation arrives in a later G5.3 task).
func watchlistNavbarProps(shaper *text.Shaper, bulkSlot layout.Widget) navbar.Props {
	brand := func(gtx layout.Context) layout.Dimensions {
		return drawLabel(gtx, shaper, "Watchlist editor", unit.Sp(18), color.NRGBA{A: 0xff})
	}
	var newClick widget.Clickable
	newWatchlist := func(gtx layout.Context) layout.Dimensions {
		// No-op for now: consuming the click keeps the affordance live without
		// landing a message (creation is out of scope for G5.3a).
		newClick.Clicked(gtx)
		return newClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("New watchlist").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, shaper, "New watchlist", unit.Sp(14), color.NRGBA{R: 0x60, G: 0x80, B: 0xff, A: 0xff})
		})
	}
	return navbar.Props{
		Brand:   brand,
		Shaper:  shaper,
		Actions: []layout.Widget{bulkSlot, newWatchlist},
	}
}

func drawLabel(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	size unit.Sp,
	c color.NRGBA,
) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	wl := widget.Label{MaxLines: 1}
	return wl.Layout(gtx, shaper, font.Font{}, size, msg, material)
}

func drawLabelAt(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	size unit.Sp,
	c color.NRGBA,
	at image.Point,
) {
	stk := op.Offset(at).Push(gtx.Ops)
	drawLabel(gtx, shaper, msg, size, c)
	stk.Pop()
}
