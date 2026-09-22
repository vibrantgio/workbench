package main

import (
	"image/color"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// railFocusMoves is how many forward focus moves the window is given to land
// the keyboard on a rail row. The rail's rows stand near the head of the op
// tree, so it takes two; the bound is loose so a control added in front of
// them fails the assertion below rather than this loop.
const railFocusMoves = 12

// TestTheRailDrawsThePlatformsTwoPills reads both pill states off the
// composed window in both appearances: the open feed wears the grey pill
// while nothing has put the keyboard on the rail, and the accent pill once
// a forward focus move lands there.
//
// Both are measured — the accent pill off voicememos-sidebar-{light,dark}.png
// and the grey one off finder-sidebar-unfocused-{light,dark}.png — and
// patterns/sidebar draws both, so what this reads is that this window's own
// rail takes the pattern's two states rather than one.
func TestTheRailDrawsThePlatformsTwoPills(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			w := windowFrame(t, tc.c, tokens.Comfortable, settledModel())
			r := new(input.Router)
			ops := new(op.Ops)
			drive := func() {
				ops.Reset()
				w(layout.Context{
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(windowSize),
					Ops:         ops,
					Source:      r.Source(),
				})
				r.Frame(ops)
			}
			pill := func() color.NRGBA {
				drive()
				img := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
					gtx.Source = r.Source()
					return w(gtx)
				})
				if img == nil {
					return color.NRGBA{}
				}
				return at(img, atOpenFeed.X, atOpenFeed.Y)
			}
			drive()
			bare := pill()
			if (bare == color.NRGBA{}) {
				return // headless unavailable; Capture called t.Skip
			}
			if want := patsidebar.SelectionFill(tc.c, true); bare != want {
				t.Errorf("with the keyboard off the rail the open feed reads %v, want the measured grey pill %v", bare, want)
			}
			want := patsidebar.SelectionFill(tc.c, false)
			for range railFocusMoves {
				r.MoveFocus(key.FocusForward)
				if pill() == want {
					return
				}
			}
			t.Errorf("no forward focus move inside %d put the keyboard on the rail: the open feed never reached the accent pill %v", railFocusMoves, want)
		})
	}
}
