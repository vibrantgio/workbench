package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

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
