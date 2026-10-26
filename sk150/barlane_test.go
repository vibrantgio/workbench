package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheListTabsDrawTheWindowsBar draws the window on the Presets tab and
// on the Device tab, in both schemes, in a frame shorter than either tab's
// rows, once per setting, on a frame nothing has scrolled.
//
// At Always the bar is drawn at rest in a lane at the column's trailing
// edge, which is the window's, the thumb inside the
// lane's padding, and the rows stop short of the lane: below the tab strip
// nothing but the thumb is drawn in the lane, apart from the seam at a
// grid's foot, which runs the window's whole width. Where the bar fades a resting
// frame draws no thumb, and the Presets tab's rows, whose buttons are drawn
// against their trailing end, end the page's Padding and the tab's 4 dp
// before the window's trailing edge: they run on by the lane's width.
func TestTheListTabsDrawTheWindowsBar(t *testing.T) {
	size := image.Pt(int(winW), 300)
	frameRight := size.X
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
					t.Fatalf("at Always a resting frame drew no thumb in the lane x %d–%d at the window's trailing edge", thumbMin, thumbMax)
				}
				seam := tc.c.StatusFooterSeam
				seam.A = 0xff
				for y := top; y < size.Y; y++ {
					if isSeamRow(standing, y, seam) {
						continue
					}
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
					inset := int(Padding) + 4
					reaches := false
					for y := top; y < size.Y && !reaches; y++ {
						for x := laneMin - inset; x < frameRight-inset; x++ {
							if drawn(fading, x, y) {
								reaches = true
								break
							}
						}
					}
					if !reaches {
						t.Errorf("at WhenScrolling the Presets rows draw nothing in x %d–%d; with no lane they run the lane's width further", laneMin-inset, frameRight-inset)
					}
				}
			})
		}
	}
}

// isSeamRow reports whether row y is drawn in the seam colour across the
// whole width of img: the hairline at a grid's foot.
func isSeamRow(img *image.RGBA, y int, seam color.NRGBA) bool {
	for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
		if at(img, x, y) != seam {
			return false
		}
	}
	return true
}
