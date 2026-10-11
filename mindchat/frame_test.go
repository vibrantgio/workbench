package main

import (
	"reflect"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/tokens"
)

// paneStripFrame drives the composed window through a real input.Router: the
// frame, the pane with its own top strip, and the chrome row beside it. It
// holds the clickables the window's two halves stand on, so a test can ask
// where each one answers and read back what a press on it posted.
//
// The window is composed rather than the strip laid out alone, because what
// is under test is where a mark stands in the window and which of the two
// bands it stands in — neither of which a strip laid out by itself can show.
type paneStripFrame struct {
	ops    op.Ops
	gtx    layout.Context
	r      gioinput.Router
	f      *windowFrame
	rail   *railState
	th     themed
	m      Model
	posted []mvu.Message

	// The pane's own marks and the chrome row's recalling half are separate
	// clickables, as the window keeps them.
	newChat, toggle, settings widget.Clickable
}

func newPaneStripFrame(t *testing.T, hidden bool) *paneStripFrame {
	t.Helper()
	p := &paneStripFrame{
		f:    &windowFrame{},
		rail: newRailState(),
		th:   railThemed(t, tokens.PlatformLight),
		m:    demoModel(),
	}
	// The chrome row's control is drawn at the density's own toolbar-control
	// height, which the live window reads off the theme.
	p.th.den = tokens.Comfortable
	p.th.motion = tokens.Motion
	p.m.SidebarHidden = hidden
	p.gtx = layout.Context{
		Constraints: layout.Exact(windowSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      p.r.Source(),
		Ops:         &p.ops,
	}
	return p
}

func (p *paneStripFrame) frame() {
	p.ops.Reset()
	sidebar := SidebarPane(p.th, p.m.ChatList, p.m.CurrentChat.Name, nil, p.rail,
		&p.newChat, &p.toggle, &p.settings,
		func(_ layout.Context, msg mvu.Message) { p.posted = append(p.posted, msg) })
	p.f.layout(p.gtx, p.m, p.th, sidebar, nil, nil)
	p.r.Frame(&p.ops)
}

// spanIn is one control's own columns between x0 and x1 on the line y, found
// by walking the line and asking which control the pointer is on. Both ends
// are inclusive; a control standing nowhere in the run answers -1, -1.
func (p *paneStripFrame) spanIn(x0, x1, y int, c *widget.Clickable) (int, int) {
	lo, hi := -1, -1
	for x := x0; x < x1; x++ {
		p.r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x), float32(y)), Source: pointer.Mouse})
		p.frame()
		if !c.Hovered() {
			continue
		}
		if lo < 0 {
			lo = x
		}
		hi = x
	}
	return lo, hi
}

// paneMarkStride is what one mark and the gap after it spend: the box every
// chrome mark in this window is drawn at and the pattern's measured band
// between two bare marks, which together are the 42 dp between drawn centres
// read off voicememos-multi-folder-2026-09-18.png.
const paneMarkStride = int(paneMarkDp) + pane.MarkGapDp

// TestThePaneStripStandsNewChatBeforeTheToggle reads the pane's top strip off
// a composed window: new chat, then the toggle at the trailing corner, each
// the bare mark box wide, their drawn centres the measured 42 dp apart, and
// the toggle's own box one margin in from the pane's trailing edge where the
// Pane pattern keeps it.
//
// New chat stands nowhere in the band beside the pane: it adds to the list
// the pane shows, so it waits for the pane, and the band is walked end to end
// and must not answer for it.
func TestThePaneStripStandsNewChatBeforeTheToggle(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	p := newPaneStripFrame(t, false)
	p.frame()
	p.frame()

	region := pane.Bounds(p.gtx, windowSize, SidebarWidth, false)
	stripY := region.Min.Y + p.gtx.Dp(unit.Dp(pane.StripDp))/2
	newLo, newHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.newChat)
	togLo, togHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.toggle)

	box := p.gtx.Dp(paneMarkDp)
	for _, c := range []struct {
		name   string
		lo, hi int
	}{
		{"new chat", newLo, newHi},
		{"the toggle", togLo, togHi},
	} {
		if c.lo < 0 {
			t.Fatalf("%s stands nowhere in the pane's strip", c.name)
		}
		if w := c.hi - c.lo + 1; w != box {
			t.Errorf("%s is %d dp wide, want the bare mark box's %d", c.name, w, box)
		}
	}
	if newHi >= togLo {
		t.Errorf("the strip runs new chat %d-%d, the toggle %d-%d; it reads new chat, then the toggle",
			newLo, newHi, togLo, togHi)
	}
	if got := togLo - newLo; got != paneMarkStride {
		t.Errorf("new chat and the toggle stand %d dp between centres, want the measured %d", got, paneMarkStride)
	}
	if got, want := region.Max.X-togHi-1, PaneMargin; got != want {
		t.Errorf("the toggle ends %d dp from the pane's trailing edge, want the pattern's margin of %d", got, want)
	}

	// The band beside the pane, walked end to end on the line its own
	// controls stand on.
	if lo, hi := p.spanIn(region.Max.X, windowSize.X, int(WindowButtonCenter), &p.newChat); lo >= 0 {
		t.Errorf("new chat answers the band at %d-%d; it stands on the pane and nowhere else", lo, hi)
	}
}

// TestThePaneMarkPostsANewChat presses the strip's leading mark where the
// strip stands it and reads back what the pane posted. Where it stands is
// stated from the pane's trailing edge rather than read off the layout: the
// toggle one margin in, and the mark before it one stride further.
func TestThePaneMarkPostsANewChat(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	p := newPaneStripFrame(t, false)
	p.frame()
	p.frame()

	region := pane.Bounds(p.gtx, windowSize, SidebarWidth, false)
	box := p.gtx.Dp(paneMarkDp)
	x := float32(region.Max.X - PaneMargin - box/2 - paneMarkStride)
	at := f32.Pt(x, float32(region.Min.Y+p.gtx.Dp(unit.Dp(pane.StripDp))/2))
	p.posted = nil
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	p.frame()
	p.frame()
	if !postedOneOf(p.posted, NewChat{}) {
		t.Errorf("a press on new chat at %v posted %v, want a main.NewChat", at, p.posted)
	}
}

// postedOneOf reports whether a message of want's own kind is among what the
// pane posted.
func postedOneOf(got []mvu.Message, want mvu.Message) bool {
	for _, m := range got {
		if reflect.TypeOf(m) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}

// TestWithThePaneAwayTheRowHoldsTheToggleAlone reads the chrome row in the
// collapsed state: the toggle that brings the pane back at its leading end,
// and not one stand-in for the mark that went with the pane. Only the toggle
// travels, which is what this platform's own sidebars do — so while the pane
// is away the window offers a new chat nowhere, and a screen reader is told
// of no such control either.
func TestWithThePaneAwayTheRowHoldsTheToggleAlone(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	p := newPaneStripFrame(t, true)
	p.frame()
	p.frame()

	y := int(WindowButtonCenter)
	lo, _ := p.spanIn(0, windowSize.X, y, &p.f.rowToggle)
	if lo < 0 {
		t.Fatal("the chrome row holds no toggle to bring the pane back")
	}
	if want := int(stripLead(buttonsEndDp)); lo != want {
		t.Errorf("the row's toggle leads at %d, want the leading end past the window buttons at %d", lo, want)
	}
	if clo, chi := p.spanIn(0, windowSize.X, y, &p.newChat); clo >= 0 {
		t.Errorf("new chat answers the window at %d-%d with the pane away; it went with the pane", clo, chi)
	}
	for _, n := range p.r.AppendSemantics(nil) {
		if n.Desc.Label == "New chat" {
			t.Errorf("the window names %q with the pane away; the control it names is not there", n.Desc.Label)
		}
	}
}
