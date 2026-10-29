package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/theme/tokens"
)

// TestTheToolbarClaimsTheWindowDrag guards the window's top-edge drag: a
// press in the toolbar's strip resolves to the window move, except on the
// search field, and a press in the page below the strip does not.
//
// A go test binary has no live macOS window, so desktop.LeadingInset reports
// 0 and the strip over the rail claims its whole width here. The live
// exclusion of the control buttons' run is mvu/desktop's contract.
func TestTheToolbarClaimsTheWindowDrag(t *testing.T) {
	const width, height = windowW, 300
	depth := toolbarDp
	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	field := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 36)}
	}
	w := withToolbar(toolbarDepth, tokens.PlatformLight, field, page)

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(width, height)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)

	var r input.Router
	r.Frame(&ops)
	moveAt := func(x, y int) bool {
		a, ok := r.ActionAt(f32.Pt(float32(x), float32(y)))
		return ok && a == system.ActionMove
	}

	for _, p := range []image.Point{
		{X: 0, Y: 0},
		{X: docsOutlineWidthDp / 2, Y: depth / 2},
		{X: docsOutlineWidthDp + 10, Y: depth / 2},
		{X: 800, Y: depth - 1},
		{X: width - 1, Y: depth / 2},
	} {
		if !moveAt(p.X, p.Y) {
			t.Errorf("no window-move action at %v; the toolbar's strip is the drag band", p)
		}
	}
	for _, p := range []image.Point{
		{X: width - 8 - toolbarSearchWDp/2, Y: depth / 2},
		{X: width / 2, Y: depth},
		{X: width / 2, Y: height - 1},
	} {
		if moveAt(p.X, p.Y) {
			t.Errorf("window-move action at %v; the search field and the page keep their own presses", p)
		}
	}
}
