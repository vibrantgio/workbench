package main

import (
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
	w := desktop.CapTop(func() unit.Dp { return titleBandDp }, page)

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
