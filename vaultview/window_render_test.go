package main

// The whole vault window rendered headlessly, through the layer stack the
// application itself composes. The app has no offscreen mode of its own — it
// is a native window binary — but the layers its window renders are plain
// observables of layout.Widget, so composing them over a frozen theme and
// drawing them into an image produces the frame the window would show.
//
// Every still picture of this window in this package is taken here, so that
// what the pictures record is the composition the application runs and not a
// second one built to be photographed.

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/dialog"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// windowTheme is the theme a still render of this window draws from: one
// scheme frozen, the shipped radius scale, and every other field the
// package default — the shape theme/window feeds the live layers, minus the
// platform poll.
func windowTheme(c tokens.PlatformColors) rx.Observable[theme.Theme] {
	return rx.Of(golden.StaticTheme(c, golden.WithRadius(goldenRadius)))
}

// windowTokens is the snapshot the window's own frame-time closures read,
// the counterpart of the live token mirror: the same scheme the theme
// above emits, and the pinned shaper, so the rasterisation cannot depend on
// which faces the host carries. The scroll bar setting is stated here for
// that same reason — a still picture of this window holds the overlay bar
// every desktop platform draws, whatever the machine running it asked for.
func windowTokens(c tokens.PlatformColors) themeTokens {
	return themeTokens{
		col:        c,
		typ:        tokens.DefaultTypography,
		sp:         tokens.Spacing,
		den:        tokens.Comfortable,
		shaper:     tokens.DefaultTypography.DeterministicShaper(),
		scrollBars: desktop.ScrollBarsWhenScrolling,
	}
}

// windowMemory is the arrangement a still render opens on: this window's
// own defaults, read from a path that holds nothing and written back to a
// directory the test owns. A render arranges a window nobody is sitting at,
// so the delay is longer than any test run and nothing is ever written.
func windowMemory(t *testing.T) *shell.ArrangementMemory {
	t.Helper()
	return shell.OpenArrangement(filepath.Join(t.TempDir(), "layout.json"),
		time.Hour, defaultArrangement(), frameLimits)
}

// windowLayers is the layer stack the vault window renders, back to front:
// the window's own plane and the vault screen on it. The model does not
// move and the theme emits once, which is what makes the frame a still
// picture.
//
// still carries the one number a frame off a window cannot measure and
// takes back what each frame arranged; see [stillWindow].
func windowLayers(t *testing.T, m Model, c tokens.PlatformColors, still *stillWindow) []rx.Observable[layout.Widget] {
	t.Helper()
	th := windowTheme(c)
	tok := windowTokens(c)
	return []rx.Observable[layout.Widget]{
		backdropLayer(th),
		vaultLayer(th, rx.Of(m), func() Model { return m }, func() themeTokens { return tok },
			windowMemory(t), still),
	}
}

// switchLayers is that stack with the vault-switch dialog over it, which is
// where the application puts it: a decision takes the window entire rather
// than standing under the chrome row.
func switchLayers(t *testing.T, m Model, c tokens.PlatformColors, still *stillWindow) []rx.Observable[layout.Widget] {
	t.Helper()
	th := windowTheme(c)
	tok := windowTokens(c)
	return append(windowLayers(t, m, c, still),
		vaultPickerLayer(th, rx.Of(m), func() Model { return m },
			func() themeTokens { return tok }, dialog.NewArbiter()))
}

// newStill opens a still render of this window: the control buttons pinned
// where a live macOS window measured them once, and nothing arranged yet.
func newStill() *stillWindow {
	return &stillWindow{ButtonsEnd: goldenLeading}
}

// windowFrame composes the window's layer stack into one frame.
func windowFrame(t *testing.T, m Model, c tokens.PlatformColors, still *stillWindow) layout.Widget {
	t.Helper()
	return golden.Layers(t, windowLayers(t, m, c, still)...)
}

// windowShot draws the window at the size it opens at and hands back the
// picture beside the arrangement the frame it drew laid out.
func windowShot(t *testing.T, m Model, c tokens.PlatformColors) (*image.RGBA, shell.PaneLayout) {
	t.Helper()
	return windowShotAt(t, m, c, windowFrameSize)
}

// windowShotAt is [windowShot] at a stated size, for the readings that are
// about how the composition answers a window of another shape.
func windowShotAt(t *testing.T, m Model, c tokens.PlatformColors, size image.Point) (*image.RGBA, shell.PaneLayout) {
	t.Helper()
	still := newStill()
	img := golden.Window(t, size, windowLayers(t, m, c, still)...)
	return img, still.Arrangement
}

// windowDriver drives the composed window through a real router: frames on
// demand, keys between them, and a capture of the frame the last of them
// left behind. It is how a picture is taken of a state no model holds —
// what the reader has typed into the window's own field, and which of its
// matches they have stepped onto.
type windowDriver struct {
	t *testing.T
	w layout.Widget
	r *gioinput.Router
}

// newWindowDriver composes the window and settles it: one frame to register
// the tags everything else is addressed to.
func newWindowDriver(t *testing.T, m Model, c tokens.PlatformColors) *windowDriver {
	t.Helper()
	d := &windowDriver{t: t, w: windowFrame(t, m, c, newStill()), r: new(gioinput.Router)}
	d.frame()
	return d
}

func (d *windowDriver) frame() {
	var ops op.Ops
	d.w(layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(windowFrameSize),
		Ops:         &ops,
		Source:      d.r.Source(),
	})
	d.r.Frame(&ops)
}

// frames runs n of them, which is how an answer that takes a frame to
// reach the model and another to be drawn is waited for.
func (d *windowDriver) frames(n int) {
	for range n {
		d.frame()
	}
}

func (d *windowDriver) press(k key.Event) {
	d.r.Queue(k)
	d.frames(2)
}

// typeFind puts the query in the window's find in the page the way the
// reader does: the platform's find shortcut takes the keyboard into the
// field, the query goes in, and one Enter steps onto the second of what it
// found.
func (d *windowDriver) typeFind(q string) {
	d.press(key.Event{Name: findKey, Modifiers: key.ModShortcut, State: key.Press})
	d.r.Queue(key.EditEvent{Text: q})
	d.frames(3)
	// The query arrives as one edit, which leaves the caret where the edit
	// began; a reader typing it ends after the last letter.
	d.press(key.Event{Name: key.NameEnd, State: key.Press})
	d.press(key.Event{Name: key.NameReturn, State: key.Press})
	d.frames(2)
}

func (d *windowDriver) capture() *image.RGBA {
	d.t.Helper()
	return golden.Capture(d.t, windowFrameSize, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = d.r.Source()
		return d.w(gtx)
	})
}
