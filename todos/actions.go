package main

// Ids are assigned by the reducer, never by the view.

// SetRoute navigates: "" home, "add.todo" or "edit.todo" for the dialog.
type SetRoute struct {
	Route string
}

// AddTodo appends a new todo; the reducer assigns its Id.
type AddTodo struct {
	Text string
}

// UpdateTodo replaces the text of the todo with the given Id.
type UpdateTodo struct {
	Id   int
	Text string
}

// ToggleTodo flips the completed state of the todo with the given Id.
type ToggleTodo struct {
	Id int
}

// SelectTodo selects the todo with the given Id in the list. The edit dialog
// edits the selected todo.
type SelectTodo struct {
	Id int
}

// DeleteTodo removes the todo with the given Id.
type DeleteTodo struct {
	Id int
}

// DeleteSelected removes the list's selected todo. The update resolves the
// todo from Model, so a menu item can post this fixed message. It does
// nothing while no todo is selected or while a dialog is open.
type DeleteSelected struct{}
