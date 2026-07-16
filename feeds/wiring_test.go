package main

import (
	"image"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
)

// TestModelObsConsumerCountMatchesConst measures the EXACT number of cold
// subscriptions feedsShellLayer makes to modelObs when subscribed once (as
// spectrum/window does) and asserts it equals modelObsConsumers. rx.Publish()
// does not replay, so Publish().AutoConnect(modelObsConsumers) connects the
// upstream — and lets the seed flow — only when the count-th subscriber
// attaches; if this count drifts from the wiring, late consumers miss the seed
// on launch (too low) or the app freezes (too high). A reducer-only test cannot
// catch that. This guards the load-bearing N against future topology edits.
func TestModelObsConsumerCountMatchesConst(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))

	base := rx.Of(initialModel()) // cold; replays the seed to each subscription
	var n int32
	counting := rx.Observable[Model](func(observe rx.Observer[Model], sched rx.Scheduler, sub rx.Subscriber) {
		atomic.AddInt32(&n, 1)
		base(observe, sched, sub)
	})

	layer := feedsShellLayer(rx.Of(theme.Default()), shaper, counting)
	sub := layer.Subscribe(func(layout.Widget, error, bool) {}, rx.Goroutine)
	defer sub.Unsubscribe()

	// Poll until the count stabilises (the graph attaches asynchronously on
	// rx.Goroutine), then assert it matches the declared constant.
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

// TestRealAutoConnectPathDeliversSeedAndReEmits exercises the PRODUCTION seam
// that run() builds — mvu.Loop(messages) → Publish().AutoConnect(
// modelObsConsumers) → feedsShellLayer — rather than the rx.Subject shortcut the
// re-emission test uses. It proves two things the recipe calls out:
//  1. the layer emits a (seed-derived) widget once all consumers have attached
//     and Connect has fired — i.e. modelObsConsumers is not too high (no freeze)
//     and the seed actually reaches every consumer (not too low);
//  2. a message pushed through the real message channel re-emits the layer with
//     the updated model — the same-frame repaint driver.
func TestRealAutoConnectPathDeliversSeedAndReEmits(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))

	// Mirror mvuWin.Messages(): a buffered channel drained by rx.Recv. This is
	// exactly the source run() scans over.
	msgCh := make(chan mvu.Message, 16)
	messages := rx.Recv(msgCh)

	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	// The command runner is deliberately leaked along with the layer
	// subscription below (see the teardown note).
	models, _ := mvu.Loop(messages, init, Update)
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	layer := feedsShellLayer(rx.Of(theme.Default()), shaper, modelObs)

	emissions := make(chan layout.Widget, 32)
	// No defer sub.Unsubscribe()/close(msgCh): unsubscribing the AutoConnect
	// chain triggers rx's Multicast.remove() on the unsubscribe path, which
	// mutates its channel slice without holding the lock the observer send loop
	// holds — a race INSIDE reactivego/rx (multicast.go:68/100 vs :28), not in
	// this app. This test asserts seed-delivery + re-emit, both of which happen
	// before any teardown; letting the chain leak (it parks on a local channel,
	// touches no shared state) keeps the test -race-clean. Steady-state message
	// delivery is race-free; production only unsubscribes at DestroyEvent.
	_ = layer.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	}, rx.Goroutine)

	await := func(what string) layout.Widget {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case w := <-emissions:
				return w
			case <-time.After(10 * time.Millisecond):
			}
		}
		t.Fatalf("layer did not emit after %s (AutoConnect N likely wrong: "+
			"Connect never fired or the seed was missed)", what)
		return nil
	}

	// 1. Seed must reach every consumer so the layer renders on launch.
	if w := await("seed (AutoConnect Connect + StartWith)"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	for {
		select {
		case <-emissions:
			continue
		default:
		}
		break
	}

	// 2. A real message through the channel must re-emit the layer.
	msgCh <- SelectFeed{Feed: "bbc"}
	if w := await("SelectFeed via real channel"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
}
