package main

import "github.com/vibrantgio/mvu"

// Update is the MVU update function. This scaffold has no messages and no
// side effects, so every call returns the model unchanged.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	return model, mvu.DoNothing()
}
