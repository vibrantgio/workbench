// note.go is the vault screen's note column — the main slot of the
// composition frame.go builds, with the folder tree leading and the
// backlinks panel trailing. The column renders the current note: a header row
// with back/forward and the breadcrumb, a collapsible properties panel fed by
// the frontmatter split,
// and the parsed body as a markdown Document. Wikilink clicks resolve
// against the index and navigate; a task checkbox writes its marker in
// the file before the click returns; web links open the system browser;
// a link matching several notes raises the chooser, and every other
// refusal surfaces as a toast.

package main

import (
	"image"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/icons"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/patterns/toast"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Note-page layout constants.
const (
	noteInsetDp = 24
	// noteEndSpaceDp is how far above the foot of its own column a note
	// comes to rest once it is scrolled as far as it goes — that foot being
	// where the window's status bar starts, a band further in than the
	// glass. It is the document's foot margin, and like a printed page's it
	// is set wider than the reading margin the column keeps at its sides, so
	// that the text reads as having ended rather than as having been cut off
	// by the window.
	//
	// The amount is measured, not chosen: on this display, at one pixel per
	// dp, a note read to its end in the reading surfaces this viewer is
	// judged beside rests its last line about fifty px above the foot of
	// the surface it is read in — the window's bottom edge, on the ones
	// with nothing below the text. Forty dp, with the last block's own
	// closing gap on top of it, puts this one at forty-eight above its own
	// column's foot, which is the bar. A plain text view — the platform's
	// own, with no reading margins at any edge — rests its last line five px
	// up instead, which is the same answer for a surface that is not
	// designed to be read at length.
	//
	// It is spent at the document's end and nowhere else. Part way down a
	// note every row of the column carries text, and a line cut by the
	// window's edge is the window cutting it; a margin held back on every
	// frame would leave a strip of blank paper under that half-cut line.
	noteEndSpaceDp = 40
	// noteGapDp is the page's own rhythm: the space between the rows above
	// the document, and the space the document rests below the last of them.
	// The second is spent inside the document rather than by the page, for
	// the reason the end space is.
	noteGapDp = 16

	// The properties panel's rhythm: the gap between one metadata row and
	// the next — and between the disclosure head and the box under it — the
	// pad inside that box, and the gap between the key column and the
	// values.
	//
	// The row gap is measured against the reading app this viewer is judged
	// beside, which sets its metadata on a twenty-one px line box and spends
	// eleven px between rows; this panel sets on a twenty px box, already
	// tighter than the thing being copied. Four is one stop down the spacing
	// scale — six is not a stop on the scale at all — and lands the pitch at
	// twenty-four against the reference's thirty-two.
	//
	// The padding is where a box costs what a list does not: the reference
	// draws no box round its metadata and spends nothing here, and twelve
	// is the pad the note's code fences take, which is a screenful of code
	// and not three lines of key and value. Eight is one stop under it.
	//
	// Measured off the rendered page, this rhythm holds a three-field panel
	// to eighty-four px — fourteen less than the same panel one stop up the
	// scale, which stands ninety-six and pushes the note's title from row 189
	// of a seven hundred px viewport down to row 203, twenty-seven per cent
	// of the window against twenty-nine.
	propRowGapDp = 4
	propPadDp    = 8
	propKeyGapDp = 16

	// propEdgeDp is the panel's hairline and propEdgeStep the neutral step
	// it is drawn in: one past the separator's.
	//
	// A block that takes the paper for its ground has its edge for a
	// channel and nothing else, and on the dark page one hair of the
	// separator's tint is not channel enough: the line reads 1.31:1 off
	// that paper, and at one device pixel per dp the box dissolves into it.
	// A step further up the neutral ramp the same hair reads 1.91:1 in the
	// dark scheme and 1.88:1 in the light — twice the eight-bit distance
	// from the paper, forty-seven levels against twenty-two in the dark —
	// while staying far under the ink it bounds (6.19:1 and 11.06:1), which
	// is the one thing an edge may not out-read.
	//
	// A faint fill would be the second channel the dark box wants, and
	// there is no fill to spend: the neutral ramp's first step IS the paper
	// in both schemes, and its second is the fill the window's rail and
	// aside wear — 1.13:1 off the light page against the code blocks'
	// 1.05:1, which would make the note's metadata heavier than the code it
	// has to stay quieter than, wearing the chrome's own colour to do it.
	// Nothing lies between the two, so the edge carries the whole of the
	// channel and the panel keeps its paper.
	propEdgeDp   = 1
	propEdgeStep = 400

	// propLabelStep is the neutral step the properties panel writes its
	// quiet ink at: the field keys, the disclosure head above them, and the
	// raw block a frontmatter too odd to split falls back to. The values
	// beside them are read a step stronger, so this is the panel's muted
	// tier and has the body floor to clear on the panel's own ground and
	// not on the page's.
	//
	// It is a measurement. On the paper the panel stands on, the step reads
	// 6.19:1 in the light scheme and 11.06:1 in the dark, both clear of the
	// 4.5:1 the design system holds body-sized text to; the step below it
	// reads 4.03:1 in the light scheme, under the floor, so 700 is the
	// quietest step this panel can be written in. On a heavier fill the same
	// step measures 4.51:1 — over the floor by a hundredth, a floor touched
	// rather than cleared, which is why the ink is floored against the
	// panel's own ground.
	propLabelStep = 700

	// propValueStep is the step the field values are read at: one under the
	// prose, which is where the note's own text is written.
	//
	// A value at the body ink itself — the very step the note's title and
	// every paragraph below it are set in — would stand level with the
	// note's own words while sitting above them on the page, landing the eye
	// on the metadata before the note. One step down, a value reads 9.30:1
	// on the panel's ground in the light scheme and 13.07:1 in the dark,
	// both far clear of the 4.5:1 body floor, and still comfortably over the
	// keys beside it — 6.19:1 and 11.06:1 — because the value is the content
	// of its row and stays the stronger of the pair.
	propValueStep = 800

	// propHeadWeight is what tells the panel's disclosure head from the
	// field keys under it.
	//
	// At the keys' own step, size and weight the head reads as one more
	// label in the column of labels it opens. The reading app this viewer is
	// judged beside separates the two by ink: its head is set at the prose
	// ink its values are, and its keys a measured step under both — 218
	// against 179 on a 28 paper. That axis is already spent here, on the
	// order between the values and the keys, so the head takes the other
	// axis it has: the bold the note's own headings take, at the same size
	// and the same step as the keys. Letter-spacing is not available — the
	// label helper this window draws with sets no tracking.
	propHeadWeight = tokens.WeightBold

	// noteNavMarkDp sizes the two history controls and propMarkDp the
	// properties panel's disclosure; both take the size a mark takes next
	// to a line of text. The history controls sit in the head row beside
	// the breadcrumb, so that is the size they belong at: a chevron is a
	// diagonal spanning the whole of its square, and at the size a mark
	// takes as a control in its own right its ink stands half again over
	// the caps of the label it serves. The row centres its children, so
	// the smaller square costs the row no height and moves nothing else.
	noteNavMarkDp = markSmallDp
	propMarkDp    = markSmallDp

	// noteNavInkStep and noteNavDimStep are the neutral steps the two
	// history controls take. Navigation chrome reads under the text it
	// stands beside rather than at that text's own ink, so the enabled
	// control takes a step short of the body ink; the reference reading
	// app mutes its own history arrows further still, to about a quarter
	// of its title's contrast, but the dim step has to stay clearly below
	// the enabled one and the neutral ramp's dark scale leaves no room
	// under a quarter for it. So the enabled ink is muted as far as the
	// end-of-stack ink can follow: the two steps read a third of the scale
	// apart in both appearances.
	noteNavInkStep = 600
	noteNavDimStep = 300
)

// noteCodeBases are the syntax palettes a note's fences are drawn in, one per
// appearance: the ones the kept theme names when it names ones this build can
// resolve, and the highlighter's own defaults otherwise — a theme with no base
// in it, or one naming a style file that has since left the styles folder,
// draws code exactly as it did for somebody who never chose.
//
// A pair and not a name because a syntax palette is fitted to a ground: the
// set of inks balanced against a near-white page is not the set anybody would
// balance against a near-black one. So the light appearance and the dark one
// each wear their own member, ground and all, and a desktop switching between
// them switches the code's plate with everything else.
//
// They are set once, at startup, from the same kept theme the palette comes
// from, and read from then on. The choice is a preference and not a mode:
// there is no affordance in this window to change it, because the window
// that chooses a theme is the one that keeps it.
var noteCodeBases = highlight.DefaultBases()

// adoptCodeBases resolves the syntax bases a kept theme asks for. The styles
// folder is read first, since a kept base may name a style somebody wrote
// themselves; a name that cannot be resolved, or one fitted to the appearance
// it is not kept for, falls back to that appearance's default rather than
// failing. What the folder could not read is not surfaced here — this window
// shows notes, and the place a style file gets fixed is the window that
// offered it.
func adoptCodeBases(kept brand.Brand) highlight.BasePair {
	if dir, err := brand.StylesDir(); err == nil {
		highlight.LoadDir(dir)
	}
	return highlight.BasesOrDefault(kept.Base.Names())
}

// noteStyle derives the markdown document style for the current tokens: the
// token-themed defaults, the reading typeface for code, and a fenced block
// wearing the chosen syntax base. The link hook is attached per frame in
// layoutNotePage, where the model is at hand.
//
// The base is worn rather than re-fitted. A fence takes the ground its author
// drew their inks on and the inks as they were drawn, so a block in a note is
// the palette itself and not a rendering of it; the page around it — the
// prose, the chip an inline span sits on, the bar a wide block scrolls under
// — stays this theme's. Which member of the pair reaches the fence follows
// the tokens, so a change of appearance is a change of plate.
//
// There is no memo: wearing resolves a name and reads four colours off it,
// which is a map lookup cheap enough for a path that runs every frame.
func noteStyle(c tokens.ColorTokens, typ tokens.Typography) markdown.Style {
	st := markdown.FromTokens(c, typ)
	st.Mono = font.Typeface(typ.Code.Typeface)
	st.CodeSize = unit.Sp(typ.Code.Size)
	highlight.WearPair(&st, noteCodeBases, c)
	return st
}

// docEntry is one cached document and the Note value it was built from.
type docEntry struct {
	note *Note
	doc  *markdown.Document
}

// readerTag is a non-zero-size type so its address is a unique event tag.
// A zero-size struct{} field could share an address with its neighbour,
// which would break tag identity.
type readerTag struct{ _ byte }

// reader is the note column's keyboard target: one focus tag covering the
// document, and the platform's reading keys filtered on it.
//
// The tag is what keeps the keys the reader's own. A filter with no focus
// would match whoever is typing in the find field and whatever the folder
// rail has selected — Home and End mean the line's ends to an editor and the
// first and last row to a list, and all three would answer the same press.
// Filtered on this tag, the keys move the document only while the document
// holds the keyboard, and the field and the rail keep theirs untouched.
//
// The column takes the keyboard whenever a note arrives — the first one it
// shows, and every one opened after it — so a note can be read the moment it
// is opened rather than after finding somewhere to click. Choosing a note
// from the rail or following a link is a request to read it, and the keys
// that read it are the document's; filtering the rail opens nothing and so
// takes nothing away from the field being typed in.
type reader struct {
	tag readerTag
	// shown is the document the column last laid out. A different pointer
	// means a different note is on screen, which is the moment to claim the
	// keyboard.
	shown *markdown.Document
}

// layout lays w out and covers it with the reading area: one focus target
// over the whole document, driven by whatever keys reached it this frame.
//
// The area sits over the content rather than under it, and passes pointer
// events through. Under it, the document's own links and the list's scroll
// gesture would swallow the press before it arrived, and a click on prose is
// how a reader says which pane they mean to read; passing through is what
// lets the same press both follow a link and hand the column the keyboard.
func (r *reader) layout(gtx layout.Context, doc *markdown.Document, w layout.Widget) layout.Dimensions {
	r.process(gtx, doc)
	if r.shown != doc {
		r.shown = doc
		gtx.Execute(key.FocusCmd{Tag: &r.tag})
	}
	dims := w(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &r.tag)
	area.Pop()
	pass.Pop()
	return dims
}

// process drains this frame's reading keys. The focus filter is what makes
// the tag focusable at all: without it the router refuses to hold focus on
// it and every key filter below is dead.
//
// Command with an arrow is the macOS spelling of the document's ends, and
// Home and End are the same two places; the platform's own text views answer
// both, so the viewer answers both rather than inventing a third.
func (r *reader) process(gtx layout.Context, doc *markdown.Document) {
	tag := &r.tag
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: tag},
			pointer.Filter{Target: tag, Kinds: pointer.Press},
			key.Filter{Focus: tag, Name: key.NamePageUp},
			key.Filter{Focus: tag, Name: key.NamePageDown},
			key.Filter{Focus: tag, Name: key.NameHome},
			key.Filter{Focus: tag, Name: key.NameEnd},
			key.Filter{Focus: tag, Name: key.NameUpArrow, Required: key.ModCommand},
			key.Filter{Focus: tag, Name: key.NameDownArrow, Required: key.ModCommand},
		)
		if !ok {
			return
		}
		switch e := e.(type) {
		case pointer.Event:
			if e.Kind == pointer.Press {
				gtx.Execute(key.FocusCmd{Tag: tag})
			}
		case key.Event:
			if e.State != key.Press || doc == nil {
				continue
			}
			switch e.Name {
			case key.NamePageDown:
				doc.PageDown()
			case key.NamePageUp:
				doc.PageUp()
			case key.NameEnd, key.NameDownArrow:
				doc.ScrollToEnd()
			case key.NameHome, key.NameUpArrow:
				doc.ScrollToStart()
			}
		}
	}
}

// vaultLayer composes the vault screen: the window frame with the folder
// tree in the leading column, the backlinks panel in the trailing one,
// and the note between them. The note column reads the model and token
// snapshots at frame time; repaints on model change are driven by the
// routed layer's re-emission.
func vaultLayer(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	// Documents are cached per note path and reused on every frame, so
	// each note's scroll position and richtext interaction state survive
	// revisiting. A landing that carries an anchor (NavSeq moved and
	// CurAnchor is set) re-creates the target's document seated at the
	// anchor's block index; every other visit keeps the cached viewport.
	// The cached document also remembers which Note value it was built
	// from: notes are never mutated, so a different pointer at the same
	// path means the file was read again and the document must be rebuilt
	// — without that, a note edited outside the app would reload into a
	// viewport still showing the old blocks. The outgoing viewport is
	// copied onto the new document so the reader does not jump. All state
	// here is touched only on the frame goroutine.
	var (
		docs      = map[string]docEntry{}
		docsVault string
		seatedSeq int
		propClick widget.Clickable
		backClick widget.Clickable
		fwdClick  widget.Clickable
		read      reader
	)
	docFor := func(m Model, n *Note) *markdown.Document {
		if m.Vault != docsVault {
			docs = map[string]docEntry{}
			docsVault = m.Vault
		}
		e, cached := docs[n.Path]
		switch {
		case m.CurAnchor >= 0 && m.NavSeq != seatedSeq:
			seatedSeq = m.NavSeq
			e = docEntry{note: n, doc: markdown.NewDocumentAt(n.Blocks, m.CurAnchor)}
		case !cached || e.note != n:
			e = docEntry{note: n, doc: rebuildDocument(n, e.doc)}
		default:
			return e.doc
		}
		docs[n.Path] = e
		return e.doc
	}
	// The document the column is showing, shared with the aside so its
	// outline can mark where the reader is and move them within the note
	// they already have open.
	cur := &docCursor{}
	// The trail comes off the theme stream, as the two side columns do, so a
	// palette change redraws it; its interaction state is the stream's and
	// not the frame's, so a click survives the theme changing under the
	// pointer.
	mainSlot := rx.Map(breadcrumb.Trail(th, breadcrumb.TrailProps{Chevron: trailChevronDp}),
		func(trail breadcrumb.TrailLayout) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				return layoutNotePage(gtx, loadModel(), loadTok(), &propClick, &backClick, &fwdClick, trail, &read, cur, docFor)
			}
		})
	return vaultFrame(loadModel, loadTok,
		treeSidebar(th, loadModel, loadTok),
		asideColumn(cur, loadModel, loadTok),
		mainSlot,
	)
}

// layoutNotePage lays out the main slot: the header row (back/forward
// and the breadcrumb), properties panel, document — or the
// scanning/error/empty message standing in for them.
func layoutNotePage(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	propClick, backClick, fwdClick *widget.Clickable,
	trail breadcrumb.TrailLayout,
	read *reader,
	cur *docCursor,
	docFor func(Model, *Note) *markdown.Document,
) layout.Dimensions {
	note := m.CurrentNote()
	// The reading column lies on its own paper: the pinned app background,
	// one storey above the floor the window chrome — the chrome row, tree
	// rail, aside — is painted in, in both schemes. The panel and the code
	// fills below take their rungs from this paper rather than from the
	// ramp, so the note reads as a document resting on darker furniture
	// rather than as more chrome.
	paint.FillShape(gtx.Ops, tok.col.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
	// The page's trailing and bottom margins are spent inside the document
	// rather than by the page, so the document's viewport reaches the
	// column's own edges the way the platform's reading surfaces do: the
	// scrollbar stands hard against the trailing edge, and the last row of
	// pixels above the column's foot carries text like every other row.
	// The margins are not lost — the document keeps its own reading measure
	// through the style's gutter, and comes to rest a foot margin above the
	// bottom edge through its end space. Everything that is not the document
	// — the header row, the properties panel, the standing messages — puts
	// the trailing margin back itself.
	inset := layout.Inset{Top: noteInsetDp, Left: noteInsetDp}
	trailing := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: noteInsetDp}.Layout(gtx, w)
		}
	}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		crumbs := trailSegments(notePlaces(m), revealFolder)

		var body layout.FlexChild
		// scrolling records whether the body is the document or a standing
		// message, which is what decides the gap above it.
		scrolling := false
		switch {
		case m.Scanning && note == nil:
			cur.show(nil)
			body = messageChild(tok, "Scanning vault…")
		case m.ScanErr != "":
			cur.show(nil)
			body = messageChild(tok, m.ScanErr)
		case note == nil:
			cur.show(nil)
			body = messageChild(tok, "No notes in this vault.")
		default:
			scrolling = true
			doc := docFor(m, note)
			// The aside's outline reads and moves this document. It lays
			// out after this column does, so what it reads is this frame's
			// position and what it moves shows on the next.
			cur.show(doc)
			style := noteStyle(tok.col, tok.typ)
			style.Text.OnLinkClick = func(gtx layout.Context, url string) {
				linkClicked(gtx, m, url)
			}
			style.OnTaskClick = func(gtx layout.Context, item *markdown.ListItem) {
				mvu.MessageOp{Message: ToggleTask{Path: note.Path, Item: item}}.Add(gtx.Ops)
			}
			// The bar is the design system's, taking the same colour
			// tokens the document's style did. Occupy rather than
			// Overlay: the gutter costs the prose ten dp of measure
			// once, where an overlay bar lands on the ends of the lines
			// it floats over. It draws nothing at all while the whole
			// note fits, and fades out a second after the note stops
			// moving — both the treatment's own behaviour, not the
			// app's.
			//
			// The document is the one row that runs to the column's edge,
			// where the bar belongs. The reading margin the other rows get
			// from the page inset it takes as its own gutter, less the
			// width the bar already reserves, so the prose ends level with
			// the breadcrumb above it.
			//
			// The two spaces are the vertical half of the same bargain, one
			// at each end: the column runs from the row above it to the
			// status bar below it, and the note still rests a gap below the
			// row at its start and a foot margin above that bar at its end.
			// Both are the document's own content, so the scroll bounds, the
			// page moves and the bar's track account for them; held back
			// outside the viewport either would leave a dead strip beside
			// every half-cut line the reader scrolls past.
			bar := scrollbar.FromTokens(tok.col)
			style.Gutter = max(noteInsetDp-bar.Width(), 0)
			style.StartSpace = noteGapDp
			style.EndSpace = noteEndSpaceDp
			body = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				// The reading keys are filtered on the column's own focus
				// tag, so they move the document only while it holds the
				// keyboard — never out from under the find field or the
				// folder rail.
				return read.layout(gtx, doc, func(gtx layout.Context) layout.Dimensions {
					return doc.LayoutScrollbar(gtx, tok.shaper, style, bar, list.Occupy)
				})
			})
		}

		header := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return navButton(gtx, tok, backClick, icons.HistoryBack, "Back", m.Cursor > 0, GoBack{})
				}),
				layout.Rigid(complayout.HSpacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return navButton(gtx, tok, fwdClick, icons.HistoryForward, "Forward", m.Cursor+1 < len(m.History), GoForward{})
				}),
				layout.Rigid(complayout.HSpacer(noteGapDp)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return trail(gtx, crumbs)
				}),
			)
		}

		children := []layout.FlexChild{layout.Rigid(trailing(header))}
		if note != nil && note.FM.Present {
			children = append(children,
				layout.Rigid(complayout.VSpacer(noteGapDp)),
				layout.Rigid(trailing(func(gtx layout.Context) layout.Dimensions {
					return layoutProperties(gtx, tok, note.FM, m.PropsOpen, propClick)
				})),
			)
		}
		// The page puts no gap above the document: its viewport begins on
		// the lower edge of whatever row stands over it — the breadcrumb, or
		// the properties panel when the note carries one — so a line
		// scrolling out of the top is cut by that edge and disappears under
		// it. The reading gap is not lost, it is spent inside the document
		// as its start space, where it is the note's resting position rather
		// than a margin held back on every frame; held back out here it would
		// leave a strip of bare paper over every half-cut line. A standing
		// message owns no viewport and takes the gap like any other row.
		if !scrolling {
			children = append(children, layout.Rigid(complayout.VSpacer(noteGapDp)))
		}
		children = append(children, body)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// rebuildDocument returns a Document for n, seated at prev's viewport
// when prev is the document this one replaces. Scroll lives on the
// document; transferring it is how a reload — a task toggle, a note
// re-read because the file moved on — keeps the reader where they were.
func rebuildDocument(n *Note, prev *markdown.Document) *markdown.Document {
	if prev == nil {
		return markdown.NewDocument(n.Blocks)
	}
	return markdown.NewDocumentAt(n.Blocks, prev.Position().First)
}

// renderNotePage is the static counterpart of the vault screen's main
// slot, used by goldens: fresh widget state, a fresh document per note,
// laid out once from pre-resolved tokens and processing no events. The
// document is seated at the model's anchor when it carries one and at the
// top otherwise, which is the only way a still image can be taken part
// way down a note.
func renderNotePage(
	shaper *text.Shaper,
	m Model,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	typo tokens.Typography,
	den tokens.Density,
) layout.Widget {
	return renderNotePageInto(&docCursor{}, shaper, m, colors, sp, typo, den)
}

// renderNotePageInto is renderNotePage with the document cursor supplied,
// so a whole-window render can hand the same one to the aside and the two
// columns agree about which document is on screen.
func renderNotePageInto(
	cur *docCursor,
	shaper *text.Shaper,
	m Model,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	typo tokens.Typography,
	den tokens.Density,
) layout.Widget {
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	// The trail is built here rather than inside the frame closure: it owns
	// the row's clickables and has to outlive the frame it draws, static
	// render or not.
	trail := breadcrumb.NewTrail(shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
		colors, sp, typo.TitleSmall)
	var (
		propClick widget.Clickable
		backClick widget.Clickable
		fwdClick  widget.Clickable
		read      reader
	)
	docs := map[string]*markdown.Document{}
	docFor := func(m Model, n *Note) *markdown.Document {
		d := docs[n.Path]
		if d == nil {
			if m.CurAnchor >= 0 {
				d = markdown.NewDocumentAt(n.Blocks, m.CurAnchor)
			} else {
				d = markdown.NewDocument(n.Blocks)
			}
			docs[n.Path] = d
		}
		return d
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layoutNotePage(gtx, m, tok, &propClick, &backClick, &fwdClick, trail, &read, cur, docFor)
	}
}

// navButton renders one history affordance: the set's mark for that
// direction, emitting its message on click while enabled, and drawn
// dimmed and inert at the stack's end.
func navButton(
	gtx layout.Context,
	tok themeTokens,
	click *widget.Clickable,
	mark icons.Name,
	label string,
	enabled bool,
	msg mvu.Message,
) layout.Dimensions {
	if click.Clicked(gtx) && enabled {
		mvu.MessageOp{Message: msg}.Add(gtx.Ops)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		c := tok.col.Ramps.Neutral.Step(noteNavDimStep)
		if enabled {
			c = tok.col.Ramps.Neutral.Step(noteNavInkStep)
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return drawMark(gtx, mark, noteNavMarkDp, c)
	})
}

// linkClicked dispatches one link activation: wikilinks (embeds included
// — an embed navigates like an ordinary link) resolve against the index
// and navigate on the same frame; web links open the system browser; a
// link whose file part matches several notes raises the chooser with the
// candidates the resolver refused to pick between, and every other refusal
// surfaces its reason as a toast; anything else is ignored.
func linkClicked(gtx layout.Context, m Model, url string) {
	switch {
	case strings.HasPrefix(url, "wiki:"), strings.HasPrefix(url, "wikiembed:"):
		if m.Index == nil {
			return
		}
		body := strings.TrimPrefix(strings.TrimPrefix(url, "wikiembed:"), "wiki:")
		res, rerr := Resolve(m.Index, m.Current, body)
		if rerr != nil {
			if len(rerr.Candidates) > 0 {
				mvu.MessageOp{Message: OpenChooser{Body: body, Candidates: rerr.Candidates}}.Add(gtx.Ops)
				return
			}
			toast.Notify(gtx, toast.Warning, rerr.Reason)
			return
		}
		mvu.MessageOp{Message: Navigate{Path: res.Path, Headings: res.Headings, BlockID: res.BlockID}}.Add(gtx.Ops)
	case strings.HasPrefix(url, "https://"), strings.HasPrefix(url, "http://"):
		openBrowser(url)
	}
}

// openBrowser opens an absolute web URL in the system browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// messageChild renders a status line in place of the document. It takes the
// trailing margin itself, the page inset having handed that job to its rows
// so the document can reach the column's edge.
func messageChild(tok themeTokens, msg string) layout.FlexChild {
	return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: noteInsetDp}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			drawLabel(gtx, tok.shaper, msg, tok.typ.BodyLarge, tok.col.Ramps.Neutral.Step(700))
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
}

// notePlaces builds the trail's places: one per folder on the current
// note's path inside the vault, then the note itself. Each folder reveals
// itself in the tree when clicked; the note is where you already are and
// stays inert. The vault is not a place here — it names the window from
// the chrome row, and as a crumb it would promise a parent to climb to
// that a vault does not have.
//
// The places carry in-vault paths, so a folder and a note of the same name
// in different branches are different places and a click on one is never
// delivered to the other.
func notePlaces(m Model) []place {
	var places []place
	note := m.CurrentNote()
	if note == nil {
		return places
	}
	if dir := path.Dir(note.Path); dir != "." {
		cum := ""
		for _, seg := range strings.Split(dir, "/") {
			if cum == "" {
				cum = seg
			} else {
				cum += "/" + seg
			}
			places = append(places, place{label: seg, path: cum})
		}
	}
	return append(places, place{label: note.Title, path: note.Path})
}

// revealFolder is the click a folder in the note's trail carries: the tree
// opens the whole way down to it.
func revealFolder(dir string) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		mvu.MessageOp{Message: RevealFolder{Dir: dir}}.Add(gtx.Ops)
	}
}

// layoutProperties renders the collapsible properties panel: a header
// row toggling the fold, then either the key/value pairs the trivial
// split could read, or the raw block in a code style.
func layoutProperties(
	gtx layout.Context,
	tok themeTokens,
	fm obsidian.FrontMatter,
	open bool,
	click *widget.Clickable,
) layout.Dimensions {
	if click.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleProperties{}}.Add(gtx.Ops)
	}
	// The head is the panel's control: the same quiet step the keys below it
	// take, at the weight that says a row can be worked rather than read.
	ink := tok.col.Ramps.Neutral.Step(propLabelStep)
	headStyle := tok.typ.TitleSmall
	headStyle.Weight = propHeadWeight
	header := func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Properties").Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawDisclosure(gtx, open, propMarkDp, ink)
				}),
				layout.Rigid(complayout.HSpacer(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLabel(gtx, tok.shaper, "Properties", headStyle, ink)
				}),
			)
		})
	}
	if !open {
		return header(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(header),
		layout.Rigid(complayout.VSpacer(propRowGapDp)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return propertiesBody(gtx, tok, fm)
		}),
	)
}

// propertiesBody is the expanded panel: pairs when the trivial split read
// them, the raw block in code style otherwise. Both stand on the note's own
// paper — the ground floor of the surface story, the same level the column
// itself is laid on — inside a rounded hairline.
//
// The ground is a measurement, taken against the surfaces this page already
// carries rather than against the scale in the abstract. Neither fill the
// scale offers can be spent here: measured off a paper at 246, the
// separator's tint sits at 212 and the next fill up at 232, while both code
// blocks under the panel sit at 239–245 — so either would make the metadata
// darker than the code and the first thing the eye lands on, ahead of the
// note's own title, and 232 is the exact fill the window's rail and aside
// wear, which reads as a slab of chrome dropped onto the page. The dark
// scheme measures the same shape: paper 24, code 30, the two candidate
// fills 46 and 34.
//
// So the panel takes no fill of its own and wears the page's existing idiom
// for a bounded block that does not shout — as the code blocks do, a
// hairline around a ground barely off the paper. The outline is the whole
// of the panel's budget: the hair takes the step past the separator's, for
// the measurement in propEdgeStep, since a line that dissolves is not an
// outline. The corners are the code blocks', so the page has one shape for
// a bounded box rather than two.
func propertiesBody(gtx layout.Context, tok themeTokens, fm obsidian.FrontMatter) layout.Dimensions {
	// The panel names its own ground rather than inheriting whatever it is
	// dropped on, so the hairline always has the surface it was judged
	// against inside it.
	fill := tok.col.Background
	radius := gtx.Dp(unit.Dp(tokens.Radius.Base))
	rec := func(gtx layout.Context) layout.Dimensions {
		return complayout.Inset(propPadDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if len(fm.Fields) == 0 {
				raw := strings.TrimRight(fm.Raw, "\n")
				if raw == "" {
					raw = "(empty)"
				}
				return drawText(gtx, tok.shaper, raw, tok.typ.Code, tok.col.Ramps.Neutral.Step(propLabelStep))
			}
			// Key and value are one face at one weight, told apart by ink
			// alone — the arrangement the reading app this viewer is judged
			// beside uses, and the only one in which the steps chosen for
			// them are the order the reader sees.
			//
			// A weight difference outvotes the ink: with the key in the
			// heavier label role, measured off the rendered panel, the
			// value's darkest pixels reach 78 against the key's 92 on a 246
			// paper — the twenty-six levels the two steps are apart collapse
			// to fourteen — and in the dark scheme the key's brightest
			// reaches 196 against the value's 181, the order inverted
			// outright. A regular weight has too little of each glyph fully
			// covered for its nominal ink to arrive; two columns can only be
			// ranked by ink if the ink is the only thing that differs.
			keyStyle := tok.typ.BodyMedium
			keyInk := tok.col.Ramps.Neutral.Step(propLabelStep)
			// The key column is as wide as the longest key plus a fixed
			// gap: each key is measured into a discarded recording, and
			// the widest ink wins, capped at half the panel so a runaway
			// key cannot squeeze the values out.
			keyW := 0
			mg := gtx
			mg.Constraints.Min = image.Point{}
			for _, f := range fm.Fields {
				macro := op.Record(mg.Ops)
				d := drawLabel(mg, tok.shaper, f.Key, keyStyle, keyInk)
				macro.Stop()
				if d.Size.X > keyW {
					keyW = d.Size.X
				}
			}
			if maxW := gtx.Constraints.Max.X / 2; keyW > maxW {
				keyW = maxW
			}
			keyGap := gtx.Dp(unit.Dp(propKeyGapDp))
			var rows []layout.FlexChild
			for i, f := range fm.Fields {
				f := f
				if i > 0 {
					rows = append(rows, layout.Rigid(complayout.VSpacer(propRowGapDp)))
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							g := gtx
							if g.Constraints.Max.X > keyW {
								g.Constraints.Max.X = keyW
							}
							dims := drawLabel(g, tok.shaper, f.Key, keyStyle, keyInk)
							return layout.Dimensions{Size: image.Pt(keyW+keyGap, dims.Size.Y), Baseline: dims.Baseline}
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return drawText(gtx, tok.shaper, fieldValue(f), tok.typ.BodyMedium,
								tok.col.Ramps.Neutral.Step(propValueStep))
						}),
					)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	}
	// Measure, then lay the edge and the ground under the measured content.
	// The hairline is drawn as the whole box in the edge's colour with the
	// ground inset over it, rather than as a stroke on the path: a stroke is
	// centred on its path and would spend half its width outside the box the
	// panel was measured at, and at one hair every pixel of it would be a
	// blend of the two colours instead of either. Inset the same way the
	// page's other bounded blocks are drawn, the line is the colour it says
	// it is and the panel occupies exactly the space it asked for.
	macro := op.Record(gtx.Ops)
	dims := rec(gtx)
	call := macro.Stop()
	box := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
	edge := max(gtx.Dp(unit.Dp(propEdgeDp)), 1)
	paint.FillShape(gtx.Ops, tok.col.Ramps.Neutral.Step(propEdgeStep), clip.UniformRRect(box, radius).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(box.Inset(edge), max(radius-edge, 0)).Op(gtx.Ops))
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
}

// fieldValue renders one frontmatter field's value: the scalar as
// written, or a block list joined with commas.
func fieldValue(f obsidian.Field) string {
	if len(f.Items) > 0 {
		return strings.Join(f.Items, ", ")
	}
	if f.Value == "" {
		return "—"
	}
	return f.Value
}
