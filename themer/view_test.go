package main

import (
	"image"
	stdcolor "image/color"
	"testing"

	"gioui.org/gesture"
	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// The window renders are captured, never stored: what is asserted is that
// the swatches carry the candidates' own colours and that the chosen one is
// marked, and both are pixel facts a stored image would only illustrate.

// pinned builds the application's typography with the default faces and
// nothing else, system fonts off, so a render here cannot depend on the
// machine's font set.
func pinned() Type {
	ty := TypeFrom(tokens.DefaultTypography)
	ty.Shaper = tokens.DefaultTypography.DeterministicShaper()
	return ty
}

// page renders the whole window for a model and returns the capture.
func page(t *testing.T, m Model, os tokens.ColorTokens) *image.RGBA {
	t.Helper()
	clicks := make([]gesture.Click, imageseed.DefaultMax)
	size := image.Pt(windowW, windowH)
	widget := Page(themed{os: os, typ: pinned()}, m, &desktop.ZoneGroup{}, clicks)
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		// The backdrop is its own layer at runtime; here it is one fill
		// under the page, resolved the same way that layer resolves it.
		fill(gtx, size, SchemeFor(os, m).Background)
		return widget(gtx)
	})
}

func fill(gtx layout.Context, size image.Point, c stdcolor.NRGBA) {
	fillRRect(gtx, image.Rectangle{Max: size}, 0, c)
}

// dropped is the model after a picture has landed: a real extraction of a
// painted scene, so the swatches under test are the ones the pipeline
// actually produces.
func dropped(t *testing.T) Model {
	t.Helper()
	img := scene(480, 360)
	candidates := imageseed.Extract(img)
	if len(candidates) < 3 {
		t.Fatalf("the fixture scene yielded %d candidates, want a row to test", len(candidates))
	}
	return Model{Preview: preview(img), Name: "scene.png", Candidates: candidates}
}

// cardTop is the y of the candidate cards' top edge, cardW the width they
// share out between them, and cardX the left edge of card i — all computed
// from the same constants and the same arithmetic the row lays out with. The
// row sits at the bottom of the page, inside the window margin.
func cardTop() int {
	rowH := int(RowLabelH) + int(RowTop) + int(CellH)
	return windowH - int(Pad) - rowH + int(RowLabelH) + int(RowTop)
}

func cardW(n int) int { return cellWidth(windowW-2*int(Pad), int(CellGap), n) }

func cardX(n, i int) int { return int(Pad) + i*(cardW(n)+int(CellGap)) }

// cellCentre is the middle of candidate i's colour swatch, in a row of n.
func cellCentre(n, i int) image.Point {
	return image.Pt(cardX(n, i)+cardW(n)/2, cardTop()+int(CellPad)+int(SwatchH)/2)
}

// TestRowFillsItsWidth: a full row reaches the same right margin as the
// page above it, rather than stopping short of it with dead space wide
// enough to read as a card that failed to load.
func TestRowFillsItsWidth(t *testing.T) {
	n := len(dropped(t).Candidates)
	right := cardX(n, n-1) + cardW(n)
	if slack := (windowW - int(Pad)) - right; slack > int(CellGap) {
		t.Errorf("a row of %d cards ends %d dp short of the %d dp margin", n, slack, int(Pad))
	}
}

// TestSwatchesCarryTheCandidateColours: every candidate in the row is drawn
// as its own colour. A row of swatches that were not the extracted colours
// would be a convincing picture of nothing.
func TestSwatchesCarryTheCandidateColours(t *testing.T) {
	m := dropped(t)
	img := page(t, m, tokens.DefaultLight)
	n := len(m.Candidates)
	for i, c := range m.Candidates {
		p := cellCentre(n, i)
		if p.X+cardW(n)/2 > windowW-int(Pad) {
			break // a swatch the window is too narrow for is not drawn
		}
		got := img.RGBAAt(p.X, p.Y)
		if got.R != c.Color.R || got.G != c.Color.G || got.B != c.Color.B {
			t.Errorf("swatch %d at %v drew %v, want the candidate's %v", i, p, got, c.Color)
		}
	}
}

// TestChosenCandidateIsMarked: exactly one card carries the ring, and it is
// the chosen one. Asserted within each render rather than between two, so a
// window that re-themes wholesale on every choice cannot pass by accident.
func TestChosenCandidateIsMarked(t *testing.T) {
	m := dropped(t)
	n := len(m.Candidates)
	edge := func(img *image.RGBA, i int) stdcolor.RGBA {
		return img.RGBAAt(cardX(n, i)+cardW(n)/2, cardTop()+int(Ring)/2)
	}
	for _, chosen := range []int{0, 1} {
		img := page(t, ReduceModel(m, SelectCandidate{Index: chosen}), tokens.DefaultLight)
		other := 1 - chosen
		if edge(img, chosen) == edge(img, other) {
			t.Errorf("with candidate %d chosen, its card edge matches card %d's — nothing marks the choice", chosen, other)
		}
		if edge(img, other) != edge(img, 2) {
			t.Errorf("with candidate %d chosen, card %d's edge differs from card 2's — more than one card is marked", chosen, other)
		}
	}
}

// TestEmptyWindowInvites: with nothing dropped, the window is not blank —
// it says what to do, and the drop well is drawn.
func TestEmptyWindowInvites(t *testing.T) {
	empty := page(t, Model{}, tokens.DefaultLight)
	blank := golden.Capture(t, image.Pt(windowW, windowH), func(gtx layout.Context) layout.Dimensions {
		fill(gtx, image.Pt(windowW, windowH), tokens.DefaultLight.Background)
		return layout.Dimensions{Size: image.Pt(windowW, windowH)}
	})
	if golden.PixelDiff(empty, blank) == 0 {
		t.Error("the empty window drew nothing at all")
	}
}

// TestDragHighlightIsVisible: while a drag hovers, the window looks
// different — the affordance is what tells the user the drop will land.
func TestDragHighlightIsVisible(t *testing.T) {
	m := Model{}
	rest := page(t, m, tokens.DefaultLight)
	over := page(t, ReduceModel(m, desktop.FilesEntered{Zone: dropZone}), tokens.DefaultLight)
	if golden.PixelDiff(rest, over) == 0 {
		t.Error("a drag over the window changed no pixel")
	}
}

// TestBothSchemesRender: the page draws in the dark palette as well as the
// light one, and the two differ.
func TestBothSchemesRender(t *testing.T) {
	m := dropped(t)
	light := page(t, m, tokens.DefaultLight)
	dark := page(t, m, tokens.DefaultDark)
	if golden.PixelDiff(light, dark) == 0 {
		t.Error("the dark scheme rendered identically to the light one")
	}
}
