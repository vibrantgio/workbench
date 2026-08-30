package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
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

// TestOnShortcutKeyDoesNotOccludePointer holds the pointer-transparency
// invariant: gio input areas occlude pointer events by default, so a
// window-wide key area laid out over the content would swallow every click in
// the app. The helper's PassOp must keep it pointer-transparent even when it
// is laid out ON TOP of a clickable.
func TestOnShortcutKeyDoesNotOccludePointer(t *testing.T) {
	clicked := 0
	click := new(widget.Clickable)
	shortcut := OnShortcutKey("Z", func(layout.Context) {})

	w := func(gtx layout.Context) layout.Dimensions {
		for click.Clicked(gtx) {
			clicked++
		}
		dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		// The WORST ordering: the key area over the content.
		shortcut(gtx)
		return dims
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	size := image.Pt(400, 300)

	driveKeyFrame(w, ops, r, size)
	centre := f32.Pt(200, 150)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: centre, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	driveKeyFrame(w, ops, r, size)
	driveKeyFrame(w, ops, r, size)

	if clicked != 1 {
		t.Fatalf("clickable fired %d times under the shortcut area, want 1 (pointer occluded?)", clicked)
	}
}

// TestTheWindowsChordsAreLiveOnTheirNames drives each of the accelerators
// this window binds through a real router: new chat, settings and the pane
// toggle. The undo chord is covered above. These three are the ones the
// floating pane's composition depends on: with the pane away, Cmd-comma is
// the only route to settings.
//
// The name each is bound to is what the check is for: a chord bound to the
// wrong key name is silently dead, and a settings control only reachable by
// a chord that does not fire is not reachable at all.
func TestTheWindowsChordsAreLiveOnTheirNames(t *testing.T) {
	for _, tc := range []struct {
		name key.Name
		what string
	}{
		{"N", "new chat"},
		{",", "settings"},
		{"\\", "the pane toggle"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			fired := 0
			w := OnShortcutKey(tc.name, func(layout.Context) { fired++ })

			r := new(gioinput.Router)
			ops := new(op.Ops)
			size := image.Pt(400, 300)

			driveKeyFrame(w, ops, r, size)
			r.Queue(key.Event{Name: tc.name, Modifiers: key.ModShortcut, State: key.Press})
			driveKeyFrame(w, ops, r, size)
			if fired != 1 {
				t.Fatalf("%s: the chord on %q fired %d times, want 1", tc.what, tc.name, fired)
			}
			// Without the modifier it is just a keystroke the editor wants.
			r.Queue(key.Event{Name: tc.name, State: key.Press})
			driveKeyFrame(w, ops, r, size)
			if fired != 1 {
				t.Fatalf("%s: the bare key fired the accelerator", tc.what)
			}
		})
	}
}
