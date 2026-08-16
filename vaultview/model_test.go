package main

import (
	"os"
	"path/filepath"
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
