package main

import (
	"image"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// driveKeyFrame lays w out once against a live input router, exactly like a
// real window frame.
func driveKeyFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Ops:         ops,
		Source:      r.Source(),
	}
	w(gtx)
	r.Frame(ops)
}

// TestOnShortcutKeyFiresOnChord drives a real Cmd/Ctrl-Z key event through a
// gio input router and asserts the callback fires — the mechanics behind the
// undo shortcut.
func TestOnShortcutKeyFiresOnChord(t *testing.T) {
	fired := 0
	w := OnShortcutKey("Z", func(layout.Context) { fired++ })

	r := new(gioinput.Router)
	ops := new(op.Ops)
	size := image.Pt(400, 300)

	// Frame 1 registers the key interest area.
	driveKeyFrame(w, ops, r, size)

	r.Queue(key.Event{Name: "Z", Modifiers: key.ModShortcut, State: key.Press})
	driveKeyFrame(w, ops, r, size)
	if fired != 1 {
		t.Fatalf("callback fired %d times after Cmd-Z press, want 1", fired)
	}

	// A release must not fire again.
	r.Queue(key.Event{Name: "Z", Modifiers: key.ModShortcut, State: key.Release})
	driveKeyFrame(w, ops, r, size)
	if fired != 1 {
		t.Fatalf("callback fired %d times after release, want still 1", fired)
	}

	// The bare key without the shortcut modifier must not fire.
	r.Queue(key.Event{Name: "Z", State: key.Press})
	driveKeyFrame(w, ops, r, size)
	if fired != 1 {
		t.Fatalf("callback fired %d times after unmodified press, want still 1", fired)
	}
}
