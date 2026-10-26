package main

// The Monitor tab before the first reading: one empty state in the tab's
// content frame — the line and the demo button under it, centred as one
// group — read off a render of the whole window, because the frame the group
// centres in is the arrangement's.

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/mvu"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// unconnectedModel is the window as it opens with no device reachable: on
// the Monitor tab, no reading, the demo not running, the connection status
// the failed connect left.
func unconnectedModel() Model {
	return Model{Screen: "monitor", Status: "no serial adapter found", EditPreset: noEdit}
}

// renderUnconnected draws the whole window with no device reachable. Two
// frames are drawn and the second kept, for the reason renderWindow does.
func renderUnconnected(t *testing.T, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	w := windowFrame(t, c, unconnectedModel())
	golden.Capture(t, windowSize, w)
	return golden.Capture(t, windowSize, w)
}

// monitorFrame is the Monitor tab's content frame, read off the
// arrangement: it begins on the row under the tab strip's trim, which is
// the hairline inside the strip's foot, spans the strip's width, and ends
// at the page's own padding at the window's foot, where the tabs end when
// no notice stands under them.
func monitorFrame(t *testing.T, img *image.RGBA, c tokens.PlatformColors) image.Rectangle {
	t.Helper()
	return monitorFrameIn(t, img, c, windowSize)
}

// monitorFrameIn is [monitorFrame] in a window of the given size.
func monitorFrameIn(t *testing.T, img *image.RGBA, c tokens.PlatformColors, windowSize image.Point) image.Rectangle {
	t.Helper()
	fill := pageFill(c)
	trim := vgcolor.Flatten(c.Separator, fill)
	mid := windowSize.X / 2
	row := -1
	for y := 0; y < windowSize.Y; y++ {
		if sameColour(at(img, mid, y), trim) {
			row = y
			break
		}
	}
	if row < 0 {
		t.Fatal("no tab strip trim crosses the window's middle column")
	}
	left, right := -1, -1
	for x := 0; x < windowSize.X; x++ {
		if !sameColour(at(img, x, row), fill) {
			if left < 0 {
				left = x
			}
			right = x + 1
		}
	}
	return image.Rect(left, row+1, right, windowSize.Y-int(Padding))
}

// sameColour answers whether two painted colours are the same, alpha aside.
func sameColour(a, b color.NRGBA) bool { return a.R == b.R && a.G == b.G && a.B == b.B }

// paintedRuns are the runs of rows inside the frame that hold any pixel not of
// the fill, top to bottom: the line and the button, in that order, because
// the gap between the group's parts is bare rows.
func paintedRuns(img *image.RGBA, frame image.Rectangle, fill color.NRGBA) []image.Rectangle {
	var runs []image.Rectangle
	open := false
	for y := frame.Min.Y; y < frame.Max.Y; y++ {
		box := image.Rectangle{}
		for x := frame.Min.X; x < frame.Max.X; x++ {
			if sameColour(at(img, x, y), fill) {
				continue
			}
			px := image.Rect(x, y, x+1, y+1)
			if box.Empty() {
				box = px
			} else {
				box = box.Union(px)
			}
		}
		switch {
		case box.Empty():
			open = false
		case open:
			runs[len(runs)-1] = runs[len(runs)-1].Union(box)
		default:
			runs, open = append(runs, box), true
		}
	}
	return runs
}

// TestMonitorEmptyStateCentresInTheTabsFrame holds the group's placement:
// the line and the button stand as two parts, and the block they make
// centres across and down in the tab's content frame.
//
// The block's top is the line's glyphs and not its line box, so the block
// read off the pixels stands a few pixels lower than the group the pattern
// centres; the old placement, anchored at the tab's top, stood hundreds of
// pixels higher.
func TestMonitorEmptyStateCentresInTheTabsFrame(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderUnconnected(t, tc.c)
			frame := monitorFrame(t, img, tc.c)
			runs := paintedRuns(img, frame, pageFill(tc.c))
			if len(runs) != 2 {
				t.Fatalf("the tab's frame %v holds %d runs of painted rows %v, want the line and the button", frame, len(runs), runs)
			}
			block := runs[0].Union(runs[1])
			if got, want := block.Min.X+block.Max.X, frame.Min.X+frame.Max.X; abs(got-want) > 2 {
				t.Errorf("the group centres at %.1f across, want the tab frame's %.1f", float64(got)/2, float64(want)/2)
			}
			if got, want := block.Min.Y+block.Max.Y, frame.Min.Y+frame.Max.Y; abs(got-want) > 6 {
				t.Errorf("the group centres at %.1f down, want the tab frame's %.1f", float64(got)/2, float64(want)/2)
			}
			for i, r := range runs {
				if c := r.Min.X + r.Max.X; abs(c-(frame.Min.X+frame.Max.X)) > 2 {
					t.Errorf("part %d centres at %.1f across, want the frame's %.1f", i, float64(c)/2, float64(frame.Min.X+frame.Max.X)/2)
				}
			}
		})
	}
}

// TestMonitorEmptyStateLineSpendsTertiaryLabel pins the colour the line's
// glyphs plateau at: the platform's tertiary label flattened over the tab's
// fill, which is the fill the strip stands on. A fully covered pixel is
// painted the flattened colour exactly.
func TestMonitorEmptyStateLineSpendsTertiaryLabel(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderUnconnected(t, tc.c)
			fill := pageFill(tc.c)
			frame := monitorFrame(t, img, tc.c)
			runs := paintedRuns(img, frame, fill)
			if len(runs) == 0 {
				t.Fatal("the tab's frame is bare, want the empty state's line")
			}
			var got color.NRGBA
			far := -1
			for y := runs[0].Min.Y; y < runs[0].Max.Y; y++ {
				for x := runs[0].Min.X; x < runs[0].Max.X; x++ {
					c := at(img, x, y)
					if d := abs(int(c.R)-int(fill.R)) + abs(int(c.G)-int(fill.G)) + abs(int(c.B)-int(fill.B)); d > far {
						far, got = d, c
					}
				}
			}
			want := vgcolor.Flatten(tc.c.TertiaryLabel, fill)
			if !sameColour(got, want) {
				t.Errorf("the line's glyphs plateau at #%02x%02x%02x, want TertiaryLabel over the tab's fill, #%02x%02x%02x",
					got.R, got.G, got.B, want.R, want.G, want.B)
			}
		})
	}
}

// TestMonitorEmptyStateDemoButtonPostsEnterDemo drives the live demo button
// the window lays out through a real input.Router, at the site read off the
// render, and reads back what the window's post was handed.
func TestMonitorEmptyStateDemoButtonPostsEnterDemo(t *testing.T) {
	c := tokens.PlatformLight
	img := renderUnconnected(t, c)
	runs := paintedRuns(img, monitorFrame(t, img, c), pageFill(c))
	if len(runs) != 2 {
		t.Fatalf("the tab's frame holds %d runs of painted rows, want the line and the button", len(runs))
	}
	button := runs[1]

	var posted []mvu.Message
	w := windowFramePosting(t, c, unconnectedModel(), func(_ layout.Context, msg mvu.Message) {
		posted = append(posted, msg)
	})
	var (
		ops op.Ops
		r   gioinput.Router
	)
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
	frame()
	mid := f32.Pt(float32(button.Min.X+button.Max.X)/2, float32(button.Min.Y+button.Max.Y)/2)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: mid, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: mid, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	frame()
	frame()
	found := false
	for _, msg := range posted {
		if _, ok := msg.(EnterDemo); ok {
			found = true
		}
	}
	if !found {
		t.Errorf("a press on the demo button at %v posted %v, want a main.EnterDemo", mid, posted)
	}
}

// TestMonitorEmptyStateWaitsWithoutActions holds the second state: once a
// device or the demo answers and no reading has come, the tab stands its
// own line alone, with no description and no button.
func TestMonitorEmptyStateWaitsWithoutActions(t *testing.T) {
	for _, m := range []Model{
		{Screen: "monitor", Online: true, Status: "connected", EditPreset: noEdit},
		{Screen: "monitor", Demo: true, Status: "starting demo…", EditPreset: noEdit},
	} {
		p := monitorEmptyState(m, slotSet{})
		if p.Line != "Waiting for the first reading…" || p.Description != "" || len(p.Actions) != 0 {
			t.Errorf("online %v demo %v: the empty state is %q / %q with %d actions, want the waiting line alone",
				m.Online, m.Demo, p.Line, p.Description, len(p.Actions))
		}
	}
}

// TestMonitorEmptyStateHasNoDescription holds the empty state while no
// device is connected: the line and the demo button, and no description,
// because the header's status line already shows the connection status.
func TestMonitorEmptyStateHasNoDescription(t *testing.T) {
	m := unconnectedModel()
	p := monitorEmptyState(m, slotSet{})
	if p.Line != "No SK150 Connected" || p.Description != "" || len(p.Actions) != 1 {
		t.Errorf("with the status %q the empty state is %q / %q with %d actions, want the line, no description and the demo button",
			m.Status, p.Line, p.Description, len(p.Actions))
	}
}

// TestMonitorEmptyStateDemoButtonTooltip holds the demo button's tooltip:
// its text is the sentence, and a pointer resting on the button through a
// real input.Router shows the tooltip's surface under the button once the
// tooltip's delay has passed, and not before.
func TestMonitorEmptyStateDemoButtonTooltip(t *testing.T) {
	if demoTooltip != "Shows demo readings until you quit" {
		t.Errorf("the demo button's tooltip text is %q, want %q", demoTooltip, "Shows demo readings until you quit")
	}
	c := tokens.PlatformLight
	fill := pageFill(c)
	img := renderUnconnected(t, c)
	runs := paintedRuns(img, monitorFrame(t, img, c), fill)
	if len(runs) != 2 {
		t.Fatalf("the tab's frame holds %d runs of painted rows, want the line and the button", len(runs))
	}
	button := runs[1]

	w := windowFrame(t, c, unconnectedModel())
	var (
		ops op.Ops
		r   gioinput.Router
	)
	start := time.Unix(1_700_000_000, 0)
	now := start
	ctx := func(ops *op.Ops) layout.Context {
		return layout.Context{
			Constraints: layout.Exact(windowSize),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Source:      r.Source(),
			Now:         now,
		}
	}
	frame := func() {
		ops.Reset()
		w(ctx(&ops))
		r.Frame(&ops)
	}
	// below is the band under the button the surface takes: the rows
	// between the button's foot and the foot of the tab's frame, across the
	// button's width.
	below := image.Rect(button.Min.X, button.Max.Y+1, button.Max.X, button.Max.Y+40)
	painted := func() bool {
		shot := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
			gtx.Source = r.Source()
			gtx.Now = now
			return w(gtx)
		})
		for y := below.Min.Y; y < below.Max.Y; y++ {
			for x := below.Min.X; x < below.Max.X; x++ {
				if !sameColour(at(shot, x, y), fill) {
					return true
				}
			}
		}
		return false
	}

	frame()
	frame()
	mid := f32.Pt(float32(button.Min.X+button.Max.X)/2, float32(button.Min.Y+button.Max.Y)/2)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: mid, Source: pointer.Mouse})
	frame()
	frame()
	now = start.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	frame()
	if painted() {
		t.Errorf("before the tooltip's delay the band under the demo button %v holds painted pixels, want it bare", below)
	}
	now = start.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	frame()
	if !painted() {
		t.Errorf("after the tooltip's delay the band under the demo button %v is bare, want the tooltip's surface", below)
	}
}

// TestMonitorEmptyStateDemoTooltipIsDrawnWholeInANarrowWindow narrows the
// window until the demo button extends to its trailing padding and rests the
// pointer on the button. The tooltip's surface with its text whole is wider
// than the window; it is drawn whole inside the window, because the
// window's content area is set as the available room at the root and moved
// by every layer down to the empty state.
func TestMonitorEmptyStateDemoTooltipIsDrawnWholeInANarrowWindow(t *testing.T) {
	c := tokens.PlatformLight
	size := image.Pt(180, windowSize.Y)
	fill := pageFill(c)
	w := windowFrame(t, c, unconnectedModel())
	golden.Capture(t, size, w)
	img := golden.Capture(t, size, w)
	// The line is set on more lines in a frame this narrow, so the button is
	// the last run of painted rows and the line is every run above it.
	runs := paintedRuns(img, monitorFrameIn(t, img, c, size), fill)
	if len(runs) < 2 {
		t.Fatalf("the tab's frame holds %d runs of painted rows, want the line and the button", len(runs))
	}
	button := runs[len(runs)-1]

	var (
		ops op.Ops
		r   gioinput.Router
	)
	start := time.Unix(1_700_000_000, 0)
	now := start
	frame := func() {
		ops.Reset()
		w(layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         &ops,
			Source:      r.Source(),
			Now:         now,
		})
		r.Frame(&ops)
	}
	shot := func() *image.RGBA {
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Source = r.Source()
			gtx.Now = now
			return w(gtx)
		})
	}

	frame()
	frame()
	mid := f32.Pt(float32(button.Min.X+button.Max.X)/2, float32(button.Min.Y+button.Max.Y)/2)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: mid, Source: pointer.Mouse})
	frame()
	frame()
	now = start.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	frame()
	waiting := shot()
	now = start.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	frame()
	shown := shot()

	box := image.Rectangle{}
	for y := button.Max.Y; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if waiting.RGBAAt(x, y) != shown.RGBAAt(x, y) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if box.Empty() {
		t.Fatal("the delay ran out and no tooltip was drawn under the demo button")
	}
	if box.Min.X < 1 || box.Max.X > size.X-1 {
		t.Errorf("the demo button's tooltip spans columns %d..%d in a window %d wide; it is cut off by the window's edges",
			box.Min.X, box.Max.X-1, size.X)
	}
}
