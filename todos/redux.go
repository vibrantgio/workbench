package main

// The reducers are pure functions from (state, message) to state — one per
// Model field, composed by ReduceModel. They are trivially unit-testable;
// see redux_test.go.

func ReduceModel(m Model, message any) Model {
	return Model{
		Route:    ReduceRoute(m.Route, message),
		Selected: ReduceSelected(m.Selected, message),
		List:     ReduceTodoList(m.List, message),
	}
}

func ReduceRoute(route string, message any) string {
	switch msg := message.(type) {
	case SetRoute:
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
