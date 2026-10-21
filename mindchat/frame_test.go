package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// paneStripFrame drives the composed window through a real input.Router: the
// arrangement, the pane with its own top strip, and the toolbar beside it.
// It holds the clickables the window's controls stand on, so a test can ask
// where each one answers and read back what a press on it posted.
//
// The window is composed rather than the strip laid out alone, because what
// is under test is where a mark stands in the window and which of the two
// strips it stands in — neither of which a strip laid out by itself can show.
type paneStripFrame struct {
	ops    op.Ops
	gtx    layout.Context
	r      gioinput.Router
	f      *windowFrame
	rail   *railState
	th     themed
	m      Model
	posted []mvu.Message

	// newChat is the pane's own mark. The Toggle Sidebar button is ONE
	// clickable in two places and the frame holds it.
	newChat widget.Clickable
}

func newPaneStripFrame(t *testing.T, hidden bool) *paneStripFrame {
	t.Helper()
	p := &paneStripFrame{
		f:    newWindowFrame(nil),
		rail: newRailState(),
		th:   railThemed(t, tokens.PlatformLight),
		m:    demoModel(),
	}
	p.f.post = func(_ layout.Context, msg mvu.Message) { p.posted = append(p.posted, msg) }
	// The toolbar's control is drawn at the density's own toolbar-control
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
		&p.newChat,
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
const paneMarkStride = int(shell.MarkDp) + pane.MarkGapDp

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
	togLo, togHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.f.toggleClick)

	box := p.gtx.Dp(unit.Dp(shell.MarkDp))
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
	box := p.gtx.Dp(unit.Dp(shell.MarkDp))
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

// TestWithThePaneAwayTheRowHoldsTheToggleAlone reads the toolbar in the
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
	lo, _ := p.spanIn(0, windowSize.X, y, &p.f.toggleClick)
	if lo < 0 {
		t.Fatal("the toolbar holds no toggle to bring the pane back")
	}
	if want := int(desktop.BandLeadFrom(buttonsEndDp, pane.ButtonGapDp, transcriptInsetDp)); lo != want {
		t.Errorf("the toolbar's toggle leads at %d, want the leading end past the window buttons at %d", lo, want)
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

// settingsFootBand is how deep a band at the pane's foot the sweep below
// walks. It is wider than the row the footer used to stand in, so a control
// left behind anywhere near the pane's foot is caught.
const settingsFootBand = 64

// TestThePaneOffersNoSettingsControlAndTheMenusMessageOpensIt reads the
// composed window through a real input.Router.
//
// Settings acts on the application and not on the conversation, so it has no
// control in the window at all: the pane's semantic tree names none, and a
// press anywhere in the band at the pane's foot — where its footer used to
// stand — posts nothing that opens the dialog. The menu's own item carries
// the message instead, and that message still opens the dialog through
// Update, which is the whole of what moved.
func TestThePaneOffersNoSettingsControlAndTheMenusMessageOpensIt(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	p := newPaneStripFrame(t, false)
	p.frame()
	p.frame()

	for _, n := range p.r.AppendSemantics(nil) {
		if strings.Contains(n.Desc.Label, "Settings") {
			t.Errorf("the window names %q; settings stands in the application menu and has no control here", n.Desc.Label)
		}
	}

	region := pane.Bounds(p.gtx, windowSize, SidebarWidth, false)
	p.posted = nil
	x := float32(region.Min.X+region.Max.X) / 2
	for y := region.Max.Y - settingsFootBand; y < region.Max.Y; y++ {
		for _, kind := range []pointer.Kind{pointer.Press, pointer.Release} {
			p.r.Queue(pointer.Event{
				Kind:     kind,
				Source:   pointer.Mouse,
				Position: f32.Pt(x, float32(y)),
				Buttons:  pointer.ButtonPrimary,
			})
			p.frame()
		}
	}
	if postedOneOf(p.posted, OpenSettings{}) {
		t.Error("a press at the pane's foot opened settings; the footer's control is gone")
	}

	var item MenuCommand
	for _, c := range MenuCommands {
		if c.Title == "Settings…" {
			item = c
		}
	}
	if item.Title == "" {
		t.Fatal("the application menu no longer carries Settings…")
	}
	if item.Key != "," {
		t.Errorf("Settings… stands on %q, want the platform's comma", item.Key)
	}
	if _, ok := item.Msg.(OpenSettings); !ok {
		t.Fatalf("Settings… posts %#v, want main.OpenSettings", item.Msg)
	}
	next, _ := Update(demoModel(), item.Msg)
	if !next.Settings.Open {
		t.Error("the menu's message left the settings dialog closed")
	}
}

// TestThePanesTrailingEdgeIsASplitterTheReaderCanDrag is the owner's finding:
// a window with a sidebar has a splitter on the pane's trailing edge, and
// this one had none. The boundary is taken hold of where the pane's rim
// stands and dragged along the row; the pane follows the hand, the content
// column reflows from where it ends, and the width the drag left is what the
// window keeps.
//
// The arrangement is read back through the shell's own report rather than
// recomputed from the tokens that placed it: what is asserted is the frame
// that was drawn.
func TestThePanesTrailingEdgeIsASplitterTheReaderCanDrag(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	p := newPaneStripFrame(t, false)
	p.frame()
	p.frame()

	opened := p.f.geom.Pane.Max.X
	if got, want := opened, int(PaneMargin)+int(SidebarWidth); got != want {
		t.Fatalf("the pane's trailing edge stands at %d, want the sidebar's own column at %d", got, want)
	}

	// The hand takes the boundary where it is drawn and carries it sixty
	// columns out.
	const by = 60
	y := float32(windowSize.Y / 2)
	at := f32.Pt(float32(opened)-0.5, y)
	p.r.Queue(pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	p.frame()
	p.r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(at.X+by, y), Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	p.frame()
	p.r.Queue(pointer.Event{Kind: pointer.Release, Position: f32.Pt(at.X+by, y), Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	p.frame()

	if got, want := int(p.f.paneW), int(SidebarWidth)+by; got != want {
		t.Fatalf("the drag left the pane %d dp wide, want %d", got, want)
	}
	if got, want := p.f.geom.Pane.Max.X, opened+by; got != want {
		t.Errorf("the frame reports the pane's trailing edge at %d, want %d", got, want)
	}
	if got, want := p.f.geom.ContentX, opened+by; got != want {
		t.Errorf("the content column begins at %d, want the pane's trailing edge at %d", got, want)
	}
}

// TestTheWindowKeepsThePanesWidthAndShownState holds the arrangement to one
// file: the width the reader dragged the pane to and whether it stands at
// all are written where the shell keeps them, and a window opened on that
// file arranges itself the way it was left.
func TestTheWindowKeepsThePanesWidthAndShownState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	widths := shell.OpenArrangement(path, time.Millisecond, defaultArrangement(), frameLimits)

	if got, want := widths.Arrangement().PaneWidth, SidebarWidth; got != want {
		t.Fatalf("a window with nothing kept opens its pane at %v, want %v", got, want)
	}
	// The first report is what the window opened with and is never news;
	// the second is the reader's change.
	widths.Record(widths.Arrangement())
	widths.Record(shell.Arrangement{PaneWidth: 300, PaneShown: false})
	widths.Stop()

	reopened := shell.OpenArrangement(path, time.Millisecond, defaultArrangement(), frameLimits)
	if got := reopened.Arrangement(); got.PaneWidth != 300 || got.PaneShown {
		t.Fatalf("the reopened window arranges itself at %+v, want the 300 dp pane sent away", got)
	}
	m, _ := opensOn(func() (Model, mvu.Command) { return demoModel(), mvu.DoNothing() }, reopened.Arrangement())()
	if !m.SidebarHidden {
		t.Error("the window opened with the pane standing after the reader sent it away")
	}
}
