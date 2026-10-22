package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheLastRowEndsAboveTheAddButton lays the resting page out under the
// title-bar strip with forty rows, each a solid frame as tall as a todo's
// row, and a button that fills its own frame in a second colour, scrolls the
// list to its end, and reads both frames off the render. The last row's
// frame, its delete mark included, ends above the button's frame, so every
// row's delete mark can be clicked.
func TestTheLastRowEndsAboveTheAddButton(t *testing.T) {
	th := staticThemed(tokens.PlatformLight)
	rowColor := color.NRGBA{R: 0xff, A: 0xff}
	fabColor := color.NRGBA{B: 0xff, A: 0xff}
	solid := func(c color.NRGBA) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
			paint.ColorOp{Color: c}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return layout.Dimensions{Size: size}
		}
	}

	// A todo's row is as tall as its tallest part; the solid rows take
	// that height so the list scrolls as the real one does.
	var ops op.Ops
	rowH := Row(th.typ, rx.Of(th.components), th.palette, Todo{Text: "Todo"})(layout.Context{
		Constraints: layout.Constraints{Max: image.Pt(windowSize.X, windowSize.Y)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}).Size.Y
	if rowH <= 0 {
		t.Fatalf("a todo's row is %d px tall", rowH)
	}
	rows := make([]layout.Widget, 40)
	for i := range rows {
		rows[i] = func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = rowH
			gtx.Constraints.Min.Y = rowH
			return solid(rowColor)(gtx)
		}
	}
	fab := Fab(solid(fabColor), 1.0, 1.0, fabSize, fabSize, false, func(layout.Context) {})
	page := desktop.CapTop(func() unit.Dp { return titleBandDp },
		restingPage(scrollingList(rows, th.listBar(), complist.NewState()), fab))

	scrollToEnd(page)
	img := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(windowSize)
		return page(gtx)
	})

	rowBottom, fabTop := -1, -1
	for y := range windowSize.Y {
		for x := range windowSize.X {
			switch pixelAt(img, image.Pt(x, y)) {
			case rowColor:
				rowBottom = y + 1
			case fabColor:
				if fabTop < 0 {
					fabTop = y
				}
			}
		}
	}
	if rowBottom < 0 || fabTop < 0 {
		t.Fatalf("the render shows no row (bottom %d) or no button (top %d)", rowBottom, fabTop)
	}
	t.Logf("scrolled to the end: last row's bottom %d px, button's top %d px", rowBottom, fabTop)
	if rowBottom > fabTop {
		t.Errorf("scrolled to the end, the last row's frame ends at %d px, %d px into the add button's frame at %d px; want it to end above the button", rowBottom, rowBottom-fabTop, fabTop)
	}
}
