package main

// Model is the single application state. Every mutation flows through
// Update; nothing outside the reducers writes to it.
type Model struct {
	// Route selects what is on screen: "" (the list), "add.todo", or
	// "edit.todo".
	Route string

	// Selected is the Id of the todo being edited while Route is
	// "edit.todo".
	Selected int

	List TodoList
}

type TodoList []Todo

type Todo struct {
	Id        int
	Text      string
	Completed bool
}

// Find returns the todo with the given id, or a zero Todo with ok=false.
// Todos are identified by Id, never by slice index: indexes shift when
// earlier todos are deleted.
func (l TodoList) Find(id int) (Todo, bool) {
	for _, todo := range l {
		if todo.Id == id {
			return todo, true
		}
	}
	return Todo{}, false
}

// NextId returns an Id one past the largest in use, so deletions never cause
// Id reuse within a session.
func (l TodoList) NextId() int {
	next := 0
	for _, todo := range l {
		if todo.Id >= next {
			next = todo.Id + 1
		}
	}
	return next
}
