package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
)

// TestAnEmptyListDrawsItsEmptyState draws the whole window with no todo,
// once per setting, in both schemes, under the toolbar's band.
//
// The list is the window's one column, so the line is drawn centred both
// ways in the window's full frame, the strip included: where the bar fades
// that is the whole window, and where it stands it is the window less the
// bar's lane on the trailing side. The toolbar's band is left out of the
// reading; its add button stands at the band's trailing end, far from the
// centre.
//
// The glyph box is the painted glyphs' own and the line box is what is
// centred, so the two centres differ by a pixel across and a few down.
func TestAnEmptyListDrawsItsEmptyState(t *testing.T) {
	for _, tc := range windowSchemes {
		for _, bars := range []desktop.ScrollBarSetting{desktop.ScrollBarsWhenScrolling, desktop.ScrollBarsAlways} {
			t.Run(tc.name+"/"+bars.String(), func(t *testing.T) {
				th := staticThemed(tc.c)
				th.scrollBars = bars
				fill := th.palette.Backdrop
				paintBackdrop := backdrop.Widget(fill)
				page := view(th, Model{}, complist.NewState(), func() unit.Dp { return bandDp })
				img := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
					paintBackdrop(gtx)
					return page(gtx)
				})
				toolbar := toolbarBox(img, fill)
				box := image.Rectangle{}
				first := true
				for y := range windowSize.Y {
					for x := range windowSize.X {
						p := image.Pt(x, y)
						if p.In(toolbar) || pixelAt(img, p) == fill {
							continue
						}
						if first {
							box, first = image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))}, false
							continue
						}
						box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
					}
				}
				if box.Empty() {
					t.Fatal("the window with no todo is bare, want the line the empty state draws")
				}
				frameW := windowSize.X
				if bars == desktop.ScrollBarsAlways {
					frameW -= int(th.listBar().Width())
				}
				if got := box.Min.X + box.Max.X; abs(got-frameW) > 2 {
					t.Errorf("the line's glyphs centre at %.1f across, want the frame's %.1f",
						float64(got)/2, float64(frameW)/2)
				}
				if got := box.Min.Y + box.Max.Y; abs(got-windowSize.Y) > 4 {
					t.Errorf("the line's glyphs centre at %.1f down, want the window's %.1f",
						float64(got)/2, float64(windowSize.Y)/2)
				}
			})
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
