package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// railKeyFrameSize is the column the rail's keys are driven in below: the
// pane's own width, tall enough for every row the fixture carries.
var railKeyFrameSize = image.Pt(220, 400)

// railKeyChats is the fixture: three conversations with the first one open,
// which is where the cursor starts.
var railKeyChats = ChatList{"Alpha.jsonl", "Beta.jsonl", "Gamma.jsonl"}

// railFrame drives the rail's own column through a real router, records what
// it posts, and answers where the keyboard is after each frame.
//
// The column is laid out directly rather than through the window, for the
// reason feeds' rail is: what is under test is the rail's one focus tag and
// the rows' pointer targets, and nothing about either turns on what else the
// window draws.
type railFrame struct {
	t      *testing.T
	rail   *railState
	th     themed
	router *gioinput.Router
	posted []mvu.Message

	// sentinel is the next focusable after the rail, standing where the
	// pane's own next control stands: what a forward focus move must reach
	// once the rail lets go.
	sentinel struct{ _ byte }

	// takeKeys asks the next frame to hand the rail the keyboard, executed
	// inside a frame that lays the rail out: a focus command naming a tag the
	// frame never registered reaches nothing.
	takeKeys bool

	onRail     bool
	onSentinel bool
}

func newRailFrame(t *testing.T) *railFrame {
	t.Helper()
	return &railFrame{
		t:      t,
		rail:   newRailState(),
		th:     railThemed(t, tokens.PlatformLight),
		router: new(gioinput.Router),
	}
}

func (f *railFrame) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(railKeyFrameSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      f.router.Source(),
	}
	f.rail.layout(gtx, f.th, railKeyChats, railKeyChats[0], nil,
		func(_ layout.Context, msg mvu.Message) { f.posted = append(f.posted, msg) })
	area := clip.Rect{Max: image.Pt(1, 1)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &f.sentinel)
	area.Pop()
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &f.sentinel}); !ok {
			break
		}
	}
	if f.takeKeys {
		gtx.Execute(key.FocusCmd{Tag: f.rail.list.Focus()})
		f.takeKeys = false
	}
	f.onRail = gtx.Focused(f.rail.list.Focus())
	f.onSentinel = gtx.Focused(&f.sentinel)
	f.router.Frame(ops)
}

// focusRail hands the rail the keyboard and settles it.
func (f *railFrame) focusRail() {
	f.t.Helper()
	f.frame() // registers the rail's tags
	f.takeKeys = true
	f.frame() // asks for the keyboard
	f.frame() // the keyboard has arrived
	if !f.onRail {
		f.t.Fatal("the rail did not take the keyboard it was handed")
	}
}

func (f *railFrame) press(name key.Name) {
	f.router.Queue(key.Event{Name: name, State: key.Press})
	f.frame()
}

// TestTheRailsArrowsWalkItsConversationsAndReturnOpensOne drives the rail's
// keys through a real input.Router and reads the conversation they stand on.
//
// A list is a focusable wherever it stands, so the rail takes the keyboard on
// one tag of its own and the arrows walk its rows. The cursor starts on the
// conversation on screen — a reader who puts the keys on the rail without
// having walked them starts from the one in front of them — and Return does
// not move: it opens the conversation the cursor stands on.
func TestTheRailsArrowsWalkItsConversationsAndReturnOpensOne(t *testing.T) {
	f := newRailFrame(t)
	f.focusRail()

	if got := f.rail.cursor; got != railKeyChats[0] {
		t.Errorf("the rail opened with the cursor on %q, want the conversation on screen %q", got, railKeyChats[0])
	}

	for _, c := range []struct {
		what string
		name key.Name
		want string
	}{
		{"Down walks one row on", key.NameDownArrow, railKeyChats[1]},
		{"Down again walks one row on", key.NameDownArrow, railKeyChats[2]},
		{"Down at the last row stays there, as nothing wraps", key.NameDownArrow, railKeyChats[2]},
		{"Up walks back one row", key.NameUpArrow, railKeyChats[1]},
		{"Home lands on the first row", key.NameHome, railKeyChats[0]},
		{"Up at the first row stays there", key.NameUpArrow, railKeyChats[0]},
		{"End lands on the last row", key.NameEnd, railKeyChats[2]},
	} {
		f.press(c.name)
		if f.rail.cursor != c.want {
			t.Errorf("%s: the cursor reads %q, want %q", c.what, f.rail.cursor, c.want)
		}
	}

	f.posted = nil
	f.press(key.NameReturn)
	if len(f.posted) != 1 {
		t.Fatalf("Return posted %v; want the one message that opens the conversation the cursor stands on", f.posted)
	}
	sel, ok := f.posted[0].(SelectChat)
	if !ok || sel.Name != railKeyChats[2] {
		t.Errorf("Return posted %#v, want SelectChat{Name: %q}", f.posted[0], railKeyChats[2])
	}
	if f.rail.cursor != railKeyChats[2] {
		t.Errorf("Return moved the cursor to %q; opening a conversation does not walk the rail", f.rail.cursor)
	}
}

// TestTheRailIgnoresTheOutlineKeys reads what this rail does not answer. Its
// conversations stand in one run with nothing above them to open or close, so
// the platform's outline keys have nothing to act on: they move neither the
// cursor nor anything else, and the rail posts nothing for them.
func TestTheRailIgnoresTheOutlineKeys(t *testing.T) {
	f := newRailFrame(t)
	f.focusRail()
	f.press(key.NameDownArrow)
	if f.rail.cursor != railKeyChats[1] {
		t.Fatalf("the walk left the cursor on %q, want %q", f.rail.cursor, railKeyChats[1])
	}

	f.posted = nil
	for _, name := range []key.Name{key.NameLeftArrow, key.NameRightArrow} {
		f.press(name)
		if f.rail.cursor != railKeyChats[1] {
			t.Errorf("%v moved the cursor to %q; a rail with no groups answers neither outline key", name, f.rail.cursor)
		}
	}
	if len(f.posted) != 0 {
		t.Errorf("the outline keys posted %v; a rail with no groups answers neither", f.posted)
	}
}

// TestAClickOnAConversationRowHandsTheRailTheKeys reads the other half of the
// rail's keyboard: a row's target is a pointer gesture and takes no keyboard
// of its own, so the click asks for the rail's — without which the arrows
// would land wherever the reader last tabbed and the rail would answer
// nothing.
func TestAClickOnAConversationRowHandsTheRailTheKeys(t *testing.T) {
	f := newRailFrame(t)
	f.frame() // registers the rail's tags
	if f.onRail {
		t.Fatal("the rail holds the keyboard before anything asked it to")
	}

	// The second row, at its own middle: the rail's rows stand at the
	// sidebar's measured row height from the column's leading edge.
	rowH := float32(sidebar.RowHeight)
	on := f32.Pt(float32(railKeyFrameSize.X)/2, 1.5*rowH)
	for _, kind := range []pointer.Kind{pointer.Press, pointer.Release} {
		f.router.Queue(pointer.Event{
			Kind:     kind,
			Source:   pointer.Mouse,
			Position: on,
			Buttons:  pointer.ButtonPrimary,
		})
		f.frame()
	}
	f.frame() // the keyboard the click asked for has arrived

	if !f.onRail {
		t.Error("a click on a row left the keyboard off the rail: the click did not hand the rail the keys")
	}
	if f.rail.cursor != railKeyChats[1] {
		t.Errorf("a click on the second row left the cursor on %q, want %q", f.rail.cursor, railKeyChats[1])
	}
	var opened []string
	for _, m := range f.posted {
		if sel, ok := m.(SelectChat); ok {
			opened = append(opened, sel.Name)
		}
	}
	if len(opened) != 1 || opened[0] != railKeyChats[1] {
		t.Errorf("the click posted %v, want the one SelectChat for %q", opened, railKeyChats[1])
	}

	// And the arrows walk from where the click landed.
	f.press(key.NameDownArrow)
	if f.rail.cursor != railKeyChats[2] {
		t.Errorf("Down after the click put the cursor on %q, want the row under the one clicked, %q", f.rail.cursor, railKeyChats[2])
	}
}

// TestTabLeavesTheRailForTheNextFocusable drives forward focus moves through
// a real input.Router over the rail's own column.
//
// A list is ONE focusable wherever it stands: the rail takes the keys on its
// one tag, and the very next move hands them on to what stands after it, as
// the platform's sidebar does. A focus filter per row — which is what a
// widget.Clickable registers — would instead walk Tab through the rows one by
// one, and the arrows would go dead the moment it did.
func TestTabLeavesTheRailForTheNextFocusable(t *testing.T) {
	f := newRailFrame(t)
	f.frame() // registers the tags

	moves := 0
	for !f.onRail {
		if moves == len(railKeyChats)+4 {
			t.Fatalf("no forward focus move inside %d put the keyboard on the rail", moves)
		}
		f.router.MoveFocus(key.FocusForward)
		f.frame()
		moves++
	}
	if moves != 1 {
		t.Errorf("the rail took the keyboard on move %d; it stands first in the column, so it takes them on the first", moves)
	}

	f.router.MoveFocus(key.FocusForward)
	f.frame()
	if f.onRail {
		t.Error("one move on from the rail the keys are still on it: the rail is more than one focus target, so Tab walks its rows")
	}
	if !f.onSentinel {
		t.Error("the focusable after the rail did not take the keys: the move landed on something inside the rail")
	}
}
