package main

import "github.com/vibrantgio/mvu"

// Model is the single application state. Every mutation flows through
// Update; nothing outside the reducers writes to it. The scaffold is an
// empty page — the landing sections arrive later.
type Model struct{}

// Init returns the seed Model and startup command the loop starts from.
func Init() (Model, mvu.Command) {
	return Model{}, mvu.DoNothing()
}
