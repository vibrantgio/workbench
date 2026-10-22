package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
)

// TestTheTranscriptsRowsReachTheTrailingEdgeWhereTheBarFades lays the
// transcript out once per setting, narrower than the turns' band so that the
// band runs from the row's inset to the row's inset, with more of the user's
// turns than fit. Each turn's card is set against the band's trailing edge,
// which is where the row ends less the turn's inset. At Always the bar has a
// lane of its own and the cards end before it, a lane's width further in than
// at When scrolling, where the bar lies over the rows' trailing edge.
func TestTheTranscriptsRowsReachTheTrailingEdgeWhereTheBarFades(t *testing.T) {
	size := image.Pt(400, 320)
	chat := make([]msgRow, 8)
	for i := range chat {
		chat[i] = msgRow{Msg: Message{Role: RoleUser, Kind: KindTurn, Content: "How does a click reach the update?"}}
	}
	for _, sc := range schemes {
		t.Run(sc.name, func(t *testing.T) {
			fading := schemeThemed(t, sc.c)
			fading.bar = scrollbar.FromTokens(sc.c, sc.c.ControlBackground)
			render := func(bars desktop.ScrollBarSetting) *image.RGBA {
				th := fading
				th.bar = scrollBarStyle(fading.bar, bars)
				pane := ChatPane(th, chat, list.NewState(), func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				})
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
					gtx.Constraints = layout.Exact(size)
					return pane(gtx)
				})
			}
			standing := scrollBarStyle(fading.bar, desktop.ScrollBarsAlways)
			lane, pad := int(standing.Width()), int(standing.TrackPadding)
			laneMin := size.X - lane
			// The rows above the seam and the composer under it.
			region := image.Rect(0, 0, size.X, size.Y/2)
			thumb := image.Rect(laneMin+pad, 0, size.X-pad, size.Y)
			fill := fading.palette.Transcript

			at := rowsEndOutside(render(desktop.ScrollBarsAlways), region, thumb, fill)
			still := rowsEndOutside(render(desktop.ScrollBarsWhenScrolling), region, image.Rectangle{}, fill)
			if want := size.X - int(TurnInset); still != want {
				t.Errorf("at When scrolling the cards end at x %d, want the row's trailing edge less the turn's inset, x %d", still, want)
			}
			if at == 0 || at > laneMin {
				t.Errorf("at Always the cards end at x %d, want them to end before the %d px lane at x %d", at, lane, laneMin)
			}
			if still-at != lane {
				t.Errorf("at Always the cards end at x %d and at When scrolling at x %d, want them %d px apart, the lane's width",
					at, still, lane)
			}
		})
	}
}
