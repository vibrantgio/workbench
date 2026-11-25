package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// The Edit menu's items, in declaration order.
const (
	undoItem = iota
	redoItem
)

// editMenu is the Edit menu's declaration: Undo with Cmd-Z and Redo with
// Shift-Cmd-Z, both disabled, as a board with no change shows them.
//
// An enabled item takes its key before the window sees it. A disabled item
// gives its key to the window. There, a field that holds the keyboard reads
// the key first and changes its own text. Without such a field, the board
// reads the key, and its stack is empty.
func editMenu() []desktop.MenuItem {
	return []desktop.MenuItem{
		undoItem: {Menu: "Edit", Title: "Undo", Key: "z", Disabled: true, Msg: Undo{}},
		redoItem: {Menu: "Edit", Title: "Redo", Key: "Z", Disabled: true, Msg: Redo{}},
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
// stack is empty or when a field holds the keyboard and Cmd-Z changes the
// field's text. An open edit holds the keyboard, because the edit ends when
// its field loses the keyboard. The new card's field counts once the user
// typed in it: in an empty new card, Cmd-Z changes no text.
func editMenuState(m Model) []menuItem {
	field := m.IsEditing || m.IsAdding && m.AddTyping
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
