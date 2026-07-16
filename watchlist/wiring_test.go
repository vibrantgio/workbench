package main

import (
	"image"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
)

// testDoc is a richer-than-starter document (two watchlists) so selection can
// actually be exercised — the starter's single watchlist cannot demonstrate a
// Main change on SelectWatchlist.
func testDoc() Document {
	return Document{
		Version:  formatVersion,
		Selected: "majors",
		Watchlists: []Watchlist{
			{Name: "majors", Symbols: []Symbol{{Symbol: "BTC/USD"}, {Symbol: "ETH/USD"}}},
			{Name: "alts", Symbols: []Symbol{{Symbol: "SOL/USD"}, {Symbol: "AVAX/USD"}}},
		},
	}
}

const (
	shellCanvasW = 1100
	shellCanvasH = 760
)

// TestModelObsConsumerCountMatchesConst measures the EXACT number of cold
// subscriptions watchlistShellLayer makes to modelObs when subscribed once (as
// spectrum/window does) and asserts it equals modelObsConsumers.
// Publish().AutoConnect(N) connects the upstream — and lets the seed flow —
// only when the N-th subscriber attaches; if the count drifts from the wiring,
// late consumers miss the seed (too low) or the app freezes (too high).
func TestModelObsConsumerCountMatchesConst(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))

	base := rx.Of(initialModel(testDoc())) // cold; replays the seed to each subscription
	var n int32
	counting := rx.Observable[Model](func(observe rx.Observer[Model], sched rx.Scheduler, sub rx.Subscriber) {
		atomic.AddInt32(&n, 1)
		base(observe, sched, sub)
	})

	layer := watchlistShellLayer(rx.Of(theme.Default()), shaper, counting, filepath.Join(t.TempDir(), "watchlists.json"))
	sub := layer.Subscribe(func(layout.Widget, error, bool) {}, rx.Goroutine)
	defer sub.Unsubscribe()

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

// TestRealAutoConnectPathDeliversSeedAndReEmits exercises the production seam
// run() builds — mvu.Loop(messages) → Publish().AutoConnect(
// modelObsConsumers) → watchlistShellLayer — proving the seed reaches every
// consumer (N not too low, no blank launch; not too high, no freeze) and that a
// message through the real channel re-emits the layer (same-frame repaint).
func TestRealAutoConnectPathDeliversSeedAndReEmits(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))

	msgCh := make(chan mvu.Message, 16)
	messages := rx.Recv(msgCh)

	init := func() (Model, mvu.Command) { return initialModel(testDoc()), mvu.DoNothing() }
	// The command runner is deliberately leaked along with the layer
	// subscription below (see the teardown note).
	models, _ := mvu.Loop(messages, init, Update)
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	layer := watchlistShellLayer(rx.Of(theme.Default()), shaper, modelObs, filepath.Join(t.TempDir(), "watchlists.json"))

	emissions := make(chan layout.Widget, 32)
	// No teardown: unsubscribing the AutoConnect chain trips a known
	// reactivego/rx multicast unsubscribe-path race; seed-delivery + re-emit
	// both happen before any teardown, so letting the chain leak keeps the test
	// -race-clean (same rationale as feeds/wiring_test.go).
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
		t.Fatalf("layer did not emit after %s (AutoConnect N likely wrong)", what)
		return nil
	}

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

	msgCh <- SelectWatchlist{Name: "alts"}
	if w := await("SelectWatchlist via real channel"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
}

// drawShellOnce lays a widget out once on a fresh op buffer so a re-emitted
// shell widget is exercised through its full layout path (catching panics in
// the composed sidebar/navbar/Main) without requiring a GPU.
func drawShellOnce(t *testing.T, size image.Point, w layout.Widget) {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)
}
