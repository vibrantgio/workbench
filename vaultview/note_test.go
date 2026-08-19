package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/theme/tokens"
)

// notePad drives the note column through a real input router, so the keys
// under test travel the path they travel in the running app: queued at the
// window, matched against the filters the column registered last frame,
// delivered only if the column holds the keyboard.
type notePad struct {
	m    Model
	tok  themeTokens
	doc  *markdown.Document
	read reader
	cur  docCursor
	r    input.Router
	ops  op.Ops
	size image.Point

	propClick, backClick, fwdClick widget.Clickable
	trail                          breadcrumb.TrailLayout

	// rival is a second focusable target standing for the find field and the
	// folder rail: anything else in the window that can hold the keyboard.
	rival     rivalTag
	takeFocus bool
}

type rivalTag struct{ _ byte }

func newNotePad(t *testing.T, m Model) *notePad {
	t.Helper()
	p := &notePad{
		m:    m,
		size: image.Pt(760, 520),
		tok: themeTokens{
			col:    tokens.DefaultLight,
			typ:    tokens.DefaultTypography,
			sp:     tokens.Spacing,
			den:    tokens.Comfortable,
			shaper: tokens.DefaultTypography.DeterministicShaper(),
		},
	}
	p.trail = breadcrumb.NewTrail(p.tok.shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
		p.tok.col, p.tok.sp, p.tok.typ.TitleSmall)
	note := m.CurrentNote()
	if note == nil {
		t.Fatal("the fixture model has no current note")
	}
	p.doc = markdown.NewDocument(note.Blocks)
	return p
}

func (p *notePad) frame() {
	p.ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(p.size),
		Ops:         &p.ops,
		Source:      p.r.Source(),
	}
	layoutNotePage(gtx, p.m, p.tok, &p.propClick, &p.backClick, &p.fwdClick, p.trail, &p.read,
		&p.cur, func(Model, *Note) *markdown.Document { return p.doc })
	// The rival is registered over nothing, in the corner: it exists to hold
	// the keyboard, not to be seen.
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &p.rival}); !ok {
			break
		}
	}
	area := clip.Rect{Max: image.Pt(1, 1)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &p.rival)
	area.Pop()
	if p.takeFocus {
		p.takeFocus = false
		gtx.Execute(key.FocusCmd{Tag: &p.rival})
	}
	p.r.Frame(&p.ops)
}

func (p *notePad) press(name key.Name, mods key.Modifiers) {
	p.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	p.frame()
}

// clickInDocument presses near the foot of the column, where the document is
// and no control of the page's own is.
func (p *notePad) clickInDocument() {
	at := f32.Pt(float32(p.size.X)/2, float32(p.size.Y)-30)
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	p.frame()
}

// shot renders the column as it stands, so a test can ask where the ink
// stops. The reading keys move a scroll position, and no position value says
// how much paper is left under the last line — only the pixels do.
func (p *notePad) shot(t *testing.T) *image.RGBA {
	t.Helper()
	return golden.Capture(t, p.size, func(gtx layout.Context) layout.Dimensions {
		return layoutNotePage(gtx, p.m, p.tok, &p.propClick, &p.backClick, &p.fwdClick, p.trail, &p.read,
			&p.cur, func(Model, *Note) *markdown.Document { return p.doc })
	})
}

// blankFoot returns how many rows of bare paper stand between the column's
// last prose ink and the window's bottom edge. The trailing gutter is left
// out of the scan: the scroll indicator lives there and reaches the edge by
// design, and it is the prose the reader measures the margin by.
func (p *notePad) blankFoot(img *image.RGBA) int {
	ground := p.tok.col.Background
	for y := p.size.Y - 1; y >= 0; y-- {
		for x := 0; x < p.size.X-noteInsetDp; x++ {
			if c := img.RGBAAt(x, y); c.R != ground.R || c.G != ground.G || c.B != ground.B {
				return p.size.Y - 1 - y
			}
		}
	}
	return p.size.Y
}

// blankHead returns how many rows of bare paper stand between the row above
// the document — the breadcrumb row, in the models these tests use — and the
// document's first ink. It reads the image the way the reader does: the first
// band of ink from the top is that row, and what follows it is the paper the
// document's viewport begins on. The trailing gutter is left out of the scan
// for the reason blankFoot leaves it out.
func (p *notePad) blankHead(img *image.RGBA) int {
	ground := p.tok.col.Background
	ink := func(y int) bool {
		for x := 0; x < p.size.X-noteInsetDp; x++ {
			if c := img.RGBAAt(x, y); c.R != ground.R || c.G != ground.G || c.B != ground.B {
				return true
			}
		}
		return false
	}
	y := 0
	for ; y < p.size.Y && !ink(y); y++ { // the paper above the row
	}
	for ; y < p.size.Y && ink(y); y++ { // the row's own ink
	}
	n := 0
	for ; y < p.size.Y && !ink(y); y++ {
		n++
	}
	return n
}

func (p *notePad) pos() layout.Position { return p.doc.Position() }

// atEnd reports whether the document has come to rest: the last block is laid
// out and its trailing edge is at or above the viewport's.
func (p *notePad) atEnd() bool {
	q := p.pos()
	return q.First+q.Count == len(p.doc.Blocks()) && q.OffsetLast >= 0
}

func (p *notePad) atStart() bool {
	q := p.pos()
	return q.First == 0 && q.Offset == 0
}

// TestReadingKeysMoveTheNote is the wiring assertion: each of the platform's
// reading keys, pressed at the window, moves the document the note column is
// showing — no click first, because a reader who has just opened a note has
// not clicked anything.
func TestReadingKeysMoveTheNote(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()
	if !p.atStart() {
		t.Fatalf("the note opened at %+v, want the top", p.pos())
	}

	p.press(key.NamePageDown, 0)
	down := p.pos()
	if down.First == 0 && down.Offset == 0 {
		t.Fatal("Page Down did not move the note")
	}

	p.press(key.NamePageUp, 0)
	if !p.atStart() {
		t.Fatalf("Page Up left the note at %+v, want the top", p.pos())
	}

	for _, tc := range []struct {
		name string
		key  key.Name
		mods key.Modifiers
	}{
		{"End", key.NameEnd, 0},
		{"Command-Down", key.NameDownArrow, key.ModCommand},
	} {
		p.doc.ScrollToStart()
		p.frame()
		p.press(tc.key, tc.mods)
		q := p.pos()
		if q.First+q.Count != len(p.doc.Blocks()) {
			t.Errorf("%s left the note at %+v, want the end", tc.name, q)
		}
		for _, back := range []struct {
			name string
			key  key.Name
			mods key.Modifiers
		}{{"Home", key.NameHome, 0}, {"Command-Up", key.NameUpArrow, key.ModCommand}} {
			p.press(back.key, back.mods)
			if !p.atStart() {
				t.Errorf("%s after %s left the note at %+v, want the top", back.name, tc.name, p.pos())
			}
			p.press(tc.key, tc.mods)
		}
	}
}

// TestReadingKeysDoNotStealFromAnotherTarget is D2's other half. Home and End
// mean the line's ends to the find field and the first and last row to the
// folder rail; while either holds the keyboard the document must not answer
// them, and pressing in the document must be enough to get them back.
func TestReadingKeysDoNotStealFromAnotherTarget(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()
	p.takeFocus = true
	p.frame()

	for _, tc := range []struct {
		name string
		key  key.Name
		mods key.Modifiers
	}{
		{"Page Down", key.NamePageDown, 0},
		{"End", key.NameEnd, 0},
		{"Command-Down", key.NameDownArrow, key.ModCommand},
	} {
		p.press(tc.key, tc.mods)
		if !p.atStart() {
			t.Fatalf("%s moved the note to %+v while another target held the keyboard", tc.name, p.pos())
		}
	}

	p.clickInDocument()
	p.press(key.NamePageDown, 0)
	if p.atStart() {
		t.Fatal("pressing in the document did not hand it the keyboard back: Page Down still moves nothing")
	}
}

// TestOpeningANoteHandsTheColumnTheKeyboard is the other way back. Choosing a
// note in the folder rail leaves the keyboard on the rail, and a reader who
// has just opened a note wants to read it — so the arrival of a note is
// itself the request. Without this the only way to page a note opened from
// the rail is to find somewhere in it to click first.
func TestOpeningANoteHandsTheColumnTheKeyboard(t *testing.T) {
	m := longNoteModel(-1)
	p := newNotePad(t, m)
	p.frame()
	p.takeFocus = true
	p.frame()
	p.press(key.NamePageDown, 0)
	if !p.atStart() {
		t.Fatalf("the fixture's rival never held the keyboard: the note moved to %+v", p.pos())
	}

	// A note opened from the rail is a new document in the column.
	p.doc = markdown.NewDocument(m.CurrentNote().Blocks)
	p.frame()
	p.press(key.NamePageDown, 0)
	if p.atStart() {
		t.Fatal("opening a note left the keyboard elsewhere: Page Down moves nothing")
	}
}

// TestTheEndsHoldAfterAnAnchorLanding is the exit condition's second half: a
// followed link seats the document at its anchor, and the reading keys must
// still cross it — the seating is a scroll position, not a mode.
func TestTheEndsHoldAfterAnAnchorLanding(t *testing.T) {
	m := longNoteModel(30)
	p := newNotePad(t, m)
	note := m.CurrentNote()
	p.doc = markdown.NewDocumentAt(note.Blocks, m.CurAnchor)
	p.frame()
	if p.atStart() {
		t.Fatalf("the anchored landing sat at the top; the fixture cannot say anything")
	}
	p.press(key.NameHome, 0)
	if !p.atStart() {
		t.Errorf("Home after an anchor landing left the note at %+v", p.pos())
	}
	p.press(key.NameEnd, 0)
	if q := p.pos(); q.First+q.Count != len(p.doc.Blocks()) {
		t.Errorf("End after an anchor landing left the note at %+v, want the end", q)
	}
}

// TestTheNoteRestsClearOfTheWindowsBottomEdge is the reading surface's foot
// margin, in pixels: a note scrolled as far as it goes stops well short of the
// window, so the last line reads as the end of the note rather than as the
// window running out of room.
func TestTheNoteRestsClearOfTheWindowsBottomEdge(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()
	p.press(key.NameEnd, 0)
	if !p.atEnd() {
		t.Fatalf("End left the note at %+v, want its end", p.pos())
	}
	if got := p.blankFoot(p.shot(t)); got < noteEndSpaceDp {
		t.Errorf("the last line rests %d px above the window's bottom edge, want at least %d", got, noteEndSpaceDp)
	}
}

// TestOnlyTheNotesEndSpendsTheFootMargin is the other half of the same
// decision. Part way down a note the column has no margin at the foot at all:
// every row of it carries text, and a line the window's edge cuts is the
// window cutting it. Holding the margin back on every frame would leave a
// strip of bare paper under that half-cut line, which reads as a clipping
// fault, and it would cost the reader a margin's worth of note on every screen
// they cross.
func TestOnlyTheNotesEndSpendsTheFootMargin(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()

	full := false
	for page := 1; page <= 4 && !p.atEnd(); page++ {
		p.press(key.NamePageDown, 0)
		if p.atEnd() {
			break
		}
		switch got := p.blankFoot(p.shot(t)); {
		case got == 0:
			full = true
		case got >= noteEndSpaceDp:
			t.Fatalf("page %d down the note left %d px of bare paper at the foot; the margin belongs to the note's end, not to every screen", page, got)
		}
	}
	if !full {
		t.Error("no screen part way down the note reached the window's bottom edge; the column is not using its full height")
	}
}

// noteHeadSlack is how close to the row's edge a line part way down the note
// must come for the column to count as meeting it. Not zero: where the cut
// falls inside a line's own leading, the topmost rows of the viewport carry
// that line's blank rather than its ink, and a few rows of it are the type
// setting rather than a margin.
const noteHeadSlack = 6

// TestOnlyTheNotesStartSpendsTheGapUnderTheRow is the foot margin's mirror at
// the other end of the column. The note opens a gap below the row above it,
// and part way down it spends none: the viewport begins on that row's lower
// edge, so a line scrolling out of the top is cut by the row and vanishes
// under it. A gap held back on every frame would leave a strip of bare paper
// over that half-cut line — which is what the reader reported seeing, and what
// reads as a rendering fault rather than as scrolling.
//
// The evidence is a screen that brings a line hard against the row, not a
// bound on every screen: the note's own rhythm now opens blanks wider than
// the page's gap — an ordinary block gap, wider still above a heading — so a
// page boundary landing inside one begins on the note's own paper, and no
// per-screen bound can tell that from a margin held back. A page that did
// hold the gap back could never reach the row's edge on any screen, which is
// exactly what this measures.
func TestOnlyTheNotesStartSpendsTheGapUnderTheRow(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()
	if got := p.blankHead(p.shot(t)); got < noteGapDp {
		t.Errorf("at its start the note leaves %d px of paper under the row above it, want at least the page's own %d", got, noteGapDp)
	}

	met := false
	for page := 1; page <= 8 && !p.atEnd() && !met; page++ {
		p.press(key.NamePageDown, 0)
		if p.atEnd() {
			break
		}
		if got := p.blankHead(p.shot(t)); got <= noteHeadSlack {
			met = true
		}
	}
	if !met {
		t.Errorf("no screen part way down the note brought a line up against the row's edge; the column is not using its full height")
	}
}

// TestTheKeyboardLandsOnTheRestingPosition: the end the End key reaches is the
// end paging arrives at and the end the note stays at. A key that stopped
// somewhere else would put the note's foot margin at one height for the reader
// who paged there and another for the reader who jumped.
func TestTheKeyboardLandsOnTheRestingPosition(t *testing.T) {
	paged := newNotePad(t, longNoteModel(-1))
	paged.frame()
	for i := 0; i < 200 && !paged.atEnd(); i++ {
		paged.press(key.NamePageDown, 0)
	}
	if !paged.atEnd() {
		t.Fatalf("paging down 200 times never reached the end: %+v", paged.pos())
	}

	for _, tc := range []struct {
		name string
		key  key.Name
		mods key.Modifiers
	}{
		{"End", key.NameEnd, 0},
		{"Command-Down", key.NameDownArrow, key.ModCommand},
	} {
		jumped := newNotePad(t, longNoteModel(-1))
		jumped.frame()
		jumped.press(tc.key, tc.mods)
		if a, b := paged.pos(), jumped.pos(); a.First != b.First || a.Offset != b.Offset {
			t.Errorf("%s landed at %+v, paging to the end at %+v", tc.name, b, a)
		}
		// The end is a place the note stays: another frame must not drift, or
		// the foot margin would creep after the reader stopped.
		before := jumped.pos()
		jumped.frame()
		if got := jumped.pos(); got.First != before.First || got.Offset != before.Offset {
			t.Errorf("after %s the note drifted from %+v to %+v on the next frame", tc.name, before, got)
		}
	}
}
