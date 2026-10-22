package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
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

// TestNoteOutlineOfANoteWithNoHeadings: nothing to list, and nothing invented
// to list.
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
		style: markdown.FromTokens(tok.col, tok.typ, tok.col.TextBackground),
		docSz: image.Pt(noteFrameW, 400),
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

// clickAt presses and releases in the middle of the column at the given
// height — the way a reader's pointer reaches a row, through the router and
// against the areas the last frame registered.
func (p *asidePad) clickAt(y int) {
	at := f32.Pt(float32(p.colSz.X)/2, float32(y))
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	p.frame()
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

// TestTheMarkFollowsTheDocument: the document is moved by the reading keys'
// own machinery, and the outline's mark lands on the section the reader is
// now inside — without anything telling the column that the document moved.
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

// TestChoosingAnEntryMovesTheDocument: an entry takes the reader to its
// heading in the note they already have open. The document is the same object
// afterwards — a reload would lose the viewport, the interaction state and the
// history entry alike — and the heading it names leads the viewport.
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

// tailOutlineSource is a note that ends soon after its last heading: three
// sections of prose enough to carry it well past any window, then a closing
// heading with a single line under it. It is the shape where a heading can
// never lead the viewport — the note runs out before the heading can get
// there — and the outline has few enough rows that every one of them stands
// where the pane's own geometry says it does.
func tailOutlineSource() string {
	var b strings.Builder
	b.WriteString("# A note with a short tail\n\n")
	for i := 1; i <= 3; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n", i)
		for j := 1; j <= 12; j++ {
			fmt.Fprintf(&b, "Paragraph %d of section %d, prose enough to carry the note "+
				"well past the bottom of any window it is read in.\n\n", j, i)
		}
	}
	b.WriteString("## Closing\n\nOne line, and the note is over.\n")
	return b.String()
}

// TestAHeadingThatCannotLeadTheViewportIsStillChosen is the reader
// picking the last entry in the column: the note goes as far toward that
// heading as it can, and the entry they pressed is the one shown picked —
// for as long as they leave the note where the press put it, even though
// the heading itself never reaches the top.
func TestAHeadingThatCannotLeadTheViewportIsStillChosen(t *testing.T) {
	m := citedModel("guide/Short tail.md", tailOutlineSource(), 2)
	p := newAsidePad(t, m, 700)
	p.frame()

	entries := noteOutline(m.CurrentNote())
	if len(entries) != 5 {
		t.Fatalf("fixture outline has %d entries, want 5", len(entries))
	}
	tail := entries[len(entries)-1]
	rowH := asideRowPx()
	g := p.v.geom
	if want := len(entries) * rowH; g.outline.Dy() < want {
		t.Fatalf("the outline's region is %d px for %d rows; every row must stand in it",
			g.outline.Dy(), len(entries))
	}

	// The note as far along as it goes, which is where pressing the last
	// entry has to leave it.
	p.doc.ScrollToEnd()
	p.frame()
	end := p.doc.Position().First
	if end >= tail.Block {
		t.Fatalf("the last heading leads the viewport at block %d; the fixture must end too soon after it for that",
			end)
	}
	p.doc.ScrollToStart()
	p.frame()

	// The press wins at once: the entry is picked on the frame it landed
	// on, before the document has laid out anywhere else.
	p.clickAt(g.outline.Min.Y + tail.Idx*rowH + rowH/2)
	if got := p.v.outlineList.Selected(); got != tail.Idx {
		t.Errorf("pressing the last entry picked entry %d, want %d", got, tail.Idx)
	}
	if p.v.marked != tail.Idx {
		t.Errorf("pressing the last entry marked entry %d, want %d", p.v.marked, tail.Idx)
	}
	p.frame() // the document carries the move out
	if got := p.doc.Position().First; got != end {
		t.Errorf("pressing the last entry left the note at block %d, want %d — as far toward the heading as it goes",
			got, end)
	}
	// And it stays picked while the reader leaves the note alone: the
	// leading block is under an earlier heading and always will be, so a
	// mark taken from it alone would put the pick straight back.
	for i := range 3 {
		p.frame()
		if got := p.v.outlineList.Selected(); got != tail.Idx {
			t.Fatalf("frame %d after the press: the pick moved to entry %d, want %d",
				i, got, tail.Idx)
		}
		if p.v.marked != tail.Idx {
			t.Fatalf("frame %d after the press: the mark moved to entry %d, want %d",
				i, p.v.marked, tail.Idx)
		}
	}

	// The moment the reader moves the note themselves, the mark is the
	// note's again.
	p.doc.PageUp()
	p.frame()
	if got := p.doc.Position().First; got == end {
		t.Fatalf("the note did not move on a page up; it is still at block %d", got)
	}
	want := outlineActive(entries, p.doc.Position().First)
	if got := p.v.outlineList.Selected(); got != want {
		t.Errorf("after the reader moved the note the pick is entry %d, want %d", got, want)
	}
	if p.v.marked != want {
		t.Errorf("after the reader moved the note the mark is entry %d, want %d", p.v.marked, want)
	}
}

// TestEveryEntryCanBeChosen walks the whole outline of a note with a short
// tail, entry by entry: each one is the one picked once it has been pressed,
// whether or not its heading can rise to the top of the viewport.
func TestEveryEntryCanBeChosen(t *testing.T) {
	m := citedModel("guide/Short tail.md", tailOutlineSource(), 2)
	p := newAsidePad(t, m, 700)
	p.frame()

	entries := noteOutline(m.CurrentNote())
	rowH := asideRowPx()
	g := p.v.geom
	for _, e := range entries {
		p.clickAt(g.outline.Min.Y + e.Idx*rowH + rowH/2)
		p.frame()
		if got := p.v.outlineList.Selected(); got != e.Idx {
			t.Errorf("pressing entry %d (%q) picked entry %d", e.Idx, e.Title, got)
		}
		if p.v.marked != e.Idx {
			t.Errorf("pressing entry %d (%q) marked entry %d", e.Idx, e.Title, p.v.marked)
		}
	}
}

// TestAChoiceLetsGoOfADocumentItDidNotFollow is the choice's own state
// machine, without a column around it: it stands while the same document
// rests where the choice left it, lets go the moment that document is
// somewhere else, and lets go of a document that was replaced under it even
// if the new one happens to rest on the same block.
func TestAChoiceLetsGoOfADocumentItDidNotFollow(t *testing.T) {
	n := noteFromSource("guide/Short tail.md", tailOutlineSource())
	doc := markdown.NewDocument(n.Blocks)
	other := markdown.NewDocument(n.Blocks)

	var c outlineChoice
	if at, held := c.stands(doc, 4); held {
		t.Errorf("a choice nobody made stands on entry %d", at)
	}

	c.take(doc, 4)
	// The frame the choice is made on has not laid the document out again,
	// so the first frame after it is what says where the note came to rest.
	for i, first := range []int{31, 31, 31} {
		at, held := c.stands(doc, first)
		if !held || at != 4 {
			t.Fatalf("frame %d with the note still at block %d: entry %d, held %v; want entry 4 held",
				i, first, at, held)
		}
	}
	if at, held := c.stands(doc, 12); held {
		t.Errorf("the note moved to block 12 and the choice still stands on entry %d", at)
	}
	if at, held := c.stands(doc, 31); held {
		t.Errorf("the note came back to block 31 and the dropped choice stands again on entry %d", at)
	}

	c.take(doc, 4)
	if _, held := c.stands(doc, 31); !held {
		t.Fatal("a fresh choice did not stand on the frame after it was made")
	}
	if at, held := c.stands(other, 31); held {
		t.Errorf("the document was replaced under the choice and it stands on entry %d", at)
	}
	c.take(doc, 4)
	if at, held := c.stands(nil, 31); held {
		t.Errorf("there is no document at all and the choice stands on entry %d", at)
	}
}

// TestBothPanesStandInEitherState: a note with many headings and a note with
// none both leave the outline and the backlinks their own band of the column,
// the two meeting without overlapping, and neither pane standing taller than
// what it has to put in it.
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
			if ceiling := asideBacklinkCap * asideRowPx(); g.backlinks.Dy() > ceiling {
				t.Errorf("the backlinks pane is %d tall; the cap is %d", g.backlinks.Dy(), ceiling)
			}
			// The pane stands on the column's foot, one inset off the
			// bottom edge like everything else the column holds.
			if got, want := g.backlinks.Max.Y, colH-asideInsetDp; got != want {
				t.Errorf("the backlinks pane ends at %d in a %d column; the column's foot is %d", got, colH, want)
			}
		})
	}
}

// TestTheOutlineHoldsTheColumnsSlack is the aside's composition: the
// backlinks group stands on the foot and the room the column has spare
// opens inside the outline's region, above the rule, rather than under the
// pane.
//
// It is measured across notes that fill the column to very different
// depths — none, a few, forty headings; none, a few, twenty citations. The
// pane ends on the column's foot in every one, and the run between the
// outline's region and the pane is the same in every one,
// which is the slack being inside that region rather than below it: were
// any of it falling to the foot, the note with the least to show would
// have the longest run.
func TestTheOutlineHoldsTheColumnsSlack(t *testing.T) {
	const colH = 700
	gaps := map[string]int{}
	outlines := map[string]int{}
	for _, tc := range []struct {
		name  string
		model Model
	}{
		{"nothing to show", citedModel("Sources.md", plainNoteSource, 0)},
		{"a few of each", citedModel("guide/Outline.md", outlineSource, 3)},
		{"more than fits", citedModel("guide/Long note.md", longNoteSource(), 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newAsidePad(t, tc.model, colH)
			p.frame()
			g := p.v.geom
			if got, want := g.backlinks.Max.Y, colH-asideInsetDp; got != want {
				t.Errorf("the backlinks pane ends at %d, want the column's foot at %d", got, want)
			}
			if g.outline.Empty() {
				t.Fatalf("the outline holds no region at all: %v", g.outline)
			}
			gaps[tc.name] = g.backlinks.Min.Y - g.outline.Max.Y
			outlines[tc.name] = g.outline.Dy()
		})
	}
	if gaps["nothing to show"] != gaps["a few of each"] || gaps["a few of each"] != gaps["more than fits"] {
		t.Errorf("the run between the outline's region and the backlinks pane is %v; the trim between them is one fixed thing, so the slack must be above it and not below",
			gaps)
	}
	// A note with nothing to list holds the larger region, since its
	// backlinks pane is a single line rather than the capped four.
	if outlines["nothing to show"] <= outlines["more than fits"] {
		t.Errorf("a note with no headings and no citations gives its outline %d px, one with forty headings and twenty citations gives %d; the room the smaller pane gives up is the outline's",
			outlines["nothing to show"], outlines["more than fits"])
	}
}

// asideRowPx is a pane row's height in the pixels these tests measure in,
// where one dp is one pixel. Both panes' rows stand at the sidebar's own
// row height, which no density moves.
func asideRowPx() int { return int(sidebar.RowHeight) }

// citedModel makes a note current and gives it cites citing notes, so the
// backlinks pane has exactly that many rows to size itself to. src is the
// note's own text, which is what decides how many rows the outline above
// it has.
func citedModel(notePath, src string, cites int) Model {
	m := goldenModel()
	m = cacheNote(m, noteFromSource(notePath, src))
	m.Current = notePath
	m.CurAnchor = -1
	idx := &Index{Root: "/v", Files: []FileScan{{Path: notePath}}}
	for i := range cites {
		idx.Files = append(idx.Files, FileScan{
			Path:  fmt.Sprintf("notes/Citing %02d.md", i),
			Links: []string{noteTitle(notePath)},
		})
	}
	m.Index = idx
	return m
}

// TestTheBacklinksPaneIsAsTallAsItsRows is the cap at its boundaries: no
// citations, one, exactly the cap, and far past it. The pane is its own
// rows and nothing else — a note cited twice does not hold four rows of
// room, and a note cited twenty times does not take the column.
//
// The room it does not take is the outline's, which the second half
// measures: between a note cited none and one cited the cap, the outline
// gains exactly the three rows the backlinks gave up.
func TestTheBacklinksPaneIsAsTallAsItsRows(t *testing.T) {
	const colH = 700
	rowH := asideRowPx()
	outlineDy := map[string]int{}
	for _, tc := range []struct {
		name  string
		cites int
		rows  int
	}{
		// Nothing to list is one line saying so, not four empty rows.
		{"none", 0, 1},
		{"one", 1, 1},
		{"the cap", asideBacklinkCap, asideBacklinkCap},
		{"past the cap", 20, asideBacklinkCap},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := citedModel("guide/Long note.md", longNoteSource(), tc.cites)
			if got := len(Backlinks(m.Index, m.Current)); got != tc.cites {
				t.Fatalf("the fixture yielded %d backlinks, want %d", got, tc.cites)
			}
			p := newAsidePad(t, m, colH)
			p.frame()
			g := p.v.geom
			if got, want := g.backlinks.Dy(), tc.rows*rowH; got != want {
				t.Errorf("%d citations: the pane is %d tall, want %d (%d rows)", tc.cites, got, want, tc.rows)
			}
			if g.outline.Max.Y > g.backlinks.Min.Y {
				t.Errorf("the panes overlap: outline %v, backlinks %v", g.outline, g.backlinks)
			}
			outlineDy[tc.name] = g.outline.Dy()
		})
	}
	if got, want := outlineDy["none"]-outlineDy["the cap"], (asideBacklinkCap-1)*rowH; got != want {
		t.Errorf("a note cited none gives the outline %d px more than one cited %d, want the %d px the backlinks gave up",
			got, asideBacklinkCap, want)
	}
	if outlineDy["past the cap"] != outlineDy["the cap"] {
		t.Errorf("past the cap the outline is %d tall, at the cap %d; the pane must stop taking room",
			outlineDy["past the cap"], outlineDy["the cap"])
	}
}

// TestBacklinksPastTheCapScrollWithinIt is the other half of the cap: the
// citations it cannot show are operable by scrolling the pane, not lost,
// and the pane scrolling moves neither the outline nor the document.
func TestBacklinksPastTheCapScrollWithinIt(t *testing.T) {
	m := citedModel("guide/Long note.md", longNoteSource(), 20)
	p := newAsidePad(t, m, 700)
	p.frame()
	docFirst := p.doc.Position().First
	outlineFirst := p.v.outlineList.Position().First

	if got := p.v.list.Position().Count; got > asideBacklinkCap+1 {
		t.Errorf("the pane laid out %d of 20 rows; the cap shows at most %d", got, asideBacklinkCap)
	}
	p.v.list.ScrollTo(16)
	p.frame()
	if got := p.v.list.Position().First; got != 16 {
		t.Fatalf("the backlinks pane leads with row %d, want 16", got)
	}
	if got := p.doc.Position().First; got != docFirst {
		t.Errorf("scrolling the backlinks moved the document to block %d, from %d", got, docFirst)
	}
	if got := p.v.outlineList.Position().First; got != outlineFirst {
		t.Errorf("scrolling the backlinks moved the outline to entry %d, from %d", got, outlineFirst)
	}
	// The last citation is operable from the keyboard, which is the
	// selection walking rows the capped pane never laid out.
	p.v.list.Select(-1)
	p.v.list.Reveal(19)
	p.frame()
	q := p.v.list.Position()
	if q.First+q.Count < 20 {
		t.Errorf("revealing the last citation left the pane at %+v; row 19 was never laid out", q)
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

	// Far enough down that the pane cannot lead with it and clamps at its
	// own end: what is asserted is that the outline moved, not where a
	// viewport of a particular height comes to rest.
	p.v.outlineList.ScrollTo(20)
	p.frame()
	if got := p.v.outlineList.Position().First; got == 0 {
		t.Fatalf("the outline pane still leads with entry %d; scrolling it moved nothing", got)
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

// TestASqueezedColumnStillShowsBothPanes is the window dragged short:
// neither pane may eat the column and leave the other with nothing,
// however many headings the note has and however often it is cited.
func TestASqueezedColumnStillShowsBothPanes(t *testing.T) {
	for _, colH := range []int{240, 160, 100} {
		p := newAsidePad(t, citedModel("guide/Long note.md", longNoteSource(), 20), colH)
		p.frame()
		g := p.v.geom
		if g.backlinks.Dy() > colH/asideBacklinkShare {
			t.Errorf("column %d tall: the backlinks pane took %d", colH, g.backlinks.Dy())
		}
		if g.outline.Max.Y > g.backlinks.Min.Y {
			t.Errorf("column %d tall: the panes overlap: %v and %v", colH, g.outline, g.backlinks)
		}
	}
}

// TestThePanesKeepTheirNavigationWhenResized presses in each pane where the
// resized layout puts its rows, through the router the running window uses:
// an outline row moves the document to its heading, a backlink row marks the
// citation it names — which is the click the Navigate message rides out on —
// and the document-tracking mark returns to where the reader actually is the
// moment the document moves again.
func TestThePanesKeepTheirNavigationWhenResized(t *testing.T) {
	m := citedModel("guide/Long note.md", longNoteSource(), 20)
	p := newAsidePad(t, m, 700)
	p.frame()

	entries := noteOutline(m.CurrentNote())
	rowH := asideRowPx()
	// The third row of each pane: far enough down that a press landing on
	// the pane's first row by accident would be caught.
	const row = 2
	p.clickAt(p.v.geom.outline.Min.Y + row*rowH + rowH/2)
	if got, want := p.doc.Position().First, entries[row].Block; got != want {
		t.Errorf("pressing outline row %d left the document at block %d, want %d", row, got, want)
	}
	if got := p.v.outlineList.Selected(); got != row {
		t.Errorf("pressing outline row %d marked entry %d", row, got)
	}

	p.clickAt(p.v.geom.backlinks.Min.Y + row*rowH + rowH/2)
	if got := p.v.list.Selected(); got != row {
		t.Errorf("pressing backlink row %d marked row %d", row, got)
	}

	// And the mark is the document's again as soon as the document moves.
	p.doc.ScrollToEnd()
	p.frame()
	if got, want := p.v.outlineList.Selected(), outlineActive(entries, p.doc.Position().First); got != want {
		t.Errorf("after the document moved the mark is entry %d, want %d", got, want)
	}
}

// sparseOutlineSource is a note far taller than any viewport with only
// four headings in it: many pages of prose under each. It is the shape
// whose outline leaves most of its region empty, and the shape the hit
// geometry has to be measured on — a note with headings enough to fill
// the pane cannot say where its rows begin.
func sparseOutlineSource() string {
	var b strings.Builder
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n", i)
		for j := 1; j <= 20; j++ {
			fmt.Fprintf(&b, "Paragraph %d of section %d, one of many under a heading that is one of few.\n\n", j, i)
		}
	}
	return b.String()
}

// TestThePanesFindTheirRowsWithRoomToSpare is the hit geometry once the
// panes stand apart: on a note whose outline is far shorter than the
// region it holds, a press still finds the row it landed on — the rows lead
// their region, so they are counted from its top edge — a press in the spare
// surface
// below the last row moves nothing, because the slack is room and not a
// target, and the pane at the foot answers its own rows where they stand.
func TestThePanesFindTheirRowsWithRoomToSpare(t *testing.T) {
	m := citedModel("guide/Sections.md", sparseOutlineSource(), 3)
	p := newAsidePad(t, m, 700)
	p.frame()

	entries := noteOutline(m.CurrentNote())
	if len(entries) != 4 {
		t.Fatalf("fixture outline has %d entries, want 4", len(entries))
	}
	rowH := asideRowPx()
	g := p.v.geom
	if want := (len(entries) + 2) * rowH; g.outline.Dy() < want {
		t.Fatalf("the outline's region is %d px for %d rows; the fixture is meant to leave room to spare",
			g.outline.Dy(), len(entries))
	}

	const row = 2
	p.clickAt(g.outline.Min.Y + row*rowH + rowH/2)
	if got, want := p.doc.Position().First, entries[row].Block; got != want {
		t.Errorf("pressing outline row %d left the document at block %d, want %d", row, got, want)
	}
	if got := p.v.outlineList.Selected(); got != row {
		t.Errorf("pressing outline row %d marked entry %d", row, got)
	}

	at := p.doc.Position().First
	p.clickAt(g.outline.Min.Y + (len(entries)+1)*rowH + rowH/2)
	if got := p.doc.Position().First; got != at {
		t.Errorf("pressing the outline's spare surface moved the document to block %d, from %d", got, at)
	}
	if got := p.v.outlineList.Selected(); got != row {
		t.Errorf("pressing the outline's spare surface moved the mark to entry %d, from %d", got, row)
	}

	p.clickAt(p.v.geom.backlinks.Min.Y + row*rowH + rowH/2)
	if got := p.v.list.Selected(); got != row {
		t.Errorf("pressing backlink row %d at the column's foot marked row %d", row, got)
	}
}

// TestEachScrollingPaneShowsItsIndicator, in pixels: a pane with more rows
// than it can show draws the indicator in its own trailing gutter, and a pane
// whose rows all fit draws nothing there. Both panes, one column — the outline
// scrolling and the backlinks capped past their cap.
//
// The reading is taken with the reader's "always" setting on the snapshot,
// which is the one that puts a bar on a resting frame at all; what the
// other settings draw at rest is read in
// [TestTheColumnSpendsTheScrollBarSetting].
func TestEachScrollingPaneShowsItsIndicator(t *testing.T) {
	tok := standingBars(goldenTokens())
	surface := chromeSurface(tok.col)
	shot := func(m Model, colH int) (*image.RGBA, asideGeom) {
		v := newAsideView(&docCursor{})
		size := image.Pt(frameAsideDp, colH)
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, surface, clip.Rect{Max: size}.Op())
			return v.layout(gtx, m, tok)
		})
		return img, v.geom
	}
	// The strip scanned is the thumb's own columns, inside the lane each
	// pane hands its bar. Nothing else in the column is drawn there — the
	// rows, their marks and the hairline all stop where the lane starts —
	// so anything in these columns is the bar.
	bar := asideIndicator(tok)
	thumbFrom := gtx1Dp(bar.Width() - bar.TrackPadding) // the thumb's leading edge
	thumbTo := gtx1Dp(bar.TrackPadding)                 // its trailing padding
	gutterPixels := func(img *image.RGBA, band image.Rectangle) int {
		n := 0
		for y := band.Min.Y; y < band.Max.Y; y++ {
			for x := band.Max.X - thumbFrom; x < band.Max.X-thumbTo; x++ {
				if c := img.RGBAAt(x, y); c.R != surface.R || c.G != surface.G || c.B != surface.B {
					n++
				}
			}
		}
		return n
	}

	full, g := shot(citedModel("guide/Long note.md", longNoteSource(), 20), 700)
	if n := gutterPixels(full, g.outline); n == 0 {
		t.Error("an outline with more headings than its pane can show drew no indicator")
	}
	if n := gutterPixels(full, g.backlinks); n == 0 {
		t.Error("a backlinks pane holding twenty citations in four rows drew no indicator")
	}

	short, sg := shot(citedModel("Sources.md", plainNoteSource, 2), 700)
	if n := gutterPixels(short, sg.backlinks); n != 0 {
		t.Errorf("a pane whose two citations both fit drew %d indicator pixels, want none", n)
	}
}

// gtx1Dp converts dp to the pixels these tests measure in, where the metric
// is one pixel per dp.
func gtx1Dp(v unit.Dp) int { return int(v) }

// TestTheBacklinkHeaderCountsWhatItCannotShow pins both halves of the
// header: a note with citations carries their number, a note with none
// carries nothing extra — and either way it is the same one line tall, which
// is what the column's height arithmetic measures the pane below it against.
func TestTheBacklinkHeaderCountsWhatItCannotShow(t *testing.T) {
	tok := goldenTokens()
	surface := chromeSurface(tok.col)
	size := image.Pt(frameAsideDp-2*asideInsetDp, 40)
	shot := func(n int) (*image.RGBA, int) {
		var h int
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, surface, clip.Rect{Max: size}.Op())
			d := asideBacklinkHeader(gtx, tok, n)
			h = d.Size.Y
			return d
		})
		return img, h
	}
	paintedPixels := func(img *image.RGBA) int {
		n := 0
		for y := range size.Y {
			for x := range size.X {
				if c := img.RGBAAt(x, y); c.R != surface.R || c.G != surface.G || c.B != surface.B {
					n++
				}
			}
		}
		return n
	}

	none, noneH := shot(0)
	over, overH := shot(20)
	if noneH != overH {
		t.Errorf("the header is %d tall with a count and %d without; the pane below it is measured off one line", overH, noneH)
	}
	if paintedPixels(over) <= paintedPixels(none) {
		t.Errorf("a pane holding 20 citations painted no more of its header (%d) than one holding none (%d); the count is missing",
			paintedPixels(over), paintedPixels(none))
	}
	if few, _ := shot(2); paintedPixels(few) <= paintedPixels(none) {
		t.Errorf("a pane holding 2 citations painted no more of its header (%d) than one holding none (%d); the count must not appear only past the cap",
			paintedPixels(few), paintedPixels(none))
	}
}

// ---- the column's rhythm, its axis and its tiers ------------------------

// rhythmSource nests three heading levels whose titles all begin with the
// same letter, so a level's step is the difference between two identical
// glyphs' leading edges and not two different left bearings.
const rhythmSource = `# Section, the first

Prose under the title.

## Section, the second

More prose.

### Section, the third

Yet more prose.
`

// asideShotFrom lays a note's document out and then the trailing column on
// its own, on the surface the frame paints under the column, and captures
// the column. The document goes into a recording that is thrown away: the
// outline reads the position it resolves, and none of the note's own text
// reaches the picture.
//
// Two frames, because the mark is written from the position the first one
// resolves and drawn by the one after it.
//
// The document is placed at its start, or at its end, which puts the mark
// on the column's last entry and leaves every row above it bare. A
// measurement that reads a row's own leading edge needs a bare row in the
// light appearance, where the label the platform pairs with a selection is
// white and the chrome material is white too, so no instrument can find a
// marked row's text on its pill.
func asideShotFrom(t *testing.T, m Model, col tokens.PlatformColors, h int, atEnd bool) (*image.RGBA, *asideView) {
	t.Helper()
	tok := goldenTokens()
	tok.col = col
	n := m.CurrentNote()
	if n == nil {
		t.Fatal("the model has no current note")
	}
	doc := markdown.NewDocument(n.Blocks)
	style := markdown.FromTokens(tok.col, tok.typ, tok.col.TextBackground)
	cur := &docCursor{}
	v := newAsideView(cur)
	w := func(gtx layout.Context) layout.Dimensions {
		dgtx := gtx
		dgtx.Constraints = layout.Exact(image.Pt(noteFrameW, 400))
		rec := op.Record(dgtx.Ops)
		doc.Layout(dgtx, tok.shaper, style)
		rec.Stop()
		if atEnd {
			doc.ScrollToEnd()
		}
		cur.show(doc)
		return v.layout(gtx, m, tok)
	}
	size := image.Pt(frameAsideDp, h)
	golden.Capture(t, size, scene(w, chromeSurface(col)))
	return golden.Capture(t, size, scene(w, chromeSurface(col))), v
}

// absGap is the distance between two eight-bit channels.
func absGap(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

// asideDrawnAt answers where the paint between two rows of the captured
// column starts — its leading column and its first row, or -1, -1 for a
// band of bare surface. over is the fill lying under that band: the
// chrome material where the rows are bare, one of the pattern's two
// selection fills where a row is spoken for. Both it and the chrome read as
// surface, and so is any blend of the two, because a pill's rounded corner
// ramps from one to the other over two or three pixels and the emphasized
// pill is the accent, far enough from the chrome that a blend of the two is
// near neither.
func asideDrawnAt(img *image.RGBA, col tokens.PlatformColors, y0, y1 int, over, foreground color.NRGBA) (int, int) {
	bare := asideBare(col, over, foreground)
	lead, top := -1, -1
	for y := max(y0, img.Bounds().Min.Y); y < min(y1, img.Bounds().Max.Y); y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if bare(img.RGBAAt(x, y)) {
				continue
			}
			if top < 0 {
				top = y
			}
			if lead < 0 || x < lead {
				lead = x
			}
			break
		}
	}
	return lead, top
}

// asideDrawnSpan answers the first and last row of the same band that carry
// paint, or -1, -1 for a band of bare surface. It reads the picture the way
// [asideDrawnAt] does and differs only in the axis it reports: where that
// one answers where paint starts, this one answers how far down the band it
// reaches, which is what says whether a line of text stands whole inside it.
func asideDrawnSpan(img *image.RGBA, col tokens.PlatformColors, y0, y1 int, over, foreground color.NRGBA) (int, int) {
	bare := asideBare(col, over, foreground)
	top, bottom := -1, -1
	for y := max(y0, img.Bounds().Min.Y); y < min(y1, img.Bounds().Max.Y); y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if bare(img.RGBAAt(x, y)) {
				continue
			}
			if top < 0 {
				top = y
			}
			bottom = y
			break
		}
	}
	return top, bottom
}

// asideBare reports whether a captured pixel is bare surface rather than
// paint: the chrome material, the fill lying over it where a row is spoken
// for, or any blend of the two.
func asideBare(col tokens.PlatformColors, over, foreground color.NRGBA) func(color.RGBA) bool {
	// Half the distance between the surface and the foreground drawn on it:
	// past that a pixel carries more than half a glyph's coverage. It is
	// derived per band rather than written down because the platform's label
	// strengths are coverages of very different depth, and a fixed threshold
	// would find a strong title at a tenth of a glyph and a faint line only
	// at three quarters of one — which reads as a placement difference that
	// is not there.
	surf := chromeSurface(col)
	slack := float64(max(absGap(foreground.R, surf.R),
		max(absGap(foreground.G, surf.G), absGap(foreground.B, surf.B)))) / 2
	a, b := surf, over
	ax, ay, az := float64(a.R), float64(a.G), float64(a.B)
	dx, dy, dz := float64(b.R)-ax, float64(b.G)-ay, float64(b.B)-az
	den := dx*dx + dy*dy + dz*dz
	return func(c color.RGBA) bool {
		t := 0.0
		if den > 0 {
			t = ((float64(c.R)-ax)*dx + (float64(c.G)-ay)*dy + (float64(c.B)-az)*dz) / den
			t = min(max(t, 0), 1)
		}
		return max(math.Abs(float64(c.R)-(ax+t*dx)),
			max(math.Abs(float64(c.G)-(ay+t*dy)), math.Abs(float64(c.B)-(az+t*dz)))) < slack
	}
}

// TestTheOutlineStepsOnTheColumnsRhythm measures one heading level's step
// off the picture: the leading edge of a level-two title against a
// level-one's, and a level-three's against the level-two's. The column moves
// everything it holds by one distance — the pad a row's text stands inside its
// fill, the lane its bar stands in — so the outline's step is that same eight
// and not a second rhythm down one narrow list.
func TestTheOutlineStepsOnTheColumnsRhythm(t *testing.T) {
	rowH := asideRowPx()
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			// The document is placed at its end, so the mark is on the last
			// entry and the rows measured here are bare.
			img, v := asideShotFrom(t, citedModel("guide/Rhythm.md", rhythmSource, 2), tc.colors, 700, true)
			tok := goldenTokens()
			tok.col = tc.colors
			fg := asideForegrounds(tok)
			var lead [2]int
			for i := range lead {
				top := v.geom.outline.Min.Y + i*rowH
				// A top-level heading is read at the column's reading tier
				// and everything under it a tier fainter.
				title := fg.reading
				if i > 0 {
					title = fg.nested
				}
				lead[i], _ = asideDrawnAt(img, tc.colors, top, top+rowH, chromeSurface(tc.colors), title)
				if lead[i] < 0 {
					t.Fatalf("the outline's level-%d row drew nothing to measure", i+1)
				}
			}
			for i := 1; i < len(lead); i++ {
				if got := lead[i] - lead[i-1]; got != asideRowPadDp {
					t.Errorf("a level-%d title stands %d px in from the level-%d title above it, want %d — one column, one rhythm",
						i+1, got, i, asideRowPadDp)
				}
			}
		})
	}
}

// asideDescenderSource is a note whose headings drop below the baseline,
// which is what a row too short for its line box takes off first.
const asideDescenderSource = `# Typography

Prose under the title.

## Paragraphs

More prose.
`

// TestBothPanesRowsHoldTheirWholeText reads a row's text off the picture and
// requires all of it to stand inside the row, centred there by the row
// itself: the letters paint every row they paint with nothing cutting them,
// and they paint them where a line box centred in the row would put them.
// A row shorter than its own line box clips the text helper's box, and what
// goes first is the part of the letters below the baseline — which is a row
// that looks right in a list of capitals and loses letters in the next note.
func TestBothPanesRowsHoldTheirWholeText(t *testing.T) {
	rowH := asideRowPx()
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			// The document is placed at its end, so the mark is on the last
			// entry and the first row of each pane is bare.
			img, v := asideShotFrom(t, citedModel("guide/Typography.md", asideDescenderSource, 2), tc.colors, 700, true)
			tok := goldenTokens()
			tok.col = tc.colors
			foreground := asideForegrounds(tok).reading
			for _, c := range []struct {
				pane  string
				at    image.Rectangle
				title string
			}{
				{"outline", v.geom.outline, "Typography"},
				{"backlinks", v.geom.backlinks, "Citing 00"},
			} {
				box, first, last := asideLineBox(t, tok, c.title, foreground)
				if box > rowH {
					t.Errorf("the %s pane's line box is %d tall in a row of %d; the row holds the box", c.pane, box, rowH)
					continue
				}
				top, bottom := asideDrawnSpan(img, tc.colors, c.at.Min.Y, c.at.Min.Y+rowH, chromeSurface(tc.colors), foreground)
				if top < 0 {
					t.Fatalf("the %s pane's first row drew no text to measure", c.pane)
				}
				// What the row has over the box is shared above and below
				// it, half each and the odd pixel above the text, which is
				// how the text helper centres a line in a floor.
				boxTop := c.at.Min.Y + (rowH-box)/2
				if top != boxTop+first || bottom != boxTop+last {
					t.Errorf("the %s pane's %q paints rows %d to %d, and whole in a line box centred in its row it would paint %d to %d",
						c.pane, c.title, top, bottom, boxTop+first, boxTop+last)
				}
			}
		})
	}
}

// asideLineBox measures one line of a row's text with nothing cutting it:
// the height the text helper gives the line box, and the first and last row
// of paint the letters cover, counted from the box's own top. A row that
// cuts its text, or stands it anywhere but centred, paints elsewhere.
func asideLineBox(t *testing.T, tok themeTokens, title string, foreground color.NRGBA) (box, first, last int) {
	t.Helper()
	var dims layout.Dimensions
	w := func(gtx layout.Context) layout.Dimensions {
		// Released from the capture's own floor, so the box stands at the
		// picture's top edge and the helper reports the box itself rather
		// than the picture.
		gtx.Constraints.Min.Y = 0
		dims = typeset.Text(gtx, tok.shaper, title, tok.typ.BodyMedium, foreground, 1)
		return dims
	}
	size := image.Pt(frameAsideDp, 64)
	img := golden.Capture(t, size, scene(w, chromeSurface(tok.col)))
	top, bottom := asideDrawnSpan(img, tok.col, 0, size.Y, chromeSurface(tok.col), foreground)
	if top < 0 {
		t.Fatalf("the measurement drew no %q to read", title)
	}
	return dims.Size.Y, top, bottom
}

// TestTheColumnSpendsTheScrollBarSetting reads the inspector at both
// settings, in pixels, off one model whose outline is longer than its pane.
//
// With the reader's "always" setting the bar is a standing scroller in a
// lane of its own: the thumb is there on a resting frame, and the rows stop
// short of the lane. With the setting the platform decides for itself the
// bar fades, so a resting frame holds no thumb at all — and the lane goes
// with it, the marked row running to the column's own trailing lane, the
// width it keeps on the leading side.
func TestTheColumnSpendsTheScrollBarSetting(t *testing.T) {
	fading := goldenTokens()
	standing := standingBars(fading)
	surface := chromeSurface(fading.col)
	m := citedModel("guide/Long note.md", longNoteSource(), 20)
	size := image.Pt(frameAsideDp, 700)
	// A row of the outline is marked, because the mark's own fill is what
	// the reading measures the rows' trailing edge by: it is one colour laid
	// down as a fill, where a row's text is antialiased glyphs.
	shot := func(tok themeTokens) (*image.RGBA, asideGeom) {
		v := newAsideView(&docCursor{})
		v.outlineList.Select(1)
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, surface, clip.Rect{Max: size}.Op())
			return v.layout(gtx, m, tok)
		})
		return img, v.geom
	}
	// The band the bar's thumb stands in, which nothing else in the column
	// is drawn in while a lane is reserved for it.
	bar := asideIndicator(standing)
	band := image.Rect(size.X-gtx1Dp(bar.Width()-bar.TrackPadding), 0,
		size.X-gtx1Dp(bar.TrackPadding), size.Y)
	// The thumb is counted by its own colour rather than as "not the
	// surface": with no lane reserved the rows run into the band the thumb
	// would have stood in, and a marked row there is not a bar.
	thumb := func(img *image.RGBA, box image.Rectangle) int {
		n := 0
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				if c := img.RGBAAt(x, y); c.R == bar.ThumbColor.R && c.G == bar.ThumbColor.G && c.B == bar.ThumbColor.B {
					n++
				}
			}
		}
		return n
	}
	// The trailing column of the marked row's own fill, which is what says
	// where the rows stop.
	markEnd := func(img *image.RGBA, box image.Rectangle) int {
		hi := -1
		fill := sidebar.SelectionFill(fading.col, true)
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				c := img.RGBAAt(x, y)
				if c.R == fill.R && c.G == fill.G && c.B == fill.B {
					hi = max(hi, x)
				}
			}
		}
		return hi
	}

	stand, sg := shot(standing)
	if n := thumb(stand, band.Intersect(sg.outline)); n == 0 {
		t.Error("a standing bar drew no thumb on a resting frame")
	}
	if got, want := markEnd(stand, sg.outline), size.X-gtx1Dp(asideBarLane(standing)+asideLaneDp)-1; abs(got-want) > 1 {
		t.Errorf("with the bar standing the outline's mark ends at %d, want %d — a lane short of the bar's own", got, want)
	}

	fade, fg := shot(fading)
	if n := thumb(fade, band.Intersect(fg.outline)); n != 0 {
		t.Errorf("a fading bar drew %d thumb pixels on a resting frame, want none", n)
	}
	if got := asideBarLane(fading); got != 0 {
		t.Errorf("a fading bar was handed a lane of %v, want none", got)
	}
	if got, want := markEnd(fade, fg.outline), size.X-gtx1Dp(asideLaneDp)-1; abs(got-want) > 1 {
		t.Errorf("with the bar away the outline's mark ends at %d, want %d — the column's own trailing lane", got, want)
	}
}

// TestAnOutlineRowHoldsItsTitleWhenSelected requires a row's title to stand
// at the same leading edge whether the row is selected or not: the pill is
// the fill behind a selected row and moves nothing. The reading is taken in
// the dark scheme alone, because the label the platform pairs with a
// selection is white and so is the light chrome material, which leaves no
// instrument able to find a marked row's text on its pill.
func TestAnOutlineRowHoldsItsTitleWhenSelected(t *testing.T) {
	rowH := asideRowPx()
	col := tokens.PlatformDark
	m := citedModel("guide/Reading list.md", goldenNoteSource, 2)
	// The same column twice: the document at its start, which puts the
	// mark on the first row, and at its end, which leaves that row bare.
	marked, mv := asideShotFrom(t, m, col, 700, false)
	bare, bv := asideShotFrom(t, m, col, 700, true)
	tok := goldenTokens()
	tok.col = col
	fg := asideForegrounds(tok)

	// The first two rows: the note's own title, and the first heading under
	// it, which stands one level in. Each is read marked and bare.
	for i := range 2 {
		title := fg.reading
		if i > 0 {
			title = fg.nested
		}
		top := mv.geom.outline.Min.Y + i*rowH
		on, _ := asideDrawnAt(marked, col, top, top+rowH,
			sidebar.SelectionFill(col, false), sidebar.SelectionLabel(col, false))
		off, _ := asideDrawnAt(bare, col, bv.geom.outline.Min.Y+i*rowH, bv.geom.outline.Min.Y+(i+1)*rowH,
			chromeSurface(col), title)
		if on < 0 || off < 0 {
			t.Fatalf("row %d drew nothing to measure: marked %d, bare %d", i, on, off)
		}
		if on != off {
			t.Errorf("row %d leads at x=%d marked and x=%d bare; the pill is a fill and moves no title", i, on, off)
		}
	}
}
