package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// TestTheNotesRowsReachTheTrailingEdgeWhereTheBarFades reads the note
// column's rows against the reader's setting, at a column narrower than the
// measure and both paddings and at one wider. At When scrolling the bar lies
// over the rows' trailing edge and takes no lane, so the rows run to the
// column's trailing edge; at Always the bar has a lane of its own and the
// rows stop short of it. The text is placed between the column's edges at
// either setting: the padding at the trailing edge is the page's inset less
// the lane, so the text does not move when the lane is given back.
func TestTheNotesRowsReachTheTrailingEdgeWhereTheBarFades(t *testing.T) {
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(windowW, windowH)),
		Ops:         new(op.Ops),
	}
	fading := goldenTokens()
	standing := standingBars(fading)
	lane := gtx.Dp(noteScrollBar(standing).Width())
	for _, colW := range []int{526, 774} {
		start, width := noteRowsBox(gtx, noteScrollBar(fading), colW)
		if end := start + width; end != colW {
			t.Errorf("a %d dp column at When scrolling runs its rows to x=%d, want the column's trailing edge %d",
				colW, end, colW)
		}
		start, width = noteRowsBox(gtx, noteScrollBar(standing), colW)
		if end := start + width; end != colW-lane {
			t.Errorf("a %d dp column at Always runs its rows to x=%d, want them to stop short of the %d dp lane at %d",
				colW, end, lane, colW-lane)
		}
		fl, fw := noteTextBox(gtx, fading, colW)
		sl, sw := noteTextBox(gtx, standing, colW)
		if fl != sl || fw != sw {
			t.Errorf("a %d dp column places its text at x=%d over %d dp at When scrolling and at x=%d over %d dp at Always; want the same place",
				colW, fl, fw, sl, sw)
		}
	}
}
