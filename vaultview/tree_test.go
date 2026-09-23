package main

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

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
// fold state — depth per level, folders and notes in one run in name
// order, hidden dot-directories, a closed fold hiding its whole subtree.
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
			name: "folders and notes stand in one name-ordered run",
			idx:  treeIndex("beta.md", "Alpha/y.md", "zeta/x.md"),
			want: []string{
				"0 dir-closed Alpha",
				"0 note beta.md",
				"0 dir-closed zeta",
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

// TestTreeClaimsOnlyItsRail: a slot that leaves the width open lets its
// sidebar size itself and gives the main column whatever is left, so a
// rail answering with the constraint it was handed would take the whole
// window and leave the note nothing. Every state must answer the rail's
// own width — with rows, filtered to none, and before the scan lands.
func TestTreeClaimsOnlyItsRail(t *testing.T) {
	tok := themeTokens{
		col:    tokens.PlatformLight,
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

// TestTreeFillsTheWidthTheSlotStates verifies the other half of the rule
// above: where the slot states a width — which is what the pane does,
// laying its column out at exactly the width the reader dragged the
// pane's edge to — the rail is that wide. A rail that held its own 240 dp
// inside a pane dragged wider would widen into nothing.
func TestTreeFillsTheWidthTheSlotStates(t *testing.T) {
	tok := themeTokens{
		col:    tokens.PlatformLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	filled := treeModel()
	filled.Folds = map[string]bool{"guide": true}
	for _, w := range []int{treeWidthDp - 60, treeWidthDp + 140} {
		v := &treeView{list: list.NewState()}
		var ops op.Ops
		gtx := layout.Context{
			Constraints: layout.Exact(image.Pt(w, 700)),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         &ops,
		}
		if got := v.layout(gtx, filled, tok, nil).Size.X; got != w {
			t.Errorf("in a %d dp slot the rail lays out %d dp wide, want the slot's own width", w, got)
		}
	}
}

// TestPaneRowsRunToThePaneFoot asserts the arrangement the rail keeps now
// that the vault's actions stand in the window's toolbar band: the rows run
// from under the rail's own find field to the pane's bottom edge, with
// nothing standing below them to take a band off it.
//
// It is asserted off what the pane laid out rather than recomputed from
// the constants that placed it.
func TestPaneRowsRunToThePaneFoot(t *testing.T) {
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
			if g.rows.Empty() {
				t.Fatal("the pane laid out no rows")
			}
			if g.rows.Max.Y != paneH {
				t.Errorf("the rows end at y=%d, want the pane's own bottom edge %d", g.rows.Max.Y, paneH)
			}
		})
	}
}

// TestTreeIsOneRunInNameOrder pins the rail's run: a folder is one
// collection, so its entries stand in one name-ordered run with the folders
// and the notes interleaved, and no row carries a heading. A section parts
// collections an application keeps apart, never one collection sorted by
// kind, and this vault has one.
func TestTreeIsOneRunInNameOrder(t *testing.T) {
	// Names chosen so a folders-first order and a name order disagree at
	// both levels: alpha.md stands before the Design folder, and Aims.md
	// before the notes folder inside it.
	idx := treeIndex(
		"zebra.md",
		"alpha.md",
		"Design/Aims.md",
		"Design/notes/deep.md",
		"Mixed.md",
	)
	got := rowSigs(TreeRows(idx, map[string]bool{"Design": true}))
	want := []string{
		"0 note alpha.md",
		"0 dir-open Design",
		"1 note Design/Aims.md",
		"1 dir-closed Design/notes",
		"0 note Mixed.md",
		"0 note zebra.md",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// A folder and a note of one name leave the folder first, so the order
	// is total and a frame cannot draw the pair two ways.
	twice := rowSigs(TreeRows(treeIndex("Ideas/a.md", "Ideas.md"), nil))
	if len(twice) != 2 || twice[0] != "0 dir-closed Ideas" || twice[1] != "0 note Ideas.md" {
		t.Errorf("a folder and a note named Ideas stand %v, want the folder first", twice)
	}
}

// TestTreeIndentIsOnePerDepth holds the rail's one indent rule: a row's parts
// begin one indent further in per depth, the step is the same at every depth,
// and the find's flat answer puts every row back at depth 0 — so a name never
// shifts sideways for a reason the reader did not give.
func TestTreeIndentIsOnePerDepth(t *testing.T) {
	step := treeRowLead(1) - treeRowLead(0)
	for d := 1; d < 8; d++ {
		if got := treeRowLead(d) - treeRowLead(d-1); got != step {
			t.Errorf("depth %d stands %v past depth %d, where depth 1 stands %v past depth 0", d, got, d-1, step)
		}
	}
	if got, want := treeRowLead(0), float32(treeDiscloseColDp); got != want {
		t.Errorf("the vault's own level stands %v in on top of the sidebar's columns, want the one disclosure column %v: a disclosure drawn before the rail's first column would stand outside the pill", got, want)
	}

	m := treeModel()
	m.Folds = map[string]bool{"guide": true, "guide/deep": true}
	if rows := TreeRows(m.Index, m.Folds); len(rows) < 3 {
		t.Fatalf("the folder tree draws %d rows, too few to read a depth off", len(rows))
	}
	for _, r := range MatchRows(m.Index, "n") {
		if r.Depth != 0 {
			t.Errorf("the find answers %s at depth %d; a flat answer is one level, and a row that moved sideways under the find would move under the reader", r.Path, r.Depth)
		}
	}
}

// TestTreeSortsNamesAsTheFileBrowserDoes holds the rail's order against the
// one every name-ordered list in the library sorts by (theme/system/naming):
// a run of digits reads as the number it spells, so note 2 stands before note
// 10; case does not part a name from its neighbours; and where a folder and a
// note carry one name the folder stands first, which is this caller's tie
// rule and not the comparison's.
func TestTreeSortsNamesAsTheFileBrowserDoes(t *testing.T) {
	idx := treeIndex(
		"note 10.md", "note 2.md", "note 1.md",
		"Note 3.md", "NOTE 4.md",
		"Ideas/a.md", "Ideas.md",
	)
	got := rowSigs(TreeRows(idx, nil))
	want := []string{
		"0 dir-closed Ideas",
		"0 note Ideas.md",
		"0 note note 1.md",
		"0 note note 2.md",
		"0 note Note 3.md",
		"0 note NOTE 4.md",
		"0 note note 10.md",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
