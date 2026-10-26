package main

import (
	"fmt"
	"image"
	"strings"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/tokens"
)

// menuFrameSize is the size of the frame the chord tests lay out, at one
// pixel per dp.
var menuFrameSize = image.Pt(400, 300)

// driveFrame lays one frame of w out through r at size.
func driveFrame(w layout.Widget, size image.Point, ops *op.Ops, r *gioinput.Router) {
	ops.Reset()
	w(layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      r.Source(),
	})
	r.Frame(ops)
}

// postedByChord drives the key areas ChordAreas builds and returns every
// message they post for one press of name with the modifiers mods.
func postedByChord(t *testing.T, name key.Name, mods key.Modifiers) []mvu.Message {
	t.Helper()
	var posted []mvu.Message
	areas := ChordAreas(func(_ layout.Context, msg mvu.Message) { posted = append(posted, msg) })
	frame := func(gtx layout.Context) layout.Dimensions {
		for _, a := range areas {
			a(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(frame, menuFrameSize, ops, r)
	r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	driveFrame(frame, menuFrameSize, ops, r)
	return posted
}

// menuWant is one item the bar must hold.
type menuWant struct {
	menu, title, key string
	msg              mvu.Message
}

// The bar holds, in this order: File with Save on Command-S and the header's
// and the notices' acts; Edit with the Monitor's Set and each preset row's
// Recall and Edit; View with the three tabs. Each item posts the message of
// its control.
func TestTheBarHoldsTheWindowsActions(t *testing.T) {
	want := []menuWant{
		{"File", "Save", "s", SavePreset{}},
		{"File", "Output On/Off", "", ToggleOutput{}},
		{"File", "Clear Protection", "", ClearProtection{}},
		{"File", "Try Demo Mode", "", EnterDemo{}},
		{"Edit", "Edit Active Preset", "", EditActivePreset{}},
	}
	for n := 0; n <= 9; n++ {
		want = append(want, menuWant{"Edit", fmt.Sprintf("Recall M%d", n), "", RecallPreset{N: n}})
	}
	for n := 0; n <= 9; n++ {
		want = append(want, menuWant{"Edit", fmt.Sprintf("Edit M%d", n), "", EditPreset{N: n}})
	}
	want = append(want,
		menuWant{"View", "Monitor", "", SetScreen{Screen: "monitor"}},
		menuWant{"View", "Presets", "", SetScreen{Screen: "presets"}},
		menuWant{"View", "Device", "", SetScreen{Screen: "device"}},
	)
	items := MenuItems()
	if len(items) != len(want) {
		t.Fatalf("the menu declares %d items, want %d", len(items), len(want))
	}
	for i, w := range want {
		it := items[i]
		if it.Menu != w.menu || it.Title != w.title || it.Key != w.key || it.Msg != w.msg {
			t.Errorf("item %d is %s %q on %q posting %#v, want %s %q on %q posting %#v",
				i, it.Menu, it.Title, it.Key, it.Msg, w.menu, w.title, w.key, w.msg)
		}
	}
}

// Each item with a key equivalent posts the same message as the chord the
// window lays out for that key. The bare key posts nothing.
func TestTheMenuItemsPostTheSameMessagesAsTheChords(t *testing.T) {
	chords := 0
	for _, item := range MenuItems() {
		if item.Key == "" {
			continue
		}
		chords++
		name := key.Name(strings.ToUpper(item.Key))
		posted := postedByChord(t, name, key.ModShortcut)
		if len(posted) != 1 || posted[0] != item.Msg {
			t.Errorf("the chord on %q posts %#v, the item %q posts %#v", name, posted, item.Title, item.Msg)
		}
		if bare := postedByChord(t, name, 0); len(bare) != 0 {
			t.Errorf("the bare %q posts %#v, want nothing", name, bare)
		}
	}
	if got := len(ChordAreas(func(layout.Context, mvu.Message) {})); got != chords {
		t.Errorf("the window lays out %d key areas, want one per key equivalent (%d)", got, chords)
	}
}

// SavePreset counts a request only while the editor is open.
func TestSavePresetCountsOnlyWithTheEditorOpen(t *testing.T) {
	m := goldenModel()
	if got, _ := Update(m, SavePreset{}); got.Saves != 0 {
		t.Errorf("Save with the editor closed counts %d requests, want 0", got.Saves)
	}
	m.EditPreset = 3
	if got, _ := Update(m, SavePreset{}); got.Saves != 1 {
		t.Errorf("Save with the editor open counts %d requests, want 1", got.Saves)
	}
}

// Edit on a preset opens the editor on the Presets tab from every tab, so
// the item works where the row's button is not drawn.
func TestEditPresetOpensTheEditorOnThePresetsTab(t *testing.T) {
	m := goldenModel()
	got, _ := Update(m, EditPreset{N: 4})
	if got.Screen != "presets" || got.EditPreset != 4 {
		t.Errorf("Edit M4 on the Monitor gives tab %q, editor %d; want presets, 4", got.Screen, got.EditPreset)
	}
}

// postedByEditor lays out the window with the editor open on preset 2 and
// the Save count at saves, and returns the SavePresetEdit messages the frame
// posts.
func postedByEditor(t *testing.T, saves int) []SavePresetEdit {
	t.Helper()
	m := goldenModel()
	m.Screen, m.HavePresets, m.EditPreset, m.Saves = "presets", true, 2, saves
	var posted []SavePresetEdit
	w := windowFramePosting(t, tokens.PlatformLight, m, func(_ layout.Context, msg mvu.Message) {
		if s, ok := msg.(SavePresetEdit); ok {
			posted = append(posted, s)
		}
	})
	gtx := layout.Context{
		Constraints: layout.Exact(windowSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         new(op.Ops),
	}
	w(gtx)
	return posted
}

// The editor's Save button saves the open preset on the first frame after
// the Save count changes, as a press of the button does, and not before.
func TestTheSaveButtonAnswersTheSaveCount(t *testing.T) {
	if got := postedByEditor(t, 0); len(got) != 0 {
		t.Errorf("with no Save request the frame posts %+v", got)
	}
	got := postedByEditor(t, 1)
	if len(got) != 1 || got[0].N != 2 || got[0].Stay {
		t.Errorf("after a Save request the frame posts %+v, want one save of preset 2 that leaves the editor", got)
	}
}
