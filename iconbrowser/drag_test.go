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

// TestDragUnderStripClaimsTheWindowDrag is R6's second half for this window:
// the native drag leaves with the native strip, so a window that takes the
// full-size-content treatment and claims nothing back cannot be moved by its
// top edge at all. dragUnderStrip declares desktop.DragTop over the same
// measured height desktop.InsetTop pads by, so a press in the strip resolves to
// the window move and a press in the page below it does not.
//
// The strip height is stated rather than read from desktop.TopInset, because a
// go test binary has no live macOS window behind it and that function reports 0
// without one — the same substitution mvu/desktop's own
// TestDragTopClaimsTheStripAndNothingBelow makes for the same reason.
func TestDragUnderStripClaimsTheWindowDrag(t *testing.T) {
	const width, height = 400, 300
	const stripH = int(titleBandDp)

	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w := dragUnderStrip(func() unit.Dp { return titleBandDp }, page)

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
	// Below the strip is the page dragUnderStrip's InsetTop pads down to, which
	// keeps its own presses — the search field's editor and the grid's scroll.
	for _, p := range []image.Point{{X: width / 2, Y: stripH}, {X: width / 2, Y: 150}, {X: width / 2, Y: height - 1}} {
		if moveAt(p.X, p.Y) {
			t.Errorf("window-move action at %v; the page below the strip is the page's, not the band's", p)
		}
	}
}

// TestDragUnderStripIsANoOpWithNoStrip mirrors InsetTop's own no-strip case: a
// height of 0 — headless rendering, and every platform other than macOS —
// claims nothing and lays the page out at the window's own top.
func TestDragUnderStripIsANoOpWithNoStrip(t *testing.T) {
	const width, height = 400, 300

	page := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w := dragUnderStrip(func() unit.Dp { return 0 }, page)

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

// TestTheSearchFieldKeepsItsOwnPresses is the shadowing question this window
// has to answer, and the reason it is asked here is todos: there a modal's
// whole-window Escape catcher, recorded after the band, swallowed the drag
// claim, and the fix was a second claim on top. The catcher this page carries
// is the search field's, and it points the other way — the field is a focus and
// key target, and a drag band laid over a control makes that control's press the
// window's, because a move action swallows the press before the control sees
// one.
//
// Neither happens, and the frame cannot show it: the band is claimed above the
// inset the field is laid out below, so the two never overlap. Pressed at its
// own centre the field is still the field's, and the strip above it is still
// the window's.
func TestTheSearchFieldKeepsItsOwnPresses(t *testing.T) {
	tok := staticThemed(t, tokens.DefaultLight)
	page := Page(tok, staticSearch(t, tokens.DefaultLight), Model{}, &layout.List{Axis: layout.Vertical})
	w := dragUnderStrip(func() unit.Dp { return titleBandDp }, page)

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(windowSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)

	var r input.Router
	r.Frame(&ops)

	// The strip, past the leading run the buttons stand in — which is 0 here,
	// with no live window to measure.
	at := f32.Pt(float32(windowSize.X/2), float32(int(titleBandDp)/2))
	if a, ok := r.ActionAt(at); !ok || a != system.ActionMove {
		t.Errorf("no window-move action at %v; the page has taken the window's own drag band", at)
	}

	// The search field's own middle: the strip, then the page's Padding, then
	// half a comfortable control height into the field.
	field := f32.Pt(float32(windowSize.X/2), float32(int(titleBandDp)+int(Padding)+int(tokens.Comfortable.ControlHeight)/2))
	if a, ok := r.ActionAt(field); ok && a == system.ActionMove {
		t.Errorf("window-move action at %v, in the search field; a band over a control eats that control's press", field)
	}
}
