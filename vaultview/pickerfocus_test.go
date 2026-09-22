package main

// Where the keyboard lands when the switch dialog opens, read off the
// rendered frame. The Modal entry binds: the first field holds the keyboard
// focus when a dialog opens, as the platform's sheet shows. This dialog's
// body is a folder browser rather than a field, so its first control is the
// browser's list — and what has to be true either way is that the keyboard
// does NOT open on one of the two answers in the footer.

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

// livePicker builds the switch dialog's own layer over model m, live: a real
// theme stream, a real modal, a real browser list. It is the one path a
// keyboard reaches — the stored window image is a static render, which holds
// no focus at all.
func livePicker(t *testing.T, m Model, c tokens.PlatformColors, shaper *text.Shaper) layout.Widget {
	t.Helper()
	th := theme.Default()
	th.Platform = rx.Of(c)
	tok := themeTokens{col: c, typ: tokens.DefaultTypography, sp: tokens.Spacing,
		den: tokens.Comfortable, shaper: shaper}
	layer := vaultPickerLayer(rx.Of(th), rx.Of(m),
		func() Model { return m }, func() themeTokens { return tok }, modal.NewArbiter())
	var w layout.Widget
	if err := layer.Subscribe(rx.GoroutineContext(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("the switch dialog's layer: %v", err)
	}
	if w == nil {
		t.Fatal("the switch dialog's layer emitted no layout.Widget")
	}
	return w
}

// settledPicker drives w through the frames a live focus command needs — one
// to register the tags, one for the router to act on the command — with keys
// queued after, and captures the frame those keys left behind.
func settledPicker(t *testing.T, w layout.Widget, c tokens.PlatformColors, keys ...key.Event) *image.RGBA {
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

// TestTheSwitchDialogOpensOnTheBrowserAndNotOnAnAnswer reads the opening
// keyboard off the live dialog, in both schemes.
//
// Three readings. As it opens, no halo stands anywhere: neither answer holds
// the keyboard, and the browser's list draws no band of its own. One Down
// then moves the browser's cursor onto its first row — which only a list
// holding the keyboard does — and a Shift+Tab from the same opening state
// wraps the keyboard onto the last answer, where the halo appears. So the
// keyboard opens in the body and the footer is a Tab away, not the other way
// round.
func TestTheSwitchDialogOpensOnTheBrowserAndNotOnAnAnswer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := switchGoldenModel()
	down := key.Event{Name: key.NameDownArrow, State: key.Press}
	shiftTab := key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press}

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			ring := vgcolor.Flatten(tc.colors.KeyboardFocusIndicator, tc.colors.WindowBackground)
			cursor := tc.colors.SelectedContentBackground

			opened := settledPicker(t, livePicker(t, m, tc.colors, shaper), tc.colors)
			if n := countColor(opened, ring); n != 0 {
				t.Errorf("%d halo pixels stand in the frame as the dialog opens: an answer took the keyboard the body should have", n)
			}
			if n := countColor(opened, cursor); n != 0 {
				t.Errorf("the browser opens with %d pixels of its cursor row painted; it opens with no row picked", n)
			}

			walked := settledPicker(t, livePicker(t, m, tc.colors, shaper), tc.colors, down)
			if n := countColor(walked, cursor); n == 0 {
				t.Error("one Down moved nothing in the browser: the dialog did not open with the keyboard on its list")
			}

			tabbed := settledPicker(t, livePicker(t, m, tc.colors, shaper), tc.colors, shiftTab)
			if n := countColor(tabbed, ring); n == 0 {
				t.Error("Shift+Tab put no halo on an answer: the footer is not in the dialog's keyboard cycle")
			}
		})
	}
}
