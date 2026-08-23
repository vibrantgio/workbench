package main

import "github.com/vibrantgio/mvu"

// Model is the single application state. Every mutation flows through
// Update; nothing outside the reducers writes to it. The page has no
// fields — scroll position lives in the view subscription.
type Model struct{}

// Init returns the seed Model and startup command the loop starts from.
func Init() (Model, mvu.Command) {
	return Model{}, mvu.DoNothing()
}
