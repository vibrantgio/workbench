package main

// The keyboard's landing place when a dialog opens, read off the rendered
// frame. The Modal entry binds: the first field holds the keyboard focus
// when a dialog opens, as the platform's sheet shows — its "Save As:" field
// is the focused one, wearing the halo, and neither answer in the footer is.
// So this reads the halo rather than the focus tag: what a reader is told is
// the band, and a tag that holds focus without drawing one tells nobody.

import (
	"image"
	"image/color"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// settled drives w through the frames a live focus command needs — one to
// register the tags, one for the router to act on the command — with keys
// queued in between, and returns a layout.Widget drawing from the state they
// left behind.
func settled(w layout.Widget, size image.Point, keys ...key.Event) layout.Widget {
	r := new(gioinput.Router)
	drive := func() {
		var ops op.Ops
		w(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         &ops,
			Source:      r.Source(),
		})
		r.Frame(&ops)
	}
	drive()
	drive()
	for _, k := range keys {
		r.Queue(k)
		drive()
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Source = r.Source()
		return w(gtx)
	}
}

// haloBox returns the bounding box of every pixel drawn in the focus halo's
// own colour — the platform's keyboard focus indicator flattened onto the
// dialog's plane, which is the band's outer half, the one that lies on the
// surface rather than over the control. An empty rectangle means no control
// in the frame is wearing one.
func haloBox(img *image.RGBA, c tokens.PlatformColors) image.Rectangle {
	ring := vgcolor.Flatten(c.KeyboardFocusIndicator, c.WindowBackground)
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !sameRGBA(img.RGBAAt(x, y), ring) {
				continue
			}
			p := image.Rect(x, y, x+1, y+1)
			if box.Empty() {
				box = p
			} else {
				box = box.Union(p)
			}
		}
	}
	return box
}

func sameRGBA(got color.RGBA, want color.NRGBA) bool {
	return got.R == want.R && got.G == want.G && got.B == want.B && got.A == want.A
}

// TestTheSettingsDialogOpensWithTheHaloOnItsFirstField reads the opening
// halo off the window the settings dialog stands in, in both schemes. Three
// frames are taken of the same dialog: as it opens, one Tab on, and one
// Shift+Tab on. The keyboard's cycle here is the three provider fields and
// then the two footer answers, so the opening halo has to stand above the
// one a Tab moves it to (the second field) and above the one a Shift+Tab
// wraps it to (the last answer, in the footer).
func TestTheSettingsDialogOpensWithTheHaloOnItsFirstField(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	settings, _ := Update(demoModel(), OpenSettings{})
	tab := key.Event{Name: key.NameTab, State: key.Press}
	shiftTab := key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press}

	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			box := func(keys ...key.Event) image.Rectangle {
				w := settled(frame(t, tc.c, settings), windowSize, keys...)
				return haloBox(golden.Capture(t, windowSize, w), tc.c)
			}
			opened := box()
			if opened.Empty() {
				t.Fatal("the settings dialog opened with no halo anywhere in the window: nothing holds the keyboard")
			}
			next := box(tab)
			if next.Empty() {
				t.Fatal("Tab left no halo in the window")
			}
			if opened.Min.Y >= next.Min.Y {
				t.Errorf("the opening halo stands at y %d and Tab moves it to y %d: the dialog did not open on the FIRST field",
					opened.Min.Y, next.Min.Y)
			}
			footer := box(shiftTab)
			if footer.Empty() {
				t.Fatal("Shift+Tab left no halo in the window")
			}
			if opened.Max.Y >= footer.Min.Y {
				t.Errorf("the opening halo runs to y %d and the footer's answer wears one from y %d: the dialog opened on an answer rather than on its first field",
					opened.Max.Y, footer.Min.Y)
			}
		})
	}
}
