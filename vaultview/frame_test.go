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

// TestToolbarDeclaresWindowDrag asserts what the chrome row declares to
// the window: a move action over the empty space it leaves between its
// controls, and none over the controls themselves. The row stands where
// the native title bar's drag would be, so without the declaration the
// window has no top edge to move it by; with it laid over a control, the
// control's press would be the window's rather than the application's.
//
// The probe is the same one the window makes on a press — the frame's
// own hit test, asked what action stands at a point — so it measures the
// composed row rather than the ops in isolation.
func TestToolbarDeclaresWindowDrag(t *testing.T) {
	tok := themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	rowW := 1100
	rowH := int(toolbarHeight(tok))

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(rowW, rowH)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	f := &frameState{asideW: frameAsideDp}
	f.layoutToolbar(gtx, goldenModel(), tok)

	var r input.Router
	r.Frame(&ops)
	moveAt := func(x int) bool {
		a, ok := r.ActionAt(f32.Pt(float32(x), float32(rowH)/2))
		return ok && a == system.ActionMove
	}

	// The middle of the row — between the vault's name and the trailing
	// actions — is the largest empty stretch, and the one a hand reaches
	// for. Both ends of it drag.
	for _, x := range []int{rowW / 3, rowW / 2} {
		if !moveAt(x) {
			t.Errorf("no window-move action at x=%d; the row's empty middle must move the window", x)
		}
	}
	// Every control's own span belongs to the control.
	for _, x := range []int{
		frameEdgeDp + frameGapDp + toggleBarWDp/2, // the rail toggle
		rowW - frameEdgeDp - 20,                   // inside the trailing action
	} {
		if moveAt(x) {
			t.Errorf("window-move action at x=%d; a control's own span must not move the window", x)
		}
	}
}

// TestChromeIsOneRow holds the vault window's chrome to a single row: the
// distance from the top of the window to the first pixel of content, in
// the density the app ships. Two stacked bands is what this composition
// replaced, and a band creeping back is a defect a screenshot should not
// have to be the one to catch.
func TestChromeIsOneRow(t *testing.T) {
	const budget = 40
	tok := themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	if h := toolbarHeight(tok); h > budget {
		t.Errorf("chrome above the first content row is %v dp, over the %d dp budget", h, budget)
	}
}
