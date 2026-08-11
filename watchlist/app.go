package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/font"
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

	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/toast"
	"github.com/vibrantgio/patterns/tooltip"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// modelObsConsumers is the EXACT number of cold subscriptions that reach
// modelObs when watchlistShellLayer is subscribed once (as theme/window
// does). It is LOAD-BEARING and must be MEASURED, not hand-counted:
// mvuWin.Messages() drains a channel and rx.Publish() multicasts WITHOUT
// replay, so Publish().AutoConnect(modelObsConsumers) in run() connects the
// loop's upstream scan — and lets the seed emitted by mvu.Loop flow — only
// once the count-th subscription attaches. Too low and late consumers miss the seed (blank
// launch); too high and Connect never fires (frozen app).
//
// The count is MEASURED by TestModelObsConsumerCountMatchesConst (which fails
// if a topology edit changes it without updating this), not hand-counted — the
// measured total is 23 (22 at G5.3c, 11 at G5.3b). The F1.3 theme migration
// left it untouched: mirrorTokens subscribes only the THEME streams, never
// modelObs. G0C.3 added the twenty-third and it is the first entry ADR-008 put
// ON the ledger rather than took off it: the toast queue moved out of a
// process-global rx.Subject and into the model, so toast.Stack reads modelObs
// like every other component.
//
// G0C.4 moved it by NOTHING, and that is the correction worth recording.
// ADR-008 expected the per-row rx.Subject open flags to be what this census
// was counting; they were not. Removing all three of this app's — the row
// delete confirm, the sidebar context menu, the bulk-delete confirm — left it
// at 23, because none of them ever subscribed modelObs. They subscribed the
// THEME, through the popover they fed. This number counts what reads the
// model, and ephemeral interaction state was never in it.
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
//   - modelObs directly   → addSymbolModal + watchlistMain + watchlistSidebar
//   - bulkDeletePopover eager mirrors              (4)
//   - watchlistsObs       → sidebar CombineLatest                         (1)
//   - selectedObs         → sidebar + Main                                (2)
//   - symbolsObs          → Main rowsObs + Main pageCountObs              (2)
//   - modalOpenObs        → symbol modal Open prop                        (1)
//   - modalErrorObs       → symbol modal errorCell mirror                 (1)
//   - editObs             → symbol modal per-field epoch SwitchMaps ×4    (4)
//   - selectionObs        → Main rowsObs                                  (1)
//   - pageObs             → Main rowsObs + Main paginationObs             (2)
//   - renameOpenObs       → rename modal Open prop                        (1)
//   - renameErrorObs      → rename modal errorCell mirror                 (1)
//   - renameEditObs       → rename modal name-field epoch SwitchMap       (1)
//   - toastsObs           → toast.Stack Toasts prop (G0C.3)               (1)
//
// (Trust the measured 23 over this breakdown if they ever disagree.)
const modelObsConsumers = 23

// themeTokens is the colour/typography snapshot the app's own drawing code
// reads at frame time. The shaper is the theme's cached Typography shaper
// (F1.3): the app builds none of its own, so the typeface — Roboto — comes
// from the theme.
type themeTokens struct {
	col    tokens.ColorTokens
	typ    tokens.Typography
	shaper *text.Shaper
}

// mirrorTokens subscribes the theme's Color and Typography streams into an
// atomic cell and returns a frame-time loader. It is the layer-boundary
// adapter for closures that run outside any rx scope (static component
// slots, table cell closures, navbar widgets) — the same hand-off pattern
// feeds uses (F1.2).
func mirrorTokens(th rx.Observable[theme.Theme]) func() themeTokens {
	var cell atomic.Value
	cell.Store(themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		shaper: tokens.DefaultTypography.Shaper(),
	})
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	_ = rx.CombineLatest2(colorObs, typObs).Subscribe(rx.GoroutineContext(), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography], _ error, done bool) {
		if !done {
			typ := t.Second
			cell.Store(themeTokens{col: t.First, typ: typ, shaper: typ.Shaper()})
		}
	})
	return func() themeTokens { return cell.Load().(themeTokens) }
}

// buildLayers returns the theme/window build function: a Surface backdrop
// under the watchlist shell. storePath is the on-disk file the save callback
// writes back to atomically; tests inject a t.TempDir() path.
func buildLayers(modelObs rx.Observable[Model], storePath string) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			watchlistShellLayer(th, modelObs, storePath),
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
// patterns/shell exposes Sidebar as an rx.Observable[layout.Widget] but Main
// (and navbar Actions) as static layout.Widget slots, and Shell re-emits only
// when its Sidebar or Navbar stream emits. So the live Main widget is folded
// onto the sidebar-driving observable and the latest is published into an
// atomic layer-boundary cell read by the static Main slot at frame time — the
// same hand-off feeds uses. Any model change therefore re-emits the sidebar
// stream, which makes Shell re-emit and the window repaint on the same frame.
func watchlistShellLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	storePath string,
) rx.Observable[layout.Widget] {
	// This window's arbitration registers (ADR-008). They are plain values
	// with no synchronisation, so the scope they are created at is the scope
	// they are safe at: theme/window calls the build function once per
	// window and this layer is composed exactly once inside it, which makes
	// this function body the window. Every popover, tooltip and modal below
	// is handed one of these — a second arbitrable LAYER would have to take
	// them as parameters instead, because it would be composed beside this
	// one rather than within it.
	popArb := popover.NewArbiter()
	tipArb := tooltip.NewArbiter()
	modalArb := modal.NewArbiter()

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
	toastsObs := rx.Map(modelObs, func(m Model) []toast.Toast { return m.toasts.Items() })

	sidebarObs := watchlistSidebar(th, watchlistsObs, selectedObs, storePath, modelObs, popArb)
	mainObs := watchlistMain(th, selectedObs, symbolsObs, selectionObs, pageObs, storePath, modelObs, popArb, tipArb)
	modalObs := addSymbolModal(th, storePath, modelObs, modalOpenObs, modalErrorObs, editObs, modalArb)
	renameModalObs := renameWatchlistModal(th, storePath, modelObs, renameOpenObs, renameErrorObs, renameEditObs, modalArb)
	bulkDeleteObs := bulkDeletePopover(th, storePath, modelObs, popArb)
	toastObs := toast.Stack(th, toast.Props{Position: toast.TopRight, Toasts: toastsObs})

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
		Navbar:  watchlistNavbarProps(mirrorTokens(th), bulkSlot),
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
// "New watchlist" action (creation arrives in a later G5.3 task). Brand and
// action labels are the app's own text, so they read the theme snapshot from
// loadTok at frame time: the brand in TitleMedium on the Text pin, the action
// in LabelLarge on the Primary pin (the accent role the hardcoded link-blue
// approximated).
func watchlistNavbarProps(loadTok func() themeTokens, bulkSlot layout.Widget) navbar.Props {
	brand := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		return drawLabel(gtx, s.shaper, "Watchlist editor", s.typ.TitleMedium, s.col.Text)
	}
	var newClick widget.Clickable
	newWatchlist := func(gtx layout.Context) layout.Dimensions {
		// No-op for now: consuming the click keeps the affordance live without
		// landing a message (creation is out of scope for G5.3a).
		newClick.Clicked(gtx)
		s := loadTok()
		return newClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("New watchlist").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, s.shaper, "New watchlist", s.typ.LabelLarge, s.col.Primary)
		})
	}
	return navbar.Props{
		Brand:   brand,
		Actions: []layout.Widget{bulkSlot, newWatchlist},
	}
}

// drawLabel renders a single-line label in one Typography role — typeface,
// weight, size and line height all come from the theme's TextStyle.
func drawLabel(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	style tokens.TextStyle,
	c color.NRGBA,
) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	return typeset.Layout(gtx, shaper, typeset.Label(style, 1),
		typeset.Font(style, font.Normal), unit.Sp(style.Size), msg, material)
}

func drawLabelAt(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	style tokens.TextStyle,
	c color.NRGBA,
	at image.Point,
) {
	stk := op.Offset(at).Push(gtx.Ops)
	drawLabel(gtx, shaper, msg, style, c)
	stk.Pop()
}
