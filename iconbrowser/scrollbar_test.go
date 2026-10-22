package main

// The icon grid's bar, read at the reader's two kinds of setting: a bar that
// stands at Always, with a lane of its own the cells stop short of, and a
// bar that fades otherwise, lying over the rows' trailing end and drawing
// nothing on a frame nothing has scrolled.

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// gridAt is one theme snapshot of the window at the given setting.
func gridAt(t *testing.T, c tokens.PlatformColors, bars desktop.ScrollBarSetting) themed {
	t.Helper()
	tok := staticThemed(t, c)
	tok.scrollBars = bars
	return tok
}

// TestTheGridDrawsTheWindowsBar lays the whole catalogue out in a frame far
// shorter than it, once per setting, on a frame nothing has scrolled. The
// frame is three cells and half a lane wide, so three columns fit where no
// lane is taken and two where one is.
//
// At Always the thumb is drawn at rest in the trailing lane, inside the
// lane's padding, and the cells end before the lane: two columns are drawn,
// since a third cell would end inside it. Where the
// bar fades the cells take the whole width: a third column is drawn.
func TestTheGridDrawsTheWindowsBar(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			standing := gridAt(t, tc.c, desktop.ScrollBarsAlways)
			bar := gridBar(standing)
			lane, pad := int(bar.Width()), int(bar.TrackPadding)
			cellW := int(CellW)
			size := image.Pt(3*cellW+lane/2, 300)
			fill := standing.palette.Backdrop
			render := func(tok themed) *image.RGBA {
				grid := Grid(tok, FilterIcons(""), "", list.NewState())
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					return grid(gtx)
				})
			}
			laneMin := size.X - lane
			thumb := image.Rect(laneMin+pad, 0, size.X-pad, size.Y)

			img := render(standing)
			thumbPixels, rowsEnd := 0, 0
			for y := range size.Y {
				for x := range size.X {
					if pixelAt(img, image.Pt(x, y)) == opaque(fill) {
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
			// A third cell would end at three cells' width, inside the lane,
			// so the rows end before the lane exactly when two columns are
			// drawn.
			if rowsEnd > 2*cellW || rowsEnd > laneMin {
				t.Errorf("at Always the cells draw to x %d, want two columns, ending by x %d, before the %d px lane at x %d",
					rowsEnd, 2*cellW, lane, laneMin)
			}

			img = render(gridAt(t, tc.c, desktop.ScrollBarsWhenScrolling))
			fadingEnd := 0
			for y := range size.Y {
				for x := range size.X {
					if pixelAt(img, image.Pt(x, y)) != opaque(fill) {
						fadingEnd = max(fadingEnd, x+1)
					}
				}
			}
			if fadingEnd <= 2*cellW {
				t.Errorf("at WhenScrolling the cells draw to x %d, want a third column past x %d", fadingEnd, 2*cellW)
			}
		})
	}
}

// TestTheGridsEmptyStateStandsOutsideAStandingBarsLane holds the grid's
// empty state against the setting: where the bar stands, the frame the line
// centres in is the one the cells would occupy, short of the lane, so the
// line's centre is half a lane further in than where the bar fades; up and
// down it does not move.
func TestTheGridsEmptyStateStandsOutsideAStandingBarsLane(t *testing.T) {
	size := image.Pt(600, 300)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			box := func(bars desktop.ScrollBarSetting) image.Rectangle {
				tok := gridAt(t, tc.c, bars)
				grid := Grid(tok, nil, noMatchQuery, list.NewState())
				img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, tok.palette.Backdrop, clip.Rect{Max: size}.Op())
					return grid(gtx)
				})
				b := gridTextBox(img, img.Bounds(), opaque(tok.palette.Backdrop))
				if b.Empty() {
					t.Fatalf("at %v the grid is bare, want the line the empty state draws", bars)
				}
				return b
			}
			fading, standing := box(desktop.ScrollBarsWhenScrolling), box(desktop.ScrollBarsAlways)
			lane := int(gridBar(gridAt(t, tc.c, desktop.ScrollBarsAlways)).Width())
			moved := (fading.Min.X + fading.Max.X) - (standing.Min.X + standing.Max.X)
			if d := moved - lane; d < -2 || d > 2 {
				t.Errorf("at Always the line's centre moves %.1f px in, want half the %d px lane, %.1f",
					float64(moved)/2, lane, float64(lane)/2)
			}
			if fading.Min.Y != standing.Min.Y || fading.Max.Y != standing.Max.Y {
				t.Errorf("at Always the line is drawn at y %d–%d, want %d–%d as where the bar fades",
					standing.Min.Y, standing.Max.Y, fading.Min.Y, fading.Max.Y)
			}
		})
	}
}

// opaque is a fill as pixelAt reports it.
func opaque(c color.NRGBA) color.NRGBA {
	c.A = 0xff
	return c
}
