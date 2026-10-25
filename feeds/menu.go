// menu.go is the window's keyboard accelerators and the application menu
// bar, declared once: every action this window offers through a control
// also stands in the bar with its key equivalent, which is what keeps an
// action offered when the control that offers it has gone away with the
// pane.
//
// The INVOCATION half of the dialog grammar lives here with them. A dialog
// cannot own how you arrived: patterns/dialog owns dismissal (a dismissible dialog's ghost
// X, Escape, the backdrop; a decision's Escape-to-Cancel and
// Return-to-default) because those are affordances the dialog draws or traps
// while it is on screen, and arrival has to be live when NO dialog exists.
//
// The platform-correct modifier is Gio's, not ours: key.ModShortcut is Cmd on
// darwin and Ctrl everywhere else. Testing runtime.GOOS here would be a
// second copy of a decision the toolkit already made.
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
// key area over the whole frame.
type Chord struct {
	Key key.Name
	Msg mvu.Message
}

// MenuCommand is a chord the application menu carries as well: the same key
// and the same message, under a title in a named menu. The two are one
// declaration: an item that posts a different message from the chord printed
// beside it is unwriteable.
type MenuCommand struct {
	Chord

	// Menu is the menu the item sits in, desktop.ApplicationMenu for the
	// application's own.
	Menu string

	// Title is the item's own text in the menu.
	Title string
}

// MenuCommands are the two actions this window performs that are not an edit
// of the text under the cursor, which is what makes it safe for the menu to
// answer their key equivalents first: a menu key equivalent is taken before
// the key reaches the window, so whatever holds the focus never sees these.
//
// Add Feed… is the one the pane took away. It stands bare in the pane's own
// strip, so with the pane sent away the window offers it through no control
// at all; the bar is where the platform keeps every act a window performs,
// and it holds this one whether the pane stands or not.
//
// Preferences… has no control anywhere in the window, so the bar is the only
// place a reader on this platform looks for it — and the place this platform
// keeps it.
var MenuCommands = []MenuCommand{
	{Chord: Chord{Key: "N", Msg: OpenAddFeed{}}, Menu: "File", Title: "Add Feed…"},
	{Chord: Chord{Key: ",", Msg: OpenPreferences{}}, Menu: desktop.ApplicationMenu, Title: "Preferences…"},
}

// Chords returns every chord the window lays out. The window declares all of
// them even where the menu carries the same key: away from macOS the menu
// declaration is inert, and the chord is then the only way to the action.
func Chords() []Chord {
	chords := make([]Chord, 0, len(MenuCommands))
	for _, c := range MenuCommands {
		chords = append(chords, c.Chord)
	}
	return chords
}

// ChordAreas returns one key area per chord, each posting that chord's
// message through post when the chord fires. The window lays them out at the
// bottom of its hit stack; post is what recording the message costs, and it
// is a parameter so a test can drive the very areas the window lays out and
// read what they post — the message a chord posts and the message the menu
// item beside it posts have to be the same one, and only a test that reads
// both can say so.
func ChordAreas(post func(gtx layout.Context, msg mvu.Message)) []layout.Widget {
	chords := Chords()
	areas := make([]layout.Widget, 0, len(chords))
	for _, c := range chords {
		msg := c.Msg
		areas = append(areas, shortcutArea(c.Key, func(gtx layout.Context) {
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
