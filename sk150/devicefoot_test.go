package main

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheDeviceGridEndsInASeamAgainstItsFooter draws the Device tab in both
// schemes in frames shorter than its rows, so a row is cut at the grid's
// foot.
//
// The grid's foot against the footer is a seam: the grid draws the hairline
// in its last pixel row, the status bar's measured seam across the whole
// window width, and the row it cuts is clipped above the hairline, so the line is
// whole and nothing of the row is drawn below it before the footer's text.
func TestTheDeviceGridEndsInASeamAgainstItsFooter(t *testing.T) {
	for _, tc := range schemes {
		for _, h := range []int{300, 320} {
			t.Run(fmt.Sprintf("%s-%d", tc.name, h), func(t *testing.T) {
				size := image.Pt(int(winW), h)
				m := goldenModel()
				m.Screen = "device"
				m.HavePresets, m.HaveD = true, true
				w := windowFrameBars(t, tc.c, m, postMessage, desktop.ScrollBarsAlways)
				golden.Capture(t, size, w)
				img := golden.Capture(t, size, w)
				fill := pageFill(tc.c)
				seam := tc.c.StatusFooterSeam
				seam.A = 0xff

				whole := func(y int, col func(x int) bool) bool {
					for x := 0; x < size.X; x++ {
						if !col(x) {
							return false
						}
					}
					return true
				}
				// The foot is the lowest row drawn in the seam colour
				// across the whole width.
				foot := -1
				for y := size.Y - 1; y > size.Y/2; y-- {
					if isSeamRow(img, y, seam) {
						foot = y
						break
					}
				}
				if foot < 0 {
					t.Fatalf("no row in the lower half is drawn in the seam colour %v across the window's width; the grid draws the seam hairline at its foot", seam)
				}
				// A row is cut at the foot: the row above the hairline has
				// content drawn in it.
				if whole(foot-1, func(x int) bool { return at(img, x, foot-1) == fill }) {
					t.Fatalf("y %d, above the hairline at y %d, is all fill; the frame is meant to cut a row at the grid's foot", foot-1, foot)
				}
				// The row below the hairline is the footer's band: nothing of
				// the cut row is drawn in it.
				if !whole(foot+1, func(x int) bool { return at(img, x, foot+1) == fill }) {
					t.Errorf("y %d, under the hairline at y %d, has something drawn in it; the grid is clipped at its foot", foot+1, foot)
				}
				// The footer draws no line of its own: the hairline is one
				// pixel row, drawn once.
				if isSeamRow(img, foot-1, seam) || isSeamRow(img, foot+1, seam) {
					t.Errorf("the seam beside y %d is more than one row; it is drawn once, by the grid", foot)
				}
			})
		}
	}
}

// TestTheDeviceFooterIsDrawnAtTheStatusBarsMeasures draws the Device tab in
// both schemes and reads the footer's text off the frame against the
// platform's status bar, read off Finder's in the reference's "What the
// status bar measures": 27 px of fill under the 1 px seam to the window's
// bottom edge, the text's baseline 17 px under the fill's top, so its last
// row above the baseline is 17 rows under the seam, and the text centred
// on the column's middle, which is the window's middle in this one-column
// window.
func TestTheDeviceFooterIsDrawnAtTheStatusBarsMeasures(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			size := image.Pt(int(winW), 420)
			m := goldenModel()
			m.Screen = "device"
			m.HavePresets, m.HaveD = true, true
			m.Notice = ""
			w := windowFrameBars(t, tc.c, m, postMessage, desktop.ScrollBarsAlways)
			golden.Capture(t, size, w)
			img := golden.Capture(t, size, w)
			fill := pageFill(tc.c)
			seam := tc.c.StatusFooterSeam
			seam.A = 0xff

			foot := -1
			for y := size.Y - 1; y > size.Y/2; y-- {
				if isSeamRow(img, y, seam) {
					foot = y
					break
				}
			}
			if foot < 0 {
				t.Fatalf("no row in the lower half is drawn in the seam colour %v across the window's width", seam)
			}
			if got := size.Y - 1 - foot; got != 27 {
				t.Errorf("the footer is %d px of fill under the seam at y %d; the platform's status bar is 27", got, foot)
			}
			// The text's rows: the pixels off the fill under the seam.
			textRow := make([]int, size.Y)
			minX, maxX, peak := size.X, -1, 0
			for y := foot + 1; y < size.Y; y++ {
				for x := 0; x < size.X; x++ {
					if off(at(img, x, y), fill) > 48 {
						textRow[y]++
						minX, maxX = min(minX, x), max(maxX, x)
					}
				}
				peak = max(peak, textRow[y])
			}
			if maxX < 0 {
				t.Fatalf("nothing is drawn under the seam at y %d; the footer draws its text there", foot)
			}
			// The baseline: the last row holding a quarter of the peak row's
			// pixels; the descenders under it hold fewer. A pixel is the
			// text's when it is more than 48 levels off the fill, over a
			// third of the way to the text's colour, so the glyphs'
			// antialiased overshoot under the baseline is not counted.
			last := -1
			for y := foot + 1; y < size.Y; y++ {
				if textRow[y]*4 >= peak {
					last = y
				}
			}
			if got := last - foot; got != 17 {
				t.Errorf("the text's last row above its baseline is y %d, %d rows under the seam at y %d; the platform's status bar has 17", last, got, foot)
			}
			// Centred: the text's middle is within 3 px of the column's,
			// which spans the window. The footer centres the text's advance
			// box, so the drawn text's middle differs by the side bearings of
			// the first and last glyphs.
			if mid2, want2 := minX+maxX, size.X-1; mid2 < want2-6 || mid2 > want2+6 {
				t.Errorf("the text is drawn over x %d–%d, its middle at %.1f; it is centred on the column's middle, %.1f", minX, maxX, float64(mid2)/2, float64(want2)/2)
			}
		})
	}
}

// off is the largest difference between two colours on one channel.
func off(a, b color.NRGBA) int {
	d := func(p, q uint8) int { return max(int(p)-int(q), int(q)-int(p)) }
	return max(d(a.R, b.R), d(a.G, b.G), d(a.B, b.B))
}

// TestTheDeviceFooterHoldsTheLinksStatusOnly reads the Device footer's text:
// the link's baud rate and address, with no explanation after them. A footer
// of status text holds status only.
func TestTheDeviceFooterHoldsTheLinksStatusOnly(t *testing.T) {
	want := "Link: 9600 baud, Modbus address 1"
	if got := linkStatus(); got != want {
		t.Errorf("the Device footer's text is %q; it is the link's status only, %q", got, want)
	}
}
