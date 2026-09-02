package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/theme/tokens"
)

// TestOnlyAFollowedLinkArmsTheArrival walks every way a note reaches the
// screen and asks which of them is an arrival: the vault opening on a note
// is not, a link followed to one is, and a note rebuilt under the reader —
// a task box written — is a re-render of the arrival already made and must
// not make a second.
func TestOnlyAFollowedLinkArmsTheArrival(t *testing.T) {
	model := scannedModel(t)
	model.Arrival = 0
	idx := treeIndex("x.md", "f.md")
	opened, _ := Update(model, vaultScanned{vault: "/v", index: idx, note: model.Notes["x.md"]})
	if opened.Arrival != 0 {
		t.Errorf("the note the vault opened on armed the marking (Arrival=%d); it was not sought", opened.Arrival)
	}

	opened = cacheNote(opened, fNote())
	followed, _ := Update(opened, Navigate{Path: "f.md", Headings: []string{"A", "B"}})
	if followed.Arrival == 0 {
		t.Fatal("following a link armed no marking")
	}
	if followed.Arrival != followed.NavSeq {
		t.Errorf("Arrival = %d, NavSeq = %d; the marking must name the landing it was made by",
			followed.Arrival, followed.NavSeq)
	}

	// A re-render: the same note, a fresh value at the same path, which is
	// what a written task box leaves behind.
	rebuilt, _ := Update(followed, taskToggled{vault: "/v", note: noteFromSource("f.md", "# F\n\nintro\n")})
	if rebuilt.Arrival != followed.Arrival {
		t.Errorf("re-rendering the note re-armed the marking: Arrival %d → %d", followed.Arrival, rebuilt.Arrival)
	}

	// Stepping back through the history is a return, not an arrival.
	back, _ := Update(followed, GoBack{})
	if back.Arrival != followed.Arrival {
		t.Errorf("Back re-armed the marking: Arrival %d → %d", followed.Arrival, back.Arrival)
	}
	fwd, _ := Update(back, GoForward{})
	if fwd.Arrival != followed.Arrival {
		t.Errorf("Forward re-armed the marking: Arrival %d → %d", followed.Arrival, fwd.Arrival)
	}

	// Following a link a second time is a second arrival.
	again, _ := Update(followed, Navigate{Path: "x.md"})
	if again.Arrival == followed.Arrival {
		t.Error("a second followed link did not arm a marking of its own")
	}
}

// arrivalGtx is a frame at now, with no event source: the marking asks for
// the repaints its fade needs, and a source that delivers nothing lets the
// test read what the marking returns rather than what a router does with it.
func arrivalGtx(ops *op.Ops, now time.Time) layout.Context {
	ops.Reset()
	return layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(600, 400)),
		Ops:         ops,
		Now:         now,
	}
}

// TestArrivalMarkingLivesItsCauseOut pins the marking's whole life: full
// strength while it holds, less of it across the fade, nothing once the life
// has run — and no dismissal anywhere, because the only thing that ends it
// early is the reader leaving the note it was made on.
func TestArrivalMarkingLivesItsCauseOut(t *testing.T) {
	var ops op.Ops
	col := tokens.DefaultLight
	m := Model{Current: "f.md", CurAnchor: 4, NavSeq: 3, Arrival: 3}
	t0 := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	full := col.HighlightOn(col.Background)

	var arr arrival
	block, wash, ok := arr.mark(arrivalGtx(&ops, t0), m, col)
	if !ok {
		t.Fatal("the arrival frame drew no marking")
	}
	if block != 4 {
		t.Errorf("marked block %d, want the block the link landed on, 4", block)
	}
	if wash != full {
		t.Errorf("wash = %v, want the highlight walked against the column's paper, %v", wash, full)
	}

	hold := arrivalLife - arrivalFade
	if _, w, ok := arr.mark(arrivalGtx(&ops, t0.Add(hold)), m, col); !ok || w.A != full.A {
		t.Errorf("at the end of the hold the wash was %v (drawn=%v), want full strength", w, ok)
	}
	_, mid, ok := arr.mark(arrivalGtx(&ops, t0.Add(hold+arrivalFade/2)), m, col)
	if !ok || mid.A == 0 || mid.A >= full.A {
		t.Errorf("half way through the fade the wash was %v (drawn=%v), want part strength", mid, ok)
	}
	if _, _, ok := arr.mark(arrivalGtx(&ops, t0.Add(arrivalLife)), m, col); ok {
		t.Error("the marking outlived its cause")
	}

	// A landing that carried no block anchor marks the note's opening
	// content, which is its first top-level block.
	top := Model{Current: "f.md", CurAnchor: -1, NavSeq: 4, Arrival: 4}
	var opening arrival
	if block, _, ok := opening.mark(arrivalGtx(&ops, t0), top, col); !ok || block != 0 {
		t.Errorf("a landing with no anchor marked block %d (drawn=%v), want the opening block 0", block, ok)
	}

	// Nothing followed, nothing marked.
	var none arrival
	if _, _, ok := none.mark(arrivalGtx(&ops, t0), Model{Current: "f.md"}, col); ok {
		t.Error("a note reached without following a link was marked")
	}
}

// TestArrivalMarkingDoesNotFollowTheReader asserts the marking belongs to
// the note it was made on: leaving that note ends it there and then, and it
// is not waiting on the other side when the reader comes back.
func TestArrivalMarkingDoesNotFollowTheReader(t *testing.T) {
	var ops op.Ops
	col := tokens.DefaultLight
	t0 := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	landed := Model{Current: "f.md", CurAnchor: 4, NavSeq: 3, Arrival: 3}

	var arr arrival
	if _, _, ok := arr.mark(arrivalGtx(&ops, t0), landed, col); !ok {
		t.Fatal("the arrival frame drew no marking")
	}
	// Back: the same landing is still the newest one the model records, but
	// the note on screen is another.
	away := landed
	away.Current = "x.md"
	away.CurAnchor = -1
	if _, _, ok := arr.mark(arrivalGtx(&ops, t0.Add(time.Second)), away, col); ok {
		t.Error("the marking followed the reader to another note")
	}
	if _, _, ok := arr.mark(arrivalGtx(&ops, t0.Add(2*time.Second)), landed, col); ok {
		t.Error("the marking was waiting on the note when the reader came back to it")
	}
}
