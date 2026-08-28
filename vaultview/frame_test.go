package main

import (
	"image"
	"image/color"
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

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	vgcolor "github.com/vibrantgio/theme/color"
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

	// The hidden row lays out past the buttons, which stand at the
	// window's own inset in both rail states — so the probe pins the one
	// measurement there is.
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
			// inset and the vault's name is its first and only control.
			controls: []int{noteInsetDp + 8},
		},
		{
			name: "rail hidden", hidden: true, lead: lead,
			controls: []int{
				lead + frameGapDp + railToggleMarkDp/2, // the show toggle
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
			// Probed on the window buttons' own centre line, which is
			// where the row's controls stand: a probe on the row's middle
			// would ask about a control from above it and be told, quite
			// correctly, that nothing is there.
			moveAt := func(x int) bool {
				a, ok := r.ActionAt(f32.Pt(float32(x), float32(windowButtons.Center)))
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
	const footH = 24

	shown := frameGeometry(gtx, size, barH, footH, false)
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
	// The status bar is the content area's own foot and the pane's bottom
	// margin is not measured against it: the pane floats one margin inside
	// the window's bottom edge whatever the content area spends down there.
	if want := size.Y - footH; shown.footTop != want {
		t.Errorf("the status bar starts at y=%d, want %d — one bar above the window's bottom edge", shown.footTop, want)
	}
	if got := shown.rowTop + shown.rowH; got != shown.footTop {
		t.Errorf("the content area's columns end at y=%d and the status bar starts at y=%d; they must meet", got, shown.footTop)
	}

	hidden := frameGeometry(gtx, size, barH, footH, true)
	if !hidden.pane.Empty() {
		t.Errorf("rail pane %v with the rail hidden, want none", hidden.pane)
	}
	if hidden.contentX != 0 {
		t.Errorf("note column starts at x=%d with the rail hidden, want the window's own edge", hidden.contentX)
	}
	if hidden.rowH != shown.rowH || hidden.rowTop != shown.rowTop || hidden.footTop != shown.footTop {
		t.Errorf("hiding the rail moved the content row: %+v vs %+v", hidden, shown)
	}

	// A window too narrow to seat both the pane and a readable note keeps
	// the note: the pane yields rather than squeezing the document.
	narrow := image.Pt(200, 800)
	ngtx := gtx
	ngtx.Constraints = layout.Exact(narrow)
	if g := frameGeometry(ngtx, narrow, barH, footH, false); g.pane.Dx() > narrow.X/2 {
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
			w, st := renderWindow(shaper, c.model, tokens.DefaultLight, tokens.Spacing,
				goldenRadius, tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
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
	// chrome band is the row's height, and the buttons sit at the window's
	// own inset.
	topBand.Store(row)
	placeWindowButtons(buttonPlacementFor(goldenModel()))
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

// TestWindowButtonsStandStillWhenThePaneGoes asserts the one thing the
// window's control buttons owe the reader: they are the window's, seen
// through whatever the application draws under them, and nothing the
// application draws moves them. The vault screen must therefore answer
// one placement for both rail states — dismissing the pane is a change to
// what is behind the buttons, not to where they are — and that placement
// must be stated from the window's own edges rather than from any pane's.
//
// The pane's own strip is measured against them in the same breath, since
// the strip exists to keep the pane's content out from under the buttons:
// in window coordinates it must reach past their bottom edge, and its
// middle line — where the pane's toggle centres — must be their centre
// line, so the two read as one row of furniture.
func TestWindowButtonsStandStillWhenThePaneGoes(t *testing.T) {
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true

	want := buttonPlacement{leading: windowButtons.Leading, center: windowButtons.Center}
	if got := buttonPlacementFor(shown); got != want {
		t.Errorf("with the pane standing the buttons are placed at %+v, want %+v — the window's own inset", got, want)
	}
	if got := buttonPlacementFor(hidden); got != want {
		t.Errorf("with the pane away the buttons are placed at %+v, want %+v — dismissing the pane moved a control that is not the pane's", got, want)
	}

	// The buttons' band in window coordinates, and the pane's strip in the
	// same coordinates: the pane floats one margin in from the top edge.
	buttonsTop, buttonsBottom := buttonInsetDp, buttonInsetDp+desktop.WindowButtonDiameter
	stripTop, stripBottom := railMarginDp, railMarginDp+paneStripDp
	if stripTop > buttonsTop {
		t.Errorf("the pane's strip begins at y=%d, below the buttons' top edge at y=%d — the pane's content would start under them", stripTop, buttonsTop)
	}
	if stripBottom < buttonsBottom {
		t.Errorf("the pane's strip ends at y=%d, above the buttons' bottom edge at y=%d — the pane's content would run under them", stripBottom, buttonsBottom)
	}
	if mid := stripTop + paneStripDp/2; unit.Dp(mid) != windowButtons.Center {
		t.Errorf("the strip's middle line is y=%d and the buttons' is y=%v; the pane's toggle centres on the strip and would sit off their line", mid, windowButtons.Center)
	}
}

// TestTheRailWearsThePlatformsSeam pins the derivation of the floating
// pane's own edge: how far it stands from the fill it is drawn on, which
// way it goes, and that it is a whisper rather than a mark.
//
// The number is the platform's. Voice Memos outlines its floating panel at
// #3A3A3A on a #1B1B1B panel — 1.514:1 — and leaves the flush side of the
// same window unoutlined (owner-attested, 2026-08-28). Both halves are
// checked here: the derived ink lands on that ratio against this window's
// own floor in BOTH schemes, and it lands nowhere near the 3:1 graphic
// floor an object's outline is derived to elsewhere in the system, which
// on these grounds would answer ink several times louder than anything the
// platform draws around a sidebar.
func TestTheRailWearsThePlatformsSeam(t *testing.T) {
	const (
		measured  = 1.51 // Voice Memos, panel outline against panel fill
		tolerance = 0.02 // eight bits' worth of slack, no more
	)
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := chromeSurface(tc.colors)
			ink := paneSeam(tc.colors)
			got := vgcolor.ContrastRatio(ink, fill)
			if got < measured-tolerance || got > measured+tolerance {
				t.Errorf("the pane's edge stands %.3f:1 off its fill (%v on %v), want the measured %.2f:1",
					got, ink, fill, measured)
			}
			// Toward the scheme's own ink: lighter than the pane in a dark
			// scheme, as the platform draws it, and darker in a light one,
			// which is the only direction a light floor has room in.
			towardInk := lightnessOf(tc.colors.Text) > lightnessOf(fill)
			if lighter := lightnessOf(ink) > lightnessOf(fill); lighter != towardInk {
				t.Errorf("the pane's edge is %v against a fill of %v and ink of %v; the edge steps toward the ink",
					ink, fill, tc.colors.Text)
			}
			// Not a mark. 3:1 is what an outline owes its ground when the
			// line IS the object; a pane's edge is read beside a fill, an
			// inset and a radius saying the same thing.
			if got >= 3.0 {
				t.Errorf("the pane's edge reads %.2f:1, at or over the graphic floor — this is a seam, not a mark", got)
			}
		})
	}
}

// TestTheRailIsOutlinedAndCastsNothing reads the composed window: the pane
// carries a hairline just inside its own boundary, and the ground around
// it is the window's own paper with nothing cast onto it.
//
// The two go together. The pane is chrome furniture, so its storey is the
// floor and the floor's dp is zero — the desk has nothing to cast onto.
// What says the pane floats is its edge, so the edge has to be there and
// the shadow has to be gone; a test that checked only one of them would
// pass on the arrangement this task replaced.
func TestTheRailIsOutlinedAndCastsNothing(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowCanvasSize, scene(w, tc.bg))
			pane := st.geom.pane
			if pane.Empty() {
				t.Fatal("the window laid out no pane to read")
			}
			ink := paneSeam(tc.colors)
			// A row clear of the corners' arcs, of the toggle and of every
			// row's own ink — the middle of the pane's own top strip. On it
			// the pane's leading and trailing edge columns are the hairline,
			// and the pixel inside each of them is the fill.
			y := pane.Min.Y + paneStripDp/2
			for _, probe := range []struct {
				what     string
				edge, in int
			}{
				{"leading", pane.Min.X, pane.Min.X + seamDp},
				{"trailing", pane.Max.X - seamDp, pane.Max.X - seamDp - 1},
			} {
				if got := img.RGBAAt(probe.edge, y); !sameInk(got, ink) {
					t.Errorf("the pane's %s edge at x=%d draws %v, want the seam %v", probe.what, probe.edge, got, ink)
				}
				if got, want := img.RGBAAt(probe.in, y), chromeSurface(tc.colors); !sameInk(got, want) {
					t.Errorf("one pixel inside the pane's %s edge draws %v, want the floor %v — the hairline is wider than a hairline",
						probe.what, got, want)
				}
			}
			// The gutter the pane floats in, its whole height: bare paper.
			for x := 0; x < pane.Min.X; x++ {
				for y := 0; y < windowH; y++ {
					if got := img.RGBAAt(x, y); !sameInk(got, tc.colors.Background) {
						t.Fatalf("the ground at (%d,%d) draws %v, want the window's paper %v — the pane is casting something onto its own desk",
							x, y, got, tc.colors.Background)
					}
				}
			}
		})
	}
}

// TestTheAsideKeepsAPlainSeam reads the trailing column's boundary off the
// same window: one hairline of the divider's own ink, on the column's
// leading edge, running the window's full height — over the chrome row at
// the top and the status bar at the foot, because the platform's split
// seams are not interrupted by a band either.
//
// And the column is NOT outlined: it is integral furniture, fixed and
// flush, so it has no edge of its own on the three sides it shares with
// the window. The trailing column of pixels is read for that.
func TestTheAsideKeepsAPlainSeam(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowCanvasSize, scene(w, tc.bg))
			asideX := windowW - frameAsideDp
			floor := chromeSurface(tc.colors)

			for y := 0; y < windowH; y++ {
				if got := img.RGBAAt(asideX, y); !sameInk(got, tc.colors.Divider) {
					t.Fatalf("the column's seam at y=%d draws %v, want the divider %v — the seam stops where a band crosses it",
						y, got, tc.colors.Divider)
				}
				if got := img.RGBAAt(windowW-1, y); !sameInk(got, floor) {
					t.Fatalf("the column's trailing edge at y=%d draws %v, want its own floor %v — flush furniture wears no outline",
						y, got, floor)
				}
			}
			// One pixel wide: the column's own fill starts immediately.
			if got := img.RGBAAt(asideX+seamDp, st.geom.footTop-1); !sameInk(got, floor) {
				t.Errorf("one pixel past the seam draws %v, want the column's floor %v", got, floor)
			}
		})
	}
}

// sameInk compares a captured pixel with a token colour on the channels a
// capture keeps: what is drawn over the ground is opaque by the time it is
// read back.
func sameInk(got color.RGBA, want color.NRGBA) bool {
	return got.R == want.R && got.G == want.G && got.B == want.B
}
