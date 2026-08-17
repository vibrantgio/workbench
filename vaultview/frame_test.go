package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/tokens"
)

// goldenTokens is the token snapshot the frame tests lay out from: the
// shipped light set on a deterministic shaper, so a measurement cannot
// depend on which faces the host carries.
func goldenTokens() themeTokens {
	return themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
}

// TestToolbarDeclaresWindowDrag asserts what the chrome row declares to
// the window: a move action over the empty space it leaves between its
// controls, and none over the controls themselves. The row stands where
// the native title bar's drag would be, so without the declaration the
// window has no top edge to move it by; with it laid over a control, the
// control's press would be the window's rather than the application's.
//
// Both rail states are probed, because the row is not the same row in
// them: with the pane away it leads with the toggle that brings the pane
// back, and it leaves the window buttons' own span alone in front of it.
//
// The probe is the same one the window makes on a press — the frame's
// own hit test, asked what action stands at a point — so it measures the
// composed row rather than the ops in isolation.
func TestToolbarDeclaresWindowDrag(t *testing.T) {
	tok := goldenTokens()
	rowW := 1100
	rowH := int(toolbarHeight(tok))

	const lead = goldenLeading
	for _, c := range []struct {
		name     string
		hidden   bool
		lead     unit.Dp
		controls []int
	}{
		{
			name: "rail shown", hidden: false, lead: 0,
			// With the pane standing the row starts at its own edge
			// inset and the vault's name is its first control.
			controls: []int{rowW - frameEdgeDp - 20},
		},
		{
			name: "rail hidden", hidden: true, lead: lead,
			controls: []int{
				lead + frameGapDp + toggleMarkWDp/2, // the show toggle
				rowW - frameEdgeDp - 20,             // inside the trailing action
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var ops op.Ops
			gtx := layout.Context{
				Constraints: layout.Exact(image.Pt(rowW, rowH)),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Ops:         &ops,
			}
			m := goldenModel()
			m.SidebarHidden = c.hidden
			f := &frameState{asideW: frameAsideDp}
			f.layoutToolbar(gtx, m, tok, c.lead)

			var r input.Router
			r.Frame(&ops)
			moveAt := func(x int) bool {
				a, ok := r.ActionAt(f32.Pt(float32(x), float32(rowH)/2))
				return ok && a == system.ActionMove
			}

			// The middle of the row — between the vault's name and the
			// trailing actions — is the largest empty stretch, and the one
			// a hand reaches for. Both ends of it drag.
			for _, x := range []int{rowW / 3, rowW / 2} {
				if !moveAt(x) {
					t.Errorf("no window-move action at x=%d; the row's empty middle must move the window", x)
				}
			}
			// Every control's own span belongs to the control.
			for _, x := range c.controls {
				if moveAt(x) {
					t.Errorf("window-move action at x=%d; a control's own span must not move the window", x)
				}
			}
			// The window buttons' own span is nobody's to claim: a move
			// action there would fight them for the press.
			if c.lead > 0 && moveAt(int(c.lead)/2) {
				t.Error("window-move action over the window buttons' own span")
			}
		})
	}
}

// TestRailPaneRunsToTheWindowTop asserts the sidebar is the leading
// column and starts at the window's own top edge: nothing crosses above
// it, it reaches the bottom edge as well, and the content area starts
// where it ends. That top edge is the whole point of the arrangement —
// the window buttons stand inside the pane's own strip, which they can
// only do if the pane is what is under them.
//
// Hidden, the pane is gone entirely and the note column reflows from the
// window's leading edge — the freed width goes to the document, not to a
// stripe of nothing where the rail used to be.
func TestRailPaneRunsToTheWindowTop(t *testing.T) {
	var ops op.Ops
	size := image.Pt(1100, 800)
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	const barH = 28

	shown := frameGeometry(gtx, size, barH, false)
	if shown.pane.Empty() {
		t.Fatal("no rail pane with the rail shown")
	}
	if shown.pane.Min.X != 0 || shown.pane.Min.Y != 0 {
		t.Errorf("pane starts at %v, want the window's own top-leading corner — no band may cross above the sidebar", shown.pane.Min)
	}
	if shown.pane.Max.Y != size.Y {
		t.Errorf("pane bottom at y=%d, want the window's own bottom edge %d", shown.pane.Max.Y, size.Y)
	}
	if w := shown.pane.Dx(); w != treeWidthDp {
		t.Errorf("pane width %d, want the rail's own %d", w, treeWidthDp)
	}
	if shown.contentX != shown.pane.Max.X {
		t.Errorf("content area starts at x=%d, want the pane's trailing edge %d", shown.contentX, shown.pane.Max.X)
	}
	if shown.rowTop != barH {
		t.Errorf("the content area's first document row starts at y=%d, want %d — below its own chrome row and nothing else", shown.rowTop, barH)
	}

	hidden := frameGeometry(gtx, size, barH, true)
	if !hidden.pane.Empty() {
		t.Errorf("rail pane %v with the rail hidden, want none", hidden.pane)
	}
	if hidden.contentX != 0 {
		t.Errorf("note column starts at x=%d with the rail hidden, want the window's own edge", hidden.contentX)
	}
	if hidden.rowH != shown.rowH || hidden.rowTop != shown.rowTop {
		t.Errorf("hiding the rail moved the content row: %+v vs %+v", hidden, shown)
	}

	// A window too narrow to seat both the pane and a readable note keeps
	// the note: the pane yields rather than squeezing the document.
	narrow := image.Pt(200, 800)
	ngtx := gtx
	ngtx.Constraints = layout.Exact(narrow)
	if g := frameGeometry(ngtx, narrow, barH, false); g.pane.Dx() > narrow.X/2 {
		t.Errorf("pane %v takes more than half of a %d dp window", g.pane, narrow.X)
	}
}

// TestPaneFocusOrder walks the focus ring the way Tab does and asserts
// the order it visits the sidebar in: the find field, then the rows, and
// only after them the pane's own toggle. The order is the point, not the
// reachability. With the toggle laid out where it is drawn — the pane's
// top-right corner — it stood between the field and the rows, so Tab and
// then Return from the field put the whole pane away instead of opening
// the note the reader had just filtered for.
//
// The field's slot takes a stand-in with a focus tag of its own: the live
// field is a component whose static render path processes no events, and
// what is under test is the order of the pane's own ops, not the field.
func TestPaneFocusOrder(t *testing.T) {
	tok := goldenTokens()
	var ops op.Ops
	var r input.Router
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(treeWidthDp, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
		Ops:         &ops,
	}
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
	var field widget.Clickable
	fieldW := func(gtx layout.Context) layout.Dimensions {
		return field.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 24)}
		})
	}
	v.layout(gtx, goldenModel(), tok, fieldW)

	r.Frame(&ops)
	src := r.Source()
	stops := []struct {
		name string
		tag  event.Tag
	}{
		{"the find field", &field},
		{"the rail's rows", v.list.Focus()},
		{"the pane's own toggle", &v.hideClick},
	}
	first := map[string]int{}
	for i := range 64 {
		r.MoveFocus(key.FocusForward)
		for _, s := range stops {
			if _, seen := first[s.name]; !seen && src.Focused(s.tag) {
				first[s.name] = i
			}
		}
	}
	for _, s := range stops {
		if _, seen := first[s.name]; !seen {
			t.Fatalf("the focus ring never reaches %s", s.name)
		}
	}
	for i := 1; i < len(stops); i++ {
		if first[stops[i].name] <= first[stops[i-1].name] {
			t.Errorf("Tab reaches %s before %s; the order must run field, rows, then the pane's own controls",
				stops[i].name, stops[i-1].name)
		}
	}
}

// TestTheRowRecallsTheHiddenPane asserts the one thing the pane's own
// toggle cannot do: bring the pane back. With the rail hidden the chrome
// row carries a toggle of its own, and the keyboard reaches it — a pane
// only a pointer can recall is a pane half the window's users cannot
// recall. With the rail shown the row carries no such control, because
// the pane's own is right there.
func TestTheRowRecallsTheHiddenPane(t *testing.T) {
	tok := goldenTokens()
	for _, c := range []struct {
		name   string
		hidden bool
		want   bool
	}{
		{"rail hidden", true, true},
		{"rail shown", false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			var ops op.Ops
			var r input.Router
			gtx := layout.Context{
				Constraints: layout.Exact(image.Pt(860, int(toolbarHeight(tok)))),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Source:      r.Source(),
				Ops:         &ops,
			}
			m := goldenModel()
			m.SidebarHidden = c.hidden
			f := &frameState{asideW: frameAsideDp}
			f.layoutToolbar(gtx, m, tok, 0)

			r.Frame(&ops)
			src := r.Source()
			seen := false
			for range 32 {
				r.MoveFocus(key.FocusForward)
				if src.Focused(&f.toggleClick) {
					seen = true
				}
			}
			if seen != c.want {
				t.Errorf("the chrome row's rail toggle reachable=%v, want %v", seen, c.want)
			}
		})
	}
}

// chromeBudgetDp is what the vault window may spend between its top edge
// and its first row of content. The composition this replaced spent about
// eighty — a native title-bar strip holding nothing but the window
// buttons, and a full navbar band under it holding a label and two links
// — and nothing in the suite could see it. Forty leaves the single row
// the window now draws its full height and no room for a second thing.
const chromeBudgetDp = 40

// TestChromeBudget holds the vault window's chrome to that budget, by
// laying the whole window out at the size it opens at and asking the
// frame where it put the first content row. It supersedes a cheaper
// version that checked the toolbar row's own height: a row that measures
// twenty-eight dp says nothing about a band stacked above it, and a band
// stacked above it is precisely the defect.
//
// The arrangement the budget is measured against has changed and the
// budget has not. The chrome row no longer spans the window: it belongs
// to the content area, so the measurement is now stated per column. The
// content area spends the row's own height above its first document row
// and no more — the twenty-eight dp it measured before, which is what
// must not regress. The sidebar column spends nothing at all, because it
// starts at the window's top edge, and the assertion says so rather than
// leaving a column unmeasured.
//
// Both rail states are measured. Hiding the rail rebuilds the whole
// composition, and a budget that only holds in one of them holds in
// neither.
func TestChromeBudget(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true
	row := int(toolbarHeight(goldenTokens()))

	for _, c := range []struct {
		name  string
		model Model
	}{
		{"rail shown", shown},
		{"rail hidden", hidden},
	} {
		t.Run(c.name, func(t *testing.T) {
			w, st := renderWindow(shaper, c.model, tokens.DefaultLight, tokens.Spacing,
				sharpRadius, tokens.DefaultTypography, tokens.Comfortable, goldenLeading)
			drawOnce(t, windowCanvasSize, w)

			if st.geom.rowTop > chromeBudgetDp {
				t.Errorf("the content area spends %d dp above its first document row, over the %d dp budget — a band has come back",
					st.geom.rowTop, chromeBudgetDp)
			}
			if st.geom.rowTop != row {
				t.Errorf("the content area spends %d dp above its first document row, want the chrome row's own %d dp — nothing else may stand there",
					st.geom.rowTop, row)
			}
			// The sidebar's own budget: none. It is the leading column
			// from the window's top edge, and anything above it is the
			// band this whole arrangement exists without.
			if !st.geom.pane.Empty() && st.geom.pane.Min.Y != 0 {
				t.Errorf("the sidebar starts at y=%d, want the window's own top edge", st.geom.pane.Min.Y)
			}
		})
	}
}

// TestChromeHeightMatchesTheRow asserts the two answers to "how much of
// the top is not document" agree: the one the frame lays the content row
// out from, and the one chromeHeight gives the overlays. They are
// computed in different files from different facts — the row's height
// here, the band the screen said it had taken there — and if they drift,
// a toast lands on the vault's own controls or floats a band below them,
// which is the same class of defect as the band itself.
//
// It also holds the screen's own inset to zero while the vault is up.
// The vault screen draws in the native strip on purpose — the sidebar
// reaches the window's top edge — and a layer padded down past the strip
// would put the retired band back under another name.
func TestChromeHeightMatchesTheRow(t *testing.T) {
	row := toolbarHeight(goldenTokens())
	// What the vault screen states on every emission that selects it.
	topBand.Store(row)
	placeWindowButtons(row / 2)
	if got := chromeHeight(); got != row {
		t.Errorf("the overlays are inset by %v dp while the content row starts at %v dp", got, row)
	}
	if got := screenTopInset(); got != 0 {
		t.Errorf("the vault screen is padded down by %v dp; it lays out its own top band", got)
	}
	if row > chromeBudgetDp {
		t.Errorf("the chrome row alone is %v dp, over the %d dp budget", row, chromeBudgetDp)
	}
}
