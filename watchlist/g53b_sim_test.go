// g53b_sim_test.go verifies the G5.3b symbols editor headlessly, against the
// REAL composed shell — the same widget tree the running app renders. Launching
// the Gio app from a shell has no window-server session, so every live
// behaviour is proven here at the pixel level (or, for persistence, at the
// store level — see store_test.go's TestSaveRoundTripPersistsEdits).
//
// Asserted:
//   - a golden of the add/edit modal body (light + dark, with the empty-Symbol
//     alert banner and all four fields) via the static Render paths,
//   - OpenAddSymbol paints the modal scrim over the window,
//   - an empty SubmitSymbol raises the alert band,
//   - a non-empty SubmitSymbol updates the symbols table (a new row appears),
//   - the toast path, twice over since G0C.3: a toast.Requested reduced through
//     Update renders in toast.Stack and a toast.Expired takes it back off, and
//     a toast raised by a COMMAND — off the frame goroutine entirely — arrives
//     the same way.
//
// Verification is HEADLESS throughout: there is no GUI driving; clicks are
// modelled by applying messages to the model directly and asserting rendered
// output, mirroring feeds/g52d_sim_test.go.
package main

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/alert"
	"github.com/vibrantgio/cadence/card"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/golden"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

// modalCanvas is the canvas the modal golden draws into.
const (
	modalCanvasW = 600
	modalCanvasH = 620
)

var modalSharpRadius = tokens.RadiusScale{}

// staticSymbolModalBody assembles the modal Body from the STATIC Render paths of
// the same components the live addSymbolModal composes: a card wrapping the
// error alert (shown to capture the empty-submit state), the four fields, and
// the Save button. Sharp radii + the static Render paths keep the golden
// deterministic. Field placeholders model an edit-mode pre-population (the
// current row's values shown as placeholders — the G5.3b workaround).
func staticSymbolModalBody(shaper *text.Shaper, colors tokens.ColorTokens) layout.Widget {
	body := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(unit.Dp(symGapDp))
		alertH := gtx.Dp(unit.Dp(symAlertHDp))
		fieldH := gtx.Dp(unit.Dp(symFieldHDp))
		btnH := gtx.Dp(unit.Dp(symBtnHDp))
		y := 0
		place := func(wdg layout.Widget, h int) {
			s := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			cg := gtx
			cg.Constraints = layout.Exact(image.Pt(w, h))
			wdg(cg)
			s.Pop()
			y += h + gap
		}

		place(alert.Render(shaper, alert.Props{Variant: alert.Error, Title: "Symbol is required"},
			colors, tokens.Spacing, modalSharpRadius, tokens.DefaultTypography.TitleMedium), alertH)
		for _, ph := range []string{"BTC/USD", "Coinbase", "1h", "Notes"} {
			place(input.Render(shaper, ph, colors, tokens.Spacing, modalSharpRadius,
				tokens.DefaultTypography.BodyLarge, tokens.Comfortable, input.RenderState{}), fieldH)
		}
		place(button.Render(shaper, "Save", colors, tokens.Spacing, modalSharpRadius,
			tokens.DefaultTypography.LabelLarge, tokens.Comfortable, button.RenderState{}), btnH)
		y -= gap
		return layout.Dimensions{Size: image.Pt(w, y)}
	}
	return func(gtx layout.Context) layout.Dimensions {
		c := card.Render(card.Props{Body: body}, colors, tokens.Spacing, modalSharpRadius)
		return c(gtx)
	}
}

// TestSymbolModalGolden renders the add/edit modal (open, with the alert banner
// and all four fields) in light and dark token sets.
func TestSymbolModalGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"symbol-modal-light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"symbol-modal-dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := staticSymbolModalBody(shaper, tc.colors)
			m := modal.Render(shaper, modal.Props{Title: "Symbol", Body: body, Shaper: shaper},
				true, tc.colors, tokens.Spacing, modalSharpRadius,
				tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
			golden.Render(t, tc.name, image.Pt(modalCanvasW, modalCanvasH), scene(m, tc.bg))
		})
	}
}

// scrimRegion samples the centre of the window, where an open modal paints its
// scrim + surface over the shell.
var scrimRegion = image.Rect(shellCanvasW/2-200, shellCanvasH/2-180, shellCanvasW/2+200, shellCanvasH/2+180)

// TestG53bSymbolEditorStatesHeadless renders the real shell at the CRUD model
// states and asserts the pixel-level deltas the G5.3b Measurable describes.
func TestG53bSymbolEditorStatesHeadless(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1, 256)
	storePath := filepath.Join(t.TempDir(), "watchlists.json")
	layer := watchlistShellLayer(rx.Of(theme.Default()), modelObs, storePath)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(shellCanvasW, shellCanvasH)
	snap := func(what string) *image.RGBA {
		w := awaitStableWidget(t, emissions, what)
		img := golden.Capture(t, size, scene(w, bg))
		return img
	}

	m := initialModel(testDoc()) // "majors" selected, two symbols
	send.Next(m)
	closed := snap("initial model")

	// OpenAddSymbol paints the modal scrim + surface over the whole window.
	m, _ = Update(m, OpenAddSymbol{})
	send.Next(m)
	modalOpen := snap("OpenAddSymbol")
	if n := regionDiff(closed, modalOpen, scrimRegion); n <= 0 {
		t.Errorf("window unchanged after OpenAddSymbol (diff=%d in scrim region); modal did not open", n)
	}

	// Empty SubmitSymbol raises the alert band inside the modal.
	m, _ = Update(m, SubmitSymbol{Symbol: ""})
	send.Next(m)
	withAlert := snap("SubmitSymbol(empty)")
	if !m.modalError {
		t.Fatal("empty submit did not set modalError")
	}
	if n := regionDiff(modalOpen, withAlert, scrimRegion); n <= 0 {
		t.Errorf("modal unchanged after empty submit (diff=%d); alert did not appear", n)
	}

	// A non-empty SubmitSymbol appends a row and closes the modal — the Main
	// table now shows the new symbol, so the Main region changes.
	m, _ = Update(m, SubmitSymbol{Symbol: "AVAX/USD", Exchange: "Binance"})
	send.Next(m)
	added := snap("SubmitSymbol(non-empty)")
	wl, _ := m.selectedWatchlist()
	if len(wl.Symbols) != 3 || wl.Symbols[2].Symbol != "AVAX/USD" {
		t.Fatalf("non-empty submit did not append the symbol: %+v", wl.Symbols)
	}
	if n := regionDiff(closed, added, mainRegion); n <= 0 {
		t.Errorf("Main table unchanged after add (diff=%d); new row did not appear", n)
	}

	// Edit row 0 in place: reopen pre-populated, change the exchange, submit.
	m, _ = Update(m, OpenEditSymbol{Row: 0})
	send.Next(m)
	_ = snap("OpenEditSymbol(0)")
	if m.editIndex != 0 || m.editSeed.Symbol != "BTC/USD" {
		t.Fatalf("OpenEditSymbol did not seed the edit target: editIndex=%d seed=%+v", m.editIndex, m.editSeed)
	}
	m, _ = Update(m, SubmitSymbol{Symbol: "BTC/USD", Exchange: "Kraken", Timeframe: "1h"})
	send.Next(m)
	edited := snap("SubmitSymbol(edit)")
	wl, _ = m.selectedWatchlist()
	if wl.Symbols[0].Exchange != "Kraken" {
		t.Fatalf("edit did not apply: %+v", wl.Symbols[0])
	}
	if n := regionDiff(added, edited, mainRegion); n <= 0 {
		t.Errorf("Main table unchanged after edit (diff=%d); edited row did not update", n)
	}
}

// toastCanvas is the canvas the two toast tests below capture.
var toastCanvas = image.Pt(600, 300)

// toastBG is a flat pane the toast surface has to separate from.
var toastBG = color.NRGBA{R: 240, G: 240, B: 240, A: 255}

// toastStackOf subscribes a live toast.Stack whose queue comes from modelObs,
// exactly as watchlistShellLayer composes it, and hands back its emissions.
func toastStackOf(t *testing.T, modelObs rx.Observable[Model]) (<-chan layout.Widget, func()) {
	t.Helper()
	stackObs := toast.Stack(rx.Of(theme.Default()), toast.Props{
		Position: toast.TopRight,
		Toasts:   rx.Map(modelObs, func(m Model) []toast.Toast { return m.toasts.Items() }),
		Shaper:   tokens.DefaultTypography.DeterministicShaper(),
	})
	emissions := make(chan layout.Widget, 32)
	sub := stackObs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	return emissions, sub.Unsubscribe
}

// emptyToastFrame is what the stack paints for a model with no toasts: the
// baseline both tests below diff against. It is captured from its own
// subscription so it cannot race with whatever the test under way is doing.
func emptyToastFrame(t *testing.T) *image.RGBA {
	t.Helper()
	send, modelObs := rx.Subject[Model](0, 1, 4)
	emissions, stop := toastStackOf(t, modelObs)
	defer stop()
	send.Next(initialModel(testDoc()))
	return golden.Capture(t, toastCanvas, scene(awaitStableWidget(t, emissions, "empty queue"), toastBG))
}

// waitForToastFrame drains the stack's emissions until one paints something
// an empty queue does not. Written as a poll rather than "settle, then
// capture" because the toast in the command test arrives on a command
// goroutine at a moment no test controls.
func waitForToastFrame(t *testing.T, emissions <-chan layout.Widget, empty *image.RGBA) *image.RGBA {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case w := <-emissions:
			if w == nil {
				continue
			}
			if img := golden.Capture(t, toastCanvas, scene(w, toastBG)); golden.PixelDiff(empty, img) > 0 {
				return img
			}
		case <-deadline:
			t.Fatal("the toast stack never painted anything an empty queue would not")
			return nil
		}
	}
}

// TestToastRequestRendersInStack drives the toast through Update, which is the
// whole of G0C.3. Before it, toast.Notify published to a process-global
// rx.Subject fired from the addSymbolModal submit callback: the toast was on
// screen and in no model, so a test that applied messages could not produce
// one and this test had to reach for the side channel instead. Now the request
// is a message, the queue is model state, and the expiry is a message back —
// so the canvas returns to empty through Update as well.
func TestToastRequestRendersInStack(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1, 16)
	emissions, stop := toastStackOf(t, modelObs)
	defer stop()
	snap := func(what string) *image.RGBA {
		t.Helper()
		return golden.Capture(t, toastCanvas, scene(awaitStableWidget(t, emissions, what), toastBG))
	}

	m := initialModel(testDoc())
	send.Next(m)
	before := snap("seeded empty stack")

	// The exact message the five confirm/submit callbacks land via toast.Notify.
	m, _ = Update(m, toast.Requested{Level: toast.Success, Text: "Saved", At: time.Now()})
	if m.toasts.Len() != 1 {
		t.Fatalf("model queue length = %d after toast.Requested; want 1", m.toasts.Len())
	}
	send.Next(m)
	if n := golden.PixelDiff(before, snap("toast.Requested")); n <= 0 {
		t.Errorf("stack frame unchanged after toast.Requested (diff=%d); toast did not render", n)
	}

	m, _ = Update(m, toast.Expired{ID: m.toasts.Items()[0].ID})
	if m.toasts.Len() != 0 {
		t.Fatalf("model queue length = %d after toast.Expired; want 0", m.toasts.Len())
	}
	send.Next(m)
	if n := golden.PixelDiff(before, snap("toast.Expired")); n != 0 {
		t.Errorf("stack frame differs from empty after toast.Expired (diff=%d); toast did not leave", n)
	}
}

// TestToastFromACommandGoroutineArrives is the point of the conversion: a
// toast raised off the frame goroutine, by a command, arrives on screen — and
// the caller never thinks about goroutines, because command → message →
// Update is the loop's own path.
//
// The command is the row delete done asynchronously, built out of this app's
// own deleteSymbolAt/documentOf/saveStore. The production delete still writes
// synchronously in the confirm callback by the decision logged in
// FEEDBACK-G5.3.md and in model.go's header; what this proves is that the
// decision is now the only thing holding it there. Under the Subject that was
// not true: a command goroutine calling the old Notify published into
// cadence's process-global bus, which is precisely the cross-goroutine hazard
// a single-goroutine renderer cannot otherwise express (ADR-008).
//
// Everything below is real: mvu.Loop, this app's Update, a real disk write on
// a command goroutine, a real toast.Stack rendering the model's queue.
func TestToastFromACommandGoroutineArrives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	doc := testDoc()
	if err := saveStore(path, doc); err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	// The delete, as a command. It runs on the runner's goroutine, has no gtx
	// and no callback, and says what happened by returning a message.
	deleteRow := func(m Model, row int) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) {
			next := deleteSymbolAt(m.watchlists, m.selected, row)
			if err := saveStore(path, documentOf(next, m.selected)); err != nil {
				return toast.Request(toast.Error, "Delete failed"), nil
			}
			return toast.Request(toast.Success, "Symbol deleted"), nil
		})
	}

	// The baseline comes from its own subscription: the command below fires
	// the moment the loop starts, so there is no "before" to capture from the
	// loop's own stack.
	empty := emptyToastFrame(t)

	seed := initialModel(doc)
	msgCh := make(chan mvu.Message, 16)
	init := func() (Model, mvu.Command) { return seed, deleteRow(seed, 0) }
	// The runner is deliberately leaked, as in wiring_test.go: unsubscribing
	// the rx chain races inside reactivego/rx itself, and everything asserted
	// here happens before any teardown.
	models, _ := mvu.Loop(rx.Recv(msgCh), init, Update)
	modelObs := models.Publish().AutoConnect(2)

	queues := make(chan toast.Queue, 32)
	_ = rx.Map(modelObs, func(m Model) toast.Queue { return m.toasts }).
		Subscribe(rx.GoroutineContext(), func(q toast.Queue, _ error, done bool) {
			if !done {
				select {
				case queues <- q:
				default:
				}
			}
		})
	emissions, stop := toastStackOf(t, modelObs)
	defer stop()

	deadline := time.After(5 * time.Second)
	var queued toast.Toast
	for queued.ID == 0 {
		select {
		case q := <-queues:
			if q.Len() > 0 {
				queued = q.Items()[0]
			}
		case <-deadline:
			t.Fatal("no toast reached the model within 5s; the command → message → Update path is broken")
		}
	}
	if queued.Text != "Symbol deleted" {
		t.Errorf("queued toast text = %q; want the command's success message", queued.Text)
	}
	if queued.At.IsZero() {
		t.Error("toast.Request left At zero; a command-raised toast would never fade")
	}
	waitForToastFrame(t, emissions, empty)

	// The write really happened, on that same goroutine, before the message.
	saved, err := loadStore(path)
	if err != nil {
		t.Fatalf("reading back the store: %v", err)
	}
	if got := len(saved.Watchlists[0].Symbols); got != len(doc.Watchlists[0].Symbols)-1 {
		t.Errorf("saved symbol count = %d; want %d — the command's delete did not land",
			got, len(doc.Watchlists[0].Symbols)-1)
	}
}
