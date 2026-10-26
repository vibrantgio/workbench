package main

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/icons"
)

// TestOutlineDisclosureDrawsTheSetsChevrons compares a row's disclosure with
// the set's own marks, pixel for pixel: a closed row draws chevron-right and an
// open row draws chevron-down, both at the size the row measures.
func TestOutlineDisclosureDrawsTheSetsChevrons(t *testing.T) {
	size := image.Pt(40, 40)
	fg := color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff}
	for _, tc := range []struct {
		open bool
		name icons.Name
	}{{false, icons.ChevronRight}, {true, icons.ChevronDown}} {
		got := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			drawOutlineDisclosure(gtx, tc.open, docsOutlineMarkDp, fg)
			return layout.Dimensions{Size: size}
		})
		if got == nil {
			t.Skip("no capture backend")
		}
		want := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			icons.Mark(tc.name)(gtx, gtx.Dp(docsOutlineMarkDp), fg)
			return layout.Dimensions{Size: size}
		})
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Errorf("a row with open=%v does not draw the set's %s", tc.open, tc.name)
		}
	}
}
