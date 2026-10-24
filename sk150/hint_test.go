package main

import (
	"image"
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
	"github.com/vibrantgio/theme/tokens"
)

// TestAHoveredHintCellShowsTheLibrarysTooltipBelowIt moves the pointer onto
// the label of the Device tab's first settings cell through a real
// input.Router and keeps it there. Once the tooltip's delay has passed the
// library tooltip is drawn below the cell, inside the window; once the
// pointer has left the cell no tooltip is drawn.
func TestAHoveredHintCellShowsTheLibrarysTooltipBelowIt(t *testing.T) {
	c := tokens.PlatformLight
	fill := pageFill(c)
	m := goldenModel()
	m.Screen = "device"
	m.HavePresets, m.HaveD = true, true
	w := windowFrame(t, c, m)
	golden.Capture(t, windowSize, w)
	img := golden.Capture(t, windowSize, w)

	// The grid's first column of labels: the heading, then the first cell's
	// label.
	frame := monitorFrame(t, img, c)
	left := int(Padding) + 4
	labels := paintedRuns(img, image.Rect(left, frame.Min.Y, left+int(compactLabelWidth), frame.Max.Y), fill)
	if len(labels) < 2 {
		t.Fatalf("the grid's label column holds %d runs of painted rows, want the heading and the first cell's label", len(labels))
	}
	label := labels[1]
	// The cell is the run of painted rows across the grid's first column
	// that holds the label: its switch is taller than the label.
	var cell image.Rectangle
	for _, run := range paintedRuns(img, image.Rect(left, frame.Min.Y, left+int(deviceColWidth), frame.Max.Y), fill) {
		if run.Overlaps(label) {
			cell = run
		}
	}
	if cell.Empty() {
		t.Fatal("no run of painted rows across the grid's first column holds the first cell's label")
	}

	var (
		ops op.Ops
		r   gioinput.Router
	)
	start := time.Unix(1_700_000_000, 0)
	now := start
	frameOnce := func() {
		ops.Reset()
		w(layout.Context{
			Constraints: layout.Exact(windowSize),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         &ops,
			Source:      r.Source(),
			Now:         now,
		})
		r.Frame(&ops)
	}
	shot := func() *image.RGBA {
		return golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
			gtx.Source = r.Source()
			gtx.Now = now
			return w(gtx)
		})
	}

	frameOnce()
	frameOnce()
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse,
		Position: f32.Pt(float32(label.Min.X+label.Max.X)/2, float32(label.Min.Y+label.Max.Y)/2)})
	frameOnce()
	frameOnce()
	now = start.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	frameOnce()
	before := shot()
	now = start.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	frameOnce()
	hovered := shot()

	surface := differing(hovered, before)
	if surface.Empty() {
		t.Fatal("the pointer rested on the cell past the tooltip's delay and no tooltip was drawn")
	}
	if surface.Min.Y < cell.Max.Y {
		t.Errorf("the tooltip spans %v, reaching above the cell's foot at y %d; it is asked for below the cell", surface, cell.Max.Y)
	}
	if !surface.In(image.Rectangle{Max: windowSize}) {
		t.Errorf("the tooltip spans %v, outside the window %v", surface, windowSize)
	}

	// The pointer moves to the window's trailing margin, beside the grid:
	// the cell is not hovered and its tooltip is not drawn.
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse,
		Position: f32.Pt(float32(windowSize.X)-30, float32(label.Min.Y+label.Max.Y)/2)})
	frameOnce()
	now = now.Add(2 * tooltip.DefaultDelay)
	frameOnce()
	if left := differing(shot(), before); left.Overlaps(surface) {
		t.Errorf("with the pointer off the cell the window differs from the unhovered one at %v, over the tooltip's place %v; want no tooltip", left, surface)
	}
}

// differing is the bounding box of the pixels where a and b differ.
func differing(a, b *image.RGBA) image.Rectangle {
	var box image.Rectangle
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}
