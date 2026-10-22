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
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/theme/tokens"
)

// searchFieldSize is the room the page gives the field in the window it
// opens as: the window's width less the page's Padding on each side, at the
// comfortable field height the sidebar recess is drawn at.
var searchFieldSize = image.Pt(int(winW-2*Padding), int(tokens.Comfortable.FieldHeight))

// driveSearchField builds the field ContentLayer lays out and returns a frame
// driver, the queries it has posted so far, and the router the frames run
// through. The field is the live one — searchField's, with its own editor,
// its clear mark and its Escape — not a copy of it.
func driveSearchField(t *testing.T) (func(), *[]SetQuery, *gioinput.Router) {
	t.Helper()

	var posted []SetQuery
	var w layout.Widget
	obs := searchField(
		rx.Of(golden.StaticTheme(tokens.PlatformLight)),
		input.SearchFieldProps{},
		func(_ layout.Context, msg SetQuery) { posted = append(posted, msg) },
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

// typeIntoSearchField presses in the field to take the keyboard and types one
// string into it, a frame per step, so the field reports the edit the way a
// reader's typing does.
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

// Escape empties the field and posts the empty query, which brings the whole
// catalogue back. The field on this page has nothing to close, so Escape does
// what the clear mark does and the keyboard stays in the field.
func TestEscapeClearsTheSearchFieldAndPostsTheEmptyQuery(t *testing.T) {
	frame, posted, r := driveSearchField(t)

	typeIntoSearchField(frame, r, "folder")
	if len(*posted) == 0 {
		t.Fatal("typing into the search field posted nothing")
	}
	if got := (*posted)[len(*posted)-1]; got.Text != "folder" {
		t.Fatalf("typing posted %#v, want SetQuery{Text: \"folder\"}", got)
	}
	typed := len(*posted)

	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	frame()

	if len(*posted) == typed {
		t.Fatal("Escape in the search field posted nothing; it empties the field as the clear mark does")
	}
	if got := (*posted)[len(*posted)-1]; got.Text != "" {
		t.Errorf("Escape posted SetQuery{Text: %q}, want the empty query", got.Text)
	}

	// The field is empty afterwards, read off the field itself: a second
	// Escape has nothing left to empty and so posts nothing.
	emptied := len(*posted)
	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	frame()
	if len(*posted) != emptied {
		t.Errorf("a second Escape posted %v; the field was already empty", (*posted)[emptied:])
	}
}

// The window drawn with a query in the field draws the clear mark at the
// field's trailing end, and the window with no query draws none there. The
// field is the page's first row, Padding in from the window's edges and the
// page held down past the title-bar strip; the clear mark is drawn inside the
// last 40 dp of it, where neither query's text reaches.
func TestTheFilteredWindowDrawsTheClearMark(t *testing.T) {
	top := int(titleBandDp + Padding)
	right := windowSize.X - int(Padding)
	mark := image.Rect(right-40, top, right, top+searchFieldSize.Y)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			empty := renderWindow(t, tc.c, "", titleBandDp)
			for _, q := range []string{"folder", noMatchQuery} {
				img := renderWindow(t, tc.c, q, titleBandDp)
				n := 0
				for y := mark.Min.Y; y < mark.Max.Y; y++ {
					for x := mark.Min.X; x < mark.Max.X; x++ {
						if pixelAt(img, image.Pt(x, y)) != pixelAt(empty, image.Pt(x, y)) {
							n++
						}
					}
				}
				if n == 0 {
					t.Errorf("the window with %q in the field draws nothing at the field's trailing end; the clear mark is missing", q)
				}
			}
		})
	}
}
