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
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestThePagesRowsStopShortOfAStandingBarsLane lays the page out shorter
// than its sections, once per setting, with each section painting the
// whole width it is given, in a window narrower than the page's measure and
// in one wider. At Always the bar takes a lane of its own and the sections
// stop short of it: in the narrow window they end before the lane, and in
// the wide one the column is centred in the width less the lane. At When
// scrolling the bar lies over the trailing edge and takes no lane.
func TestThePagesRowsStopShortOfAStandingBarsLane(t *testing.T) {
	// An opaque fill no part of the bar draws, so a section pixel reads as
	// itself.
	fill := color.NRGBA{R: 0x20, G: 0x80, B: 0x40, A: 0xff}
	sections := make([]layout.Widget, 8)
	for i := range sections {
		sections[i] = func(gtx layout.Context) layout.Dimensions {
			w := gtx.Constraints.Max.X
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: image.Pt(w, 100)}.Op())
			return layout.Dimensions{Size: image.Pt(w, 100)}
		}
	}
	span := func(size image.Point, bars desktop.ScrollBarSetting) (lead, end int) {
		c := tokens.PlatformLight
		page := scrollingPage(sections, list.NewState(), c, bars)
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Metric = unit.Metric{PxPerDp: 1, PxPerSp: 1}
			gtx.Constraints = layout.Exact(size)
			return page(gtx)
		})
		lead = size.X
		want := color.RGBA{R: fill.R, G: fill.G, B: fill.B, A: 0xff}
		for y := range size.Y {
			for x := range size.X {
				if img.RGBAAt(x, y) == want {
					lead, end = min(lead, x), max(end, x+1)
				}
			}
		}
		return lead, end
	}
	c := tokens.PlatformLight
	lane := int(scrollbar.FromTokens(c, c.WindowBackground).Always().Width())

	narrow := image.Pt(800, 300)
	if lead, end := span(narrow, desktop.ScrollBarsAlways); lead != 0 || end != narrow.X-lane {
		t.Errorf("at Always in a narrow window the sections span x %d–%d, want 0–%d, short of the %d px lane",
			lead, end, narrow.X-lane, lane)
	}
	if lead, end := span(narrow, desktop.ScrollBarsWhenScrolling); lead != 0 || end != narrow.X {
		t.Errorf("at WhenScrolling in a narrow window the sections span x %d–%d, want 0–%d", lead, end, narrow.X)
	}

	wide := image.Pt(1400, 300)
	for _, tc := range []struct {
		bars  desktop.ScrollBarSetting
		width int
	}{{desktop.ScrollBarsAlways, wide.X - lane}, {desktop.ScrollBarsWhenScrolling, wide.X}} {
		wantLead := (tc.width - contentMaxWidthDp) / 2
		if lead, end := span(wide, tc.bars); lead != wantLead || end != wantLead+contentMaxWidthDp {
			t.Errorf("at %v in a wide window the column spans x %d–%d, want %d–%d, centred in the %d px the sections are given",
				tc.bars, lead, end, wantLead, wantLead+contentMaxWidthDp, tc.width)
		}
	}
}
