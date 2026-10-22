package main

// The window's own scroll bars, read at the two settings the platform
// answers with. The bar a reader sees is built from the colours alone and
// then spent against the setting the window read when it was composed; what
// that spending comes to is a bar that stands on a resting frame or one that
// is not there at all, which is what these pixels are.

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheWindowSpendsTheScrollBarSetting lays the window's bar out on a
// frame nothing has scrolled and nothing is pointing at, once per setting.
//
// With the reader's "always" setting the thumb is there: the bar stands for
// as long as the content overflows. With either of the other two it is the
// platform's overlay bar, which a resting frame holds nothing of.
func TestTheWindowSpendsTheScrollBarSetting(t *testing.T) {
	c := tokens.PlatformLight
	// A tab's column rides the content plane, which is what its sections
	// fill with.
	rides := c.ControlBackground
	size := image.Pt(24, 400)
	// The pixels the bar leaves on the fill it rides, on a frame with no
	// scroll and no pointer behind it.
	restingPixels := func(bars desktop.ScrollBarSetting) int {
		style := scrollBarStyle(scrollbar.FromTokens(c, rides), bars)
		state := scrollbar.NewState()
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
			gtx.Now = time.Unix(1700000000, 0)
			paint.FillShape(gtx.Ops, rides, clip.Rect{Max: gtx.Constraints.Max}.Op())
			style.Layout(gtx, state, layout.Vertical, 0.35, 0.65)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		n := 0
		for y := range size.Y {
			for x := range size.X {
				if got := img.RGBAAt(x, y); got.R != rides.R || got.G != rides.G || got.B != rides.B {
					n++
				}
			}
		}
		return n
	}

	if n := restingPixels(desktop.ScrollBarsAlways); n == 0 {
		t.Error("with scroll bars always, a resting frame drew no thumb")
	}
	for _, bars := range []desktop.ScrollBarSetting{desktop.ScrollBarsAutomatic, desktop.ScrollBarsWhenScrolling} {
		if n := restingPixels(bars); n != 0 {
			t.Errorf("at %v, a resting frame drew %d thumb pixels, want none", bars, n)
		}
	}
}

// TestTheDocsColumnStandsABar renders the Docs tab's document column at
// the two settings and reads the bar off the pixels, in both schemes. The
// guide is the longest content this window holds, so the column that shows
// it overflows and has a bar to draw.
//
// The region that scrolls is the whole column. With the reader's "always"
// setting the thumb is on a resting frame in the lane at the column's
// trailing edge, the lane a tab of rows' bar is in, and the text stops
// short of the lane: the document's blocks are centred in the column less
// the lane. With "when scrolling" the frame is the frame the Docs golden
// holds: nothing of the bar is on it, and the blocks are centred in the
// whole column.
//
// The document's blocks are capped at the reading measure: the fenced
// block, the widest thing on the page, is the measure wide and starts half
// the room left beside it in, each within a pixel.
func TestTheDocsColumnStandsABar(t *testing.T) {
	typo := tokens.DefaultTypography
	shaper := typo.DeterministicShaper()
	for _, sc := range []struct {
		name string
		c    tokens.PlatformColors
	}{{"light", tokens.PlatformLight}, {"dark", tokens.PlatformDark}} {
		t.Run(sc.name, func(t *testing.T) {
			c := sc.c
			style := docsMarkdownStyle(c, typo)
			bg := c.ControlBackground

			// At PxPerDp 1 a standing bar's lane is the trailing Width() dp
			// of the frame, and its thumb is inside the lane's padding.
			standing := columnBar(c, desktop.ScrollBarsAlways)
			laneW, pad := int(standing.Width()), int(standing.TrackPadding)
			laneMin := docsFrameW - laneW
			thumb := image.Rect(laneMin+pad, 0, docsFrameW-pad, docsFrameH)

			// The bar of a tab of rows, read off the same frame size.
			rows := func(bars desktop.ScrollBarSetting) *image.RGBA {
				items := make([]layout.Widget, 60)
				for i := range items {
					items[i] = func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 40)}
					}
				}
				return golden.Capture(t, docsFrameSize, scene(scrollingColumn(list.NewState(), c, bars, items), bg))
			}
			barStart, ok := firstDiffX(rows(desktop.ScrollBarsAlways), rows(desktop.ScrollBarsWhenScrolling))
			if !ok || barStart < laneMin {
				t.Fatalf("a tab of rows' bar starts at x=%d (found %v), want it in the lane from x=%d", barStart, ok, laneMin)
			}

			frame := func(bars desktop.ScrollBarSetting) *image.RGBA {
				doc := markdown.NewDocument(markdown.Parse(guideFixture(t)))
				bar := columnBar(c, bars)
				return golden.Capture(t, docsFrameSize, scene(func(gtx layout.Context) layout.Dimensions {
					return drawGuideDoc(gtx, doc, shaper, style, bar)
				}, bg))
			}
			drawn := func(img *image.RGBA, x, y int) bool {
				p := img.RGBAAt(x, y)
				return p.R != bg.R || p.G != bg.G || p.B != bg.B
			}
			// widest is the widest run of drawn pixels left of the lane,
			// which is the fenced block's fill.
			widest := func(img *image.RGBA) (lead, width int) {
				for y := range docsFrameH {
					l, r := -1, -1
					for x := range laneMin {
						if drawn(img, x, y) {
							if l < 0 {
								l = x
							}
							r = x
						}
					}
					if l >= 0 && r-l+1 > width {
						lead, width = l, r-l+1
					}
				}
				return lead, width
			}

			always := frame(desktop.ScrollBarsAlways)
			thumbPixels, textEnd := 0, 0
			for y := range docsFrameH {
				for x := range docsFrameW {
					if !drawn(always, x, y) {
						continue
					}
					if image.Pt(x, y).In(thumb) {
						thumbPixels++
						continue
					}
					textEnd = max(textEnd, x+1)
				}
			}
			if thumbPixels == 0 {
				t.Errorf("at Always the Docs column drew no thumb in the lane x %d–%d, where a tab of rows has its bar", thumb.Min.X, thumb.Max.X)
			}
			if textEnd > laneMin {
				t.Errorf("at Always the Docs column's text draws to x %d, want it to end before the %d px lane at x %d", textEnd, laneW, laneMin)
			}
			for _, tc := range []struct {
				name   string
				img    *image.RGBA
				column int
			}{
				{"Always", always, docsFrameW - laneW},
				{"WhenScrolling", frame(desktop.ScrollBarsWhenScrolling), docsFrameW},
			} {
				lead, width := widest(tc.img)
				wantLead := (tc.column - docsMeasureDp) / 2
				if absInt(lead-wantLead) > 1 || absInt(width-docsMeasureDp) > 1 {
					t.Errorf("at %s the widest block spans x=%d..%d (%d wide), want it to start at %d and be %d wide, centred in the %d px the text is given",
						tc.name, lead, lead+width, width, wantLead, docsMeasureDp, tc.column)
				}
			}
		})
	}
}

// TestTheTabsRowsStopShortOfAStandingBarsLane lays a tab of rows out in a
// frame shorter than they are, once per setting, with each row painting the
// whole width it is given. At Always the rows end before the bar's lane
// and the thumb is drawn in it; where the bar fades the rows run the
// frame's full width under where the bar would be.
func TestTheTabsRowsStopShortOfAStandingBarsLane(t *testing.T) {
	c := tokens.PlatformLight
	bg := c.ControlBackground
	// An opaque fill no part of the bar draws, so a row pixel reads as itself.
	rowFill := color.NRGBA{R: 0x20, G: 0x80, B: 0x40, A: 0xff}
	size := image.Pt(400, 300)
	render := func(bars desktop.ScrollBarSetting) *image.RGBA {
		items := make([]layout.Widget, 40)
		for i := range items {
			items[i] = func(gtx layout.Context) layout.Dimensions {
				// A row paints its top half, so the row's own width is in
				// the pixels and the bar's thumb is not covered by it.
				w := gtx.Constraints.Max.X
				paint.FillShape(gtx.Ops, rowFill, clip.Rect{Max: image.Pt(w, 10)}.Op())
				return layout.Dimensions{Size: image.Pt(w, 20)}
			}
		}
		return golden.Capture(t, size, scene(scrollingColumn(list.NewState(), c, bars, items), bg))
	}
	rowsEnd := func(img *image.RGBA) int {
		end := 0
		for y := range size.Y {
			for x := range size.X {
				p := img.RGBAAt(x, y)
				if p.R == rowFill.R && p.G == rowFill.G && p.B == rowFill.B {
					end = max(end, x+1)
				}
			}
		}
		return end
	}
	laneW := int(columnBar(c, desktop.ScrollBarsAlways).Width())
	laneMin := size.X - laneW
	if got := rowsEnd(render(desktop.ScrollBarsAlways)); got != laneMin {
		t.Errorf("at Always the rows end at x %d, want them to stop short of the %d px lane at x %d", got, laneW, laneMin)
	}
	if got := rowsEnd(render(desktop.ScrollBarsWhenScrolling)); got != size.X {
		t.Errorf("at WhenScrolling the rows end at x %d, want the frame's full width %d", got, size.X)
	}
}

// firstDiffX is the smallest x at which two frames differ.
func firstDiffX(a, b *image.RGBA) (int, bool) {
	r := a.Bounds()
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return x, true
			}
		}
	}
	return 0, false
}
