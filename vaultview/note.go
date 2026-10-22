// note.go is the vault screen's note column — the main slot of the
// composition frame.go builds, with the folder tree leading and the
// backlinks pane trailing. The column renders the current note: a pinned
// head — the collapsible properties pane fed by the frontmatter split —
// parted from the scrolled document below it by a seam,
// and the parsed body as a markdown Document. Wikilink clicks resolve
// against the index and navigate; a task checkbox writes its marker in
// the file before the click returns; web links open the system browser;
// a link matching several notes raises the chooser, and every other
// refusal surfaces as a toast.

package main

import (
	"image"
	"image/color"
	"math"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

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

	"github.com/vibrantgio/components/breadcrumb"
	"github.com/vibrantgio/components/input"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/components/toast"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/notifications"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/brand"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
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
	// frame would leave a strip of blank page under that half-cut line.
	noteEndSpaceDp = 40
	// noteGapDp is the page's own rhythm: the space between the rows above
	// the document, and the space the document rests below the last of them.
	// The second is spent inside the document rather than by the page, for
	// the reason the end space is.
	noteGapDp = 16
	// noteMeasureDp is the widest a block of this note may lay out: the
	// column's measure. Past it a wider window gives the reader more of the
	// document rather than longer lines, and the run of blocks is centred in
	// what is left.
	//
	// It is 90 characters of this system's body text. BodyLarge sets at
	// sixteen px, and shaping prose in it measures an average advance of
	// 7.19 dp a character, so 90 characters is 647 dp. Ninety is the
	// owner's choice, taken against an A4 page: at 15 mm margins such a
	// page carries 85 to 100 characters a line at book sizes.
	//
	// The reading surface this viewer is judged beside sets a shorter
	// line. In the stored capture of it its longest line of prose measures
	// 540 px between its first and last painted pixel — x 91 to x 630, one
	// pixel per dp on that display — which is 75 characters at that same
	// advance, its lines falling between 66 and 75. That is what that
	// surface sets.
	noteMeasureDp = 647

	// noteMinMeasureDp is the narrowest a line of this note may lay out
	// at: 60 characters at that same 7.19 dp advance. Sixty is the bottom
	// of the band typography has long held a reader can walk back along
	// without losing the next line. Narrower than this the column is no
	// longer a column of prose.
	noteMinMeasureDp = 431

	// noteMinWidthDp is the narrowest the note column may be dragged to:
	// that measure and the insets either side of it. It bounds both of the
	// window's splitters and nothing else, so a window is free to be
	// narrower than it — what it stops is a boundary taking the last of
	// the document's room, not a small window from opening.
	noteMinWidthDp = noteMinMeasureDp + 2*noteInsetDp

	// The properties pane's rhythm: the gap between one metadata row and
	// the next — and between the disclosure head and the box under it — the
	// pad inside that box, and the gap between the key column and the
	// values.
	//
	// The row gap is measured against the reading app this viewer is judged
	// beside, which sets its metadata on a twenty-one px line box and spends
	// eleven px between rows; this pane sets on a twenty px box, already
	// tighter than the thing being copied. Four is one stop down the spacing
	// scale — six is not a stop on the scale at all — and lands the pitch at
	// twenty-four against the reference's thirty-two.
	//
	// The padding is where a box costs what a list does not: the reference
	// draws no box round its metadata and spends nothing here, and twelve
	// is the pad the note's code fences take, which is a screenful of code
	// and not three lines of key and value. Eight is one stop under it.
	//
	// Measured off the rendered page, this rhythm holds a three-field pane
	// to eighty-four px — fourteen less than the same pane one stop up the
	// scale, which stands ninety-six and pushes the note's title from row 189
	// of a seven hundred px viewport down to row 203, twenty-seven per cent
	// of the window against twenty-nine.
	propRowGapDp = 4
	propPadDp    = 8
	propKeyGapDp = 16

	// propEdgeDp is the pane's hairline. The pane is a group: it takes the
	// page for its own fill and draws the platform's separator at its edge,
	// which is the whole of the channel it has.
	propEdgeDp = 1

	// propHeadWeight is what tells the pane's disclosure head from the
	// field keys under it.
	//
	// At the keys' own step, size and weight the head reads as one more
	// label in the column of labels it opens. The reading app this viewer is
	// judged beside separates the two by colour: its head is set at the prose
	// foreground its values are, and its keys a measured step under both — 218
	// against 179 on a 28 page. That axis is already spent here, on the
	// order between the values and the keys, so the head takes the other
	// axis it has: the bold the note's own headings take, at the same size
	// and the same step as the keys. Letter-spacing is not available — the
	// label helper this window draws with sets no tracking.
	propHeadWeight = tokens.WeightBold

	// propMarkDp sizes the properties pane's disclosure: the size a mark
	// takes next to a line of text. A chevron is a diagonal spanning the
	// whole of its square, and at the size a mark takes as a control in its
	// own right its stroke stands half again over the caps of the text it
	// serves. The row centres its children, so the smaller square costs the
	// row no height and moves nothing else.
	propMarkDp = markSmallDp
)

// noteCodeStyles are the syntax highlighter styles a note's fences are drawn
// in, one per appearance: the ones the kept theme names when it names ones
// this build can resolve, and the highlighter's own defaults otherwise — a
// theme with no style in it, or one naming a style file that has since left
// the styles folder, draws code exactly as it did for somebody who never
// chose.
//
// A pair and not a name because a style is fitted to a background:
// the set of colours balanced against a near-white page is not the set
// anybody would balance against a near-black one. So the light appearance and
// the dark one each wear their own member, background and all, and a desktop
// switching between them switches the code's plate with everything else.
//
// They are set once, at startup, from the same kept theme the colours come
// from, and read from then on. The choice is a preference and not a mode:
// there is no affordance in this window to change it, because the window
// that chooses a theme is the one that keeps it.
var noteCodeStyles = highlight.DefaultStyles()

// adoptCodeStyles resolves the highlighter styles a kept theme asks for. The
// styles folder is read first, since a kept name may be a style somebody
// wrote themselves; a name that cannot be resolved, or one fitted to the appearance
// it is not kept for, falls back to that appearance's default rather than
// failing. What the folder could not read is not surfaced here — this window
// shows notes, and the place a style file gets fixed is the window that
// offered it.
func adoptCodeStyles(kept brand.Brand) highlight.StylePair {
	if dir, err := brand.StylesDir(); err == nil {
		highlight.LoadDir(dir)
	}
	return highlight.StylesOrDefault(kept.Style.Names())
}

// noteStyle derives the markdown document style for the current tokens: the
// token-themed defaults, the reading typeface for code, and a fenced block
// wearing the chosen syntax highlighter style. The link hook is attached per
// frame in layoutNotePage, where the model is at hand.
//
// The style is worn rather than re-fitted. A fence takes the background the
// author drew their colours on and those colours as they were drawn, so a
// block in a note is the style itself and not a rendering of it; the page
// around it — the prose, the chip an inline span sits on, the bar a wide
// block scrolls under — stays this theme's. Which member of the pair reaches
// the fence follows the tokens, so a change of appearance is a change of
// plate.
//
// There is no memo: wearing resolves a name and reads four colours off it,
// which is a map lookup cheap enough for a path that runs every frame.
func noteStyle(c tokens.PlatformColors, typ tokens.Typography) markdown.Style {
	st := markdown.FromTokens(c, typ, c.TextBackground)
	st.Mono = font.Typeface(typ.Code.Typeface)
	st.CodeSize = unit.Sp(typ.Code.Size)
	highlight.WearPair(&st, noteCodeStyles, c)
	return st
}

// docEntry is one cached document and the Note value it was built from.
type docEntry struct {
	note *Note
	doc  *markdown.Document
}

// The arrival highlight's life. A followed link marks the content it brought
// the reader to, and the mark then leaves by itself, so the duration is the
// one this house already gives a mark that leaves by itself: the toast's
// lifetime, faded over the trailing slice a toast tweens its alpha over,
// which is the motion scale's DurSlow stop (400 ms).
var (
	arrivalLife = notifications.DefaultLifetime
	arrivalFade = tokens.Motion.DurSlow
)

// arrivalAlpha is how much of the highlight fill is left at age: all of it
// until the fade opens, linearly less across the fade, and none of it once
// the life has run.
func arrivalAlpha(age time.Duration) float64 {
	switch {
	case age >= arrivalLife:
		return 0
	case arrivalFade <= 0 || age <= arrivalLife-arrivalFade:
		return 1
	}
	return float64(arrivalLife-age) / float64(arrivalFade)
}

// arrival is the note column's view state for the followed-link highlight:
// which landing is being marked, on which note, and the instant the marking
// was armed.
//
// It is view state and not model state because the marking is a moment
// passing rather than something the vault knows. The model records that a
// link was followed; how much of the mark is left is a question only the
// frame drawing it asks, and a model carrying it would have to be re-emitted
// every frame of the fade.
type arrival struct {
	seq   int       // the Model.Arrival value this marking is for
	path  string    // the note it landed on
	at    time.Time // gtx.Now of the frame it was armed on
	spent bool      // the reader left the note; the marking does not wait
}

// mark returns the top-level block the arrival highlight is on this frame and
// base at what the fade has left of its alpha, arming a landing the model has
// newly reported and asking for the frames the fade needs. A landing that
// carried a block anchor marks the block it seated on; one that carried none
// marks the note's opening content, which is its first top-level block.
//
// base is the document style's own arrival fill, so the flash is the fill the
// library lays over the surface the note is read on.
//
// The marking is over the moment the reader leaves the note it was made on,
// and it does not come back when they do: a highlight lives as long as its
// cause, and the cause was one arrival.
func (a *arrival) mark(gtx layout.Context, m Model, base color.NRGBA) (int, color.NRGBA, bool) {
	if m.Arrival == 0 {
		return 0, color.NRGBA{}, false
	}
	if a.seq != m.Arrival {
		*a = arrival{seq: m.Arrival, path: m.Current, at: gtx.Now}
	}
	if a.spent || m.Current != a.path {
		a.spent = true
		return 0, color.NRGBA{}, false
	}
	age := gtx.Now.Sub(a.at)
	alpha := arrivalAlpha(age)
	if alpha <= 0 {
		return 0, color.NRGBA{}, false
	}
	// Holding costs one frame at the instant the fade opens; fading costs
	// every frame until it is done.
	if hold := arrivalLife - arrivalFade; age < hold {
		gtx.Execute(op.InvalidateCmd{At: a.at.Add(hold)})
	} else {
		gtx.Execute(op.InvalidateCmd{})
	}
	fill := base
	fill.A = uint8(math.Round(float64(fill.A) * alpha))
	return max(m.CurAnchor, 0), fill, fill.A > 0
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
// The column takes the keyboard when a note arrives while the rail's rows do
// not hold it — the first note it shows, a followed link, back and forward —
// so a note can be read the moment it is opened rather than after finding
// somewhere to click. A note opened from the rail is the one arrival that
// leaves the keyboard where it is: the platform's sidebars keep it after a
// click on a row, and the arrows there go on walking the rows. Filtering the
// rail opens nothing and so takes nothing away from the field being typed in.
type reader struct {
	tag readerTag
	// rail is where the rail publishes its focus tag, nil in a composition
	// that has no rail.
	rail *railFocus
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
		if !r.rail.holds(gtx) {
			gtx.Execute(key.FocusCmd{Tag: &r.tag})
		}
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
// tree in the leading column, the backlinks pane in the trailing one,
// and the note between them. The note column reads the model and token
// snapshots at frame time; repaints on model change are driven by the
// routed layer's re-emission.
//
// modelObs is the same replay-latest models stream the routed layer reads,
// taken here for the one thing a frame-time snapshot cannot serve: the
// tooltip beside a hovered link takes its text when it is built, so the
// column needs an emission per address and not a value per frame.
func vaultLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], loadModel func() Model, loadTok func() themeTokens, widths *shell.ArrangementMemory, still *stillWindow) rx.Observable[layout.Widget] {
	// The rail fills this in when its own stream is subscribed, and the note
	// column reads it: a note opened from the rail leaves the keyboard there.
	rail := &railFocus{}
	// Documents are cached per note path and reused on every frame, so
	// each note's scroll position and paragraph interaction state survive
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
		read      = reader{rail: rail}
		arr       arrival
		find      pageFind
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
	// The find field is the components search field, built once at
	// subscription scope so what was typed survives every emission, and it
	// hands over its focus tag and its clearing: the shortcut that opens the
	// field puts the keyboard in it, and Escape takes the query back out.
	//
	// It stands in the window's toolbar band, so it is the platform's toolbar
	// search RECESS and not the sidebar's field — the two are different
	// controls and the reference measures them apart — and it is handed to
	// the frame rather than to the page.
	field := input.SearchField(th, input.SearchFieldProps{
		Placeholder: "Search",
		Description: "find in this note",
		Variant:     input.Chrome,
		Region:      input.Toolbar,
		Surface:     findFieldSurface,
		FocusTag:    func(tag event.Tag) { find.tag = tag },
		Clear:       func(clear func()) { find.clear = clear },
		OnChange:    func(_ layout.Context, text string) { find.typed(text) },
		// What the query has found, standing inside the field at its
		// trailing end. It is asked every frame because the count is the
		// document's answer and changes with the note under the query, not
		// with the theme the field was built under.
		Count: find.label,
	})
	// The tooltip that says where the link the reader is on goes — the one
	// under the pointer or the one holding the keyboard, as the status bar
	// says it. The window has one, and it is rebuilt for each address
	// rather than told one: the component takes its text when it is built,
	// and a fresh address is a fresh dwell anyway — the reader who crosses
	// onto another link waits for it as they waited for the first. The
	// arbiter is this window's, so this tooltip and any other in the window
	// cannot both stand.
	//
	// The empty address — the reader is on no link — builds nothing at
	// all, and the column lays nothing out.
	tip := &linkTip{}
	tips := tooltip.NewArbiter()
	tipSlot := rx.SwitchMap(
		rx.Map(modelObs, func(m Model) string { return m.LinkUnder }).
			DistinctUntilChanged(func(a, b string) bool { return a == b }),
		func(address string) rx.Observable[layout.Widget] {
			if address == "" {
				return rx.Of(layout.Widget(nil))
			}
			return tooltip.Tooltip(th, tooltip.Props{
				Text: address,
				// Below the link, which is where this platform puts an
				// annotation about a word in prose; the component takes
				// the side above when the link sits too near the foot of
				// the column for the annotation to fit under it.
				Placement: tooltip.Bottom,
				Arbiter:   tips,
				Anchor:    tip.anchor,
			})
		},
	)
	mainSlot := rx.Map(rx.CombineLatest2(th, tipSlot), func(next rx.Tuple2[theme.Theme, layout.Widget]) layout.Widget {
		layer := next.Second
		return func(gtx layout.Context) layout.Dimensions {
			// Taken from the emission inside the frame, as the frame takes
			// its own slots: an emission arrives on the stream's goroutine
			// and everything the column touches is the frame's.
			tip.layer = layer
			return layoutNotePage(gtx, loadModel(), loadTok(), &propClick, &read, &arr, cur, docFor, &find, tip)
		}
	})
	// The trail comes off the theme stream, as the two side columns do, so a
	// palette change redraws it; its interaction state is the stream's and
	// not the frame's, so a click survives the theme changing under the
	// pointer. It is handed to the frame: the trail stands in the band,
	// where the platform shows the document's place.
	return vaultFrame(loadModel, loadTok, widths, still, &find, &read,
		treeSidebar(th, loadModel, loadTok, rail),
		asideColumn(cur, loadModel, loadTok),
		mainSlot,
		field,
		breadcrumb.Trail(th, breadcrumb.TrailProps{Chevron: trailChevronDp}),
	)
}

// linkTip is the annotation standing beside the link the reader is on — the
// one under the pointer or the one holding the keyboard: the tooltip the
// window built for the address the model carries, and the box the document
// last drew that link at.
//
// The box is the tooltip's anchor, written here before the tooltip lays out
// and read back by it through Props.Anchor. Both happen on the frame
// goroutine, in that order. The tooltip is given the whole viewport as its
// frame, which is what it measures the address against, what it reads from
// which side of the link has room for the annotation, and what it keeps the
// annotation inside the sides of.
type linkTip struct {
	layer layout.Widget
	box   image.Rectangle
}

// anchor is where the tooltip's trigger stands: the link the reader is on,
// as the document reported it this frame, and the empty rectangle when the
// document reports none. The tooltip draws no trigger of its own — the link
// is already on the page, hit-tested and in the window's Tab order — so the
// anchor is the whole of what it knows: a box standing is the link standing,
// and the dwell the annotation appears after runs while it does.
func (t *linkTip) anchor(layout.Context) image.Rectangle { return t.box }

// beside lays the tooltip over the link the document reports the reader on,
// in the plane the document was just drawn into — this column's own, since
// the box the document reports is in its viewport. It is laid out whether or
// not there is a box, because the empty anchor is how the annotation is told
// the link has gone; it draws nothing at all before the window has an
// address to say.
//
// The annotation itself is deferred by the component, so it paints over
// everything the window lays out after this column and is clipped by
// nothing: a tooltip standing beside a link near the column's edge is the
// one thing here that may cross it.
func (t *linkTip) beside(gtx layout.Context, doc *markdown.Document) {
	if t == nil || t.layer == nil {
		return
	}
	if box, ok := doc.LinkBox(); ok && !box.Empty() {
		t.box = box
	} else {
		t.box = image.Rectangle{}
	}
	t.layer(gtx)
}

// layoutNotePage lays out the main slot: the pinned head — the properties
// pane, where the note carries frontmatter — the seam under it, and the
// document, or the scanning/error/empty message standing in for them.
//
// The window's navigation and the note's place are NOT here: both stand in
// the toolbar band, where the platform keeps a window's history and, in its
// centre, the document's place as the path bar draws it.
func layoutNotePage(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	propClick *widget.Clickable,
	read *reader,
	arr *arrival,
	cur *docCursor,
	docFor func(Model, *Note) *markdown.Document,
	find *pageFind,
	tip *linkTip,
) layout.Dimensions {
	note := m.CurrentNote()
	// The reading column lies on the platform's text background, the fill a
	// document is read on. The properties pane and the code fills below are
	// read against this surface rather than against the chrome the rail and
	// the trailing column wear.
	paint.FillShape(gtx.Ops, tok.col.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
	// A column with nothing to show draws the platform's empty state in
	// place of the page: one line saying why, centred in the column. It is
	// not content and stands on no measure, so it is drawn before the page's
	// own reading inset is spent and the page is not laid out at all.
	if msg := emptyStateLine(m); msg != "" {
		cur.show(nil)
		return noteEmptyState(gtx, tok, msg)
	}
	// The page's trailing and bottom margins are spent inside the document
	// rather than by the page, so the document's viewport reaches the
	// column's own edges the way the platform's reading surfaces do: the
	// scrollbar stands hard against the trailing edge, and the last row of
	// pixels above the column's foot carries text like every other row.
	// The margins are not lost — the document keeps its own reading measure
	// through the style's gutter, and comes to rest a foot margin above the
	// bottom edge through its end space. Everything that is not the document
	// — the header row, the properties pane — puts the trailing margin back
	// itself.
	inset := layout.Inset{Top: noteInsetDp, Left: noteInsetDp}
	// Where the column's text stands, the properties pane included: the
	// document seats its blocks on the measure and centres them in a column
	// wider than it, so a pane standing at the page's own inset would part
	// from the prose the moment the column has room to spare.
	insetPx := gtx.Dp(unit.Dp(noteInsetDp))
	lead, measure := noteTextBox(gtx, tok, gtx.Constraints.Max.X)
	onMeasure := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			full := gtx.Constraints.Max.X
			mgtx := gtx
			mgtx.Constraints.Max.X = min(measure, full)
			mgtx.Constraints.Min.X = min(mgtx.Constraints.Min.X, mgtx.Constraints.Max.X)
			defer op.Offset(image.Pt(lead-insetPx, 0)).Push(gtx.Ops).Pop()
			dims := w(mgtx)
			return layout.Dimensions{Size: image.Pt(full, dims.Size.Y)}
		}
	}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		doc := docFor(m, note)
		// The find keys are drained before anything lays out, so a step
		// asked for on this frame is a step this frame's document makes.
		find.keys(gtx, m, read)
		// The aside's outline reads and moves this document. It lays
		// out after this column does, so what it reads is this frame's
		// position and what it moves shows on the next.
		cur.show(doc)
		style := noteStyle(tok.col, tok.typ)
		// The bar inside an overflowing fence is the reader's as much as
		// the note column's own is: it takes the setting the snapshot
		// carries, so a reader who asked for scroll bars always finds one
		// standing in the fence too.
		style.CodeScrollbar = scrollBarStyle(style.CodeScrollbar, tok.scrollBars)
		// The followed-link marking is set on the document every frame
		// it is alive and taken off it on the first frame it is not:
		// documents are cached across frames, so a marking left behind
		// would outlive the arrival that caused it.
		if block, fill, ok := arr.mark(gtx, m, style.ArrivalFill); ok {
			doc.Highlight(block, fill)
		} else {
			doc.ClearHighlight()
		}
		style.Text.OnLinkClick = func(gtx layout.Context, url string) {
			linkClicked(gtx, m, url)
		}
		style.Text.OnLink = func(gtx layout.Context, url string) {
			mvu.MessageOp{Message: LinkUnder{URL: linkAddress(m, url)}}.Add(gtx.Ops)
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
		// app's — unless the reader asked for scroll bars always,
		// which is the setting the snapshot carries.
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
		bar := noteScrollBar(tok)
		// The query is put on the document every frame it is alive and
		// taken off it on the first frame it is not, the way the arrival
		// marking is, and the places it finds go on the bar: one query
		// marks the prose and the bar alike, and one dismissal takes
		// both.
		bar.Matches = find.apply(m, doc)
		bar.Current = find.current
		style.Gutter = max(noteInsetDp-bar.Width(), 0)
		style.Measure = noteMeasureDp
		style.StartSpace = noteGapDp
		style.EndSpace = noteEndSpaceDp
		body := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			// The reading keys are filtered on the column's own focus
			// tag, so they move the document only while it holds the
			// keyboard — never out from under the find field or the
			// folder rail.
			return read.layout(gtx, doc, func(gtx layout.Context) layout.Dimensions {
				dims := doc.LayoutScrollbar(gtx, tok.shaper, style, bar, list.Occupy)
				// The address is said twice: at the window's foot while
				// the pointer is on the link, and beside the link once
				// the pointer has stood on it for the tooltip's own
				// delay. This is the second of them, laid over the
				// document it names.
				tip.beside(gtx, doc)
				return dims
			})
		})

		// The page's own top inset is where it was: the trail that stood
		// in this strip moved into the band, and what followed it moves up
		// into the room the strip held. The inset above is the platform's
		// and is unchanged, so the first thing the column shows — the
		// properties pane, or the document itself — now starts where the
		// trail started.
		var children []layout.FlexChild
		if note != nil && note.FM.Present {
			children = append(children,
				layout.Rigid(onMeasure(func(gtx layout.Context) layout.Dimensions {
					return layoutProperties(gtx, tok, note.FM, m.PropsOpen, propClick)
				})),
			)
		}
		// The page puts no gap above the document: its viewport begins on
		// the seam under the pinned head — the properties pane, where the
		// note carries frontmatter — so a line scrolling out of the top is
		// cut by that seam and disappears under it. The reading gap is
		// not lost, it is spent inside the document as its start space, where
		// it is the note's resting position rather than a margin held back on
		// every frame; held back out here it would leave a strip of bare page
		// over every half-cut line. What the gap here buys is the pinned
		// head's own foot: the air it keeps above the seam is the air it
		// keeps between its rows.
		//
		// A note with no frontmatter has no pinned head at all now the trail
		// stands in the band, and then there is neither a foot to keep nor a
		// region boundary to draw: the document begins at the page's own top
		// inset, which is where the trail began.
		if len(children) > 0 {
			children = append(children, layout.Rigid(complayout.VSpacer(noteGapDp)))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return pinnedSeam(gtx, tok)
			}))
		}
		children = append(children, body)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// noteScrollBar is the bar the document rides: the design system's, taking
// the colour tokens the document's style took, at the standing or fading
// behaviour the reader's own setting asks for.
func noteScrollBar(tok themeTokens) scrollbar.Style {
	return scrollBarStyle(scrollbar.FromTokens(tok.col, tok.col.TextBackground), tok.scrollBars)
}

// noteTextBox is where a line of this column's text begins and how wide it
// may run, both measured from the column's own leading edge: the page's
// reading inset, and the measure centred in what the viewport shows once
// the bar's lane has come off it.
//
// It is the document's own seating rule — [markdown.Style.Measure] over
// [markdown.Style.Gutter], with the bar occupying its lane — restated here
// so the rows the page draws around the document stand where the document's
// blocks stand. A test holds the two together.
func noteTextBox(gtx layout.Context, tok themeTokens, colW int) (lead, width int) {
	inset := gtx.Dp(unit.Dp(noteInsetDp))
	page := max(colW-inset, 0)
	barW := gtx.Dp(noteScrollBar(tok).Width())
	view := max(page-barW, 0)
	gutter := max(inset-barW, 0)
	avail := max(view-gutter, 0)
	m := gtx.Dp(unit.Dp(noteMeasureDp))
	if m <= 0 || m >= avail {
		return inset, max(page-inset, 0)
	}
	return inset + min((view-m)/2, avail-m), m
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

// renderNotePage is the static counterpart of the vault screen's main slot,
// used by goldens: fresh widget.Clickable state, a fresh document per note,
// laid out once from pre-resolved tokens and processing no events. The
// document is seated at the model's anchor when it carries one and at the top
// otherwise, which is the only way a still image can be taken part way down a
// note.
func renderNotePage(
	shaper *text.Shaper,
	m Model,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	typo tokens.Typography,
	den tokens.Density,
) layout.Widget {
	return renderNotePageInto(&docCursor{}, shaper, m, colors, sp, typo, den, &pageFind{})
}

// renderNotePageInto is renderNotePage with the document cursor supplied,
// so a whole-window render can hand the same one to the aside and the two
// columns agree about which document is on screen.
func renderNotePageInto(
	cur *docCursor,
	shaper *text.Shaper,
	m Model,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	typo tokens.Typography,
	den tokens.Density,
	find *pageFind,
) layout.Widget {
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	var (
		propClick widget.Clickable
		read      reader
		arr       arrival
	)
	// The page is drawn showing the note the model has current, so that is
	// the note the query is being marked in: a static render has no earlier
	// frame for the column to have noticed it on.
	find.note = m.Current
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
		// No tooltip in a still image: it appears after a dwell, and a
		// render that processes no events has nobody standing anywhere.
		return layoutNotePage(gtx, m, tok, &propClick, &read, &arr, cur, docFor, find, nil)
	}
}

// pinnedSeam draws the boundary between the column's pinned head and the
// document that scrolls under it: the hairline the Language gives two flush
// regions, the platform's separator laid over the surface beneath it, drawn
// once by the region above.
//
// It runs the note column's whole width and not the head's: the page's own
// leading inset is spent outside this row, and a boundary that stopped where
// the text starts would read as a rule under the breadcrumb rather than as
// the edge of a region. The column's trailing margin is the document's own
// gutter, which the seam runs to for the same reason.
//
// At the trailing end it runs past the column's own edge to the seam the
// inspector stands behind, which is [frameSplitterDp] further on: a seam is
// one continuous line, and a rule stopping a lane short of the line it meets
// reads as two edges. A page rendered with no inspector beside it paints
// that last stretch outside the picture, as the leading offset paints
// outside the page's inset.
func pinnedSeam(gtx layout.Context, tok themeTokens) layout.Dimensions {
	h := max(gtx.Dp(unit.Dp(seamDp)), 1)
	w := gtx.Constraints.Max.X
	lead := gtx.Dp(unit.Dp(noteInsetDp))
	toSeam := gtx.Dp(unit.Dp(frameSplitterDp))
	st := op.Offset(image.Pt(-lead, 0)).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, vgcolor.Flatten(tok.col.Separator, tok.col.TextBackground),
		clip.Rect(image.Rect(0, 0, w+lead+toSeam, h)).Op())
	st.Pop()
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// linkAddress is what the window says a link goes to while the pointer is on
// it: a wikilink the vault-relative path of the note it lands on — the same
// resolution a click makes, so the reader is told where the click will take
// them — and a web link its URL as written. A wikilink the index cannot
// resolve, or one matching several notes, shows the body as written: that is
// all the reader has asked for and all the vault can say about it.
func linkAddress(m Model, url string) string {
	switch {
	case url == "":
		return ""
	case strings.HasPrefix(url, "wiki:"), strings.HasPrefix(url, "wikiembed:"):
		body := strings.TrimPrefix(strings.TrimPrefix(url, "wikiembed:"), "wiki:")
		if m.Index == nil {
			return body
		}
		res, rerr := Resolve(m.Index, m.Current, body)
		if rerr != nil {
			return body
		}
		return res.Path
	default:
		return url
	}
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
			notifications.Notify(gtx, toast.Warning, rerr.Reason)
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

// emptyStateLine is what the note column has to say when it has no note to
// show, and the empty string when it has one: the fetch the window is
// waiting on, the first reading of the vault, a reading that failed, or a
// vault holding nothing to read.
//
// The fetch leads. The note the reader asked for is in the cloud and
// nothing is at its path yet, so the column says so in place of the note it
// is going to show — including over the note the reader was on, which is no
// longer the one they asked for.
func emptyStateLine(m Model) string {
	switch {
	case m.Downloading != "":
		return downloadingLabel
	case m.Scanning && m.CurrentNote() == nil:
		return "Scanning vault…"
	case m.ScanErr != "":
		// The line is the application's own sentence. The system's text
		// for the failure is written to the log, with the vault's path,
		// where the scan reports it.
		return "This vault can't be read."
	case m.CurrentNote() == nil:
		return "No notes in this vault."
	}
	return ""
}

// noteEmptyState draws the platform's empty state in the content column's
// FULL frame: the main slot's own box grown upward by the toolbar's share,
// which is the frame the line centres in rather than the row below the
// toolbar. The pattern carries the reading.
//
// The slot is recorded and replayed under the toolbar's own ops — the
// window hands the shell MainFirst — so the toolbar paints over the strip
// this frame reaches up into.
func noteEmptyState(gtx layout.Context, tok themeTokens, msg string) layout.Dimensions {
	row := gtx.Constraints.Max
	bar := gtx.Dp(unit.Dp(shell.ToolbarDepth))
	full := gtx
	full.Constraints = layout.Exact(image.Pt(row.X, row.Y+bar))
	defer op.Offset(image.Pt(0, -bar)).Push(gtx.Ops).Pop()
	shell.EmptyState(full, tok.shaper, tok.col, tok.typ, tok.col.TextBackground, msg)
	return layout.Dimensions{Size: row}
}

// notePlaces builds the trail's places: the vault, then one per folder on
// the current note's path inside it, then the note itself. Each earlier
// place reveals itself in the tree when clicked, and the last is the note
// the window is showing, which is where the reader already is.
//
// The note is the trail's last place because the trail stands where the
// window's title stands — the band's centre, which is where the platform
// shows a document's place — and a path bar that stopped at the folder
// would leave the document it is the path of unnamed.
//
// The places carry in-vault paths, so a folder and a note of the same name
// in different branches are different places and a click on one is never
// delivered to the other; the vault's own path is empty, which is the tree's
// root.
func notePlaces(m Model) []place {
	var places []place
	// The trail is built from the path the window stands on rather than
	// from a loaded note: a note being fetched out of the cloud is the
	// note the reader asked for and the trail names it, file or no file.
	// A loaded note's title is this same basename without its extension.
	rel := m.Current
	if rel == "" {
		return places
	}
	places = append(places, place{title: vaultName(m)})
	if dir := path.Dir(rel); dir != "." {
		cum := ""
		for _, seg := range strings.Split(dir, "/") {
			if cum == "" {
				cum = seg
			} else {
				cum += "/" + seg
			}
			places = append(places, place{title: seg, path: cum})
		}
	}
	return append(places, place{title: noteTitle(rel), path: rel})
}

// layoutProperties renders the collapsible properties pane: a header
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
	// The head is the pane's control: the same faint step the keys below it
	// take, at the weight that says a row can be worked rather than read.
	foreground := vgcolor.Flatten(tok.col.SecondaryLabel, tok.col.TextBackground)
	headStyle := tok.typ.TitleSmall
	headStyle.Weight = propHeadWeight
	header := func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Properties").Add(gtx.Ops)
			pointershape.OverSize(gtx.Ops, gtx.Constraints.Max, pointer.CursorPointer)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawDisclosure(gtx, open, propMarkDp, foreground)
				}),
				layout.Rigid(complayout.HSpacer(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return typeset.Text(gtx, tok.shaper, "Properties", headStyle, foreground, 1)
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

// propertiesBody is the expanded pane: pairs when the trivial split read
// them, the raw block in code style otherwise. Both stand on the note's own
// surface — the base of the surface story, the same level the column
// itself is laid on — inside a rounded hairline.
//
// The fill is a measurement, taken against the surfaces this page already
// carries rather than against the scale in the abstract. Neither fill the
// scale offers can be spent here: measured off a page at 246, the
// separator's tint sits at 212 and the next fill up at 232, while both code
// blocks under the pane sit at 239–245 — so either would make the metadata
// darker than the code and the first thing the eye lands on, ahead of the
// note's own title, and 232 is the exact fill the window's rail and aside
// wear, which reads as a slab of chrome dropped onto the page. The dark
// scheme measures the same shape: page 24, code 30, the two candidate
// fills 46 and 34.
//
// So the pane takes no fill of its own and wears the page's existing idiom
// for a bounded block of low prominence — as the code blocks do, a
// hairline around a fill one step off the page. The outline is the whole
// of the pane's budget: the hair takes the step past the separator's, for
// the measurement in propEdgeStep, since a line that dissolves is not an
// outline. The corners are the code blocks', so the page has one shape for
// a bounded box rather than two.
func propertiesBody(gtx layout.Context, tok themeTokens, fm obsidian.FrontMatter) layout.Dimensions {
	// The pane names its own fill rather than inheriting whatever it is
	// dropped on, so the hairline always has the surface it was judged
	// against inside it.
	fill := tok.col.TextBackground
	radius := gtx.Dp(unit.Dp(tokens.Radius.Base))
	rec := func(gtx layout.Context) layout.Dimensions {
		return complayout.Inset(propPadDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if len(fm.Fields) == 0 {
				raw := strings.TrimRight(fm.Raw, "\n")
				if raw == "" {
					raw = "(empty)"
				}
				return typeset.Text(gtx, tok.shaper, raw, tok.typ.Code,
					vgcolor.Flatten(tok.col.SecondaryLabel, fill), 0)
			}
			// Key and value are one face at one weight, told apart by colour
			// alone — the arrangement the reading app this viewer is judged
			// beside uses, and the only one in which the steps chosen for
			// them are the order the reader sees.
			//
			// A weight difference outvotes the colour: with the key in the
			// heavier label role, measured off the rendered pane, the
			// value's darkest pixels reach 78 against the key's 92 on a 246
			// page — the twenty-six levels the two steps are apart collapse
			// to fourteen — and in the dark scheme the key's brightest
			// reaches 196 against the value's 181, the order inverted
			// outright. A regular weight has too little of each glyph fully
			// covered for its nominal colour to arrive; two columns can only be
			// ranked by colour if the colour is the only thing that differs.
			keyStyle := tok.typ.BodyMedium
			keyForeground := vgcolor.Flatten(tok.col.SecondaryLabel, fill)
			// The key column is as wide as the longest key plus a fixed
			// gap: each key is measured into a discarded recording, and
			// the widest of them wins, capped at half the pane so a runaway
			// key cannot squeeze the values out.
			keyW := 0
			mg := gtx
			mg.Constraints.Min = image.Point{}
			for _, f := range fm.Fields {
				macro := op.Record(mg.Ops)
				d := typeset.Text(mg, tok.shaper, f.Key, keyStyle, keyForeground, 1)
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
							dims := typeset.Text(g, tok.shaper, f.Key, keyStyle, keyForeground, 1)
							return layout.Dimensions{Size: image.Pt(keyW+keyGap, dims.Size.Y), Baseline: dims.Baseline}
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return typeset.Text(gtx, tok.shaper, fieldValue(f), tok.typ.BodyMedium,
								vgcolor.Flatten(tok.col.Label, fill), 0)
						}),
					)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	}
	// Measure, then lay the edge and the fill under the measured content.
	// The hairline is drawn as the whole box in the edge's colour with the
	// fill inset over it, rather than as a stroke on the path: a stroke is
	// centred on its path and would spend half its width outside the box the
	// pane was measured at, and at one hair every pixel of it would be a
	// blend of the two colours instead of either. Inset the same way the
	// page's other bounded blocks are drawn, the line is the colour it says
	// it is and the pane occupies exactly the space it asked for.
	macro := op.Record(gtx.Ops)
	dims := rec(gtx)
	call := macro.Stop()
	box := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
	edge := max(gtx.Dp(unit.Dp(propEdgeDp)), 1)
	paint.FillShape(gtx.Ops, vgcolor.Flatten(tok.col.Separator, fill), clip.UniformRRect(box, radius).Op(gtx.Ops))
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
