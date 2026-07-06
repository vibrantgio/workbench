package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/prism/theme"
)

// TestBuildLayersConstructsWithoutPanic verifies that buildLayers returns two
// observable layers and that each emits at least one widget without error.
// The model observable is seeded with the initial model and never receives
// further messages, so the test is deterministic and free of OS timing.
func TestBuildLayersConstructsWithoutPanic(t *testing.T) {
	seed := initialModel()
	modelObs := rx.Of(seed)
	layers := buildLayers(modelObs)(rx.Of(theme.Default()))
	if len(layers) != 2 {
		t.Fatalf("buildLayers returned %d layers; want 2 (backdrop, shell)", len(layers))
	}
	for i, layer := range layers {
		got, err := collectOne(layer)
		if err != nil {
			t.Errorf("layer %d subscribe: %v", i, err)
			continue
		}
		if got == nil {
			t.Errorf("layer %d produced no widget", i)
		}
	}
}

// TestInitialModelSeedsHome verifies that initialModel() produces a model
// with currentPage == pageHome and at least section 0 open — the same
// invariants the former rx.Subject-based controllers guaranteed.
func TestInitialModelSeedsHome(t *testing.T) {
	m := initialModel()
	if m.currentPage != pageHome {
		t.Errorf("initialModel.currentPage = %q; want %q", m.currentPage, pageHome)
	}
	if !m.openSections[0] {
		t.Errorf("initialModel.openSections[0] = false; want true (first section seeded open)")
	}
}

// TestUpdateSetRouteAdvancesPage verifies that a SetRoute message advances
// the model's currentPage field synchronously — no goroutine, no polling.
func TestUpdateSetRouteAdvancesPage(t *testing.T) {
	m := initialModel()
	next, _ := Update(m, SetRoute{Page: pageDocsGettingStarted})
	if next.currentPage != pageDocsGettingStarted {
		t.Errorf("after SetRoute: currentPage = %q; want %q", next.currentPage, pageDocsGettingStarted)
	}
}

// TestUpdateToggleAccordionSingleOpen verifies the single-open reducer policy:
// opening a section replaces the open set with just that index, and clicking
// the already-open section collapses it.
func TestUpdateToggleAccordionSingleOpen(t *testing.T) {
	m := initialModel() // section 0 is open
	if !m.openSections[0] {
		t.Fatal("precondition: section 0 must start open")
	}
	// Open section 1 — section 0 must close (single-open).
	m, _ = Update(m, ToggleAccordion{Idx: 1})
	if !m.openSections[1] {
		t.Error("after ToggleAccordion(1): section 1 should be open")
	}
	if m.openSections[0] {
		t.Error("after ToggleAccordion(1): section 0 should have closed (single-open)")
	}
	// Click the open section 1 again — it collapses, leaving nothing open.
	m, _ = Update(m, ToggleAccordion{Idx: 1})
	if m.openSections[1] {
		t.Error("after second ToggleAccordion(1): section 1 should be closed")
	}
	if len(m.openSections) != 0 {
		t.Errorf("expected all sections closed; got %v", m.openSections)
	}
}

// TestDocsShellLayerReEmitsOnModelChange is the GX.9 same-frame-repaint
// regression test. The bug it guards against: the shell layer observable did
// not re-emit when the model changed (accordion state and routing were shunted
// into atomic mirrors disconnected from the layer chain), so a click never
// reached spectrum/window's Invalidate() and the canvas only repainted on the
// next unrelated input event (FEEDBACK-G5.1).
//
// Driving the same modelObs the app uses and asserting docsShellLayer's
// returned observable emits a fresh widget on each ToggleAccordion / SetRoute
// is the seam the bug lived on; a reducer-only test passes without proving the
// layer re-emits. (Live same-frame repaint is confirmed by running the app —
// the unit test proves the necessary re-emission, not the OS frame timing.)
func TestDocsShellLayerReEmitsOnModelChange(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))

	send, modelObs := rx.Subject[Model](0, 1)
	shell := docsShellLayer(rx.Of(theme.Default()), shaper, modelObs)

	emissions := make(chan layout.Widget, 16)
	sub := shell.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	}, rx.Goroutine)
	defer sub.Unsubscribe()

	await := func(what string) layout.Widget {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			select {
			case w := <-emissions:
				return w
			case <-time.After(10 * time.Millisecond):
			}
		}
		t.Fatalf("shell layer did not re-emit after %s", what)
		return nil
	}

	send.Next(initialModel())
	if w := await("initial model"); w != nil {
		drawOnce(t, image.Pt(docsCanvasW, docsCanvasH), w)
	}
	drainEmissions(emissions)

	// A ToggleAccordion-derived model must produce a fresh layer emission.
	m, _ := Update(initialModel(), ToggleAccordion{Idx: 1})
	send.Next(m)
	if w := await("ToggleAccordion"); w != nil {
		drawOnce(t, image.Pt(docsCanvasW, docsCanvasH), w)
	}
	drainEmissions(emissions)

	// A SetRoute-derived model (navigation) must also re-emit the layer.
	m, _ = Update(m, SetRoute{Page: pageDocsGettingStarted})
	send.Next(m)
	if w := await("SetRoute"); w != nil {
		drawOnce(t, image.Pt(docsCanvasW, docsCanvasH), w)
	}
}

func drainEmissions(ch chan layout.Widget) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// collectOne subscribes to obs, returns the first emitted value (if any)
// and the subscription's terminal error.
func collectOne(obs rx.Observable[layout.Widget]) (layout.Widget, error) {
	var got layout.Widget
	sched := rx.NewScheduler()
	err := obs.Subscribe(func(v layout.Widget, _ error, done bool) {
		if !done && got == nil {
			got = v
		}
	}, sched).Wait()
	return got, err
}
