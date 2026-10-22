package main

import (
	"image"
	"strings"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// commandKey answers the key the command posting msg carries, out of the
// window's own table. A message with no command in the table answers the
// empty name, which every caller here fails on.
func commandKey(msg mvu.Message) key.Name {
	for _, c := range MenuCommands {
		if c.Msg == msg {
			return c.Key
		}
	}
	return ""
}

// driveChordFrame lays one frame of w out through a real input.Router.
func driveChordFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) {
	ops.Reset()
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      r.Source(),
	}
	w(gtx)
	r.Frame(ops)
}

// postedByChord drives the real key areas the window lays out — the
// layout.Widgets ChordAreas builds, not a copy of them — and returns every
// message they posted for one press of name with the platform modifier.
func postedByChord(t *testing.T, name key.Name) []mvu.Message {
	t.Helper()

	var posted []mvu.Message
	areas := ChordAreas(func(_ layout.Context, msg mvu.Message) {
		posted = append(posted, msg)
	})
	frame := func(gtx layout.Context) layout.Dimensions {
		for _, a := range areas {
			a(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	size := image.Pt(400, 300)

	driveChordFrame(frame, ops, r, size)
	r.Queue(key.Event{Name: name, Modifiers: key.ModShortcut, State: key.Press})
	driveChordFrame(frame, ops, r, size)
	return posted
}

// The bar carries the two acts this window performs, and each item posts the
// message its own control posts: Add Feed… the pane mark's OpenAddFeed, and
// Preferences… the accelerator's OpenPreferences. Reading the message out of
// the declaration handed to the menu bar and out of the key area the window
// lays out is the point — a menu that opened the dialog while the chord
// printed beside it opened the preferences is what this forbids.
func TestTheMenuItemsPostTheSameMessagesAsTheChords(t *testing.T) {
	items := MenuItems()
	if len(items) != 2 {
		t.Fatalf("the menu declares %d items, want the 2 this window's acts need", len(items))
	}
	for _, item := range items {
		t.Run(item.Title, func(t *testing.T) {
			if item.Key == "" {
				t.Fatalf("%q carries no chord; every item here is a chord the window already answers", item.Title)
			}
			// The menu's key equivalent is the character; the window's chord
			// is the key's name, which Gio spells upper-case.
			name := key.Name(strings.ToUpper(item.Key))
			posted := postedByChord(t, name)
			if len(posted) != 1 {
				t.Fatalf("the chord on %q posted %d messages, want exactly 1", name, len(posted))
			}
			if posted[0] != item.Msg {
				t.Fatalf("the chord on %q posts %T, while the menu item %q posts %T",
					name, posted[0], item.Title, item.Msg)
			}
		})
	}
}

// What the bar holds, and where. Preferences… belongs to the application's
// own menu on this platform — the only place this window offers it at all —
// and Add Feed… to File beside it, which is where the platform keeps an act
// that makes something new. Both chords are the platform's own: Command-N
// for the new thing, Command-comma for the settings.
func TestTheBarHoldsAddFeedAndPreferencesWithTheirChords(t *testing.T) {
	want := map[string]struct {
		menu string
		key  string
		msg  mvu.Message
	}{
		"Add Feed…":    {menu: "File", key: "n", msg: OpenAddFeed{}},
		"Preferences…": {menu: desktop.ApplicationMenu, key: ",", msg: OpenPreferences{}},
	}
	for _, item := range MenuItems() {
		w, ok := want[item.Title]
		if !ok {
			t.Fatalf("unexpected menu item %q", item.Title)
		}
		if item.Menu != w.menu {
			t.Errorf("%q sits in menu %q, want %q", item.Title, item.Menu, w.menu)
		}
		if item.Key != w.key {
			t.Errorf("%q carries the key equivalent %q, want %q", item.Title, item.Key, w.key)
		}
		if item.Msg != w.msg {
			t.Errorf("%q posts %T, want %T", item.Title, item.Msg, w.msg)
		}
		delete(want, item.Title)
	}
	for title := range want {
		t.Errorf("the menu no longer declares %q", title)
	}
}

// Every chord the table declares is laid out as a key area as well: away
// from macOS the menu declaration is inert, and the key area is then the
// action's only route.
func TestTheWindowLaysOutEveryChord(t *testing.T) {
	if got, want := len(ChordAreas(func(layout.Context, mvu.Message) {})), len(MenuCommands); got != want {
		t.Fatalf("the window lays out %d key areas, want one per chord (%d)", got, want)
	}
}
