package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// drawnBox is the bounding box of the pixels inside r that are not fill.
func drawnBox(img *image.RGBA, r image.Rectangle, fill color.NRGBA) image.Rectangle {
	var box image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if pixelAt(img, image.Pt(x, y)) != fill {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// shadowReach is how far below the band the scan for the toolbar's drawing
// runs. The button's drop shadow reaches past the toolbar's foot, as the
// platform's does, and the first row's paint starts further down.
const shadowReach = 24

// toolbarBox is the bounding box of what the toolbar draws in its band: the
// add button, its drop shadow included.
func toolbarBox(img *image.RGBA, fill color.NRGBA) image.Rectangle {
	return drawnBox(img, image.Rect(0, 0, img.Bounds().Dx(), int(bandDp)+shadowReach), fill)
}

// TestTheToolbarDrawsTheAddButton renders the resting window in both schemes,
// with todos and with none. Under the Toolbar entry the add action is a
// bordered toolbar button titled with a symbol in the label colour, and no
// action floats over the content. The toolbar's band draws one bordered
// control at its trailing end: the toolbar control's fill, the symbol in the
// toolbar's label colour, no pixel near the accent. The page below the band
// draws nothing in its bottom-trailing corner, where the floating button
// stood.
func TestTheToolbarDrawsTheAddButton(t *testing.T) {
	for _, tc := range windowSchemes {
		for _, m := range []struct {
			name  string
			model Model
		}{{"todos", fixtureModel("")}, {"empty", Model{}}} {
			t.Run(tc.name+"/"+m.name, func(t *testing.T) {
				fill := PaletteFrom(tc.c).Backdrop
				img := renderModel(t, tc.c, m.model)
				box := toolbarBox(img, fill)
				t.Logf("the toolbar draws %v", box)
				if box.Empty() {
					t.Fatal("the toolbar's band draws nothing; want the add button")
				}
				if box.Min.X < windowSize.X/2 {
					t.Errorf("the toolbar draws at %v, reaching the window's leading half; want one button at its trailing end", box)
				}
				if h := box.Dy(); h < int(tokens.ComfortableToolbarControlHeight) {
					t.Errorf("the toolbar's drawing is %d px tall; want at least the toolbar control's %v", h, tokens.ComfortableToolbarControlHeight)
				}
				if gap := windowSize.X - box.Max.X; gap > int(shell.PlatformToolbar.Trail) {
					t.Errorf("the button's drawing ends %d px inside the window's edge; want at most the toolbar's trail %v", gap, shell.PlatformToolbar.Trail)
				}
				if n := countFill(img, box, tc.c.ToolbarControlFill); n == 0 {
					t.Errorf("no pixel in %v is the toolbar control's fill %v", box, tc.c.ToolbarControlFill)
				}
				label, accent := 0, 0
				for y := box.Min.Y; y < box.Max.Y; y++ {
					for x := box.Min.X; x < box.Max.X; x++ {
						c := pixelAt(img, image.Pt(x, y))
						if near(c, tc.c.ToolbarLabel) {
							label++
						}
						if near(c, tc.c.ControlAccent) {
							accent++
						}
					}
				}
				if label == 0 {
					t.Errorf("no pixel of the plus symbol is the toolbar's label colour %v", tc.c.ToolbarLabel)
				}
				if accent != 0 {
					t.Errorf("%d pixels of the button are near the accent %v", accent, tc.c.ControlAccent)
				}
				corner := image.Rect(windowSize.X-120, windowSize.Y-120, windowSize.X, windowSize.Y)
				if b := drawnBox(img, corner, fill); !b.Empty() {
					t.Errorf("the page draws %v in its bottom-trailing corner; want no floating button", b)
				}
			})
		}
	}
}

// TestTheAddButtonAndItsKeyAddATodo drives the toolbar through an input
// router. A click on the button calls the add action, and so does Command-N,
// the platform's New chord. The bare N calls nothing.
func TestTheAddButtonAndItsKeyAddATodo(t *testing.T) {
	added := 0
	add := func(layout.Context) { added++ }
	th := staticThemed(tokens.PlatformLight)
	bar := Toolbar(th.platform, th.density, new(widget.Clickable), func() unit.Dp { return bandDp }, add)
	keys := OnAddKey(add)
	w := func(gtx layout.Context) layout.Dimensions {
		bar(gtx)
		return keys(gtx)
	}
	var r input.Router
	var ops op.Ops
	frame := func() {
		ops.Reset()
		w(layout.Context{
			Constraints: layout.Exact(windowSize),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         &ops,
			Source:      r.Source(),
		})
		r.Frame(&ops)
	}
	frame()

	img := renderModel(t, tokens.PlatformLight, fixtureModel(""))
	box := toolbarBox(img, PaletteFrom(tokens.PlatformLight).Backdrop)
	at := f32.Pt(float32(box.Min.X+box.Max.X)/2, float32(box.Min.Y+box.Max.Y)/2)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
	)
	frame()
	if added != 1 {
		t.Fatalf("a click on the add button at %v called the add action %d times, want 1", at, added)
	}

	r.Queue(key.Event{Name: "N", Modifiers: key.ModShortcut, State: key.Press})
	frame()
	if added != 2 {
		t.Fatalf("Command-N called the add action %d times in all, want 2", added)
	}
	r.Queue(key.Event{Name: "N", State: key.Press})
	frame()
	if added != 2 {
		t.Errorf("the bare N called the add action; %d calls in all, want 2", added)
	}
}

// near reports whether two opaque colours are within a small distance of each
// other, summed over the three channels.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R)+d(a.G, b.G)+d(a.B, b.B) <= 24
}
