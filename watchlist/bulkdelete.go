// bulkdelete.go composes the navbar "Delete N" bulk-delete action: an anchor
// labelled "Delete N" (N = the current selection count) and a cadence/popover
// confirm showing the count. The action HIDES itself when N == 0 (decided:
// hide, not disable — a "Delete 0" affordance is meaningless; logged in
// FEEDBACK-G5.3.md). The confirm click writes the file (bulkDeleteRows), fires
// a toast, lands BulkDelete, and closes.
//
// popover-canvas coupling (FEEDBACK-G5.2 recurrence): the anchor centres in its
// canvas and Content measures at canvas/2, so the action is rendered inside the
// navbar Actions slot's canvas and the Content overrides its constraints to
// self-size. Open state is ephemeral, like the row confirms: a plain bool
// owned by this closure, written and read during layout on the frame
// goroutine and read back by cadence/popover through Props.OpenNow — ADR-008
// destination 2. It joins the window's Arbiter, so opening it closes whatever
// row confirm was up, and vice versa.
//
// Until G0C.4 the flag was an rx.Subject with an atomic.Bool mirror beside
// it, and the auto-close below ran on the rx goroutine, which is the one
// write in this app that a plain bool would have made a data race. It now
// happens where it belongs: during the layout pass that has just decided the
// action is not on screen.

package main

import (
	"image"
	"strconv"
	"sync/atomic"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
)

const (
	bulkConfirmWDp   = 168
	bulkConfirmRowDp = 28
)

// bulkDeletePopover returns the navbar "Delete N" action observable. It re-emits
// on theme changes; the selection count is read live from a model mirror each
// frame so the count + visibility track the model without re-subscription.
func bulkDeletePopover(
	th rx.Observable[theme.Theme],
	storePath string,
	modelMirrorObs rx.Observable[Model],
	popArb *popover.Arbiter,
) rx.Observable[layout.Widget] {
	// open is frame state: the anchor click, the confirm click, OnDismiss and
	// the hide-when-empty check below are the only writers, and every one of
	// them runs during layout.
	var open bool
	toggle := func() { open = !open }
	closePop := func() { open = false }

	loadTok := mirrorTokens(th)

	// Model mirror: the live selection count + the watchlists/selection the
	// confirm callback writes from. Read at frame time (count) and at confirm.
	var modelCell atomic.Value
	modelCell.Store(Model{editIndex: -1})
	_ = modelMirrorObs.Subscribe(rx.GoroutineContext(), func(m Model, _ error, done bool) {
		if !done {
			modelCell.Store(m)
		}
	})
	selCount := func() int {
		m, _ := modelCell.Load().(Model)
		return len(m.selection)
	}

	var anchorClick, confirmClick widget.Clickable

	anchor := func(gtx layout.Context) layout.Dimensions {
		if anchorClick.Clicked(gtx) {
			toggle()
		}
		s := loadTok()
		label := "Delete " + strconv.Itoa(selCount())
		return anchorClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, s.shaper, label, s.typ.LabelLarge, s.col.Error)
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if confirmClick.Clicked(gtx) {
			m, _ := modelCell.Load().(Model)
			next := bulkDeleteRows(m.watchlists, m.selected, selectedRows(m.selection))
			if err := saveStore(storePath, documentOf(next, m.selected)); err == nil {
				toast.Notify(gtx, toast.Success, "Symbols deleted")
			} else {
				toast.Notify(gtx, toast.Error, "Delete failed")
			}
			mvu.MessageOp{Message: BulkDelete{}}.Add(gtx.Ops)
			closePop()
		}
		w := gtx.Dp(unit.Dp(bulkConfirmWDp))
		promptH := gtx.Dp(unit.Dp(bulkConfirmRowDp))
		btnH := gtx.Dp(unit.Dp(bulkConfirmRowDp))
		prompt := "Delete " + strconv.Itoa(selCount()) + " symbols?"
		drawLabel(gtx, s.shaper, prompt, s.typ.BodyMedium, s.col.Ramps.Neutral.Step(900))
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm bulk delete").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			drawLabel(gtx, s.shaper, "Delete", s.typ.LabelLarge, s.col.Error)
			return layout.Dimensions{Size: image.Pt(w, btnH)}
		})
		btnStk.Pop()
		return layout.Dimensions{Size: image.Pt(w, promptH+btnH)}
	}

	popObs := popover.Popover(th, popover.Props{
		OpenNow:   func() bool { return open },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		Arbiter:   popArb,
		OnDismiss: func(layout.Context) { closePop() },
	})

	// Hide the whole action (anchor + popover) when nothing is selected, and
	// close the confirm on the way out — the selection can empty from under
	// an open confirm (a SelectWatchlist clears the set), and an action that
	// is not on screen must not come back already open.
	//
	// The popover is not laid out on those frames, so per ADR-008 it neither
	// claims nor releases arbitration top: a confirm hidden this way keeps a
	// hold nothing is drawing. Nothing is painted from it and the next
	// claimant evicts it in the ordinary way; the hold is released properly
	// the first frame the action is back on screen and laid out closed.
	return rx.Map(popObs, func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if selCount() == 0 {
				closePop()
				return layout.Dimensions{}
			}
			return w(gtx)
		}
	})
}
