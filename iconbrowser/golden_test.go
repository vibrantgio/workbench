package main

import (
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
// shaped with the pinned shaper. The icon grid has no scroll bar, so no
// scroll bar setting is passed.
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
