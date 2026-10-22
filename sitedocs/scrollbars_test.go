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
// the two settings and reads the bar off the pixels. The guide is the
// longest content this window holds, so the column that shows it overflows
// and has a bar to draw.
//
// With the reader's "always" setting the thumb stands on a resting frame,
// and it stands in the column's trailing gutter — the trailing edge of the
// reading measure, which is where the document's own viewport ends — and
// nowhere else. With "when scrolling" the frame is the frame the Docs
// golden holds: nothing of the bar is on it, which is what keeps that
// golden byte-identical.
func TestTheDocsColumnStandsABar(t *testing.T) {
	c := tokens.PlatformLight
	typo := tokens.DefaultTypography
	shaper := typo.DeterministicShaper()
	style := docsMarkdownStyle(c, typo)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	// At PxPerDp 1 the column runs from the outer inset to the capped
	// reading measure, and the bar rides the last Width() dp of it.
	measureEnd := docsOuterInsetDp + docsMeasureDp
	gutterStart := measureEnd - int(columnBar(c, desktop.ScrollBarsAlways).Width())

	frame := func(bars desktop.ScrollBarSetting) *image.RGBA {
		doc := markdown.NewDocument(markdown.Parse(guideFixture(t)))
		bar := columnBar(c, bars)
		return golden.Capture(t, docsFrameSize, scene(func(gtx layout.Context) layout.Dimensions {
			return drawGuideDoc(gtx, doc, shaper, style, bar)
		}, bg))
	}

	always, fading := frame(desktop.ScrollBarsAlways), frame(desktop.ScrollBarsWhenScrolling)
	if golden.PixelDiff(always, fading) == 0 {
		t.Fatal("the Docs column renders identically with scroll bars always and when scrolling; it stands no bar")
	}
	outside, inside := 0, 0
	for y := range docsFrameH {
		for x := range docsFrameW {
			if always.RGBAAt(x, y) == fading.RGBAAt(x, y) {
				continue
			}
			if x >= gutterStart && x < measureEnd {
				inside++
			} else {
				outside++
			}
		}
	}
	if inside == 0 {
		t.Errorf("no thumb pixel between x=%d and x=%d; the standing bar is not in the column's trailing gutter", gutterStart, measureEnd)
	}
	if outside != 0 {
		t.Errorf("%d pixels outside the trailing gutter moved when the bar stood; asking for the bar moved the prose", outside)
	}
}
