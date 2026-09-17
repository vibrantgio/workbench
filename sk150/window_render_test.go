package main

// A whole-window render, headless, in both colour schemes. The app has no
// offscreen mode of its own — it is a native window binary — but the layers
// the window renders are plain observables of layout.Widget, so composing
// them over a frozen theme and drawing them into a headless image produces
// the frame the window would show, at the size the window opens at.
//
// The stored pair is what a reviewer is given: the composition itself, which
// is the one thing about this window no pixel assertion states. Regenerate
// with:
//
//	go test ./ -run TestTheWholeWindow -golden.update

import (
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// windowSize is the size the window opens at (main.go), and the only size
// these frames are drawn at.
var windowSize = image.Pt(int(winW), int(winH))

// sharpRadius keeps the renders comparable between machines: anti-aliased
// rounded corners vary slightly between GPU contexts, which is enough to fail
// a pixel-exact diff on a window of panels, pills and buttons.
var sharpRadius = tokens.RadiusScale{}

// schemes is the pair every frame below is drawn in.
var schemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

func init() {
	// Faces and nothing the machine happens to own; see appShaper.
	appShaper = func(t tokens.Typography) *text.Shaper { return t.DeterministicShaper() }
}

// staticTheme freezes one colour scheme into a Theme whose every field emits
// once — the shape theme/window feeds the layers, minus the live OS poll.
func staticTheme(c tokens.PlatformColors) theme.Theme {
	return theme.Theme{
		Platform:   rx.Of(c),
		Typography: rx.Of(tokens.DefaultTypography),
		Density:    rx.Of(tokens.Comfortable),
		Motion:     rx.Of(tokens.Motion),
		Spacing:    rx.Of(tokens.Spacing),
		Radius:     rx.Of(sharpRadius),
		Elevation:  rx.Of(tokens.Elevation),
	}
}

// goldenReading is the device as the photograph in the plan's
// reference/sk150-display.png shows it: 21 V out at 7 A, 147 W, regulating on
// voltage with the output on — the state the readout panel is reviewed in.
var goldenReading = Reading{
	VSet: 21, ISet: 7,
	VOut: 21, IOut: 7, Power: 147,
	VIn:     24.6,
	MilliAh: 1240, MilliWh: 26800,
	Hours: 0, Mins: 10, Secs: 38,
	TempIn: 31.5,
	CC:     false,
	On:     true,
}

// goldenHistory is a fixed chart window: a settling output, written down here
// rather than sampled from a clock, so the two charts draw the same strokes on
// every run.
func goldenHistory() []Sample {
	base := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	out := make([]Sample, 60)
	for i := range out {
		f := float64(i) / float64(len(out)-1)
		out[i] = Sample{
			At: base.Add(time.Duration(i) * time.Second),
			V:  20.4 + 0.6*f,
			I:  6.55 + 0.45*f,
		}
	}
	return out
}

// goldenModel is the window as the stored images show it: online against the
// simulated device, on the Monitor tab, with the reading above.
func goldenModel() Model {
	return Model{
		Screen:     "monitor",
		Status:     "demo mode",
		Online:     true,
		Demo:       true,
		PollCount:  60,
		R:          goldenReading,
		HaveR:      true,
		History:    goldenHistory(),
		EditPreset: noEdit,
	}
}

// windowFrame composes the window's layers for one scheme into a single
// layout.Widget: the backdrop first, the content over it, exactly as
// theme/window stacks them.
func windowFrame(t *testing.T, c tokens.PlatformColors, m Model) layout.Widget {
	t.Helper()
	layers := buildLayers(rx.Of(m))(rx.Of(staticTheme(c)))
	widgets := make([]layout.Widget, len(layers))
	for i, layer := range layers {
		w, err := collectOne(layer)
		if err != nil {
			t.Fatalf("layer %d never emitted a layout.Widget: %v", i, err)
		}
		widgets[i] = w
	}
	return func(gtx layout.Context) layout.Dimensions {
		for _, w := range widgets {
			w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// collectOne subscribes an observable and returns its first layout.Widget.
func collectOne(obs rx.Observable[layout.Widget]) (layout.Widget, error) {
	got := make(chan layout.Widget, 1)
	fail := make(chan error, 1)
	sub := obs.Subscribe(rx.GoroutineContext(), func(v layout.Widget, err error, done bool) {
		if done {
			select {
			case fail <- err:
			default:
			}
			return
		}
		if v != nil {
			select {
			case got <- v:
			default:
			}
		}
	})
	defer sub.Unsubscribe()
	select {
	case w := <-got:
		return w, nil
	case err := <-fail:
		select {
		case w := <-got:
			return w, nil
		default:
		}
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, errors.New("no emission within five seconds")
	}
}

// renderWindow draws the settled window in one scheme. Two frames are drawn
// and the second is kept: the first registers the pointer and click tags the
// layout tree needs before any of them can report state.
func renderWindow(t *testing.T, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	w := windowFrame(t, c, goldenModel())
	golden.Capture(t, windowSize, w)
	return golden.Capture(t, windowSize, w)
}

// at reads one pixel as an opaque NRGBA, which is what every colour this
// window hands the rasterizer is.
func at(img *image.RGBA, x, y int) color.NRGBA {
	r, g, b, _ := img.At(x, y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
}

// TestTheWholeWindow stores the window as it opens on the Monitor tab, both
// appearances, at the size it opens at.
func TestTheWholeWindow(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			golden.Compare(t, "window-"+tc.name, renderWindow(t, tc.c))
		})
	}
}

// TestReadoutPanelIsTheMetersOwnDisplay reads the device's values off the
// rendered frame: the panel's black behind the readouts, and the three lit
// colours on it, the same in both schemes because the meter has one panel.
func TestReadoutPanelIsTheMetersOwnDisplay(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			counts := map[color.NRGBA]int{}
			b := img.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					counts[at(img, x, y)]++
				}
			}
			for _, want := range []struct {
				name string
				c    color.NRGBA
			}{
				{"the panel's black", displayPanel},
				{"the volt line's green", displayVolt},
				{"the amp line's yellow", displayAmp},
				{"the watt line's magenta", displayWatt},
			} {
				if counts[want.c] == 0 {
					t.Errorf("%s (%v) is nowhere in the %s frame", want.name, want.c, tc.name)
				}
			}
		})
	}
}
