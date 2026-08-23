package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// guideFixture reads the test guide document. Tests never load the
// checkout's llms.txt and never the network — the fixture is the one
// document they parse.
func guideFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "guide_fixture.md"))
	if err != nil {
		t.Fatalf("read guide fixture: %v", err)
	}
	return b
}

// TestGuideOutlineFixture pins the outline extraction: the lone # is
// skipped, each ## is a top-level entry in document order, ### headings
// are its children, a #### never appears, and a childless ## still has
// its entry. Every entry's Block must point at the heading block wearing
// that entry's title, which is what ScrollToBlock seats the reader at.
func TestGuideOutlineFixture(t *testing.T) {
	blocks := markdown.Parse(guideFixture(t))
	entries := guideOutline(blocks)

	want := []struct {
		title    string
		children []string
	}{
		{"First section", []string{"First child", "Second child"}},
		{"Second section, childless", nil},
		{"Third section", []string{"Lone child"}},
	}
	if len(entries) != len(want) {
		t.Fatalf("outline has %d top-level entries, want %d: %+v", len(entries), len(want), entries)
	}
	checkEntry := func(e outlineEntry, title string, level int) {
		t.Helper()
		if e.Title != title {
			t.Errorf("entry title = %q, want %q", e.Title, title)
		}
		h, ok := blocks[e.Block].(*markdown.Heading)
		if !ok {
			t.Fatalf("entry %q Block %d is %T, want *markdown.Heading", title, e.Block, blocks[e.Block])
		}
		if h.Level != level {
			t.Errorf("entry %q points at a level-%d heading, want %d", title, h.Level, level)
		}
		if got := headingText(h); got != title {
			t.Errorf("entry %q Block %d carries heading %q; the index is off", title, e.Block, got)
		}
	}
	for i, w := range want {
		checkEntry(entries[i], w.title, 2)
		if len(entries[i].Children) != len(w.children) {
			t.Fatalf("entry %q has %d children, want %d", w.title, len(entries[i].Children), len(w.children))
		}
		for j, c := range w.children {
			checkEntry(entries[i].Children[j], c, 3)
		}
	}
}

// TestGuideOutlineDropsOrphanChild pins the degenerate shape: a ###
// before any ## has no row to disclose under and is dropped rather than
// crashing or becoming a root.
func TestGuideOutlineDropsOrphanChild(t *testing.T) {
	src := []byte("# Title\n\n### Orphan\n\ntext\n\n## Real\n\ntext\n")
	entries := guideOutline(markdown.Parse(src))
	if len(entries) != 1 || entries[0].Title != "Real" {
		t.Fatalf("outline = %+v, want exactly the one ## entry %q", entries, "Real")
	}
	if len(entries[0].Children) != 0 {
		t.Errorf("the orphan ### was adopted: %+v", entries[0].Children)
	}
}

// TestOutlineRowsFlattening pins the tree's visible-row policy: every ##
// appears whether or not it is open, children appear only under an open
// ##, and the disclosure flag is set only on a ## that has something to
// disclose.
func TestOutlineRowsFlattening(t *testing.T) {
	entries := guideOutline(markdown.Parse(guideFixture(t)))

	closed := outlineRows(entries, nil)
	if len(closed) != 3 {
		t.Fatalf("all closed: %d rows, want the 3 ## rows", len(closed))
	}
	for _, r := range closed {
		if r.Child {
			t.Errorf("closed tree shows child row %q", r.Title)
		}
		if r.Open {
			t.Errorf("closed tree marks %q open", r.Title)
		}
	}
	if !closed[0].HasChildren || closed[1].HasChildren || !closed[2].HasChildren {
		t.Errorf("HasChildren flags wrong: %+v", closed)
	}

	open := outlineRows(entries, map[int]bool{0: true, 1: true})
	wantTitles := []string{"First section", "First child", "Second child", "Second section, childless", "Third section"}
	if len(open) != len(wantTitles) {
		t.Fatalf("first two open: %d rows, want %d (%+v)", len(open), len(wantTitles), open)
	}
	for i, w := range wantTitles {
		if open[i].Title != w {
			t.Errorf("row %d = %q, want %q", i, open[i].Title, w)
		}
	}
	if !open[0].Open {
		t.Error("the disclosed first ## should carry Open")
	}
	if open[3].Open {
		t.Error("the childless ## has nothing to disclose and must not read as open")
	}
	if !open[1].Child || !open[2].Child {
		t.Error("the ### rows should be marked Child")
	}
}

// TestLoadGuidePrefersCheckoutFile pins the loading order's first leg: a
// readable file on the candidate paths wins, and the network leg is never
// consulted — the fetch stub fails the test if called.
func TestLoadGuidePrefersCheckoutFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "llms.txt")
	if err := os.WriteFile(p, []byte("# From the checkout\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadGuideFrom([]string{filepath.Join(dir, "missing.txt"), p}, func() ([]byte, error) {
		t.Fatal("fetch called although a checkout file exists")
		return nil, nil
	})
	if string(got) != "# From the checkout\n" {
		t.Errorf("loadGuideFrom = %q, want the checkout file", got)
	}
}

// TestLoadGuideFallsBackToFetch pins the second leg: with no file on any
// candidate path the fetch answers, and its bytes are what the app keeps
// in memory.
func TestLoadGuideFallsBackToFetch(t *testing.T) {
	got := loadGuideFrom([]string{filepath.Join(t.TempDir(), "missing.txt")}, func() ([]byte, error) {
		return []byte("# Fetched\n"), nil
	})
	if string(got) != "# Fetched\n" {
		t.Errorf("loadGuideFrom = %q, want the fetched bytes", got)
	}
}

// TestLoadGuideNoticeStillADocument pins the last resort: no file, no
// network — the in-memory notice must itself parse into a document with
// an outline, so the app never opens on a blank pane.
func TestLoadGuideNoticeStillADocument(t *testing.T) {
	got := loadGuideFrom([]string{filepath.Join(t.TempDir(), "missing.txt")}, nil)
	entries := guideOutline(markdown.Parse(got))
	if len(entries) == 0 {
		t.Error("the unavailable notice has no ## outline; the tree would be empty")
	}
}

// TestOutlineClickScrollsTheDocument is the click-to-scroll pin. It lays
// out the real outline tree over the fixture with the first ## open,
// drives a pointer press/release through Gio's input router onto the
// "Second child" row, and then lays the document out once: the document's
// resolved position must lead with exactly that heading's block. This is
// the whole seam an outline click crosses — clickable → scrollTo →
// Document.ScrollToBlock → the next layout seating the block.
func TestOutlineClickScrollsTheDocument(t *testing.T) {
	blocks := markdown.Parse(guideFixture(t))
	doc := markdown.NewDocument(blocks)
	entries := guideOutline(blocks)
	v := newOutlineView(entries, doc.ScrollToBlock)

	st := outlineState{open: map[int]bool{0: true}, selected: -1}
	typ := tokens.DefaultTypography
	tok := themeTokens{col: tokens.DefaultLight, typ: typ, shaper: typ.DeterministicShaper()}
	tree := func(gtx layout.Context) layout.Dimensions {
		return v.layout(gtx, st, tok)
	}

	// Visible rows with the first ## open: First section, First child,
	// Second child, the childless ##, Third section. Row 2 is the target.
	target := entries[0].Children[1] // "Second child"
	rowH := float32(docsOutlineRowHDp)
	hit := f32.Pt(float32(docsOutlineWidthDp)/2, docsOutlineTopPadDp+2*rowH+rowH/2)

	driveWidget(tree, image.Pt(docsOutlineWidthDp, 400),
		pointer.Event{Kind: pointer.Press, Position: hit, Source: pointer.Touch},
		pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Touch},
	)

	// One document layout resolves the recorded scroll.
	style := docsMarkdownStyle(tokens.DefaultLight, typ)
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(600, 400)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	doc.Layout(gtx, tok.shaper, style)

	if p := doc.Position(); p.First != target.Block || p.Offset != 0 {
		t.Fatalf("after clicking %q: document position = %+v, want First %d Offset 0",
			target.Title, p, target.Block)
	}
}

// driveWidget frames the widget through an input router, queues the
// events, and frames again so tags register, events deliver, and the
// resulting state draws — the same cadence the markdown package's own
// pointer tests use.
func driveWidget(w layout.Widget, size image.Point, evs ...event.Event) {
	r := new(gioinput.Router)
	var ops op.Ops
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         &ops,
			Source:      r.Source(),
		}
		w(gtx)
		r.Frame(&ops)
	}
	frame()
	frame()
	r.Queue(evs...)
	frame()
	frame()
}

// TestDocsTabGolden records or diffs the whole Docs tab — outline tree in
// the leading column, the one document filling the rest — in light and
// dark, with the first ## open so the children show and its heading
// selected so the selection pill is pinned too. The fixture document
// keeps the pixels stable: the real llms.txt is prose that moves with
// every guide edit, and a golden must only move when the composition
// does.
func TestDocsTabGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	source := guideFixture(t)
	first := guideOutline(markdown.Parse(source))[0]
	st := outlineState{open: map[int]bool{0: true}, selected: first.Block}

	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderDocsTab(shaper, source, st, tc.colors, tokens.DefaultTypography)
			golden.Render(t, "docs-tab-"+tc.name, image.Pt(1180, 760), scene(w, tc.bg))
		})
	}
}

// TestDocsTabLightDarkDiffer confirms swapping the colour token set moves
// the rendered Docs tab's pixels.
func TestDocsTabLightDarkDiffer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	source := guideFixture(t)
	st := outlineState{open: map[int]bool{0: true}, selected: -1}
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	size := image.Pt(1180, 760)

	a := golden.Capture(t, size, scene(renderDocsTab(shaper, source, st, tokens.DefaultLight, tokens.DefaultTypography), bg))
	b := golden.Capture(t, size, scene(renderDocsTab(shaper, source, st, tokens.DefaultDark, tokens.DefaultTypography), bg))
	if golden.PixelDiff(a, b) == 0 {
		t.Error("light and dark Docs tab render identically")
	}
}

// TestRealGuideOutlineShape is the checkout-only smoke over the real
// document: when ../llms.txt is present (it is, in a checkout), its
// outline must come from the file's actual headings — a # that is not a
// root, a working set of ## rows, and ### children under at least some of
// them. No fixed list of titles is asserted: the tree follows the file.
// Outside a checkout the file is absent and the test skips; it never
// fetches.
func TestRealGuideOutlineShape(t *testing.T) {
	src, err := os.ReadFile("../llms.txt")
	if err != nil {
		t.Skipf("no checkout guide at ../llms.txt: %v", err)
	}
	blocks := markdown.Parse(src)
	if h, ok := blocks[0].(*markdown.Heading); !ok || h.Level != 1 {
		t.Fatalf("the guide should open with its lone # title, got %T", blocks[0])
	}
	entries := guideOutline(blocks)
	if len(entries) < 10 {
		t.Fatalf("the guide's outline has %d ## sections; expected a real working set", len(entries))
	}
	withChildren := 0
	for _, e := range entries {
		if e.Title == "" {
			t.Errorf("entry at block %d has an empty title", e.Block)
		}
		if len(e.Children) > 0 {
			withChildren++
		}
	}
	if withChildren == 0 {
		t.Error("no ## section carries ### children; AF1.1 put them there")
	}
}
