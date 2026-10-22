package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheRailsRowsStopShortOfAStandingBarsLane lays the rail out shorter
// than its sections, once per setting, with each row painting the whole
// width it is given. At Always the bar takes a lane of its own and the rows
// end before it; at When scrolling the bar lies over the rows' trailing
// edge and the rows run the column's full width.
func TestTheRailsRowsStopShortOfAStandingBarsLane(t *testing.T) {
	// An opaque fill no heading or bar draws, so a row pixel reads as
	// itself.
	rowFill := color.NRGBA{R: 0x20, G: 0x80, B: 0x40, A: 0xff}
	rows := railRowSet{
		Count: func() int { return 12 },
		Row: func(gtx layout.Context, _ int) layout.Dimensions {
			w := gtx.Constraints.Max.X
			paint.FillShape(gtx.Ops, rowFill, clip.Rect{Max: image.Pt(w, 4)}.Op())
			return layout.Dimensions{Size: image.Pt(w, gtx.Dp(patsidebar.RowHeight))}
		},
		Drain: func(layout.Context) {},
	}
	sections := []railSection{{Title: "News", Rows: rows}, {Title: "Tech", Rows: rows}}
	size := image.Pt(int(patsidebar.ExpandedWidth), 200)
	render := func(bars desktop.ScrollBarSetting) *image.RGBA {
		headings := make([]gesture.Click, len(sections))
		open := map[int]bool{0: true, 1: true}
		scroll := list.NewState()
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
			gtx.Constraints = layout.Exact(size)
			return drawRailColumn(gtx, tokens.PlatformLight, tokens.DefaultTypography, bars,
				sections, headings, open, scroll, railKeyboard{Keys: newRailKeys()})
		})
	}
	rowsEnd := func(img *image.RGBA) int {
		end := 0
		for y := range size.Y {
			for x := range size.X {
				if img.RGBAAt(x, y) == (color.RGBA{R: rowFill.R, G: rowFill.G, B: rowFill.B, A: 0xff}) {
					end = max(end, x+1)
				}
			}
		}
		return end
	}
	c := tokens.PlatformLight
	lane := int(scrollbar.FromTokens(c, c.WindowBackground).Width())
	laneMin := size.X - lane
	if got := rowsEnd(render(desktop.ScrollBarsAlways)); got != laneMin {
		t.Errorf("at Always the rows end at x %d, want them to stop short of the %d px lane at x %d", got, lane, laneMin)
	}
	if got := rowsEnd(render(desktop.ScrollBarsWhenScrolling)); got != size.X {
		t.Errorf("at WhenScrolling the rows end at x %d, want the column's full width %d", got, size.X)
	}
}
