package main

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// TestWidthsRoundTrip asserts the whole point of the file: what one run
// left is what the next run opens at.
func TestWidthsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vaultview", widthFile)
	want := columnWidths{Rail: 260, Aside: 420}
	if err := saveWidths(path, want); err != nil {
		t.Fatalf("saving widths: %v", err)
	}
	if got := openWidths(path, widthDelay).widths(); got != want {
		t.Errorf("opened at %+v, want the kept %+v", got, want)
	}
}

// TestWidthsFallBackOnDefaults covers every way of having nothing to
// apply. They differ in cause and not in consequence: the window opens on
// its own defaults, and nothing about the file is trusted further.
func TestWidthsFallBackOnDefaults(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		content string
		write   bool
	}{
		{name: "no file"},
		{name: "not JSON", content: "{", write: true},
		{name: "not a number", content: `{"rail": "wide"}`, write: true},
		{name: "not finite", content: `{"rail": 1e999, "aside": 320}`, write: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name, widthFile)
			if c.write {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := openWidths(path, widthDelay).widths(); got != defaultWidths() {
				t.Errorf("opened at %+v, want the defaults %+v", got, defaultWidths())
			}
		})
	}
}

// TestWidthsClampOnRestore asserts that what the file says is not obeyed
// past what the window can lay out. A file is a text file and a reader can
// edit it; a rail wider than the window is not an arrangement.
func TestWidthsClampOnRestore(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		kept columnWidths
		want columnWidths
	}{
		{"too narrow", columnWidths{Rail: 1, Aside: 1}, columnWidths{Rail: railMinWidthDp, Aside: frameMinAsideDp}},
		{"too wide", columnWidths{Rail: 9000, Aside: 9000}, columnWidths{Rail: railMaxWidthDp, Aside: frameMaxAsideDp}},
		{"zero", columnWidths{}, columnWidths{Rail: railMinWidthDp, Aside: frameMinAsideDp}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name, widthFile)
			if err := saveWidths(path, c.kept); err != nil {
				t.Fatal(err)
			}
			if got := openWidths(path, widthDelay).widths(); got != c.want {
				t.Errorf("opened at %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestWidthsWriteOnce asserts the debounce: a drag hands in a width per
// frame and the file is written once, at the end. The opening arrangement
// is not a change and writes nothing at all, which is what keeps a launch
// the reader only looked at from touching the file.
func TestWidthsWriteOnce(t *testing.T) {
	var (
		mu      sync.Mutex
		written []columnWidths
	)
	m := &columnMemory{delay: time.Millisecond}
	m.write = func(w columnWidths) error {
		mu.Lock()
		defer mu.Unlock()
		written = append(written, w)
		return nil
	}

	m.record(defaultWidths())
	for w := unit.Dp(320); w >= 300; w-- {
		m.record(columnWidths{Rail: treeWidthDp, Aside: w})
	}
	m.stop()

	mu.Lock()
	defer mu.Unlock()
	if len(written) != 1 {
		t.Fatalf("%d writes for one drag, want 1: %+v", len(written), written)
	}
	if want := (columnWidths{Rail: treeWidthDp, Aside: 300}); written[0] != want {
		t.Errorf("wrote %+v, want where the drag ended, %+v", written[0], want)
	}
}

// TestWidthsOpeningArrangementWritesNothing separates the opening
// arrangement from a change to it. Opening a window and closing it must
// leave the file exactly as it was found — an untouched launch that
// rewrote the file would turn a damaged one into a silent reset.
func TestWidthsOpeningArrangementWritesNothing(t *testing.T) {
	var writes int
	m := &columnMemory{delay: time.Millisecond}
	m.write = func(columnWidths) error { writes++; return nil }
	for range 10 {
		m.record(defaultWidths())
	}
	m.stop()
	if writes != 0 {
		t.Errorf("%d writes with nothing moved, want none", writes)
	}
}

// TestWidthsNoMemory asserts the nil receiver every method takes: a frame
// laid out for measurement has no memory behind it, and must neither open
// on a reader's arrangement nor write over one.
func TestWidthsNoMemory(t *testing.T) {
	var m *columnMemory
	if got := m.widths(); got != defaultWidths() {
		t.Errorf("no memory opened at %+v, want the defaults %+v", got, defaultWidths())
	}
	m.record(columnWidths{Rail: 300, Aside: 300})
	m.stop()
}

// TestWidthsFileShape pins the file's own shape: two named widths, so
// that a reader opening it in an editor can see what it says and a later
// version can tell what it is reading.
func TestWidthsFileShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), widthFile)
	if err := saveWidths(path, columnWidths{Rail: 240, Aside: 320}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]float64
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("the file is not JSON: %v", err)
	}
	want := map[string]float64{"rail": 240, "aside": 320}
	if len(fields) != len(want) {
		t.Errorf("file holds %v, want exactly %v", fields, want)
	}
	for k, v := range want {
		if fields[k] != v {
			t.Errorf("%q is %v, want %v", k, fields[k], v)
		}
	}
}

// TestTheAsideIsKeptWhereItWasDragged is the whole path in one test: a
// window opens on the kept arrangement, the aside's boundary is dragged,
// the file takes the new width as the window closes, and the next window
// opens where the drag left it.
func TestTheAsideIsKeptWhereItWasDragged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vaultview", widthFile)
	tok := goldenTokens()
	size := image.Pt(windowW, windowH)

	var ops op.Ops
	var r input.Router
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
		Ops:         &ops,
	}
	memory := openWidths(path, time.Millisecond)
	f := newFrameState(memory.widths())
	f.widths = memory
	f.leading = func() unit.Dp { return goldenLeading }
	frame := func() {
		ops.Reset()
		f.layout(gtx, goldenModel(), tok, nil, nil, nil)
		r.Frame(&ops)
	}
	frame()

	// The boundary's own middle, and a drag toward the leading edge, which
	// is the direction that widens the aside.
	const pull = 60
	x := float32(size.X) - float32(f.asideW) - float32(frameSplitterDp)/2
	y := float32(f.geom.rowTop + f.geom.rowH/2)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, y)})
	frame()
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x-pull, y)})
	frame()
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x-pull, y)})
	frame()

	want := columnWidths{Rail: treeWidthDp, Aside: frameAsideDp + pull}
	if got := (columnWidths{Rail: f.railW, Aside: f.asideW}); got != want {
		t.Fatalf("the drag left the frame at %+v, want %+v", got, want)
	}

	// The window closes, which writes whatever is still pending.
	memory.stop()
	if got := openWidths(path, widthDelay).widths(); got != want {
		t.Errorf("the next window opens at %+v, want where the drag left it, %+v", got, want)
	}
}
