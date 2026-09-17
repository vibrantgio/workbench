package main

import (
	"testing"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// TestALateSubscriberReadsTheModelInForce drives the production seam — this
// app's own reducer under mvu.Loop, over the message channel run() scans —
// and subscribes it a second time only after a message has moved the model.
//
// This is the invariant an AutoConnect count used to stand for, and it holds
// without one: feedsShellLayer derives a couple of dozen streams from the
// model and several of them are subscribed more than once, and a stream that
// flattens an observable of observables re-subscribes what it combines the
// inner one with on every outer emission, so no fixed count could be right. A
// stream without replay leaves such a subscriber with no model — and so the
// table and the pagination with nothing to draw — until the next message.
func TestALateSubscriberReadsTheModelInForce(t *testing.T) {
	messages := make(chan mvu.Message, 1)
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
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
	await(t, early, func(m Model) bool { return m.selectedFeed == defaultFeedID() }, "the seed")

	messages <- SelectFeed{Feed: "bbc"}
	await(t, early, func(m Model) bool { return m.selectedFeed == "bbc" }, "the model the message left")

	// The late subscriber attaches with the feed already switched.
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
	await(t, late, func(m Model) bool { return m.selectedFeed == "bbc" }, "the model in force at a late subscription")
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
