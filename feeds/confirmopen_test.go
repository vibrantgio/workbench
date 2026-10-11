package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/theme/theme"
)

// The trash gutter, the window it stands in, and the two places this test
// presses. The gutter is the Exact space feedEntryRows gives one row's
// delete confirm; the confirm surface hangs below it and to the leading
// side, which is why the gutter is stood well inside the window.
//
// The surface's own arithmetic is patterns/popover's: the content is
// deleteConfirmWDp wide and two deleteConfirmRowHDp rows tall, the surface
// pads it by Spacing.S3 all round, it hangs Spacing.S2 below the anchor,
// and it centres on the anchor's midline. At one pixel per dp that puts the
// "Delete" button where confirmButtonAt stands.
var (
	confirmWindow = image.Pt(400, 300)
	gutterOrigin  = image.Pt(200, 100)
	gutterSize    = image.Pt(trashColWDp, 28)
	trashAt       = f32.Pt(212, 114)
	confirmAt     = f32.Pt(212, 190)
	outsideAt     = f32.Pt(40, 40)
)

// confirmFrame drives one row's delete confirm through one router under the
// window's own redraw rule: a frame happens because something asked for one,
// and the three things that ask are an event reaching a handler that asked
// for it, a command asking the driver, and — the loop's own route — a message
// reduced into a new model, which re-emits the layers and invalidates the
// window. A view that changes what it draws by none of the three draws the
// change on whatever later frame something else happens to cause.
type confirmFrame struct {
	t   *testing.T
	dc  *deleteConfirm
	m   Model
	ops op.Ops
	r   input.Router
	now time.Time

	posted []mvu.Message

	// asked is whether the last frame left the window asking for another.
	asked bool
}

// awaitPopover returns a delete confirm whose popover layout.Widget has
// arrived: that one comes from the theme subscription on the rx goroutine,
// where the OPEN answer is the model's and is read during layout.
func awaitPopover(t *testing.T, id FeedID, confirming func() FeedID, post func(layout.Context, mvu.Message)) *deleteConfirm {
	t.Helper()
	var trash, confirm widget.Clickable
	dc := newDeleteConfirm(rx.Of(theme.Default()), id, confirming, &trash, &confirm, popover.NewArbiter(), post)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w, ok := dc.cell.Load().(layout.Widget); ok && w != nil {
			return dc
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("row %q never received a popover layout.Widget from the theme stream", id)
	return nil
}

func newConfirmFrame(t *testing.T) *confirmFrame {
	t.Helper()
	b := &confirmFrame{t: t, m: initialModel(), now: time.Unix(1, 0)}
	b.dc = awaitPopover(t, deleteConfirmFeed, func() FeedID { return b.m.confirmDelete },
		func(_ layout.Context, msg mvu.Message) { b.posted = append(b.posted, msg) })
	return b
}

// deleteConfirmFeed is the feed whose row this test operates: the first of
// the seed model's first group, so the reducer has a row to delete.
var deleteConfirmFeed = initialModel().feeds[0].Entries[0].ID

func (b *confirmFrame) frame() {
	b.ops.Reset()
	gtx := layout.Context{
		Constraints: layout.Exact(confirmWindow),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      b.r.Source(),
		Ops:         &b.ops,
		Now:         b.now,
	}
	stk := op.Offset(gutterOrigin).Push(gtx.Ops)
	ggtx := gtx
	ggtx.Constraints = layout.Exact(gutterSize)
	b.dc.layout(ggtx, true)
	stk.Pop()
	b.r.Frame(&b.ops)
	// What the window itself consults after a frame: a pending event or a
	// command's own request for the next one, which has to fall at or before
	// this frame to be this frame asking for the next one.
	wt, ok := b.r.WakeupTime()
	b.asked = ok && !wt.After(b.now)
	// And the loop's own route: every message this frame posted is reduced,
	// and the re-emission that follows invalidates the window.
	for _, msg := range b.posted {
		b.m, _ = Update(b.m, msg)
		b.asked = true
	}
	b.posted = nil
}

// follow lays out the frame the window asked for after the last one, with no
// further input, and fails where it asked for none.
func (b *confirmFrame) follow() {
	b.t.Helper()
	if !b.asked {
		b.t.Fatal("the frame that took the click left the window asking for nothing: no message, no invalidation, so the window draws on whatever later frame something else causes")
	}
	b.frame()
}

func (b *confirmFrame) click(at f32.Point) {
	b.t.Helper()
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	b.frame()
}

// TestOneClickOnARowsTrashOpensTheDeleteConfirm is the rail's own case of the
// owner's finding: one click on a row's trash, the frame that click asks for,
// and the confirm is standing under it with its Delete button taking presses.
// Nothing else is pressed, moved or typed in between.
//
// The second click is the proof that the surface was laid out on the frame
// the first one asked for: a control nothing drew takes no press, so the
// Delete button answering is the surface being there.
func TestOneClickOnARowsTrashOpensTheDeleteConfirm(t *testing.T) {
	b := newConfirmFrame(t)
	b.frame()
	b.frame() // the trash's hit area is now known to the router

	b.click(trashAt)
	b.follow()

	if b.m.confirmDelete != deleteConfirmFeed {
		t.Fatalf("the trash click left the model confirming %q, want %q", b.m.confirmDelete, deleteConfirmFeed)
	}
	b.click(confirmAt)
	if b.m.confirmDelete != "" {
		t.Error("the confirm's Delete button took no press on the frame the trash click asked for; one click has to open the confirm")
	}
	for _, g := range b.m.feeds {
		for _, e := range g.Entries {
			if e.ID == deleteConfirmFeed {
				t.Error("the confirm's Delete button did not delete the feed it stands for")
			}
		}
	}
}

// TestRowConfirmArbitratesOnTheRowTheModelNames pins the claim the goldens
// cannot make: which row's confirm stands open is the model's, read during
// layout through Props.OpenNow, so no emission stands between the message and
// the frame that draws it.
//
// Arbitration is what observes that read from outside the row. A popover
// registers its outside-press absorber only while it is open AND holds the
// window's popover top, so a press away from both rows comes back from the
// row the model names and from no other.
func TestRowConfirmArbitratesOnTheRowTheModelNames(t *testing.T) {
	confirming := FeedID("")
	var from []FeedID
	row := func(id FeedID) *deleteConfirm {
		return awaitPopover(t, id, func() FeedID { return confirming },
			func(_ layout.Context, msg mvu.Message) {
				if _, ok := msg.(CloseDeleteConfirm); ok {
					from = append(from, id)
				}
			})
	}
	a, b := row("row-a"), row("row-b")

	var ops op.Ops
	var r input.Router
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(confirmWindow),
			Source:      r.Source(),
			Ops:         &ops,
		}
		for i, dc := range []*deleteConfirm{a, b} {
			stk := op.Offset(gutterOrigin.Add(image.Pt(0, i*40))).Push(gtx.Ops)
			ggtx := gtx
			ggtx.Constraints = layout.Exact(gutterSize)
			dc.layout(ggtx, true)
			stk.Pop()
		}
		r.Frame(&ops)
	}
	press := func() {
		r.Queue(
			pointer.Event{Kind: pointer.Press, Position: outsideAt, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
			pointer.Event{Kind: pointer.Release, Position: outsideAt, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		)
		frame()
	}

	for _, want := range []FeedID{"row-a", "row-b"} {
		confirming = want
		frame()
		frame()
		from = nil
		press()
		if len(from) != 1 || from[0] != want {
			t.Errorf("with the model naming %q, the press outside came back from %v; want %q alone", want, from, want)
		}
	}
}
