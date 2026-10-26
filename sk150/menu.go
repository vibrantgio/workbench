// menu.go declares the window's keyboard accelerators and the application
// menu bar once. Every action this window offers through a control also
// stands in the bar.
//
// key.ModShortcut is Command on darwin and Control on every other platform.
// Gio sets it, so this file does not read runtime.GOOS.
package main

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// Chord is one of the window's keyboard accelerators: the key, with the
// platform's shortcut modifier implied, and the message a press posts. The
// window lays out each chord as a key area over the whole frame. An empty
// Key is no chord.
type Chord struct {
	Key key.Name
	Msg mvu.Message
}

// MenuCommand is a menu item and the chord it shares a message with. The
// item and its chord are one declaration, so the item cannot post a
// different message from the chord it shows.
type MenuCommand struct {
	Chord

	// Menu is the menu that holds the item.
	Menu string

	// Title is the item's text in the menu.
	Title string
}

// MenuCommands are the window's actions. Each item posts the message of its
// control. The bar shows every item in every state of the window, and the
// update ignores a message that does not apply.
var MenuCommands = menuCommands()

func menuCommands() []MenuCommand {
	cmds := []MenuCommand{
		// The preset editor's Save button reads the editor's fields, so the
		// item posts SavePreset and the button's frame turns it into the
		// button's own SavePresetEdit. Command-S is the platform's Save
		// chord.
		{Chord: Chord{Key: "S", Msg: SavePreset{}}, Menu: "File", Title: "Save"},
		// The header's power button.
		{Chord: Chord{Msg: ToggleOutput{}}, Menu: "File", Title: "Output On/Off"},
		// The protection row's Clear button.
		{Chord: Chord{Msg: ClearProtection{}}, Menu: "File", Title: "Clear Protection"},
		// The offline notice's demo button.
		{Chord: Chord{Msg: EnterDemo{}}, Menu: "File", Title: "Try Demo Mode"},
		// The Monitor's Set button.
		{Chord: Chord{Msg: EditActivePreset{}}, Menu: "Edit", Title: "Edit Active Preset"},
	}
	// Each preset row's Recall and Edit buttons, titled with the row's
	// memory group.
	for n := 0; n <= 9; n++ {
		cmds = append(cmds, MenuCommand{Chord: Chord{Msg: RecallPreset{N: n}}, Menu: "Edit", Title: fmt.Sprintf("Recall M%d", n)})
	}
	for n := 0; n <= 9; n++ {
		cmds = append(cmds, MenuCommand{Chord: Chord{Msg: EditPreset{N: n}}, Menu: "Edit", Title: fmt.Sprintf("Edit M%d", n)})
	}
	// The tab strip's three tabs.
	for i, s := range tabScreens {
		cmds = append(cmds, MenuCommand{Chord: Chord{Msg: SetScreen{Screen: s}}, Menu: "View", Title: tabTitles[i]})
	}
	return cmds
}

// Chords returns every chord the window lays out. The window declares a
// chord even where the menu shows the same key: away from macOS the menu
// declaration is inert, and the chord is then the only way to the action.
func Chords() []Chord {
	chords := make([]Chord, 0, len(MenuCommands))
	for _, c := range MenuCommands {
		if c.Key != "" {
			chords = append(chords, c.Chord)
		}
	}
	return chords
}

// ChordAreas returns one key area per chord. Each area posts its chord's
// message through post when the chord fires. post is a parameter so a test
// can drive the areas the window lays out and read what they post.
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

// MenuItems returns the menu bar's declaration for desktop.NewMenuBar. The
// key goes lowercased: Gio names a key upper-case ("N"), and an upper-case
// key equivalent asks for Shift as well.
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

// OnShortcutKey calls cb on each press of name with the platform's shortcut
// modifier. Its area spans the frame it is given and lets pointer input
// pass through. A focused control that claims the same chord gets it first.
func OnShortcutKey(name key.Name, cb func(gtx layout.Context)) layout.Widget {
	tag := new(int)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer pointer.PassOp{}.Push(gtx.Ops).Pop()
		defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, tag)
		for {
			e, ok := gtx.Event(key.Filter{Name: name, Required: key.ModShortcut})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				cb(gtx)
			}
		}
		return layout.Dimensions{Size: size}
	}
}
