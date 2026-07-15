package main

import (
	"reflect"
	"testing"
)

func seedList() TodoList {
	return TodoList{
		{Id: 0, Text: "Learn Go", Completed: true},
		{Id: 1, Text: "Learn ReactiveX"},
	}
}

func TestAddTodoAssignsNextId(t *testing.T) {
	next := ReduceTodoList(seedList(), AddTodo{Text: "Ship it"})
	if len(next) != 3 {
		t.Fatalf("len = %d, want 3", len(next))
	}
	got := next[2]
	want := Todo{Id: 2, Text: "Ship it"}
	if got != want {
		t.Fatalf("appended = %+v, want %+v", got, want)
	}
}

func TestAddTodoNeverReusesIdAfterDelete(t *testing.T) {
	// Delete the highest-Id todo, then add: the new todo must not reuse
	// Id 1, or a stale SelectTodo/edit route would target the wrong todo.
	l := ReduceTodoList(seedList(), DeleteTodo{Id: 0})
	l = ReduceTodoList(l, AddTodo{Text: "New"})
	if got := l[len(l)-1].Id; got != 2 {
		t.Fatalf("new Id = %d, want 2", got)
	}
}

func TestUpdateTodoChangesOnlyTarget(t *testing.T) {
	next := ReduceTodoList(seedList(), UpdateTodo{Id: 1, Text: "Learn rx"})
	want := TodoList{
		{Id: 0, Text: "Learn Go", Completed: true},
		{Id: 1, Text: "Learn rx"},
	}
	if !reflect.DeepEqual(next, want) {
		t.Fatalf("list = %+v, want %+v", next, want)
	}
}

func TestToggleTodoFlipsCompleted(t *testing.T) {
	next := ReduceTodoList(seedList(), ToggleTodo{Id: 0})
	if next[0].Completed {
		t.Fatal("todo 0 still completed after toggle")
	}
	if next[1].Completed {
		t.Fatal("toggle leaked to todo 1")
	}
}

func TestDeleteTodoRemovesById(t *testing.T) {
	next := ReduceTodoList(seedList(), DeleteTodo{Id: 0})
	want := TodoList{{Id: 1, Text: "Learn ReactiveX"}}
	if !reflect.DeepEqual(next, want) {
		t.Fatalf("list = %+v, want %+v", next, want)
	}
}

func TestReducersAreImmutable(t *testing.T) {
	orig := seedList()
	ReduceTodoList(orig, UpdateTodo{Id: 0, Text: "mutated?"})
	ReduceTodoList(orig, ToggleTodo{Id: 0})
	if !reflect.DeepEqual(orig, seedList()) {
		t.Fatalf("input list mutated: %+v", orig)
	}
}

func TestReduceModelRoutesAndSelection(t *testing.T) {
	m := Init()
	m = ReduceModel(m, SelectTodo{Id: 1})
	m = ReduceModel(m, SetRoute{Route: "edit.todo"})
	if m.Route != "edit.todo" || m.Selected != 1 {
		t.Fatalf("route %q selected %d, want edit.todo / 1", m.Route, m.Selected)
	}
	if todo, ok := m.List.Find(m.Selected); !ok || todo.Text != "Learn ReactiveX" {
		t.Fatalf("Find(%d) = %+v, %v", m.Selected, todo, ok)
	}
	m = ReduceModel(m, SetRoute{})
	if m.Route != "" {
		t.Fatalf("route %q, want home", m.Route)
	}
}

func TestFindByIdNotIndex(t *testing.T) {
	// After deleting todo 0, todo 1 sits at index 0. Find must still
	// resolve it by Id — indexing List[Selected] was a crash in the
	// pre-migration app.
	m := Model{List: seedList(), Selected: 1}
	m.List = ReduceTodoList(m.List, DeleteTodo{Id: 0})
	todo, ok := m.List.Find(m.Selected)
	if !ok || todo.Id != 1 {
		t.Fatalf("Find(1) = %+v, %v", todo, ok)
	}
	if _, ok := m.List.Find(0); ok {
		t.Fatal("Find(0) found a deleted todo")
	}
}

func TestUnknownMessageIsIdentity(t *testing.T) {
	m := ReduceModel(Init(), struct{ Unrelated string }{"noop"})
	if !reflect.DeepEqual(m, Init()) {
		t.Fatalf("unknown message changed the model: %+v", m)
	}
}
