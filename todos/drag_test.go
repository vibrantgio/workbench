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

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheStripClaimsTheWindowDrag is R6's second half for this window:
// the native drag leaves with the native strip, so a window that takes the
// full-size-content treatment and claims nothing back cannot be moved by its
// top edge at all. desktop.CapTop declares desktop.DragTop over the same
// measured height desktop.InsetTop pads by, so a press in the strip resolves
// to the window move and a press in the page below it does not.
//
// The strip height is stated rather than read from desktop.TopInset, because a
// go test binary has no live macOS window behind it and that function reports
// 0 without one — the same substitution mvu/desktop's own
// TestDragTopClaimsTheStripAndNothingBelow makes for the same reason.
func TestTheStripClaimsTheWindowDrag(t *testing.T) {
	const width, height = 400, 300
	const stripH = int(titleBandDp)

	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w := desktop.CapTop(func() unit.Dp { return titleBandDp }, page)

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
	// which DragTop consults to leave the platform's own control buttons their
	// run — reports 0, so the strip's whole width is its empty run here, the
	// same headless case mvu/desktop's own TestDragTopClaimsTheStripAndNothingBelow
	// documents. (LeadingInset's live exclusion of the buttons' own run is
	// mvu/desktop's contract, pinned there by TestDragTopLeavesTheButtonsTheirRun;
	// this app adds no arithmetic of its own around it, only the call.)
	for _, p := range []image.Point{{X: 0, Y: 0}, {X: width / 2, Y: stripH / 2}, {X: width - 1, Y: stripH - 1}} {
		if !moveAt(p.X, p.Y) {
			t.Errorf("no window-move action at %v; the strip above the page is the drag band", p)
		}
	}
	// Below the strip is the page desktop.CapTop's InsetTop pads down to, which
	// keeps its own presses — the checkboxes, the row labels, the delete
	// glyphs and the floating add button.
	for _, p := range []image.Point{{X: width / 2, Y: stripH}, {X: width / 2, Y: 150}, {X: width / 2, Y: height - 1}} {
		if moveAt(p.X, p.Y) {
			t.Errorf("window-move action at %v; the page below the strip is the page's, not the band's", p)
		}
	}
}

// TestTheCapIsANoOpWithNoStrip mirrors InsetTop's own no-strip case: a
// height of 0 — headless rendering, and every platform other than macOS —
// claims nothing and lays the page out at the window's own top.
func TestTheCapIsANoOpWithNoStrip(t *testing.T) {
	const width, height = 400, 300

	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w := desktop.CapTop(func() unit.Dp { return 0 }, page)

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

// TestTheStripStaysDraggableUnderTheModal is the one place this window's page
// differs from the plain-page recipe: the modal covers the strip, and the
// strip's drag claim has to outlive it. A cover is not a reason a window may
// not be moved — the platform's own sheets never stop one — but the modal's
// Escape catcher is a whole-window input region recorded after the band, and
// it shadowed the claim until View gave the strip back on top of the dialog.
// This is the regression that would return the moment those two lines were
// reordered, and no frame would show it.
func TestTheStripStaysDraggableUnderTheModal(t *testing.T) {
	const width, height = 650, 600
	const stripH = int(titleBandDp)

	th := staticThemed(tokens.DefaultLight)
	w := view(th, fixtureModel("add.todo"), func() unit.Dp { return titleBandDp })

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(width, height)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)

	var r input.Router
	r.Frame(&ops)
	at := f32.Pt(float32(width/2), float32(stripH/2))
	if a, ok := r.ActionAt(at); !ok || a != system.ActionMove {
		t.Errorf("no window-move action at %v with the modal open; the scrim has taken the window's own drag band", at)
	}
}
