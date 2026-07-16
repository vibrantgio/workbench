package main

import (
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/prism/theme"
)

// TestModelObsConsumerCountMatchesConst measures the EXACT number of cold
// subscriptions the layers make to modelObs when subscribed once (as
// spectrum/window does) and asserts it equals modelObsConsumers.
// Publish().AutoConnect(N) connects the upstream — and lets the seed flow —
// only when the N-th subscriber attaches; if the count drifts from the
// wiring, late consumers miss the seed (too low) or the app freezes (too
// high).
func TestModelObsConsumerCountMatchesConst(t *testing.T) {
	base := rx.Of(Model{}) // cold; replays the seed to each subscription
	var n int32
	counting := rx.Observable[Model](func(observe rx.Observer[Model], sched rx.Scheduler, sub rx.Subscriber) {
		atomic.AddInt32(&n, 1)
		base(observe, sched, sub)
	})

	layers := buildLayers(counting)(rx.Of(theme.Default()))
	subs := make([]rx.Subscription, 0, len(layers))
	for _, layer := range layers {
		subs = append(subs, layer.Subscribe(func(layout.Widget, error, bool) {}, rx.Goroutine))
	}
	defer func() {
		for _, sub := range subs {
			sub.Unsubscribe()
		}
	}()

	deadline := time.Now().Add(time.Second)
	var got int32
	for time.Now().Before(deadline) {
		got = atomic.LoadInt32(&n)
		if int(got) == modelObsConsumers {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if int(got) != modelObsConsumers {
		t.Fatalf("modelObs cold subscription count = %d; modelObsConsumers = %d. "+
			"AutoConnect(N) needs N to equal the real count exactly — update the "+
			"constant (and its comment) to %d.", got, modelObsConsumers, got)
	}
}
