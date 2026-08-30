package main

import (
	"context"
	"image"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// TestBuildLayersConstructsWithoutPanic verifies that buildLayers returns two
// observable layers and that each emits at least one widget without error.
// The model observable is seeded with the initial model and never receives
// further messages, so the test is deterministic and free of OS timing.
func TestBuildLayersConstructsWithoutPanic(t *testing.T) {
	start := initialModel()
	modelObs := rx.Of(start)
	layers := buildLayers(modelObs, tokens.DefaultSeed)(rx.Of(theme.Default()))
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

// TestInitialModelSeedsDocs verifies that initialModel() produces a model
// with currentPage == pageDocs, the first ## section of the docs outline
// disclosed (so the Docs tab opens with children showing), and no
// selected heading.
func TestInitialModelSeedsDocs(t *testing.T) {
	m := initialModel()
	if m.currentPage != pageDocs {
		t.Errorf("initialModel.currentPage = %q; want %q", m.currentPage, pageDocs)
	}
	if !m.outlineOpen[0] {
		t.Errorf("initialModel.outlineOpen[0] = false; want true (first section seeded open)")
	}
	if m.selectedHeading != -1 {
		t.Errorf("initialModel.selectedHeading = %d; want -1 (nothing selected)", m.selectedHeading)
	}
}

// TestUpdateSetRouteAdvancesPage verifies that a SetRoute message advances
// the model's currentPage field synchronously — no goroutine, no polling.
func TestUpdateSetRouteAdvancesPage(t *testing.T) {
	m := initialModel()
	next, _ := Update(m, SetRoute{Page: pageComponents})
	if next.currentPage != pageComponents {
		t.Errorf("after SetRoute: currentPage = %q; want %q", next.currentPage, pageComponents)
	}
}

// TestTabIndexRoundTrips pins the strip order against the identifiers: a
// click's index maps to a page that maps back to the same index, and an
// unrecognised identifier lands on the Docs tab.
func TestTabIndexRoundTrips(t *testing.T) {
	for i, p := range tabPages {
		if got := tabIndex(p); got != i {
			t.Errorf("tabIndex(%q) = %d; want %d", p, got, i)
		}
	}
	if got := tabIndex("no-such-page"); got != 0 {
		t.Errorf("tabIndex(unknown) = %d; want 0 (the Docs tab)", got)
	}
}

// TestUpdateToggleOutlineIsIndependent verifies the outline reducer:
// sections disclose independently (opening one leaves others open),
// toggling an open section closes just it, and the reducer never mutates
// the map a previous model holds.
func TestUpdateToggleOutlineIsIndependent(t *testing.T) {
	m := initialModel() // section 0 is open
	if !m.outlineOpen[0] {
		t.Fatal("precondition: section 0 must start open")
	}
	before := m
	// Open section 1 — section 0 must stay open (independent disclosure).
	m, _ = Update(m, ToggleOutline{Idx: 1})
	if !m.outlineOpen[1] {
		t.Error("after ToggleOutline(1): section 1 should be open")
	}
	if !m.outlineOpen[0] {
		t.Error("after ToggleOutline(1): section 0 should have stayed open")
	}
	if before.outlineOpen[1] {
		t.Error("Update mutated the previous model's outlineOpen map; the reducer must copy")
	}
	// Toggle the open section 1 again — it closes, section 0 unaffected.
	m, _ = Update(m, ToggleOutline{Idx: 1})
	if m.outlineOpen[1] {
		t.Error("after second ToggleOutline(1): section 1 should be closed")
	}
	if !m.outlineOpen[0] {
		t.Error("after second ToggleOutline(1): section 0 should still be open")
	}
}

// TestUpdateSelectHeading verifies SelectHeading lands the block index in
// the model.
func TestUpdateSelectHeading(t *testing.T) {
	m := initialModel()
	m, _ = Update(m, SelectHeading{Block: 42})
	if m.selectedHeading != 42 {
		t.Errorf("after SelectHeading{42}: selectedHeading = %d; want 42", m.selectedHeading)
	}
}

// TestDocsTabReEmitsOnModelChange guards the same-frame repaint: the docs
// layer observable must re-emit when the model changes, or a click never
// reaches theme/window's Invalidate() and the canvas only repaints on the
// next unrelated input event.
//
// Driving the same modelObs the app uses and asserting docsTabFrom's
// returned observable emits a fresh widget on each ToggleOutline /
// SelectHeading is the seam; a reducer-only test passes without proving the
// layer re-emits. The unit test proves the necessary re-emission, not the OS
// frame timing.
func TestDocsTabReEmitsOnModelChange(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1)
	tab := docsTabFrom(rx.Of(theme.Default()), modelObs, guideFixture(t))

	emissions := make(chan layout.Widget, 16)
	sub := tab.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
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

	// A ToggleOutline-derived model must produce a fresh layer emission.
	m, _ := Update(initialModel(), ToggleOutline{Idx: 1})
	send.Next(m)
	if w := await("ToggleOutline"); w != nil {
		drawOnce(t, image.Pt(docsCanvasW, docsCanvasH), w)
	}
	drainEmissions(emissions)

	// A SelectHeading-derived model (a row click) must also re-emit the layer.
	m, _ = Update(m, SelectHeading{Block: 2})
	send.Next(m)
	if w := await("SelectHeading"); w != nil {
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
	err := obs.Subscribe(context.Background(), func(v layout.Widget, _ error, done bool) {
		if !done && got == nil {
			got = v
		}
	}).Wait()
	return got, err
}

// TestStripOrderIsLabelled pins the two lists the strip is built from
// against each other: every route identifier has a label, in the order
// the cells are drawn.
func TestStripOrderIsLabelled(t *testing.T) {
	if len(tabLabels) != len(tabPages) {
		t.Fatalf("%d labels for %d tabs", len(tabLabels), len(tabPages))
	}
	want := []string{"Docs", "Theme", "Components", "Patterns", "Markdown"}
	for i, w := range want {
		if tabLabels[i] != w {
			t.Errorf("tab %d is labelled %q, want %q", i, tabLabels[i], w)
		}
	}
}
