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
// self-size. Open state is ephemeral (per-instance rx.Subject), like the row
// confirms.

package main

import (
	"image"
	"strconv"
	"sync/atomic"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
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
	shaper *text.Shaper,
	storePath string,
	modelMirrorObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	send, openObs := rx.Subject[bool](0, 1)
	send.Next(false)
	var openVal atomic.Bool
	toggle := func() {
		n := !openVal.Load()
		openVal.Store(n)
		send.Next(n)
	}
	closePop := func() {
		if openVal.Swap(false) {
			send.Next(false)
		}
	}

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

	// Model mirror: the live selection count + the watchlists/selection the
	// confirm callback writes from. Read at frame time (count) and at confirm.
	var modelCell atomic.Value
	modelCell.Store(Model{editIndex: -1})
	_ = modelMirrorObs.Subscribe(rx.GoroutineContext(), func(m Model, _ error, done bool) {
		if !done {
			modelCell.Store(m)
			// Auto-close the confirm if the selection emptied out from under it
			// (e.g. a SelectWatchlist cleared the set while the popover was open).
			if len(m.selection) == 0 {
				closePop()
			}
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
		s := tokenCell.Load().(tokenState)
		label := "Delete " + strconv.Itoa(selCount())
		return anchorClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, shaper, label, unit.Sp(14), s.col.Error)
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := tokenCell.Load().(tokenState)
		if confirmClick.Clicked(gtx) {
			m, _ := modelCell.Load().(Model)
			next := bulkDeleteRows(m.watchlists, m.selected, selectedRows(m.selection))
			if err := saveStore(storePath, documentOf(next, m.selected)); err == nil {
				toast.Notify(toast.Success, "Symbols deleted")
			} else {
				toast.Notify(toast.Error, "Delete failed")
			}
			mvu.MessageOp{Message: BulkDelete{}}.Add(gtx.Ops)
			closePop()
		}
		w := gtx.Dp(unit.Dp(bulkConfirmWDp))
		promptH := gtx.Dp(unit.Dp(bulkConfirmRowDp))
		btnH := gtx.Dp(unit.Dp(bulkConfirmRowDp))
		prompt := "Delete " + strconv.Itoa(selCount()) + " symbols?"
		drawLabel(gtx, shaper, prompt, unit.Sp(s.typ.BodyMedium), s.col.OnSurface)
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm bulk delete").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			drawLabel(gtx, shaper, "Delete", unit.Sp(s.typ.LabelLarge), s.col.Error)
			return layout.Dimensions{Size: image.Pt(w, btnH)}
		})
		btnStk.Pop()
		return layout.Dimensions{Size: image.Pt(w, promptH+btnH)}
	}

	popObs := popover.Popover(th, popover.Props{
		Open:      openObs,
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		OnDismiss: func(layout.Context) { closePop() },
	})

	// Hide the whole action (anchor + popover) when nothing is selected.
	return rx.Map(popObs, func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if selCount() == 0 {
				return layout.Dimensions{}
			}
			return w(gtx)
		}
	})
}
