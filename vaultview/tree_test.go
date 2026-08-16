package main

import (
	"fmt"
	"strings"
	"testing"
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

// TestTreeActiveTracksNavigation is the headless half of the exit
// criterion: as the model moves through Navigate and GoBack/GoForward,
// the current note's row stays visible in the flattened tree — landing
// inside a closed folder opens its ancestors — and the row the view
// would mark active (Path == Current) is always exactly the current one.
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
