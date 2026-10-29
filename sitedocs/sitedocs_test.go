package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
)

// TestBuildLayersConstructsWithoutPanic verifies that buildLayers returns two
// observable layers and that each emits at least one layout.Widget without
// error. The model observable is seeded with the initial model and never
// receives further messages, so the test is deterministic and free of OS
// timing.
func TestBuildLayersConstructsWithoutPanic(t *testing.T) {
	start := initialModel()
	modelObs := rx.Of(start)
	layers := buildLayers(modelObs, desktop.ScrollBarsWhenScrolling)(rx.Of(theme.Default()))
	if len(layers) != 2 {
		t.Fatalf("buildLayers returned %d layers; want 2 (backdrop, page)", len(layers))
	}
	for i, layer := range layers {
		got, err := golden.Resolve(layer)
		if err != nil {
			t.Errorf("layer %d subscribe: %v", i, err)
			continue
		}
		if got == nil {
			t.Errorf("layer %d produced no layout.Widget", i)
		}
	}
}

// TestInitialModelOpensOnDocs verifies that initialModel() produces a model
// with the first ## section of the docs outline disclosed (so the page opens
// with children showing), and no selected heading.
func TestInitialModelOpensOnDocs(t *testing.T) {
	m := initialModel()
	if !m.outlineOpen[0] {
		t.Errorf("initialModel.outlineOpen[0] = false; want true (first section opens disclosed)")
	}
	if m.selectedHeading != -1 {
		t.Errorf("initialModel.selectedHeading = %d; want -1 (nothing selected)", m.selectedHeading)
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

// TestDocsPageReEmitsOnModelChange guards the same-frame repaint: the docs
// layer observable must re-emit when the model changes, or a click never
// reaches theme/window's Invalidate() and the window only repaints on the
// next unrelated input event.
//
// Driving the same modelObs the app uses and asserting docsWindow's
// returned observable emits a fresh layout.Widget on each ToggleOutline /
// SelectHeading is the seam; a reducer-only test passes without proving the
// layer re-emits. The unit test proves the necessary re-emission, not the OS
// frame timing.
func TestDocsPageReEmitsOnModelChange(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1)
	post := func(layout.Context, mvu.Message) {}
	page, _, _ := docsWindow(rx.Of(theme.Default()), modelObs, guideFixture(t), nil, desktop.ScrollBarsWhenScrolling, post)

	emissions := make(chan layout.Widget, 16)
	sub := page.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
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
		t.Fatalf("page layer did not re-emit after %s", what)
		return nil
	}

	send.Next(initialModel())
	if w := await("initial model"); w != nil {
		drawOnce(t, image.Pt(docsFrameW, docsFrameH), w)
	}
	drainEmissions(emissions)

	// A ToggleOutline-derived model must produce a fresh layer emission.
	m, _ := Update(initialModel(), ToggleOutline{Idx: 1})
	send.Next(m)
	if w := await("ToggleOutline"); w != nil {
		drawOnce(t, image.Pt(docsFrameW, docsFrameH), w)
	}
	drainEmissions(emissions)

	// A SelectHeading-derived model (a row click) must also re-emit the layer.
	m, _ = Update(m, SelectHeading{Block: 2})
	send.Next(m)
	if w := await("SelectHeading"); w != nil {
		drawOnce(t, image.Pt(docsFrameW, docsFrameH), w)
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
