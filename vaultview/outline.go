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
// "At or above" is the whole rule: the reader is under a heading from the
// moment it reaches the top of the viewport until the next one does, so
// the marked entry changes exactly when a new section's heading crosses
// the leading edge.
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

// outlineChoice is an entry the reader picked, held against the mark the
// document would otherwise write.
//
// The last headings of a note cannot lead the viewport: the document stops
// at its own end, so the leading block stays under some earlier heading
// however far the reader is taken toward the one they picked, and a mark
// read from that block alone would make those entries unpickable. So the
// pick stands for exactly as long as the reader leaves the note where
// pressing it put them: move the note by any means and the mark is the
// document's again.
//
// The zero value is a reader who has picked nothing.
type outlineChoice struct {
	// entryPlusOne is the picked entry offset by one, so that nothing
	// picked is the zero value rather than the first entry.
	entryPlusOne int
	// doc is the document the pick was made in. A note replaced under a
	// pick — followed link, reload — is not a note standing still, even
	// when the block leading it happens to be the same one.
	doc *markdown.Document
	// first is the block the document came to rest on once it had carried
	// the move out, and seated says that has been read. Until then there is
	// nothing to compare a later frame against: the move takes effect on
	// the document's next layout, which has not happened yet.
	first  int
	seated bool
}

// take records the entry the reader picked and the document it was picked
// in, letting go of whatever was held before.
func (c *outlineChoice) take(doc *markdown.Document, idx int) {
	*c = outlineChoice{entryPlusOne: idx + 1, doc: doc}
}

// drop lets go of the pick, leaving the mark to the document.
func (c *outlineChoice) drop() { *c = outlineChoice{} }

// stands reports whether the reader's pick still holds, and which entry it
// marks when it does. first is the block leading the viewport of the
// document laid out this frame.
//
// It is the frame's one reading of the pick and it keeps what it reads: the
// first frame after a pick records where the document came to rest — as far
// toward the heading as the note goes, whether or not that put the heading
// at the top — and any later frame that finds the document somewhere else
// lets the pick go.
func (c *outlineChoice) stands(doc *markdown.Document, first int) (int, bool) {
	switch {
	case c.entryPlusOne == 0:
		return -1, false
	case doc == nil || doc != c.doc:
		c.drop()
		return -1, false
	case !c.seated:
		c.seated, c.first = true, first
	case first != c.first:
		c.drop()
		return -1, false
	}
	return c.entryPlusOne - 1, true
}
