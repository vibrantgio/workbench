package main

import (
	"image"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheListTabsScrollAsThePagesColumn draws the window on the Presets tab
// and on the Device tab at Always, in both schemes, in a frame shorter than
// either tab's rows, on a frame nothing has scrolled.
//
// The region that scrolls is the page's column, which runs to the window's
// edges with the page's Padding inside it: the thumb is drawn against the
// window's trailing edge, inside the lane's padding only, and the Presets
// rows reach the window's bottom edge, so the band of the page's bottom
// Padding has slots drawn in it.
func TestTheListTabsScrollAsThePagesColumn(t *testing.T) {
	size := image.Pt(int(winW), 300)
	for _, tc := range schemes {
		for _, screen := range []string{"presets", "device"} {
			t.Run(tc.name+"-"+screen, func(t *testing.T) {
				m := goldenModel()
				m.Screen = screen
				m.HavePresets, m.HaveD = true, true
				w := windowFrameBars(t, tc.c, m, postMessage, desktop.ScrollBarsAlways)
				golden.Capture(t, size, w)
				img := golden.Capture(t, size, w)
				fill := pageFill(tc.c)
				drawn := func(x, y int) bool { return at(img, x, y) != fill }

				bar := themed{platform: tc.c, scrollBars: desktop.ScrollBarsAlways}.listBar()
				lane, track := int(bar.Width()), int(bar.TrackPadding)
				laneMin := size.X - lane

				// The thumb is the drawn run on the lane's middle column
				// below the tab strip; its columns are read on its middle
				// line.
				mid := laneMin + lane/2
				top, bot := -1, -1
				for y := size.Y / 4; y < size.Y; y++ {
					if drawn(mid, y) {
						if top < 0 {
							top = y
						}
						bot = y
					}
				}
				if top < 0 {
					t.Fatalf("at Always a resting frame drew no thumb at x %d", mid)
				}
				y := (top + bot) / 2
				right := mid
				for right+1 < size.X && drawn(right+1, y) {
					right++
				}
				if want := size.X - track - 1; right != want {
					t.Errorf("the thumb's last column is x %d; against the window's trailing edge, inside the lane's %d px padding, it is x %d", right, track, want)
				}

				if screen != "presets" {
					return
				}
				// The rows reach the window's bottom edge: the band the
				// page's bottom Padding took has slots drawn in it.
				pad := int(Padding)
				for y := size.Y - pad; y < size.Y; y++ {
					for x := 0; x < laneMin; x++ {
						if drawn(x, y) {
							return
						}
					}
				}
				t.Errorf("nothing is drawn in y %d–%d left of the lane; the rows reach the window's bottom edge", size.Y-pad, size.Y)
			})
		}
	}
}
