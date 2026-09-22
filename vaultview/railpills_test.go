package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// railPillFrames renders the folder rail twice against one router: once as
// nothing has clicked into it, and once after a click on the open note's own
// row, which is what hands the rail the keyboard.
//
// The click lands on the row the model marks current, so the SAME row is
// filled in both frames and the only thing that moves between them is where
// the keyboard is.
func railPillFrames(t *testing.T, colors tokens.PlatformColors, bg color.NRGBA) (bare, held *image.RGBA) {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	w := renderTree(shaper, goldenModel(), colors, tokens.Spacing, goldenRadius,
		tokens.DefaultTypography, tokens.Comfortable, goldenLeading)
	r := new(input.Router)
	ops := new(op.Ops)
	drive := func() {
		ops.Reset()
		w(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(treeFrameSize),
			Ops:         ops,
			Source:      r.Source(),
		})
		r.Frame(ops)
	}
	capture := func() *image.RGBA {
		drive()
		return golden.Capture(t, treeFrameSize, scene(func(gtx layout.Context) layout.Dimensions {
			gtx.Source = r.Source()
			return w(gtx)
		}, bg))
	}
	drive()
	bare = capture()
	if bare == nil {
		return nil, nil
	}
	hit := f32.Pt(float32(treeWidthDp)/2, float32(railRowMidY(bare, colors)))
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: hit, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Touch},
	)
	drive()
	return bare, capture()
}

// railRowMidY finds the middle row of the filled pill in a rendered rail:
// the rows the grey pill covers, halved.
func railRowMidY(img *image.RGBA, colors tokens.PlatformColors) int {
	want := sidebar.SelectionFill(colors, true)
	lo, hi := -1, -1
	x := treeWidthDp / 2
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		c := img.RGBAAt(x, y)
		if c.R == want.R && c.G == want.G && c.B == want.B {
			if lo < 0 {
				lo = y
			}
			hi = y
		}
	}
	if lo < 0 {
		return -1
	}
	return (lo + hi) / 2
}

// TestTheRailDrawsThePlatformsTwoPills reads both pill states off a rendered
// rail in both appearances: the accent pill while the rail holds the
// keyboard, the grey one while it does not.
//
// Both are measured — the accent pill off voicememos-sidebar-{light,dark}.png
// and the grey one off finder-sidebar-unfocused-{light,dark}.png — and
// patterns/sidebar is where both are drawn from, so what this reads is that
// this window's own rail takes the pattern's two states rather than one.
func TestTheRailDrawsThePlatformsTwoPills(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			bare, held := railPillFrames(t, tc.colors, tc.bg)
			if bare == nil {
				return // headless unavailable; Capture called t.Skip
			}
			y := railRowMidY(bare, tc.colors)
			if y < 0 {
				t.Fatal("the rail drew no filled row to read")
			}
			x := treeWidthDp - int(sidebar.SelectionInset) - 4
			for _, c := range []struct {
				what string
				img  *image.RGBA
				want color.NRGBA
			}{
				{"while the rail does not hold the keyboard", bare, sidebar.SelectionFill(tc.colors, true)},
				{"while it does", held, sidebar.SelectionFill(tc.colors, false)},
			} {
				got := c.img.RGBAAt(x, y)
				if got.R != c.want.R || got.G != c.want.G || got.B != c.want.B {
					t.Errorf("the pill %s reads %v at (%d, %d), want the measured %v", c.what, got, x, y, c.want)
				}
			}
		})
	}
}
