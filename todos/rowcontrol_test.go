package main

import (
	"image"
	"path/filepath"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/composite"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// renderWithPointer draws the list route of the window in two frames through
// a router, as [captureWithPointer] states.
func renderWithPointer(t *testing.T, p tokens.PlatformColors, at *f32.Point) *image.RGBA {
	t.Helper()
	return captureWithPointer(t, windowSize, windowFrame(p, fixtureModel(""), titleBandDp), at)
}

// captureWithPointer draws draw over size in two frames through a router: the
// first registers the rows' areas, the pointer is then moved to at, and the
// second frame is the one returned. A nil at leaves the pointer off the frame.
func captureWithPointer(t *testing.T, size image.Point, draw layout.Widget, at *f32.Point) *image.RGBA {
	t.Helper()
	w, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("headless rendering not supported: %v", err)
	}
	defer w.Release()

	var r input.Router
	var ops op.Ops
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         &ops,
			Source:      r.Source(),
		}
		composite.Frame(gtx)
		draw(gtx)
		r.Frame(&ops)
	}
	frame()
	if at != nil {
		r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: *at})
	}
	frame()
	if err := w.Frame(&ops); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := w.Screenshot(img); err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	return img
}

// accentBox is the bounding box of the pixels in the frame's trailing band
// that are nearer the accent than the window's plane, the band being the 60
// columns at the window's trailing edge above the add button's band.
func accentBox(img *image.RGBA, th themed) image.Rectangle {
	accent, plane := th.palette.Icon, th.palette.Backdrop
	dist := func(a, b [3]int) int {
		d := 0
		for i := range a {
			if a[i] > b[i] {
				d += a[i] - b[i]
			} else {
				d += b[i] - a[i]
			}
		}
		return d
	}
	ac := [3]int{int(accent.R), int(accent.G), int(accent.B)}
	pl := [3]int{int(plane.R), int(plane.G), int(plane.B)}
	var box image.Rectangle
	size := img.Bounds().Size()
	bottom := size.Y - int(fabSize+2*Padding)
	for y := 0; y < bottom; y++ {
		for x := size.X - 60; x < size.X; x++ {
			c := pixelAt(img, image.Pt(x, y))
			px := [3]int{int(c.R), int(c.G), int(c.B)}
			if dist(px, ac) < dist(px, pl) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// TestTheDeleteMarkIsDrawnUnderThePointerOnly checks the delete mark against
// the MEASURED rows of a control inside a row: no row draws it at rest; the
// row under the pointer draws it, 18 px across, 16 px inside the window's
// trailing edge, in the accent; and no other row draws it then. The list
// selects no row, so the selected row's state has no frame here.
func TestTheDeleteMarkIsDrawnUnderThePointerOnly(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			th := staticThemed(tc.c)
			rest := renderWithPointer(t, tc.c, nil)
			if box := accentBox(rest, th); !box.Empty() {
				t.Fatalf("at rest the accent is drawn at %v in the trailing band; no row draws its mark at rest", box)
			}

			// The second row's middle: the row under the pointer is found
			// from the checkbox column of the rest frame.
			ys := rowMiddles(rest, th, int(titleBandDp))
			if len(ys) < 2 {
				t.Fatalf("found %d rows in the rest frame, want the fixture's four", len(ys))
			}
			at := f32.Pt(float32(windowSize.X/2), float32(ys[1]))
			hover := renderWithPointer(t, tc.c, &at)
			if *windowDump != "" {
				path := filepath.Join(*windowDump, "todos-"+tc.name+"-list-pointer.png")
				if err := golden.Save(path, hover); err != nil {
					t.Fatalf("save %s: %v", path, err)
				}
			}
			box := accentBox(hover, th)
			if box.Empty() {
				t.Fatalf("with the pointer over the second row no mark is drawn")
			}
			t.Logf("rows at %v, mark at %v", ys, box)
			if w, h := box.Dx(), box.Dy(); w < 17 || w > 19 || h < 17 || h > 19 {
				t.Errorf("the mark's drawn extent is %d x %d, want 18 x 18 within a pixel", w, h)
			}
			if gap := windowSize.X - box.Max.X; gap < 15 || gap > 17 {
				t.Errorf("the mark ends %d px inside the window's trailing edge, want 16 within a pixel", gap)
			}
			if mid := (box.Min.Y + box.Max.Y) / 2; mid < ys[0]+(ys[1]-ys[0])/2 || mid > ys[1]+(ys[2]-ys[1])/2 {
				t.Errorf("the mark's middle row is %d, outside the second row around %d", mid, ys[1])
			}
			if n := golden.PixelDiff(rest, hover); n == 0 {
				t.Errorf("the frame with the pointer over a row is the frame at rest")
			}
		})
	}
}

// rowMiddles returns the middle row of each todo row from row top down, read
// as the vertical middle of each run of drawn rows in the checkbox column of a
// frame with no mark drawn.
func rowMiddles(img *image.RGBA, th themed, top int) []int {
	plane := th.palette.Backdrop
	x0 := int(2 * Padding)
	var mids []int
	start := -1
	bottom := img.Bounds().Dy() - int(fabSize+2*Padding)
	for y := top; y < bottom; y++ {
		drawn := false
		for x := x0; x < x0+12; x++ {
			if pixelAt(img, image.Pt(x, y)) != plane {
				drawn = true
				break
			}
		}
		switch {
		case drawn && start < 0:
			start = y
		case !drawn && start >= 0:
			mids = append(mids, (start+y-1)/2)
			start = -1
		}
	}
	return mids
}
