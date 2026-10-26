package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheArticleColumnScrollsToTheArticlesEnd lays the article's body region
// out shorter than the article, scrolled to its end, once per setting, and
// holds it against the same article laid out tall enough to need no
// scrolling. The scrolled region shows exactly the foot of the tall one, so
// the last line is drawn and nothing under it is cut off. At Always the bar
// is drawn in a lane of its own and the prose ends before the lane. The tall
// article fits and draws no bar, so at Always it is laid out a lane narrower,
// at the width the scrolled region's rows have.
func TestTheArticleColumnScrollsToTheArticlesEnd(t *testing.T) {
	tok := themeTokens{
		col:    tokens.PlatformLight,
		typ:    tokens.DefaultTypography,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.Shaper(),
	}
	a := hardCodedArticles()[0]
	fill := tok.col.ControlBackground
	const width, tall, short = 320, 2000, 160

	render := func(bars desktop.ScrollBarSetting, w, h int, toEnd bool) *image.RGBA {
		scroll := list.NewState()
		size := image.Pt(w, h)
		frame := func(gtx layout.Context) layout.Dimensions {
			gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
			gtx.Constraints = layout.Exact(size)
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
			return drawBody(gtx, tok, detailBar(tok, bars), scroll, a, tok.typ.Body)
		}
		// The first frame opens the article at its first line; the second
		// is laid out after the region was asked for its end.
		img := golden.Capture(t, size, frame)
		if toEnd {
			scroll.ScrollToEnd(1)
			img = golden.Capture(t, size, frame)
		}
		return img
	}
	isFill := func(c color.RGBA) bool { return c.R == fill.R && c.G == fill.G && c.B == fill.B }
	lane := int(detailBar(tok, desktop.ScrollBarsAlways).Width())
	// The columns compared: everything left of where a bar may be drawn.
	cmpW := width - lane

	for _, bars := range []desktop.ScrollBarSetting{desktop.ScrollBarsWhenScrolling, desktop.ScrollBarsAlways} {
		t.Run(bars.String(), func(t *testing.T) {
			fullW := width
			if bars == desktop.ScrollBarsAlways {
				fullW = cmpW
			}
			full := render(bars, fullW, tall, false)
			// The tall render's last drawn row: the foot of the article.
			last := -1
			for y := range tall {
				for x := range cmpW {
					if !isFill(full.RGBAAt(x, y)) {
						last = y
					}
				}
			}
			if last < short {
				t.Fatalf("the article's last line is at y %d, want an article taller than the %d px region", last, short)
			}
			got := render(bars, width, short, true)
			// Find where in the tall render the scrolled region's rows start.
			// Glyph edges are rasterized a few levels apart at different
			// heights, so two pixels count as one within a small tolerance.
			at := -1
			for k := 1; k+short <= tall && at < 0; k++ {
				same := true
				for y := 0; y < short && same; y++ {
					for x := range cmpW {
						if !near(got.RGBAAt(x, y), full.RGBAAt(x, k+y)) {
							same = false
							break
						}
					}
				}
				if same {
					at = k
				}
			}
			if at < 0 {
				t.Fatal("the scrolled region shows no run of the article's rows, want its foot")
			}
			if at+short <= last {
				t.Errorf("the scrolled region ends at the article's row %d, want its last line at row %d drawn", at+short, last)
			}
			if bars != desktop.ScrollBarsAlways {
				return
			}
			barPixels := 0
			proseEnd := 0
			for y := range short {
				for x := range width {
					if isFill(got.RGBAAt(x, y)) {
						continue
					}
					if x >= cmpW {
						barPixels++
					} else {
						proseEnd = max(proseEnd, x+1)
					}
				}
			}
			if barPixels == 0 {
				t.Error("at Always no bar was drawn in the lane")
			}
			if proseEnd > cmpW-detailPadDp {
				t.Errorf("at Always the prose ends at x %d, want it to end before the %d px lane and the column's inset, at x %d at most",
					proseEnd, lane, cmpW-detailPadDp)
			}
		})
	}
}

// near reports whether two pixels are the same within the few levels glyph
// edges differ by when the same text is rasterized at different heights.
func near(a, b color.RGBA) bool {
	d := func(x, y uint8) int { return abs(int(x) - int(y)) }
	return d(a.R, b.R) <= 8 && d(a.G, b.G) <= 8 && d(a.B, b.B) <= 8
}
