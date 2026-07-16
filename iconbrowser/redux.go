package main

import "github.com/vibrantgio/mvu"

// Update is the MVU update function; no side effects, so always DoNothing.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	return ReduceModel(model, message), mvu.DoNothing()
}

// ReduceModel is the pure reducer — see redux_test.go.
func ReduceModel(m Model, message any) Model {
	switch msg := message.(type) {
	case SetQuery:
		m.Query = msg.Text
	}
	return m
}
