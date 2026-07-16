package main

import "github.com/vibrantgio/mvu"

// Init returns the seed Model and startup command the loop starts from.
func Init() (Model, mvu.Command) {
	return Model{List: TodoList{
		{Id: 0, Text: "Learn Go", Completed: true},
		{Id: 1, Text: "Learn ReactiveX"},
	}}, mvu.DoNothing()
}
