package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestTheListKeepsItsScrollPositionWhenATodoIsTicked renders the page the
// content layer builds for a list taller than the window, scrolls it to the
// end with the mouse wheel, and then renders the page the layer builds once the last todo is
// ticked. The first visible row is the same in both: the list's state is kept
// across the layer's emissions, so a change to a todo does not draw the list
// from its first row again.
func TestTheListKeepsItsScrollPositionWhenATodoIsTicked(t *testing.T) {
	var before Model
	for i := range 40 {
		before.List = append(before.List, Todo{Id: i, Text: fmt.Sprintf("Todo %d", i)})
	}
	last := before.List[len(before.List)-1].Id
	after, _ := Update(before, ToggleTodo{Id: last})
	if !after.List[len(after.List)-1].Completed {
		t.Fatalf("ticking todo %d left it open", last)
	}

	th := staticThemed(tokens.PlatformLight)
	models, modelObs := rx.Subject[Model](0, 1)
	built := make(chan layout.Widget, 4)
	sub := ContentLayer(rx.Of(th.components), modelObs, desktop.ScrollBarsWhenScrolling).
		Tap(func(page layout.Widget, err error, done bool) {
			if !done {
				built <- page
			}
		}).Go()
	defer sub.Unsubscribe()
	next := func() layout.Widget {
		t.Helper()
		select {
		case page := <-built:
			return page
		case <-time.After(5 * time.Second):
			t.Fatal("the content layer built no page for the model")
			return nil
		}
	}

	frame := func(page layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(windowSize)
			return page(gtx)
		}
	}
	// The top half of the window, short of the bar at its trailing edge: the
	// rows there are the same before and after the tick only if the list
	// keeps its position, as the ticked last row is drawn below it.
	band := image.Rect(0, 0, windowSize.X*3/4, windowSize.Y/2)
	same := func(a, b *image.RGBA) bool {
		for y := band.Min.Y; y < band.Max.Y; y++ {
			for x := band.Min.X; x < band.Max.X; x++ {
				if pixelAt(a, image.Pt(x, y)) != pixelAt(b, image.Pt(x, y)) {
					return false
				}
			}
		}
		return true
	}

	models.Next(before)
	page := next()
	atStart := golden.Capture(t, windowSize, frame(page))
	scrollToEnd(page)
	scrolled := golden.Capture(t, windowSize, frame(page))
	if same(atStart, scrolled) {
		t.Fatalf("scrolling to the end of %d todos left the first rows in place; the list is not taller than the window", len(before.List))
	}

	models.Next(after)
	ticked := golden.Capture(t, windowSize, frame(next()))
	if !same(scrolled, ticked) {
		if same(atStart, ticked) {
			t.Errorf("after ticking the last todo the list is drawn from its first row again, want the rows scrolled to before the tick")
		} else {
			t.Errorf("after ticking the last todo the rows at the top of the window changed, want the rows scrolled to before the tick")
		}
	}
}

// scrollToEnd lays page out under a router and scrolls the list under the
// pointer further than its length, so the list's state rests at its end. The
// state is the one the content layer keeps; the test has no other way to
// move it.
func scrollToEnd(page layout.Widget) {
	var r input.Router
	var ops op.Ops
	lay := func() {
		ops.Reset()
		page(layout.Context{
			Constraints: layout.Exact(windowSize),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         &ops,
			Source:      r.Source(),
		})
		r.Frame(&ops)
	}
	lay()
	r.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Source:   pointer.Mouse,
		Position: f32.Pt(float32(windowSize.X/2), float32(windowSize.Y/2)),
		Scroll:   f32.Pt(0, 100000),
	})
	lay()
	lay()
}
