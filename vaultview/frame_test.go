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
	"github.com/vibrantgio/patterns/pane"
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
// Nothing but the vault's name stands in this row, so everything past it —
// to the row's last dp — moves the window.
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
			f := newFrameState(defaultWidths())
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

			// The stretch past the vault's name is the largest empty one,
			// and the one a hand reaches for: it drags to the row's last dp.
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
// bottom edges, with nothing above it but that margin of backdrop and no
// chrome band. The window buttons stand inside the pane's own strip, which
// they can only do if the pane is what is under them; the margin merely
// moves the strip a step in from the glass.
//
// Hidden, the pane is gone entirely and the note column reflows from the
// window's leading edge, so the freed width goes to the document.
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

	shown := frameGeometry(gtx, size, treeWidthDp, barH, footH, false)
	if shown.pane.Empty() {
		t.Fatal("no rail pane with the rail shown")
	}
	if want := image.Pt(railMarginDp, railMarginDp); shown.pane.Min != want {
		t.Errorf("pane starts at %v, want %v — one margin inside the window's top-leading corner, and nothing above it but backdrop", shown.pane.Min, want)
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

	hidden := frameGeometry(gtx, size, treeWidthDp, barH, footH, true)
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
	if g := frameGeometry(ngtx, narrow, treeWidthDp, barH, footH, false); g.pane.Dx() > narrow.X/2 {
		t.Errorf("pane %v takes more than half of a %d dp window", g.pane, narrow.X)
	}
}

// TestPaneFocusOrder walks the focus ring the way Tab does and asserts
// the order it visits the sidebar in: the find field, then the rows, then
// the vault's own actions at the foot, and only after all of them the
// pane's own toggle. What is asserted is the order, not the reachability:
// laid out where it is drawn — the pane's top-right corner — the toggle would
// stand between the field and the rows, and Tab then Return from the field
// would put the whole pane away instead of opening the note the reader had
// just filtered for.
//
// The foot sits after the rows and before the toggle because it acts on
// what the pane shows rather than on the pane. The toggle stays last: it is
// the pane talking about itself, and nothing the pane is for may stand
// behind it.
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
// strip declares, measured on the composed window rather than on the strip
// alone — through the frame's inset offset and the pane's rounded clip,
// which is the geometry a real press crosses. Three claims and a delivery:
//
//   - the strip's empty middle still moves the window — the pane owns
//     the top of the window, so it owes the reader the drag the native
//     title bar handed over;
//   - the pane's own toggle is not covered by that claim, because a move
//     action swallows the press before the control sees one;
//   - the margin of backdrop the inset reveals claims nothing — it is bare
//     backdrop, and an eight-dp sliver is not a handle a hand aims for;
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
	f := newFrameState(defaultWidths())
	f.leading = func() unit.Dp { return goldenLeading }
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
		t.Errorf("action %v claimed over the margin above the pane; the revealed backdrop is bare", a)
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
			f := newFrameState(defaultWidths())
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
// and its first row of content. Forty leaves the single chrome row its full
// height and no room for a second thing above it.
const chromeBudgetDp = 40

// TestChromeBudget holds the vault window's chrome to that budget, by
// laying the whole window out at the size it opens at and asking the frame
// where it put the first content row. The row's own height says nothing
// about a band stacked above it, and a band stacked above it is the defect.
//
// The chrome row belongs to the content area rather than spanning the
// window, so the measurement is stated per column. The content area spends
// the row's own height above its first document row (twenty-eight dp) and
// no more. The sidebar column spends one margin — the inset the pane floats
// off the window's edges by — and no chrome at all: what is above the pane
// is backdrop, not band, and the assertion pins the margin so a band cannot
// creep in wearing its name.
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
			drawOnce(t, windowFrameSize, w)

			if st.geom.rowTop > chromeBudgetDp {
				t.Errorf("the content area spends %d dp above its first document row, over the %d dp budget — a band has come back",
					st.geom.rowTop, chromeBudgetDp)
			}
			if st.geom.rowTop != row {
				t.Errorf("the content area spends %d dp above its first document row, want the chrome row's own %d dp — nothing else may stand there",
					st.geom.rowTop, row)
			}
			// The sidebar's own budget: one margin of backdrop and nothing
			// else. Anything more above the pane would be a chrome band.
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
// line, so the two read as one row of chrome.
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
// checked here: the derived colour lands on that ratio against this window's
// own floor in BOTH schemes, and it lands nowhere near the 3:1 graphic
// floor an object's outline is derived to elsewhere in the system, which
// on these surfaces would answer a colour several times more pronounced than
// anything the platform draws around a sidebar.
func TestTheRailWearsThePlatformsSeam(t *testing.T) {
	const (
		measured  = 1.51 // Voice Memos, panel outline against panel fill
		tolerance = 0.02 // eight bits' worth of slack, no more
	)
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := chromeSurface(tc.colors)
			seamColor := paneSeam(tc.colors)
			got := vgcolor.ContrastRatio(seamColor, fill)
			if got < measured-tolerance || got > measured+tolerance {
				t.Errorf("the pane's edge stands %.3f:1 off its fill (%v on %v), want the measured %.2f:1",
					got, seamColor, fill, measured)
			}
			// Toward the scheme's own foreground: lighter than the pane in a dark
			// scheme, as the platform draws it, and darker in a light one,
			// which is the only direction a light floor has room in.
			towardForeground := lightnessOf(tc.colors.Text) > lightnessOf(fill)
			if lighter := lightnessOf(seamColor) > lightnessOf(fill); lighter != towardForeground {
				t.Errorf("the pane's edge is %v against a fill of %v and foreground of %v; the edge steps toward the foreground",
					seamColor, fill, tc.colors.Text)
			}
			// Not a mark. 3:1 is what an outline owes what it stands on when the
			// line IS the object; a pane's edge is read beside a fill, an
			// inset and a radius saying the same thing.
			if got >= 3.0 {
				t.Errorf("the pane's edge reads %.2f:1, at or over the graphic floor — this is a seam, not a mark", got)
			}
		})
	}
}

// TestTheRailIsOutlinedAndCastsNothing reads the composed window: the pane
// carries a hairline just inside its own boundary, and what shows in the
// gap around it is the backdrop, with nothing cast onto it.
//
// The two go together. The pane stands at the chrome level and the chrome
// level's dp is zero — chrome lies flat on the backdrop and has nothing to
// cast onto. What says the pane is set in from the window's edges is the
// backdrop showing around it and the edge inside it, so both have to be
// there and the shadow has to be gone; checking one of them proves neither.
func TestTheRailIsOutlinedAndCastsNothing(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			pane := st.geom.pane
			if pane.Empty() {
				t.Fatal("the window laid out no pane to read")
			}
			seamColor := paneSeam(tc.colors)
			// A row clear of the corners' arcs, of the toggle and of every
			// row's own text — the middle of the pane's own top strip. On it
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
				if got := img.RGBAAt(probe.edge, y); !sameColor(got, seamColor) {
					t.Errorf("the pane's %s edge at x=%d draws %v, want the seam %v", probe.what, probe.edge, got, seamColor)
				}
				if got, want := img.RGBAAt(probe.in, y), chromeSurface(tc.colors); !sameColor(got, want) {
					t.Errorf("one pixel inside the pane's %s edge draws %v, want the chrome level %v — the hairline is wider than a hairline",
						probe.what, got, want)
				}
			}
			// The two corners the pane rounds away from its TRAILING edge.
			// That side is the one it is not set in from — the note stands
			// flush against it — so what shows behind those arcs is the
			// note's own surface and not the backdrop, which would read as a
			// nick bitten out of the boundary.
			for _, at := range []image.Point{
				{X: pane.Max.X - 2, Y: pane.Min.Y + 1},
				{X: pane.Max.X - 2, Y: pane.Max.Y - 2},
			} {
				if got := img.RGBAAt(at.X, at.Y); !sameColor(got, tc.colors.Background) {
					t.Errorf("the pane's trailing corner at %v draws %v, want the note's surface %v", at, got, tc.colors.Background)
				}
			}
			// The gap the pane is set into, its whole height: the bare
			// backdrop, which is what an inset object stands on.
			backdrop := tc.colors.SurfaceAt(tokens.LevelBackdrop)
			for x := 0; x < pane.Min.X; x++ {
				for y := 0; y < windowH; y++ {
					if got := img.RGBAAt(x, y); !sameColor(got, backdrop) {
						t.Fatalf("the gap at (%d,%d) draws %v, want the backdrop %v — the pane is casting something onto the plane it stands on",
							x, y, got, backdrop)
					}
				}
			}
		})
	}
}

// TestTheAsideKeepsAPlainSeam reads the trailing column's boundary off the
// same window: one hairline of the seam's own colour, on the column's
// leading edge, running the window's full height — over the chrome row at
// the top and the status bar at the foot, because the platform's split
// seams are not interrupted by a band either.
//
// And the column is NOT outlined: it is integral chrome, fixed and
// flush, so it has no edge of its own on the three sides it shares with
// the window. The trailing column of pixels is read for that.
func TestTheAsideKeepsAPlainSeam(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			asideX := windowW - frameAsideDp
			floor := chromeSurface(tc.colors)

			for y := 0; y < windowH; y++ {
				if got := img.RGBAAt(asideX, y); !sameColor(got, tc.colors.Seam) {
					t.Fatalf("the column's seam at y=%d draws %v, want the seam %v — the seam stops where a band crosses it",
						y, got, tc.colors.Seam)
				}
				if got := img.RGBAAt(windowW-1, y); !sameColor(got, floor) {
					t.Fatalf("the column's trailing edge at y=%d draws %v, want its own floor %v — flush chrome wears no outline",
						y, got, floor)
				}
			}
			// One pixel wide: the column's own fill starts immediately.
			if got := img.RGBAAt(asideX+seamDp, st.geom.footTop-1); !sameColor(got, floor) {
				t.Errorf("one pixel past the seam draws %v, want the column's floor %v", got, floor)
			}
		})
	}
}

// sameColor compares a captured pixel with a token colour on the channels a
// capture keeps: what is drawn over the surface is opaque by the time it is
// read back.
func sameColor(got color.RGBA, want color.NRGBA) bool {
	return got.R == want.R && got.G == want.G && got.B == want.B
}

// dragFrame is one live window laid out through a router, so that the
// splitters' hit areas, pointers and drags all go through the path a real
// frame does. The sidebar slot records the width the pane hands its
// column, which is the only place a caller can read it back from.
type dragFrame struct {
	f    *frameState
	gtx  layout.Context
	ops  op.Ops
	r    input.Router
	size image.Point
	slot int
}

func newDragFrame(size image.Point, w columnWidths) *dragFrame {
	d := &dragFrame{f: newFrameState(w), size: size}
	d.f.leading = func() unit.Dp { return goldenLeading }
	d.gtx = layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      d.r.Source(),
		Ops:         &d.ops,
	}
	return d
}

func (d *dragFrame) frame() {
	d.ops.Reset()
	sb := func(gtx layout.Context) layout.Dimensions {
		d.slot = gtx.Constraints.Max.X
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	d.f.layout(d.gtx, goldenModel(), goldenTokens(), sb, nil, nil)
	d.r.Frame(&d.ops)
}

// drag takes hold at x on the row's own middle and pulls to x+by. Two
// frames go first, so that the hit area is known to the router before a
// pointer is put on it.
func (d *dragFrame) drag(x, by float32) {
	d.frame()
	d.frame()
	y := float32(d.f.geom.rowTop + d.f.geom.rowH/2)
	d.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, y)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x+by, y)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x+by, y)},
	)
	d.frame()
}

// TestTheRailEdgeIsDraggedAndItsColumnFollows verifies the leading
// boundary: the pane's own trailing hairline is what a hand takes hold
// of, the pane widens with the drag, and the column standing in the pane
// is as wide as the pane. A rail dragged wider that still held a 240 dp
// column would be a rail widened into nothing.
func TestTheRailEdgeIsDraggedAndItsColumnFollows(t *testing.T) {
	const pull = 40
	d := newDragFrame(image.Pt(windowW, windowH), defaultWidths())
	d.frame()
	edge := d.f.geom.pane.Max.X
	d.drag(float32(edge-seamDp), pull)

	if got, want := d.f.railW, unit.Dp(treeWidthDp+pull); got != want {
		t.Fatalf("the drag left the rail at %v dp, want %v", got, want)
	}
	if got, want := d.f.geom.pane.Dx(), treeWidthDp+pull; got != want {
		t.Errorf("the pane draws %d px wide, want %d — the pane is not where its own edge was dragged to", got, want)
	}
	if got, want := d.slot, treeWidthDp+pull; got != want {
		t.Errorf("the pane hands its column %d px, want %d", got, want)
	}
}

// TestTheAsideEdgeIsDraggedByTheRowAlone verifies the trailing boundary's
// hand-hold: the seam runs the window's whole height, and the part of it
// a hand may take is the document row alone. The bands crossing over it
// are the window's own — one carries the window's drag, the other reports
// on the document — and neither is resized by this boundary.
func TestTheAsideEdgeIsDraggedByTheRowAlone(t *testing.T) {
	const pull = 40
	size := image.Pt(windowW, windowH)

	over := newDragFrame(size, defaultWidths())
	over.frame()
	over.frame()
	x := float32(size.X - frameAsideDp)
	// The chrome row's own middle, on the seam's line: over the seam, and
	// over a band the boundary does not part.
	y := float32(over.f.geom.rowTop / 2)
	over.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, y)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x-pull, y)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x-pull, y)},
	)
	over.frame()
	if got := over.f.asideW; got != frameAsideDp {
		t.Errorf("a drag begun in the chrome row left the aside at %v dp, want the untouched %v", got, unit.Dp(frameAsideDp))
	}

	in := newDragFrame(size, defaultWidths())
	in.drag(x, -pull)
	if got, want := in.f.asideW, unit.Dp(frameAsideDp+pull); got != want {
		t.Errorf("a drag begun in the document row left the aside at %v dp, want %v", got, want)
	}
}

// TestTheNoteKeepsItsMinimumBetweenTheTwoBoundaries verifies what stops
// both splitters: neither column may be widened into the note's own
// minimum. Both are dragged well past where the note runs out, and both
// stop at the width that leaves the note exactly that much.
func TestTheNoteKeepsItsMinimumBetweenTheTwoBoundaries(t *testing.T) {
	size := image.Pt(windowW, windowH)
	gap := frameSplitterDp

	rail := newDragFrame(size, defaultWidths())
	rail.frame()
	rail.drag(float32(rail.f.geom.pane.Max.X-seamDp), 400)
	if got, want := int(rail.f.railW), windowW-railMarginDp-gap-frameAsideDp-noteMinWidthDp; got != want {
		t.Errorf("the rail dragged past the note's minimum stopped at %d dp, want %d", got, want)
	}

	aside := newDragFrame(size, defaultWidths())
	aside.drag(float32(size.X-frameAsideDp), -400)
	if got, want := int(aside.f.asideW), windowW-railMarginDp-treeWidthDp-gap-noteMinWidthDp; got != want {
		t.Errorf("the aside dragged past the note's minimum stopped at %d dp, want %d", got, want)
	}
}

// TestTheRailEdgeDrawsNoSecondLine verifies what the leading splitter
// draws at rest: nothing the pane did not already draw. The boundary
// there is the pane's own hairline, and a splitter drawing its own line
// beside it would put two edges a pixel apart down the whole pane.
func TestTheRailEdgeDrawsNoSecondLine(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			p := st.geom.pane
			if p.Empty() {
				t.Fatal("the window laid out no pane to read")
			}
			// Down the straight run of the trailing edge, clear of the
			// arcs the two corners round away: one hairline of the pane's
			// own edge colour, and the note's surface the pixel after it.
			for y := p.Min.Y + pane.RadiusDp; y < p.Max.Y-pane.RadiusDp; y++ {
				if got := img.RGBAAt(p.Max.X-seamDp, y); !sameColor(got, paneSeam(tc.colors)) {
					t.Fatalf("the pane's trailing edge at y=%d draws %v, want its own edge %v", y, got, paneSeam(tc.colors))
				}
				if got := img.RGBAAt(p.Max.X, y); !sameColor(got, tc.colors.Background) {
					t.Fatalf("the pixel past the pane's trailing edge at y=%d draws %v, want the note's surface %v — a second line stands beside the pane's own",
						y, got, tc.colors.Background)
				}
			}
		})
	}
}
