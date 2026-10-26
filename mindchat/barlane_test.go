package main

// The rail's and the provider well's rows against the reader's setting: at
// Always the bar takes a lane of its own and the rows stop short of it; at
// When scrolling the bar lies over the rows' trailing edge and the rows run
// the whole width. The rows overflow, because a lane exists only for a bar
// that is drawn.

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// overflowChats is more conversations than the rail's frame shows, so the
// rail's bar is drawn.
func overflowChats() ChatList {
	chats := ChatList{}
	for i := range 60 {
		chats = append(chats, fmt.Sprintf("Chat %02d.jsonl", i))
	}
	return chats
}

// rowsEndOutside is the x one past the last pixel in region that differs
// from fill and is not in skip: where the rows' drawing ends.
func rowsEndOutside(img *image.RGBA, region, skip image.Rectangle, fill color.NRGBA) int {
	end := 0
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if image.Pt(x, y).In(skip) {
				continue
			}
			if p := img.RGBAAt(x, y); p.R != fill.R || p.G != fill.G || p.B != fill.B {
				end = max(end, x+1)
			}
		}
	}
	return end
}

// TestTheRailsRowsStopShortOfAStandingBarsLane lays the rail's
// conversations out with the open one drawn with its pill, which runs to the
// row's trailing end less the pill's own inset, once per setting. At Always
// the rows end before the bar's lane, a lane's width further in than where
// the bar fades.
func TestTheRailsRowsStopShortOfAStandingBarsLane(t *testing.T) {
	for _, sc := range schemes {
		t.Run(sc.name, func(t *testing.T) {
			fill := color.NRGBA{R: 0x40, G: 0x10, B: 0x70, A: 0xff}
			size := railKeyFrameSize
			render := func(bars desktop.ScrollBarSetting) *image.RGBA {
				th := railThemed(t, sc.c)
				th.bar = scrollBarStyle(th.bar, bars)
				rail := newRailState()
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					chats := overflowChats()
					return rail.layout(gtx, th, chats, chats[0], nil,
						func(layout.Context, mvu.Message) {})
				})
			}
			standing := scrollBarStyle(railThemed(t, sc.c).bar, desktop.ScrollBarsAlways)
			lane, pad := int(standing.Width()), int(standing.TrackPadding)
			laneMin := size.X - lane
			thumb := image.Rect(laneMin+pad, 0, size.X-pad, size.Y)
			whole := image.Rectangle{Max: size}

			at := rowsEndOutside(render(desktop.ScrollBarsAlways), whole, thumb, fill)
			still := rowsEndOutside(render(desktop.ScrollBarsWhenScrolling), whole, image.Rectangle{}, fill)
			if at == 0 || at > laneMin {
				t.Errorf("at Always the rows draw to x %d, want them to end before the %d px lane at x %d", at, lane, laneMin)
			}
			if still-at != lane {
				t.Errorf("at Always the rows end at x %d and at WhenScrolling at x %d, want them %d px apart, the lane's width",
					at, still, lane)
			}
		})
	}
}

// TestTheProviderWellsRowsStopShortOfAStandingBarsLane lays the provider
// well out with the first provider selected, whose fill runs to the row's
// trailing end less its own inset, once per setting. At Always the rows end
// before the bar's lane, a lane's width further in than where the bar fades.
func TestTheProviderWellsRowsStopShortOfAStandingBarsLane(t *testing.T) {
	for _, sc := range schemes {
		t.Run(sc.name, func(t *testing.T) {
			fading := wellThemed(sc.c)
			fading.bar = scrollbar.FromTokens(sc.c, sc.c.WindowBackground)
			standing := fading
			standing.bar = scrollBarStyle(fading.bar, desktop.ScrollBarsAlways)
			// More providers than the well shows, so the well's bar is drawn.
			s := SettingsState{Selected: 0}
			for i := range 40 {
				s.Draft = append(s.Draft, Provider{Name: fmt.Sprintf("Provider %02d", i)})
			}
			fill := fading.palette.Pane
			region := wellRowsRegion(t, fading)
			lane, pad := int(standing.bar.Width()), int(standing.bar.TrackPadding)
			laneMin := region.Max.X - lane
			thumb := image.Rect(laneMin+pad, region.Min.Y, region.Max.X-pad, region.Max.Y)

			// The rows take a click each, which the empty well's scene has
			// no need of.
			scene := func(th settingsThemed) layout.Widget {
				clicks := map[int]*widget.Clickable{}
				for i := range s.Draft {
					clicks[i] = new(widget.Clickable)
				}
				var addClick, removeClick widget.Clickable
				rows := list.NewState()
				return func(gtx layout.Context) layout.Dimensions {
					paint.FillShape(gtx.Ops, sc.c.WindowBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
					return providerColumn(gtx, th, s, clicks, &addClick, &removeClick, rows)
				}
			}
			at := rowsEndOutside(golden.Capture(t, wellSize, scene(standing)), region, thumb, fill)
			still := rowsEndOutside(golden.Capture(t, wellSize, scene(fading)), region, image.Rectangle{}, fill)
			if at == 0 || at > laneMin {
				t.Errorf("at Always the rows draw to x %d, want them to end before the %d px lane at x %d", at, lane, laneMin)
			}
			if still-at != lane {
				t.Errorf("at Always the rows end at x %d and at WhenScrolling at x %d, want them %d px apart, the lane's width",
					at, still, lane)
			}
		})
	}
}
