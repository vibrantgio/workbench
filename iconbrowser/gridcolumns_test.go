package main

import (
	"image"
	"testing"
)

// TestGridColumnsTakeALaneOnlyForADrawnBar holds the grid's column count
// against the rule that a lane exists only for a bar that is
// drawn: rows that fit at the full width take no lane, rows that overflow take
// it, and icons that fit only at the full width take none.
func TestGridColumnsTakeALaneOnlyForADrawnBar(t *testing.T) {
	const pad, lane, cellW, cellH = 20, 17, 100, 80
	// Four cells and half a lane fit across inside the padding.
	width := 2*pad + 4*cellW + lane/2
	cases := []struct {
		name   string
		height int
		count  int
		lane   int
		want   int
	}{
		{"fits at the full width", 2*cellH + pad, 8, lane, 4},
		{"overflows and takes the lane", 2*cellH + pad, 9, lane, 3},
		{"fits only at the full width", cellH + pad, 4, lane, 4},
		{"overflows with no lane to take", 2*cellH + pad, 9, 0, 4},
	}
	for _, tc := range cases {
		got := gridColumns(image.Pt(width, tc.height), pad, tc.lane, cellW, cellH, tc.count)
		if got != tc.want {
			t.Errorf("%s: %d columns, want %d", tc.name, got, tc.want)
		}
	}
}
