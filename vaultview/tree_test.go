package main

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/tokens"
)

// treeIndex builds an index carrying just the file paths, the only part
// of a FileScan the tree reads.
func treeIndex(paths ...string) *Index {
	idx := &Index{Root: "/v"}
	for _, p := range paths {
		idx.Files = append(idx.Files, FileScan{Path: p})
	}
	return idx
}

// rowSig renders one row compactly for comparison: depth, kind, path.
func rowSig(r TreeRow) string {
	kind := "note"
	if r.IsDir {
		kind = "dir-closed"
		if r.Open {
			kind = "dir-open"
		}
	}
	return fmt.Sprintf("%d %s %s", r.Depth, kind, r.Path)
}

func rowSigs(rows []TreeRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = rowSig(r)
	}
	return out
}

// TestTreeRows table-tests the flattening: visible rows from index plus
// fold state — depth per level, folders before notes in name order,
// hidden dot-directories, a closed fold hiding its whole subtree.
func TestTreeRows(t *testing.T) {
	cases := []struct {
		name  string
		idx   *Index
		folds map[string]bool
		want  []string
	}{
		{
			name: "nil index yields no rows",
			idx:  nil,
			want: nil,
		},
		{
			name: "root notes sort by title, case-insensitively",
			idx:  treeIndex("b.md", "A.md", "c.md"),
			want: []string{
				"0 note A.md",
				"0 note b.md",
				"0 note c.md",
			},
		},
		{
			name: "folders precede notes and sort among themselves",
			idx:  treeIndex("zeta.md", "beta/x.md", "Alpha/y.md"),
			want: []string{
				"0 dir-closed Alpha",
				"0 dir-closed beta",
				"0 note zeta.md",
			},
		},
		{
			name: "closed folds hide every descendant",
			idx:  treeIndex("guide/deep/x.md", "guide/top.md", "readme.md"),
			want: []string{
				"0 dir-closed guide",
				"0 note readme.md",
			},
		},
		{
			name:  "an open fold reveals one level, children still closed",
			idx:   treeIndex("guide/deep/x.md", "guide/top.md", "readme.md"),
			folds: map[string]bool{"guide": true},
			want: []string{
				"0 dir-open guide",
				"1 dir-closed guide/deep",
				"1 note guide/top.md",
				"0 note readme.md",
			},
		},
		{
			name:  "nested opens indent per depth",
			idx:   treeIndex("guide/deep/x.md", "guide/top.md", "readme.md"),
			folds: map[string]bool{"guide": true, "guide/deep": true},
			want: []string{
				"0 dir-open guide",
				"1 dir-open guide/deep",
				"2 note guide/deep/x.md",
				"1 note guide/top.md",
				"0 note readme.md",
			},
		},
		{
			name:  "re-collapsing an ancestor hides the open descendant too",
			idx:   treeIndex("guide/deep/x.md", "readme.md"),
			folds: map[string]bool{"guide": false, "guide/deep": true},
			want: []string{
				"0 dir-closed guide",
				"0 note readme.md",
			},
		},
		{
			name: "dot-directory segments are hidden at any depth",
			idx:  treeIndex(".obsidian/workspace.md", "guide/.trash/gone.md", "guide/kept.md"),
			folds: map[string]bool{
				"guide": true, ".obsidian": true, "guide/.trash": true,
			},
			want: []string{
				"0 dir-open guide",
				"1 note guide/kept.md",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rowSigs(TreeRows(tc.idx, tc.folds))
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

// treeModel is a vault-screen model over a nested index with both notes
// cached, current on the root note — the state vaultScanned leaves when
// the first indexed file sits at the root.
func treeModel() Model {
	model := Model{Screen: screenVault, Vault: "/v", CurAnchor: -1}
	model.Index = treeIndex("top.md", "guide/deep/n.md")
	model = cacheNote(model, noteFromSource("top.md", "# Top\n"))
	model = cacheNote(model, noteFromSource("guide/deep/n.md", "# N\n"))
	model.Current = "top.md"
	model.NavSeq++
	model.History = []HistEntry{{Path: "top.md", Anchor: -1}}
	model.Cursor = 0
	return model
}

// visibleNoteRow returns the tree row for a note path, or nil when the
// flattening does not show it.
func visibleNoteRow(m Model, p string) *TreeRow {
	for _, r := range TreeRows(m.Index, m.Folds) {
		if !r.IsDir && r.Path == p {
			r := r
			return &r
		}
	}
	return nil
}

// TestTreeActiveTracksNavigation walks the model through Navigate and
// GoBack/GoForward: the current note's row stays visible in the flattened
// tree — landing inside a closed folder opens its ancestors — and the row
// the view marks active (Path == Current) is always exactly the current one.
func TestTreeActiveTracksNavigation(t *testing.T) {
	model := treeModel()

	// Navigating into a folded subtree reveals it and moves the mark.
	model, _ = Update(model, Navigate{Path: "guide/deep/n.md"})
	if model.Current != "guide/deep/n.md" {
		t.Fatalf("Current = %q after Navigate", model.Current)
	}
	if !model.Folds["guide"] || !model.Folds["guide/deep"] {
		t.Errorf("Folds = %v; want guide and guide/deep opened by the landing", model.Folds)
	}
	if visibleNoteRow(model, model.Current) == nil {
		t.Fatalf("current note %q not visible in the tree after Navigate", model.Current)
	}

	// GoBack moves the mark back to the root note.
	model, _ = Update(model, GoBack{})
	if model.Current != "top.md" {
		t.Fatalf("Current = %q after GoBack", model.Current)
	}
	if visibleNoteRow(model, model.Current) == nil {
		t.Fatalf("current note %q not visible in the tree after GoBack", model.Current)
	}

	// Collapse the folder by hand, then GoForward back into it: the
	// landing must re-open the fold so the marked row is visible again.
	model, _ = Update(model, ToggleFold{Dir: "guide"})
	if r := visibleNoteRow(model, "guide/deep/n.md"); r != nil {
		t.Fatal("guide/deep/n.md still visible after collapsing guide")
	}
	model, _ = Update(model, GoForward{})
	if model.Current != "guide/deep/n.md" {
		t.Fatalf("Current = %q after GoForward", model.Current)
	}
	if visibleNoteRow(model, model.Current) == nil {
		t.Fatalf("current note %q not visible in the tree after GoForward", model.Current)
	}

	// A tree click is a Navigate too: activating the root note's row
	// moves the mark there and pushes history.
	model, _ = Update(model, Navigate{Path: "top.md"})
	if model.Current != "top.md" {
		t.Fatalf("Current = %q after tree-click Navigate", model.Current)
	}
	if got := len(model.History); got != 3 {
		t.Errorf("history length = %d after link, back, forward, tree click; want 3", got)
	}
}

// TestToggleFoldCopiesTheMap guards the value semantics: toggling a fold
// must not mutate the map a previous model holds.
func TestToggleFoldCopiesTheMap(t *testing.T) {
	before := treeModel()
	before.Folds = map[string]bool{"guide": true}
	after, _ := Update(before, ToggleFold{Dir: "guide"})
	if !before.Folds["guide"] {
		t.Error("ToggleFold mutated the previous model's fold map")
	}
	if after.Folds["guide"] {
		t.Error("ToggleFold did not close the open fold")
	}
}

// TestMatchRows table-tests the find field's filter: name matches, the
// folder fallback, case-insensitivity, the folder annotation, hidden
// dot-directories, and the blank query that answers nothing.
func TestMatchRows(t *testing.T) {
	idx := treeIndex(
		"Design.md",
		"guide/Getting Started.md",
		"guide/deep/design notes.md",
		".obsidian/design.md",
	)
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"a blank query answers nothing", "", nil},
		{"whitespace is not a query", "   ", nil},
		{
			name:  "name matches ignore case and are substrings",
			query: "design",
			want:  []string{"Design ()", "design notes (guide/deep)"},
		},
		{
			name:  "the folder path matches too, so a folder narrows",
			query: "guide/",
			want:  []string{"design notes (guide/deep)", "Getting Started (guide)"},
		},
		{
			name:  "no match is an empty answer, not the whole vault",
			query: "nothing here",
			want:  nil,
		},
		{
			name:  "dot-directories stay hidden",
			query: ".obsidian",
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := MatchRows(idx, tc.query)
			var got []string
			for i, r := range rows {
				if r.IsDir {
					t.Errorf("row %d is a folder; the filter answers notes only", i)
				}
				if r.Depth != 0 {
					t.Errorf("row %d has depth %d; filtered rows are flat", i, r.Depth)
				}
				if r.Idx != i {
					t.Errorf("row %d carries Idx %d", i, r.Idx)
				}
				got = append(got, fmt.Sprintf("%s (%s)", r.Name, r.Detail))
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}

	if rows := MatchRows(nil, "design"); rows != nil {
		t.Errorf("MatchRows over no index = %v, want nothing", rows)
	}
}

// TestTreeClaimsOnlyItsRail: the shell lets its sidebar slot size itself
// and gives the main column whatever is left, so a rail answering with the
// constraint it was handed would take the whole window and leave the note
// nothing. Every state must answer the rail's own width — with rows,
// filtered to none, and before the scan lands.
func TestTreeClaimsOnlyItsRail(t *testing.T) {
	tok := themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	filled := treeModel()
	filled.Folds = map[string]bool{"guide": true, "guide/deep": true}
	filtered := filled
	filtered.Filter = "no such note"

	cases := []struct {
		name  string
		model Model
	}{
		{"rows", filled},
		{"filtered to nothing", filtered},
		{"before the scan lands", Model{Screen: screenVault, Vault: "/v", Scanning: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &treeView{list: list.NewState()}
			var ops op.Ops
			gtx := layout.Context{
				Constraints: layout.Constraints{Max: image.Pt(1100, 700)},
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Ops:         &ops,
			}
			dims := v.layout(gtx, tc.model, tok, nil)
			if dims.Size.X != treeWidthDp {
				t.Errorf("rail width = %d dp, want %d — the main column would get %d",
					dims.Size.X, treeWidthDp, 1100-dims.Size.X)
			}
			if dims.Size.Y != 700 {
				t.Errorf("rail height = %d, want the full row height 700", dims.Size.Y)
			}
		})
	}
}

// TestPaneFootStandsOutsideTheRows asserts the arrangement that keeps the
// vault's actions where a reader can always reach them: the foot takes
// its band off the pane's bottom, the rows end exactly where it begins,
// and the rows still get the bulk of the pane. The rows scroll inside
// their own band, so no number of notes can push the actions off the
// window and no scroll position can take them out of reach.
//
// It is asserted off what the pane laid out rather than recomputed from
// the constants that placed it.
func TestPaneFootStandsOutsideTheRows(t *testing.T) {
	tok := goldenTokens()
	const paneH = 700
	for _, c := range []struct {
		name  string
		model Model
	}{
		{"a filled vault", goldenModel()},
		{"an empty vault", Model{Screen: screenVault, Vault: "/v", Index: &Index{}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
			var ops op.Ops
			gtx := layout.Context{
				Constraints: layout.Exact(image.Pt(treeWidthDp, paneH)),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Ops:         &ops,
			}
			v.layout(gtx, c.model, tok, nil)

			g := v.geom
			if g.foot.Empty() {
				t.Fatal("the pane laid out no foot; the vault's actions have nowhere to stand")
			}
			if g.foot.Max.Y != paneH {
				t.Errorf("the foot ends at y=%d, want the pane's own bottom edge %d", g.foot.Max.Y, paneH)
			}
			if g.rows.Max.Y != g.foot.Min.Y {
				t.Errorf("the rows end at y=%d and the foot begins at y=%d; the two must meet and not overlap",
					g.rows.Max.Y, g.foot.Min.Y)
			}
			if g.rows.Dy() <= g.foot.Dy() {
				t.Errorf("the rows get %d dp against the foot's %d; the pane is for the notes", g.rows.Dy(), g.foot.Dy())
			}
		})
	}
}

// TestPaneFootActionsAnswerTheKeyboard drives the foot the way a reader
// without a pointer does: Tab to each action and activate it, through the
// frame's own input router. Both must report a press. What the press then
// means — a rescan that counts what it found, a switch that returns to the
// picker — is asserted at the model; this covers only whether the keyboard
// can get there at all.
func TestPaneFootActionsAnswerTheKeyboard(t *testing.T) {
	tok := goldenTokens()
	for _, c := range []struct {
		name  string
		click func(v *treeView) *widget.Clickable
	}{
		{"rescan", func(v *treeView) *widget.Clickable { return &v.rescanClick }},
		{"switch vault", func(v *treeView) *widget.Clickable { return &v.switchClick }},
	} {
		t.Run(c.name, func(t *testing.T) {
			var r input.Router
			v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
			frame := func() {
				var ops op.Ops
				gtx := layout.Context{
					Constraints: layout.Exact(image.Pt(treeWidthDp, 700)),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Source:      r.Source(),
					Ops:         &ops,
				}
				v.layout(gtx, goldenModel(), tok, nil)
				r.Frame(&ops)
			}
			frame()

			target := c.click(v)
			src := r.Source()
			reached := false
			for range 64 {
				r.MoveFocus(key.FocusForward)
				if src.Focused(target) {
					reached = true
					break
				}
			}
			if !reached {
				t.Fatalf("Tab never reaches the foot's %s action", c.name)
			}
			r.ClickFocus()

			var ops op.Ops
			gtx := layout.Context{
				Constraints: layout.Exact(image.Pt(treeWidthDp, 700)),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Source:      r.Source(),
				Ops:         &ops,
			}
			if !target.Clicked(gtx) {
				t.Errorf("activating the foot's %s action from the keyboard produced no press", c.name)
			}
		})
	}
}

// TestPaneFootNamesItsActions asserts the foot's two affordances are in
// the pane's semantic tree under the names a screen reader speaks. The
// drawn label and the spoken one are set separately, and a control the
// keyboard reaches but nothing names is a control only a sighted reader
// has.
func TestPaneFootNamesItsActions(t *testing.T) {
	tok := goldenTokens()
	var r input.Router
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(treeWidthDp, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
		Ops:         &ops,
	}
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
	v.layout(gtx, goldenModel(), tok, nil)
	r.Frame(&ops)

	spoken := map[string]bool{}
	for _, n := range r.AppendSemantics(nil) {
		spoken[n.Desc.Label] = true
	}
	for _, want := range []string{"Rescan", "Switch Vault"} {
		if !spoken[want] {
			t.Errorf("the pane's semantic tree does not name %q", want)
		}
	}
}

// TestPaneFootAnswersThePointer asserts the foot's actions look like
// controls when a pointer is on them: a bare label says nothing about
// being pressable until something answers, so the hit area fills under the
// pointer. The assertion is pixels — the fill is the whole point, and
// dimensions cannot see it.
//
// The hover is delivered through the frame's own input router at a point
// beside the label rather than on it, which also says the hit area is
// bigger than the glyphs it holds.
func TestPaneFootAnswersThePointer(t *testing.T) {
	tok := goldenTokens()
	size := image.Pt(treeWidthDp, 700)
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
	var r input.Router
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      r.Source(),
		Ops:         &ops,
	}
	v.layout(gtx, goldenModel(), tok, nil)
	r.Frame(&ops)

	foot := v.geom.foot
	at := f32.Pt(float32(treeRowInsetDp+2), float32(foot.Min.Y+foot.Max.Y)/2)
	r.Queue(pointer.Event{Kind: pointer.Move, Position: at, Source: pointer.Mouse})

	ops.Reset()
	v.layout(gtx, goldenModel(), tok, nil)
	r.Frame(&ops)
	if !v.rescanClick.Hovered() {
		t.Fatalf("a pointer at %v is not on the rescan action; its hit area is the label's glyphs alone", at)
	}

	// Captured after the hover has landed: the clickable holds the state,
	// so the stored frame is the hovered one.
	plain := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
	shot := func(v *treeView) *image.RGBA {
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			return v.layout(gtx, goldenModel(), tok, nil)
		})
	}
	if n := golden.PixelDiff(shot(plain), shot(v)); n == 0 {
		t.Error("the foot's action draws the same under the pointer as away from it; nothing marks it as pressable")
	}
}
