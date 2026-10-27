package main

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/composite"
	"github.com/vibrantgio/components/golden"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// renderWithPointer draws the list route of the window in two frames through
// a router, as [captureWithPointer] states.
func renderWithPointer(t *testing.T, p tokens.PlatformColors, at *f32.Point) *image.RGBA {
	t.Helper()
	return captureWithPointer(t, windowSize, windowFrame(p, fixtureModel(""), bandDp), at)
}

// captureWithPointer draws draw over size in two frames through a router: the
// first registers the rows' areas, the pointer is then moved to at, and the
// second frame is the one returned. A nil at leaves the pointer off the frame.
func captureWithPointer(t *testing.T, size image.Point, draw layout.Widget, at *f32.Point) *image.RGBA {
	t.Helper()
	if at == nil {
		return captureWithEvents(t, size, draw, nil)
	}
	return captureWithEvents(t, size, draw, []event.Event{
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: *at},
	})
}

// captureWithEvents draws draw over size through a router: one frame to
// register the areas, then one frame after each batch of events is queued.
// The last frame is the one returned.
func captureWithEvents(t *testing.T, size image.Point, draw layout.Widget, batches ...[]event.Event) *image.RGBA {
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
	for _, batch := range batches {
		r.Queue(batch...)
		frame()
	}
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
// that are more than 20 levels off the window's plane, summed over the
// channels, which is how the reference reads a mark's extent. The band is the
// 60 columns at the window's trailing edge below the toolbar's band and the
// shadow its button casts.
func accentBox(img *image.RGBA, th themed) image.Rectangle {
	plane := th.palette.Backdrop
	abs := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	var box image.Rectangle
	size := img.Bounds().Size()
	for y := int(bandDp) + shadowReach; y < size.Y; y++ {
		for x := size.X - 60; x < size.X; x++ {
			c := pixelAt(img, image.Pt(x, y))
			if abs(c.R, plane.R)+abs(c.G, plane.G)+abs(c.B, plane.B) > 20 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// TestTheDeleteMarkIsDrawnUnderThePointerOnly checks the delete mark against
// the MEASURED rows of a control inside a row: no row draws it at rest; the
// row under the pointer draws it, 18 px across, 16 px inside the window's
// trailing edge, in the accent; and no other row draws it then. The selected
// row's state is checked in [TestAClickSelectsTheRow].
func TestTheDeleteMarkIsDrawnUnderThePointerOnly(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			th := staticThemed(tc.c)
			rest := renderWithPointer(t, tc.c, nil)
			if box := accentBox(rest, th); !box.Empty() {
				t.Fatalf("at rest a mark is drawn at %v in the trailing band; no row draws its mark at rest", box)
			}

			// The second row's middle: the row under the pointer is found
			// from the checkbox column of the rest frame.
			ys := rowMiddles(rest, th, int(bandDp))
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
			if w, h := box.Dx(), box.Dy(); w != 18 || h != 18 {
				t.Errorf("the mark's drawn extent is %d x %d, want 18 x 18", w, h)
			}
			if gap := windowSize.X - box.Max.X; gap != 16 {
				t.Errorf("the mark ends %d px inside the window's trailing edge, want 16", gap)
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
	for y := top; y < img.Bounds().Dy(); y++ {
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

// measuredSelection is the content list selection as the reference reads it
// on the selected row: #ebebeb light and #404040 dark, MEASURED in
// reference/macos/controls.md under "What a control inside a row measures".
var measuredSelection = map[string]color.NRGBA{
	"light": {R: 0xeb, G: 0xeb, B: 0xeb, A: 0xff},
	"dark":  {R: 0x40, G: 0x40, B: 0x40, A: 0xff},
}

// TestAClickSelectsTheRow checks the list's selection against the MEASURED
// selected row of a content list. At start no row is selected: no pixel of
// the list has the selected fill, and no mark is drawn. A click on the second
// row's text selects that row. With the pointer then off the window, the row
// has the measured fill across its full height and the window's full width,
// with square ends, and the row draws its delete mark. No other row draws the
// fill or the mark.
func TestAClickSelectsTheRow(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			th := staticThemed(tc.c)
			want := measuredSelection[tc.name]
			if th.palette.RowSelection != want {
				t.Fatalf("the palette's row selection is %v, want the measured %v", th.palette.RowSelection, want)
			}
			list := image.Rect(0, int(bandDp)+shadowReach, windowSize.X, windowSize.Y)
			// lead is the rows' leading inset, where a row draws nothing of
			// its own, so the fill is the only thing that can change it. A
			// text's antialiased edge can land on the fill's bytes, so the
			// whole list is not read for the fill.
			lead := image.Rect(0, list.Min.Y, int(2*Padding)-4, windowSize.Y)

			rest := renderWithPointer(t, tc.c, nil)
			if n := countFill(rest, lead, want); n != 0 {
				t.Fatalf("at start %d pixels of the list have the selected fill; no row is selected at start", n)
			}
			if box := accentBox(rest, th); !box.Empty() {
				t.Fatalf("at start a mark is drawn at %v; no row is selected at start", box)
			}

			ys := rowMiddles(rest, th, int(bandDp))
			if len(ys) < 3 {
				t.Fatalf("found %d rows in the rest frame, want the fixture's four", len(ys))
			}
			at := f32.Pt(float32(windowSize.X/2), float32(ys[1]))
			off := f32.Pt(-10, -10)
			img := captureWithEvents(t, windowSize, windowFrame(tc.c, fixtureModel(""), bandDp),
				[]event.Event{
					pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at},
					pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
					pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
				},
				[]event.Event{
					pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: off},
				},
			)
			if *windowDump != "" {
				path := filepath.Join(*windowDump, "todos-"+tc.name+"-list-selected.png")
				if err := golden.Save(path, img); err != nil {
					t.Fatalf("save %s: %v", path, err)
				}
			}

			// The fill's run down the window's leading column is the
			// selected row: it holds the clicked row's middle, it is the
			// content list row tall, and the trailing column holds the
			// same run, so the ends are square.
			top, bottom := fillRun(img, 0, ys[1], want)
			if top < 0 {
				t.Fatalf("the leading column at the second row's middle reads %v, want the measured fill %v", pixelAt(img, image.Pt(0, ys[1])), want)
			}
			if h, rowH := bottom-top, int(th.density.ContentListRowHeight); h != rowH {
				t.Errorf("the selected fill is %d px tall, rows %d-%d; want the row's %d", h, top, bottom-1, rowH)
			}
			if tt, tb := fillRun(img, windowSize.X-1, ys[1], want); tt != top || tb != bottom {
				t.Errorf("the trailing column holds the fill at rows %d-%d, the leading at %d-%d; want the same rows, square ends", tt, tb-1, top, bottom-1)
			}
			row := image.Rect(0, top, lead.Max.X, bottom)
			if n, all := countFill(img, lead, want), countFill(img, row, want); n != all || all != row.Dx()*row.Dy() {
				t.Errorf("the rows' leading inset holds %d pixels of the selected fill, %d of them in the selected row of %d pixels; want the selected row only, whole", n, all, row.Dx()*row.Dy())
			}

			// The mark on the selected row, read against the fill as the
			// reference reads it.
			box := drawnOn(img, image.Rect(windowSize.X-60, top, windowSize.X, bottom), want)
			if box.Empty() {
				t.Fatalf("the selected row draws no mark with the pointer off the window")
			}
			if w, h := box.Dx(), box.Dy(); w != 18 || h != 18 {
				t.Errorf("the mark's drawn extent is %d x %d, want 18 x 18", w, h)
			}
			if gap := windowSize.X - box.Max.X; gap != 16 {
				t.Errorf("the mark ends %d px inside the window's trailing edge, want 16", gap)
			}
			// No other row draws a mark.
			for _, r := range []image.Rectangle{
				image.Rect(windowSize.X-60, list.Min.Y, windowSize.X, top),
				image.Rect(windowSize.X-60, bottom, windowSize.X, windowSize.Y),
			} {
				if other := drawnOn(img, r, th.palette.Backdrop); !other.Empty() {
					t.Errorf("a mark is drawn at %v, outside the selected row", other)
				}
			}

			// The unchecked box on each row is the separator flattened onto
			// what the row lays under it: the content list selection on the
			// selected row, the window's plane on the others. The second and
			// third todos are unchecked; the box is centred in its footprint
			// at the rows' leading inset.
			boxX := int(2*Padding) + int(th.density.CheckboxRowHeight)/2
			for _, r := range []struct {
				what string
				y    int
				on   color.NRGBA
			}{
				{"selected", ys[1], want},
				{"unselected", ys[2], th.palette.Backdrop},
			} {
				fill := vgcolor.Flatten(tc.c.Separator, r.on)
				if got := pixelAt(img, image.Pt(boxX, r.y)); got != fill {
					t.Errorf("the unchecked box on the %s row reads %v, want the separator over %v, %v", r.what, got, r.on, fill)
				}
			}

			// A press on the row's leading inset, which holds no control,
			// selects the row too.
			blank := f32.Pt(float32(Padding), float32(ys[2]))
			img = captureWithEvents(t, windowSize, windowFrame(tc.c, fixtureModel(""), bandDp),
				[]event.Event{
					pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: blank},
					pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: blank},
				},
			)
			if got := pixelAt(img, image.Pt(0, ys[2])); got != want {
				t.Errorf("after a press on the third row's leading inset its leading column reads %v, want the measured fill %v", got, want)
			}
		})
	}
}

// fillRun returns the rows of the run of want down column x that holds row
// y, as top inclusive and bottom exclusive, or -1, -1 when row y is not want.
func fillRun(img *image.RGBA, x, y int, want color.NRGBA) (top, bottom int) {
	if pixelAt(img, image.Pt(x, y)) != want {
		return -1, -1
	}
	top, bottom = y, y+1
	for top > 0 && pixelAt(img, image.Pt(x, top-1)) == want {
		top--
	}
	for bottom < img.Bounds().Dy() && pixelAt(img, image.Pt(x, bottom)) == want {
		bottom++
	}
	return top, bottom
}

// drawnOn is the bounding box of the pixels inside r that are more than 20
// levels off fill, summed over the channels, which is how the reference reads
// a mark's extent.
func drawnOn(img *image.RGBA, r image.Rectangle, fill color.NRGBA) image.Rectangle {
	abs := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	var box image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := pixelAt(img, image.Pt(x, y))
			if abs(c.R, fill.R)+abs(c.G, fill.G)+abs(c.B, fill.B) > 20 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}
