package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
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
// The row's trailing end used to hold two vault actions, and the probe
// there asserted that their span did not drag. They stand at the foot of
// the sidebar pane now, so the same point makes the opposite statement:
// nothing but the vault's name stands in this row, and everything past it
// — to the row's last dp — moves the window.
//
// The probe is the same one the window makes on a press — the frame's
// own hit test, asked what action stands at a point — so it measures the
// composed row rather than the ops in isolation.
func TestToolbarDeclaresWindowDrag(t *testing.T) {
	tok := goldenTokens()
	rowW := 1100
	rowH := int(toolbarHeight(tok))

	// The hidden row lays out past the buttons at the platform's own
	// geometry — hiding the pane hands them back — so the probe pins the
	// hidden-state measurement, not the placed one.
	const lead = goldenLeadingHidden
	for _, c := range []struct {
		name     string
		hidden   bool
		lead     unit.Dp
		controls []int
	}{
		{
			name: "rail shown", hidden: false, lead: 0,
			// With the pane standing the row starts at its own edge
			// inset and the vault's name is its first and only control.
			controls: []int{noteInsetDp + 8},
		},
		{
			name: "rail hidden", hidden: true, lead: lead,
			controls: []int{
				lead + frameGapDp + toggleMarkWDp/2, // the show toggle
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
			// The trailing end is part of that stretch now that the vault
			// actions have left the row: the drag runs to its last dp.
			for _, x := range []int{rowW / 3, rowW / 2, rowW - frameEdgeDp - 20, rowW - 1} {
				if !moveAt(x) {
					t.Errorf("no window-move action at x=%d; the row holds no control there, so it must move the window", x)
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

// TestRailPaneFloatsAtTheWindowTop asserts the sidebar is the leading
// column and owns the top of the window the way the platform's own
// sidebars do: floating one margin inside the window's leading, top and
// bottom edges, with nothing above it but that margin of ground — no
// chrome band, which is what the arrangement exists to be without. The
// window buttons stand inside the pane's own strip, which they can only
// do if the pane is what is under them; the margin merely moves the
// strip a step in from the glass.
//
// Hidden, the pane is gone entirely and the note column reflows from the
// window's leading edge — the freed width goes to the document, not to a
// stripe of nothing where the rail used to be.
func TestRailPaneFloatsAtTheWindowTop(t *testing.T) {
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
	if want := image.Pt(railMarginDp, railMarginDp); shown.pane.Min != want {
		t.Errorf("pane starts at %v, want %v — one margin inside the window's top-leading corner, and nothing above it but ground", shown.pane.Min, want)
	}
	if want := size.Y - railMarginDp; shown.pane.Max.Y != want {
		t.Errorf("pane bottom at y=%d, want %d — one margin above the window's bottom edge", shown.pane.Max.Y, want)
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
// the order it visits the sidebar in: the find field, then the rows, then
// the vault's own actions at the foot, and only after all of them the
// pane's own toggle. The order is the point, not the reachability. With
// the toggle laid out where it is drawn — the pane's top-right corner —
// it stood between the field and the rows, so Tab and then Return from
// the field put the whole pane away instead of opening the note the
// reader had just filtered for.
//
// The foot sits after the rows and before the toggle because it acts on
// what the pane shows rather than on the pane: a reader tabbing out of
// the find field is still going to the notes, and having reached the end
// of them, the next thing worth offering is what can be done to the vault
// they are in. The toggle stays last — it is the pane talking about
// itself, and nothing the pane is for may stand behind it.
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
		{"the foot's rescan", &v.rescanClick},
		{"the foot's vault switch", &v.switchClick},
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

// TestPaneStripClaimsInsideTheInsetPane asserts what the sidebar's top
// strip declares now that the pane floats inside the window's edges,
// measured on the composed window rather than on the strip alone —
// through the frame's inset offset and the pane's rounded clip, which is
// the geometry a real press crosses. Three claims and a delivery:
//
//   - the strip's empty middle still moves the window — the pane owns
//     the top of the window, so it owes the reader the drag the native
//     title bar handed over;
//   - the pane's own toggle is not covered by that claim, because a move
//     action swallows the press before the control sees one;
//   - the margin of ground the inset reveals claims nothing — it is bare
//     ground, and an eight-dp sliver is not a handle a hand aims for;
//   - and a pointer over the toggle reaches the toggle, which is what
//     proves the rounded clip and the inset offset between the window's
//     coordinates and the pane's have not orphaned the control.
func TestPaneStripClaimsInsideTheInsetPane(t *testing.T) {
	tok := goldenTokens()
	var ops op.Ops
	var r input.Router
	size := image.Pt(1100, 800)
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
		Ops:         &ops,
	}
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
	sb := func(gtx layout.Context) layout.Dimensions {
		return v.layout(gtx, goldenModel(), tok, nil)
	}
	f := &frameState{asideW: frameAsideDp, leading: func() unit.Dp { return goldenLeading }}
	frame := func() {
		f.layout(gtx, goldenModel(), tok, sb, nil, nil)
		r.Frame(&ops)
	}
	frame()

	pane := f.geom.pane
	stripY := float32(pane.Min.Y) + float32(paneStripDp)/2
	middle := f32.Pt(float32(pane.Min.X+120), stripY)
	toggle := f32.Pt(float32(pane.Max.X-railMarginDp-treeHideBoxDp/2), stripY)

	if a, ok := r.ActionAt(middle); !ok || a != system.ActionMove {
		t.Errorf("no window-move action at %v; the strip's empty middle is the pane's drag handle", middle)
	}
	if a, ok := r.ActionAt(toggle); ok && a == system.ActionMove {
		t.Errorf("window-move action at %v; the toggle's own span must not move the window", toggle)
	}
	if a, ok := r.ActionAt(f32.Pt(middle.X, float32(pane.Min.Y)/2)); ok {
		t.Errorf("action %v claimed over the margin above the pane; the revealed ground is bare", a)
	}

	r.Queue(pointer.Event{Kind: pointer.Move, Position: toggle, Source: pointer.Mouse})
	ops.Reset()
	frame()
	if !v.hideClick.Hovered() {
		t.Errorf("a pointer at %v does not reach the pane's toggle through the inset and the rounded clip", toggle)
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
// must not regress. The sidebar column spends one margin — the inset the
// pane floats off the window's edges by — and no chrome at all: what is
// above the pane is ground, not band, and the assertion pins the margin
// so a band cannot creep in wearing its name.
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
			lead := unit.Dp(goldenLeading)
			if c.model.SidebarHidden {
				lead = goldenLeadingHidden
			}
			w, st := renderWindow(shaper, c.model, tokens.DefaultLight, tokens.Spacing,
				sharpRadius, tokens.DefaultTypography, tokens.Comfortable, lead)
			drawOnce(t, windowCanvasSize, w)

			if st.geom.rowTop > chromeBudgetDp {
				t.Errorf("the content area spends %d dp above its first document row, over the %d dp budget — a band has come back",
					st.geom.rowTop, chromeBudgetDp)
			}
			if st.geom.rowTop != row {
				t.Errorf("the content area spends %d dp above its first document row, want the chrome row's own %d dp — nothing else may stand there",
					st.geom.rowTop, row)
			}
			// The sidebar's own budget: one margin of ground and nothing
			// else. The pane floats that far inside the window's top
			// edge; anything more above it would be the band this whole
			// arrangement exists without.
			if !st.geom.pane.Empty() && st.geom.pane.Min.Y != railMarginDp {
				t.Errorf("the sidebar starts at y=%d, want the frame's own %d dp margin and nothing else above it", st.geom.pane.Min.Y, railMarginDp)
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
	// What the vault screen states on every emission that selects it: the
	// chrome band is the row's height, and the buttons sit at the pane's
	// own inset, centred on the middle line of its top strip.
	topBand.Store(row)
	placeWindowButtons(unit.Dp(railMarginDp+buttonInsetDp),
		unit.Dp(railMarginDp)+unit.Dp(paneStripDp)/2)
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
