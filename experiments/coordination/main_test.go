package main

import (
	"testing"

	"gioui.org/f32"
	"github.com/vibrantgio/prism/coordination"
)

// newTestBoard constructs a *board using coordination.Subject[T] from
// prism/coordination, verifying the type plumbing introduced in G1.7.
func newTestBoard(t *testing.T) *board {
	t.Helper()
	dragObs, _ := coordination.Subject[DragState](coordination.BufCapPointer)
	modalObs, _ := coordination.Subject[ModalState](coordination.BufCapSignal)
	tooltipObs, _ := coordination.Subject[TooltipState](coordination.BufCapSignal)
	return newBoard(dragObs, modalObs, tooltipObs)
}

func TestMoveCard(t *testing.T) {
	b := newTestBoard(t)
	// Initial: cols[0]={0,1}, cols[1]={2,3}
	if got := len(b.cols[0]); got != 2 {
		t.Fatalf("cols[0] len = %d, want 2", got)
	}
	b.moveCard(0, 0, 1)
	if got := len(b.cols[0]); got != 1 {
		t.Errorf("cols[0] after move: got %d, want 1", got)
	}
	if got := len(b.cols[1]); got != 3 {
		t.Errorf("cols[1] after move: got %d, want 3", got)
	}
	// Card 0 must not be in col 0 anymore.
	for _, id := range b.cols[0] {
		if id == 0 {
			t.Error("card 0 still in col 0 after move")
		}
	}
}

func TestColUnder(t *testing.T) {
	b := newTestBoard(t)
	b.colW = 350
	cases := []struct {
		x    float32
		want int
	}{
		{0, 0},
		{349, 0},
		{350, 1},
		{700, 1},
	}
	for _, c := range cases {
		if got := b.colUnder(f32.Point{X: c.x}); got != c.want {
			t.Errorf("colUnder(%.0f) = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestIsOverCol(t *testing.T) {
	b := newTestBoard(t)
	b.colW = 300
	if !b.isOverCol(f32.Point{X: 100}, 0) {
		t.Error("x=100 should be over col 0")
	}
	if b.isOverCol(f32.Point{X: 100}, 1) {
		t.Error("x=100 should not be over col 1")
	}
	if b.isOverCol(f32.Point{X: 400}, 0) {
		t.Error("x=400 should not be over col 0")
	}
	if !b.isOverCol(f32.Point{X: 400}, 1) {
		t.Error("x=400 should be over col 1")
	}
}
