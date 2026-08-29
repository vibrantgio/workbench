package main

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// Chord is one of the window's keyboard accelerators: the key, with the
// platform's shortcut modifier implied (Cmd on macOS, Ctrl elsewhere), and
// the message pressing it posts. The window lays every one of them out as a
// key area over the whole frame; see OnShortcutKey.
type Chord struct {
	Key key.Name
	Msg mvu.Message
}

// MenuCommand is a chord the application menu carries as well: the same key
// and the same message, under a label in a named menu. The two are one
// declaration on purpose — an item that posts a different message from the
// chord printed beside it is the defect this shape makes unwriteable.
type MenuCommand struct {
	Chord

	// Menu is the menu the item sits in, desktop.ApplicationMenu for the
	// application's own.
	Menu string

	// Title is the item's label in the menu.
	Title string
}

// MenuCommands are the three actions the application menu carries. Each is
// something the window does to itself rather than to the text under the
// cursor, which is what makes it safe for the menu to answer the chord first:
// a menu key equivalent is taken before the key reaches the window, so
// whatever has focus never sees these.
//
// Settings is the one that must be here. With the conversations pane away it
// has no control anywhere in the window — it lives at the pane's foot — so
// the menu is where a reader on this platform goes to look for it, and until
// there was a menu the chord carried that state alone.
var MenuCommands = []MenuCommand{
	// The application's primary action, reachable whether the pane is
	// standing or away.
	{Chord: Chord{Key: "N", Msg: NewChat{}}, Menu: "File", Title: "New Chat"},
	// Sends the pane away and brings it back, so the switch is reachable
	// from the keyboard and the menu in both of its states.
	{Chord: Chord{Key: "\\", Msg: ToggleSidebar{}}, Menu: "View", Title: "Hide/Show Conversations"},
	// Where this platform keeps settings, which is the whole reason the gear
	// could retreat into the pane's foot.
	{Chord: Chord{Key: ",", Msg: OpenSettings{}}, Menu: desktop.ApplicationMenu, Title: "Settings…"},
}

// WindowChords are the accelerators the window answers that the menu
// deliberately does not carry.
//
// Undo is the whole list, and the reason is layering rather than taste. A
// menu key equivalent is answered before the key reaches the window, so an
// Undo item in the menu would take Cmd-Z away from every focused text editor
// in the application and undo a deleted chat while someone was editing a
// prompt. As a window chord it stays behind the editor's own claim, which is
// where an application-wide undo belongs.
var WindowChords = []Chord{
	// Undoes a pending chat delete; the reducer ignores it when nothing is
	// pending.
	{Key: "Z", Msg: UndoDelete{}},
}

// Chords returns every chord the window lays out, menu-carried and window-only
// together. The window declares all of them even where the menu carries the
// same key: away from macOS the menu declaration is inert, and the chord is
// then the only way to the action.
func Chords() []Chord {
	chords := make([]Chord, 0, len(MenuCommands)+len(WindowChords))
	for _, c := range MenuCommands {
		chords = append(chords, c.Chord)
	}
	return append(chords, WindowChords...)
}

// ChordAreas returns one key area per chord, each posting that chord's
// message through post when the chord fires. The window lays them out at the
// bottom of its hit stack; post is what recording the message costs, and it
// is a parameter so a test can drive the very widgets the window lays out and
// read what they post — the message a chord posts and the message the menu
// item beside it posts have to be the same one, and only a test that reads
// both can say so.
func ChordAreas(post func(gtx layout.Context, msg mvu.Message)) []layout.Widget {
	chords := Chords()
	areas := make([]layout.Widget, 0, len(chords))
	for _, c := range chords {
		msg := c.Msg
		areas = append(areas, OnShortcutKey(c.Key, func(gtx layout.Context) {
			post(gtx, msg)
		}))
	}
	return areas
}

// MenuItems returns the application menu's declaration, for
// desktop.NewMenuBar. The key crosses lowercased: a chord is named by the
// key's own name here and in Gio ("N"), while a menu's key equivalent is the
// character it stands for, and an upper-case one would ask for Shift as well.
func MenuItems() []desktop.MenuItem {
	items := make([]desktop.MenuItem, 0, len(MenuCommands))
	for _, c := range MenuCommands {
		items = append(items, desktop.MenuItem{
			Menu:  c.Menu,
			Title: c.Title,
			Key:   strings.ToLower(string(c.Key)),
			Msg:   c.Msg,
		})
	}
	return items
}
