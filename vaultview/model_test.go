package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/toast"
)

// TestLoadNoteWiresWikilinksAndAnchors covers the render-path wiring: a
// loaded note's blocks carry wikilink hyperlink spans (the grammar itself
// is the dialect package's business) and its block-id anchors are
// stripped into the map the viewport seats on.
func TestLoadNoteWiresWikilinksAndAnchors(t *testing.T) {
	root := t.TempDir()
	src := "---\ntitle: X\n---\nSee [[Other Note|friend]] here.\n\nAnchored paragraph. ^blk-1\n"
	if err := os.WriteFile(filepath.Join(root, "x.md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := LoadNote(root, "x.md")
	if err != nil {
		t.Fatalf("LoadNote: %v", err)
	}
	var linkURL, linkText string
	for _, b := range n.Blocks {
		p, ok := b.(*markdown.Paragraph)
		if !ok {
			continue
		}
		for _, s := range p.Spans {
			if strings.HasPrefix(s.URL, "wiki:") {
				linkURL, linkText = s.URL, s.Text
			}
		}
	}
	if linkURL != "wiki:Other Note|friend" || linkText != "friend" {
		t.Errorf("wikilink span = (%q, %q), want the aliased hyperlink span", linkText, linkURL)
	}
	if at, ok := n.Anchors["blk-1"]; !ok {
		t.Error("block id blk-1 missing from the anchors map")
	} else if p, isPara := n.Blocks[at].(*markdown.Paragraph); !isPara || spanText(p.Spans) != "Anchored paragraph." {
		t.Errorf("anchor blk-1 points at %#v, want the stripped paragraph", n.Blocks[at])
	}
	if n.FM.Present != true {
		t.Error("frontmatter not split off")
	}
}

// scannedModel builds a vault-screen model the way vaultScanned leaves
// it: note x cached and current, history seeded with the landing.
func scannedModel(t *testing.T) Model {
	t.Helper()
	model := Model{Screen: screenVault, Vault: "/v", CurAnchor: -1}
	x := noteFromSource("x.md", "# X\n\nSee [[f#A#B]].\n")
	model = cacheNote(model, x)
	model.Current = "x.md"
	model.NavSeq++
	model.History = []HistEntry{{Path: "x.md", Anchor: -1}}
	model.Cursor = 0
	return model
}

// fNote is the navigation target note: heading path A → B present.
func fNote() *Note {
	return noteFromSource("f.md", strings.Join([]string{
		"# F",
		"",
		"intro",
		"",
		"## A",
		"",
		"### B",
		"",
		"target",
	}, "\n"))
}

// TestHistoryFollowBackForward is the headless exit-criterion walk:
// navigating x → f#A#B pushes a history entry anchored on B's block
// index; Back returns to x without re-anchoring (the cached document
// keeps its scroll); Forward returns to f; recorded anchors survive the
// round trip.
func TestHistoryFollowBackForward(t *testing.T) {
	model := scannedModel(t)
	f := fNote()
	model = cacheNote(model, f)

	wantAnchor, ok := AnchorBlock(f, []string{"A", "B"}, "")
	if !ok {
		t.Fatal("AnchorBlock found no target for A#B")
	}

	seqBefore := model.NavSeq
	model, _ = Update(model, Navigate{Path: "f.md", Headings: []string{"A", "B"}})
	if model.Current != "f.md" || model.CurAnchor != wantAnchor {
		t.Fatalf("after Navigate: Current=%q CurAnchor=%d, want f.md at block %d", model.Current, model.CurAnchor, wantAnchor)
	}
	if model.NavSeq == seqBefore {
		t.Error("Navigate did not bump NavSeq; the viewport would not re-seat")
	}
	if len(model.History) != 2 || model.Cursor != 1 {
		t.Fatalf("history = %v cursor %d, want 2 entries, cursor 1", model.History, model.Cursor)
	}
	if e := model.History[1]; e.Path != "f.md" || e.Anchor != wantAnchor {
		t.Errorf("pushed entry = %+v, want f.md anchored at %d", e, wantAnchor)
	}

	model, _ = Update(model, GoBack{})
	if model.Current != "x.md" || model.Cursor != 0 {
		t.Fatalf("after Back: Current=%q cursor %d, want x.md, 0", model.Current, model.Cursor)
	}
	if model.CurAnchor != -1 {
		t.Errorf("Back set CurAnchor %d; it must not re-seat the cached document", model.CurAnchor)
	}

	model, _ = Update(model, GoForward{})
	if model.Current != "f.md" || model.Cursor != 1 {
		t.Fatalf("after Forward: Current=%q cursor %d, want f.md, 1", model.Current, model.Cursor)
	}
	// The recorded anchors survive the round trip untouched.
	if model.History[0].Anchor != -1 || model.History[1].Anchor != wantAnchor {
		t.Errorf("history anchors = %d,%d, want -1,%d", model.History[0].Anchor, model.History[1].Anchor, wantAnchor)
	}

	// At the newest entry Forward is a no-op; at the oldest Back is.
	if m2, _ := Update(model, GoForward{}); m2.Cursor != 1 {
		t.Error("Forward moved past the newest entry")
	}
	model, _ = Update(model, GoBack{})
	if m2, _ := Update(model, GoBack{}); m2.Cursor != 0 {
		t.Error("Back moved past the oldest entry")
	}
}

// TestNavigateTruncatesForwardTail: navigating from the middle of the
// stack drops the forward tail, the browser rule.
func TestNavigateTruncatesForwardTail(t *testing.T) {
	model := scannedModel(t)
	model = cacheNote(model, fNote())
	model = cacheNote(model, noteFromSource("g.md", "# G\n"))

	model, _ = Update(model, Navigate{Path: "f.md"})
	model, _ = Update(model, GoBack{})
	model, _ = Update(model, Navigate{Path: "g.md"})

	if len(model.History) != 2 || model.Cursor != 1 {
		t.Fatalf("history = %v cursor %d, want x,g with cursor 1", model.History, model.Cursor)
	}
	if model.History[0].Path != "x.md" || model.History[1].Path != "g.md" {
		t.Errorf("history paths = %q,%q, want x.md,g.md", model.History[0].Path, model.History[1].Path)
	}
}

// TestNavigateUncachedLoadsThenLands: an uncached target goes through the
// load command; the noteLoaded message lands it and pushes history.
func TestNavigateUncachedLoadsThenLands(t *testing.T) {
	model := scannedModel(t)
	before := model
	model, _ = Update(model, Navigate{Path: "f.md"})
	if model.Current != before.Current || len(model.History) != 1 {
		t.Fatal("an uncached Navigate must not land before its note is loaded")
	}
	f := fNote()
	model, _ = Update(model, noteLoaded{vault: "/v", nav: Navigate{Path: "f.md"}, note: f})
	if model.Current != "f.md" || len(model.History) != 2 {
		t.Fatalf("after noteLoaded: Current=%q history %v", model.Current, model.History)
	}
	if model.Notes["f.md"] == nil {
		t.Error("loaded note not cached")
	}
}

// TestNoteLoadErrorRaisesToast: a failed load surfaces as a toast in the
// model's queue, not a navigation.
func TestNoteLoadErrorRaisesToast(t *testing.T) {
	model := scannedModel(t)
	model, _ = Update(model, noteLoaded{vault: "/v", nav: Navigate{Path: "f.md"}, err: "read failed"})
	if model.Current != "x.md" || len(model.History) != 1 {
		t.Error("a failed load must not navigate")
	}
	if model.Toasts.Len() != 1 {
		t.Fatalf("toast queue length %d, want 1", model.Toasts.Len())
	}
	id := model.Toasts.Items()[0].ID
	model, _ = Update(model, toast.Expired{ID: id})
	if model.Toasts.Len() != 0 {
		t.Error("Expired did not retire the toast")
	}
}

// TestToastRequestedQueues: a toast request lands in the model's queue —
// the path a refused resolution's toast.Notify takes through Update.
func TestToastRequestedQueues(t *testing.T) {
	model := scannedModel(t)
	model, _ = Update(model, toast.Request(toast.Warning, `no note "nope" in this vault`))
	if model.Toasts.Len() != 1 {
		t.Fatalf("toast queue length %d, want 1", model.Toasts.Len())
	}
}

// TestAmbiguousLinkResolvesThroughChooser is the headless exit-criterion
// walk for the chooser: the resolver refuses an ambiguous link with its
// candidates, the refusal raises the chooser carrying them, and choosing
// one navigates there — anchored where the refused link pointed.
func TestAmbiguousLinkResolvesThroughChooser(t *testing.T) {
	idx := backlinkIndex()
	idx.Files[2].Headings = []Heading{{Level: 2, Title: "Deep"}} // x/Dup.md
	model := scannedModel(t)
	model.Index = idx
	model.Current = "notes/three.md"

	// The refusal a click on [[Dup#Deep]] produces, with the candidate
	// list the chooser is built from.
	_, rerr := Resolve(idx, "notes/three.md", "Dup#Deep")
	if rerr == nil {
		t.Fatal("an ambiguous file part must refuse")
	}
	want := []string{"x/Dup.md", "y/Dup.md"}
	if !reflect.DeepEqual(rerr.Candidates, want) {
		t.Fatalf("candidates = %v, want %v", rerr.Candidates, want)
	}

	model, _ = Update(model, OpenChooser{Body: "Dup#Deep", Candidates: rerr.Candidates})
	if !model.ChooserOpen() {
		t.Fatal("the refusal did not raise the chooser")
	}
	if model.ChooserBody != "Dup#Deep" || !reflect.DeepEqual(model.ChooserCandidates, want) {
		t.Fatalf("chooser state = %q %v, want the refused body and its candidates", model.ChooserBody, model.ChooserCandidates)
	}

	chosen := noteFromSource("x/Dup.md", "# Dup\n\nintro\n\n## Deep\n\ntarget\n")
	model = cacheNote(model, chosen)
	wantAnchor, ok := AnchorBlock(chosen, []string{"Deep"}, "")
	if !ok {
		t.Fatal("AnchorBlock found no target for the chosen note's heading")
	}

	model, _ = Update(model, ChooseCandidate{Path: "x/Dup.md"})
	if model.ChooserOpen() {
		t.Error("choosing left the chooser open")
	}
	if model.Current != "x/Dup.md" || model.CurAnchor != wantAnchor {
		t.Fatalf("after choosing: Current=%q CurAnchor=%d, want x/Dup.md at block %d", model.Current, model.CurAnchor, wantAnchor)
	}
	if len(model.History) != 2 || model.Cursor != 1 {
		t.Errorf("history = %v cursor %d, want the choice pushed", model.History, model.Cursor)
	}
}

// TestChooserDismissal: closing the chooser navigates nowhere, and
// opening a vault clears any chooser the previous vault left up.
func TestChooserDismissal(t *testing.T) {
	model := scannedModel(t)
	model, _ = Update(model, OpenChooser{Body: "Dup", Candidates: []string{"x/Dup.md", "y/Dup.md"}})
	model, _ = Update(model, CloseChooser{})
	if model.ChooserOpen() {
		t.Error("CloseChooser left the chooser open")
	}
	if model.Current != "x.md" || len(model.History) != 1 {
		t.Error("dismissing the chooser must not navigate")
	}

	model, _ = Update(model, OpenChooser{Body: "Dup", Candidates: []string{"x/Dup.md", "y/Dup.md"}})
	model, _ = Update(model, OpenVault{Path: "/w"})
	if model.ChooserOpen() {
		t.Error("opening a vault left the previous vault's chooser up")
	}
}

// TestSwitchVaultReRootsAndFollowsTheStore is the headless exit-criterion
// walk for switching vaults: the model returns to the picker seated at the
// open vault's parent; opening another vault rewrites the store and
// re-roots the tree on the new index.
func TestSwitchVaultReRootsAndFollowsTheStore(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	oldVault := filepath.Join(root, "old")
	newVault := filepath.Join(root, "new")
	if err := os.MkdirAll(filepath.Join(newVault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newVault, "sub", "n.md"), []byte("# N\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := scannedModel(t)
	model.Vault = oldVault
	model.Index = treeIndex("x.md", "old-folder/y.md")
	model.Folds = map[string]bool{"old-folder": true}

	model, _ = Update(model, SwitchVault{})
	if model.Screen != screenPicker {
		t.Fatal("SwitchVault did not return to the picker")
	}
	if model.PickerDir != root {
		t.Errorf("picker seated at %q, want the open vault's parent %q", model.PickerDir, root)
	}

	model, cmd := Update(model, OpenVault{Path: newVault})
	if model.Screen != screenVault || model.Vault != newVault {
		t.Fatalf("OpenVault left screen=%v vault=%q", model.Screen, model.Vault)
	}
	if model.Index != nil || model.Folds != nil || model.Current != "" || len(model.History) != 0 {
		t.Error("OpenVault did not clear the previous vault's state")
	}

	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("open command: %v", err)
	}
	scanned, ok := msg.(vaultScanned)
	if !ok {
		t.Fatalf("open command returned %T, want vaultScanned", msg)
	}
	if got := LoadStoredVault(); got != newVault {
		t.Errorf("stored vault = %q, want %q — the store did not follow the switch", got, newVault)
	}

	model, _ = Update(model, scanned)
	if model.Current != "sub/n.md" {
		t.Fatalf("landed on %q, want the new vault's first note", model.Current)
	}
	got := rowSigs(TreeRows(model.Index, model.Folds))
	want := []string{"0 dir-open sub", "1 note sub/n.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tree rows = %v, want %v — the tree did not re-root on the new vault", got, want)
	}
}

// TestNoteCrumbsTrail: the trail grows the folder path, the vault crumb
// roots the tree, each folder crumb reveals its own folder, and the note
// title is where you already are.
func TestNoteCrumbsTrail(t *testing.T) {
	model := Model{Screen: screenVault, Vault: "/home/rene/Second Brain", CurAnchor: -1}
	model = cacheNote(model, &Note{Path: "a/b/note.md", Title: "note"})
	model.Current = "a/b/note.md"

	got := noteCrumbs(model)
	want := []crumb{
		{label: "Second Brain", msg: RootTree{}},
		{label: "a", msg: RevealFolder{Dir: "a"}},
		{label: "b", msg: RevealFolder{Dir: "a/b"}},
		{label: "note"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("noteCrumbs = %+v, want %+v", got, want)
	}

	// Before a note loads the trail is the vault crumb alone.
	empty := Model{Screen: screenVault, Vault: "/home/rene/Second Brain"}
	if c := noteCrumbs(empty); len(c) != 1 || c[0].label != "Second Brain" {
		t.Errorf("noteCrumbs without a note = %+v, want the vault crumb alone", c)
	}
}

// TestRevealAndRootTree: the crumb messages move the tree's disclosure —
// a folder crumb opens the whole way down to its folder, the vault crumb
// closes everything.
func TestRevealAndRootTree(t *testing.T) {
	model := Model{Screen: screenVault, Index: treeIndex("a/b/note.md", "top.md")}

	model, _ = Update(model, RevealFolder{Dir: "a/b"})
	got := rowSigs(TreeRows(model.Index, model.Folds))
	want := []string{"0 dir-open a", "1 dir-open a/b", "2 note a/b/note.md", "0 note top.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after RevealFolder: %v, want %v", got, want)
	}

	model, _ = Update(model, RootTree{})
	got = rowSigs(TreeRows(model.Index, model.Folds))
	want = []string{"0 dir-closed a", "0 note top.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after RootTree: %v, want %v", got, want)
	}
}
