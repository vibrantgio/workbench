package main

import (
	"fmt"
	"image"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheDeviceGridEndsInASeamAgainstItsFooter draws the Device tab in both
// schemes in frames shorter than its rows, so a row is cut at the grid's
// foot.
//
// The grid's foot against the footer is a seam: the grid draws the hairline
// in its last pixel row, the measured seam colour across the whole window
// width, and the row it cuts is clipped above the hairline, so the line is
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
				seam := tc.c.Seam
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
