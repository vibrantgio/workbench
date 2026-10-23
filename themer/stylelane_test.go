package main

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheStyleColumnHasNoPaddingAtEitherEnd lays the style column out once
// per setting, shorter than its list of names, with the kept style's row
// marked. A column of rows has no padding at either end: the marked row's
// hit area spans the column from its leading edge to the lane at Always and
// to the box's edge at When scrolling, where the bar's own area lies over the
// row's trailing end. The row's own inset keeps the marked row's fill
// StyleRowInset from each of the row's ends.
func TestTheStyleColumnHasNoPaddingAtEitherEnd(t *testing.T) {
	size := image.Pt(240, 300)
	for _, tc := range []struct {
		name string
		c    tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			typ := tokens.DefaultTypography
			ty := TypeFrom(typ)
			ty.Shaper = typ.DeterministicShaper()
			m := goldenModel(t, tc.dark)
			chosen := m.StyleAt(tc.dark)
			pane := func(gtx layout.Context, sel *styleChooser, bars desktop.ScrollBarSetting) layout.Dimensions {
				gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
				gtx.Constraints = layout.Exact(size)
				paint.FillShape(gtx.Ops, p.Surface, clip.Rect{Max: size}.Op())
				return StylePane(p, tc.c, ty, m, tc.dark, sel, bars)(gtx)
			}
			render := func(bars desktop.ScrollBarSetting) *image.RGBA {
				sel := newStyleChooser()
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					return pane(gtx, sel, bars)
				})
			}
			// hovers reports whether the pointer at pt lands in the marked
			// row's hit area, read off the row's own click handler.
			hovers := func(bars desktop.ScrollBarSetting, pt image.Point) bool {
				sel := newStyleChooser()
				r := new(input.Router)
				ops := new(op.Ops)
				frame := func() {
					ops.Reset()
					pane(layout.Context{Ops: ops, Source: r.Source()}, sel, bars)
					r.Frame(ops)
				}
				frame()
				frame()
				r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(pt.X)+0.5, float32(pt.Y)+0.5), Source: pointer.Mouse})
				frame()
				return sel.clicks[chosen].Hovered()
			}
			is := func(c color.RGBA, want color.NRGBA) bool { return c.R == want.R && c.G == want.G && c.B == want.B }
			// fill reads the marked row: the lines that hold the selection
			// colour, and across them the first and one past the last pixel
			// off the surface, its rounded ends' edges included. The thumb's
			// own run in the lane is left out.
			standing := scrollBarStyle(scrollbar.FromTokens(tc.c, p.Surface), desktop.ScrollBarsAlways)
			lane, pad := int(standing.Width()), int(standing.TrackPadding)
			laneMin := size.X - lane
			fill := func(img *image.RGBA) (lead, end, mid int) {
				lead, top, bottom := size.X, size.Y, 0
				for y := range size.Y {
					marked := false
					for x := range size.X {
						if is(img.RGBAAt(x, y), p.Selection) {
							marked = true
							break
						}
					}
					if !marked {
						continue
					}
					top, bottom = min(top, y), max(bottom, y)
					for x := range size.X {
						if x >= laneMin+pad && x < size.X-pad {
							continue
						}
						if !is(img.RGBAAt(x, y), p.Surface) {
							lead, end = min(lead, x), max(end, x+1)
						}
					}
				}
				return lead, end, (top + bottom) / 2
			}
			inset := int(StyleRowInset)

			// At When scrolling the bar lies over the rows' trailing edge, and
			// its own area takes the pointer there so that it can show under
			// the pointer. The row's hit area is probed up to that area, and
			// the fill's inset from the row's end shows where the row ends.
			over := scrollBarStyle(scrollbar.FromTokens(tc.c, p.Surface), desktop.ScrollBarsWhenScrolling)
			for _, set := range []struct {
				name   string
				bars   desktop.ScrollBarSetting
				rowEnd int
				reach  int
				where  string
			}{
				{"Always", desktop.ScrollBarsAlways, laneMin, laneMin, "the lane"},
				{"When scrolling", desktop.ScrollBarsWhenScrolling, size.X, size.X - int(over.Width()), "the bar's area at the box's edge"},
			} {
				lead, end, y := fill(render(set.bars))
				if end == 0 {
					t.Fatalf("at %s no row is marked", set.name)
				}
				if lead != inset || end != set.rowEnd-inset {
					t.Errorf("at %s the marked row's fill spans x %d-%d, want x %d-%d, one row inset from each of the row's ends at x 0 and x %d",
						set.name, lead, end, inset, set.rowEnd-inset, set.rowEnd)
				}
				if !hovers(set.bars, image.Pt(0, y)) {
					t.Errorf("at %s the marked row's hit area does not reach the column's leading edge at x 0", set.name)
				}
				if !hovers(set.bars, image.Pt(set.reach-1, y)) {
					t.Errorf("at %s the marked row's hit area does not reach %s at x %d", set.name, set.where, set.reach)
				}
			}
			if hovers(desktop.ScrollBarsAlways, image.Pt(laneMin, size.Y/2)) {
				t.Errorf("at Always the lane at x %d takes the pointer as a row", laneMin)
			}
		})
	}
}

// TestTheStyleBarIsDrawnAgainstTheBoxsTrailingEdge composes the window at
// Always and reads the standing thumb in the style box: the region that
// scrolls is the box's column with the box's padding inside it, so the thumb
// is drawn the track's own padding in from the box's trailing edge, never
// the box's padding further in.
func TestTheStyleBarIsDrawnAgainstTheBoxsTrailingEdge(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			img := pageBars(t, goldenModel(t, tc.dark), tc.c, image.Pt(windowW, windowH), newCodeState(), desktop.ScrollBarsAlways)
			standing := scrollBarStyle(scrollbar.FromTokens(tc.c, p.Surface), desktop.ScrollBarsAlways)
			lane, pad := int(standing.Width()), int(standing.TrackPadding)
			trail := int(Pad) + int(StyleW)
			// Every pixel the bar draws in the lane, read down the middle of
			// the box clear of its rounded corners.
			lead, end := trail, 0
			for y := judgingBoxTop() + int(BoxRadius); y < judgingBoxBottom()-int(BoxRadius); y++ {
				for x := trail - lane; x < trail; x++ {
					if c := img.RGBAAt(x, y); c.R != p.Surface.R || c.G != p.Surface.G || c.B != p.Surface.B {
						lead, end = min(lead, x), max(end, x+1)
					}
				}
			}
			if end == 0 {
				t.Fatalf("no bar is drawn in the style box at Always")
			}
			t.Logf("at Always the bar is drawn at x %d-%d against the box's trailing edge at x %d", lead, end, trail)
			if end != trail-pad {
				t.Errorf("at Always the bar is drawn at x %d-%d, want it to end at x %d, %d px in from the box's trailing edge at x %d",
					lead, end, trail-pad, pad, trail)
			}
		})
	}
}
