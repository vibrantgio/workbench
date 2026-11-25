package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// The menu bar's items, in declaration order: the Edit menu's, then the
// Card menu's.
const (
	undoItem = iota
	redoItem
	moveItem
	moveToColumnItem
)

// menuBar is the declaration of the menu bar's items: the Edit menu's
// and the Card menu's, as a board with no change and no selected card
// shows them.
func menuBar() []desktop.MenuItem {
	return append(editMenu(), cardMenu(Model{})...)
}

// editMenu is the Edit menu's declaration: Undo with Cmd-Z and Redo with
// Shift-Cmd-Z, both disabled, as a board with no change shows them.
//
// An enabled item takes its key before the window sees it. A disabled item
// gives its key to the window. There, a field that holds the keyboard reads
// the key first. The field changes its own text when it holds typing for
// the key. After the user types, the field drops Shift-Cmd-Z until an
// Undo reverses a change, or until it loses the keyboard. The field gives
// each other press to the board's Undo, which reports whether it found a
// change. Without such a field, the board reads the key.
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
// stack is empty. The Undo item is also disabled when the field that holds
// the keyboard holds typing for Undo: that field takes Cmd-Z and changes
// its own text. A field with no such typing gives the key to the board.
//
// The Redo item is also disabled when the user typed in that field after
// the last Undo: the field then drops Shift-Cmd-Z. A field that loses the
// keyboard drops its typing, so a field that gets the keyboard again holds
// none, and the item follows the board's stack. Otherwise the board's
// last undone change comes first, also when the field holds typing for
// Redo. Typing empties the redo stack, so each step on it comes from an
// Undo after the field's Undo. With the stack empty, the field takes
// Shift-Cmd-Z.
func editMenuState(m Model) []menuItem {
	var field Typing
	if m.IsAdding {
		field = m.AddTyping
	}
	if m.IsEditing {
		field.Undo = field.Undo || m.EditTyping.Undo
		field.RedoEnded = field.RedoEnded || m.EditTyping.RedoEnded
	}
	item := func(verb string, stack []Step, held bool) menuItem {
		if held || len(stack) == 0 {
			return menuItem{Title: verb, Disabled: true}
		}
		return menuItem{Title: verb + " " + stack[len(stack)-1].Name}
	}
	return []menuItem{
		undoItem: item("Undo", m.Undo, field.Undo),
		redoItem: item("Redo", m.Redo, field.RedoEnded),
	}
}

// cardMenu is the Card menu's items for m, as the Notes and Reminders menu
// files hold them. "Move Item" holds "Up" on Ctrl-Cmd-Up and "Down" on
// Ctrl-Cmd-Down. "Move to Column" has no key equivalent and holds an item
// for each column, in the file's order. [menuBar] puts the result after
// [editMenu], so its first item is moveItem.
//
// With no selected card, every item is disabled. "Up" is disabled on the
// column's first card and "Down" on its last. The card's own column is
// disabled. Each item is enabled exactly when the board's update moves
// the card for its message, so the menu and the window's keys never
// differ.
//
// A column whose heading has no text has no item: a menu item needs a
// title, and the board invents no text. The other items keep their
// columns' indices in their messages.
func cardMenu(m Model) []desktop.MenuItem {
	_, upOK := moveTarget(m, MoveUp{})
	_, downOK := moveTarget(m, MoveDown{})
	var columns []desktop.MenuItem
	for i, c := range m.Board.Columns {
		if c.Title == "" {
			continue
		}
		_, ok := moveTarget(m, MoveToColumn{Column: i})
		columns = append(columns, desktop.MenuItem{Title: c.Title, Disabled: !ok, Msg: MoveToColumn{Column: i}})
	}
	return []desktop.MenuItem{
		{Menu: "Card", Title: "Move Item", Submenu: true, Disabled: !m.HasSelected, Items: []desktop.MenuItem{
			{Title: "Up", Key: desktop.UpArrow, Control: true, Disabled: !upOK, Msg: MoveUp{}},
			{Title: "Down", Key: desktop.DownArrow, Control: true, Disabled: !downOK, Msg: MoveDown{}},
		}},
		{Menu: "Card", Title: "Move to Column", Submenu: true, Disabled: !m.HasSelected, Items: columns},
	}
}

// showMenus gives each item of bar, as [menuBar] declared it, the state
// that [editMenuState] and [cardMenu] give for m.
func showMenus(bar *desktop.MenuBar, m Model) {
	for i, it := range editMenuState(m) {
		bar.Set(i, it.Title, it.Disabled)
	}
	for j, it := range cardMenu(m) {
		bar.Set(moveItem+j, it.Title, it.Disabled)
		bar.SetItems(moveItem+j, it.Items...)
	}
}
