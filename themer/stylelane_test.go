package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheStyleColumnsRowsReachTheTrailingEdgeWhereTheBarFades lays the
// style column out once per setting, shorter than its list of names, with
// the kept style's row marked: the mark runs to the row's trailing end less
// its own inset. At Always the bar has a lane of its own and the rows end
// before it, a lane's width further in than at When scrolling, where the bar
// lies over the rows' trailing edge and takes no lane.
func TestTheStyleColumnsRowsReachTheTrailingEdgeWhereTheBarFades(t *testing.T) {
	size := image.Pt(240, 300)
	for _, tc := range []struct {
		name string
		c    tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			typ := tokens.DefaultTypography
			ty := TypeFrom(typ)
			ty.Shaper = typ.DeterministicShaper()
			m := goldenModel(t, tc.dark)
			render := func(bars desktop.ScrollBarSetting) *image.RGBA {
				pane := StylePane(p, tc.c, ty, m, tc.dark, newStyleChooser(), bars)
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, p.Surface, clip.Rect{Max: size}.Op())
					return pane(gtx)
				})
			}
			standing := scrollBarStyle(scrollbar.FromTokens(tc.c, p.Surface), desktop.ScrollBarsAlways)
			lane, pad := int(standing.Width()), int(standing.TrackPadding)
			laneMin := size.X - lane
			thumb := image.Rect(laneMin+pad, 0, size.X-pad, size.Y)
			ends := func(img *image.RGBA, skip image.Rectangle) int {
				end := 0
				for y := range size.Y {
					for x := range size.X {
						if image.Pt(x, y).In(skip) {
							continue
						}
						if c := img.RGBAAt(x, y); c.R != p.Surface.R || c.G != p.Surface.G || c.B != p.Surface.B {
							end = max(end, x+1)
						}
					}
				}
				return end
			}

			at := ends(render(desktop.ScrollBarsAlways), thumb)
			still := ends(render(desktop.ScrollBarsWhenScrolling), image.Rectangle{})
			if at == 0 || at > laneMin {
				t.Errorf("at Always the rows draw to x %d, want them to end before the %d px lane at x %d", at, lane, laneMin)
			}
			if still-at != lane {
				t.Errorf("at Always the rows end at x %d and at When scrolling at x %d, want them %d px apart, the lane's width",
					at, still, lane)
			}
		})
	}
}
