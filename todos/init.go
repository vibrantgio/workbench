package main

// Init returns the seed Model the message scan starts from.
func Init() Model {
	return Model{List: TodoList{
		{Id: 0, Text: "Learn Go", Completed: true},
		{Id: 1, Text: "Learn ReactiveX"},
	}}
}
