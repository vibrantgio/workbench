package main

import (
	"image"
	"image/color"
	"reflect"
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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
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

// fillSlot is a slot that takes everything it is given and paints nothing,
// so a measurement is of the arrangement and not of a payload.
func fillSlot(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// railColumn lays the rail's own column out at the size the context gives,
// the way the pane does: the strip reserved and drawn last, the find field
// under it, the rows under that, and the window's Toggle Sidebar button
// bare at the strip's trailing corner.
func railColumn(gtx layout.Context, v *treeView, m Model, tok themeTokens,
	fieldW layout.Widget, toggle *shell.ToggleSidebar) (layout.Dimensions, shell.PaneColumnLayout) {
	col := v.column(func() Model { return m }, func() themeTokens { return tok }, fieldW)
	return col.Layout(gtx, tok.col, goldenToolbarLead, toggle)
}

// resolveColumn pulls the first pane column a sidebar stream emits, the way
// components/golden pulls the first value a slot stream emits.
func resolveColumn(obs rx.Observable[shell.PaneColumn]) (shell.PaneColumn, error) {
	var col shell.PaneColumn
	_, err := golden.Resolve(rx.Map(obs, func(c shell.PaneColumn) layout.Widget {
		col = c
		return fillSlot
	}))
	return col, err
}

// paneSlot is a pane column of the same shape, recording the width the pane
// handed its rows — which is the only place a caller can read that back
// from.
func paneSlot(w *int) shell.PaneColumn {
	return shell.PaneColumn{Rows: func(gtx layout.Context) layout.Dimensions {
		if w != nil {
			*w = gtx.Constraints.Max.X
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}}
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
	const ctrl = railToggleWidthDp
	nav := 2*ctrl + 1

	for _, c := range []struct {
		name   string
		hidden bool
	}{
		{name: "rail shown", hidden: false},
		{name: "rail hidden", hidden: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			var ops op.Ops
			gtx := layout.Context{
				Constraints: layout.Exact(image.Pt(windowW, windowH)),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Ops:         &ops,
			}
			m := goldenModel()
			m.SidebarHidden = c.hidden
			f := newFrameState(defaultArrangement())
			f.leading = func() unit.Dp { return goldenLeading }
			f.layout(gtx, m, tok, paneSlot(nil), fillSlot, nil)

			var r input.Router
			r.Frame(&ops)
			// Probed on the window buttons' own centre line, which is
			// where the toolbar's controls stand: a probe on the row's
			// middle would ask about a control from above it and be told,
			// quite correctly, that nothing is there.
			moveAt := func(x int) bool {
				a, ok := r.ActionAt(f32.Pt(float32(x), float32(windowButtons.Center)))
				return ok && a == system.ActionMove
			}

			// Where the leading cluster starts: the content column's own
			// edge inset with the pane standing, and past the window
			// buttons with it away, where the show toggle stands first.
			contentX := f.geom.ContentX
			start := contentX + noteInsetDp
			lead := 0
			if c.hidden {
				lead = int(goldenToolbarLead)
				start = lead + ctrl + toolbarControlGapDp
			}
			nameW := bandTrailWidth(f, m, tok, windowW-contentX)

			// The bare run between the document's name and the find's slot
			// is the largest empty stretch, and the one a hand reaches for.
			// Its own two ends are probed: the column after the trail and
			// the one before the find's slot begins. The trailing gutter
			// moves the window too, so the probe ends on the row's last dp.
			clusterEnd := start + nav + toolbarTitleGapDp + nameW
			slotLo := windowW - toolbarTrailDp - int(findFieldDp)
			// The reserved room inside the find's own slot moves the window
			// too: shut, the slot is bare with a capsule at its trailing end.
			slack := windowW - toolbarTrailDp - railToggleWidthDp - 20
			for _, x := range []int{clusterEnd + 20, slotLo - 20, slack, windowW - 1} {
				if !moveAt(x) {
					t.Errorf("no window-move action at x=%d; the toolbar holds no control there, so it must move the window", x)
				}
			}
			// Every control's own span belongs to the control.
			controls := []int{start + ctrl/2, windowW - toolbarTrailDp - ctrl/2}
			if c.hidden {
				controls = append(controls, lead+ctrl/2)
			}
			for _, x := range controls {
				if moveAt(x) {
					t.Errorf("window-move action at x=%d; a control's own span must not move the window", x)
				}
			}
			// The window buttons' own span is nobody's to claim: a move
			// action there would fight them for the press.
			if c.hidden && moveAt(lead/2) {
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

	measure := func(hidden bool) shell.PaneLayout {
		return shell.TwoColumn{
			PaneWidth:   treeWidthDp,
			PaneHidden:  hidden,
			Toolbar:     shell.Toolbar{Metrics: shell.ToolbarMetrics{Depth: barH}},
			Main:        fillSlot,
			Footer:      fillSlot,
			FooterDepth: footH,
		}.Measure(gtx, size)
	}

	shown := measure(false)
	if shown.Pane.Empty() {
		t.Fatal("no rail pane with the rail shown")
	}
	if want := (image.Pt(railMarginDp, railMarginDp)); shown.Pane.Min != want {
		t.Errorf("pane starts at %v, want %v — one margin inside the window's top-leading corner, with the window's own plane above it", shown.Pane.Min, want)
	}
	if want := size.Y - railMarginDp; shown.Pane.Max.Y != want {
		t.Errorf("pane bottom at y=%d, want %d — one margin above the window's bottom edge", shown.Pane.Max.Y, want)
	}
	if w := shown.Pane.Dx(); w != treeWidthDp {
		t.Errorf("pane width %d, want the rail's own %d", w, treeWidthDp)
	}
	if shown.ContentX != shown.Pane.Max.X {
		t.Errorf("content area starts at x=%d, want the pane's trailing edge %d", shown.ContentX, shown.Pane.Max.X)
	}
	if shown.RowTop != barH {
		t.Errorf("the content area's first document row starts at y=%d, want %d — below its own toolbar and nothing else", shown.RowTop, barH)
	}
	// The status bar is the content area's own foot and the rail is not
	// measured against it: the rail runs to the window's bottom edge
	// whatever the content area spends down there.
	if want := size.Y - footH; shown.FootTop != want {
		t.Errorf("the status bar starts at y=%d, want %d — one bar above the window's bottom edge", shown.FootTop, want)
	}
	if got := shown.RowTop + shown.RowH; got != shown.FootTop {
		t.Errorf("the content area's columns end at y=%d and the status bar starts at y=%d; they must meet", got, shown.FootTop)
	}

	hidden := measure(true)
	if !hidden.Pane.Empty() {
		t.Errorf("rail pane %v with the rail hidden, want none", hidden.Pane)
	}
	if hidden.ContentX != 0 {
		t.Errorf("note column starts at x=%d with the rail hidden, want the window's own edge", hidden.ContentX)
	}
	if hidden.RowH != shown.RowH || hidden.RowTop != shown.RowTop || hidden.FootTop != shown.FootTop {
		t.Errorf("hiding the rail moved the content row: %+v vs %+v", hidden, shown)
	}

	// A window too narrow to seat both the pane and a readable note keeps
	// the note: the pane yields rather than squeezing the document.
	narrow := image.Pt(200, 800)
	ngtx := gtx
	ngtx.Constraints = layout.Exact(narrow)
	if g := (shell.TwoColumn{PaneWidth: treeWidthDp, Main: fillSlot}).Measure(ngtx, narrow); g.Pane.Dx() > narrow.X/2 {
		t.Errorf("pane %v takes more than half of a %d dp window", g.Pane, narrow.X)
	}
}

// TestPaneFocusOrder walks the focus ring the way Tab does and asserts
// the order it visits the sidebar in: the find field, then the rows, and
// only after both of them the pane's own three marks, in the order they
// stand. What is asserted is the order, not the reachability:
// laid out where they are drawn — the pane's top trailing corner — the
// marks would stand between the field and the rows, and Tab then Return
// from the field would put the whole pane away instead of opening the note
// the reader had just filtered for.
//
// The foot sits after the rows and before the marks because it acts on
// what the pane shows rather than on the pane. The toggle stays last of
// them: it is the pane talking about itself, and nothing the pane is for
// may stand behind it.
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
	v := &treeView{list: list.NewState()}
	var field, hide widget.Clickable
	fieldW := func(gtx layout.Context) layout.Dimensions {
		return field.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 24)}
		})
	}
	railColumn(gtx, v, goldenModel(), tok, fieldW, railToggle(&hide, nil))

	r.Frame(&ops)
	src := r.Source()
	stops := []struct {
		name string
		tag  event.Tag
	}{
		{"the find field", &field},
		{"the rail's rows", v.list.Focus()},
		{"the pane's rescan", &v.rescanClick},
		{"the pane's vault switch", &v.switchClick},
		{"the pane's own toggle", &hide},
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
	v := &treeView{list: list.NewState()}
	f := newFrameState(defaultArrangement())
	f.leading = func() unit.Dp { return goldenLeading }
	m := goldenModel()
	col := v.column(func() Model { return m }, func() themeTokens { return tok }, nil)
	frame := func() {
		f.layout(gtx, m, tok, col, fillSlot, nil)
		r.Frame(&ops)
	}
	frame()

	pane := f.geom.Pane
	stripY := float32(pane.Min.Y) + float32(paneStripDp)/2
	middle := f32.Pt(float32(pane.Min.X+120), stripY)
	// The toggle stands BARE at the pane's top trailing corner, one margin
	// in from its trailing edge, where
	// voicememos-multi-folder-2026-09-18.png keeps a sidebar pane's own
	// marks.
	toggle := f32.Pt(float32(pane.Max.X-railMarginDp-markLargeDp/2), stripY)

	if a, ok := r.ActionAt(middle); !ok || a != system.ActionMove {
		t.Errorf("no window-move action at %v; the strip's empty middle is the pane's drag handle", middle)
	}
	if a, ok := r.ActionAt(toggle); ok && a == system.ActionMove {
		t.Errorf("window-move action at %v; the toggle's own span must not move the window", toggle)
	}

	r.Queue(pointer.Event{Kind: pointer.Move, Position: toggle, Source: pointer.Mouse})
	ops.Reset()
	frame()
	if !f.toggleClick.Hovered() {
		t.Errorf("a pointer at %v does not reach the pane's toggle through the inset and the rounded clip", toggle)
	}
}

// paneStripFrame lays the whole window out with a live folder rail in it and
// collects what the pane's own marks post. Everything is read through the
// frame's inset and the pane's rounded clip, which is the geometry a real
// press crosses, and through the frame's own input router, which is what
// delivers one.
type paneStripFrame struct {
	v      *treeView
	f      *frameState
	gtx    layout.Context
	ops    op.Ops
	r      input.Router
	m      Model
	posted []mvu.Message
}

func newPaneStripFrame(hidden bool) *paneStripFrame {
	p := &paneStripFrame{
		v: &treeView{list: list.NewState()},
		f: newFrameState(defaultArrangement()),
		m: goldenModel(),
	}
	p.m.SidebarHidden = hidden
	p.f.leading = func() unit.Dp { return goldenLeading }
	p.v.post = func(_ layout.Context, msg mvu.Message) { p.posted = append(p.posted, msg) }
	p.gtx = layout.Context{
		Constraints: layout.Exact(image.Pt(windowW, windowH)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      p.r.Source(),
		Ops:         &p.ops,
	}
	return p
}

func (p *paneStripFrame) frame() {
	tok := goldenTokens()
	p.ops.Reset()
	col := p.v.column(func() Model { return p.m }, func() themeTokens { return tok }, nil)
	p.f.layout(p.gtx, p.m, tok, col, fillSlot, nil)
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

// paneMarkStride is what one mark and the gap after it spend: the mark box
// every chrome mark in this window is drawn at and the pattern's measured
// band between two bare marks, which together are the 42 dp between drawn
// centres read off voicememos-multi-folder-2026-09-18.png.
const paneMarkStride = markLargeDp + pane.MarkGapDp

// TestThePaneStripStandsItsMarksBeforeTheToggle reads the pane's top strip
// off a composed window: three marks at its trailing corner, in reading
// order rescan, switch vault, toggle, each the bare mark box wide, each
// pair's drawn centres the measured 42 dp apart, and the toggle's own box
// one margin in from the pane's trailing edge where the Pane pattern keeps
// it.
//
// The two that act on the vault stand nowhere in the chrome row beside the
// pane: a control that acts on the pane's collection waits for the pane, so
// the band is walked end to end and must answer neither.
func TestThePaneStripStandsItsMarksBeforeTheToggle(t *testing.T) {
	p := newPaneStripFrame(false)
	p.frame()
	p.frame()

	region := p.f.geom.Pane
	stripY := region.Min.Y + paneStripDp/2
	rescanLo, rescanHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.v.rescanClick)
	switchLo, switchHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.v.switchClick)
	hideLo, hideHi := p.spanIn(region.Min.X, region.Max.X, stripY, &p.f.toggleClick)

	for _, c := range []struct {
		name   string
		lo, hi int
	}{
		{"rescan", rescanLo, rescanHi},
		{"the vault switch", switchLo, switchHi},
		{"the toggle", hideLo, hideHi},
	} {
		if c.lo < 0 {
			t.Fatalf("%s stands nowhere in the pane's strip", c.name)
		}
		if w := c.hi - c.lo + 1; w != markLargeDp {
			t.Errorf("%s is %d dp wide, want the bare mark box's %d", c.name, w, markLargeDp)
		}
	}
	if !(rescanHi < switchLo && switchHi < hideLo) {
		t.Errorf("the strip runs rescan %d-%d, switch %d-%d, toggle %d-%d; it reads rescan, switch vault, then the toggle",
			rescanLo, rescanHi, switchLo, switchHi, hideLo, hideHi)
	}
	for _, c := range []struct {
		name   string
		lo, hi int
	}{
		{"rescan and the vault switch", rescanLo, switchLo},
		{"the vault switch and the toggle", switchLo, hideLo},
	} {
		if got := c.hi - c.lo; got != paneMarkStride {
			t.Errorf("%s stand %d dp between centres, want the measured %d", c.name, got, paneMarkStride)
		}
	}
	if got, want := region.Max.X-hideHi-1, railMarginDp; got != want {
		t.Errorf("the toggle ends %d dp from the pane's trailing edge, want the pattern's margin of %d", got, want)
	}

	// The band beside the pane, walked end to end on the line its own
	// controls stand on.
	for _, c := range []struct {
		name  string
		click *widget.Clickable
	}{
		{"rescan", &p.v.rescanClick},
		{"the vault switch", &p.v.switchClick},
	} {
		if lo, hi := p.spanIn(region.Max.X, windowW, int(windowButtons.Center), c.click); lo >= 0 {
			t.Errorf("%s answers the band at %d-%d; it stands on the pane and nowhere else", c.name, lo, hi)
		}
	}
}

// TestThePaneMarksPostTheVaultsActions presses each of the pane's own two
// vault actions where the strip stands it and reads back what the pane
// posted. Where each stands is stated from the pane's trailing edge rather
// than read off the layout: the toggle one margin in, and each mark before
// it one stride further.
func TestThePaneMarksPostTheVaultsActions(t *testing.T) {
	for _, c := range []struct {
		name string
		back int
		want mvu.Message
	}{
		{"rescan", 2, Rescan{}},
		{"the vault switch", 1, SwitchVault{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := newPaneStripFrame(false)
			p.frame()
			p.frame()

			region := p.f.geom.Pane
			x := float32(region.Max.X - railMarginDp - markLargeDp/2 - c.back*paneMarkStride)
			at := f32.Pt(x, float32(region.Min.Y)+float32(paneStripDp)/2)
			p.posted = nil
			p.r.Queue(
				pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
				pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
			)
			p.frame()
			p.frame()
			if !postedOneOf(p.posted, c.want) {
				t.Errorf("a press on %s at %v posted %v, want %T", c.name, at, p.posted, c.want)
			}
		})
	}
}

// TestThePaneMarksAnswerTheKeyboard drives the pane's two vault actions the
// way a reader without a pointer does: Tab to each and activate it. A
// control only a pointer can operate is a control half the window's users
// cannot operate.
func TestThePaneMarksAnswerTheKeyboard(t *testing.T) {
	for _, c := range []struct {
		name  string
		click func(v *treeView) *widget.Clickable
		want  mvu.Message
	}{
		{"rescan", func(v *treeView) *widget.Clickable { return &v.rescanClick }, Rescan{}},
		{"the vault switch", func(v *treeView) *widget.Clickable { return &v.switchClick }, SwitchVault{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := newPaneStripFrame(false)
			p.frame()

			target := c.click(p.v)
			src := p.r.Source()
			reached := false
			for range 64 {
				p.r.MoveFocus(key.FocusForward)
				p.frame()
				if src.Focused(target) {
					reached = true
					break
				}
			}
			if !reached {
				t.Fatalf("Tab never reaches %s in the pane's strip", c.name)
			}
			p.r.ClickFocus()
			p.posted = nil
			p.frame()
			p.frame()
			if !postedOneOf(p.posted, c.want) {
				t.Errorf("activating %s from the keyboard posted %v, want %T", c.name, p.posted, c.want)
			}
		})
	}
}

// TestWithThePaneAwayTheRowHoldsTheToggleAlone reads the chrome row in the
// collapsed state: the toggle that brings the pane back at its leading end,
// and not one stand-in for the two controls that went with the pane. Only
// the toggle travels, which is what this platform's own sidebars do — so
// while the pane is away the window offers the vault's actions nowhere, and
// a screen reader is told of neither.
func TestWithThePaneAwayTheRowHoldsTheToggleAlone(t *testing.T) {
	p := newPaneStripFrame(true)
	p.frame()
	p.frame()

	y := int(windowButtons.Center)
	lo, hi := p.spanIn(0, windowW, y, &p.f.toggleClick)
	if lo < 0 {
		t.Fatal("the chrome row holds no toggle to bring the pane back")
	}
	if want := int(goldenToolbarLead); lo != want {
		t.Errorf("the toolbar's toggle leads at %d, want the shell's leading rule over the window buttons' measured edge, %d", lo, want)
	}
	if w := hi - lo + 1; w != railToggleWidthDp {
		t.Errorf("the row's toggle is %d dp wide, want the bordered toolbar control's %d", w, railToggleWidthDp)
	}
	for _, c := range []struct {
		name  string
		click *widget.Clickable
	}{
		{"rescan", &p.v.rescanClick},
		{"the vault switch", &p.v.switchClick},
	} {
		if clo, chi := p.spanIn(0, windowW, y, c.click); clo >= 0 {
			t.Errorf("%s answers the window at %d-%d with the pane away; it went with the pane", c.name, clo, chi)
		}
	}
	spoken := map[string]bool{}
	for _, n := range p.r.AppendSemantics(nil) {
		spoken[n.Desc.Label] = true
	}
	for _, gone := range []string{"Rescan", "Switch Vault"} {
		if spoken[gone] {
			t.Errorf("the window names %q with the pane away; the control it names is not there", gone)
		}
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

// TestTheRowRecallsTheHiddenPane asserts the one thing the pane's own
// corner cannot do: bring the pane back. The Toggle Sidebar button is one
// control with two places — bare at the pane's trailing corner while the
// rail stands, bordered at the toolbar's leading end once it is away — and
// the keyboard reaches it in both, since a pane only a pointer can recall
// is a pane half the window's users cannot recall.
func TestTheRowRecallsTheHiddenPane(t *testing.T) {
	tok := goldenTokens()
	for _, c := range []struct {
		name   string
		hidden bool
	}{
		{"rail hidden", true},
		{"rail shown", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			var ops op.Ops
			var r input.Router
			gtx := layout.Context{
				Constraints: layout.Exact(image.Pt(windowW, windowH)),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Source:      r.Source(),
				Ops:         &ops,
			}
			m := goldenModel()
			m.SidebarHidden = c.hidden
			f := newFrameState(defaultArrangement())
			f.leading = func() unit.Dp { return goldenLeading }
			f.layout(gtx, m, tok, paneSlot(nil), fillSlot, nil)

			r.Frame(&ops)
			src := r.Source()
			seen := false
			for range 32 {
				r.MoveFocus(key.FocusForward)
				if src.Focused(&f.toggleClick) {
					seen = true
				}
			}
			if !seen {
				t.Error("the keyboard never reaches the Toggle Sidebar button; a pane only a pointer can recall is half a pane")
			}
			// And it stands in one place per state: on the pane while the
			// rail is there, in the toolbar once it is away.
			if c.hidden && !f.geom.Pane.Empty() {
				t.Error("the pane still stands with the rail hidden")
			}
			if !c.hidden && f.geom.Pane.Empty() {
				t.Error("the pane is gone with the rail shown")
			}
		})
	}
}

// chromeBudgetDp is what the vault window may spend between its top edge
// and its first row of content. It is the platform's toolbar band and
// nothing more: 52, the depth every stored toolbar capture measures and the
// band the rail's pane is set one margin into — the single chrome row's
// full height and no room for a second thing above it.
var chromeBudgetDp = toolbarDepthDp

// TestChromeBudget holds the vault window's chrome to that budget, by
// laying the whole window out at the size it opens at and asking the frame
// where it put the first content row. The row's own height says nothing
// about a band stacked above it, and a band stacked above it is the defect.
//
// The chrome row belongs to the content area rather than spanning the
// window, so the measurement is stated per column. The content area spends
// the row's own height above its first document row — the band's 52 dp — and
// no more. The sidebar column spends its own margin and nothing else: its
// pane starts one margin below the window's top edge, and the assertion
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
	row := toolbarDepthDp

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

			if st.geom.RowTop > chromeBudgetDp {
				t.Errorf("the content area spends %d dp above its first document row, over the %d dp budget — a band has come back",
					st.geom.RowTop, chromeBudgetDp)
			}
			if st.geom.RowTop != row {
				t.Errorf("the content area spends %d dp above its first document row, want the chrome row's own %d dp — nothing else may stand there",
					st.geom.RowTop, row)
			}
			// The sidebar's own budget: its margin and nothing else.
			// Anything more above the pane would be a chrome band.
			if !st.geom.Pane.Empty() && st.geom.Pane.Min.Y != railMarginDp {
				t.Errorf("the sidebar's pane starts at y=%d, want its own margin %d and nothing above it but the window's surface", st.geom.Pane.Min.Y, railMarginDp)
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
	row := unit.Dp(shell.ToolbarDepth)
	// What the vault screen states on every emission that selects it: the
	// chrome band is the row's height, and the buttons sit at the window's
	// own inset.
	topBand.Store(row)
	if got := chromeHeight(); got != row {
		t.Errorf("the overlays are inset by %v dp while the content row starts at %v dp", got, row)
	}
	if int(row) > chromeBudgetDp {
		t.Errorf("the toolbar alone is %v dp, over the %d dp budget", row, chromeBudgetDp)
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

	if !standsInTheStrip(shown) {
		t.Error("with the pane standing the window asks for no placement of its own; the buttons stand at the window's own inset")
	}
	if !standsInTheStrip(hidden) {
		t.Error("with the pane away the window asks for no placement; dismissing the pane moved a control that is not the pane's")
	}

	// The buttons' band in the PANE's own coordinates, which the pane's
	// strip is cut in: the pane stands one margin inside the window's top
	// edge and the buttons are measured from the window's glass, so the
	// strip owes that margin back at both ends.
	buttonsTop := buttonInsetDp - railMarginDp
	buttonsBottom := buttonsTop + desktop.WindowButtonDiameter
	stripTop, stripBottom := 0, paneStripDp
	if stripTop > buttonsTop {
		t.Errorf("the pane's strip begins at y=%d, below the buttons' top edge at y=%d — the pane's content would start under them", stripTop, buttonsTop)
	}
	if stripBottom < buttonsBottom {
		t.Errorf("the pane's strip ends at y=%d, above the buttons' bottom edge at y=%d — the pane's content would run under them", stripBottom, buttonsBottom)
	}
	if mid := railMarginDp + paneStripDp/2; unit.Dp(mid) != windowButtons.Center {
		t.Errorf("the strip's middle line is y=%d in the window and the buttons' is y=%v; the pane's toggle centres on the strip and would sit off their line", mid, windowButtons.Center)
	}
	if mid := toolbarDepthDp / 2; unit.Dp(mid) != windowButtons.Center {
		t.Errorf("the band's middle line is y=%d and the buttons' is y=%v; a control centred in the band would sit off their line", mid, windowButtons.Center)
	}
}

// TestTheRailIsAnInsetPane reads the composed window: the rail is a pane
// set one margin in from the window's leading, top and bottom edges, flush
// with the note on the fourth, carrying the platform's rim on every one of
// its four sides and no seam anywhere — the toolbar band's rows included,
// since the pane is one object from its top edge to its foot.
//
// MEASURED, voicememos-multi-folder-2026-09-18.png and
// finder-window-untinted-dark.png: the pane stands eight pixels inside the
// window on three sides with the window's own plane showing in them, and the
// content begins at the fourth with no gap.
func TestTheRailIsAnInsetPane(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			rail := st.geom.Pane
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
			// line crosses the band and the pane passes straight through it.
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
					t.Errorf("the pane's %s edge at (%d,%d) draws %v, want the platform's rim %v", probe.what, probe.x, probe.y, got, rim)
				}
				if got := img.RGBAAt(probe.x+probe.inX, probe.y+probe.inY); !sameColor(got, fill) {
					t.Errorf("one pixel inside the pane's %s edge draws %v, want the chrome level %v — the rim is wider than a hairline", probe.what, got, fill)
				}
				if rim != seam {
					if got := img.RGBAAt(probe.x, probe.y); sameColor(got, seam) {
						t.Errorf("the pane's %s edge draws the seam %v; an inset pane is bounded by its rim", probe.what, seam)
					}
				}
			}
			// Past the trailing rim stands the note's own surface under the
			// pane's shadow, which only darkens it and recovers outward.
			v0 := img.RGBAAt(rail.Max.X, midY)
			v1 := img.RGBAAt(rail.Max.X+1, midY)
			far := img.RGBAAt(rail.Max.X+paneShadowReachDp, midY)
			if v0.R > tc.colors.TextBackground.R || v0.R > v1.R {
				t.Errorf("past the pane's trailing rim the note's surface reads %v then %v against its own %v — a second line stands beside the pane's own",
					v0, v1, tc.colors.TextBackground)
			}
			if !sameColor(far, tc.colors.TextBackground) {
				t.Errorf("a reach past the pane's trailing rim the note's surface draws %v, want its own %v — the shadow is spent by its reach",
					far, tc.colors.TextBackground)
			}
			// The window's own surface shows in the margin beside the pane,
			// and the pane casts its shadow on it: neither the bare surface
			// nor the pane's own fill is what stands there.
			if got := img.RGBAAt(rail.Min.X-1, midY); sameColor(got, tc.colors.WindowBackground) {
				t.Errorf("the column beside the pane's leading rim draws the bare surface %v; the pane casts a shadow on it", tc.colors.WindowBackground)
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
			for y := 0; y < toolbarDepthDp; y++ {
				if got := img.RGBAAt(asideX, y); sameColor(got, splitter.SeamColor(tc.colors, color.NRGBA{})) {
					t.Fatalf("the column's seam is drawn at y=%d, inside the toolbar band — the band is one across the window's columns and no line crosses it", y)
				}
			}
			// The scan starts clear of the band's own drop shadows. A
			// bordered toolbar control casts onto whatever lies below it and
			// the platform does not cut that off at the band's lower edge
			// (reference/macos/controls.md), and the band's action cluster
			// now stands over this boundary: the rows immediately under the
			// band carry the shadow and not the seam's own colour.
			for y := toolbarDepthDp + toolbarShadowReachDp; y < windowH; y++ {
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
			if got := img.RGBAAt(asideX+seamDp, st.geom.FootTop-1); !sameColor(got, floor) {
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

func newDragFrame(size image.Point, a shell.Arrangement) *dragFrame {
	d := &dragFrame{f: newFrameState(a), size: size}
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
	d.f.layout(d.gtx, goldenModel(), goldenTokens(), paneSlot(&d.slot), fillSlot, nil)
	d.r.Frame(&d.ops)
}

// drag takes hold at x on the row's own middle and pulls to x+by. Two
// frames go first, so that the hit area is known to the router before a
// pointer is put on it.
func (d *dragFrame) drag(x, by float32) {
	d.frame()
	d.frame()
	y := float32(d.f.geom.RowTop + d.f.geom.RowH/2)
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
	d := newDragFrame(image.Pt(windowW, windowH), defaultArrangement())
	d.frame()
	edge := d.f.geom.Pane.Max.X
	d.drag(float32(edge-seamDp), pull)

	if got, want := d.f.railW, unit.Dp(treeWidthDp+pull); got != want {
		t.Fatalf("the drag left the rail at %v dp, want %v", got, want)
	}
	if got, want := d.f.geom.Pane.Dx(), treeWidthDp+pull; got != want {
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

	over := newDragFrame(size, defaultArrangement())
	over.frame()
	over.frame()
	x := float32(size.X - frameAsideDp)
	// The chrome row's own middle, on the seam's line: over the seam, and
	// over a band the boundary does not part.
	y := float32(over.f.geom.RowTop / 2)
	over.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, y)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x-pull, y)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x-pull, y)},
	)
	over.frame()
	if got := over.f.asideW; got != frameAsideDp {
		t.Errorf("a drag begun in the chrome row left the aside at %v dp, want the untouched %v", got, unit.Dp(frameAsideDp))
	}

	in := newDragFrame(size, defaultArrangement())
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

	rail := newDragFrame(size, defaultArrangement())
	rail.frame()
	rail.drag(float32(rail.f.geom.Pane.Max.X-seamDp), 400)
	// The pane's own margin comes off what is left for the three columns:
	// it stands one margin inside the window's leading edge.
	if got, want := int(rail.f.railW), windowW-railMarginDp-gap-frameAsideDp-noteMinWidthDp; got != want {
		t.Errorf("the rail dragged past the note's minimum stopped at %d dp, want %d", got, want)
	}

	aside := newDragFrame(size, defaultArrangement())
	aside.drag(float32(size.X-frameAsideDp), -400)
	if got, want := int(aside.f.asideW), windowW-railMarginDp-treeWidthDp-gap-noteMinWidthDp; got != want {
		t.Errorf("the aside dragged past the note's minimum stopped at %d dp, want %d", got, want)
	}
}

// TestTheRailEdgeDrawsNoSecondLine verifies what the leading splitter
// draws at rest: nothing the pane did not already draw. The boundary there
// is the pane's own rim, and a splitter drawing a line of its own beside it
// would put two edges a pixel apart down the whole pane.
func TestTheRailEdgeDrawsNoSecondLine(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			p := st.geom.Pane
			if p.Empty() {
				t.Fatal("the window laid out no pane to read")
			}
			// The whole straight run of the trailing edge, the band's own
			// rows included: one hairline of the pane's own rim, and the
			// note's surface the pixel after it. The corners' arcs are read
			// out, because the edge is not straight there and the splitter
			// does not draw over them.
			// At one pixel per dp the pane's corner radius is the span's
			// own inset at both ends.
			top, bottom := p.Min.Y+pane.RadiusDp, p.Max.Y-pane.RadiusDp
			// The note column's own pinned seam — the hairline between its
			// pinned head and the document scrolling under it — crosses this
			// edge, because a boundary between two flush regions runs the
			// whole width of the region above it. It is a ROW and not a
			// second line down the pane, so its rows are read out: a row
			// carrying a hundred columns of the seam's own colour well clear
			// of the pane's shadow is that seam and nothing else.
			pinned := pinnedSeamRows(img, tc.colors, p.Max.X+paneShadowReachDp, windowW-frameAsideDp, top, bottom)
			for y := top; y < bottom; y++ {
				if pinned[y] {
					continue
				}
				if got := img.RGBAAt(p.Max.X-seamDp, y); !sameColor(got, paneRim(tc.colors)) {
					t.Fatalf("the pane's trailing edge at y=%d draws %v, want its own rim %v", y, got, paneRim(tc.colors))
				}
				// Past the rim stands the note's surface under the pane's
				// shadow, which only recovers outward: a second line would
				// be a column darker than the one beyond it.
				v0, v1, v2 := img.RGBAAt(p.Max.X, y).R, img.RGBAAt(p.Max.X+1, y).R, img.RGBAAt(p.Max.X+2, y).R
				if v0 > v1 || v1 > v2 || v2 > tc.colors.TextBackground.R {
					t.Fatalf("past the pane's trailing edge at y=%d the note's surface reads %d, %d, %d against its own %d — a second line stands beside the pane's own",
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
	m    Model
	gtx  layout.Context
	ops  op.Ops
	r    input.Router
	size image.Point

	// posted is what the band put on this frame for the loop, reduced into
	// the frame's own model at the end of it: whether the find stands open
	// is the model's, so the band's magnifier asks for it by message.
	posted []mvu.Message
}

func newBandFrame() *bandFrame {
	b := &bandFrame{f: newFrameState(defaultArrangement()), m: goldenModel(), size: image.Pt(windowW, windowH)}
	b.f.leading = func() unit.Dp { return goldenLeading }
	b.f.band = bandFind{find: &b.find}
	b.find.post = func(_ layout.Context, msg mvu.Message) { b.posted = append(b.posted, msg) }
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
	b.f.layout(b.gtx, b.m, goldenTokens(), paneSlot(nil), fillSlot, nil)
	b.r.Frame(&b.ops)
	for _, msg := range b.posted {
		b.m, _ = Update(b.m, msg)
	}
	b.posted = nil
}

// span is one control's own columns, found by walking the band's centre line
// and asking which control the pointer is on. Both leading and trailing are
// inclusive; a control the band does not hold answers -1, -1.
func (b *bandFrame) span(c *widget.Clickable) (int, int) {
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

// bandTrailWidth is how wide the band's trail of places lays out. It is the
// one part of the leading cluster that is not a measurement, so the columns
// after it are read against what the trail actually takes.
//
// It is measured in the window the band stands in — the row's own width,
// with the room the controls after it need — because that room is what
// decides whether the trail is elided.
func bandTrailWidth(f *frameState, m Model, tok themeTokens, rowW int) int {
	var ops op.Ops
	// The room the row's flex hands the trail: the row less what stands
	// before it, with Min zero, which is what a Rigid child is laid out
	// under and what lets the trail report its own width.
	room := rowW - noteInsetDp - (2*railToggleWidthDp + 1) - toolbarTitleGapDp
	gtx := layout.Context{
		Constraints: layout.Constraints{Max: image.Pt(max(room, 0), toolbarDepthDp)},
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	return f.layoutBandTrail(gtx, m, tok).Size.X
}

// TestBandNamesItsControls asserts the band's affordances are in the
// window's semantic tree under the names a screen reader speaks. These
// controls carry a symbol and no word at all — which is what every document
// action in every stored toolbar band carries — so the spoken name is the
// only name there is. The document's own name is in there too: it is drawn
// as bare text, which a reader who cannot see it would otherwise never be
// told.
//
// The vault's two actions are not among them: they stand on the pane, and
// the band this frame lays out carries no pane.
func TestBandNamesItsControls(t *testing.T) {
	b := newBandFrame()
	b.frame()

	spoken := map[string]bool{}
	for _, n := range b.r.AppendSemantics(nil) {
		spoken[n.Desc.Label] = true
	}
	for _, want := range []string{"Find in this note", "Back", "Forward", "Reading list"} {
		if !spoken[want] {
			t.Errorf("the window's semantic tree does not name %q", want)
		}
	}
	for _, gone := range []string{"Rescan", "Switch Vault"} {
		if spoken[gone] {
			t.Errorf("the band names %q; the vault's actions stand on the pane", gone)
		}
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
	at := f32.Pt(float32(windowW-toolbarTrailDp-ctrl/2), float32(paneStripDp)/2)
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	b.frame()
	if !b.m.FindOpen {
		t.Error("pressing the band's search control did not open the find")
	}
}

// TestTheBandComposesLikeTheStoredWindows reads the band's own arrangement
// back off a laid-out frame: the navigation at the leading end of the
// content column's share with the document's name one measured gap after
// it, the search last of all, and the last control eight clear of the
// window's trailing edge. The vault's own two actions stand on the pane and
// so stand nowhere in this row.
//
// MEASURED, reference/macos/controls.md under "What the toolbar band's
// composition measures" and "What the band's segmented control measures":
// the search stands last in all five stored bands,
// finder-window-light.png leaves 14 between its navigation pair's last
// column at x=398 and its title's first painted one at x=413, and the last
// control in all four full windows ends 8 px from the window's own trailing
// edge.
func TestTheBandComposesLikeTheStoredWindows(t *testing.T) {
	b := newBandFrame()
	b.frame()
	b.frame()

	findLo, findHi := b.span(&b.f.findClick)
	navLo, _ := b.span(&b.f.backClick)
	_, fwdHi := b.span(&b.f.fwdClick)

	if findLo < 0 {
		t.Fatal("the search stands nowhere in the band")
	}
	if w := findHi - findLo + 1; w != railToggleWidthDp {
		t.Errorf("the search is %d dp wide, want the bordered toolbar control's %d", w, railToggleWidthDp)
	}
	if !(navLo < findLo) {
		t.Errorf("the band runs navigation from %d and the search %d-%d; the navigation leads and the search stands last in every stored band",
			navLo, findLo, findHi)
	}
	// What lies between the document's name and the find's SLOT is bare
	// band, the run the row flexes: the slot's leading edge is its own
	// fixed trailing end less the width the open recess takes, and the
	// cluster before it does not reach it.
	trail := bandTrailWidth(b.f, b.m, goldenTokens(), windowW-int(b.f.geom.ContentX))
	nameEnd := int(b.f.geom.ContentX) + noteInsetDp + 2*railToggleWidthDp + 1 + toolbarTitleGapDp + trail
	slotLo := windowW - toolbarTrailDp - int(findFieldDp)
	if nameEnd >= slotLo {
		t.Errorf("the document's name runs to %d and the find's slot begins at %d; the cluster stands clear of the slot",
			nameEnd, slotLo)
	}
	if got := windowW - findHi - 1; got != toolbarTrailDp {
		t.Errorf("the band's last control ends %d dp from the window's trailing edge, want the measured %d",
			got, toolbarTrailDp)
	}
	// The navigation is the segmented pair Finder keeps at the leading end of
	// the content column's share: two segments of the bordered control's own
	// width with one hairline between them, the whole standing on the note
	// column's own edge inset.
	if want := int(b.f.geom.ContentX) + noteInsetDp; navLo != want {
		t.Errorf("the navigation leads at %d, want the content column's own edge inset at %d", navLo, want)
	}
	if got, want := fwdHi-navLo+1, 2*railToggleWidthDp+1; got != want {
		t.Errorf("the navigation is %d dp wide, want two segments and a hairline: %d", got, want)
	}
}

// TestNothingInTheBandWalksWhenTheFindOpens is the whole point of reserving
// the find's room: the recess grows leftward from a fixed trailing end, the
// way Finder's does when it expands, so a reader who presses the shortcut —
// or the capsule itself — finds every other control exactly where it was.
//
// The columns are read off the laid-out frame with the find shut and again
// with it open, through the same pointer walk the composition test uses.
func TestNothingInTheBandWalksWhenTheFindOpens(t *testing.T) {
	b := newBandFrame()
	b.frame()
	b.frame()

	type column struct{ lo, hi int }
	controls := []struct {
		name  string
		click *widget.Clickable
	}{
		{"the navigation's back segment", &b.f.backClick},
		{"the navigation's forward segment", &b.f.fwdClick},
	}
	read := func() []column {
		cols := make([]column, len(controls))
		for i, c := range controls {
			lo, hi := b.span(c.click)
			cols[i] = column{lo, hi}
		}
		return cols
	}
	shut := read()
	b.m.FindOpen = true
	b.frame()
	b.frame()
	open := read()

	for i, c := range controls {
		if shut[i].lo < 0 {
			t.Fatalf("%s stands nowhere in the band with the find shut", c.name)
		}
		if open[i] != shut[i] {
			t.Errorf("%s stands at %d-%d with the find shut and at %d-%d with it open; the find grows from a fixed trailing end and nothing else moves",
				c.name, shut[i].lo, shut[i].hi, open[i].lo, open[i].hi)
		}
	}
}
