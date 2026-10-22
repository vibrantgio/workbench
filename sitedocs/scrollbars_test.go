package main

// The window's own scroll bars, read at the two settings the platform
// answers with. The bar a reader sees is built from the colours alone and
// then spent against the setting the window read when it was composed; what
// that spending comes to is a bar that stands on a resting frame or one that
// is not there at all, which is what these pixels are.

import (
	"image"
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
// The region that scrolls is the whole column, so with the reader's
// "always" setting the thumb is on a resting frame at the column's
// trailing edge: the same x as the bar of a tab of rows laid out in a frame
// of the same size, and nowhere else. With "when scrolling" the frame is
// the frame the Docs golden holds: nothing of the bar is on it.
//
// The document's blocks are capped at the reading measure and centred in
// the column: the fenced block, the widest thing on the page, starts at
// (column - measure)/2 and is the measure wide, each within a pixel.
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

			// At PxPerDp 1 a bar of a tab of rows is in the trailing
			// Width() dp of the frame.
			lane := docsFrameW - int(columnBar(c, desktop.ScrollBarsAlways).Width())

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
			if !ok || barStart < lane {
				t.Fatalf("a tab of rows' bar starts at x=%d (found %v), want it in the lane from x=%d", barStart, ok, lane)
			}

			frame := func(bars desktop.ScrollBarSetting) *image.RGBA {
				doc := markdown.NewDocument(markdown.Parse(guideFixture(t)))
				bar := columnBar(c, bars)
				return golden.Capture(t, docsFrameSize, scene(func(gtx layout.Context) layout.Dimensions {
					return drawGuideDoc(gtx, doc, shaper, style, bar)
				}, bg))
			}
			always, fading := frame(desktop.ScrollBarsAlways), frame(desktop.ScrollBarsWhenScrolling)
			x, ok := firstDiffX(always, fading)
			if !ok {
				t.Fatal("the Docs column renders identically with scroll bars always and when scrolling; it has no bar")
			}
			if x != barStart {
				t.Errorf("the Docs column's bar starts at x=%d, want %d, where a tab of rows has its bar", x, barStart)
			}

			// The widest run of non-background pixels left of the bar's
			// lane is the fenced block's fill: its start is the lead.
			lead, width := 0, 0
			for y := range docsFrameH {
				l, r := -1, -1
				for x := range lane {
					if p := fading.RGBAAt(x, y); p.R != bg.R || p.G != bg.G || p.B != bg.B {
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
			wantLead := (docsFrameW - docsMeasureDp) / 2
			if absInt(lead-wantLead) > 1 || absInt(width-docsMeasureDp) > 1 {
				t.Errorf("the widest block spans x=%d..%d (%d wide), want it to start at %d and be %d wide, centred in the column",
					lead, lead+width, width, wantLead, docsMeasureDp)
			}
		})
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
