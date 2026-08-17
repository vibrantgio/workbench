// outline.go builds the current note's heading outline and tracks where
// in it the reader is. Both are pure functions over the note's parsed
// blocks — the same blocks the document lays out and the same ones
// AnchorBlock walks for a followed link's heading path, so an outline
// entry and a wikilink that name the same heading land on the same place.
// The line scanner's headings are not used here for that reason: they
// answer "does this anchor exist", and only the parsed blocks can say
// which block the viewport must seat.

package main

import (
	"strings"

	"github.com/vibrantgio/markdown"
)

// outlineEntry is one heading of the current note: its position in the
// entry slice, the top-level block index the document seats at to reach
// it, its level for the indent, and its plain text.
type outlineEntry struct {
	Idx   int
	Block int
	Level int
	Title string
}

// noteOutline lists the note's top-level headings in document order. A
// note with no headings yields nothing, which is a state the panel shows
// rather than a case it hides: an outline that vanished would take the
// backlinks with it up the column.
//
// Only top-level blocks count, because only a top-level block index is
// something the document can be scrolled to — a heading nested inside a
// blockquote is part of the quotation, not part of the note's structure.
// An empty heading is dropped: it has nothing to put in a row.
func noteOutline(n *Note) []outlineEntry {
	if n == nil {
		return nil
	}
	var out []outlineEntry
	for i, b := range n.Blocks {
		h, ok := b.(*markdown.Heading)
		if !ok {
			continue
		}
		title := strings.TrimSpace(spanText(h.Spans))
		if title == "" {
			continue
		}
		out = append(out, outlineEntry{Idx: len(out), Block: i, Level: h.Level, Title: title})
	}
	return out
}

// outlineActive is the entry the reader is inside: the last heading at or
// above the block leading the viewport. It is -1 before the first
// heading, which is where a note that opens with prose or frontmatter
// starts, and -1 for a note with no headings at all.
//
// "At or above" is the whole rule. The reader is under a heading from the
// moment it reaches the top of the viewport until the next one does, so
// the marked entry changes exactly when a new section's heading crosses
// the leading edge — which is what a reader watching the column expects,
// and what makes the mark agree with what the top line of the page says.
func outlineActive(entries []outlineEntry, first int) int {
	active := -1
	for _, e := range entries {
		if e.Block > first {
			break
		}
		active = e.Idx
	}
	return active
}
