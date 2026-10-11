package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/tokens"
)

// seePlansFrame drives the page through one router under the window's own
// redraw rule: a frame happens because something asked for one, and what
// asks is an event reaching a handler that asked for it or a command asking
// the driver. A view that changes what it draws by neither draws the change
// on whatever later frame something else happens to cause.
//
// The page is the real one — scrollingPage over its own list.State — with
// the hero section stood in for by a plain click target carrying the page's
// own handler, so the click lands where the test can put the pointer.
type seePlansFrame struct {
	t     *testing.T
	state *list.State
	click widget.Clickable
	page  layout.Widget
	ops   op.Ops
	r     input.Router
	now   time.Time

	// asked is whether the last frame left the window asking for another.
	asked bool
}

// seePlansSectionH is the height of each stand-in section: taller than the
// frame, so the list shows one section at a time and the row it seats is
// the whole of what the page draws.
const seePlansSectionH = pageFrameH

func newSeePlansFrame(t *testing.T) *seePlansFrame {
	t.Helper()
	b := &seePlansFrame{t: t, state: list.NewState(), now: time.Unix(1, 0)}
	trigger := func(gtx layout.Context) layout.Dimensions {
		if b.click.Clicked(gtx) {
			scrollToSection(b.state, pricingSection)(gtx)
		}
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(seePlansSectionH))
		gtx.Constraints = layout.Exact(size)
		return b.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			area := clip.Rect{Max: size}.Push(gtx.Ops)
			event.Op(gtx.Ops, &b.click)
			area.Pop()
			return layout.Dimensions{Size: size}
		})
	}
	blank := func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(seePlansSectionH))}
	}
	b.page = scrollingPage([]layout.Widget{trigger, blank, blank, blank}, b.state, tokens.PlatformLight)
	return b
}

func (b *seePlansFrame) frame() {
	b.ops.Reset()
	gtx := layout.Context{
		Constraints: layout.Exact(pageFrameSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      b.r.Source(),
		Ops:         &b.ops,
		Now:         b.now,
	}
	b.page(gtx)
	b.r.Frame(&b.ops)
	// What the window itself consults after a frame: a pending event or a
	// command's own request for the next one. A request for a LATER frame —
	// the overlay scrollbar's own fade asks for one a second out on every
	// frame — is not this frame asking for the next one, so the request has
	// to fall at or before the frame that made it.
	wt, ok := b.r.WakeupTime()
	b.asked = ok && !wt.After(b.now)
}

// follow lays out the frame the window asked for after the last one, with no
// further input, and fails where it asked for none.
func (b *seePlansFrame) follow() {
	b.t.Helper()
	if !b.asked {
		b.t.Fatal("the frame that took the click left the window asking for nothing: no message, no invalidation, so the window draws on whatever later frame something else causes")
	}
	b.frame()
}

func (b *seePlansFrame) clickTrigger() {
	b.t.Helper()
	at := f32.Pt(pageFrameW/2, pageFrameH/2)
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	b.frame()
}

// TestOneClickOnSeePlansMovesThePageToPricing is the page's one control that
// takes the reader somewhere: one click, the frame that click asks for, and
// the pricing section is the row the page is seated on. Nothing else is
// pressed, moved or typed in between.
func TestOneClickOnSeePlansMovesThePageToPricing(t *testing.T) {
	b := newSeePlansFrame(t)
	b.frame()
	b.frame() // the trigger's hit area is now known to the router
	if at := b.state.Position().First; at != 0 {
		t.Fatalf("the page started seated on row %d, want the hero", at)
	}

	b.clickTrigger()
	b.follow()

	if at := b.state.Position().First; at != pricingSection {
		t.Errorf("the frame the click asked for left the page seated on row %d; one click on See plans has to bring the pricing section up", at)
	}
}
