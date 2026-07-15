package keyed_test

import (
	"sync"
	"testing"
	"time"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/experiments/keyed"
)

// Item is a minimal todo-list item. ID is the stable key; Text is mutable.
type Item struct {
	ID   string
	Text string
}

// RowState holds per-row Gio widget state that must survive list reorders.
type RowState struct {
	EditText string
	Checked  bool
}

// emit sends items to a Subject observer and waits for the subscriber to
// receive the emission via the result channel. It fails the test on timeout.
func emit(t *testing.T, send rx.Observer[[]Item], result <-chan []*RowState, list []Item) []*RowState {
	t.Helper()
	send(list, nil, false)
	select {
	case got := <-result:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for emission")
		return nil
	}
}

// setup creates a Subject[[]Item] and a mapped Observable that returns the
// per-key *RowState slice for each emission, collected into a channel.
// Returns the send observer, the results channel, and a teardown function.
func setup(t *testing.T) (rx.Observer[[]Item], <-chan []*RowState, *keyed.Deferred[string, *RowState], func()) {
	t.Helper()
	send, items := rx.Subject[[]Item](0, 1)

	d := keyed.New(func(_ string) *RowState {
		return &RowState{}
	})

	results := make(chan []*RowState, 4)

	// The Map closure mirrors what an rx.Defer body would do: call d.For() for
	// each item and return the stable state slice as the "rendered" output.
	sub := rx.Map(items, func(list []Item) []*RowState {
		states := make([]*RowState, len(list))
		for i, item := range list {
			states[i] = d.For(item.ID)
		}
		return states
	}).Subscribe(func(next []*RowState, err error, done bool) {
		if !done {
			results <- next
		}
	}, rx.Goroutine)

	return send, results, d, func() { sub.Unsubscribe() }
}

// TestReorderPreservesState verifies that per-key state (mutations made
// between emissions) follows keys rather than positions when the list is
// reordered.
func TestReorderPreservesState(t *testing.T) {
	send, results, _, teardown := setup(t)
	defer teardown()

	// Initial list: [A, B, C].
	got := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "B", Text: "beta"},
		{ID: "C", Text: "gamma"},
	})

	// Simulate user interaction between frames.
	got[0].EditText = "edited A" // position 0 → key A
	got[1].Checked = true        // position 1 → key B

	// Reorder to [C, A, B].
	got2 := emit(t, send, results, []Item{
		{ID: "C", Text: "gamma"},
		{ID: "A", Text: "alpha"},
		{ID: "B", Text: "beta"},
	})

	// C (position 0): untouched → zero state.
	if got2[0].EditText != "" || got2[0].Checked {
		t.Errorf("C: expected zero state, got EditText=%q Checked=%v", got2[0].EditText, got2[0].Checked)
	}
	// A (position 1): carried EditText from position 0 before reorder.
	if got2[1].EditText != "edited A" {
		t.Errorf("A: expected EditText=%q, got %q", "edited A", got2[1].EditText)
	}
	// B (position 2): carried Checked from position 1 before reorder.
	if !got2[2].Checked {
		t.Error("B: expected Checked=true after reorder")
	}
}

// TestInsertionPreservesExistingState verifies that inserting a new item
// into the list does not disturb the state of existing items.
func TestInsertionPreservesExistingState(t *testing.T) {
	send, results, _, teardown := setup(t)
	defer teardown()

	// Initial list: [A, B].
	got := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "B", Text: "beta"},
	})

	got[0].EditText = "edited A"
	got[1].Checked = true

	// Insert C between A and B → [A, C, B].
	got2 := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "C", Text: "new"},
		{ID: "B", Text: "beta"},
	})

	// A at position 0: state unchanged.
	if got2[0].EditText != "edited A" {
		t.Errorf("A: expected EditText=%q, got %q", "edited A", got2[0].EditText)
	}
	// C at position 1: fresh state (first time seen).
	if got2[1].EditText != "" || got2[1].Checked {
		t.Errorf("C: expected zero state, got EditText=%q Checked=%v", got2[1].EditText, got2[1].Checked)
	}
	// B at position 2: state unchanged despite position shift.
	if !got2[2].Checked {
		t.Error("B: expected Checked=true after insertion of C")
	}
}

// TestDeletionAndSweep verifies two behaviours:
// (a) sticky: after deletion and before sweep, the deleted key's state is
// preserved and returned if the key re-appears.
// (b) swept: after Sweep, the deleted key's state is freed; re-adding it
// produces a fresh zero value.
func TestDeletionAndSweep(t *testing.T) {
	send, results, d, teardown := setup(t)
	defer teardown()

	// Initial list: [A, B, C].
	got := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "B", Text: "beta"},
		{ID: "C", Text: "gamma"},
	})

	got[0].EditText = "edited A"
	got[1].Checked = true
	got[2].EditText = "edited C"

	// Delete B → [A, C].
	got2 := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "C", Text: "gamma"},
	})

	if got2[0].EditText != "edited A" {
		t.Errorf("A: expected EditText=%q after deletion of B, got %q", "edited A", got2[0].EditText)
	}
	if got2[1].EditText != "edited C" {
		t.Errorf("C: expected EditText=%q after deletion of B, got %q", "edited C", got2[1].EditText)
	}

	// (a) Sticky: B's state is still in the registry before Sweep.
	// Re-add B → [A, B, C].
	got3 := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "B", Text: "beta"},
		{ID: "C", Text: "gamma"},
	})

	if !got3[1].Checked {
		t.Error("B: expected Checked=true on re-add before Sweep (sticky semantics)")
	}

	// Now sweep with only A and C active, then remove B again.
	got4 := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "C", Text: "gamma"},
	})
	_ = got4

	d.Sweep([]string{"A", "C"})

	if d.Len() != 2 {
		t.Errorf("after Sweep([A,C]): expected Len=2, got %d", d.Len())
	}

	// (b) Swept: re-adding B now produces fresh state.
	got5 := emit(t, send, results, []Item{
		{ID: "A", Text: "alpha"},
		{ID: "B", Text: "beta"},
		{ID: "C", Text: "gamma"},
	})

	if got5[1].Checked {
		t.Error("B: expected Checked=false after Sweep (fresh state on re-add)")
	}
	if got5[1].EditText != "" {
		t.Errorf("B: expected EditText=\"\" after Sweep, got %q", got5[1].EditText)
	}
}

// TestConcurrentFor exercises the mutex protection on the internal map by
// calling For from multiple goroutines simultaneously with distinct keys.
// Each goroutine owns a unique *RowState so writes do not race across
// goroutines; what is being tested is that For itself (map get/put) is
// race-free. Run with -race.
func TestConcurrentFor(t *testing.T) {
	d := keyed.New(func(id int) *RowState { return &RowState{} })
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each goroutine uses its own key, so the returned pointer is
			// not shared. The race detector only sees the map access.
			s := d.For(i)
			s.EditText = "x"
		}(i)
	}
	wg.Wait()
}
