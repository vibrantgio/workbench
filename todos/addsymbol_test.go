package main

import (
	"image"
	"image/color"
	"testing"
)

// TestTheAddSymbolIsDrawnInTheButtonsForeground renders the resting window in
// both schemes and reads the add button's corner of it. Under the Icon entry
// an icon in a control is in that control's foreground: the add symbol's
// solid pixels are the label colour, and no pixel there is near the accent.
func TestTheAddSymbolIsDrawnInTheButtonsForeground(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			th := staticThemed(tc.c)
			img := renderWindow(t, tc.c, "", titleBandDp)
			corner := int(fabSize + 2*Padding)
			r := image.Rect(windowSize.X-corner, windowSize.Y-corner, windowSize.X, windowSize.Y)
			drawn, label, accent := 0, 0, 0
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					c := pixelAt(img, image.Pt(x, y))
					if c != th.palette.Backdrop {
						drawn++
					}
					if c == th.palette.Label {
						label++
					}
					if near(c, tc.c.ControlAccent) {
						accent++
					}
				}
			}
			if drawn == 0 {
				t.Fatalf("no add symbol is drawn inside %v", r)
			}
			if label == 0 {
				t.Errorf("no pixel of the add symbol is the label colour %v", th.palette.Label)
			}
			if accent != 0 {
				t.Errorf("%d pixels of the add symbol are near the accent %v", accent, tc.c.ControlAccent)
			}
		})
	}
}

// near reports whether two opaque colours are within a small distance of each
// other, summed over the three channels.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R)+d(a.G, b.G)+d(a.B, b.B) <= 24
}
