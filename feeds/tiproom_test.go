package main

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
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/table"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// bandMargin is the width of the marked band drawn around the window in
// the capture: a tooltip surface placed past the window's edge paints into
// it.
const bandMargin = 64

// bandMarker is the band's colour, one the window and the tooltip never
// paint.
var bandMarker = color.NRGBA{R: 0xff, G: 0x00, B: 0xff, A: 0xff}

// surfaceBox returns the box of the tooltip's surface in window
// coordinates: every pixel that differs between the two captures and is
// the surface's fill. The edge and the shadow around the surface are not
// its pixels, so the window can cut them and leave the surface whole.
// Inside the window a fill pixel over content of the same colour does not
// differ; in the band every fill pixel differs from the marker.
func surfaceBox(waiting, shown *image.RGBA, fill color.NRGBA) image.Rectangle {
	want := color.RGBA{R: fill.R, G: fill.G, B: fill.B, A: fill.A}
	var box image.Rectangle
	bounds := shown.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if c := shown.RGBAAt(x, y); c == want && c != waiting.RGBAAt(x, y) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box.Sub(image.Pt(bandMargin, bandMargin))
}

// TestUnreadTooltipIsDrawnWholeAtTheWindowsTrailingEdge lays the list
// column's rows, as the window builds them, against the window's trailing
// edge, the way the arrangement places a column: the window's content area
// is set as the available room at the root, and the column is laid out at
// its offset with the area moved by it. The Unread header is then the
// control at the window's trailing edge, and the surface of its tooltip,
// wider than the header, stands inside the window with at least one column
// of room on each side. The window is captured inside a marked band, so a
// surface that runs past the window's edge is read in the band.
func TestUnreadTooltipIsDrawnWholeAtTheWindowsTrailingEdge(t *testing.T) {
	m := initialModel()
	th := rx.Of(theme.Default())
	rowsObs := rx.Map(articlesMain(th,
		rx.Of(m.selectedFeed), rx.Of(m.selectedArticle), rx.Of(m.currentPage),
		rx.Of[table.Sort](m.sort), rx.Of(m.rowsPerPage), rx.Of(m.unreadOnly), rx.Of(m.query),
		tooltip.NewArbiter(), desktop.ScrollBarsWhenScrolling,
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
		cgtx := attachment.AvailableRoomAt(gtx, colAt)
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
	outer := size.Add(image.Pt(2*bandMargin, 2*bandMargin))
	shot := func(now time.Time) *image.RGBA {
		return golden.Capture(t, outer, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, bandMarker, clip.Rect{Max: outer}.Op())
			st := op.Offset(image.Pt(bandMargin, bandMargin)).Push(gtx.Ops)
			wgtx := gtx
			wgtx.Constraints = layout.Exact(size)
			wgtx.Now = now
			wgtx.Source = r.Source()
			window(wgtx)
			st.Pop()
			return layout.Dimensions{Size: outer}
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
	box := surfaceBox(waiting, shot(shown), tokens.PlatformLight.WindowBackground)
	if box.Empty() {
		t.Fatal("the delay ran out and no tooltip surface was drawn under the Unread header")
	}
	if box.Min.X < 1 || box.Max.X > size.X-1 {
		t.Errorf("the Unread tooltip's surface spans columns %d..%d in a window %d wide; it is cut off by the window's trailing edge",
			box.Min.X, box.Max.X-1, size.X)
	}
}
