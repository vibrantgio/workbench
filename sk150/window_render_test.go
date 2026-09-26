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
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/textdraw"
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
// The setpoints are not on the photograph: the output holds 21 V exactly, so
// the voltage sits at its setpoint, and the current stands under its limit,
// which is what regulating on voltage means.
var goldenReading = Reading{
	VSet: 21, ISet: 7.1,
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

// goldenSettings is the live group behind that reading: the two setpoints
// the output is regulating to, and the two protections that would cut it.
// Each protection stands above the setpoint it guards, as the app's own
// write path requires.
var goldenSettings = Settings{
	VSet: 21, ISet: 7.1,
	LVP: 10.5, OVP: 24, OCP: 7.2, OPP: 150, OTP: 80,
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
		S:          goldenSettings,
		HaveS:      true,
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

// goldenThemed is the palette and the typography the window draws with in one
// scheme: what a layout.Widget rendered on its own, outside the window, needs
// to land the same pixels.
func goldenThemed(c tokens.PlatformColors) themed {
	return themed{palette: PaletteFrom(c), typ: TypeFrom(tokens.DefaultTypography)}
}

// renderPatch draws one layout.Widget at its natural size on the readout
// panel's black and returns the smallest image holding every pixel that is
// not that black — the patch as it stands in the window, where the same
// layout.Widget is placed at whole-pixel offsets on the same fill.
func renderPatch(t *testing.T, th themed, w layout.Widget) *image.RGBA {
	t.Helper()
	size := image.Pt(640, 240)
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, th.palette.DisplayPanel, clip.Rect{Max: size}.Op())
		return w(gtx)
	})
	box := image.Rectangle{Min: size, Max: image.Point{}}
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if at(img, x, y) == th.palette.DisplayPanel {
				continue
			}
			box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
			box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
		}
	}
	if box.Empty() {
		t.Fatal("the block drew nothing on the panel's black")
	}
	return img.SubImage(box).(*image.RGBA)
}

// patchTolerance is how far one channel of a matched pixel may stand from the
// patch's: the same glyphs rasterised under a different clip round one or two
// steps apart, which is not a difference in what the frame says.
const patchTolerance = 4

// near reports whether two pixels are the same within that tolerance.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) int { return int(x) - int(y) }
	return abs(d(a.R, b.R)) <= patchTolerance && abs(d(a.G, b.G)) <= patchTolerance && abs(d(a.B, b.B)) <= patchTolerance
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// findPatch reports whether patch stands somewhere in img. The search is
// anchored on the patch's first lit pixel, so only the handful of frame
// pixels wearing that colour are compared in full.
func findPatch(img, patch *image.RGBA) bool {
	pb, ib := patch.Bounds(), img.Bounds()
	w, h := pb.Dx(), pb.Dy()
	ax, ay, anchor := 0, 0, color.NRGBA{}
	for y := 0; y < h && anchor == (color.NRGBA{}); y++ {
		for x := 0; x < w; x++ {
			if c := at(patch, pb.Min.X+x, pb.Min.Y+y); c != displayPanel {
				ax, ay, anchor = x, y, c
				break
			}
		}
	}
	for y := ib.Min.Y; y <= ib.Max.Y-h; y++ {
	candidate:
		for x := ib.Min.X; x <= ib.Max.X-w; x++ {
			if !near(at(img, x+ax, y+ay), anchor) {
				continue
			}
			for py := 0; py < h; py++ {
				for px := 0; px < w; px++ {
					if !near(at(img, x+px, y+py), at(patch, pb.Min.X+px, pb.Min.Y+py)) {
						continue candidate
					}
				}
			}
			return true
		}
	}
	return false
}

// linePatch draws one line of the boxes' text at its natural size, which is
// what the boxes draw inside them at whole-pixel offsets on the same black.
func linePatch(th themed, fill color.NRGBA, str string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		sz := textdraw.MeasureText(gtx, th.typ.Shaper, th.typ.Set, str)
		textdraw.FillText(gtx, th.typ.Shaper, th.typ.Set, image.Rectangle{Max: sz}, 0, 0, fill, str)
		return layout.Dimensions{Size: sz}
	}
}

// TestTheSetAndLimitBoxesAreOnTheFrame reads the live group off the rendered
// window: the two titles and the four values — the two setpoints and the two
// protections — are drawn as the boxes at the panel's foot draw them, each in
// its own colour, and each is found in the frame in both schemes. The panel
// is the meter's own display and does not change with the window's
// appearance.
func TestTheSetAndLimitBoxesAreOnTheFrame(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			th := goldenThemed(tc.c)
			p := th.palette
			frame := renderWindow(t, tc.c)
			for _, want := range []struct {
				fill color.NRGBA
				str  string
			}{
				{p.DisplayCaption, "Set"},
				{p.DisplayVolt, "21.00 V"},
				{p.DisplayAmp, "7.100 A"},
				{p.DisplayCaption, "Limit"},
				{p.DisplayVolt, "24.00 V"},
				{p.DisplayAmp, "7.200 A"},
			} {
				if !findPatch(frame, renderPatch(t, th, linePatch(th, want.fill, want.str))) {
					t.Errorf("%q in %v is nowhere in the %s frame", want.str, want.fill, tc.name)
				}
			}
		})
	}
}

// TestTheSetAndLimitBoxesStandSideBySide reads the block's own geometry: two
// boxes of the same width with a gap between them, Set at the left and Limit
// at the right, together spanning every pixel of the width the panel gives
// the block.
func TestTheSetAndLimitBoxesStandSideBySide(t *testing.T) {
	th := goldenThemed(tokens.PlatformLight)
	const room = 420
	size := image.Pt(room, 120)
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, th.palette.DisplayPanel, clip.Rect{Max: size}.Op())
		gtx.Constraints = layout.Exact(image.Pt(room, 120))
		return setLimitBoxes(th, setLimitLines(goldenModel()))(gtx)
	})
	// A row across the middle of the block cuts the four upright edges: the
	// left and right edge of each box.
	row := 0
	for y := 0; y < size.Y; y++ {
		if at(img, 0, y) == th.palette.DisplayRim {
			row = y
		}
	}
	row /= 2
	var runs [][2]int
	for x, in := 0, false; x < room; x++ {
		lit := at(img, x, row) == th.palette.DisplayRim
		switch {
		case lit && !in:
			runs, in = append(runs, [2]int{x, x + 1}), true
		case lit:
			runs[len(runs)-1][1] = x + 1
		default:
			in = false
		}
	}
	if len(runs) != 4 {
		t.Fatalf("row %d of the block holds %d runs of the rim colour, want two boxes' four edges: %v", row, len(runs), runs)
	}
	left := [2]int{runs[0][0], runs[1][1]}
	right := [2]int{runs[2][0], runs[3][1]}
	if got, want := right[1]-right[0], left[1]-left[0]; got != want {
		t.Errorf("the boxes are %d px and %d px wide; the four values line up only if they are equal", want, got)
	}
	if left[0] != 0 || right[1] != room {
		t.Errorf("the boxes run x %d..%d inside a block %d px wide, want them across its whole width", left[0], right[1], room)
	}
	if gap := right[0] - left[1]; gap <= 0 {
		t.Errorf("the two boxes stand %d px apart, want a gap between them", gap)
	}
}

// TestTheSetAndLimitLinesDashWithoutTheLiveGroup: before the first read of
// the live group there is nothing to show, so the lines stand at the width
// the values will take with a dash per digit.
func TestTheSetAndLimitLinesDashWithoutTheLiveGroup(t *testing.T) {
	m := goldenModel()
	m.S, m.HaveS = Settings{}, false
	if got, want := setLimitLines(m), ([2]setLimitLine{
		{"Set", dashVolts, dashAmps},
		{"Limit", dashVolts, dashAmps},
	}); got != want {
		t.Errorf("without the live group the lines are %v, want %v", got, want)
	}
	th := goldenThemed(tokens.PlatformLight)
	golden.Capture(t, image.Pt(8, 8), func(gtx layout.Context) layout.Dimensions {
		measure := func(str string) int {
			return textdraw.MeasureText(gtx, th.typ.Shaper, th.typ.Set, str).X
		}
		live, dead := setLimitLines(goldenModel()), setLimitLines(m)
		for i := range live {
			if got, want := measure(dead[i].Volts), measure(live[i].Volts); got != want {
				t.Errorf("the dashed volts are %d px wide, the valued volts %d px: the dashes do not hold the digits' width", got, want)
			}
		}
		return layout.Dimensions{Size: image.Pt(8, 8)}
	})
}
