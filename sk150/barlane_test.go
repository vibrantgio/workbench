package main

import (
	"image"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheListTabsDrawTheWindowsBar draws the window on the Presets tab and
// on the Device tab, in both schemes, in a frame shorter than either tab's
// rows, once per setting, on a frame nothing has scrolled.
//
// At Always the bar is drawn at rest in a lane at the tab's trailing edge,
// which is the page's padding in from the window's, the thumb inside the
// lane's padding, and the rows stop short of the lane: below the tab strip
// nothing but the thumb is drawn in the lane. Where the bar fades a resting
// frame draws no thumb, and the Presets tab's rows, whose buttons are drawn
// against their trailing end, run on into where the lane was.
func TestTheListTabsDrawTheWindowsBar(t *testing.T) {
	size := image.Pt(int(winW), 300)
	frameRight := size.X - int(Padding)
	for _, tc := range schemes {
		for _, screen := range []string{"presets", "device"} {
			t.Run(tc.name+"-"+screen, func(t *testing.T) {
				m := goldenModel()
				m.Screen = screen
				m.HavePresets, m.HaveD = true, true
				render := func(bars desktop.ScrollBarSetting) *image.RGBA {
					w := windowFrameBars(t, tc.c, m, postMessage, bars)
					golden.Capture(t, size, w)
					return golden.Capture(t, size, w)
				}
				fill := pageFill(tc.c)
				drawn := func(img *image.RGBA, x, y int) bool { return at(img, x, y) != fill }
				bar := themed{platform: tc.c, scrollBars: desktop.ScrollBarsAlways}.listBar()
				lane, track := int(bar.Width()), int(bar.TrackPadding)
				laneMin := frameRight - lane
				thumbMin, thumbMax := laneMin+track, frameRight-track

				standing, fading := render(desktop.ScrollBarsAlways), render(desktop.ScrollBarsWhenScrolling)
				// The list's top is the first line the thumb differs on;
				// above it are the header and the tab strip, which span
				// the page's full width.
				top := -1
				for y := 0; y < size.Y && top < 0; y++ {
					for x := thumbMin; x < thumbMax; x++ {
						if standing.RGBAAt(x, y) != fading.RGBAAt(x, y) {
							top = y
							break
						}
					}
				}
				if top < 0 {
					t.Fatalf("at Always a resting frame drew no thumb in the lane x %d–%d at the tab's trailing edge", thumbMin, thumbMax)
				}
				for y := top; y < size.Y; y++ {
					for x := laneMin; x < frameRight; x++ {
						if x >= thumbMin && x < thumbMax {
							continue
						}
						if drawn(standing, x, y) {
							t.Fatalf("at Always (%d, %d) in the lane x %d–%d is drawn; the rows must stop short of the lane", x, y, laneMin, frameRight)
						}
					}
					for x := thumbMin; x < thumbMax; x++ {
						if drawn(fading, x, y) && screen == "device" {
							t.Fatalf("at WhenScrolling a resting frame drew (%d, %d) where the thumb stands at Always", x, y)
						}
					}
				}
				if screen == "presets" {
					reaches := false
					for y := top; y < size.Y && !reaches; y++ {
						for x := laneMin; x < frameRight; x++ {
							if drawn(fading, x, y) {
								reaches = true
								break
							}
						}
					}
					if !reaches {
						t.Errorf("at WhenScrolling the Presets rows draw nothing in x %d–%d; with no lane they run the tab's full width", laneMin, frameRight)
					}
				}
			})
		}
	}
}
