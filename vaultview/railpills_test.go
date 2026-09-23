package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
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

// treeTabFrame lays the folder rail out against a real router and answers
// where the keyboard is after each frame. The control that puts the panel
// away stands last in the rail's own reading order, so it is what Tab must
// reach once the rows let go.
type treeTabFrame struct {
	v      *treeView
	model  Model
	tok    themeTokens
	router *input.Router

	onRail  bool
	onAfter bool
}

func newTreeTabFrame() *treeTabFrame {
	m := treeModel()
	m.Folds = map[string]bool{}
	return &treeTabFrame{
		v:      &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }},
		model:  m,
		tok:    goldenTokens(),
		router: new(input.Router),
	}
}

func (f *treeTabFrame) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(treeWidthDp, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      f.router.Source(),
	}
	f.v.layout(gtx, f.model, f.tok, nil)
	f.onRail = gtx.Focused(f.v.list.Focus())
	f.onAfter = gtx.Focused(&f.v.hideClick)
	f.router.Frame(ops)
}

func (f *treeTabFrame) tab() {
	f.router.MoveFocus(key.FocusForward)
	f.frame()
}

// treeTabStops is how many forward focus moves the rail is given to reach the
// rows. The bound is loose so a focusable added in front of them fails the
// assertion rather than this loop.
const treeTabStops = 20

// TestTabLeavesTheRailForTheNextFocusable drives forward focus moves through
// a real input.Router over the folder rail.
//
// A list is one focusable wherever it stands: the rail takes the keys on the
// list's one tag, and the very next move hands them to the control after it,
// as the platform's sidebar does. A focus filter per row — which is what a
// widget.Clickable registers — would instead walk Tab through the rows one by
// one, and the arrows would go dead the moment it did.
func TestTabLeavesTheRailForTheNextFocusable(t *testing.T) {
	f := newTreeTabFrame()
	f.frame() // registers the tags

	for moves := 0; !f.onRail; moves++ {
		if moves == treeTabStops {
			t.Fatalf("no forward focus move inside %d put the keyboard on the rail", treeTabStops)
		}
		f.tab()
	}

	f.tab()
	if f.onRail {
		t.Error("one move on from the rail the keys are still on it: the rail is more than one focus target, so Tab walks its rows")
	}
	if !f.onAfter {
		t.Error("the control after the rows did not take the keys: the move landed on a row")
	}
}

// TestTheRowsArrowsWalkAfterTabLeaves reads the other half: the rows are
// walked by the arrows and not by Tab, so a rail the keys have come back to
// still answers them.
func TestTheRowsArrowsWalkAfterTabLeaves(t *testing.T) {
	f := newTreeTabFrame()
	f.frame()
	for !f.onRail {
		f.tab()
	}
	f.tab() // away
	f.router.MoveFocus(key.FocusBackward)
	f.frame()
	if !f.onRail {
		t.Fatal("a backward move did not bring the keys back to the rail")
	}
	f.router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	f.frame()
	if got := f.v.list.Selected(); got != 0 {
		t.Errorf("Down put the selection on row %d, want the first row: the arrows no longer walk the rail", got)
	}
}
