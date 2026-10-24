package main

import (
	"context"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// tipNoteSource is the note the tooltip tests read: one line of prose
// carrying one external link, so there is exactly one place the pointer can
// stand on and one address to be said about it.
const tipNoteSource = "Read [the site](https://example.test/page) for more.\n"

// tipPad drives the note column with the link tooltip the window gives it,
// at an instant the test chooses: the annotation appears after a dwell, so
// nothing about it can be read from a frame clock the test does not hold.
type tipPad struct {
	t    *testing.T
	m    Model
	tok  themeTokens
	doc  *markdown.Document
	tip  *linkTip
	size image.Point
	now  time.Time

	// pageW is the note column's width, laid against the window's
	// trailing edge; zero lays the column across the whole window.
	pageW int

	propClick widget.Clickable
	read      reader
	arr       arrival
	find      pageFind
	r         input.Router
	ops       op.Ops
}

func newTipPad(t *testing.T, address string) *tipPad {
	t.Helper()
	m := goldenModel()
	source := "Read [the site](" + address + ") for more.\n"
	m = cacheNote(m, noteFromSource("guide/Tip.md", source))
	m.Current = "guide/Tip.md"
	if m.CurrentNote() == nil {
		t.Fatal("the fixture model has no current note")
	}
	p := &tipPad{
		t:    t,
		m:    m,
		size: image.Pt(760, 520),
		now:  time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC),
		tok: themeTokens{
			col:    tokens.PlatformLight,
			typ:    tokens.DefaultTypography,
			sp:     tokens.Spacing,
			den:    tokens.Comfortable,
			shaper: tokens.DefaultTypography.DeterministicShaper(),
		},
	}
	p.doc = markdown.NewDocument(m.CurrentNote().Blocks)
	p.tip = &linkTip{}
	p.tip.layer = tipWidget(t, address, p.tip)
	return p
}

// tipWidget is the one tooltip the window builds for an address, from the
// props vaultLayer builds it from, with the pinned shaper.
func tipWidget(t *testing.T, address string, tip *linkTip) layout.Widget {
	t.Helper()
	var w layout.Widget
	props := linkTipProps(address, tooltip.NewArbiter(), tip)
	props.Shaper = tokens.DefaultTypography.DeterministicShaper()
	obs := tooltip.Tooltip(rx.Of(theme.Default()), props)
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("subscribe to the tooltip: %v", err)
	}
	if w == nil {
		t.Fatal("the tooltip emitted no layout.Widget")
	}
	return w
}

func (p *tipPad) page(gtx layout.Context) layout.Dimensions {
	return layoutNotePage(gtx, p.m, p.tok, &p.propClick, &p.read, &p.arr,
		&docCursor{}, func(Model, *Note) *markdown.Document { return p.doc }, &p.find, p.tip)
}

// window lays the note column out as the window does: the window's content
// area set as the available room at the root, and the column placed at its
// offset with the area moved by it.
func (p *tipPad) window(gtx layout.Context) layout.Dimensions {
	gtx = shell.WithAvailableRoom(gtx, image.Rectangle{Max: gtx.Constraints.Max})
	if p.pageW <= 0 {
		return p.page(gtx)
	}
	off := image.Pt(p.size.X-p.pageW, 0)
	defer op.Offset(off).Push(gtx.Ops).Pop()
	gtx = attachment.AvailableRoomAt(gtx, off)
	gtx.Constraints = layout.Exact(image.Pt(p.pageW, p.size.Y))
	p.page(gtx)
	return layout.Dimensions{Size: p.size}
}

func (p *tipPad) frame(evs ...event.Event) {
	p.r.Queue(evs...)
	p.ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(p.size),
		Now:         p.now,
		Ops:         &p.ops,
		Source:      p.r.Source(),
	}
	p.window(gtx)
	p.r.Frame(&p.ops)
}

func (p *tipPad) move(x, y int) {
	p.frame(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x), float32(y)), Source: pointer.Mouse})
}

// shot renders the column as it stands at the pad's own instant, which is
// what the annotation's dwell is measured against.
func (p *tipPad) shot() *image.RGBA {
	p.t.Helper()
	src := p.r.Source()
	now := p.now
	return golden.Capture(p.t, p.size, func(gtx layout.Context) layout.Dimensions {
		gtx.Now = now
		gtx.Source = src
		return p.window(gtx)
	})
}

// onLink is where the pointer stands on the note's one link, found from the
// shape the frame declares rather than from widths the faces decide, with
// the rows the link occupies.
func (p *tipPad) onLink() (at image.Point, top, bottom int) {
	p.t.Helper()
	for y := 1; y < p.size.Y; y += 2 {
		for x := 1; x < p.size.X; x += 3 {
			p.move(x, y)
			if p.r.Cursor() == pointer.CursorPointer {
				at = image.Pt(x, y)
				break
			}
		}
		if at != (image.Point{}) {
			break
		}
	}
	if at == (image.Point{}) {
		p.t.Fatal("the note declares no link anywhere in the column")
	}
	top, bottom = at.Y, at.Y
	for y := at.Y; y >= 0; y-- {
		p.move(at.X, y)
		if p.r.Cursor() != pointer.CursorPointer {
			break
		}
		top = y
	}
	for y := at.Y; y < p.size.Y; y++ {
		p.move(at.X, y)
		if p.r.Cursor() != pointer.CursorPointer {
			break
		}
		bottom = y
	}
	return at, top, bottom
}

// differingRows returns the first and last row the two images differ at, and
// the leading and trailing column of the difference; -1 for an image pair
// that is the same everywhere.
func differingRows(a, b *image.RGBA) (top, bottom, left, right int) {
	top, bottom, left, right = -1, -1, -1, -1
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.RGBAAt(x, y) == b.RGBAAt(x, y) {
				continue
			}
			if top < 0 {
				top = y
			}
			bottom = y
			if left < 0 || x < left {
				left = x
			}
			if x > right {
				right = x
			}
		}
	}
	return top, bottom, left, right
}

// TestTheLinkTooltipStandsBesideTheLinkAfterTheDelay hovers the note's one
// link and reads the column: nothing beside the link before the tooltip's
// own delay has run out, the annotation beside it after, and the column back
// as it was once the pointer leaves.
func TestTheLinkTooltipStandsBesideTheLinkAfterTheDelay(t *testing.T) {
	p := newTipPad(t, "https://example.test/page")

	p.frame()
	p.frame()
	at, _, bottom := p.onLink()
	// Off the link again: the column at rest, which the departure has to
	// bring back.
	p.move(p.size.X-2, at.Y)
	p.now = p.now.Add(time.Second)
	p.frame()
	rest := p.shot()

	// Onto the link. The first frames at the instant of entry are what the
	// document reports the link on and what the annotation's own hit area
	// appears in, so the dwell starts at this instant and not before it.
	entered := p.now
	p.move(at.X, at.Y)
	p.frame()
	p.frame()
	p.now = entered.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	p.frame()
	waiting := p.shot()
	if top, _, _, _ := differingRows(rest, waiting); top >= 0 && top > bottom {
		t.Errorf("the column changed at row %d, under the link, before the delay had run out; the annotation showed too early", top)
	}

	// Past the delay.
	p.now = entered.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	p.frame()
	shown := p.shot()
	top, _, left, right := differingRows(waiting, shown)
	if top < 0 {
		t.Fatal("the delay ran out and nothing was drawn")
	}
	if top <= bottom {
		t.Errorf("the annotation starts at row %d, which is not under the link's last row %d; it is not beside the link", top, bottom)
	}
	if at.X < left || at.X > right {
		t.Errorf("the annotation spans columns %d..%d, which does not hold the link at column %d", left, right, at.X)
	}

	// And away: the pointer leaves and takes the annotation with it.
	p.move(p.size.X-2, at.Y)
	p.now = p.now.Add(time.Second)
	p.frame()
	left1, _, _, _ := differingRows(rest, p.shot())
	if left1 >= 0 {
		t.Errorf("the pointer left the link and the column differs from its resting state at row %d", left1)
	}
}

// TestTheLinkTooltipSaysTheAddress stands on the same link in two notes
// whose addresses differ in length and compares what was drawn: the
// annotation is as wide as the address it says, so a longer address draws a
// wider one. A bubble of a fixed size would pass every placement check above
// without ever saying where the link goes.
func TestTheLinkTooltipSaysTheAddress(t *testing.T) {
	width := func(address string) int {
		p := newTipPad(t, address)
		p.frame()
		p.frame()
		at, _, _ := p.onLink()
		p.move(p.size.X-2, at.Y)
		p.now = p.now.Add(time.Second)
		p.frame()
		entered := p.now
		p.move(at.X, at.Y)
		p.frame()
		p.frame()
		p.now = entered.Add(tooltip.DefaultDelay - 10*time.Millisecond)
		p.frame()
		waiting := p.shot()
		p.now = entered.Add(tooltip.DefaultDelay + 10*time.Millisecond)
		p.frame()
		top, _, l, r := differingRows(waiting, p.shot())
		if top < 0 {
			t.Fatalf("no annotation was drawn for %q", address)
		}
		return r - l
	}
	short := width("https://a.test/x")
	long := width("https://a.test/a-considerably-longer-path-than-the-other-one")
	if long <= short {
		t.Errorf("the annotation is %d wide for a short address and %d for a long one; it does not say the address", short, long)
	}
}

// tab moves the keyboard on one stop and draws the frame that follows it.
func (p *tipPad) tab() {
	p.r.MoveFocus(key.FocusForward)
	p.frame()
}

// onLinkByKeyboard tabs until the note's one link holds the keyboard, which
// the column says by reporting a box for the tooltip to stand against, and
// returns how many stops that took.
func (p *tipPad) onLinkByKeyboard() int {
	p.t.Helper()
	for i := 1; i <= 12; i++ {
		p.tab()
		if !p.tip.box.Empty() {
			return i
		}
	}
	p.t.Fatal("the keyboard never reached the note's link")
	return 0
}

// TestTheLinkTooltipFollowsTheKeyboard tabs onto the note's one link and
// reads the column: nothing beside the link before the tooltip's own delay
// has run out, the annotation beside it after, and the column back as it was
// once the keyboard moves on. A link holding the keyboard says its address
// as a hovered one does.
func TestTheLinkTooltipFollowsTheKeyboard(t *testing.T) {
	p := newTipPad(t, "https://example.test/page")

	p.frame()
	p.frame()
	rest := p.shot()

	entered := p.now
	stops := p.onLinkByKeyboard()
	p.now = entered.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	p.frame()
	waiting := p.shot()

	p.now = entered.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	p.frame()
	shown := p.shot()
	top, _, _, _ := differingRows(waiting, shown)
	if top < 0 {
		t.Fatal("the link held the keyboard, the delay ran out and nothing was drawn")
	}

	// And on: the keyboard leaves the link and takes the annotation with it.
	// The focus ring the link drew while it held the keyboard goes with it
	// too, so the column is back at rest.
	for i := 0; i < stops; i++ {
		p.now = p.now.Add(time.Second)
		p.tab()
		if p.tip.box.Empty() {
			break
		}
	}
	p.now = p.now.Add(time.Second)
	p.frame()
	if row, _, _, _ := differingRows(rest, p.shot()); row >= 0 {
		t.Errorf("the keyboard left the link and the column differs from its resting state at row %d", row)
	}
}

// TestTheLinkTooltipStaysInsideANarrowWindow says a long address in a window
// narrower than it and reads the columns the annotation covers: both of its
// edges stand inside the window, because a reader cannot read what is
// outside it.
func TestTheLinkTooltipStaysInsideANarrowWindow(t *testing.T) {
	p := newTipPad(t, "https://example.test/a/considerably/longer/path/than/the/window/page.html")
	p.size = image.Pt(360, 420)

	p.frame()
	p.frame()
	at, _, _ := p.onLink()
	p.move(p.size.X-2, at.Y)
	p.now = p.now.Add(time.Second)
	p.frame()

	entered := p.now
	p.move(at.X, at.Y)
	p.frame()
	p.frame()
	p.now = entered.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	p.frame()
	waiting := p.shot()
	p.now = entered.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	p.frame()
	top, _, left, right := differingRows(waiting, p.shot())
	if top < 0 {
		t.Fatal("the delay ran out and nothing was drawn")
	}
	if left < 1 || right > p.size.X-2 {
		t.Errorf("the annotation spans columns %d..%d in a window %d wide; it is cut by the window's own edge", left, right, p.size.X)
	}
}

// TestTheLinkTooltipStandsUnderThePointedRun points at the first line of a
// link carried onto two lines and then at its last, and compares where the
// annotation landed: it stands under the line being pointed at, which is
// where the reader is looking, and not under the block around the whole
// link.
func TestTheLinkTooltipStandsUnderThePointedRun(t *testing.T) {
	p := newTipPad(t, "https://example.test/page")
	// A link whose own text is long enough to be carried onto a second line
	// in a column this narrow.
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Tip.md",
		"Read [the site with a considerably longer label than one line of this column can hold](https://example.test/page) for more.\n"))
	m.Current = "guide/Tip.md"
	p.m = m
	p.doc = markdown.NewDocument(m.CurrentNote().Blocks)
	p.size = image.Pt(420, 420)

	p.frame()
	p.frame()

	// Every row the link reaches at one column near the note's leading
	// margin: the first and the last of them are on different lines of it.
	at, _, _ := p.onLink()
	var rows []int
	for y := 0; y < p.size.Y; y++ {
		p.move(at.X, y)
		if p.r.Cursor() == pointer.CursorPointer {
			rows = append(rows, y)
		}
	}
	if len(rows) == 0 {
		t.Fatal("the note declares no link at the column the probe walked")
	}
	first, last := rows[0], rows[len(rows)-1]

	annotation := func(y int) int {
		p.t.Helper()
		p.move(p.size.X-2, y)
		p.now = p.now.Add(time.Second)
		p.frame()
		entered := p.now
		p.move(at.X, y)
		p.frame()
		p.frame()
		p.now = entered.Add(tooltip.DefaultDelay - 10*time.Millisecond)
		p.frame()
		waiting := p.shot()
		p.now = entered.Add(tooltip.DefaultDelay + 10*time.Millisecond)
		p.frame()
		top, _, _, _ := differingRows(waiting, p.shot())
		if top < 0 {
			p.t.Fatalf("no annotation was drawn for the link at row %d", y)
		}
		return top
	}

	onFirst := annotation(first)
	onLast := annotation(last)
	if onFirst >= onLast {
		t.Errorf("the annotation stands at row %d for the link's first line and at row %d for its last; it is placed against the block around the link rather than the line pointed at", onFirst, onLast)
	}
}

// TestTheLinkTooltipIsDrawnWholeAtTheWindowsTrailingEdge lays the note
// column against the window's trailing edge, narrower than the address its
// one link opens, and rests the pointer on the link. The annotation is
// placed in the window's available room and not in the column: it extends
// past the column's leading edge to say the address whole, and it is drawn
// whole inside the window, short of its trailing edge.
func TestTheLinkTooltipIsDrawnWholeAtTheWindowsTrailingEdge(t *testing.T) {
	p := newTipPad(t, "https://example.test/a/considerably/longer/path/to/the/page/than/the/note/column/holds/on/one/line/of/its/own.html")
	p.size = image.Pt(900, 420)
	p.pageW = 300
	pageX := p.size.X - p.pageW

	p.frame()
	p.frame()
	at, _, _ := p.onLink()
	p.move(pageX-10, at.Y)
	p.now = p.now.Add(time.Second)
	p.frame()

	entered := p.now
	p.move(at.X, at.Y)
	p.frame()
	p.frame()
	p.now = entered.Add(tooltip.DefaultDelay - 10*time.Millisecond)
	p.frame()
	waiting := p.shot()
	p.now = entered.Add(tooltip.DefaultDelay + 10*time.Millisecond)
	p.frame()
	top, _, left, right := differingRows(waiting, p.shot())
	if top < 0 {
		t.Fatal("the delay ran out and nothing was drawn")
	}
	if left < 1 || right > p.size.X-2 {
		t.Errorf("the annotation spans columns %d..%d in a window %d wide; it is cut off by the window's edges", left, right, p.size.X)
	}
	if left >= pageX {
		t.Errorf("the annotation spans columns %d..%d inside the note column at %d..%d; it is placed in the column and not in the window's available room",
			left, right, pageX, p.size.X-1)
	}
}
