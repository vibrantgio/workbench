package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestPresetsColumnHeadsStayAboveTheRows draws the Presets tab at Always in a
// frame shorter than its slots, at rest and after a scroll partway, in both
// schemes.
//
// The heading, its line and the column heads are drawn above the region that
// scrolls: after the scroll the column heads are drawn at the same place, the
// slots under them have moved, and the bar's lane starts under the heads, so
// the thumb is never drawn beside them.
func TestPresetsColumnHeadsStayAboveTheRows(t *testing.T) {
	size := image.Pt(int(winW), 300)
	frameRight := size.X - int(Padding)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			m := goldenModel()
			m.Screen = "presets"
			m.HavePresets = true
			w := windowFrameBars(t, tc.c, m, postMessage, desktop.ScrollBarsAlways)
			var (
				ops op.Ops
				r   gioinput.Router
			)
			frame := func() {
				ops.Reset()
				w(layout.Context{
					Constraints: layout.Exact(size),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Ops:         &ops,
					Source:      r.Source(),
				})
				r.Frame(&ops)
			}
			shot := func() *image.RGBA {
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Source = r.Source()
					return w(gtx)
				})
			}
			frame()
			frame()
			rest := shot()

			fill := pageFill(tc.c)
			drawn := func(img *image.RGBA, x, y int) bool { return at(img, x, y) != fill }
			bar := themed{platform: tc.c, scrollBars: desktop.ScrollBarsAlways}.listBar()
			laneMin := frameRight - int(bar.Width())
			thumbMin, thumbMax := laneMin+int(bar.TrackPadding), frameRight-int(bar.TrackPadding)

			// The first slot is the first line the Recall button is drawn
			// on: the column heads leave the buttons' columns empty.
			recallMin, recallMax := laneMin-4-76+10, laneMin-4-10
			lineDrawn := func(img *image.RGBA, y, x0, x1 int) bool {
				for x := x0; x < x1; x++ {
					if drawn(img, x, y) {
						return true
					}
				}
				return false
			}
			// The tab strip ends on the separator, the one line drawn across
			// the page's whole width; the tab's content is under it.
			strip := -1
			for y := 0; y < size.Y && strip < 0; y++ {
				across := true
				for x := int(Padding) + 20; x < frameRight-20 && across; x++ {
					across = drawn(rest, x, y)
				}
				if across {
					strip = y
				}
			}
			if strip < 0 {
				t.Fatal("at rest no line is drawn across the page's width under the tab strip")
			}
			slot := -1
			for y := strip + 1; y < size.Y && slot < 0; y++ {
				if lineDrawn(rest, y, recallMin, recallMax) {
					slot = y
				}
			}
			if slot < 0 {
				t.Fatalf("at rest no Recall button is drawn in x %d–%d", recallMin, recallMax)
			}
			// The column heads are the last run of drawn lines above it.
			headsBot := slot - 1
			for headsBot > strip && !lineDrawn(rest, headsBot, 0, recallMin) {
				headsBot--
			}
			headsTop := headsBot
			for headsTop > strip+1 && lineDrawn(rest, headsTop-1, 0, recallMin) {
				headsTop--
			}
			headsBot++

			// At rest the thumb's top is the lane's top plus the track's
			// padding.
			thumbTop := -1
			for y := strip + 1; y < size.Y && thumbTop < 0; y++ {
				if lineDrawn(rest, y, thumbMin, thumbMax) {
					thumbTop = y
				}
			}
			if thumbTop < 0 {
				t.Fatalf("at Always a resting frame drew no thumb in x %d–%d", thumbMin, thumbMax)
			}
			if thumbTop < headsBot {
				t.Errorf("at rest the thumb is drawn from y %d, beside the column heads at y %d–%d; the lane starts under them", thumbTop, headsTop, headsBot)
			}

			mid := f32.Pt(float32(size.X)/2, float32(size.Y-40))
			r.Queue(
				pointer.Event{Kind: pointer.Move, Position: mid, Source: pointer.Mouse},
				pointer.Event{Kind: pointer.Scroll, Position: mid, Source: pointer.Mouse, Scroll: f32.Pt(0, 30)},
			)
			frame()
			frame()
			scrolled := shot()

			moved := false
			for y := slot; y < size.Y && !moved; y++ {
				for x := 0; x < laneMin; x++ {
					if rest.RGBAAt(x, y) != scrolled.RGBAAt(x, y) {
						moved = true
						break
					}
				}
			}
			if !moved {
				t.Fatalf("a scroll at %v moved none of the slots", mid)
			}
			for y := 0; y < headsBot; y++ {
				for x := 0; x < size.X; x++ {
					if rest.RGBAAt(x, y) != scrolled.RGBAAt(x, y) {
						t.Fatalf("after a scroll (%d, %d), at or above the column heads at y %d–%d, is drawn differently; the heads are drawn at the same place", x, y, headsTop, headsBot)
					}
				}
			}
			for y := 0; y < headsBot; y++ {
				if lineDrawn(scrolled, y, laneMin, frameRight) && !lineDrawn(rest, y, laneMin, frameRight) {
					t.Fatalf("after a scroll the lane x %d–%d is drawn at y %d, beside the column heads; it starts under them", laneMin, frameRight, y)
				}
			}
		})
	}
}
