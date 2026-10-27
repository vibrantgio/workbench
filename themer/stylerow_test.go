package main

import (
	"image"
	stdcolor "image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// styleFill is the style column's selected fill for the set c.
func styleFill(p Palette, c tokens.PlatformColors) stdcolor.NRGBA {
	fill, _ := StyleSelection(p, c)
	return fill
}

// TestTheChosenStyleRowIsDrawnAsMeasured draws the style column with the kept
// style's row chosen and reads that row against the MEASURED values of
// System Settings' Preferred Languages list with the keyboard elsewhere
// (reference/macos/controls.md, "What a selected row in a list inside a
// grouped box measures"). The fill is #dfdfdf light and #5b5b5b dark, over
// the row's full height and the column's full width, with square ends. The
// name is in the row's own label colour over that fill: #e6e6e6 dark as the
// capture reads it, and #222222 light, where the capture reads #252525,
// 3 levels over the label colour's own arithmetic. A row under the pointer
// draws no fill.
func TestTheChosenStyleRowIsDrawnAsMeasured(t *testing.T) {
	size := image.Pt(240, 300)
	for _, tc := range []struct {
		name       string
		c          tokens.PlatformColors
		dark       bool
		fill, text stdcolor.NRGBA
	}{
		{"light", tokens.PlatformLight, false, stdcolor.NRGBA{R: 0xdf, G: 0xdf, B: 0xdf, A: 0xff}, stdcolor.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff}},
		{"dark", tokens.PlatformDark, true, stdcolor.NRGBA{R: 0x5b, G: 0x5b, B: 0x5b, A: 0xff}, stdcolor.NRGBA{R: 0xe6, G: 0xe6, B: 0xe6, A: 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			typ := tokens.DefaultTypography
			ty := TypeFrom(typ)
			ty.Shaper = typ.DeterministicShaper()
			m := goldenModel(t, tc.dark)
			pane := func(gtx layout.Context, sel *styleChooser) layout.Dimensions {
				gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
				gtx.Constraints = layout.Exact(size)
				paint.FillShape(gtx.Ops, p.Surface, clip.Rect{Max: size}.Op())
				return StylePane(p, tc.c, ty, m, tc.dark, sel, desktop.ScrollBarsWhenScrolling)(gtx)
			}
			capture := func(sel *styleChooser) *image.RGBA {
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions { return pane(gtx, sel) })
			}
			same := func(c stdcolor.RGBA, want stdcolor.NRGBA) bool {
				return c.R == want.R && c.G == want.G && c.B == want.B
			}

			rest := capture(newStyleChooser())
			// The chosen row: the lines whose leading pixel holds the fill.
			top, bottom := -1, -1
			for y := range size.Y {
				if same(rest.RGBAAt(0, y), tc.fill) {
					if top < 0 {
						top = y
					}
					bottom = y
				}
			}
			if top < 0 {
				t.Fatalf("no row is drawn in the measured fill %v", tc.fill)
			}
			if got, want := bottom-top+1, int(StyleRowH); got != want {
				t.Errorf("the chosen row's fill is %d px tall, want the row's %d px", got, want)
			}
			// Square ends, no inset: the four corners of the row read the fill.
			for _, pt := range []image.Point{{0, top}, {size.X - 1, top}, {0, bottom}, {size.X - 1, bottom}} {
				if got := rest.RGBAAt(pt.X, pt.Y); !same(got, tc.fill) {
					t.Errorf("the chosen row's corner %v reads %v, want the fill %v", pt, got, tc.fill)
				}
			}
			// The name: the pixel furthest from the fill is the text colour.
			far, dist := stdcolor.RGBA{}, -1
			for y := top; y <= bottom; y++ {
				for x := range size.X {
					c := rest.RGBAAt(x, y)
					d := abs(int(c.R) - int(tc.fill.R))
					if d > dist {
						far, dist = c, d
					}
				}
			}
			if !same(far, tc.text) {
				t.Errorf("the chosen row's name reads %v at its strongest, want the row's own text colour %v", far, tc.text)
			}

			// A row under the pointer: the pointer over the row two below the
			// chosen one changes no pixel.
			sel := newStyleChooser()
			r := new(input.Router)
			ops := new(op.Ops)
			frame := func() {
				ops.Reset()
				pane(layout.Context{Ops: ops, Source: r.Source()}, sel)
				r.Frame(ops)
			}
			frame()
			frame()
			at := f32.Pt(float32(size.X/2), float32(bottom+1+int(StyleRowH)+int(StyleRowH)/2))
			r.Queue(pointer.Event{Kind: pointer.Move, Position: at, Source: pointer.Mouse})
			frame()
			hovered := false
			for i := range sel.clicks {
				hovered = hovered || sel.clicks[i].Hovered()
			}
			if !hovered {
				t.Fatalf("the pointer at %v reaches no row", at)
			}
			if n := golden.PixelDiff(rest, capture(sel)); n != 0 {
				t.Errorf("a row under the pointer changes %d pixels, want none", n)
			}
		})
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
