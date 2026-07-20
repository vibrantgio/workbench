// rowdelete.go owns one symbol row's delete-confirm popover: the trash-icon
// anchor and a "Delete this symbol?" confirm surface. Open state is EPHEMERAL
// per-row interaction state (a per-row rx.Subject driving a per-row
// cadence/popover instance), NOT model state — the feeds idiom (logged choice
// in FEEDBACK-G5.3.md). The trash click toggles open; the confirm click writes
// the file (applyDelete via deleteSymbolAt), fires a toast, lands DeleteSymbol,
// and closes; OnDismiss closes.
//
// popover-canvas coupling (FEEDBACK-G5.2 Major, recurring): the popover centres
// its anchor in the canvas and measures Content at canvas/2, so the trash cell
// is wrapped as an Exact canvas and the Content overrides its incoming
// constraints to self-size (the cell is too small to hold a confirm prompt).

package main

import (
	"image"
	"image/color"
	"sync/atomic"

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

	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

const (
	rowConfirmWDp   = 148
	rowConfirmRowDp = 28
)

// rowDeleteConfirm is one symbol row's trash anchor + delete-confirm popover.
type rowDeleteConfirm struct {
	idx     int
	openCh  rx.Observer[bool]
	openVal atomic.Bool
	cell    atomic.Value // latest popover layout.Widget
}

func newRowDeleteConfirm(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	idx int,
	storePath string,
	trashClick *widget.Clickable,
	confirmClick *widget.Clickable,
	loadModel func() Model,
) *rowDeleteConfirm {
	dc := &rowDeleteConfirm{idx: idx}
	send, openObs := rx.Subject[bool](0, 1)
	send.Next(false)
	dc.openCh = send

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

	anchor := func(gtx layout.Context) layout.Dimensions {
		if trashClick.Clicked(gtx) {
			dc.toggle()
		}
		return trashClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := tokenCell.Load().(tokenState)
			semantic.LabelOp("Delete symbol").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			sz := gtx.Constraints.Max
			drawTrashIcon(gtx, sz, s.col.OnSurfaceVariant)
			return layout.Dimensions{Size: sz}
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := tokenCell.Load().(tokenState)
		if confirmClick.Clicked(gtx) {
			m := loadModel()
			next := deleteSymbolAt(m.watchlists, m.selected, dc.idx)
			if err := saveStore(storePath, documentOf(next, m.selected)); err == nil {
				toast.Notify(toast.Success, "Symbol deleted")
			} else {
				toast.Notify(toast.Error, "Delete failed")
			}
			mvu.MessageOp{Message: DeleteSymbol{Row: dc.idx}}.Add(gtx.Ops)
			dc.close()
		}
		// Override the incoming canvas/2 constraints: the popover sized the
		// anchor canvas to the tiny trash gutter, so half of it cannot hold a
		// confirm prompt. Size the content ourselves; popover pads it.
		w := gtx.Dp(unit.Dp(rowConfirmWDp))
		promptH := gtx.Dp(unit.Dp(rowConfirmRowDp))
		btnH := gtx.Dp(unit.Dp(rowConfirmRowDp))
		drawLabel(gtx, shaper, "Delete this symbol?", unit.Sp(s.typ.BodyMedium), s.col.OnSurface)
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm delete").Add(gtx.Ops)
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
		Placement: popover.Left,
		OnDismiss: func(layout.Context) { dc.close() },
	})
	dc.cell.Store(layout.Widget(nil))
	_ = popObs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			dc.cell.Store(w)
		}
	})
	return dc
}

func (dc *rowDeleteConfirm) toggle() {
	next := !dc.openVal.Load()
	dc.openVal.Store(next)
	dc.openCh.Next(next)
}

func (dc *rowDeleteConfirm) close() {
	if dc.openVal.Swap(false) {
		dc.openCh.Next(false)
	}
}

// layout draws the trash gutter for one row via the per-row popover widget (the
// trash anchor always, plus the confirm surface while open).
func (dc *rowDeleteConfirm) layout(gtx layout.Context, _ tokens.ColorTokens) layout.Dimensions {
	size := gtx.Constraints.Max
	if w, ok := dc.cell.Load().(layout.Widget); ok && w != nil {
		w(gtx)
	}
	return layout.Dimensions{Size: size}
}

// drawTrashIcon paints a minimal trash glyph (a lid line + a body box) centred
// in box, in colour col. clip/paint only, so it stays golden-deterministic.
func drawTrashIcon(gtx layout.Context, box image.Point, col color.NRGBA) {
	side := box.X
	if box.Y < side {
		side = box.Y
	}
	pad := gtx.Dp(unit.Dp(8))
	x0 := (box.X-side)/2 + pad
	x1 := box.X - (box.X-side)/2 - pad
	y0 := (box.Y-side)/2 + pad
	y1 := box.Y - (box.Y-side)/2 - pad
	stroke := gtx.Dp(unit.Dp(1))
	if stroke < 1 {
		stroke = 1
	}
	lidY := y0 + (y1-y0)/5
	rect(gtx, image.Rect(x0, lidY, x1, lidY+stroke+1), col)
	rect(gtx, image.Rect(x0, lidY, x0+stroke+1, y1), col)
	rect(gtx, image.Rect(x1-stroke-1, lidY, x1, y1), col)
	rect(gtx, image.Rect(x0, y1-stroke-1, x1, y1), col)
}

func rect(gtx layout.Context, r image.Rectangle, col color.NRGBA) {
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}
