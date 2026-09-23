package main

import (
	"image"
	"image/color"
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

	"github.com/vibrantgio/components/golden"
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
// stand open — the second section is collapsed here, and the walk steps
// straight over it. Return does not move: it answers the feed the cursor
// stands on, which is what the column turns into a SelectFeed.
func TestTheRailsArrowsWalkItsRowsAndReturnOpensOne(t *testing.T) {
	groups := hardCodedGroups()
	open := map[int]bool{0: true, 2: true}
	run := railRowRun(len(groups), groups, open)
	if len(run) < 3 {
		t.Fatalf("the fixture's open sections carry %d rows; the walk below needs three", len(run))
	}
	for _, r := range run {
		if r.Section == 1 {
			t.Fatalf("the run carries %q from the collapsed section; the arrows would walk a row the rail does not draw", r.ID)
		}
	}
	if run[0].Section == run[len(run)-1].Section {
		t.Fatalf("every row in the run stands in section %d; the walk never crosses a heading", run[0].Section)
	}

	keys := newRailKeys()
	r := new(gioinput.Router)
	var focused bool
	var opened FeedID
	var activated bool
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
		opened, activated = keys.update(gtx, run, "")
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
		want FeedID
	}{
		{"Down with nothing walked to lands on the first row", key.NameDownArrow, run[0].ID},
		{"Down again walks one row on", key.NameDownArrow, run[1].ID},
		{"End lands on the last row, past the collapsed section", key.NameEnd, run[len(run)-1].ID},
		{"Down at the last row stays there, as nothing wraps", key.NameDownArrow, run[len(run)-1].ID},
		{"Up walks back one row", key.NameUpArrow, run[len(run)-2].ID},
		{"Home lands on the first row", key.NameHome, run[0].ID},
		{"Up at the first row stays there", key.NameUpArrow, run[0].ID},
	} {
		press(c.name)
		if keys.cursor != c.want {
			t.Errorf("%s: the cursor reads %q, want %q", c.what, keys.cursor, c.want)
		}
	}

	press(key.NameReturn)
	if !activated {
		t.Fatal("Return opened no feed; the rail answers the keys but not the one that opens a row")
	}
	if opened != run[0].ID {
		t.Errorf("Return opened %q, want the row the cursor stands on, %q", opened, run[0].ID)
	}
	if keys.cursor != run[0].ID {
		t.Errorf("Return moved the cursor to %q; opening a feed does not walk the rail", keys.cursor)
	}
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
