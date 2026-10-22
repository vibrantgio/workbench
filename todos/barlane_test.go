package main

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheListDrawsTheWindowsBar lays the list out in a frame shorter than
// its rows, once per setting, in both schemes, on a frame nothing has
// scrolled.
//
// At Always the bar is drawn at rest in a lane at the list's trailing edge,
// the thumb inside the lane's padding, and the rows stop short of the lane:
// what they draw ends a lane's width further in than where the bar fades.
// The pointer is over the first row in both frames, so its delete mark is
// drawn and is what the row draws furthest along.
// Where the bar fades a resting frame holds no thumb and the rows run the
// list's full width.
func TestTheListDrawsTheWindowsBar(t *testing.T) {
	size := image.Pt(int(winW), 240)
	var model Model
	for i := range 30 {
		model.List = append(model.List, Todo{Id: i, Text: fmt.Sprintf("Todo %d", i)})
	}
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.c.WindowBackground
			render := func(bars desktop.ScrollBarSetting) (*image.RGBA, themed) {
				th := staticThemed(tc.c)
				th.scrollBars = bars
				list := List(th.typ, rx.Of(th.components), th.palette, th.listBar(), complist.NewState(), model)
				draw := func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					return list(gtx)
				}
				rest := golden.Capture(t, size, draw)
				ys := rowMiddles(rest, th, 0)
				if len(ys) == 0 {
					t.Fatalf("found no row in the rest frame")
				}
				at := f32.Pt(float32(size.X/2), float32(ys[0]))
				return captureWithPointer(t, size, draw, &at), th
			}
			drawnIn := func(img *image.RGBA, skip image.Rectangle) (inSkip, end int) {
				for y := range size.Y {
					for x := range size.X {
						if pixelAt(img, image.Pt(x, y)) == fill {
							continue
						}
						if image.Pt(x, y).In(skip) {
							inSkip++
							continue
						}
						end = max(end, x+1)
					}
				}
				return inSkip, end
			}
			img, th := render(desktop.ScrollBarsAlways)
			bar := th.listBar()
			lane, track := int(bar.Width()), int(bar.TrackPadding)
			laneMin := size.X - lane
			thumb := image.Rect(laneMin+track, 0, size.X-track, size.Y)
			thumbPixels, standingEnd := drawnIn(img, thumb)
			if thumbPixels == 0 {
				t.Errorf("at Always a resting frame drew no thumb in the trailing lane x %d–%d", thumb.Min.X, thumb.Max.X)
			}

			img, _ = render(desktop.ScrollBarsWhenScrolling)
			thumbPixels, fadingEnd := drawnIn(img, thumb)
			if thumbPixels != 0 {
				t.Errorf("at WhenScrolling a resting frame drew %d pixels in the thumb's place, want no thumb", thumbPixels)
			}
			if standingEnd > laneMin || standingEnd != fadingEnd-lane {
				t.Errorf("at Always the rows draw to x %d, want them to stop short of the %d px lane at x %d, a lane's width in from x %d where the bar fades",
					standingEnd, lane, laneMin, fadingEnd)
			}
		})
	}
}
