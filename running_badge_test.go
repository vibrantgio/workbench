package main

import (
	"image"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/badge"
	"github.com/vibrantgio/components/golden"
)

// TestARunningAppsRowDrawsTheNeutralBadge renders an app's name row on the
// window's own plane in both schemes. While the app runs, the row draws the
// Neutral badge's fill with its white text on it, and no pixel is the
// accent. A stopped app's row draws no badge fill.
func TestARunningAppsRowDrawsTheNeutralBadge(t *testing.T) {
	for _, sc := range windowSchemes {
		t.Run(sc.name, func(t *testing.T) {
			tok := staticThemed(sc.colors)
			plane := windowPlane(sc.colors)
			fill, text, accent := badge.Fill(sc.colors, badge.Neutral), badge.Foreground(sc.colors), sc.colors.ControlAccent
			size := image.Pt(int(CellW), 40)
			count := func(status Status) (fills, texts, accents int) {
				row := nameRow(tok, Apps[0], status)
				img := golden.Capture(t, size, golden.Stack(backdrop.Widget(plane), func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Point{}
					return row(gtx)
				}))
				// The text is counted only inside the box the fill spans: on a
				// light plane the white text is the plane's own colour.
				var box image.Rectangle
				for y := 0; y < size.Y; y++ {
					for x := 0; x < size.X; x++ {
						c := pixelAt(img, image.Pt(x, y))
						if c == fill {
							fills++
							box = box.Union(image.Rect(x, y, x+1, y+1))
						}
						if channelGap(c, accent) == 0 {
							accents++
						}
					}
				}
				for y := box.Min.Y; y < box.Max.Y; y++ {
					for x := box.Min.X; x < box.Max.X; x++ {
						if pixelAt(img, image.Pt(x, y)) == text {
							texts++
						}
					}
				}
				return fills, texts, accents
			}
			if f, tx, a := count(Status{State: Running}); f == 0 || tx == 0 || a != 0 {
				t.Errorf("running: Neutral fill %v %d px, its text %v %d px, accent %v %d px; want fill and text above zero and no accent",
					fill, f, text, tx, accent, a)
			}
			if f, _, _ := count(Status{}); f != 0 {
				t.Errorf("stopped: Neutral fill %d px; want no badge", f)
			}
		})
	}
}
