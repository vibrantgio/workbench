package main

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/markdown"
)

// outlineSource is the fixture the outline tests share: a note whose
// headings nest three deep, with prose before the first one, an empty
// heading that has nothing to put in a row, and a heading inside a
// blockquote that belongs to the quotation rather than to the note.
const outlineSource = `A paragraph before any heading at all.

# Reading list

Some prose.

## Open questions

> # A quoted heading
>
> Quoted prose.

### Deeper

More prose.

##

## A sample

Closing prose.
`

// TestNoteOutlineListsTopLevelHeadings pins what the outline is made of:
// the note's own top-level headings, in document order, at the block
// index each one occupies.
func TestNoteOutlineListsTopLevelHeadings(t *testing.T) {
	n := noteFromSource("guide/Outline.md", outlineSource)
	got := noteOutline(n)

	var titles []string
	for _, e := range got {
		titles = append(titles, fmt.Sprintf("%d:%s", e.Level, e.Title))
	}
	want := []string{"1:Reading list", "2:Open questions", "3:Deeper", "2:A sample"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Fatalf("outline = %v, want %v", titles, want)
	}
	for i, e := range got {
		if e.Idx != i {
			t.Errorf("entry %d carries Idx %d", i, e.Idx)
		}
		h, ok := n.Blocks[e.Block].(*markdown.Heading)
		if !ok {
			t.Fatalf("entry %q names block %d, which is not a heading", e.Title, e.Block)
		}
		if got := spanText(h.Spans); got != e.Title {
			t.Errorf("entry %q names block %d, whose heading is %q", e.Title, e.Block, got)
		}
	}
}

// TestOutlineEntriesLandWhereALinkWould is the claim the aside rests on:
// an outline entry and a wikilink naming the same heading take the reader
// to the same block. Two heading models would drift; there is one.
func TestOutlineEntriesLandWhereALinkWould(t *testing.T) {
	n := noteFromSource("guide/Outline.md", outlineSource)
	for _, e := range noteOutline(n) {
		at, ok := AnchorBlock(n, []string{e.Title}, "")
		if !ok {
			t.Errorf("no anchor for heading %q, which the outline lists", e.Title)
			continue
		}
		if at != e.Block {
			t.Errorf("heading %q: the outline seats block %d, a link seats block %d", e.Title, e.Block, at)
		}
	}
}

// TestNoteOutlineOfANoteWithNoHeadings is the other half of the exit
// condition: nothing to list, and nothing invented to list.
func TestNoteOutlineOfANoteWithNoHeadings(t *testing.T) {
	n := noteFromSource("Sources.md", "Just prose.\n\n- and a list\n")
	if got := noteOutline(n); len(got) != 0 {
		t.Fatalf("outline of a heading-less note = %v, want none", got)
	}
	if got := noteOutline(nil); got != nil {
		t.Fatalf("outline of no note at all = %v, want none", got)
	}
	if got := outlineActive(nil, 0); got != -1 {
		t.Fatalf("active entry of an empty outline = %d, want -1", got)
	}
}

// TestOutlineActiveTracksTheLeadingBlock walks the mark down the note the
// way scrolling does: it is the last heading at or above the block
// leading the viewport, it is nothing at all while the reader is in the
// prose before the first heading, and it stays on the last heading for
// everything under it.
func TestOutlineActiveTracksTheLeadingBlock(t *testing.T) {
	n := noteFromSource("guide/Outline.md", outlineSource)
	entries := noteOutline(n)
	if len(entries) != 4 {
		t.Fatalf("fixture outline has %d entries, want 4", len(entries))
	}
	for first := 0; first < len(n.Blocks); first++ {
		want := -1
		for _, e := range entries {
			if e.Block <= first {
				want = e.Idx
			}
		}
		if got := outlineActive(entries, first); got != want {
			t.Errorf("leading block %d: active entry %d, want %d", first, got, want)
		}
	}
	// Before the first heading there is no mark, and past the last block
	// the mark is the last heading.
	if got := outlineActive(entries, 0); got != -1 {
		t.Errorf("the prose before the first heading marked entry %d, want none", got)
	}
	if got := outlineActive(entries, len(n.Blocks)+10); got != len(entries)-1 {
		t.Errorf("past the end the mark is entry %d, want the last", got)
	}
}

// ---- the column as laid out --------------------------------------------

// asidePad lays the note column's document and the aside out together, in
// that order, exactly as the frame does — so the aside reads the position
// the document just resolved, which is the whole mechanism of the mark.
type asidePad struct {
	m     Model
	tok   themeTokens
	v     *asideView
	cur   *docCursor
	doc   *markdown.Document
	style markdown.Style
	r     input.Router
	ops   op.Ops
	docSz image.Point
	colSz image.Point
}

func newAsidePad(t *testing.T, m Model, colH int) *asidePad {
	t.Helper()
	tok := goldenTokens()
	n := m.CurrentNote()
	if n == nil {
		t.Fatal("the model has no current note")
	}
	p := &asidePad{
		m:     m,
		tok:   tok,
		cur:   &docCursor{},
		doc:   markdown.NewDocument(n.Blocks),
		style: markdown.FromTokens(tok.col, tok.typ),
		docSz: image.Pt(noteCanvasW, 400),
		colSz: image.Pt(frameAsideDp, colH),
	}
	p.v = newAsideView(p.cur)
	p.cur.show(p.doc)
	return p
}

func (p *asidePad) frame() {
	p.ops.Reset()
	base := layout.Context{
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:    &p.ops,
		Source: p.r.Source(),
	}
	dgtx := base
	dgtx.Constraints = layout.Exact(p.docSz)
	p.doc.Layout(dgtx, p.tok.shaper, p.style)

	agtx := base
	agtx.Constraints = layout.Exact(p.colSz)
	p.v.layout(agtx, p.m, p.tok)
	p.r.Frame(&p.ops)
}

// gtx is a context for driving the column's own affordances outside a
// frame, which is how the tests reach what a synthetic click cannot.
func (p *asidePad) gtx() layout.Context {
	return layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(p.colSz),
		Ops:         &p.ops,
		Source:      p.r.Source(),
	}
}

// outlineModel is a long, heavily sectioned note made current, so the
// outline has more entries than its pane can show and the document has
// more blocks than its viewport can.
func outlineModel() Model {
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Long note.md", longNoteSource()))
	m.Current = "guide/Long note.md"
	m.CurAnchor = -1
	return m
}

// TestTheMarkFollowsTheDocument is the tracking condition as a
// transition: the document is moved by the reading keys' own machinery,
// and the outline's mark is on the section the reader is now inside —
// without anything telling the column that the document moved.
func TestTheMarkFollowsTheDocument(t *testing.T) {
	p := newAsidePad(t, outlineModel(), 700)
	p.frame()

	entries := noteOutline(p.m.CurrentNote())
	if len(entries) < 20 {
		t.Fatalf("fixture outline has %d entries, want a long one", len(entries))
	}
	if got := p.v.outlineList.Selected(); got != 0 {
		t.Fatalf("at the top of the note the mark is entry %d, want the first heading", got)
	}

	// Down the note a page at a time, and back up: the mark is whatever
	// the document's own leading block says it is, every time.
	for i, move := range []func(){p.doc.PageDown, p.doc.PageDown, p.doc.PageDown, p.doc.PageUp, p.doc.ScrollToEnd, p.doc.ScrollToStart} {
		move()
		p.frame()
		want := outlineActive(entries, p.doc.Position().First)
		if got := p.v.outlineList.Selected(); got != want {
			t.Errorf("move %d: the mark is entry %d, want %d (leading block %d)",
				i, got, want, p.doc.Position().First)
		}
	}
	if got := p.v.outlineList.Selected(); got != 0 {
		t.Errorf("back at the top the mark is entry %d, want the first heading", got)
	}
}

// TestChoosingAnEntryMovesTheDocument is the other half of D4: an entry
// takes the reader to its heading in the note they already have open. The
// document is the same object afterwards — a reload would lose the
// viewport, the interaction state and the history entry alike — and the
// heading it names leads the viewport.
func TestChoosingAnEntryMovesTheDocument(t *testing.T) {
	p := newAsidePad(t, outlineModel(), 700)
	p.frame()
	before := p.cur.document()
	entries := noteOutline(p.m.CurrentNote())

	for _, i := range []int{12, 3, 25, 1} {
		p.v.seek(p.gtx(), entries[i])
		p.frame()
		if got := p.doc.Position().First; got != entries[i].Block {
			t.Fatalf("entry %d (%q): leading block %d, want %d",
				i, entries[i].Title, got, entries[i].Block)
		}
		if got := p.v.outlineList.Selected(); got != i {
			t.Errorf("entry %d chosen, mark is on entry %d", i, got)
		}
	}
	if p.cur.document() != before {
		t.Error("the document was replaced; choosing an entry must move the note, not reload it")
	}
}

// TestBothPanesStandInEitherState is the exit condition as a
// measurement: a note with many headings and a note with none both leave
// the outline and the backlinks their own band of the column, the two
// meeting without overlapping, and the outline never taking more than its
// share however many headings the note has.
func TestBothPanesStandInEitherState(t *testing.T) {
	plain := goldenModel()
	plain = cacheNote(plain, noteFromSource("Sources.md", "Just prose, no sections.\n"))
	plain.Current = "Sources.md"

	for _, tc := range []struct {
		name  string
		model Model
	}{
		{"many headings", outlineModel()},
		{"no headings", plain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const colH = 700
			p := newAsidePad(t, tc.model, colH)
			p.frame()
			g := p.v.geom
			if g.outline.Empty() {
				t.Fatalf("the outline pane occupies nothing: %v", g.outline)
			}
			if g.backlinks.Empty() {
				t.Fatalf("the backlinks pane occupies nothing: %v", g.backlinks)
			}
			if g.outline.Max.Y > g.backlinks.Min.Y {
				t.Errorf("the panes overlap: outline %v, backlinks %v", g.outline, g.backlinks)
			}
			if share := colH / asideOutlineShare; g.outline.Dy() > share {
				t.Errorf("the outline pane is %d tall in a %d column; it may not exceed %d",
					g.outline.Dy(), colH, share)
			}
			// Whatever the outline does, the backlinks keep a readable
			// band of the column below it.
			if g.backlinks.Dy() < colH/4 {
				t.Errorf("the backlinks pane is %d tall in a %d column; it was buried",
					g.backlinks.Dy(), colH)
			}
		})
	}
}

// TestTheOutlinePaneScrollsInItsOwnRight checks the two panes' scrolling
// is separate: moving the outline leaves the document, and the backlinks,
// exactly where they were.
func TestTheOutlinePaneScrollsInItsOwnRight(t *testing.T) {
	m := outlineModel()
	m.Index = &Index{Root: "/v", Files: []FileScan{
		{Path: "guide/Long note.md"},
		{Path: "Sources.md", Links: []string{"guide/Long note"}},
		{Path: "Design/Principles.md", Links: []string{"guide/Long note"}},
	}}
	p := newAsidePad(t, m, 700)
	p.frame()
	docFirst := p.doc.Position().First
	backFirst := p.v.list.Position().First

	p.v.outlineList.ScrollTo(20)
	p.frame()
	if got := p.v.outlineList.Position().First; got != 20 {
		t.Fatalf("the outline pane leads with entry %d, want 20", got)
	}
	if got := p.doc.Position().First; got != docFirst {
		t.Errorf("scrolling the outline moved the document to block %d, from %d", got, docFirst)
	}
	if got := p.v.list.Position().First; got != backFirst {
		t.Errorf("scrolling the outline moved the backlinks to row %d, from %d", got, backFirst)
	}
	// And the mark is untouched by a pane the reader merely scrolled: it
	// says where the reader is in the note, not where the pane is.
	if got, want := p.v.outlineList.Selected(), outlineActive(noteOutline(m.CurrentNote()), docFirst); got != want {
		t.Errorf("the mark moved to entry %d when the pane scrolled, want %d", got, want)
	}
}

// TestTheColumnHoldsNoDocumentBeforeOneIsShown is the state every other
// test skips past: a vault mid-scan has no document, and the column must
// lay out anyway rather than reading through a nil.
func TestTheColumnHoldsNoDocumentBeforeOneIsShown(t *testing.T) {
	v := newAsideView(&docCursor{})
	m := Model{Screen: screenVault, Vault: "/v", Scanning: true, CurAnchor: -1}
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(frameAsideDp, 700)),
		Ops:         &ops,
	}
	if d := v.layout(gtx, m, goldenTokens()); d.Size.X == 0 || d.Size.Y == 0 {
		t.Fatalf("the column produced %v with no document to describe", d)
	}
	if got := (&docCursor{}).document(); got != nil {
		t.Errorf("an empty cursor answered %v", got)
	}
	var nilCur *docCursor
	nilCur.show(nil)
	if got := nilCur.document(); got != nil {
		t.Errorf("a nil cursor answered %v", got)
	}
}

// TestASqueezedColumnStillShowsBothPanes is the window dragged short: the
// outline may not eat the column and leave the backlinks with nothing,
// however many headings the note has.
func TestASqueezedColumnStillShowsBothPanes(t *testing.T) {
	for _, colH := range []int{240, 160, 100} {
		p := newAsidePad(t, outlineModel(), colH)
		p.frame()
		g := p.v.geom
		if g.outline.Dy() > colH/asideOutlineShare {
			t.Errorf("column %d tall: the outline pane took %d", colH, g.outline.Dy())
		}
		if g.outline.Max.Y > g.backlinks.Min.Y {
			t.Errorf("column %d tall: the panes overlap: %v and %v", colH, g.outline, g.backlinks)
		}
	}
}
