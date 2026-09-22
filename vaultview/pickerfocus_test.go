package main

// Where the keyboard lands when the switch dialog opens, read off the
// rendered frame. The Modal entry binds: the first field holds the keyboard
// focus when a dialog opens, as the platform's sheet shows. This dialog's
// body is a folder browser rather than a field, so its first control is the
// browser's list — and a list at the front of a dialog is a focusable like a
// field: it wears the halo on its own box and shows a selected row.

import (
	"image"
	"image/color"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/modal"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// dialogLayer is the shape both of this application's modal layers are
// built in: a theme stream, the model behind the dialog, the two snapshot
// loaders and the arbiter the dialog declares itself to.
type dialogLayer func(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	loadModel func() Model,
	loadTok func() themeTokens,
	arb *modal.Arbiter,
) rx.Observable[layout.Widget]

// liveDialog builds one dialog's own layer over model m, live: a real theme
// stream, a real modal, real focus tags. It is the one path a keyboard
// reaches — a stored window image is a static render, which holds no focus
// at all.
func liveDialog(t *testing.T, build dialogLayer, m Model, c tokens.PlatformColors, shaper *text.Shaper) layout.Widget {
	t.Helper()
	th := theme.Default()
	th.Platform = rx.Of(c)
	tok := themeTokens{col: c, typ: tokens.DefaultTypography, sp: tokens.Spacing,
		den: tokens.Comfortable, shaper: shaper}
	layer := build(rx.Of(th), rx.Of(m),
		func() Model { return m }, func() themeTokens { return tok }, modal.NewArbiter())
	var w layout.Widget
	if err := layer.Subscribe(rx.GoroutineContext(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("the dialog's layer: %v", err)
	}
	if w == nil {
		t.Fatal("the dialog's layer emitted no layout.Widget")
	}
	return w
}

// livePicker is the switch dialog's layer.
func livePicker(t *testing.T, m Model, c tokens.PlatformColors, shaper *text.Shaper) layout.Widget {
	t.Helper()
	return liveDialog(t, vaultPickerLayer, m, c, shaper)
}

// settledDialog drives w through the frames a live focus command needs — one
// to register the tags, one for the router to act on the command — with keys
// queued after, and captures the frame those keys left behind.
func settledDialog(t *testing.T, w layout.Widget, c tokens.PlatformColors, keys ...key.Event) *image.RGBA {
	t.Helper()
	r := new(gioinput.Router)
	drive := func() {
		var ops op.Ops
		w(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(windowFrameSize),
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
	return golden.Capture(t, windowFrameSize, windowScene(func(gtx layout.Context) layout.Dimensions {
		gtx.Source = r.Source()
		return w(gtx)
	}, c))
}

// countColor reports how many pixels of the frame carry exactly col.
func countColor(img *image.RGBA, col color.NRGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if p := img.RGBAAt(x, y); p.R == col.R && p.G == col.G && p.B == col.B && p.A == col.A {
				n++
			}
		}
	}
	return n
}

// colorBox returns the bounding box of every pixel drawn in col, or the
// empty rectangle when the frame carries none.
func colorBox(img *image.RGBA, col color.NRGBA) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R != col.R || p.G != col.G || p.B != col.B || p.A != col.A {
				continue
			}
			r := image.Rect(x, y, x+1, y+1)
			if box.Empty() {
				box = r
			} else {
				box = box.Union(r)
			}
		}
	}
	return box
}

// haloRing is the colour the halo's outer half lands as on a dialog's own
// plane: the platform's keyboard focus indicator flattened onto the window
// background every floating surface here takes.
func haloRing(c tokens.PlatformColors) color.NRGBA {
	return vgcolor.Flatten(c.KeyboardFocusIndicator, c.WindowBackground)
}

// haloOutside is how far past a control's own box the band reaches at the
// metric these frames are driven at: half of the halo's measured 4 dp.
const haloOutside = 2

// TestTheSwitchDialogOpensOnTheBrowserWearingItsHalo reads the opening
// keyboard off the live dialog, in both schemes.
//
// Four readings. As it opens, the halo stands around the browser's list and
// nowhere else, and the list's first row — the listing's first, the dialog
// standing IN the vault rather than in its parent — wears the selection
// fill. One Down moves that selection one row on, which only a list holding
// the keyboard does. One Tab carries the halo off the list and onto the
// footer's first answer, leaving the selection where it stood. So the
// keyboard opens in the body and the footer is a Tab away, not the other way
// round.
func TestTheSwitchDialogOpensOnTheBrowserWearingItsHalo(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := switchGoldenModel()
	down := key.Event{Name: key.NameDownArrow, State: key.Press}
	tab := key.Event{Name: key.NameTab, State: key.Press}

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			ring := haloRing(tc.colors)
			cursor := tc.colors.SelectedContentBackground

			opened := settledDialog(t, livePicker(t, m, tc.colors, shaper), tc.colors)
			halo := colorBox(opened, ring)
			if halo.Empty() {
				t.Fatal("the switch dialog opened with no halo anywhere in the window: nothing holds the keyboard")
			}
			row := colorBox(opened, cursor)
			if row.Empty() {
				t.Fatal("the browser opened with no row selected: a list holding the keyboard shows which row it stands on")
			}
			// The band straddles the list's box, so it runs haloOutside past
			// the rows on every side, and the first row sits at the box's own
			// top edge under the half of the band lying over it.
			if halo.Min.X != row.Min.X-2*haloOutside || halo.Max.X != row.Max.X+2*haloOutside {
				t.Errorf("the halo runs x %d-%d and the selected row x %d-%d: the band is not on the list's own box",
					halo.Min.X, halo.Max.X, row.Min.X, row.Max.X)
			}
			if halo.Min.Y != row.Min.Y-2*haloOutside {
				t.Errorf("the halo starts at y %d and the selected row at y %d: the first row is not at the list's top edge",
					halo.Min.Y, row.Min.Y)
			}
			if got, want := halo.Dy(), haloOutside+vaultPickerRows*int(tokens.Comfortable.ControlHeight)+haloOutside; got != want {
				t.Errorf("the halo stands %d px tall; the list's box is %d rows and the band %d px past it on each side",
					got, vaultPickerRows, haloOutside)
			}

			// The foot of the fill rather than its top: the opening row sits
			// at the list's top edge, where the half of the band lying over
			// the box covers its first haloOutside rows.
			walked := settledDialog(t, livePicker(t, m, tc.colors, shaper), tc.colors, down)
			moved := colorBox(walked, cursor)
			if moved.Max.Y != row.Max.Y+int(tokens.Comfortable.ControlHeight) {
				t.Errorf("one Down left the selection ending at y %d, from y %d: the arrows do not move the browser's selection one row",
					moved.Max.Y, row.Max.Y)
			}

			tabbed := settledDialog(t, livePicker(t, m, tc.colors, shaper), tc.colors, tab)
			next := colorBox(tabbed, ring)
			if next.Empty() {
				t.Fatal("Tab left no halo in the window: the footer is not in the dialog's keyboard cycle")
			}
			if next.Min.Y < halo.Max.Y {
				t.Errorf("Tab left the halo at y %d-%d, still over the list at y %d-%d: it did not move off the list",
					next.Min.Y, next.Max.Y, halo.Min.Y, halo.Max.Y)
			}
			if n := countColor(tabbed, cursor); n == 0 {
				t.Error("Tab cleared the browser's selection: the selection is the list's, not the keyboard's")
			}
		})
	}
}
