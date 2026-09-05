package main

import (
	"image"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/theme/tokens"
)

// TestThePickerPaintsItsOwnChrome reads the picker screen off a render
// standing on the backdrop: the screen is one chrome region taking the
// window entire, so its own fill has to reach every edge. Nothing stands
// inset on this screen, so none of the window's plane is meant to show —
// and the window's outermost ring is where a region that borrowed the
// layer beneath it instead of painting would give itself away.
func TestThePickerPaintsItsOwnChrome(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := Model{Screen: screenPicker, PickerDir: "/notes"}
	m.PickerEntries = []DirEntry{
		{Name: "Second Brain", Path: "/notes/Second Brain", IsVault: true},
		{Name: "Archive", Path: "/notes/Archive", MDCount: 42},
	}
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
				sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}
			v := &pickerView{list: list.NewState()}
			stub := func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(120, 32)}
			}
			trail := func(gtx layout.Context, segs []breadcrumb.Segment) layout.Dimensions {
				return layout.Dimensions{}
			}
			w := func(gtx layout.Context) layout.Dimensions {
				return v.screen(gtx, m, tok, stub, trail)
			}
			img := golden.Capture(t, windowCanvasSize, windowScene(w, tc.colors))
			fill := chromeSurface(tc.colors)
			ring := func(x, y int) {
				t.Helper()
				if got := img.RGBAAt(x, y); !sameInk(got, fill) {
					t.Fatalf("the picker's edge at (%d,%d) draws %v, want its own chrome %v — the screen is standing on the plane instead of painting on it",
						x, y, got, fill)
				}
			}
			for x := 0; x < windowW; x++ {
				ring(x, 0)
				ring(x, windowH-1)
			}
			for y := 0; y < windowH; y++ {
				ring(0, y)
				ring(windowW-1, y)
			}
		})
	}
}
