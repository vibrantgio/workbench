package main

import (
	"image"
	"strings"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// postedByChord drives the real key area the window lays out for name — the
// widget ChordAreas builds, not a copy of it — and returns every message it
// posted for one press of that chord with the platform modifier.
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

	driveKeyFrame(frame, ops, r, size)
	r.Queue(key.Event{Name: name, Modifiers: key.ModShortcut, State: key.Press})
	driveKeyFrame(frame, ops, r, size)
	return posted
}

// The item and the chord must be the same action, not two spellings of one.
// Each declared menu item is matched to the chord on its key and the two
// messages compared — the chord's read out of the widget the window actually
// lays out, the item's out of the declaration handed to the menu bar. A menu
// that opened settings while Cmd-comma toggled the pane is the failure this
// forbids, and it is worth forbidding because on this platform only one of
// the two ever fires: the menu answers the key equivalent before the window
// sees it.
func TestTheMenuItemsPostTheSameMessagesAsTheChords(t *testing.T) {
	items := MenuItems()
	if len(items) != 3 {
		t.Fatalf("the application menu declares %d items, want the 3 the window's actions need", len(items))
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

// Which menu each item lands in is load-bearing: settings
// belongs to the application's own menu on this platform — that is where a
// reader looks for it with the pane away — and the other two belong to menus
// of their own beside it.
func TestSettingsStandsInTheApplicationsOwnMenu(t *testing.T) {
	want := map[string]string{
		"New Chat":                "File",
		"Hide/Show Conversations": "View",
		"Settings…":               desktop.ApplicationMenu,
	}
	for _, item := range MenuItems() {
		menu, ok := want[item.Title]
		if !ok {
			t.Fatalf("unexpected menu item %q", item.Title)
		}
		if item.Menu != menu {
			t.Errorf("%q sits in menu %q, want %q", item.Title, item.Menu, menu)
		}
		delete(want, item.Title)
	}
	for title := range want {
		t.Errorf("the menu no longer declares %q", title)
	}
}

// Undo must stay out of the menu. A menu key equivalent is answered before
// the key reaches the window, so an Undo item would take Cmd-Z from every
// focused editor in the application — the layering OnShortcutKey exists to
// preserve. The window still answers it.
func TestUndoIsAWindowChordAndNotAMenuItem(t *testing.T) {
	for _, item := range MenuItems() {
		if strings.EqualFold(item.Key, "z") {
			t.Fatalf("the menu declares %q on Cmd-Z, which would take undo from every text editor", item.Title)
		}
	}
	posted := postedByChord(t, "Z")
	if len(posted) != 1 {
		t.Fatalf("the undo chord posted %d messages, want exactly 1", len(posted))
	}
	if _, ok := posted[0].(UndoDelete); !ok {
		t.Fatalf("the undo chord posts %T, want main.UndoDelete", posted[0])
	}
}

// Every chord the window answers is laid out, menu-carried or not: away from
// macOS the menu declaration is inert, and the key area is then the action's
// only route.
func TestTheWindowLaysOutEveryChord(t *testing.T) {
	if got, want := len(ChordAreas(func(layout.Context, mvu.Message) {})), len(MenuCommands)+len(WindowChords); got != want {
		t.Fatalf("the window lays out %d key areas, want one per chord (%d)", got, want)
	}
}
