package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/popover"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// railKeyFrameSize is the column the rail's keyboard is driven in below. The
// keys are the rail's and not any row's, so nothing about the reading depends
// on what the column is wide or tall enough to draw.
var railKeyFrameSize = image.Pt(200, 400)

// TestTheRailsArrowsWalkItsRowsAndReturnOpensOne drives the rail's keys
// through a real input.Router and reads the row they stand on.
//
// A list is a focusable wherever it stands, so the rail takes the keyboard on
// one tag of its own and the arrows walk its rows across the sections that
// stand open — the second section is collapsed here, so the walk carries its
// heading and none of its rows. Return does not move: it answers the feed the
// cursor stands on, which is what the column turns into a SelectFeed.
func TestTheRailsArrowsWalkItsRowsAndReturnOpensOne(t *testing.T) {
	groups := hardCodedGroups()
	open := map[int]bool{0: true, 2: true}
	run := railRowRun(len(groups), groups, open)
	if len(run) < 3 {
		t.Fatalf("the fixture's open sections carry %d rows; the walk below needs three", len(run))
	}
	for _, r := range run {
		if r.Section == 1 && r.Index >= 0 {
			t.Fatalf("the run carries %q from the collapsed section; the arrows would walk a row the rail does not draw", r.ID)
		}
	}
	if run[0].Section == run[len(run)-1].Section {
		t.Fatalf("every row in the run stands in section %d; the walk never crosses a heading", run[0].Section)
	}
	if run[0].Index >= 0 {
		t.Fatalf("the run opens on a feed row; a section's heading is the first thing the arrows reach")
	}

	keys := newRailKeys()
	r := new(gioinput.Router)
	var focused bool
	var answer railAnswer
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(railKeyFrameSize),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Source:      r.Source(),
		}
		area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
		event.Op(gtx.Ops, keys.Focus())
		area.Pop()
		if !focused {
			gtx.Execute(key.FocusCmd{Tag: keys.Focus()})
			focused = true
		}
		answer = keys.update(gtx, run, open, "")
		r.Frame(ops)
	}
	press := func(name key.Name) {
		r.Queue(key.Event{Name: name, State: key.Press})
		frame()
	}
	frame() // registers the tag and asks for the keyboard
	frame() // the keyboard has arrived

	for _, c := range []struct {
		what string
		name key.Name
		want railRow
	}{
		{"Down with nothing walked to lands on the first row", key.NameDownArrow, run[0]},
		{"Down again walks one row on", key.NameDownArrow, run[1]},
		{"End lands on the last row, past the collapsed section's rows", key.NameEnd, run[len(run)-1]},
		{"Down at the last row stays there, as nothing wraps", key.NameDownArrow, run[len(run)-1]},
		{"Up walks back one row", key.NameUpArrow, run[len(run)-2]},
		{"Home lands on the first row", key.NameHome, run[0]},
		{"Up at the first row stays there", key.NameUpArrow, run[0]},
		{"Down lands on the first feed under the heading", key.NameDownArrow, run[1]},
	} {
		press(c.name)
		if keys.cursor != cursorOf(c.want) {
			t.Errorf("%s: the cursor reads %+v, want %+v", c.what, keys.cursor, cursorOf(c.want))
		}
	}

	press(key.NameReturn)
	if !answer.Opened {
		t.Fatal("Return opened no feed; the rail answers the keys but not the one that opens a row")
	}
	if answer.Feed != run[1].ID {
		t.Errorf("Return opened %q, want the row the cursor stands on, %q", answer.Feed, run[1].ID)
	}
	if keys.cursor != cursorOf(run[1]) {
		t.Errorf("Return moved the cursor to %+v; opening a feed does not walk the rail", keys.cursor)
	}
}

// railFoldFrame drives the rail's keys through a real input.Router and keeps
// the sections' disclosure as the window does: a key's answer flips the
// section it names, and the next frame walks the run that flip leaves.
type railFoldFrame struct {
	t       *testing.T
	keys    *railKeys
	router  *gioinput.Router
	groups  []feedGroup
	open    map[int]bool
	run     []railRow
	focused bool
	answer  railAnswer
}

func newRailFoldFrame(t *testing.T) *railFoldFrame {
	t.Helper()
	groups := hardCodedGroups()
	open := map[int]bool{}
	for i := range groups {
		open[i] = true
	}
	f := &railFoldFrame{
		t:      t,
		keys:   newRailKeys(),
		router: new(gioinput.Router),
		groups: groups,
		open:   open,
	}
	f.run = railRowRun(len(groups), groups, open)
	return f
}

func (f *railFoldFrame) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(railKeyFrameSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      f.router.Source(),
	}
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, f.keys.Focus())
	area.Pop()
	if !f.focused {
		gtx.Execute(key.FocusCmd{Tag: f.keys.Focus()})
		f.focused = true
	}
	f.answer = f.keys.update(gtx, f.run, f.open, "")
	if f.answer.Section >= 0 {
		f.open[f.answer.Section] = !f.open[f.answer.Section]
		f.run = railRowRun(len(f.groups), f.groups, f.open)
	}
	f.router.Frame(ops)
}

// focus settles the keyboard on the rail.
func (f *railFoldFrame) focus() {
	f.t.Helper()
	f.frame() // registers the tag and asks for the keyboard
	f.frame() // the keyboard has arrived
}

func (f *railFoldFrame) press(name key.Name) {
	f.t.Helper()
	f.router.Queue(key.Event{Name: name, State: key.Press})
	f.frame()
}

// heading is the cursor standing on the given section's heading.
func heading(section int) railCursor { return railCursor{Section: section} }

// firstFeed is the first feed the given section carries.
func (f *railFoldFrame) firstFeed(section int) railCursor {
	f.t.Helper()
	if section >= len(f.groups) || len(f.groups[section].Entries) == 0 {
		f.t.Fatalf("section %d carries no feed for the walk to land on", section)
	}
	return railCursor{Section: section, ID: f.groups[section].Entries[0].ID}
}

func (f *railFoldFrame) readsCursor(what string, want railCursor) {
	f.t.Helper()
	if f.keys.cursor != want {
		f.t.Errorf("%s: the cursor reads %+v, want %+v", what, f.keys.cursor, want)
	}
}

func (f *railFoldFrame) readsOpen(what string, section int, want bool) {
	f.t.Helper()
	if f.open[section] != want {
		f.t.Errorf("%s: section %d reads open=%v, want %v", what, section, f.open[section], want)
	}
}

// TestLeftAndRightCloseAndOpenTheRailsSections drives the platform's outline
// keys over the rail through a real input.Router and reads the sections'
// disclosure and the cursor after each.
//
// A rail's sections open and close on their own, so what these two keys move
// is one section's disclosure and the cursor: Left closes the section the
// cursor stands in and lands on its heading, Right opens a collapsed one and
// steps into an open one. Nothing else in the rail answers either key.
func TestLeftAndRightCloseAndOpenTheRailsSections(t *testing.T) {
	f := newRailFoldFrame(t)
	f.focus()

	f.press(key.NameDownArrow) // the first heading
	f.press(key.NameDownArrow) // the first feed under it
	f.readsCursor("Down twice from nothing", f.firstFeed(0))

	f.press(key.NameLeftArrow)
	f.readsOpen("Left on a row inside an open section", 0, false)
	f.readsCursor("Left on a row inside an open section", heading(0))
	for _, r := range f.run {
		if r.Section == 0 && r.Index >= 0 {
			t.Fatalf("the run still carries %q from the section Left closed", r.ID)
		}
	}

	f.press(key.NameLeftArrow)
	f.readsOpen("Left again on the collapsed section's heading", 0, false)
	f.readsCursor("Left again on the collapsed section's heading", heading(0))

	f.press(key.NameRightArrow)
	f.readsOpen("Right on a collapsed section's heading", 0, true)
	f.readsCursor("Right on a collapsed section's heading", heading(0))

	f.press(key.NameRightArrow)
	f.readsOpen("Right on an open section's heading", 0, true)
	f.readsCursor("Right on an open section's heading", f.firstFeed(0))

	f.press(key.NameRightArrow)
	f.readsCursor("Right on a feed row", f.firstFeed(0))
	f.readsOpen("Right on a feed row", 0, true)

	f.press(key.NameUpArrow)
	f.press(key.NameLeftArrow)
	f.readsOpen("Left on an open section's heading", 0, false)
	f.readsCursor("Left on an open section's heading", heading(0))

	// A collapsed section's rows are out of the walk, so Down from its
	// heading reaches the next section's.
	f.press(key.NameDownArrow)
	f.readsCursor("Down from a collapsed section's heading", heading(1))

	// Return on a heading is what a click on it is: the section's disclosure
	// and nothing else.
	f.press(key.NameReturn)
	if f.answer.Opened {
		t.Errorf("Return on a heading opened feed %q; a heading opens no feed", f.answer.Feed)
	}
	f.readsOpen("Return on an open section's heading", 1, false)
	f.readsCursor("Return on an open section's heading", heading(1))
}

// TestAClickOnARailRowHandsTheRailTheKeys reads the composed window in both
// appearances: a click on a feed row puts the keyboard on the rail, so the
// row the cursor stands on wears the accent pill, and an arrow then walks
// that pill to the row below while the open feed falls back to the grey one.
//
// A gioui.org/widget.Clickable does not take the keyboard when it is clicked,
// so the row asks for it; without that request the arrows would land wherever
// the reader last tabbed and the rail would answer nothing.
func TestAClickOnARailRowHandsTheRailTheKeys(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			w := windowFrame(t, tc.c, tokens.Comfortable, settledModel())
			r := new(gioinput.Router)
			ops := new(op.Ops)
			drive := func() {
				ops.Reset()
				w(layout.Context{
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(windowSize),
					Ops:         ops,
					Source:      r.Source(),
				})
				r.Frame(ops)
			}
			read := func(p image.Point) color.NRGBA {
				drive()
				img := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
					gtx.Source = r.Source()
					return w(gtx)
				})
				if img == nil {
					return color.NRGBA{}
				}
				return at(img, p.X, p.Y)
			}
			accent := patsidebar.SelectionFill(tc.c, false)
			grey := patsidebar.SelectionFill(tc.c, true)

			drive() // registers the rail's tags
			on := f32.Pt(float32(atOpenFeed.X), float32(atOpenFeed.Y))
			for _, kind := range []pointer.Kind{pointer.Press, pointer.Release} {
				r.Queue(pointer.Event{
					Kind:     kind,
					Source:   pointer.Mouse,
					Position: on,
					Buttons:  pointer.ButtonPrimary,
				})
				drive()
			}

			clicked := read(atOpenFeed)
			if (clicked == color.NRGBA{}) {
				return // headless unavailable; Capture called t.Skip
			}
			if clicked != accent {
				t.Errorf("after a click on its row the open feed reads %v, want the accent pill %v: the click did not hand the rail the keys", clicked, accent)
			}

			r.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
			drive()
			if got := read(atRestingFeed); got != accent {
				t.Errorf("after Down the row below reads %v, want the accent pill %v: the arrow did not walk the rail", got, accent)
			}
			if got := read(atOpenFeed); got != grey {
				t.Errorf("after Down the open feed reads %v, want the grey pill %v: the row the keys left keeps the accent", got, grey)
			}
		})
	}
}

// railTabFrame lays the rail's own column out and, after it, one more
// focusable of its own — the control that stands next in the window's
// reading order. It answers where the keyboard is after each frame.
type railTabFrame struct {
	keys     *railKeys
	router   *gioinput.Router
	sections []railSection
	headings []gesture.Click
	open     map[int]bool
	scroll   *list.State
	kb       railKeyboard

	sentinel struct{ _ byte }

	onRail     bool
	onSentinel bool
}

// newRailTabFrame builds the rail with its real rows: the click targets under
// test are the rows' own, so a stub run of rows would answer nothing.
func newRailTabFrame(t *testing.T) *railTabFrame {
	t.Helper()
	groups := hardCodedGroups()
	open := map[int]bool{}
	for i := range groups {
		open[i] = true
	}
	th := rx.Of(staticTheme(tokens.PlatformLight, tokens.Comfortable))
	keys := newRailKeys()
	sections := make([]railSection, len(groups))
	for i, g := range groups {
		entries := g.Entries
		sections[i] = railSection{
			Title: g.Title,
			Rows: feedEntryRows(th, func() []feedEntry { return entries },
				func() FeedID { return "" }, keys, popover.NewArbiter()),
		}
	}
	f := &railTabFrame{
		keys:     keys,
		router:   new(gioinput.Router),
		sections: sections,
		headings: make([]gesture.Click, len(sections)),
		open:     open,
		scroll:   list.NewState(),
	}
	f.kb = railKeyboard{Keys: keys, Rows: railRowRun(len(sections), groups, open)}
	return f
}

func (f *railTabFrame) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(railTabFrameSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      f.router.Source(),
	}
	drawRailColumn(gtx, tokens.PlatformLight, tokens.DefaultTypography,
		f.sections, f.headings, f.open, f.scroll, f.kb)
	// The next focusable after the rail, standing where the window's own
	// next control stands: what Tab must reach once the rail lets go.
	area := clip.Rect{Max: image.Pt(1, 1)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &f.sentinel)
	area.Pop()
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &f.sentinel}); !ok {
			break
		}
	}
	f.onRail = gtx.Focused(f.keys.Focus())
	f.onSentinel = gtx.Focused(&f.sentinel)
	f.router.Frame(ops)
}

func (f *railTabFrame) tab() {
	f.router.MoveFocus(key.FocusForward)
	f.frame()
}

// railTabFrameSize is tall enough for every row the fixture's open sections
// carry, so nothing in the reading below turns on a row being scrolled out.
var railTabFrameSize = image.Pt(200, 800)

// railTabStops is how many forward focus moves the column is given to reach
// the rail. The bound is loose so a focusable added in front of the rail
// fails the assertion rather than this loop.
const railTabStops = 20

// TestTabLeavesTheRailForTheNextFocusable drives forward focus moves through
// a real input.Router over the rail's own column.
//
// A list is one focusable wherever it stands: the rail takes the keys on its
// one tag, and the very next move hands them on to what stands after it, as
// the platform's sidebar does. A focus filter per row — which is what a
// widget.Clickable registers — would instead walk Tab through the rows one by
// one, and the arrows would go dead the moment it did.
func TestTabLeavesTheRailForTheNextFocusable(t *testing.T) {
	f := newRailTabFrame(t)
	f.frame() // registers the tags

	moves := 0
	for !f.onRail {
		if moves == railTabStops {
			t.Fatalf("no forward focus move inside %d put the keyboard on the rail", railTabStops)
		}
		f.tab()
		moves++
	}
	if moves != 1 {
		t.Errorf("the rail took the keyboard on move %d; it stands first in the column, so it takes them on the first", moves)
	}

	f.tab()
	if f.onRail {
		t.Error("one move on from the rail the keys are still on it: the rail is more than one focus target, so Tab walks its rows")
	}
	if !f.onSentinel {
		t.Error("the focusable after the rail did not take the keys: the move landed on something inside the rail")
	}
}

// TestTheArrowsStillWalkAfterTabLeaves reads the other half: the rail's rows
// are walked by the arrows and not by Tab, so a rail the keys have come back
// to still answers them.
func TestTheArrowsStillWalkAfterTabLeaves(t *testing.T) {
	f := newRailTabFrame(t)
	f.frame()
	for !f.onRail {
		f.tab()
	}
	f.tab() // away
	f.router.MoveFocus(key.FocusBackward)
	f.frame()
	if !f.onRail {
		t.Fatal("a backward move did not bring the keys back to the rail")
	}
	f.router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	f.frame()
	if f.keys.cursor != cursorOf(f.kb.Rows[0]) {
		t.Errorf("Down put the cursor on %+v, want the first row %+v: the arrows no longer walk the rail", f.keys.cursor, cursorOf(f.kb.Rows[0]))
	}
}

// TestARailHeadingAnswersThePointer reads the heading's own target after it
// stopped being a focusable: the block that collapses a section is operated
// by the pointer, so a press over it has to reach the heading's gesture.
func TestARailHeadingAnswersThePointer(t *testing.T) {
	f := newRailTabFrame(t)
	f.frame() // registers the tags

	strip := int(pane.StripDp)
	head := int(patsidebar.SectionHeight)
	on := f32.Pt(float32(railTabFrameSize.X)/2, float32(strip+head/2))
	f.router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Position: on,
		Buttons:  pointer.ButtonPrimary,
	})
	f.frame()
	if !f.headings[0].Pressed() {
		t.Fatal("a press over the first heading reached nothing; the block that collapses a section has no pointer target")
	}
	f.router.Queue(pointer.Event{
		Kind:     pointer.Release,
		Source:   pointer.Mouse,
		Position: on,
		Buttons:  pointer.ButtonPrimary,
	})
	f.frame()
	if f.headings[0].Pressed() {
		t.Error("the heading is still held after the release; its click was never drained")
	}
	// The press put no keyboard anywhere: a heading is not a focus target,
	// and the rail's own tag is the only one the column carries.
	if f.onRail {
		t.Error("a press on a heading handed the rail the keys; only a row's click does that")
	}
}
