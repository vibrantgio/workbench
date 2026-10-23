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

	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheStripStaysDraggableUnderTheDialog: the dialog covers the strip, and the
// strip's drag claim has to outlive it. The dialog's Escape catcher is a
// whole-window input region recorded after the band, so it shadows the claim
// unless View gives the strip back on top of the dialog. No frame would show
// this.
func TestTheStripStaysDraggableUnderTheDialog(t *testing.T) {
	const width, height = 650, 600
	const stripH = int(bandDp)

	th := staticThemed(tokens.PlatformLight)
	w := view(th, fixtureModel("add.todo"), complist.NewState(), func() unit.Dp { return bandDp })

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
		t.Errorf("no window-move action at %v with the dialog open; the scrim has taken the window's own drag band", at)
	}
}
