package main

import (
	"image"
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
	"github.com/vibrantgio/patterns/shell"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// layoutFrame drives the window's own frame once at the size the window
// opens at and answers the arrangement it laid out, which the frame keeps
// for exactly this reason: the composition is read back rather than
// recomputed from the numbers that placed it.
func layoutFrame(t *testing.T, m Model) shell.ThreeColumnLayout {
	t.Helper()
	f := newWindowFrame(nil)
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(windowSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
	}
	tok := themeTokens{
		col:    tokens.PlatformLight,
		typ:    tokens.DefaultTypography,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	f.layout(gtx, m, tok, frameSlots{})
	return f.geom
}

// TestTheDocumentTakesTheLargerShare reads the window's opening widths off
// the arrangement it laid out. The list column states an absolute width —
// the platform's own, which its table fits inside — and the article takes
// whatever the window has left, which at the size this window opens at is
// more than the list. The split this replaced gave the list three fifths of
// the content area and the article two, which is the sizing defect the
// absolute width corrects.
func TestTheDocumentTakesTheLargerShare(t *testing.T) {
	g := layoutFrame(t, settledModel())

	listX := int(pane.MarginDp) + int(patsidebar.ExpandedWidth)
	if g.ListX != listX {
		t.Errorf("the list column begins at %d, want the pane's trailing rim %d", g.ListX, listX)
	}
	list := g.ContentX - g.ListX - int(splitter.SeamDp)
	if want := int(shell.PlatformList.Width); list != want {
		t.Errorf("the list column opens at %d, want the platform's own %d", list, want)
	}
	article := windowSize.X - g.ContentX
	if article <= list {
		t.Errorf("the article column is %d wide against the list's %d; the document does not take the larger share",
			article, list)
	}
	t.Logf("pane %d, list %d, article %d in a window %d wide",
		g.Pane.Dx(), list, article, windowSize.X)
}

// TestTheTableFitsTheListColumnItOpensIn states the arithmetic the window
// opens on: the list column's own floor is what its four columns need side
// by side, and the width it opens at — the platform's — is wider than that,
// which is why this window states a floor and takes the platform's width
// rather than stating a width of its own.
func TestTheTableFitsTheListColumnItOpensIn(t *testing.T) {
	if got, want := listColMinWDp, titleColMinWDp+authorColWDp+publishedColWDp+unreadColWDp; got != want {
		t.Errorf("the list column's floor is %d, want the four columns' own minimums %d", got, want)
	}
	if listColMinWDp > int(shell.PlatformList.Width) {
		t.Errorf("the table needs %d and the column opens at the platform's %v; the window must state its own width",
			listColMinWDp, shell.PlatformList.Width)
	}
	if frameLimits.List.Min != unit.Dp(listColMinWDp) {
		t.Errorf("the list column's clamp floors at %v and its table needs %d", frameLimits.List.Min, listColMinWDp)
	}
	if frameLimits.List.Max != 0 {
		t.Errorf("the list column states a ceiling of %v; the platform states none and the article's floor stands in for one",
			frameLimits.List.Max)
	}
}

// TestThePaneAwayGivesItsWidthToTheColumnsBesideIt reads the one thing the
// model says about the arrangement: with the pane away it takes no width at
// all and the list column reflows to the window's own leading edge.
func TestThePaneAwayGivesItsWidthToTheColumnsBesideIt(t *testing.T) {
	m := settledModel()
	m.sidebarHidden = true
	g := layoutFrame(t, m)
	if !g.Pane.Empty() {
		t.Errorf("the pane laid out at %v with the rail away; it takes no width at all", g.Pane)
	}
	if g.ListX != 0 {
		t.Errorf("the list column begins at %d with the pane away, want the window's own edge", g.ListX)
	}
	if article := windowSize.X - g.ContentX; article <= 0 {
		t.Errorf("the article column is %d wide with the pane away", article)
	}
}

// TestTheToolbarIsOneStripAcrossTwoShares reads off the arrangement that
// the two shares stand at one depth and meet at the seam: a toolbar is one
// strip across the window, and two depths across it would read as a step in
// the window's top edge.
func TestTheToolbarIsOneStripAcrossTwoShares(t *testing.T) {
	g := layoutFrame(t, settledModel())
	depth := int(shell.ToolbarDepth)
	for _, s := range []struct {
		name string
		r    image.Rectangle
	}{
		{"over the list", g.ListToolbar},
		{"over the article", g.Toolbar},
	} {
		if s.r.Empty() {
			t.Fatalf("the toolbar's share %s laid out nothing", s.name)
		}
		if s.r.Dy() != depth {
			t.Errorf("the share %s is %d deep, want the platform's toolbar %d", s.name, s.r.Dy(), depth)
		}
		if s.r.Min.Y != 0 {
			t.Errorf("the share %s begins at row %d, want the window's own top edge", s.name, s.r.Min.Y)
		}
	}
	if gap := g.Toolbar.Min.X - g.ListToolbar.Max.X; gap != int(splitter.SeamDp) {
		t.Errorf("the two shares are %d apart, want the one seam %d", gap, int(splitter.SeamDp))
	}
	if g.RowTop != depth {
		t.Errorf("the columns' content starts at row %d, want the toolbar's foot %d", g.RowTop, depth)
	}
}

// paneStripFrame drives the composed window through a real input.Router: the
// arrangement, the pane with its own top strip, and the toolbar beside it. It
// holds nothing the window does not — the clickables are the frame's own — so
// a test can ask where each control answers and read back what a press on it
// posted.
//
// The window is composed rather than the strip laid out alone, because what
// is under test is which of the two strips a mark stands in, which a strip
// laid out by itself cannot show.
type paneStripFrame struct {
	ops    op.Ops
	gtx    layout.Context
	r      gioinput.Router
	f      *windowFrame
	m      Model
	tok    themeTokens
	posted []mvu.Message
}

func newPaneStripFrame(t *testing.T, hidden bool) *paneStripFrame {
	t.Helper()
	p := &paneStripFrame{f: newWindowFrame(nil), m: settledModel()}
	p.m.sidebarHidden = hidden
	p.tok = themeTokens{
		col:    tokens.PlatformLight,
		typ:    tokens.DefaultTypography,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	p.f.post = func(_ layout.Context, msg mvu.Message) { p.posted = append(p.posted, msg) }
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
	p.f.layout(p.gtx, p.m, p.tok, frameSlots{})
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
// between two bare marks.
const paneMarkStride = int(shell.MarkDp) + pane.MarkGapDp

// TestThePaneStripStandsAddFeedBeforeTheToggle reads the pane's top strip off
// a composed window: Add feed, then the toggle at the trailing corner, each
// the bare mark box wide, their drawn centres the measured stride apart, and
// the toggle's own box one margin in from the pane's trailing edge where the
// Pane pattern keeps it.
//
// Add feed stands nowhere in the toolbar beside the pane: it adds to the
// collection the pane lists, so it waits for the pane, and the toolbar's
// trailing group is walked end to end and must not answer for it.
func TestThePaneStripStandsAddFeedBeforeTheToggle(t *testing.T) {
	p := newPaneStripFrame(t, false)
	p.frame()
	p.frame()

	region := pane.Bounds(p.gtx, windowSize, patsidebar.ExpandedWidth, false)
	stripY := region.Min.Y + p.gtx.Dp(unit.Dp(pane.StripDp))/2
	addLo, addHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.f.addClick)
	togLo, togHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.f.toggleClick)

	box := p.gtx.Dp(unit.Dp(shell.MarkDp))
	for _, c := range []struct {
		name   string
		lo, hi int
	}{
		{"Add feed", addLo, addHi},
		{"the toggle", togLo, togHi},
	} {
		if c.lo < 0 {
			t.Fatalf("%s stands nowhere in the pane's strip", c.name)
		}
		if w := c.hi - c.lo + 1; w != box {
			t.Errorf("%s is %d dp wide, want the bare mark box's %d", c.name, w, box)
		}
	}
	if addHi >= togLo {
		t.Errorf("the strip runs Add feed %d-%d, the toggle %d-%d; it reads Add feed, then the toggle",
			addLo, addHi, togLo, togHi)
	}
	if got := togLo - addLo; got != paneMarkStride {
		t.Errorf("Add feed and the toggle stand %d dp between centres, want the measured %d", got, paneMarkStride)
	}
	if got, want := region.Max.X-togHi-1, int(pane.MarginDp); got != want {
		t.Errorf("the toggle ends %d dp from the pane's trailing edge, want the pattern's margin of %d", got, want)
	}

	// The toolbar beside the pane, walked end to end on the line its own
	// controls stand on.
	toolbarY := p.gtx.Dp(shell.ToolbarDepth) / 2
	if lo, hi := p.spanIn(region.Max.X, windowSize.X, toolbarY, &p.f.addClick); lo >= 0 {
		t.Errorf("Add feed answers the toolbar at %d-%d; it stands on the pane and nowhere else", lo, hi)
	}
}

// TestThePaneMarkPostsAnOpenAddFeed presses the strip's leading mark where
// the strip stands it and reads back what the frame posted. Where it stands
// is stated from the pane's trailing edge rather than read off the layout:
// the toggle one margin in, and the mark before it one stride further.
func TestThePaneMarkPostsAnOpenAddFeed(t *testing.T) {
	p := newPaneStripFrame(t, false)
	p.frame()
	p.frame()

	region := pane.Bounds(p.gtx, windowSize, patsidebar.ExpandedWidth, false)
	box := p.gtx.Dp(unit.Dp(shell.MarkDp))
	x := float32(region.Max.X - int(pane.MarginDp) - box/2 - paneMarkStride)
	at := f32.Pt(x, float32(region.Min.Y+p.gtx.Dp(unit.Dp(pane.StripDp))/2))
	p.posted = nil
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	p.frame()
	p.frame()
	if !postedOneOf(p.posted, OpenAddFeed{}) {
		t.Errorf("a press on Add feed at %v posted %v, want a main.OpenAddFeed", at, p.posted)
	}
}

// postedOneOf reports whether a message of want's own kind is among what the
// frame posted.
func postedOneOf(got []mvu.Message, want mvu.Message) bool {
	for _, m := range got {
		if reflect.TypeOf(m) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}

// TestWithThePaneAwayTheToolbarHoldsTheToggleAlone reads the toolbar in the
// collapsed state: the toggle that brings the pane back at its leading end,
// and not one stand-in for the mark that went with the pane. Only the toggle
// travels, which is what this platform's own sidebars do — so while the pane
// is away the window offers Add feed nowhere, and a screen reader is told of
// no such control either.
func TestWithThePaneAwayTheToolbarHoldsTheToggleAlone(t *testing.T) {
	p := newPaneStripFrame(t, true)
	p.frame()
	p.frame()

	y := p.gtx.Dp(shell.ToolbarDepth) / 2
	if lo, _ := p.spanIn(0, windowSize.X, y, &p.f.toggleClick); lo < 0 {
		t.Fatal("the toolbar holds no toggle to bring the pane back")
	}
	if lo, hi := p.spanIn(0, windowSize.X, y, &p.f.addClick); lo >= 0 {
		t.Errorf("Add feed answers the window at %d-%d with the pane away; it went with the pane", lo, hi)
	}
	for _, n := range p.r.AppendSemantics(nil) {
		if n.Desc.Label == "Add feed" {
			t.Errorf("the window names %q with the pane away; the control it names is not there", n.Desc.Label)
		}
	}
}
