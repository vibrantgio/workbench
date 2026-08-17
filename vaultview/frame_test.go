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

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/tokens"
)

// TestToolbarDeclaresWindowDrag asserts what the chrome row declares to
// the window: a move action over the empty space it leaves between its
// controls, and none over the controls themselves. The row stands where
// the native title bar's drag would be, so without the declaration the
// window has no top edge to move it by; with it laid over a control, the
// control's press would be the window's rather than the application's.
//
// The probe is the same one the window makes on a press — the frame's
// own hit test, asked what action stands at a point — so it measures the
// composed row rather than the ops in isolation.
func TestToolbarDeclaresWindowDrag(t *testing.T) {
	tok := themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	rowW := 1100
	rowH := int(toolbarHeight(tok))

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(rowW, rowH)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	f := &frameState{asideW: frameAsideDp}
	f.layoutToolbar(gtx, goldenModel(), tok)

	var r input.Router
	r.Frame(&ops)
	moveAt := func(x int) bool {
		a, ok := r.ActionAt(f32.Pt(float32(x), float32(rowH)/2))
		return ok && a == system.ActionMove
	}

	// The middle of the row — between the vault's name and the trailing
	// actions — is the largest empty stretch, and the one a hand reaches
	// for. Both ends of it drag.
	for _, x := range []int{rowW / 3, rowW / 2} {
		if !moveAt(x) {
			t.Errorf("no window-move action at x=%d; the row's empty middle must move the window", x)
		}
	}
	// Every control's own span belongs to the control.
	for _, x := range []int{
		frameEdgeDp + frameGapDp + toggleMarkWDp/2, // the rail toggle
		rowW - frameEdgeDp - 20,                    // inside the trailing action
	} {
		if moveAt(x) {
			t.Errorf("window-move action at x=%d; a control's own span must not move the window", x)
		}
	}
}

// TestRailPaneFloats asserts the rail is a pane and not a column: the
// window's ground is visible on all four sides of it, it begins below the
// toolbar row rather than against it, and the note column starts past its
// trailing margin. Hidden, the pane is gone entirely and the note column
// reflows from the window's own leading edge — the freed width goes to
// the document, not to a stripe of nothing where the rail used to be.
func TestRailPaneFloats(t *testing.T) {
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
	if shown.pane.Min.X != railMarginDp {
		t.Errorf("pane leading edge at x=%d, want the pane margin %d", shown.pane.Min.X, railMarginDp)
	}
	if want := barH + railMarginDp; shown.pane.Min.Y != want {
		t.Errorf("pane top at y=%d, want %d — the pane floats below the toolbar row, it does not butt against it", shown.pane.Min.Y, want)
	}
	if want := size.Y - railMarginDp; shown.pane.Max.Y != want {
		t.Errorf("pane bottom at y=%d, want %d — the ground shows below the pane too", shown.pane.Max.Y, want)
	}
	if w := shown.pane.Dx(); w != treeWidthDp {
		t.Errorf("pane width %d, want the rail's own %d", w, treeWidthDp)
	}
	if want := shown.pane.Max.X + railMarginDp; shown.contentX != want {
		t.Errorf("note column starts at x=%d, want %d — past the pane's trailing margin", shown.contentX, want)
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

// TestKeyboardReachesTheRailControls walks the focus ring the way Tab
// does and asserts it visits both of the rail's switches — the toolbar
// row's toggle, which is the only way back once the pane is gone, and the
// pane's own hide control — as well as the rail's rows, which take the
// arrows from there. A pane that only a pointer can put away is a pane
// half the window's users cannot put away.
func TestKeyboardReachesTheRailControls(t *testing.T) {
	tok := themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	var ops op.Ops
	var r input.Router
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(1100, 800)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
		Ops:         &ops,
	}
	f := &frameState{asideW: frameAsideDp}
	v := &treeView{list: list.NewState()}
	m := goldenModel()

	tgtx := gtx
	tgtx.Constraints = layout.Exact(image.Pt(1100, int(toolbarHeight(tok))))
	f.layoutToolbar(tgtx, m, tok)

	rgtx := gtx
	rgtx.Constraints = layout.Exact(image.Pt(treeWidthDp, 700))
	v.layout(rgtx, m, tok, nil)

	r.Frame(&ops)
	src := r.Source()
	want := map[string]event.Tag{
		"the toolbar row's rail toggle": &f.toggleClick,
		"the pane's own hide control":   &v.hideClick,
		"the rail's rows":               v.list.Focus(),
	}
	seen := map[string]bool{}
	for range 64 {
		r.MoveFocus(key.FocusForward)
		for name, tag := range want {
			if src.Focused(tag) {
				seen[name] = true
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("the focus ring never reaches %s", name)
		}
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
// Both rail states are measured. Hiding the rail rebuilds the whole
// composition, and a budget that only holds in one of them holds in
// neither.
func TestChromeBudget(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true

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
				t.Errorf("the window spends %d dp above its first content row, over the %d dp budget — a band has come back",
					st.geom.rowTop, chromeBudgetDp)
			}
			if st.geom.rowTop <= 0 {
				t.Fatalf("the content row starts at y=%d; the frame drew no chrome row at all", st.geom.rowTop)
			}
			// The rail pane hangs its own margin below the row, and
			// nothing may stand between the two.
			if !st.geom.pane.Empty() {
				if want := st.geom.rowTop + railMarginDp; st.geom.pane.Min.Y != want {
					t.Errorf("the rail pane starts at y=%d, want %d — its own margin below the row and nothing else",
						st.geom.pane.Min.Y, want)
				}
			}
		})
	}
}

// TestChromeHeightMatchesTheRow asserts the two answers to "how much of
// the top is not document" agree: the one the frame lays the content row
// out from, and the one chromeHeight gives the overlays. They are
// computed in different files from different facts — the row's height
// here, the line the window buttons were placed on there — and if they
// drift, a toast lands on the vault's own controls or floats a band below
// them, which is the same class of defect as the band itself.
func TestChromeHeightMatchesTheRow(t *testing.T) {
	tok := themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	row := toolbarHeight(tok)
	// What the vault screen asks for on every emission that selects it.
	placeWindowButtons(row / 2)
	if got := chromeHeight(); got != row {
		t.Errorf("the overlays are inset by %v dp while the content row starts at %v dp", got, row)
	}
	if row > chromeBudgetDp {
		t.Errorf("the chrome row alone is %v dp, over the %d dp budget", row, chromeBudgetDp)
	}
}
