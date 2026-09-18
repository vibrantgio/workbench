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
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// goldenTokens is the token snapshot the frame tests lay out from: the
// shipped light set on a deterministic shaper, so a measurement cannot
// depend on which faces the host carries.
func goldenTokens() themeTokens {
	return themeTokens{
		col:    tokens.PlatformLight,
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
	rowH := int(toolbarHeight())

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
			controls: []int{
				noteInsetDp + 8,
				rowW - bandTrailingDp - railToggleWidthDp/2,                                     // the find
				rowW - bandTrailingDp - railToggleWidthDp - bandGapDp - railToggleWidthDp/2,     // the vault switch
				rowW - bandTrailingDp - 2*railToggleWidthDp - 2*bandGapDp - railToggleWidthDp/2, // the rescan
			},
		},
		{
			name: "rail hidden", hidden: true, lead: lead,
			controls: []int{
				lead + railToggleWidthDp/2, // the show toggle
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

			// The stretch between the vault's name and the band's trailing
			// cluster is the largest empty one, and the one a hand reaches
			// for. The gaps inside the cluster and the band's own trailing
			// inset move the window too, so the probe ends on the row's
			// last dp.
			cluster := rowW - bandTrailingDp - 3*railToggleWidthDp - 2*bandGapDp
			for _, x := range []int{rowW / 3, rowW / 2, cluster - 20, rowW - 1} {
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

// TestRailRunsToTheWindowTop asserts the sidebar is the leading column and
// owns the top of the window the way the platform's own sidebars do: its
// fill running to the window's leading, top and bottom edges, with nothing
// above it and no chrome band. The window buttons stand inside the rail's
// own strip, which they can only do if the rail is what is under them.
//
// Hidden, the pane is gone entirely and the note column reflows from the
// window's leading edge, so the freed width goes to the document.
func TestRailRunsToTheWindowTop(t *testing.T) {
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
	if want := (image.Pt(railMarginDp, railMarginDp)); shown.pane.Min != want {
		t.Errorf("pane starts at %v, want %v — one margin inside the window's top-leading corner, with the window's own plane above it", shown.pane.Min, want)
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
	// The status bar is the content area's own foot and the rail is not
	// measured against it: the rail runs to the window's bottom edge
	// whatever the content area spends down there.
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
// the order it visits the sidebar in: the find field, then the rows, and
// only after both of them the pane's own toggle. What is asserted is the order, not the reachability:
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
//   - the strip's empty middle still moves the window — the rail owns
//     the top of the window, so it owes the reader the drag the native
//     title bar handed over;
//   - the rail's own toggle is not covered by that claim, because a move
//     action swallows the press before the control sees one;
//   - and a pointer over the toggle reaches the toggle, which is what
//     proves the rail's clip has not orphaned the control.
func TestPaneStripClaimsInsideTheRail(t *testing.T) {
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

	panel := f.geom.pane
	stripY := float32(panel.Min.Y) + float32(paneStripDp)/2
	middle := f32.Pt(float32(panel.Min.X+120), stripY)
	// The toggle stands BARE at the panel's top trailing corner, one margin
	// in from its trailing edge, where
	// voicememos-multi-folder-2026-09-18.png keeps a sidebar panel's own
	// marks.
	toggle := f32.Pt(float32(panel.Max.X-railMarginDp-markLargeDp/2), stripY)

	if a, ok := r.ActionAt(middle); !ok || a != system.ActionMove {
		t.Errorf("no window-move action at %v; the strip's empty middle is the pane's drag handle", middle)
	}
	if a, ok := r.ActionAt(toggle); ok && a == system.ActionMove {
		t.Errorf("window-move action at %v; the toggle's own span must not move the window", toggle)
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
				Constraints: layout.Exact(image.Pt(860, int(toolbarHeight()))),
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
				t.Errorf("the chrome row's rail toggle operable=%v, want %v", seen, c.want)
			}
		})
	}
}

// chromeBudgetDp is what the vault window may spend between its top edge
// and its first row of content. It is the platform's toolbar band and
// nothing more: 52, the depth every stored toolbar capture measures and the
// band the rail's panel is set one margin into — the single chrome row's
// full height and no room for a second thing above it.
const chromeBudgetDp = bandDp

// TestChromeBudget holds the vault window's chrome to that budget, by
// laying the whole window out at the size it opens at and asking the frame
// where it put the first content row. The row's own height says nothing
// about a band stacked above it, and a band stacked above it is the defect.
//
// The chrome row belongs to the content area rather than spanning the
// window, so the measurement is stated per column. The content area spends
// the row's own height above its first document row — the band's 52 dp — and
// no more. The sidebar column spends its own margin and nothing else: its
// panel starts one margin below the window's top edge, and the assertion
// pins that so a band cannot creep in above it.
//
// Both rail states are measured. Hiding the rail rebuilds the whole
// composition, and a budget that only holds in one of them holds in
// neither.
func TestChromeBudget(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true
	row := int(toolbarHeight())

	for _, c := range []struct {
		name  string
		model Model
	}{
		{"rail shown", shown},
		{"rail hidden", hidden},
	} {
		t.Run(c.name, func(t *testing.T) {
			w, st := renderWindow(shaper, c.model, tokens.PlatformLight, tokens.Spacing,
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
			// The sidebar's own budget: its margin and nothing else.
			// Anything more above the panel would be a chrome band.
			if !st.geom.pane.Empty() && st.geom.pane.Min.Y != railMarginDp {
				t.Errorf("the sidebar's panel starts at y=%d, want its own margin %d and nothing above it but the window's plane", st.geom.pane.Min.Y, railMarginDp)
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
	row := toolbarHeight()
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

	// The buttons' band in the PANEL's own coordinates, which the panel's
	// strip is cut in: the panel stands one margin inside the window's top
	// edge and the buttons are measured from the window's glass, so the
	// strip owes that margin back at both ends.
	buttonsTop := buttonInsetDp - railMarginDp
	buttonsBottom := buttonsTop + desktop.WindowButtonDiameter
	stripTop, stripBottom := 0, paneStripDp
	if stripTop > buttonsTop {
		t.Errorf("the panel's strip begins at y=%d, below the buttons' top edge at y=%d — the panel's content would start under them", stripTop, buttonsTop)
	}
	if stripBottom < buttonsBottom {
		t.Errorf("the panel's strip ends at y=%d, above the buttons' bottom edge at y=%d — the panel's content would run under them", stripBottom, buttonsBottom)
	}
	if mid := railMarginDp + paneStripDp/2; unit.Dp(mid) != windowButtons.Center {
		t.Errorf("the strip's middle line is y=%d in the window and the buttons' is y=%v; the panel's toggle centres on the strip and would sit off their line", mid, windowButtons.Center)
	}
	if mid := bandDp / 2; unit.Dp(mid) != windowButtons.Center {
		t.Errorf("the band's middle line is y=%d and the buttons' is y=%v; a control centred in the band would sit off their line", mid, windowButtons.Center)
	}
}

// TestTheRailIsAnInsetPanel reads the composed window: the rail is a panel
// set one margin in from the window's leading, top and bottom edges, flush
// with the note on the fourth, carrying the platform's rim on every one of
// its four sides and no seam anywhere — the toolbar band's rows included,
// since the panel is one object from its top edge to its foot.
//
// MEASURED, voicememos-multi-folder-2026-09-18.png and
// finder-window-untinted-dark.png: the panel stands eight pixels inside the
// window on three sides with the window's own plane showing in them, and the
// content begins at the fourth with no gap.
func TestTheRailIsAnInsetPanel(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			rail := st.geom.pane
			if rail.Empty() {
				t.Fatal("the window laid out no rail to read")
			}
			want := image.Pt(railMarginDp, railMarginDp)
			if rail.Min != want || rail.Max.Y != windowH-railMarginDp {
				t.Fatalf("the rail stands at %v; it is set one margin inside the window's leading, top and bottom edges", rail)
			}
			rim := paneRim(tc.colors)
			fill := chromeSurface(tc.colors)
			seam := splitter.SeamColor(tc.colors, fill)

			// Every one of the four edges, read mid-run where the corners'
			// arcs have let go — the band's own rows among them, because no
			// line crosses the band and the panel passes straight through it.
			midY := (rail.Min.Y + rail.Max.Y) / 2
			midX := (rail.Min.X + rail.Max.X) / 2
			for _, probe := range []struct {
				what     string
				x, y     int
				inX, inY int
			}{
				{"leading", rail.Min.X, midY, 1, 0},
				{"trailing", rail.Max.X - seamDp, midY, -1, 0},
				{"trailing, in the band", rail.Max.X - seamDp, rail.Min.Y + paneStripDp/2, -1, 0},
				{"top", midX, rail.Min.Y, 0, 1},
				{"bottom", midX, rail.Max.Y - 1, 0, -1},
			} {
				if got := img.RGBAAt(probe.x, probe.y); !sameColor(got, rim) {
					t.Errorf("the panel's %s edge at (%d,%d) draws %v, want the platform's rim %v", probe.what, probe.x, probe.y, got, rim)
				}
				if got := img.RGBAAt(probe.x+probe.inX, probe.y+probe.inY); !sameColor(got, fill) {
					t.Errorf("one pixel inside the panel's %s edge draws %v, want the chrome level %v — the rim is wider than a hairline", probe.what, got, fill)
				}
				if rim != seam {
					if got := img.RGBAAt(probe.x, probe.y); sameColor(got, seam) {
						t.Errorf("the panel's %s edge draws the seam %v; an inset panel is bounded by its rim", probe.what, seam)
					}
				}
			}
			// Past the trailing rim stands the note's own surface under the
			// panel's shadow, which only darkens it and recovers outward.
			v0 := img.RGBAAt(rail.Max.X, midY)
			v1 := img.RGBAAt(rail.Max.X+1, midY)
			far := img.RGBAAt(rail.Max.X+int(pane.ShadowReachDp), midY)
			if v0.R > tc.colors.TextBackground.R || v0.R > v1.R {
				t.Errorf("past the panel's trailing rim the note's surface reads %v then %v against its own %v — a second line stands beside the panel's own",
					v0, v1, tc.colors.TextBackground)
			}
			if !sameColor(far, tc.colors.TextBackground) {
				t.Errorf("a reach past the panel's trailing rim the note's surface draws %v, want its own %v — the shadow is spent by its reach",
					far, tc.colors.TextBackground)
			}
			// The window's own plane shows in the margin beside the panel,
			// and the panel casts its shadow on it: neither the bare plane
			// nor the panel's own fill is what stands there.
			if got := img.RGBAAt(rail.Min.X-1, midY); sameColor(got, tc.colors.WindowBackground) {
				t.Errorf("the column beside the panel's leading rim draws the bare plane %v; the panel casts a shadow on it", tc.colors.WindowBackground)
			}
		})
	}
}

// TestTheAsideKeepsAPlainSeam reads the trailing column's boundary off the
// same window: one hairline of the seam's own colour, on the column's
// leading edge, running from the toolbar band's lower edge to the window's
// foot — the status bar included, which is not a band the window's columns
// stop at, and the toolbar band excluded, which is.
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

			// The band is one across all three columns, so this boundary
			// stops at its lower edge the way the rail's does: above that
			// row the column's own fill runs on, and the seam starts under
			// it.
			for y := 0; y < bandDp; y++ {
				if got := img.RGBAAt(asideX, y); sameColor(got, splitter.SeamColor(tc.colors, color.NRGBA{})) {
					t.Fatalf("the column's seam is drawn at y=%d, inside the toolbar band — the band is one across the window's columns and no line crosses it", y)
				}
			}
			for y := bandDp; y < windowH; y++ {
				if got := img.RGBAAt(asideX, y); !sameColor(got, splitter.SeamColor(tc.colors, color.NRGBA{})) {
					t.Fatalf("the column's seam at y=%d draws %v, want the seam %v — the seam runs from the band's foot to the window's",
						y, got, splitter.SeamColor(tc.colors, color.NRGBA{}))
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
	// The panel's own margin comes off what is left for the three columns:
	// it stands one margin inside the window's leading edge.
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
// draws at rest: nothing the panel did not already draw. The boundary there
// is the panel's own rim, and a splitter drawing a line of its own beside it
// would put two edges a pixel apart down the whole panel.
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
			// The whole straight run of the trailing edge, the band's own
			// rows included: one hairline of the panel's own rim, and the
			// note's surface the pixel after it. The corners' arcs are read
			// out, because the edge is not straight there and the splitter
			// does not draw over them.
			// At one pixel per dp the panel's corner radius is the span's
			// own inset at both ends.
			top, bottom := p.Min.Y+pane.RadiusDp, p.Max.Y-pane.RadiusDp
			for y := top; y < bottom; y++ {
				if got := img.RGBAAt(p.Max.X-seamDp, y); !sameColor(got, paneRim(tc.colors)) {
					t.Fatalf("the panel's trailing edge at y=%d draws %v, want its own rim %v", y, got, paneRim(tc.colors))
				}
				// Past the rim stands the note's surface under the panel's
				// shadow, which only recovers outward: a second line would
				// be a column darker than the one beyond it.
				v0, v1, v2 := img.RGBAAt(p.Max.X, y).R, img.RGBAAt(p.Max.X+1, y).R, img.RGBAAt(p.Max.X+2, y).R
				if v0 > v1 || v1 > v2 || v2 > tc.colors.TextBackground.R {
					t.Fatalf("past the panel's trailing edge at y=%d the note's surface reads %d, %d, %d against its own %d — a second line stands beside the panel's own",
						y, v0, v1, v2, tc.colors.TextBackground.R)
				}
			}
		})
	}
}

// bandFrame drives the whole window through one input router so the toolbar
// band's own controls can be reached the way a reader reaches them: the
// document actions and the find stand in the band, and the band is part of
// the frame rather than of any column.
type bandFrame struct {
	f    *frameState
	find pageFind
	gtx  layout.Context
	ops  op.Ops
	r    input.Router
	size image.Point
}

func newBandFrame() *bandFrame {
	b := &bandFrame{f: newFrameState(defaultWidths()), size: image.Pt(windowW, windowH)}
	b.f.leading = func() unit.Dp { return goldenLeading }
	b.f.band = bandFind{find: &b.find}
	b.gtx = layout.Context{
		Constraints: layout.Exact(b.size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      b.r.Source(),
		Ops:         &b.ops,
	}
	return b
}

func (b *bandFrame) frame() {
	b.ops.Reset()
	b.f.layout(b.gtx, goldenModel(), goldenTokens(), nil, nil, nil)
	b.r.Frame(&b.ops)
}

// TestBandActionsAnswerTheKeyboard drives the band the way a reader without
// a pointer does: Tab to each of the vault's two actions and activate it.
// Both must report a press. What the press then means — a rescan that counts
// what it found, a switch that returns to the picker — is asserted at the
// model; this covers only whether the keyboard can get there at all.
func TestBandActionsAnswerTheKeyboard(t *testing.T) {
	for _, c := range []struct {
		name  string
		click func(f *frameState) *widget.Clickable
	}{
		{"rescan", func(f *frameState) *widget.Clickable { return &f.rescanClick }},
		{"switch vault", func(f *frameState) *widget.Clickable { return &f.switchClick }},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newBandFrame()
			b.frame()

			target := c.click(b.f)
			src := b.r.Source()
			reached := false
			for range 64 {
				b.r.MoveFocus(key.FocusForward)
				if src.Focused(target) {
					reached = true
					break
				}
			}
			if !reached {
				t.Fatalf("Tab never reaches the band's %s action", c.name)
			}
			b.r.ClickFocus()

			b.ops.Reset()
			if !target.Clicked(b.gtx) {
				t.Errorf("activating the band's %s action from the keyboard produced no press", c.name)
			}
		})
	}
}

// TestBandNamesItsActions asserts the band's affordances are in the window's
// semantic tree under the names a screen reader speaks. These controls carry
// a symbol and no word at all — which is what every document action in every
// stored toolbar band carries — so the spoken name is the only name there is.
func TestBandNamesItsActions(t *testing.T) {
	b := newBandFrame()
	b.frame()

	spoken := map[string]bool{}
	for _, n := range b.r.AppendSemantics(nil) {
		spoken[n.Desc.Label] = true
	}
	for _, want := range []string{"Rescan", "Switch Vault", "Find in this note"} {
		if !spoken[want] {
			t.Errorf("the window's semantic tree does not name %q", want)
		}
	}
}

// TestBandActionsAnswerThePress asserts the band's actions are controls the
// pointer reaches and that answer being held down. The pointer is delivered
// through the frame's own input router.
func TestBandActionsAnswerThePress(t *testing.T) {
	b := newBandFrame()
	b.frame()
	b.frame()

	// The rescan control stands in the band's trailing cluster: the find
	// capsule is last, the vault switch before it, the rescan before that.
	// Its centre is one control and one gap in from each.
	const ctrl = railToggleWidthDp
	x := float32(windowW - bandTrailingDp - 3*ctrl - 2*bandGapDp + ctrl/2)
	at := f32.Pt(x, float32(paneStripDp)/2)
	b.r.Queue(pointer.Event{Kind: pointer.Move, Position: at, Source: pointer.Mouse})
	b.frame()
	if !b.f.rescanClick.Hovered() {
		t.Fatalf("a pointer at %v is not on the band's rescan control", at)
	}

	b.r.Queue(pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	b.frame()
	if !b.f.rescanClick.Pressed() {
		t.Fatalf("a press at %v did not reach the band's rescan control", at)
	}
}

// TestTheBandOpensTheFind asserts the affordance the shortcut gained when the
// find moved into the band: the magnifier capsule finder-window-light.png
// keeps at the trailing end of its own band opens the field and asks for the
// keyboard, which is what Cmd+F does.
func TestTheBandOpensTheFind(t *testing.T) {
	b := newBandFrame()
	b.frame()
	b.frame()

	const ctrl = railToggleWidthDp
	at := f32.Pt(float32(windowW-bandTrailingDp-ctrl/2), float32(paneStripDp)/2)
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	b.frame()
	if !b.find.open {
		t.Error("pressing the band's search control did not open the find")
	}
}

// TestTheBandComposesLikeTheStoredWindows reads the band's own arrangement
// back off a laid-out frame: the document actions cluster at the trailing
// end with the search last of all, one measured gap between each pair, and
// the last control eight clear of the window's trailing edge.
//
// MEASURED, reference/macos/controls.md under "What the toolbar band's
// composition measures": the search stands last in all five stored bands,
// finder-window-light.png leaves 16 between its view pop-up and the group
// pull-down beside it, and the last control in all four full windows ends 8
// px from the window's own trailing edge.
func TestTheBandComposesLikeTheStoredWindows(t *testing.T) {
	b := newBandFrame()
	b.frame()
	b.frame()

	// Each control's own span, found by walking the band's centre line and
	// asking which control the pointer is on.
	span := func(c *widget.Clickable) (int, int) {
		lo, hi := -1, -1
		for x := 0; x < windowW; x++ {
			b.r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x), float32(paneStripDp)/2), Source: pointer.Mouse})
			b.frame()
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
	rescanLo, rescanHi := span(&b.f.rescanClick)
	switchLo, switchHi := span(&b.f.switchClick)
	findLo, findHi := span(&b.f.findClick)

	for _, c := range []struct {
		name   string
		lo, hi int
	}{
		{"rescan", rescanLo, rescanHi},
		{"the vault switch", switchLo, switchHi},
		{"the search", findLo, findHi},
	} {
		if c.lo < 0 {
			t.Fatalf("%s stands nowhere in the band", c.name)
		}
		if w := c.hi - c.lo + 1; w != railToggleWidthDp {
			t.Errorf("%s is %d dp wide, want the bordered toolbar control's %d", c.name, w, railToggleWidthDp)
		}
	}
	if !(rescanHi < switchLo && switchHi < findLo) {
		t.Errorf("the band runs rescan %d-%d, switch %d-%d, search %d-%d; the search stands last in every stored band",
			rescanLo, rescanHi, switchLo, switchHi, findLo, findHi)
	}
	if got := switchLo - rescanHi - 1; got != bandGapDp {
		t.Errorf("rescan and the vault switch stand %d dp apart, want the measured %d", got, bandGapDp)
	}
	if got := findLo - switchHi - 1; got != bandGapDp {
		t.Errorf("the vault switch and the search stand %d dp apart, want the measured %d", got, bandGapDp)
	}
	if got := windowW - findHi - 1; got != bandTrailingDp {
		t.Errorf("the band's last control ends %d dp from the window's trailing edge, want the measured %d",
			got, bandTrailingDp)
	}
}
