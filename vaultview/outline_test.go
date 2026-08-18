package main

import (
	"fmt"
	"image"
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
	"github.com/vibrantgio/components/scrollbar"
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
// meeting without overlapping, and neither pane standing taller than what
// it has to put in it.
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
// region it holds, a press still finds the row it landed on — the rows
// lead their region, so they are counted from its top edge as before — a
// press in the paper below the last row moves nothing, because the slack
// is room and not a target, and the pane at the foot answers its own rows
// where they now stand.
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

// TestEachScrollingPaneShowsItsIndicator is D6 in pixels: a pane with more
// rows than it can show draws the indicator in its own trailing gutter, and
// a pane whose rows all fit draws nothing there. Both panes, one column —
// the outline scrolling and the backlinks capped past their cap.
func TestEachScrollingPaneShowsItsIndicator(t *testing.T) {
	tok := goldenTokens()
	ground := tok.col.Surface
	shot := func(m Model, colH int) (*image.RGBA, asideGeom) {
		v := newAsideView(&docCursor{})
		size := image.Pt(frameAsideDp, colH)
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, ground, clip.Rect{Max: size}.Op())
			return v.layout(gtx, m, tok)
		})
		return img, v.geom
	}
	// The strip scanned is the thumb's own columns: the bar floats over the
	// rows here, so the pane's last dp carry row ink as well — but a row's
	// text and its mark both stop a row inset short of the edge, and the
	// thumb is drawn inside that. Anything in these columns is the bar.
	bar := scrollbar.FromTokens(tok.col)
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
	ground := tok.col.Surface
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
