package main

import (
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
)

// TestModelObsConsumerCountMatchesConst counts the cold subscriptions the
// window's layers make to the model stream when they are subscribed once, the
// way theme/window subscribes them, and asserts the count is the one
// [modelObsConsumers] declares.
//
// rx.Publish() does not replay, so Publish().AutoConnect(N) lets the seed flow
// only when the Nth subscriber attaches. A constant lower than the real count
// leaves the late subscribers without the seed the loop emitted; one higher
// leaves the window waiting for a subscriber that never comes, which is a
// window that never draws. Neither is something a reducer test can see, and
// the count moved the moment the embedded page grew a tab strip with a palette
// and a selection of its own.
func TestModelObsConsumerCountMatchesConst(t *testing.T) {
	base := rx.Of(judging()) // cold; replays the seed to each subscription
	var n int32
	counting := rx.Observable[Model](func(observe rx.Observer[Model], sched rx.Scheduler, sub rx.Subscriber) {
		atomic.AddInt32(&n, 1)
		base(observe, sched, sub)
	})

	var got int32
	for _, layer := range buildLayers(counting, &desktop.ZoneGroup{})(rx.Of(theme.Default())) {
		sub := layer.Subscribe(rx.GoroutineContext(), func(layout.Widget, error, bool) {})
		defer sub.Unsubscribe()
	}
	// The graph attaches asynchronously on rx.Goroutine, so the count is
	// polled until it settles rather than read once.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got = atomic.LoadInt32(&n); int(got) == modelObsConsumers {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if int(got) != modelObsConsumers {
		t.Fatalf("the layers subscribe the model %d times and modelObsConsumers says %d — AutoConnect(N) needs N to be the real count exactly, so the constant and its comment want updating to %d",
			got, modelObsConsumers, got)
	}
}

// TestTheContentLayerDraws: the page's layer is the model, the theme and the
// tab strip's own stream combined, and a combination that never emits is a
// window with nothing in it. What is asserted is that a widget arrives at all
// — what it draws is asserted everywhere else in this package, on pixels.
func TestTheContentLayerDraws(t *testing.T) {
	got := make(chan layout.Widget, 1)
	sub := ContentLayer(rx.Of(theme.Default()), rx.Of(judging()), &desktop.ZoneGroup{}).
		Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
			if done || err != nil || w == nil {
				return
			}
			select {
			case got <- w:
			default:
			}
		})
	defer sub.Unsubscribe()
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("the content layer emitted no widget — the window would open on an empty page")
	}
}
