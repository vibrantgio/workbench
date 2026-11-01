package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/theme/tokens"
)

// requestedWakeup draws one frame of w through a router, as the window does,
// and returns the wakeup that frame asked for.
func requestedWakeup(w layout.Widget, r *input.Router) (time.Time, bool) {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Now:         time.Now(),
		Source:      r.Source(),
		Metric:      unit.Metric{PxPerDp: 2, PxPerSp: 2},
		Constraints: layout.Exact(image.Pt(int(winW)*2, int(winH)*2)),
	}
	w(gtx)
	r.Frame(ops)
	return r.WakeupTime()
}

// The page and the backdrop change only when a message arrives, so a frame of
// the window without the field asks for no further frame.
func TestTheWindowWithoutTheFieldAsksForNoFrameAtRest(t *testing.T) {
	w := windowFrame(roundedThemed(tokens.PlatformDark), Model{}, titleBandDp)
	var r input.Router
	for i := 0; i < 3; i++ {
		if at, ok := requestedWakeup(w, &r); ok {
			t.Fatalf("frame %d asked for a wakeup at %v; want none", i, at)
		}
	}
}

// The field changes only at each animation tick, so every frame of the whole
// window asks for the frame of the next tick and never for a frame at once. A
// frame asked for at once redraws the window at the display's refresh rate.
func TestTheWholeWindowAsksForTheFrameOfTheNextTickOnly(t *testing.T) {
	w := livingWindowFrame(roundedThemed(tokens.PlatformDark), Model{}, titleBandDp)
	var r input.Router
	for i := 0; i < 5; i++ {
		began := time.Now()
		at, ok := requestedWakeup(w, &r)
		if !ok {
			t.Fatalf("frame %d asked for no wakeup; want the next tick's", i)
		}
		if !at.After(began) {
			t.Fatalf("frame %d asked for a wakeup at once (at %v, frame began %v)", i, at, began)
		}
		time.Sleep(time.Until(at))
	}
}
