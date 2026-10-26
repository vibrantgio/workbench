package main

// The reducers are pure functions from (state, message) to state — one per
// Model field, composed by ReduceModel.

func ReduceModel(m Model, message any) Model {
	if _, ok := message.(DeleteSelected); ok {
		message = resolveDeleteSelected(m)
	}
	return Model{
		Route:       ReduceRoute(m.Route, message),
		Selected:    ReduceSelected(m.Selected, message),
		HasSelected: ReduceHasSelected(m.HasSelected, m.Selected, message),
		List:        ReduceTodoList(m.List, message),
	}
}

// resolveDeleteSelected turns DeleteSelected into the DeleteTodo of the
// selected todo. With no todo selected, or with a dialog open, it returns
// nil, which every reducer passes through unchanged. The guard on the route
// keeps the dialog's todo: the menu bar takes a choice while a dialog is
// open.
func resolveDeleteSelected(m Model) any {
	if !m.HasSelected || m.Route != "" {
		return nil
	}
	return DeleteTodo{Id: m.Selected}
}

// ReduceRoute opens a dialog only from the list. A dialog's route does not
// replace an open dialog's route: the menu bar takes a choice while a dialog
// is open, and the text in the dialog's field must stay.
func ReduceRoute(route string, message any) string {
	switch msg := message.(type) {
	case SetRoute:
		if route != "" && msg.Route != "" {
			return route
		}
		return msg.Route
	default:
		return route
	}
}

func ReduceSelected(selected int, message any) int {
	switch msg := message.(type) {
	case SelectTodo:
		return msg.Id
	default:
		return selected
	}
}

// ReduceHasSelected keeps the selection until the selected todo is deleted.
func ReduceHasSelected(has bool, selected int, message any) bool {
	switch msg := message.(type) {
	case SelectTodo:
		return true
	case DeleteTodo:
		return has && msg.Id != selected
	default:
		return has
	}
}

func ReduceTodoList(list TodoList, message any) TodoList {
	switch msg := message.(type) {
	case AddTodo:
		next := make(TodoList, len(list), len(list)+1)
		copy(next, list)
		return append(next, Todo{Id: list.NextId(), Text: msg.Text})
	case UpdateTodo:
		next := make(TodoList, len(list))
		for i, todo := range list {
			if todo.Id == msg.Id {
				todo.Text = msg.Text
			}
			next[i] = todo
		}
		return next
	case ToggleTodo:
		next := make(TodoList, len(list))
		for i, todo := range list {
			if todo.Id == msg.Id {
				todo.Completed = !todo.Completed
			}
			next[i] = todo
		}
		return next
	case DeleteTodo:
		next := make(TodoList, 0, len(list))
		for _, todo := range list {
			if todo.Id != msg.Id {
				next = append(next, todo)
			}
		}
		return next
	default:
		return list
	}
}
