package main

import "testing"

// contentListRow is the content list row, MEASURED off Reminders' row in
// reference/macos/controls.md under "What a control inside a row measures":
// 32 px, y 169–200 in reminders-row-selected-{light,dark}.png. The test pins
// the measured number and not the token, so a token that drifts fails here.
const contentListRow = 32

// TestTheRowsAreTheMeasuredRowApart reads the rows off the frame: each
// checkbox's middle is one content list row below the one above it, and the
// first row starts at the list's top edge, because a list has no padding at
// its ends.
func TestTheRowsAreTheMeasuredRowApart(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			th := staticThemed(tc.c)
			img := renderWithPointer(t, tc.c, nil)
			ys := rowMiddles(img, th, int(bandDp))
			if len(ys) < 3 {
				t.Fatalf("found %d rows in the frame, want the fixture's four", len(ys))
			}
			for i := 1; i < len(ys); i++ {
				if d := ys[i] - ys[i-1]; d != contentListRow {
					t.Errorf("rows %d and %d are %d px apart, want the measured %d", i-1, i, d, contentListRow)
				}
			}
			if mid := ys[0] - int(bandDp); mid < contentListRow/2-1 || mid > contentListRow/2 {
				t.Errorf("the first row's middle is %d px below the list's top, want half a row, %d", mid, contentListRow/2)
			}
		})
	}
}
