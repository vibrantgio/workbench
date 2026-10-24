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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/table"
	"github.com/vibrantgio/theme/theme"
)

// drawnExtent returns the box the two images differ in: the leading and
// trailing column and the top and bottom row of the difference, the empty
// rectangle where they are the same everywhere.
func drawnExtent(a, b *image.RGBA) image.Rectangle {
	var box image.Rectangle
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// TestUnreadTooltipIsDrawnWholeAtTheWindowsTrailingEdge lays the list
// column's rows, as the window builds them, against the window's trailing
// edge, the way the arrangement places a column: the window's content area
// is set as the available room at the root, and the column is laid out at
// its offset with the area moved by it. The Unread header is then the
// control at the window's trailing edge, and its tooltip, wider than the
// header, is drawn whole inside the window.
func TestUnreadTooltipIsDrawnWholeAtTheWindowsTrailingEdge(t *testing.T) {
	m := initialModel()
	th := rx.Of(theme.Default())
	rowsObs := rx.Map(articlesMain(th,
		rx.Of(m.selectedFeed), rx.Of(m.selectedArticle), rx.Of(m.currentPage),
		rx.Of[table.Sort](m.sort), rx.Of(m.rowsPerPage), rx.Of(m.unreadOnly), rx.Of(m.query),
		tooltip.NewArbiter(),
	), func(s articleSlots) layout.Widget { return s.Rows })
	rows, err := golden.Resolve(rowsObs)
	if err != nil || rows == nil {
		t.Fatalf("the list column's rows: %v", err)
	}

	size := image.Pt(900, 400)
	const colW = 420
	colAt := image.Pt(size.X-colW, 0)
	window := func(gtx layout.Context) layout.Dimensions {
		gtx = shell.WithAvailableRoom(gtx, image.Rectangle{Max: gtx.Constraints.Max})
		st := op.Offset(colAt).Push(gtx.Ops)
		cgtx := tooltip.AvailableRoomAt(gtx, colAt)
		cgtx.Constraints = layout.Exact(image.Pt(colW, size.Y))
		rows(cgtx)
		st.Pop()
		return layout.Dimensions{Size: size}
	}

	r := new(gioinput.Router)
	t0 := time.Unix(100, 0)
	frame := func(now time.Time) {
		ops := new(op.Ops)
		window(layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Now:         now,
			Source:      r.Source(),
		})
		r.Frame(ops)
	}
	shot := func(now time.Time) *image.RGBA {
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Now = now
			gtx.Source = r.Source()
			return window(gtx)
		})
	}

	frame(t0)
	r.Queue(pointer.Event{
		Kind:     pointer.Move,
		Position: f32.Pt(float32(size.X-unreadColWDp/2), tableHeaderHDp/2),
		Source:   pointer.Mouse,
	})
	frame(t0)
	waiting := shot(t0.Add(tooltip.DefaultDelay - 10*time.Millisecond))
	shown := t0.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	frame(shown)
	box := drawnExtent(waiting, shot(shown))
	if box.Empty() {
		t.Fatal("the delay ran out and no tooltip was drawn under the Unread header")
	}
	if box.Min.X < 1 || box.Max.X > size.X-1 {
		t.Errorf("the Unread tooltip spans columns %d..%d in a window %d wide; it is cut off by the window's trailing edge",
			box.Min.X, box.Max.X-1, size.X)
	}
}
