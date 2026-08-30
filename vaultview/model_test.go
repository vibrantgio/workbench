package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	if string(n.Src) != src {
		t.Errorf("Src = %q, want the file bytes LoadNote read", n.Src)
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

// TestHistoryFollowBackForward walks the history: navigating x → f#A#B
// pushes a history entry anchored on B's block index; Back returns to x
// without re-anchoring (the cached document keeps its scroll); Forward
// returns to f; recorded anchors survive the round trip.
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

// TestAmbiguousLinkResolvesThroughChooser walks the chooser: the resolver
// refuses an ambiguous link with its candidates, the refusal raises the
// chooser carrying them, and choosing one navigates there — anchored where
// the refused link pointed.
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

// TestSwitchVaultReRootsAndFollowsTheStore walks a vault switch: the model
// returns to the picker seated at the open vault's parent; opening another
// vault rewrites the store and re-roots the tree on the new index.
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

// writeNote writes one note into a vault directory, creating folders as
// needed, and returns its absolute path.
func writeNote(t *testing.T, root, rel, src string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

// firstParagraph is the text of a note's first paragraph, the cheapest
// way to tell one revision of a file from another.
func firstParagraph(n *Note) string {
	for _, b := range n.Blocks {
		if p, ok := b.(*markdown.Paragraph); ok {
			return spanText(p.Spans)
		}
	}
	return ""
}

// TestNavigateServesUnchangedNoteFromCache: a note whose file has not
// moved lands straight from the cache — the same Note value, so the
// document behind it keeps its scroll position and link state.
func TestNavigateServesUnchangedNoteFromCache(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "x.md", "# X\n\nSee [[f]].\n")
	writeNote(t, root, "f.md", "# F\n\nfirst revision\n")
	x, err := LoadNote(root, "x.md")
	if err != nil {
		t.Fatal(err)
	}
	f, err := LoadNote(root, "f.md")
	if err != nil {
		t.Fatal(err)
	}

	model := Model{Screen: screenVault, Vault: root, CurAnchor: -1}
	model = cacheNote(model, x)
	model.Current = "x.md"
	model.History = []HistEntry{{Path: "x.md", Anchor: -1}}
	model = cacheNote(model, f)

	model, _ = Update(model, Navigate{Path: "f.md"})
	if model.Current != "f.md" {
		t.Fatalf("Current = %q, want the cached note to land on the same frame", model.Current)
	}
	if model.Notes["f.md"] != f {
		t.Error("an unchanged note was re-read; the cache must serve it")
	}
}

// TestNavigateRereadsChangedNote is the freshness walk: a note edited on
// disk after it was cached is not served from the cache — the landing
// waits for a fresh read, and the reload replaces the cached value so the
// viewport is rebuilt from the new blocks.
func TestNavigateRereadsChangedNote(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "x.md", "# X\n\nSee [[f]].\n")
	full := writeNote(t, root, "f.md", "# F\n\nfirst revision\n")
	x, err := LoadNote(root, "x.md")
	if err != nil {
		t.Fatal(err)
	}
	f, err := LoadNote(root, "f.md")
	if err != nil {
		t.Fatal(err)
	}

	model := Model{Screen: screenVault, Vault: root, CurAnchor: -1}
	model = cacheNote(model, x)
	model.Current = "x.md"
	model.History = []HistEntry{{Path: "x.md", Anchor: -1}}
	model = cacheNote(model, f)

	// The vault's owner edits the note in another application.
	writeNote(t, root, "f.md", "# F\n\nsecond revision, written elsewhere\n")
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(full, later, later); err != nil {
		t.Fatal(err)
	}

	next, cmd := Update(model, Navigate{Path: "f.md"})
	if next.Current != "x.md" {
		t.Fatalf("Current = %q; a changed note must not land before it is read again", next.Current)
	}
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("load command: %v", err)
	}
	loaded, ok := msg.(noteLoaded)
	if !ok {
		t.Fatalf("load command returned %T, want noteLoaded", msg)
	}
	if got := firstParagraph(loaded.note); got != "second revision, written elsewhere" {
		t.Errorf("re-read note reads %q, want the edited text", got)
	}

	next, _ = Update(next, loaded)
	if next.Current != "f.md" {
		t.Fatalf("after the re-read: Current = %q, want f.md", next.Current)
	}
	if next.Notes["f.md"] == f {
		t.Error("the cache still holds the stale note; the viewport would render the old text")
	}
}

// TestRescanRefreshesIndexKeepingPlace is the Rescan walk: the message
// puts a scan in flight, the result replaces the index and re-reads the
// note on screen, and nothing about where the reader was changes — the
// current note, its history and the tree's disclosure all survive.
func TestRescanRefreshesIndexKeepingPlace(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "x.md", "# X\n\nfirst revision\n")
	idx, err := ScanVault(root)
	if err != nil {
		t.Fatal(err)
	}
	x, err := LoadNote(root, "x.md")
	if err != nil {
		t.Fatal(err)
	}
	model := Model{Screen: screenVault, Vault: root, Index: idx, CurAnchor: -1}
	model = cacheNote(model, x)
	model.Current = "x.md"
	model.History = []HistEntry{{Path: "x.md", Anchor: -1}}

	// The vault grows a note and the open one is edited, both outside the
	// app — the structural change is what only a rescan can see.
	writeNote(t, root, "notes/new.md", "# New\n")
	writeNote(t, root, "x.md", "# X\n\nsecond revision\n")

	model, cmd := Update(model, Rescan{})
	if !model.Scanning {
		t.Error("Rescan did not put a scan in flight")
	}
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("rescan command: %v", err)
	}
	rescanned, ok := msg.(vaultRescanned)
	if !ok {
		t.Fatalf("rescan command returned %T, want vaultRescanned", msg)
	}

	model, _ = Update(model, rescanned)
	if model.Scanning {
		t.Error("the rescan result did not clear the in-flight flag")
	}
	if len(model.Index.Files) != 2 {
		t.Errorf("index carries %d files, want the added note included", len(model.Index.Files))
	}
	if model.Current != "x.md" || len(model.History) != 1 || model.Cursor != 0 {
		t.Errorf("rescan moved the reader: current %q history %v cursor %d", model.Current, model.History, model.Cursor)
	}
	if got := firstParagraph(model.CurrentNote()); got != "second revision" {
		t.Errorf("the note on screen reads %q, want the rescan's fresh read", got)
	}
	if model.Toasts.Len() != 1 {
		t.Errorf("toast queue length %d, want the rescan to report itself", model.Toasts.Len())
	}

	// A second rescan with nothing changed keeps the cached note, so the
	// reader's scroll position is not thrown away for no reason.
	held := model.CurrentNote()
	model, cmd = Update(model, Rescan{})
	msg, err = cmd.First()
	if err != nil {
		t.Fatalf("second rescan command: %v", err)
	}
	model, _ = Update(model, msg)
	if model.CurrentNote() != held {
		t.Error("a rescan that found no change replaced the note on screen")
	}

	// A rescan of a vault no longer open is ignored.
	stale := vaultRescanned{vault: filepath.Join(root, "elsewhere"), index: &Index{}}
	if after, _ := Update(model, stale); after.Index != model.Index {
		t.Error("a rescan result for another vault replaced the index")
	}
	// With no vault open there is nothing to rescan.
	if after, _ := Update(Model{}, Rescan{}); after.Scanning {
		t.Error("Rescan without a vault put a scan in flight")
	}
}

// TestRescanSummaryCounts pins the line a finished rescan reports, the
// only feedback a rescan that changed nothing can give.
func TestRescanSummaryCounts(t *testing.T) {
	cases := []struct {
		idx  *Index
		want string
	}{
		{nil, "Rescanned: 0 notes"},
		{&Index{}, "Rescanned: 0 notes"},
		{treeIndex("a.md"), "Rescanned: 1 note"},
		{treeIndex("a.md", "b.md"), "Rescanned: 2 notes"},
	}
	for _, tc := range cases {
		if got := rescanSummary(tc.idx); got != tc.want {
			t.Errorf("rescanSummary = %q, want %q", got, tc.want)
		}
	}
}

// TestSetFilterCarriesTheQuery: each keystroke above the tree reaches the
// model, and the model is all the filter is — no scan, no reload.
func TestSetFilterCarriesTheQuery(t *testing.T) {
	model := scannedModel(t)
	model.Index = treeIndex("a.md")
	before := model.Index

	model, cmd := Update(model, SetFilter{Text: "des"})
	if model.Filter != "des" {
		t.Errorf("Filter = %q, want the typed text", model.Filter)
	}
	if model.Index != before {
		t.Error("filtering re-scanned the vault; it must be a filter over the index")
	}
	if cmd.Observable == nil {
		t.Error("SetFilter returned no command at all")
	}

	model, _ = Update(model, SetFilter{Text: ""})
	if model.Filter != "" {
		t.Errorf("Filter = %q, want the cleared query", model.Filter)
	}
}

// TestNotePlacesTrail: the trail carries the in-vault path only — one place
// per folder and the note last, each addressed by its own path, and the
// vault is not a place: it names the window from the chrome row instead.
func TestNotePlacesTrail(t *testing.T) {
	model := Model{Screen: screenVault, Vault: "/home/rene/Second Brain", CurAnchor: -1}
	model = cacheNote(model, &Note{Path: "a/b/note.md", Title: "note"})
	model.Current = "a/b/note.md"

	got := notePlaces(model)
	want := []place{
		{label: "a", path: "a"},
		{label: "b", path: "a/b"},
		{label: "note", path: "a/b/note.md"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("notePlaces = %+v, want %+v", got, want)
	}

	// Before a note loads there is no path to show at all.
	empty := Model{Screen: screenVault, Vault: "/home/rene/Second Brain"}
	if c := notePlaces(empty); len(c) != 0 {
		t.Errorf("notePlaces without a note = %+v, want an empty trail", c)
	}

	// The vault's own name is the chrome row's, and it survives a note
	// at the vault root having no folder places of its own.
	if n := vaultName(model); n != "Second Brain" {
		t.Errorf("vaultName = %q, want the vault folder's name", n)
	}
	if n := vaultName(Model{}); n != "" {
		t.Errorf("vaultName with no vault open = %q, want empty", n)
	}
}

// TestToggleSidebar: the rail's visibility is window state — it flips on
// the message and survives a switch to another vault.
func TestToggleSidebar(t *testing.T) {
	model := Model{Screen: screenVault, Vault: "/home/rene/Second Brain"}
	model, _ = Update(model, ToggleSidebar{})
	if !model.SidebarHidden {
		t.Fatal("SidebarHidden = false after the first toggle, want the rail hidden")
	}
	switched, _ := Update(model, SwitchVault{})
	if !switched.SidebarHidden {
		t.Error("SidebarHidden = false after switching vaults, want the rail still hidden")
	}
	// And through the far side of the switch: opening a vault resets
	// everything that belongs to a vault, and the rail's visibility does
	// not belong to one.
	opened, _ := Update(switched, OpenVault{Path: "/home/rene/Other Vault"})
	if !opened.SidebarHidden {
		t.Error("SidebarHidden = false after opening another vault, want the rail still hidden")
	}
	navigated, _ := Update(opened, Navigate{Path: "note.md"})
	if !navigated.SidebarHidden {
		t.Error("SidebarHidden = false after navigating, want the rail still hidden")
	}
	model, _ = Update(model, ToggleSidebar{})
	if model.SidebarHidden {
		t.Error("SidebarHidden = true after the second toggle, want the rail shown")
	}
}

// TestRevealAndRootTree: the two disclosure messages move the tree — a
// folder crumb opens the whole way down to its folder, the chrome row's
// vault name closes everything.
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
