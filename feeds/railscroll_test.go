package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/patterns/pane"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// railRows is one section's run of rows as the scroll test stands them: n
// rows at the sidebar's own row height, reporting what they took rather than
// the room they were offered, which is the contract feedEntryListBody keeps.
func railRows(n int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, n*gtx.Dp(patsidebar.RowHeight))}
	}
}

// TestRailScrollsToItsFoot lays the rail out shorter than its sections and
// scrolls it. A rail with every section open is taller than the window it
// stands in, and a column whose foot never comes into view has entries the
// reader cannot open — so the rail's column is a scroll area, and a real
// pointer.Scroll delivered through an input.Router has to move it.
func TestRailScrollsToItsFoot(t *testing.T) {
	sections := []railSection{
		{Title: "News", Rows: railRows(12)},
		{Title: "Tech", Rows: railRows(12)},
		{Title: "Long reads", Rows: railRows(12)},
	}
	headings := make([]widget.Clickable, len(sections))
	open := map[int]bool{0: true, 1: true, 2: true}
	blocks := len(railBlocks(sections, open))
	scroll := list.NewState()

	// Short: the strip, one heading and a few rows. Every section stands
	// open, so the column is several times this.
	size := image.Pt(int(patsidebar.ExpandedWidth), int(pane.StripDp)+120)

	r := new(gioinput.Router)
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Source:      r.Source(),
		}
		drawRailColumn(gtx, tokens.PlatformLight, tokens.DefaultTypography,
			sections, headings, open, scroll, railKeyboard{Keys: newRailKeys()})
		r.Frame(ops)
	}

	frame() // register the scroll area's own tags
	if pos := scroll.Position(); !pos.BeforeEnd {
		t.Fatalf("precondition: the rail fits its %v viewport; there is nothing to scroll", size)
	}
	if pos := scroll.Position(); pos.First != 0 || pos.Offset != 0 {
		t.Fatalf("precondition: the rail starts at %+v, want its leading edge", pos)
	}

	// A wheel over the rail's column.
	at := f32.Pt(float32(size.X)/2, float32(size.Y)-10)
	scrollBy := func(dy float32) {
		r.Queue(pointer.Event{
			Kind:     pointer.Scroll,
			Source:   pointer.Mouse,
			Position: at,
			Scroll:   f32.Pt(0, dy),
		})
		frame()
	}

	scrollBy(60)
	moved := scroll.Position()
	if moved.First == 0 && moved.Offset == 0 {
		t.Fatal("a wheel over the rail moved nothing; the column does not scroll")
	}

	// Far enough that the foot arrives, whatever the rows measure.
	for range 40 {
		if !scroll.Position().BeforeEnd {
			break
		}
		scrollBy(120)
	}
	pos := scroll.Position()
	if pos.BeforeEnd {
		t.Errorf("after scrolling to the end the rail still reports content below: %+v", pos)
	}
	if got := pos.First + pos.Count; got != blocks {
		t.Errorf("the rail's foot shows blocks up to %d of %d; the last section never comes into view", got, blocks)
	}
}
