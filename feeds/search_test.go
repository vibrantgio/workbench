package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/tokens"
)

// searchFieldSize is the room the toolbar's share gives the field: the
// stated width (frame.go) at the toolbar's own depth.
var searchFieldSize = image.Pt(searchFieldWDp, 52)

// driveSearchField materialises the field the window lays out and returns a
// frame driver and the messages it has posted so far. The field is the live
// one — articlesSearch's, with its own editor, its clear mark and its
// Escape — not a copy of it.
func driveSearchField(t *testing.T) (func(), *[]mvu.Message, *gioinput.Router) {
	t.Helper()

	var posted []mvu.Message
	var w layout.Widget
	obs := articlesSearch(
		rx.Of(golden.StaticTheme(tokens.PlatformLight)),
		func(_ layout.Context, msg mvu.Message) { posted = append(posted, msg) },
	)
	sub := obs.Subscribe(rx.GoroutineContext(), func(next layout.Widget, err error, done bool) {
		if !done {
			w = next
		}
	})
	t.Cleanup(func() { sub.Unsubscribe() })
	sub.Wait()
	if w == nil {
		t.Fatal("the search field emitted nothing at all")
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Constraints: layout.Exact(searchFieldSize),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Source:      r.Source(),
		}
		w(gtx)
		r.Frame(ops)
	}
	return frame, &posted, r
}

// typeIntoSearchField presses in the field to take the keyboard and types
// one string into it, a frame per step, so the field reports each edit the
// way a reader's typing does.
func typeIntoSearchField(frame func(), r *gioinput.Router, s string) {
	frame()
	mid := f32.Pt(float32(searchFieldSize.X)/2, float32(searchFieldSize.Y)/2)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: mid, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: mid, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	frame()
	r.Queue(key.EditEvent{Text: s})
	frame()
}

// Escape takes the query back and says so. A search field standing on chrome
// has nothing to close, so Escape does what the clear mark does: it empties
// the field and reports the empty query, which is what brings the table back
// to its whole run. The keyboard stays in the field.
func TestEscapeClearsTheSearchFieldAndReportsTheEmptyQuery(t *testing.T) {
	frame, posted, r := driveSearchField(t)

	typeIntoSearchField(frame, r, "generics")
	if len(*posted) == 0 {
		t.Fatal("typing into the search field posted nothing")
	}
	if got, ok := (*posted)[len(*posted)-1].(SetQuery); !ok || got.Text != "generics" {
		t.Fatalf("typing posted %#v, want SetQuery{Text: \"generics\"}", (*posted)[len(*posted)-1])
	}
	typed := len(*posted)

	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	frame()

	if len(*posted) == typed {
		t.Fatal("Escape in the search field posted nothing; it takes the query back like the clear mark")
	}
	last := (*posted)[len(*posted)-1]
	got, ok := last.(SetQuery)
	if !ok {
		t.Fatalf("Escape posted %T, want main.SetQuery", last)
	}
	if got.Text != "" {
		t.Errorf("Escape posted SetQuery{Text: %q}, want the empty query", got.Text)
	}

	// And the field is empty afterwards, read off the field itself: a second
	// Escape has nothing left to take back and so reports nothing.
	emptied := len(*posted)
	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	frame()
	if len(*posted) != emptied {
		t.Errorf("a second Escape posted %v; the field was already empty", (*posted)[emptied:])
	}
}

// The query the reducer keeps is the one the field reported, and setting it
// takes the table back to page one — the two halves one message carries.
func TestSetQueryKeepsTheQueryAndResetsToPageOne(t *testing.T) {
	m := initialModel()
	m, _ = Update(m, SetPage{Page: 2})
	if m.currentPage != 2 {
		t.Fatalf("the table is on page %d, want 2 before the query is set", m.currentPage)
	}
	m, _ = Update(m, SetQuery{Text: "generics"})
	if m.query != "generics" {
		t.Errorf("the model keeps %q, want %q", m.query, "generics")
	}
	if m.currentPage != 1 {
		t.Errorf("the table is on page %d after a new query, want 1", m.currentPage)
	}
}
