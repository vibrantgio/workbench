package main

import "github.com/vibrantgio/mvu"

// Update is the MVU update function. This app has no side effects, so every
// message reduces synchronously and the command is always DoNothing; an app
// with I/O returns mvu.Do(...) commands here.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	return ReduceModel(model, message), mvu.DoNothing()
}
