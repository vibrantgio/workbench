package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheRailsRowsDrawTheWindowsBar lays the rail's rows out in a frame
// shorter than they are, once per setting, on a frame nothing has scrolled,
// with the first row selected so that its pill shows where the row ends.
//
// At Always the bar is drawn at rest in the trailing lane [asideIndicator]
// gives, the thumb inside the lane's padding, and the rows end before the
// lane: nothing the rows draw reaches the lane. Where the bar fades the rows
// run under where the bar would be and a resting frame holds no thumb, so
// the lane holds row pixels or nothing.
func TestTheRailsRowsDrawTheWindowsBar(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	size := image.Pt(260, 120)
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := chromeSurface(tc.colors)
			render := func(bars desktop.ScrollBarSetting) *image.RGBA {
				tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
					sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper, scrollBars: bars}
				v := &treeView{list: list.NewState()}
				// A selected row wears the pill, which runs the row's
				// width less the selection inset: the row's trailing end
				// in pixels.
				v.list.Select(0)
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					return v.rows(gtx, goldenModel(), tok)
				})
			}
			standing := themeTokens{col: tc.colors, scrollBars: desktop.ScrollBarsAlways}
			bar := asideIndicator(standing)
			lane, pad := int(bar.Width()), int(bar.TrackPadding)
			laneMin := size.X - lane
			thumb := image.Rect(laneMin+pad, 0, size.X-pad, size.Y)

			img := render(desktop.ScrollBarsAlways)
			drawn := func(x, y int) bool {
				c := img.RGBAAt(x, y)
				return c.R != fill.R || c.G != fill.G || c.B != fill.B
			}
			thumbPixels, rowsEnd := 0, 0
			for y := range size.Y {
				for x := range size.X {
					if !drawn(x, y) {
						continue
					}
					if image.Pt(x, y).In(thumb) {
						thumbPixels++
						continue
					}
					rowsEnd = max(rowsEnd, x+1)
				}
			}
			if thumbPixels == 0 {
				t.Errorf("at Always a resting frame drew no thumb in the trailing lane x %d–%d", thumb.Min.X, thumb.Max.X)
			}
			if rowsEnd > laneMin {
				t.Errorf("at Always the rows draw to x %d, want them to end before the %d px lane at x %d",
					rowsEnd, lane, laneMin)
			}

			img = render(desktop.ScrollBarsWhenScrolling)
			fadingEnd := 0
			for y := range size.Y {
				for x := range size.X {
					if drawn(x, y) {
						fadingEnd = max(fadingEnd, x+1)
					}
				}
			}
			if fadingEnd <= laneMin {
				t.Errorf("at WhenScrolling the rows draw to x %d, want them under the lane past x %d", fadingEnd, laneMin)
			}
		})
	}
}
