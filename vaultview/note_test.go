package main

import (
	"fmt"
	"image"
	"image/color"
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
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	vgcolor "github.com/vibrantgio/theme/color"
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
	arr  arrival
	cur  docCursor
	r    input.Router
	ops  op.Ops
	size image.Point

	propClick widget.Clickable

	// rival is a second focusable target standing for the find field and the
	// folder rail: anything else in the window that can hold the keyboard.
	rival     rivalTag
	takeFocus bool

	// find is the page's own find, and fieldW what its field lays out as —
	// nil in every test that is not about the field itself, which draws
	// nothing and reserves the field's width.
	find   pageFind
	fieldW layout.Widget

	// posted is what the page put on this frame for the loop, reduced into
	// the pad's own model at the end of it, as the window's loop reduces it
	// between two frames. Whether the find stands open is the model's.
	posted []mvu.Message
}

type rivalTag struct{ _ byte }

func newNotePad(t *testing.T, m Model) *notePad {
	t.Helper()
	p := &notePad{
		m:    m,
		size: image.Pt(760, 520),
		tok: themeTokens{
			col:    tokens.PlatformLight,
			typ:    tokens.DefaultTypography,
			sp:     tokens.Spacing,
			den:    tokens.Comfortable,
			shaper: tokens.DefaultTypography.DeterministicShaper(),
		},
	}
	note := m.CurrentNote()
	if note == nil {
		t.Fatal("the fixture model has no current note")
	}
	p.doc = markdown.NewDocument(note.Blocks)
	p.find.post = func(_ layout.Context, msg mvu.Message) { p.posted = append(p.posted, msg) }
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
	layoutNotePage(gtx, p.m, p.tok, &p.propClick, &p.read,
		&p.arr, &p.cur, func(Model, *Note) *markdown.Document { return p.doc }, &p.find, nil)
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
	for _, msg := range p.posted {
		p.m, _ = Update(p.m, msg)
	}
	p.posted = nil
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

// shot renders the column as it stands, so a test can ask where the paint
// stops. The reading keys move a scroll position, and no position value says
// how much blank page is left under the last line — only the pixels do.
func (p *notePad) shot(t *testing.T) *image.RGBA {
	t.Helper()
	return golden.Capture(t, p.size, func(gtx layout.Context) layout.Dimensions {
		return layoutNotePage(gtx, p.m, p.tok, &p.propClick, &p.read,
			&p.arr, &p.cur, func(Model, *Note) *markdown.Document { return p.doc }, &p.find, nil)
	})
}

// blankFoot returns how many rows of bare page stand between the column's
// last painted prose row and the window's bottom edge. The trailing gutter is left
// out of the scan: the scroll indicator lives there and reaches the edge by
// design, and it is the prose the reader measures the margin by.
func (p *notePad) blankFoot(img *image.RGBA) int {
	background := p.tok.col.TextBackground
	for y := p.size.Y - 1; y >= 0; y-- {
		for x := 0; x < p.size.X-noteInsetDp; x++ {
			if c := img.RGBAAt(x, y); c.R != background.R || c.G != background.G || c.B != background.B {
				return p.size.Y - 1 - y
			}
		}
	}
	return p.size.Y
}

// blankHead returns how many rows of bare page stand between the edge the
// document's viewport begins on and its first painted row. That edge is the
// pinned seam where the page has a pinned head — the properties pane — and
// the page's own top inset where it has none, which is every note without
// frontmatter now the trail stands in the band. The trailing gutter is left
// out of the scan for the reason blankFoot leaves it out.
func (p *notePad) blankHead(img *image.RGBA) int {
	background := p.tok.col.TextBackground
	drawn := func(y int) bool {
		for x := 0; x < p.size.X-noteInsetDp; x++ {
			if c := img.RGBAAt(x, y); c.R != background.R || c.G != background.G || c.B != background.B {
				return true
			}
		}
		return false
	}
	y := noteInsetDp
	for seam, at := pinnedSeamRows(img, p.tok.col, 0, p.size.X, 0, p.size.Y), 0; at < p.size.Y; at++ {
		if seam[at] {
			y = at + 1
			break
		}
	}
	n := 0
	for ; y < p.size.Y && !drawn(y); y++ {
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

// TestReadingKeysDoNotStealFromAnotherTarget: Home and End mean the line's
// ends to the find field and the first and last row to the folder rail;
// while either holds the keyboard the document must not answer them, and
// pressing in the document must be enough to get them back.
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

// TestOpeningANoteHandsTheColumnTheKeyboard: choosing a note in the folder
// rail leaves the keyboard on the rail, so the arrival of a note in the
// column is itself the request for it — otherwise a note opened from the
// rail cannot be paged until something in it is clicked.
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

// TestTheEndsHoldAfterAnAnchorLanding: a followed link seats the document at
// its anchor, and the reading keys must still cross it — the seating is a
// scroll position, not a mode.
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

// TestOnlyTheNotesEndSpendsTheFootMargin: part way down a note the column has
// no margin at the foot at all — every row of it carries text. Holding the
// margin back on every frame would leave a strip of bare page under the
// half-cut line the window's edge makes, and cost the reader a margin's worth
// of note on every screen they cross.
func TestOnlyTheNotesEndSpendsTheFootMargin(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()

	// One line box of slack, for the reason noteHeadSlack gives at the other
	// end: where the window's edge cuts inside a line, the rows below that
	// line's glyphs are its descent and its leading. More than a whole line
	// of bare page is a line that would have fitted and was held back.
	slack := int(p.tok.typ.BodyLarge.LineHeight)
	full := false
	for page := 1; page <= 4 && !p.atEnd(); page++ {
		p.press(key.NamePageDown, 0)
		if p.atEnd() {
			break
		}
		switch got := p.blankFoot(p.shot(t)); {
		case got <= slack:
			full = true
		case got >= noteEndSpaceDp:
			t.Fatalf("page %d down the note left %d px of bare page at the foot; the margin belongs to the note's end, not to every screen", page, got)
		}
	}
	if !full {
		t.Error("no screen part way down the note came within a line of the window's bottom edge; the column is not using its full height")
	}
}

// noteHeadSlack is how close to the row's edge a line part way down the note
// must come for the column to count as meeting it. Not zero: where the cut
// falls inside a line's own leading, the topmost rows of the viewport carry
// that line's blank rather than its glyphs, and a few rows of it are the type
// setting rather than a margin.
const noteHeadSlack = 6

// TestOnlyTheNotesStartSpendsTheGapUnderTheRow is the foot margin's mirror at
// the other end of the column. The note opens a gap below the row above it,
// and part way down it spends none: the viewport begins on that row's lower
// edge, so a line scrolling out of the top is cut by the row and vanishes
// under it.
//
// What is asserted is one screen that brings a line hard against the row, not
// a bound on every screen: the note's own rhythm opens blanks wider than the
// page's gap — an ordinary block gap, wider still above a heading — so a page
// boundary landing inside one begins on the note's own surface, and no
// per-screen bound can tell that from a margin held back. A page that did
// hold the gap back could never meet the row's edge on any screen.
func TestOnlyTheNotesStartSpendsTheGapUnderTheRow(t *testing.T) {
	p := newNotePad(t, longNoteModel(-1))
	p.frame()
	if got := p.blankHead(p.shot(t)); got < noteGapDp {
		t.Errorf("at its start the note leaves %d px of page under the row above it, want at least the page's own %d", got, noteGapDp)
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

// TestThePropertiesSlabStandsOnThePage holds the pane to the page it is read
// on: the note's own surface for a fill, inside one hair of the pane's own
// neutral step, which is the treatment the page's other bounded blocks wear.
//
// The pane is found in the pixels rather than asserted at. Its top edge is
// the first row down the page carrying the hairline's tint across most of the
// column's measure — the disclosure row above it draws no fills, so nothing
// else can be first — and its foot is the next such row. What is asserted is
// that the two are far enough apart to be a pane, and that between them the
// page is bare: no band of any other fill, which is what a slab would be.
func TestThePropertiesSlabStandsOnThePage(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()
	if !m.PropsOpen {
		t.Fatal("the model under test folds its properties away; the pane has to be open to be measured")
	}
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderNotePage(shaper, m, tc.colors, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
			img := golden.Capture(t, noteFrameSize, scene(w, tc.bg))
			is := func(c color.RGBA, want color.NRGBA) bool {
				return c.R == want.R && c.G == want.G && c.B == want.B
			}
			// banded reports whether row y carries a run of want wide enough
			// to cross the column's measure — a fill or an edge, never a
			// glyph's antialiasing.
			const wide = 300
			banded := func(y int, want color.NRGBA) bool {
				run := 0
				for x := 0; x < noteFrameSize.X; x++ {
					if !is(img.RGBAAt(x, y), want) {
						run = 0
						continue
					}
					if run++; run >= wide {
						return true
					}
				}
				return false
			}
			hair := vgcolor.Flatten(tc.colors.Separator, tc.colors.TextBackground)
			edges := []int{}
			for y := 0; y < noteFrameSize.Y && len(edges) < 2; y++ {
				if banded(y, hair) && (len(edges) == 0 || y > edges[0]+10) {
					edges = append(edges, y)
				}
			}
			if len(edges) < 2 {
				t.Fatalf("the pane drew %d hairline edges; a bounded block has a head and a foot", len(edges))
			}
			top, bot := edges[0], edges[1]
			if h := bot - top; h < 40 {
				t.Errorf("the pane's edges stand %d px apart; it holds several rows of pairs and cannot be that thin", h)
			}
			// Nothing to check here for a band of the chrome material: on
			// this platform that chrome material IS the content's white in the
			// light appearance, so a band of it inside the pane is a band
			// of the page. What the pane stands on is read below instead.
			// The rows between the pairs carry nothing across the measure, so
			// on the page's own fill most of the pane's height is a band of it.
			// A filled pane would have none.
			page := 0
			for y := top + 2; y < bot-1; y++ {
				if banded(y, tc.colors.TextBackground) {
					page++
				}
			}
			if page < (bot-top)/4 {
				t.Errorf("only %d of the pane's %d rows are the note's page; the pane is filled with something else", page, bot-top)
			}
		})
	}
}

// TestTheNoteReadsAtItsMeasure requires the reading column to stop at its
// measure and to sit at the middle of the region between the rail pane and
// the trailing column, read off the composed window rather than off the
// arithmetic that placed it.
//
// The window is opened wider than the measure needs, which is the only state
// the measure is visible in: at the width the application opens at, the
// region is narrower than the measure and every block simply takes what
// there is.
//
// The region is measured from the picture too — the run of the note page's
// own surface across a row below the document is exactly the span between
// the pane and the trailing column, so a change to either column's width
// moves the expectation with it.
//
// The two margins do not come out equal, and the difference is a constant
// the arrangement forces: the scrollbar occupies its own width out of the
// viewport's trailing edge before the document is handed what is left, so
// the run of blocks centres in a span already short by that much at one
// end. Half of it, and half of the splitter's grab strip, is how far the
// column sits from the middle. The slack below is that width and no more.
func TestTheNoteReadsAtItsMeasure(t *testing.T) {
	size := image.Pt(1600, 900)
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			// The properties pane is a row of the page rather than a
			// block of the document, so it spans the column whatever the
			// document does; closed, the widest thing painted in the
			// region is the document itself.
			m := goldenModel()
			m.PropsOpen = false
			img, g := windowShotAt(t, m, tc.colors, size)
			page := tc.colors.TextBackground

			// The region the note has to itself, taken from the frame's own
			// geometry rather than from the page's colour: on this platform
			// the chrome material IS the content's white in the light
			// appearance, so a run of the page's fill along the window's
			// foot would take in the rail and the trailing column too.
			regionLo, regionHi := g.ContentX, size.X-frameAsideDp-1
			if regionLo >= regionHi {
				t.Fatal("the note page has no region of its own between the rail and the trailing column")
			}

			// The widest run of painted pixels anywhere in the document is
			// the block that reaches the measure — here the fenced code
			// block, which fills its whole width.
			//
			// The scan starts past the rail pane's shadow: the ramp the
			// pane casts falls on the note's surface for its whole reach,
			// and it is the pane's drawing and not the document's. The
			// document's own first column stands at the note's leading
			// inset, which is the same number, so nothing of the page is
			// read out with it.
			scanLo := regionLo + paneShadowReachDp
			// The rows read are the document's own: below the reach of the
			// band's drop shadows — the band's controls stand over this
			// column at both ends and cast onto it, which the platform does
			// not cut off at the band's foot — and above the status bar.
			// The column's own pinned seam runs its whole width, which is
			// wider than any block in the document, so its rows are read out
			// too.
			scanTop, scanBot := g.RowTop+toolbarShadowReachDp, g.FootTop
			seamRows := pinnedSeamRows(img, tc.colors, scanLo, regionHi+1, scanTop, scanBot)
			blockLo, blockHi := -1, -1
			for y := scanTop; y < scanBot; y++ {
				if seamRows[y] {
					continue
				}
				lo, hi := -1, -1
				for x := scanLo; x <= regionHi; x++ {
					if img.RGBAAt(x, y) != rgba(page) {
						if lo < 0 {
							lo = x
						}
						hi = x
					}
				}
				if lo >= 0 && hi-lo > blockHi-blockLo {
					blockLo, blockHi = lo, hi
				}
			}
			if got := blockHi - blockLo + 1; got != noteMeasureDp {
				t.Errorf("the widest block reads %d dp wide; the measure is %d", got, noteMeasureDp)
			}

			slack := float64(scrollbar.FromTokens(tc.colors, tc.colors.TextBackground).Width())
			mid := float64(regionLo+regionHi+1) / 2
			at := float64(blockLo+blockHi+1) / 2
			if at < mid-slack || at > mid+slack {
				t.Errorf("the column centres on %.1f, the region between the rail and the trailing column on %.1f", at, mid)
			}
		})
	}
}

// rgba is the stored form of a token colour, for comparing against pixels
// read back out of a capture.
func rgba(c color.NRGBA) color.RGBA {
	return color.RGBAModel.Convert(c).(color.RGBA)
}

// noteColumnLead answers the leading column of paint in a band of the
// window, measured off the surface the band stands on: the first x at which
// any row between y0 and y1 draws something other than that surface.
func noteColumnLead(img *image.RGBA, surface color.NRGBA, x0, x1, y0, y1 int) int {
	lead := -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1 && (lead < 0 || x < lead); x++ {
			if c := img.RGBAAt(x, y); c.R != surface.R || c.G != surface.G || c.B != surface.B {
				lead = x
				break
			}
		}
	}
	return lead
}

// notePinnedRuleRow finds the row the pinned head's rule is drawn on: the
// one row of the content column drawing the seam's colour at the column's
// own trailing edge. The properties pane's box draws that colour too, but
// only across its own width, where the rule runs the column's.
func notePinnedRuleRow(img *image.RGBA, rule color.NRGBA, x, y0, y1 int) int {
	for y := y0; y < y1; y++ {
		if c := img.RGBAAt(x, y); c.R == rule.R && c.G == rule.G && c.B == rule.B {
			return y
		}
	}
	return -1
}

// TestThePinnedRuleRunsToTheSeam requires the rule under the column's
// pinned head to end where the inspector's seam begins. A seam is one
// continuous line, so a rule that stopped the gap's width short of it would
// put two edges where the window has one.
func TestThePinnedRuleRunsToTheSeam(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			img, g := windowShot(t, goldenModel(), tc.colors)
			rule := vgcolor.Flatten(tc.colors.Separator, tc.colors.TextBackground)
			y := notePinnedRuleRow(img, rule, g.Main.Max.X-1, g.RowTop, g.FootTop)
			if y < 0 {
				t.Fatal("no row of the content column carries the pinned head's rule")
			}
			end := g.ContentX + paneShadowReachDp
			for ; end < windowW; end++ {
				if c := img.RGBAAt(end, y); c.R != rule.R || c.G != rule.G || c.B != rule.B {
					break
				}
			}
			// The seam wears the same colour as the rule in the light
			// scheme, which is the point of the reading: the two are one
			// line. So the rule must cover every column up to the seam, and
			// stop inside it rather than running on into the inspector.
			if end < g.Inspector.Min.X {
				t.Errorf("the rule ends at x=%d and the inspector's seam stands at x=%d; the rule runs to the seam",
					end, g.Inspector.Min.X)
			}
			if end > g.Inspector.Min.X+seamDp {
				t.Errorf("the rule reaches x=%d, past the seam at x=%d; it ends on the seam", end, g.Inspector.Min.X)
			}
		})
	}
}

// TestTheContentColumnStandsOnOneLeadingEdge reads the three rows of the
// content column that carry text — the properties pane, the document's
// blocks and the count in the foot — and requires all three to begin at the
// same x, in a column at the width the window opens at and in one wide
// enough that the document centres its measure. Content inside one region
// aligns on one leading edge.
func TestTheContentColumnStandsOnOneLeadingEdge(t *testing.T) {
	for _, size := range []image.Point{windowFrameSize, {X: 1500, Y: windowH}} {
		for _, tc := range themeCases {
			t.Run(fmt.Sprintf("%dx%d/%s", size.X, size.Y, tc.name), func(t *testing.T) {
				img, g := windowShotAt(t, goldenModel(), tc.colors, size)
				page := tc.colors.TextBackground
				rule := vgcolor.Flatten(tc.colors.Separator, page)
				// The reading starts clear of the pane's shadow, which
				// reaches across the content column's leading edge.
				x0 := g.ContentX + paneShadowReachDp
				ruleY := notePinnedRuleRow(img, rule, g.Main.Max.X-1, g.RowTop, g.FootTop)
				if ruleY < 0 {
					t.Fatal("no row of the content column carries the pinned head's rule")
				}
				// Below the toolbar's own shadow, which falls across the
				// whole width of the rows under it.
				props := noteColumnLead(img, page, x0, g.Main.Max.X, g.RowTop+noteInsetDp, ruleY)
				doc := noteColumnLead(img, page, x0, g.Main.Max.X, ruleY+1, g.Main.Max.Y)
				foot := noteColumnLead(img, page, x0, g.Main.Max.X, g.FootTop, size.Y)
				if props < 0 || doc < 0 || foot < 0 {
					t.Fatalf("one of the three rows drew nothing to measure: properties %d, document %d, count %d",
						props, doc, foot)
				}
				// A pixel of slack and no more: the three begin on
				// different letters, and a letter's own bearings are not
				// the column's placement of it.
				if d := doc - props; d < -1 || d > 1 {
					t.Errorf("the properties pane leads at x=%d and the document's blocks at x=%d; one leading edge", props, doc)
				}
				if d := foot - props; d < -1 || d > 1 {
					t.Errorf("the properties pane leads at x=%d and the count in the foot at x=%d; one leading edge", props, foot)
				}
			})
		}
	}
}

// TestAWideColumnCentresTheMeasure holds the rule the edge above is one of:
// a column wider than the reading measure and the page's inset on both sides
// centres the measure between the column's edges, and a narrower column keeps
// the page's inset on both sides and gives the text what is left. The numbers
// are read off the column's own placement, which is what every row of the
// column is laid out from.
func TestAWideColumnCentresTheMeasure(t *testing.T) {
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(windowW, windowH)),
		Ops:         new(op.Ops),
	}
	tok := goldenTokens()
	if lead, width := noteTextBox(gtx, tok, 526); lead != noteInsetDp || width != 526-2*noteInsetDp {
		t.Errorf("a 526 dp column places its text at x=%d over %d dp; want the page's inset %d over what is left",
			lead, width, noteInsetDp)
	}
	lead, width := noteTextBox(gtx, tok, 774)
	if width != noteMeasureDp {
		t.Errorf("a 774 dp column lays its text out over %d dp, want the measure %d", width, noteMeasureDp)
	}
	if lead <= noteInsetDp {
		t.Errorf("a 774 dp column places its text at x=%d; a column wider than the measure centres it past the inset %d",
			lead, noteInsetDp)
	}
	if trail := 774 - lead - width; trail-lead > 1 || lead-trail > 1 {
		t.Errorf("a 774 dp column leaves %d dp before its text and %d after it; the measure is centred in the column",
			lead, trail)
	}
	for _, colW := range []int{noteMeasureDp + noteInsetDp, noteMeasureDp + 2*noteInsetDp - 1} {
		if lead, width := noteTextBox(gtx, tok, colW); lead != noteInsetDp || width != colW-2*noteInsetDp {
			t.Errorf("a %d dp column places its text at x=%d over %d dp; want the page's inset %d on both sides",
				colW, lead, width, noteInsetDp)
		}
	}
}
