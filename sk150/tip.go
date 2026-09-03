package main

// Hint cells: a grid cell that, hovered anywhere for a dwell, shows the
// setting's explanation in an inverse bubble beneath it. A
// trimmed copy of patterns/tooltip — that pattern centres its trigger on
// the canvas it is handed and sizes the bubble from that canvas, so it
// cannot sit inline as a cell label; this keeps its dwell, its inverse
// surface and its one-visible-at-a-time rule, and paints the bubble
// through op.Defer so it lands over the rest of the grid.

import (
	"image"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// tipDelay is the hover dwell before a bubble shows — the motion scale's
// slowest stop, the same one patterns/tooltip uses.
var tipDelay = tokens.Motion.DurXSlow

// tips is the hint cells' cross-frame state, one per window at
// subscription scope: a hover tracker and dwell start per cell, and the
// cell whose bubble is up. Read and written only during layout.
type tips struct {
	hov   map[string]*gesture.Hover
	entry map[string]time.Time
	top   string
	winW  int // the window's width this frame, for keeping bubbles inside it
}

func newTips() *tips {
	return &tips{hov: map[string]*gesture.Hover{}, entry: map[string]time.Time{}}
}

// wrap makes w a hint-bearing cell: the pointer anywhere over it — label,
// field, button or value — starts the dwell, after which long appears in
// a bubble under the cell's left edge. The hover area is laid over the
// cell in pass-through mode, so the controls inside still get their own
// events.
//
// col is the grid column the cell is in and colW the grid's column width;
// with the window width they bound how far right the bubble may reach.
func (tp *tips) wrap(t themed, key, long string, col int, colW unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		h, ok := tp.hov[key]
		if !ok {
			h = new(gesture.Hover)
			tp.hov[key] = h
		}
		hovered := h.Update(gtx.Source)
		now := gtx.Now
		e := tp.entry[key]
		switch {
		case hovered && e.IsZero():
			tp.entry[key] = now
			gtx.Execute(op.InvalidateCmd{At: now.Add(tipDelay)})
		case !hovered && !e.IsZero():
			delete(tp.entry, key)
			if tp.top == key {
				tp.top = ""
			}
		}
		if hovered && !e.IsZero() && !now.Before(e.Add(tipDelay)) {
			tp.top = key
		}

		dims := w(gtx)
		area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		h.Add(gtx.Ops)
		pass.Pop()
		area.Pop()

		if tp.top == key {
			macro := op.Record(gtx.Ops)
			cellLeft := gtx.Dp(Padding+4) + col*gtx.Dp(colW+compactGap)
			tp.bubble(gtx, t, long, dims.Size.Y+gtx.Dp(2), tp.winW-gtx.Dp(Padding)-cellLeft)
			op.Defer(gtx.Ops, macro.Stop())
		}
		return dims
	}
}

// bubble paints the explanation in the inverse surface under its ink,
// under the cell at y, left-aligned with it unless that would run past
// avail — the room between the cell's left edge and the window's right
// padding — in which case it slides left to fit.
func (tp *tips) bubble(gtx layout.Context, t themed, txt string, y, avail int) {
	typ := t.typ
	padH, padV := gtx.Dp(8), gtx.Dp(4)
	// The bubble is measured and painted with room of its own, not the
	// cell's constraints.
	gtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(640), gtx.Dp(120))}
	sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Small, txt)
	w := sz.X + 2*padH
	x := 0
	if w > avail {
		x = avail - w
	}
	r := image.Rect(x, y, x+w, y+sz.Y+2*padV)
	paint.FillShape(gtx.Ops, t.palette.TipFill, clip.UniformRRect(r, gtx.Dp(4)).Op(gtx.Ops))
	textdraw.FillText(gtx, typ.Shaper, typ.Small,
		image.Rect(x+padH, y+padV, x+padH+sz.X, y+padV+sz.Y), 0, 0.5, t.palette.TipInk, txt)
}
