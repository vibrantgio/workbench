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
