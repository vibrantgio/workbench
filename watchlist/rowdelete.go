// rowdelete.go owns one symbol row's delete-confirm popover: the trash-icon
// anchor and a "Delete this symbol?" confirm surface. Open state is EPHEMERAL
// per-row interaction state, NOT model state (logged choice in
// FEEDBACK-G5.3.md): a plain bool this file owns, written and read during
// layout on the frame goroutine, which cadence/popover reads back through
// Props.OpenNow — ADR-008 destination 2. Every row's popover shares the
// window's Arbiter, so opening one row's confirm dismisses whichever row had
// it open. The trash click toggles it; the confirm click writes the file
// (applyDelete via deleteSymbolAt), fires a toast, lands DeleteSymbol, and
// closes; OnDismiss closes.
//
// Until G0C.4 the flag was a per-row rx.Subject with an atomic.Bool mirror
// beside it, and the flip crossed to the rx goroutine and back before any
// frame could see it. Both are gone; the atomic cell that remains carries the
// THEME's re-emissions, which really do arrive from another goroutine.
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
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/spectrum/theme"
)

const (
	rowConfirmWDp   = 148
	rowConfirmRowDp = 28
)

// rowDeleteConfirm is one symbol row's trash anchor + delete-confirm popover.
type rowDeleteConfirm struct {
	idx int
	// open is frame state: only layout writes it and only layout reads it.
	open bool
	cell atomic.Value // latest popover layout.Widget
}

func newRowDeleteConfirm(
	th rx.Observable[theme.Theme],
	idx int,
	storePath string,
	trashClick *widget.Clickable,
	confirmClick *widget.Clickable,
	loadModel func() Model,
	popArb *popover.Arbiter,
) *rowDeleteConfirm {
	dc := &rowDeleteConfirm{idx: idx}

	loadTok := mirrorTokens(th)

	anchor := func(gtx layout.Context) layout.Dimensions {
		if trashClick.Clicked(gtx) {
			dc.toggle()
		}
		return trashClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := loadTok()
			semantic.LabelOp("Delete symbol").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			sz := gtx.Constraints.Max
			// Low-contrast icon step: Neutral 700, the OnSurfaceVariant
			// alias's resolution.
			drawTrashIcon(gtx, sz, s.col.Ramps.Neutral.Step(700))
			return layout.Dimensions{Size: sz}
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if confirmClick.Clicked(gtx) {
			m := loadModel()
			next := deleteSymbolAt(m.watchlists, m.selected, dc.idx)
			if err := saveStore(storePath, documentOf(next, m.selected)); err == nil {
				toast.Notify(gtx, toast.Success, "Symbol deleted")
			} else {
				toast.Notify(gtx, toast.Error, "Delete failed")
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
		drawLabel(gtx, s.shaper, "Delete this symbol?", s.typ.BodyMedium, s.col.Ramps.Neutral.Step(900))
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm delete").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			drawLabel(gtx, s.shaper, "Delete", s.typ.LabelLarge, s.col.Error)
			return layout.Dimensions{Size: image.Pt(w, btnH)}
		})
		btnStk.Pop()
		return layout.Dimensions{Size: image.Pt(w, promptH+btnH)}
	}

	popObs := popover.Popover(th, popover.Props{
		OpenNow:   func() bool { return dc.open },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Left,
		Arbiter:   popArb,
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

// toggle and close run during layout, from the anchor click, the confirm
// click and the arbiter's OnDismiss — all three on the frame goroutine, which
// is what lets open be a plain bool.
func (dc *rowDeleteConfirm) toggle() { dc.open = !dc.open }

func (dc *rowDeleteConfirm) close() { dc.open = false }

// layout draws the trash gutter for one row via the per-row popover widget (the
// trash anchor always, plus the confirm surface while open).
func (dc *rowDeleteConfirm) layout(gtx layout.Context) layout.Dimensions {
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
