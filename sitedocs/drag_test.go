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

// bandFill is the fill these drag tests hand the strip. They are about which
// presses the band claims and not about what it is painted in, so they name
// the same rung the app does rather than a colour of their own.
var bandFill = tokens.DefaultLight.SurfaceAt(tokens.Level1)

// TestDragUnderStripClaimsTheWindowDrag is AK6.2's guard: before this task
// neither this app nor the strip desktop.InsetTop pads carried a
// system.ActionMove anywhere, so the window could not be moved by its top
// edge at all — R6's second half, unmet while its first half (the
// full-size-content treatment and the inset) was already in place.
// dragUnderStrip now also declares desktop.DragTop over the same measured
// height InsetTop pads by, so a press in the strip resolves to the window
// move and a press in the page below it does not.
//
// The strip height is stated rather than read from desktop.TopInset,
// because a go test binary has no live macOS window behind it and that
// function reports 0 without one — the same substitution mvu/desktop's own
// TestDragTopClaimsTheStripAndNothingBelow makes for the same reason.
func TestDragUnderStripClaimsTheWindowDrag(t *testing.T) {
	const width, height, stripH = 400, 300, 32

	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w := dragUnderStrip(func() unit.Dp { return stripH }, bandFill, page)

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

	// With no live window behind a go test binary, desktop.LeadingInset —
	// which DragTop consults to leave the platform's own control buttons
	// their run — reports 0, so the strip's whole width is its empty run
	// here, the same headless case mvu/desktop's own
	// TestDragTopClaimsTheStripAndNothingBelow documents. (LeadingInset's
	// live exclusion of the buttons' own run is mvu/desktop's contract,
	// pinned there by TestDragTopLeavesTheButtonsTheirRun; this app adds no
	// arithmetic of its own around it, only the call.)
	for _, p := range []image.Point{{X: 0, Y: 0}, {X: width / 2, Y: stripH / 2}, {X: width - 1, Y: stripH - 1}} {
		if !moveAt(p.X, p.Y) {
			t.Errorf("no window-move action at %v; the strip above the page is the drag band", p)
		}
	}
	// Below the strip is the page dragUnderStrip's InsetTop pads down to,
	// which keeps its own presses.
	for _, p := range []image.Point{{X: width / 2, Y: stripH}, {X: width / 2, Y: 150}, {X: width / 2, Y: height - 1}} {
		if moveAt(p.X, p.Y) {
			t.Errorf("window-move action at %v; the page below the strip is the page's, not the band's", p)
		}
	}
}

// TestDragUnderStripIsANoOpWithNoStrip mirrors InsetTop's own no-strip case:
// a height of 0 — headless rendering, and every platform other than macOS —
// claims nothing and lays the page out at the window's own top.
func TestDragUnderStripIsANoOpWithNoStrip(t *testing.T) {
	const width, height = 400, 300

	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w := dragUnderStrip(func() unit.Dp { return 0 }, bandFill, page)

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(width, height)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)

	var r input.Router
	r.Frame(&ops)
	if _, ok := r.ActionAt(f32.Pt(float32(width/2), 0)); ok {
		t.Error("window-move action with no strip above the page; there is no band to claim")
	}
}
