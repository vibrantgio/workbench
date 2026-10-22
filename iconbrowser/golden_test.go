package main

import (
	"image"
	"testing"

	"github.com/vibrantgio/components/golden"
)

// goldenQueries are the three query states a window golden is taken in,
// each named in plain words for the file it is stored as: the whole
// catalogue with no query, a query that keeps some marks of each set, and a
// query that matches nothing, where the grid draws the empty state's line.
var goldenQueries = []struct {
	name  string
	query string
}{
	{"no-query", ""},
	{"some-match", "folder"},
	{"no-match", noMatchQuery},
}

// TestWindowGolden stores the composed window in both schemes for every
// query state, as window-{state}-{scheme}.png under testdata/golden. The
// frames are the ones the whole-window render draws: the backdrop, the page
// held down past the 32 dp title-bar strip, and the search field and grid
// shaped with the pinned shaper. No scroll bar setting is passed, so the
// grid's bar fades and a resting frame draws none.
//
// Record or refresh them with:
//
//	go test ./ -run TestWindowGolden -golden.update
func TestWindowGolden(t *testing.T) {
	for _, tc := range windowSchemes {
		for _, q := range goldenQueries {
			t.Run(q.name+"-"+tc.name, func(t *testing.T) {
				golden.Compare(t, "window-"+q.name+"-"+tc.name,
					renderWindow(t, tc.c, q.query, titleBandDp))
			})
		}
	}
}

// TestTheGoldensShowTheQueryInTheField pins that every window golden draws the
// search field holding the query it was taken at, as the running window does:
// the field's text equals the query after the field is drawn, and a field
// holding a query draws differently from an empty one, which shows its
// placeholder.
func TestTheGoldensShowTheQueryInTheField(t *testing.T) {
	fieldSize := image.Pt(windowSize.X, 64)
	for _, tc := range windowSchemes {
		empty, _ := staticSearchEditor(t, tc.c, "")
		emptyImg := golden.Capture(t, fieldSize, empty)
		for _, q := range goldenQueries {
			t.Run(q.name+"-"+tc.name, func(t *testing.T) {
				w, editor := staticSearchEditor(t, tc.c, q.query)
				img := golden.Capture(t, fieldSize, w)
				if got := editor.Text(); got != q.query {
					t.Errorf("the field's text = %q, want the query %q", got, q.query)
				}
				n := golden.PixelDiff(img, emptyImg)
				if q.query == "" && n != 0 {
					t.Errorf("the field with no query differs from the empty field in %d pixels", n)
				}
				if q.query != "" && n == 0 {
					t.Errorf("the field holding %q draws the same as the empty field", q.query)
				}
			})
		}
	}
}
