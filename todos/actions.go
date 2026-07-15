package main

// The message vocabulary. Widgets emit these via
// mvu.MessageOp{Message: ...}.Add(gtx.Ops); Update folds them into the next
// Model. Ids are assigned by the reducer (see ReduceTodoList), never by the
// view.

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

// SelectTodo marks the todo with the given Id as the edit target.
type SelectTodo struct {
	Id int
}

// DeleteTodo removes the todo with the given Id.
type DeleteTodo struct {
	Id int
}
