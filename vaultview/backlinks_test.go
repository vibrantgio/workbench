package main

import (
	"reflect"
	"testing"
)

// backlinkIndex is the fixture the reverse-edge tests share. It carries
// every case the rule has to separate: a link that resolves to the note
// under test, one that resolves elsewhere, one that refuses as ambiguous,
// one that names nothing, one aimed at an absent heading, a relative link,
// two links from the same note, and a same-file heading hop.
func backlinkIndex() *Index {
	return &Index{Root: "/v", Files: []FileScan{
		{Path: "Target.md", Headings: []Heading{{Level: 1, Title: "Target"}}, Links: []string{"#Target"}},
		{Path: "Other.md"},
		{Path: "x/Dup.md"},
		{Path: "y/Dup.md"},
		{Path: "notes/one.md", Links: []string{"Target"}},
		{Path: "notes/two.md", Links: []string{"Other"}},
		{Path: "notes/three.md", Links: []string{"Dup"}},
		{Path: "notes/four.md", Links: []string{"Target", "Target|the target"}},
		{Path: "notes/five.md", Links: []string{"Missing"}},
		{Path: "notes/six.md", Links: []string{"one", "Target#Nowhere"}},
	}}
}

// TestBacklinks table-tests the reverse edges: an edge is earned by
// resolution, never by basename string match.
func TestBacklinks(t *testing.T) {
	idx := backlinkIndex()
	cases := []struct {
		name    string
		idx     *Index
		current string
		want    []string
	}{
		{
			name: "nil index yields nothing",
			idx:  nil, current: "Target.md",
		},
		{
			name: "no current note yields nothing",
			idx:  idx,
		},
		{
			name: "citing notes in index order, once each however many links",
			idx:  idx, current: "Target.md",
			// one.md links it; four.md links it twice and counts once.
			// two.md resolves to Other, five.md names nothing, six.md aims
			// at a heading Target has not got, and Target's own #Target hop
			// is navigation inside the note, not an edge into it.
			want: []string{"notes/one.md", "notes/four.md"},
		},
		{
			name: "a link that resolves elsewhere counts only there",
			idx:  idx, current: "Other.md",
			want: []string{"notes/two.md"},
		},
		{
			name: "an ambiguous link counts for neither candidate",
			idx:  idx, current: "x/Dup.md",
		},
		{
			name: "an ambiguous link counts for neither candidate (second)",
			idx:  idx, current: "y/Dup.md",
		},
		{
			name: "a link relative to the citing note's folder counts",
			idx:  idx, current: "notes/one.md",
			want: []string{"notes/six.md"},
		},
		{
			name: "a note nothing cites has no backlinks",
			idx:  idx, current: "notes/four.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Backlinks(tc.idx, tc.current)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Backlinks(%q) = %v, want %v", tc.current, got, tc.want)
			}
		})
	}
}
