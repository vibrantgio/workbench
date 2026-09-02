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
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/markdown"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
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
	rowH := asideRowPx(p.tok)
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
// whether or not its heading can reach the top of the viewport.
func TestEveryEntryCanBeChosen(t *testing.T) {
	m := citedModel("guide/Short tail.md", tailOutlineSource(), 2)
	p := newAsidePad(t, m, 700)
	p.frame()

	entries := noteOutline(m.CurrentNote())
	rowH := asideRowPx(p.tok)
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
			if ceiling := asideBacklinkCap * asideRowPx(p.tok); g.backlinks.Dy() > ceiling {
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
// pane ends on the column's foot in every one, and the run of furniture
// between the outline's region and the pane is the same in every one,
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
		t.Errorf("the run between the outline's region and the backlinks pane is %v; the furniture between them is one fixed thing, so the slack must be above it and not below",
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
// where one dp is one pixel.
func asideRowPx(tok themeTokens) int { return int(list.RowHeight(tok.den)) }

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
	rowH := asideRowPx(goldenTokens())
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
// citations it cannot show are reachable by scrolling the pane, not lost,
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
	// The last citation is reachable from the keyboard, which is the
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
	rowH := asideRowPx(p.tok)
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
// their region, so they are counted from its top edge — a press in the paper
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
	rowH := asideRowPx(p.tok)
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
		t.Errorf("pressing the outline's spare paper moved the document to block %d, from %d", got, at)
	}
	if got := p.v.outlineList.Selected(); got != row {
		t.Errorf("pressing the outline's spare paper moved the mark to entry %d, from %d", got, row)
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
func TestEachScrollingPaneShowsItsIndicator(t *testing.T) {
	tok := goldenTokens()
	ground := chromeSurface(tok.col)
	shot := func(m Model, colH int) (*image.RGBA, asideGeom) {
		v := newAsideView(&docCursor{})
		size := image.Pt(frameAsideDp, colH)
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, ground, clip.Rect{Max: size}.Op())
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
	gutterInk := func(img *image.RGBA, band image.Rectangle) int {
		n := 0
		for y := band.Min.Y; y < band.Max.Y; y++ {
			for x := band.Max.X - thumbFrom; x < band.Max.X-thumbTo; x++ {
				if c := img.RGBAAt(x, y); c.R != ground.R || c.G != ground.G || c.B != ground.B {
					n++
				}
			}
		}
		return n
	}

	full, g := shot(citedModel("guide/Long note.md", longNoteSource(), 20), 700)
	if n := gutterInk(full, g.outline); n == 0 {
		t.Error("an outline with more headings than its pane can show drew no indicator")
	}
	if n := gutterInk(full, g.backlinks); n == 0 {
		t.Error("a backlinks pane holding twenty citations in four rows drew no indicator")
	}

	short, sg := shot(citedModel("Sources.md", plainNoteSource, 2), 700)
	if n := gutterInk(short, sg.backlinks); n != 0 {
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
	ground := chromeSurface(tok.col)
	size := image.Pt(frameAsideDp-2*asideInsetDp, 40)
	shot := func(n int) (*image.RGBA, int) {
		var h int
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, ground, clip.Rect{Max: size}.Op())
			d := asideBacklinkHeader(gtx, tok, n)
			h = d.Size.Y
			return d
		})
		return img, h
	}
	ink := func(img *image.RGBA) int {
		n := 0
		for y := range size.Y {
			for x := range size.X {
				if c := img.RGBAAt(x, y); c.R != ground.R || c.G != ground.G || c.B != ground.B {
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
	if ink(over) <= ink(none) {
		t.Errorf("a pane holding 20 citations drew no more header ink (%d) than one holding none (%d); the count is missing",
			ink(over), ink(none))
	}
	if few, _ := shot(2); ink(few) <= ink(none) {
		t.Errorf("a pane holding 2 citations drew no more header ink (%d) than one holding none (%d); the count must not appear only past the cap",
			ink(few), ink(none))
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

// asideShot lays a note's document out and then the trailing column on
// its own, on the surface the frame paints under the column, and captures
// the column. The document goes into a recording that is thrown away: the
// outline reads the position it resolves, and none of the note's own ink
// reaches the picture.
//
// Two frames, because the mark is written from the position the first one
// resolves and drawn by the one after it.
func asideShot(t *testing.T, m Model, col tokens.ColorTokens, h int) (*image.RGBA, *asideView) {
	t.Helper()
	tok := goldenTokens()
	tok.col = col
	n := m.CurrentNote()
	if n == nil {
		t.Fatal("the model has no current note")
	}
	doc := markdown.NewDocument(n.Blocks)
	style := markdown.FromTokens(tok.col, tok.typ)
	cur := &docCursor{}
	v := newAsideView(cur)
	w := func(gtx layout.Context) layout.Dimensions {
		dgtx := gtx
		dgtx.Constraints = layout.Exact(image.Pt(noteCanvasW, 400))
		rec := op.Record(dgtx.Ops)
		doc.Layout(dgtx, tok.shaper, style)
		rec.Stop()
		cur.show(doc)
		return v.layout(gtx, m, tok)
	}
	size := image.Pt(frameAsideDp, h)
	golden.Capture(t, size, scene(w, chromeSurface(col)))
	return golden.Capture(t, size, scene(w, chromeSurface(col))), v
}

// asideInkAt answers where the ink between two rows of the captured
// column starts — its leading column and its first row, or -1, -1 for a
// band of bare surface. The two fills a row may wear are read as ground
// along with the surface itself: both run to the column's ink margin and
// fill the row's whole height, so a marked row would otherwise answer
// with the fill's own corner rather than with its title's.
func asideInkAt(img *image.RGBA, col tokens.ColorTokens, y0, y1 int) (int, int) {
	ground := []color.NRGBA{chromeSurface(col), col.StateAt(tokens.LevelChrome, tokens.StateHover), col.Ramps.Primary.Step(300)}
	gap := func(a, b uint8) int {
		if a > b {
			return int(a) - int(b)
		}
		return int(b) - int(a)
	}
	// Half the distance between the surface and the quietest ink the
	// column draws on it: past that a pixel is a glyph's and not a fill's
	// own anti-aliasing.
	near := func(c color.RGBA, o color.NRGBA) bool {
		return max(gap(c.R, o.R), gap(c.G, o.G), gap(c.B, o.B)) < 60
	}
	lead, top := -1, -1
	for y := max(y0, img.Bounds().Min.Y); y < min(y1, img.Bounds().Max.Y); y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := img.RGBAAt(x, y)
			inked := true
			for _, g := range ground {
				if near(c, g) {
					inked = false
					break
				}
			}
			if !inked {
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

// TestTheOutlineStepsOnTheColumnsRhythm measures one heading level's step
// off the picture: the leading edge of a level-two title against a
// level-one's, and a level-three's against the level-two's. The column moves
// everything it holds by one distance — the pad a row's ink stands inside its
// fill, the lane its bar stands in — so the outline's step is that same eight
// and not a second rhythm down one narrow list.
func TestTheOutlineStepsOnTheColumnsRhythm(t *testing.T) {
	rowH := asideRowPx(goldenTokens())
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			img, v := asideShot(t, citedModel("guide/Rhythm.md", rhythmSource, 2), tc.colors, 700)
			var lead [3]int
			for i := range lead {
				top := v.geom.outline.Min.Y + i*rowH
				lead[i], _ = asideInkAt(img, tc.colors, top, top+rowH)
				if lead[i] < 0 {
					t.Fatalf("the outline's level-%d row drew no ink to measure", i+1)
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

// TestAnEmptyPaneStandsOnItsRowsAxis requires the line a pane shows instead
// of rows to lead where those rows would have led, and not on the axis of the
// heading above it — a pad outboard of every row in the column reads as an
// annotation on the heading rather than as the pane's own answer.
func TestAnEmptyPaneStandsOnItsRowsAxis(t *testing.T) {
	rowH := asideRowPx(goldenTokens())
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			// A note with headings and citations, and one with neither: the
			// rows, and the lines standing in for them, in the same panes.
			full, fv := asideShot(t, citedModel("guide/Rhythm.md", rhythmSource, 2), tc.colors, 700)
			bare, bv := asideShot(t, citedModel("Sources.md", plainNoteSource, 0), tc.colors, 700)

			head, _ := asideInkAt(bare, tc.colors, 0, bv.geom.outline.Min.Y)
			if head < 0 {
				t.Fatal("the column drew no heading to measure against")
			}
			for _, c := range []struct {
				pane      string
				row, line image.Rectangle
			}{
				{"outline", fv.geom.outline, bv.geom.outline},
				{"backlinks", fv.geom.backlinks, bv.geom.backlinks},
			} {
				rowX, rowY := asideInkAt(full, tc.colors, c.row.Min.Y, c.row.Min.Y+rowH)
				lineX, lineY := asideInkAt(bare, tc.colors, c.line.Min.Y, c.line.Min.Y+rowH)
				if rowX < 0 || lineX < 0 {
					t.Fatalf("the %s pane drew no ink to measure: row at %d, line at %d", c.pane, rowX, lineX)
				}
				// A pixel of slack on each axis, and no more: the line and
				// the row start on different letters, and a letter's own
				// bearings are not the column's placement of it.
				if d := rowX - lineX; d < -1 || d > 1 {
					t.Errorf("the %s pane's rows lead at x=%d and the line standing in for them at x=%d; one leading edge",
						c.pane, rowX, lineX)
				}
				// Down the column, each measured from its own pane's top:
				// the two panes stand at different heights in the two
				// pictures, because a pane is as tall as the rows it has.
				rowDown, lineDown := rowY-c.row.Min.Y, lineY-c.line.Min.Y
				if d := rowDown - lineDown; d < -1 || d > 1 {
					t.Errorf("the %s pane's first row inks %d px down its pane and the line standing in for it %d; the line stands where the row would",
						c.pane, rowDown, lineDown)
				}
				if lineX-head <= 1 {
					t.Errorf("the %s pane's line leads at %d and the heading above it at %d; the line stands where its rows do, not where the head does",
						c.pane, lineX, head)
				}
			}
		})
	}
}

// TestTheColumnsInkTiersPartInBothSchemes measures the three depths of
// ink the column speaks in — its headings and annotations, the outline's
// nested titles, and what a reader is meant to read — and requires each
// to part from the next in either appearance.
//
// The dark scheme is what this is measured for. The neutral ramp's paired
// scales keep a step's job across the two appearances, not its distance from
// the ground: one pair of steps taken in both puts the column's three tiers
// 68.8, 74.9 and 80.9 from the surface in L* on a dark ground against 52.9,
// 64.0 and 86.1 on a light one — three names for very nearly one ink, with
// the heading reading as bright as the row beneath it.
func TestTheColumnsInkTiersPartInBothSchemes(t *testing.T) {
	// The distance two inks must keep to read as two. It is under the
	// smaller of the light scheme's own two gaps, which is the separation
	// this is holding the dark scheme to.
	const partBy = 8.0
	lstar := func(c color.NRGBA) float64 {
		l, _, _ := vgcolor.LabFromNRGBA(c)
		return l
	}
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			tok := goldenTokens()
			tok.col = tc.colors
			inks := asideInks(tok)
			ground := lstar(chromeSurface(tc.colors))
			depth := func(c color.NRGBA) float64 {
				return math.Abs(lstar(c) - ground)
			}
			tiers := []struct {
				name string
				ink  color.NRGBA
			}{
				{"the headings and annotations", inks.quiet},
				{"the outline's nested titles", inks.nested},
				{"what the column is read for", inks.reading},
			}
			for i, tier := range tiers {
				// Every tier is a tier a reader reads, so none of them may
				// drop under the body-text contrast the design system holds
				// its own text to.
				if r := vgcolor.ContrastRatio(tier.ink, chromeSurface(tc.colors)); r < 4.5 {
					t.Errorf("%s reads at %.2f:1 on the column's surface, under 4.5:1", tier.name, r)
				}
				if i == 0 {
					continue
				}
				if d := depth(tier.ink) - depth(tiers[i-1].ink); d < partBy {
					t.Errorf("%s stands %.1f L* from %s, want at least %.1f — the column has three tiers or it has one",
						tier.name, d, tiers[i-1].name, partBy)
				}
			}
		})
	}
}
