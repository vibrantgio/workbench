package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
)

// findOpenFrame drives the whole window through one router under the
// window's own redraw rule: a frame happens because something asked for
// one, and the three things that ask are an event reaching a handler that
// asked for it, a command asking the driver, and — the loop's own route — a
// message reduced into a new model, which re-emits the layers and
// invalidates the window. A view that changes what it draws by none of the
// three draws the change on whatever later frame something else happens to
// cause.
type findOpenFrame struct {
	t    *testing.T
	f    *frameState
	find pageFind
	tag  findFieldTag
	m    Model
	tok  themeTokens
	ops  op.Ops
	r    input.Router

	posted []mvu.Message

	// asked is whether the last frame left the window asking for another,
	// and drawn whether the band's search field laid out on it.
	asked bool
	drawn bool
}

func newFindOpenFrame(t *testing.T) *findOpenFrame {
	t.Helper()
	b := &findOpenFrame{t: t, f: newFrameState(defaultArrangement()), m: goldenModel(), tok: goldenTokens()}
	b.f.leading = func() unit.Dp { return goldenLeading }
	// Every message this frame posts, from the band's own controls and from
	// the find alike: the loop collects them all, reduces them, and the
	// re-emission invalidates the window.
	b.f.post = func(_ layout.Context, msg mvu.Message) { b.posted = append(b.posted, msg) }
	b.find.post = b.f.post
	b.find.tag = &b.tag
	b.f.band = bandFind{find: &b.find, field: b.field}
	return b
}

// field stands in for the search field the band lays out: it registers a
// target that takes the keyboard exactly as the field's editor does, and
// draws nothing.
func (b *findOpenFrame) field(gtx layout.Context) layout.Dimensions {
	b.drawn = true
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &b.tag}); !ok {
			break
		}
	}
	area := clip.Rect{Max: image.Pt(1, 1)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &b.tag)
	area.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
}

func (b *findOpenFrame) frame() {
	b.ops.Reset()
	b.drawn = false
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(windowW, windowH)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      b.r.Source(),
		Ops:         &b.ops,
	}
	b.f.layout(gtx, b.m, b.tok, paneSlot(nil), fillSlot, nil)
	b.r.Frame(&b.ops)
	// What the window itself consults after a frame: a pending event or a
	// command's own request for the next one.
	_, b.asked = b.r.WakeupTime()
	for _, msg := range b.posted {
		b.m, _ = Update(b.m, msg)
		b.asked = true
	}
	b.posted = nil
}

// follow lays out the frame the window asked for after the last one, with
// no further input, and fails where it asked for none.
func (b *findOpenFrame) follow() {
	b.t.Helper()
	if !b.asked {
		b.t.Fatal("the frame that took the click left the window asking for nothing: no message, no invalidation, so the window draws on whatever later frame something else causes")
	}
	b.frame()
}

// clickCapsule presses the magnifier capsule at the trailing end of the
// band, which is where the composition test reads it.
func (b *findOpenFrame) clickCapsule() {
	b.t.Helper()
	at := f32.Pt(float32(windowW-toolbarTrailDp-railToggleWidthDp/2), float32(paneStripDp)/2)
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	b.frame()
}

// TestOneClickOnTheBandsMagnifierOpensTheField is the owner's finding: one
// click on the magnifier, the frame that click asks for, and the field is
// standing in the band with the keyboard in it. Nothing else is pressed,
// moved or typed in between.
func TestOneClickOnTheBandsMagnifierOpensTheField(t *testing.T) {
	b := newFindOpenFrame(t)
	b.frame()
	b.frame() // the capsule's hit area is now known to the router
	if b.drawn {
		t.Fatal("the band drew the search field before anybody asked for it")
	}

	b.clickCapsule()
	b.follow()

	if !b.drawn {
		t.Error("the band drew no search field on the frame the click asked for; one click on the magnifier has to open it")
	}
	if !b.r.Source().Focused(&b.tag) {
		t.Error("the field the click opened does not hold the keyboard")
	}
}

// TestTheFindLeavesTheBandForTheLengthOfAFetch records the rule rather
// than a defect: the platform's find acts on a document, and while the
// window is fetching one out of the cloud it has none — the column says so
// in place of a note, and the keys the field would answer have nothing to
// move. So the control is away for as long as the wait, and stands again
// the moment the note lands.
func TestTheFindLeavesTheBandForTheLengthOfAFetch(t *testing.T) {
	f := newFrameState(defaultArrangement())
	tok := goldenTokens()

	waiting := fetchingWindow(t)
	if waiting.CurrentNote() != nil {
		t.Fatal("the window has a note while it is fetching one")
	}
	if f.findTrailing(waiting, tok) != nil {
		t.Error("the band offers a find while the window is fetching the note it would search")
	}

	// The file lands and is read: the window has a document again, and the
	// control comes back with it.
	landed := cacheNote(waiting, noteFromSource(waiting.Current, "# Cloudy\n\nText.\n"))
	landed.Downloading = ""
	if landed.CurrentNote() == nil {
		t.Fatal("the note did not land")
	}
	if f.findTrailing(landed, tok) == nil {
		t.Error("the band keeps the find away from a window that has a note to search")
	}
}
