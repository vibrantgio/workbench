package main

import (
	"testing"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// TestALateSubscriberReadsTheModelInForce drives the production seam — this
// window's own reducer under mvu.Loop, over the message stream MindChat
// merges — and subscribes it a second time only after a message has moved the
// model.
//
// This is the invariant an AutoConnect count used to stand for, and it holds
// without one: the header's model picker takes its value and its options as
// static props, so it derives a key for each and subscribes a new control
// whenever one changes, and the modals subscribe their open flags as they are
// built. A stream without replay leaves a subscriber attaching after the seed
// with no model at all — a picker on an empty catalogue, a modal that never
// opens — until the next message.
func TestALateSubscriberReadsTheModelInForce(t *testing.T) {
	messages := make(chan mvu.Message, 1)
	init := func() (Model, mvu.Command) { return Model{DataDir: t.TempDir()}, mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(messages), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	// One early subscriber, so the loop is connected and draining.
	early := make(chan Model, 8)
	first := models.Subscribe(rx.GoroutineContext(), func(m Model, err error, done bool) {
		if err == nil && !done {
			select {
			case early <- m:
			default:
			}
		}
	})
	defer first.Unsubscribe()
	await(t, early, func(m Model) bool { return !m.ModelMenu }, "the seed")

	messages <- OpenModelMenu{}
	await(t, early, func(m Model) bool { return m.ModelMenu }, "the model the message left")

	// The late subscriber attaches with the menu already open.
	late := make(chan Model, 8)
	second := models.Subscribe(rx.GoroutineContext(), func(m Model, err error, done bool) {
		if err == nil && !done {
			select {
			case late <- m:
			default:
			}
		}
	})
	defer second.Unsubscribe()
	await(t, late, func(m Model) bool { return m.ModelMenu }, "the model in force at a late subscription")
}

// await polls values until one satisfies cond, and fails naming what was
// waited for.
func await(t *testing.T, values <-chan Model, cond func(Model) bool, what string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-values:
			if cond(m) {
				return
			}
		case <-deadline:
			t.Fatalf("the model stream never carried %s", what)
		}
	}
}
