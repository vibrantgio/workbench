package main

// The window's own scroll bars, read at the two settings the platform
// answers with. The bar a reader sees is built from the colours alone and
// then spent against the setting the window read when it was composed; what
// that spending comes to is a bar that stands on a resting frame or one that
// is not there at all, which is what these pixels are.

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheWindowSpendsTheScrollBarSetting lays the window's bar out on a
// frame nothing has scrolled and nothing is pointing at, once per setting.
//
// With the reader's "always" setting the thumb is there: the bar stands for
// as long as the content overflows. With either of the other two it is the
// platform's overlay bar, which a resting frame holds nothing of.
func TestTheWindowSpendsTheScrollBarSetting(t *testing.T) {
	c := tokens.PlatformLight
	// A tab's column rides the content plane, which is what its sections
	// fill with.
	rides := c.ControlBackground
	size := image.Pt(24, 400)
	// The pixels the bar leaves on the fill it rides, on a frame with no
	// scroll and no pointer behind it.
	restingPixels := func(bars desktop.ScrollBarSetting) int {
		style := scrollBarStyle(scrollbar.FromTokens(c, rides), bars)
		state := scrollbar.NewState()
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
			gtx.Now = time.Unix(1700000000, 0)
			paint.FillShape(gtx.Ops, rides, clip.Rect{Max: gtx.Constraints.Max}.Op())
			style.Layout(gtx, state, layout.Vertical, 0.35, 0.65)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		n := 0
		for y := range size.Y {
			for x := range size.X {
				if got := img.RGBAAt(x, y); got.R != rides.R || got.G != rides.G || got.B != rides.B {
					n++
				}
			}
		}
		return n
	}

	if n := restingPixels(desktop.ScrollBarsAlways); n == 0 {
		t.Error("with scroll bars always, a resting frame drew no thumb")
	}
	for _, bars := range []desktop.ScrollBarSetting{desktop.ScrollBarsAutomatic, desktop.ScrollBarsWhenScrolling} {
		if n := restingPixels(bars); n != 0 {
			t.Errorf("at %v, a resting frame drew %d thumb pixels, want none", bars, n)
		}
	}
}
