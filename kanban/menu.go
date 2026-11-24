package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// The Edit menu's items, in declaration order.
const (
	undoItem = iota
	redoItem
)

// editMenu is the Edit menu's declaration: Undo and Redo, both disabled, as
// a board with no change shows them.
//
// The items declare no key equivalent. A menu's key equivalent takes Cmd-Z
// before the window sees the key, so a field that holds the keyboard would
// lose its own undo. The board reads Cmd-Z and Shift-Cmd-Z in the window,
// behind the field's own claim on the keys.
func editMenu() []desktop.MenuItem {
	return []desktop.MenuItem{
		undoItem: {Menu: "Edit", Title: "Undo", Disabled: true, Msg: Undo{}},
		redoItem: {Menu: "Edit", Title: "Redo", Disabled: true, Msg: Redo{}},
	}
}

// menuItem is what one item of the Edit menu shows: its title, and whether
// it is disabled.
type menuItem struct {
	Title    string
	Disabled bool
}

// editMenuState is what the Edit menu's items show for m, in declaration
// order. An item names the change that it undoes or redoes, as "Undo Delete
// Card". An item is disabled, and titled "Undo" or "Redo" alone, when its
// stack is empty or when a field is open on the board. In an open field,
// Cmd-Z changes the field's text, not the board.
func editMenuState(m Model) []menuItem {
	field := m.IsAdding || m.IsEditing
	item := func(verb string, stack []Step) menuItem {
		if field || len(stack) == 0 {
			return menuItem{Title: verb, Disabled: true}
		}
		return menuItem{Title: verb + " " + stack[len(stack)-1].Name}
	}
	return []menuItem{
		undoItem: item("Undo", m.Undo),
		redoItem: item("Redo", m.Redo),
	}
}

// showEditMenu gives each item of bar, as [editMenu] declared it, the state
// that [editMenuState] gives for m.
func showEditMenu(bar *desktop.MenuBar, m Model) {
	for i, it := range editMenuState(m) {
		bar.Set(i, it.Title, it.Disabled)
	}
}
