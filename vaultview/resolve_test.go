package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/obsidian"
)

// testIndex is a hand-built vault index exercising every resolution rule.
func testIndex() *Index {
	return &Index{
		Root: "/vault",
		Files: []FileScan{
			{Path: "x.md", Headings: []Heading{{Level: 1, Title: "X"}}},
			{Path: "f.md", Headings: []Heading{
				{Level: 1, Title: "F"},
				{Level: 2, Title: "A"},
				{Level: 3, Title: "B"},
				{Level: 2, Title: "Other"},
				{Level: 3, Title: "B"},
			}, BlockIDs: []string{"blk-1"}},
			{Path: "dir/rel.md", Headings: []Heading{{Level: 1, Title: "Rel"}}},
			{Path: "dir/inner.md"},
			{Path: "deep/only/unique.md"},
			{Path: "one/dup.md"},
			{Path: "two/dup.md"},
			{Path: "twice.md", Headings: []Heading{
				{Level: 2, Title: "Same"},
				{Level: 2, Title: "Same"},
			}},
		},
	}
}

func TestResolve(t *testing.T) {
	idx := testIndex()
	tests := []struct {
		name       string
		from       string
		body       string
		want       Resolved
		refuse     string   // substring of the refusal reason; empty for success
		candidates []string // expected candidate list on ambiguity
	}{
		{
			name: "as written relative to the linking note's directory",
			from: "dir/inner.md",
			body: "rel",
			want: Resolved{Path: "dir/rel.md"},
		},
		{
			name: "as written with an explicit extension",
			from: "dir/inner.md",
			body: "rel.md",
			want: Resolved{Path: "dir/rel.md"},
		},
		{
			name: "against the vault root with .md appended",
			from: "dir/inner.md",
			body: "f",
			want: Resolved{Path: "f.md"},
		},
		{
			name: "against the vault root with a path",
			from: "f.md",
			body: "dir/rel",
			want: Resolved{Path: "dir/rel.md"},
		},
		{
			name: "unique basename anywhere below the root",
			from: "x.md",
			body: "unique",
			want: Resolved{Path: "deep/only/unique.md"},
		},
		{
			name:       "ambiguous basename refuses with candidates",
			from:       "x.md",
			body:       "dup",
			refuse:     `"dup" matches 2 notes`,
			candidates: []string{"one/dup.md", "two/dup.md"},
		},
		{
			name:   "a missing note refuses",
			from:   "x.md",
			body:   "nowhere",
			refuse: `no note "nowhere" in this vault`,
		},
		{
			name: "alias is display-only",
			from: "x.md",
			body: "f|the alias",
			want: Resolved{Path: "f.md"},
		},
		{
			name: "heading path descends case-insensitively",
			from: "x.md",
			body: "f#a#b",
			want: Resolved{Path: "f.md", Headings: []string{"a", "b"}},
		},
		{
			name:   "a heading outside its ancestor's section refuses",
			from:   "x.md",
			body:   "f#A#Other",
			refuse: `no heading "Other"`,
		},
		{
			name:   "an ambiguous heading title refuses",
			from:   "x.md",
			body:   "twice#Same",
			refuse: `"Same" matches 2 sections`,
		},
		{
			name: "a duplicated title disambiguated by its ancestor resolves",
			from: "x.md",
			body: "f#Other#B",
			want: Resolved{Path: "f.md", Headings: []string{"Other", "B"}},
		},
		{
			name:   "an ambiguous title under one ancestor refuses",
			from:   "x.md",
			body:   "f#F#B",
			refuse: `"B" matches 2 sections`,
		},
		{
			name: "block ref resolves",
			from: "x.md",
			body: "f#^blk-1",
			want: Resolved{Path: "f.md", BlockID: "blk-1"},
		},
		{
			name:   "a missing block id refuses",
			from:   "x.md",
			body:   "f#^nope",
			refuse: `no block ^nope in "f"`,
		},
		{
			name: "same-file heading resolves within the linking note",
			from: "f.md",
			body: "#A",
			want: Resolved{Path: "f.md", Headings: []string{"A"}},
		},
		{
			name: "same-file block ref resolves within the linking note",
			from: "f.md",
			body: "#^blk-1",
			want: Resolved{Path: "f.md", BlockID: "blk-1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, rerr := Resolve(idx, tc.from, tc.body)
			if tc.refuse != "" {
				if rerr == nil {
					t.Fatalf("Resolve resolved %+v, want refusal containing %q", got, tc.refuse)
				}
				if !strings.Contains(rerr.Reason, tc.refuse) {
					t.Errorf("refusal = %q, want it to contain %q", rerr.Reason, tc.refuse)
				}
				if tc.candidates != nil && !reflect.DeepEqual(rerr.Candidates, tc.candidates) {
					t.Errorf("candidates = %v, want %v", rerr.Candidates, tc.candidates)
				}
				return
			}
			if rerr != nil {
				t.Fatalf("Resolve refused: %v", rerr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Resolve = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		body string
		want Ref
	}{
		{"Note", Ref{File: "Note"}},
		{"Note|alias", Ref{File: "Note"}},
		{"Folder/Note#A#B", Ref{File: "Folder/Note", Headings: []string{"A", "B"}}},
		{"Note#^id", Ref{File: "Note", BlockID: "id"}},
		{"#Heading", Ref{Headings: []string{"Heading"}}},
		{"#^id", Ref{BlockID: "id"}},
	}
	for _, tc := range tests {
		if got := ParseRef(tc.body); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseRef(%q) = %+v, want %+v", tc.body, got, tc.want)
		}
	}
}

// noteFromSource builds a Note the way LoadNote does, without the disk.
func noteFromSource(path, src string) *Note {
	_, body := obsidian.SplitFrontMatter([]byte(src))
	blocks, anchors := obsidian.BlockAnchors(obsidian.WikiSpans(markdown.Parse(body)))
	return &Note{Path: path, Blocks: blocks, Anchors: anchors}
}

// TestAnchorBlockLandsHeadingPath is the headless half of the exit
// criterion: resolving a heading-path link lands the viewport on the
// nested heading's own top-level block, computed from the parsed blocks.
func TestAnchorBlockLandsHeadingPath(t *testing.T) {
	n := noteFromSource("f.md", strings.Join([]string{
		"# F",
		"",
		"intro",
		"",
		"## A",
		"",
		"inside a",
		"",
		"### B",
		"",
		"target prose",
		"",
		"## Other",
		"",
		"### B",
		"",
	}, "\n"))
	at, ok := AnchorBlock(n, []string{"A", "B"}, "")
	if !ok {
		t.Fatal("AnchorBlock found no target for the heading path")
	}
	h, isHeading := n.Blocks[at].(*markdown.Heading)
	if !isHeading || h.Level != 3 || spanText(h.Spans) != "B" {
		t.Fatalf("block %d = %#v, want the level-3 heading B under A", at, n.Blocks[at])
	}
	// The duplicate B under Other must be a different, later block.
	if at2, ok2 := AnchorBlock(n, []string{"Other", "B"}, ""); !ok2 || at2 <= at {
		t.Errorf("Other#B landed at %d (ok=%v), want a block after %d", at2, ok2, at)
	}
}

func TestAnchorBlockLandsBlockID(t *testing.T) {
	n := noteFromSource("f.md", "# F\n\nfirst\n\nsecond paragraph ^blk-1\n")
	at, ok := AnchorBlock(n, nil, "blk-1")
	if !ok {
		t.Fatal("AnchorBlock found no target for the block id")
	}
	p, isPara := n.Blocks[at].(*markdown.Paragraph)
	if !isPara || spanText(p.Spans) != "second paragraph" {
		t.Fatalf("block %d = %#v, want the stripped second paragraph", at, n.Blocks[at])
	}
	if _, ok := AnchorBlock(n, nil, "missing"); ok {
		t.Error("a missing block id must not anchor")
	}
}
