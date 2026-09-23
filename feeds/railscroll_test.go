package main

import (
	"image"
	"strconv"
	"testing"

	"gioui.org/f32"
	"gioui.org/gesture"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/patterns/pane"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// railRows is one section's rows as the scroll test stands them: n rows, each
// one block of the rail's column at the sidebar's own row height, which is
// the contract feedEntryRows keeps.
func railRows(n int) railRowSet {
	return railRowSet{
		Count: func() int { return n },
		Row: func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(patsidebar.RowHeight))}
		},
		Drain: func(layout.Context) {},
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
	headings := make([]gesture.Click, len(sections))
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

// TestTheRailRevealsTheCursorsOwnRow walks the rail's keys to the foot of a
// section taller than the window and reads where the column stopped.
//
// The column scrolls by blocks, and a row is one of them: revealing the row
// the cursor stands on is what brings that row into view. A block holding a
// section's whole run instead would be revealed by its head, so a cursor at
// the foot of a long section would leave the reader looking at the section's
// first rows and not at the row the arrows walked to.
func TestTheRailRevealsTheCursorsOwnRow(t *testing.T) {
	const rows = 12
	run := make([]railRow, rows)
	for i := range run {
		run[i] = railRow{Section: 0, Index: i, ID: FeedID("feed-" + strconv.Itoa(i))}
	}
	sections := []railSection{{Title: "News", Rows: railRows(rows)}}
	headings := make([]gesture.Click, len(sections))
	open := map[int]bool{0: true}
	scroll := list.NewState()
	keys := newRailKeys()
	kb := railKeyboard{Keys: keys, Rows: run}

	// Short: the strip, the heading and four rows. The section is three
	// times that, so its foot comes into view only by scrolling.
	size := image.Pt(int(patsidebar.ExpandedWidth),
		int(pane.StripDp)+int(patsidebar.SectionHeight)+4*int(patsidebar.RowHeight))

	r := new(gioinput.Router)
	var focused bool
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Source:      r.Source(),
		}
		drawRailColumn(gtx, tokens.PlatformLight, tokens.DefaultTypography,
			sections, headings, open, scroll, kb)
		if !focused {
			gtx.Execute(key.FocusCmd{Tag: keys.Focus()})
			focused = true
		}
		r.Frame(ops)
	}

	frame() // registers the rail's tags and asks for the keyboard
	frame() // the keyboard has arrived
	if pos := scroll.Position(); !pos.BeforeEnd {
		t.Fatalf("precondition: the rail fits its %v viewport; there is nothing to reveal", size)
	}

	blocks := railBlocks(sections, open)
	if got, want := len(blocks), 1+rows; got != want {
		t.Fatalf("the column carries %d blocks, want the heading and one per row (%d)", got, want)
	}

	r.Queue(key.Event{Name: key.NameEnd, State: key.Press})
	frame() // the keys move the cursor and ask for the reveal
	frame() // the reveal lands
	if keys.cursor != run[len(run)-1].ID {
		t.Fatalf("End put the cursor on %q, want the last row %q", keys.cursor, run[len(run)-1].ID)
	}

	// The last row is the column's last block, the heading standing first
	// and one block per row after it. The reading is taken against that
	// rather than against railRowBlock's answer, so a wrong answer there
	// fails the reading instead of moving it.
	at := len(blocks) - 1
	if got, ok := railRowBlock(blocks, run, keys.cursor); !ok || got != at {
		t.Errorf("railRowBlock puts the last row at block %d (found %v), want the column's last block %d", got, ok, at)
	}
	pos := scroll.Position()
	if at < pos.First || at >= pos.First+pos.Count {
		t.Errorf("the cursor stands on block %d and the column shows blocks %d to %d: the rail revealed the section and not the row",
			at, pos.First, pos.First+pos.Count-1)
	}
}
