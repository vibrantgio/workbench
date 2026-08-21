// aside.go is the window's trailing column: the current note's heading
// outline above, and the notes that link to it below, each scrolling in
// its own right. The two are panes of one column and not one list,
// because a long note's outline would otherwise push its backlinks off
// the bottom of the window — and the backlinks are what a reader keeps
// the column open for on a note that has no headings at all.
//
// The outline is the note's own structure, so it comes from the note's
// own parsed blocks (outline.go) and moves the document the reader
// already has open rather than opening it again. The backlinks are the
// vault's structure around the note, so they come from the reverse edges
// backlinks.go resolves, memoised per (index, current note) pair so the
// resolver runs once per landing rather than once per frame.
//
// The column stands on the sidebar's surface rather than on the note's
// paper. Both panes are furniture — one says what is in the note, the
// other what points at it, and neither is the note — and the window's
// rule is that furniture stands a step off the ground the document lies
// on. It also answers what the column looked like without one: a heading
// and a line of grey floating in three hundred dp of the document's own
// paper, which read as a margin somebody had forgotten to fill rather
// than as a pane. The fill is the frame's to paint, not this file's, so
// that the column can run the window's full height the way the sidebar
// opposite it does.

package main

import (
	"image"
	"image/color"
	"path"
	"strconv"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
)

// Aside layout constants.
const (
	asideInsetDp     = 16
	asideHeaderGapDp = 8
	// asideRowPadDp holds a row's own ink off its fill's edge. It is the
	// sidebar's treeRowPadDp under the sidebar's name, because the pill
	// drawn here is the pill drawn there and a pill means the same thing
	// in both: the ink it is behind is spoken for, and it is behind that
	// ink rather than up against it.
	//
	// The rail spends more than the pad on its leading edge — a fixed
	// column for the disclosure mark, so names line up per level whether
	// or not a row has a fold to offer. This column has no such mark, so
	// it reserves no column for one: what a row here spends beyond the pad
	// is its heading's own depth and nothing else.
	//
	// asideRowInsetDp is what is left of the rail's other number here: the
	// air a row keeps around its ink on the vertical — half of it above
	// and below a heading's line, the whole of it around the line a
	// citation centres in. The rail spends that number across the
	// horizontal too, holding its pill off the pane's edge — an edge the
	// pane has, drawn and rounded, for a fill to stand off. This column's
	// panes have no such edge. Held inboard, their pill stood a step in
	// from the one line the column does draw — the heading, the citations'
	// head and the hairline between them — and lined up with nothing at
	// all. So the fill takes that line as its own leading edge, and the pad
	// is the whole of what stands between the fill and the ink it is
	// behind.
	asideRowInsetDp = 8
	asideRowPadDp   = 8
	// asideGroupGapDp is what stands either side of the rule between the
	// two panes: enough that the rule reads as parting them rather than as
	// underlining the pane above it.
	asideGroupGapDp = 12
	// asideIndentDp is one heading level's step in the outline. It is
	// small on purpose — six levels of it must still leave a title room to
	// be read in a column this narrow.
	asideIndentDp = 10
	// The row fills are the sidebar's pill, in the sidebar's geometry: a
	// fresh reviewer, shown the whole window, read the column's old
	// full-bleed grey band and the sidebar's purple pill as two design
	// languages for one idea. They are one language now, and the two
	// colours mean in this column what they mean in that one.
	asidePillVPadDp   = 2
	asidePillRadiusDp = 8
)

// asideBacklinkCap is how many rows the backlinks pane may stand tall.
// The pane is the height of its own rows up to this many and no more: a
// note cited twice spends two rows on it and hands the rest of the column
// back to the outline, a note cited twenty times spends four and scrolls
// the other sixteen. Four is the count the owner's own vault reads as a
// list rather than as a second column — enough that the ordinary note
// shows every citation it has without the pane ever becoming the thing
// the column is mostly made of.
const asideBacklinkCap = 4

// asideBacklinkShare is the largest share of a short column the backlinks
// pane may take, whatever the cap says. It is not the sizing rule — the
// cap is — but the rule for a window dragged down to a few rows tall,
// where four rows of citations would leave the outline nothing at all and
// the reader would not know there was a pane above.
const asideBacklinkShare = 2

// backlinkRow is one citing note in the lower pane.
type backlinkRow struct {
	Idx    int    // position in the row slice
	Path   string // vault-relative path Navigate opens
	Title  string // the note's display name
	Folder string // the citing note's folder, "" at the vault root
}

// asideGeom is the column's internal stacking as one layout arranged it,
// in column coordinates: the region the outline holds, from under its own
// heading down to the rule, and the band the backlinks pane occupies at
// the column's foot. Each pane's rows start at the top of its own region,
// so a row is still found by counting row heights down from the region's
// leading edge. The two meet and do not overlap, which is what keeps each
// pane's scroll its own.
type asideGeom struct {
	outline   image.Rectangle
	backlinks image.Rectangle
}

// asideView is the column's widget state: a scroll state and per-row
// clickables for each pane (pointer-stable across frames), the memoised
// rows, and the note column's document, which the outline reads its mark
// from and moves when an entry is chosen.
type asideView struct {
	cur *docCursor

	outlineList  *list.State
	outlineClick []*widget.Clickable

	list      *list.State
	rowClicks []*widget.Clickable

	// Memo key and value: Backlinks re-runs only when the index pointer
	// or the current note changes.
	memoIdx *Index
	memoCur string
	memo    []backlinkRow

	// The outline is rebuilt only when the note it describes is replaced.
	// Notes are never mutated, so a different pointer is the only way the
	// headings can have changed.
	outNote *Note
	outline []outlineEntry

	// marked is the outline entry the last frame found the reader inside.
	// The mark is written into the pane's selection, and only when it
	// moves, so that arrowing through the outline is not overwritten on
	// the next frame by a document that has not gone anywhere.
	marked int

	// choice is the entry the reader chose themselves, which stands over
	// the document's own answer for as long as they leave the note where
	// choosing it put them. It is what lets the closing headings of a note
	// be chosen at all; see outlineChoice.
	choice outlineChoice

	geom asideGeom
}

// backlinks returns the citing-note rows for the model, recomputing only
// when the index or the current note moved.
func (v *asideView) backlinks(m Model) []backlinkRow {
	if m.Index != v.memoIdx || m.Current != v.memoCur {
		v.memoIdx = m.Index
		v.memoCur = m.Current
		paths := Backlinks(m.Index, m.Current)
		v.memo = make([]backlinkRow, len(paths))
		for i, p := range paths {
			folder := path.Dir(p)
			if folder == "." {
				folder = ""
			}
			v.memo[i] = backlinkRow{Idx: i, Path: p, Title: noteTitle(p), Folder: folder}
		}
	}
	return v.memo
}

// headings returns the current note's outline, rebuilt only when the note
// value behind it is replaced.
func (v *asideView) headings(m Model) []outlineEntry {
	n := m.CurrentNote()
	if n != v.outNote {
		v.outNote = n
		v.outline = noteOutline(n)
		v.marked = -2 // no mark has been written for this note yet
		v.choice.drop()
	}
	return v.outline
}

// asideColumn builds the aside slot's widget stream. The frame closure
// reads the model and token snapshots at frame time; repaints on model
// change are driven by the routed layer's re-emission. cur is the note
// column's live document, shared so the outline can mark and move it.
func asideColumn(cur *docCursor, loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := newAsideView(cur)
		return rx.Of[layout.Widget](func(gtx layout.Context) layout.Dimensions {
			return v.layout(gtx, loadModel(), loadTok())
		})
	})
}

// newAsideView allocates the column's state with both panes' scroll
// states and the note column's document to read.
func newAsideView(cur *docCursor) *asideView {
	return &asideView{cur: cur, outlineList: list.NewState(), list: list.NewState(), marked: -2}
}

// layout stacks the two panes: the outline above, a rule, the backlinks
// below. The backlinks group — the rule, the header and the pane — stands
// on the column's foot, and the outline's region is everything left above
// it, from under its own heading down to the rule. The pane is the height
// of its own rows and no more, up to the cap; the outline's rows sit at
// the top of their region, so the paper neither pane needs opens below
// the outline's rows and above the rule.
//
// That is the split's whole rule, and it is what makes the column read as
// two lists of what the note has rather than as two halves of a column. A
// note with three headings and one citation used to leave both panes
// huddled under the column's own heading with the whole lower half of the
// column empty beneath them, which reads as the column having stopped
// rather than as two panes with room to spare. The room is the outline's
// whether it has rows to put in it or not, because the backlinks are what
// a reader keeps the column open for on a note with no headings at all,
// and they are easiest to find in the one place they can always be.
func (v *asideView) layout(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	entries := v.headings(m)
	rows := v.backlinks(m)
	size := gtx.Constraints.Max
	// Three sides of the column's inset, and the trailing one spent inside
	// the panes instead: the bar's lane runs to the window's own edge the
	// way the note's runs to its column's, and what the panes ink stops a
	// lane short of it. See asideIndicator.
	layout.Inset{Top: asideInsetDp, Bottom: asideInsetDp, Left: asideInsetDp}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		inner := gtx.Constraints.Max
		rowH := gtx.Dp(list.RowHeight(tok.den))
		// The backlinks pane: its rows up to the cap, and one row for the
		// line that stands in for them when the note is cited by nothing —
		// an empty pane reserving four rows is a shape this column does not
		// have, at the foot no less than anywhere else.
		backH := rowH
		if len(rows) > 0 {
			backH = min(len(rows), asideBacklinkCap) * rowH
		}
		if ceiling := inner.Y / asideBacklinkShare; backH > ceiling {
			backH = max(ceiling, 0)
		}

		// The stacking measures itself as it is laid out. Rigid children
		// run before flexed ones whatever order they are declared in, so
		// the heights are gathered in two buckets — what stands above the
		// outline's region and what stands below it — rather than by one
		// running total, which would count the group before the region it
		// comes after.
		var above, group, outlineH, backRows int
		rigid := func(into *int, w layout.Widget) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d := w(gtx)
				*into += d.Size.Y
				return d
			})
		}
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			rigid(&above, asideTrailing(tok, func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "Outline", tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
			})),
			rigid(&above, complayout.VSpacer(asideHeaderGapDp)),
			// Whatever the group below has left over, and never less than
			// nothing: the flex pays the rigid children first, so a window
			// squeezed to a few rows tall still shows that there is a
			// second pane down there. The region is exact and the rows lead
			// it, which is what puts the slack under them.
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(inner.X, max(gtx.Constraints.Max.Y, 0)))
				d := v.outlinePane(gtx, tok, entries)
				outlineH = d.Size.Y
				return d
			}),
			rigid(&group, complayout.VSpacer(asideGroupGapDp)),
			rigid(&group, asideTrailing(tok, func(gtx layout.Context) layout.Dimensions {
				return asideRule(gtx, tok)
			})),
			rigid(&group, complayout.VSpacer(asideGroupGapDp)),
			rigid(&group, asideTrailing(tok, func(gtx layout.Context) layout.Dimensions {
				return asideBacklinkHeader(gtx, tok, len(rows))
			})),
			rigid(&group, complayout.VSpacer(asideHeaderGapDp)),
			rigid(&group, func(gtx layout.Context) layout.Dimensions {
				h := min(backH, max(gtx.Constraints.Max.Y, 0))
				gtx.Constraints = layout.Exact(image.Pt(inner.X, h))
				d := v.backlinkPane(gtx, tok, rows)
				backRows = d.Size.Y
				return d
			}),
		)
		off := gtx.Dp(unit.Dp(asideInsetDp))
		backTop := above + outlineH + group - backRows
		v.geom = asideGeom{
			outline:   image.Rect(off, off+above, off+inner.X, off+above+outlineH),
			backlinks: image.Rect(off, off+backTop, off+inner.X, off+backTop+backRows),
		}
		return layout.Dimensions{Size: inner}
	})
	return layout.Dimensions{Size: size}
}

// asidePill fills a row's mark: a rounded pill on the column's own ink
// margin, running the band the pane's headings and its hairline run and
// no further. It is the sidebar's fill — the same shape, the same radius,
// the same two colours — so one window has one way of saying a row is
// spoken for; where it stands is this column's own, because this column's
// panes have no drawn edge of their own for a fill to stand off. See
// asideRowInsetDp.
func asidePill(gtx layout.Context, size image.Point, fill color.NRGBA) {
	vp := gtx.Dp(unit.Dp(asidePillVPadDp))
	r := gtx.Dp(unit.Dp(asidePillRadiusDp))
	rect := image.Rect(0, vp, size.X, size.Y-vp)
	if rect.Empty() {
		return
	}
	pill := clip.RRect{Rect: rect, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, fill, pill.Op(gtx.Ops))
}

// asideBacklinkHeader is the lower pane's heading, with the number of
// citations beside it whenever the note has any.
//
// The cap is what makes the number worth drawing. Four rows of twenty
// look exactly like four rows of four, and the indicator that says
// otherwise is a hairline that fades a second after the pane stops
// moving — so a reader given the pane at rest would read it as the whole
// answer and never learn that sixteen more are a scroll away. The number
// is there below the cap as well, where it is merely true rather than
// necessary: a count that appeared only when the list was long would be
// a figure the reader cannot learn to trust, since its absence would
// have to be read as "few" rather than as "not counted".
func asideBacklinkHeader(gtx layout.Context, tok themeTokens, n int) layout.Dimensions {
	ink := tok.col.Ramps.Neutral.Step(700)
	title := func(gtx layout.Context) layout.Dimensions {
		return drawLabel(gtx, tok.shaper, "Backlinks", tok.typ.TitleSmall, ink)
	}
	if n == 0 {
		// The pane's own line already says none; a nought beside the
		// heading would say it twice.
		return title(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(title),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawLabel(gtx, tok.shaper, strconv.Itoa(n), tok.typ.TitleSmall, ink)
		}),
	)
}

// asideIndicator is the scroll indicator both panes carry, taken from the
// same colour tokens the note column takes its own from — the design
// system's treatment rather than a drawing of this column's, so that one
// window has one way of saying "there is more of this below". It draws
// nothing at all while a pane's rows fit, and fades a second after the
// pane stops moving; both are the treatment's own behaviour. The thumb is
// the treatment's own too: the same six dp of the same translucent
// neutral the note column's bar is drawn in.
//
// What this column states is the lane that thumb stands in: the frame's
// edge margin either side of the bar. The outboard eight is the note's
// bar, measured. Its column runs out where this one's surface begins, and
// it stands eight dp inside that seam — so with the same eight against
// the window's own edge, the two bars a reader reads between keep one
// distance from the ground each runs out of. They kept two: this one
// floated eighteen dp off the window's edge where the note's stands eight
// off its own, which is a window with two ideas about where a bar goes.
// The inboard eight is what the rows needed. They carry the mark, and a
// fill whose edge is a bar's edge is a fill with a bar stuck to it — the
// arrangement a walkthrough measured at a single pixel of daylight.
//
// It is the track padding that is stated, because padding either side of
// the thumb is what a lane is. The thumb itself is untouched.
//
// Occupy rather than Overlay follows from that: the lane is the column's
// trailing margin, so the rows stop where it starts and nothing is drawn
// under a bar. It costs the rows nothing they were using — the fill they
// carry now runs to the ink margin it used to hang inboard of, and comes
// out wider than the floating arrangement left it.
func asideIndicator(tok themeTokens) scrollbar.Style {
	s := scrollbar.FromTokens(tok.col)
	s.TrackPadding = railMarginDp
	return s
}

// asideBarLane is the trailing band a pane hands its scrollbar: the
// thumb's own width and the air either side of it. It is what the rows
// stop short of, and what everything else the column inks — the two
// headings, the citation count, the hairline between the panes — takes as
// its own trailing inset, so that one right edge runs down the column
// whether a pane is scrolling or not.
func asideBarLane(tok themeTokens) unit.Dp { return asideIndicator(tok).Width() }

// asideTrailing insets a widget by the bar's lane, which is what puts what
// it draws on the column's own trailing edge.
func asideTrailing(tok themeTokens, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: asideBarLane(tok)}.Layout(gtx, w)
	}
}

// asideRule is the hairline parting the two panes. It is a hairline for
// the reason the sidebar's foot uses one: both panes stand on the same
// surface, so there are no two grounds to separate, only a seam saying
// one pane's scrolling stops here and another's begins.
func asideRule(gtx layout.Context, tok themeTokens) layout.Dimensions {
	h := max(gtx.Dp(unit.Dp(1)), 1)
	w := gtx.Constraints.Min.X
	paint.FillShape(gtx.Ops, tok.col.Divider, clip.Rect{Max: image.Pt(w, h)}.Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// outlinePane lays out the note's headings, marking the one the reader is
// inside and moving the document to whichever is chosen.
//
// The mark is the pane's selection, written from the document's own
// leading block each frame — so it follows the note as it is scrolled by
// any means at all, the wheel and the reading keys included, without the
// outline having to know which. It is written only when it changes, which
// leaves the arrow keys usable inside the pane: a reader may run down the
// outline while the document stands still, and the moment the document
// moves the mark returns to where they are actually reading.
//
// A chosen entry is the one exception, and it is the reader's: what they
// pressed is marked until they move the note again, whether or not the
// document could put that heading at the top. The leading block cannot
// answer for the closing headings of a note — the document runs out before
// they reach the top — and an entry that cannot be marked is an entry that
// cannot be chosen. See outlineChoice.
func (v *asideView) outlinePane(gtx layout.Context, tok themeTokens, entries []outlineEntry) layout.Dimensions {
	if len(entries) == 0 {
		// Released from the region's minimum height, so the line sits under
		// its own heading rather than in the middle of the room the region
		// holds. The region is still returned whole: the room below the
		// line is the outline's to stand empty in, which is what keeps the
		// backlinks on the column's foot.
		gtx.Constraints.Min.Y = 0
		region := gtx.Constraints.Max
		layout.Inset{Right: asideBarLane(tok)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawText(gtx, tok.shaper, "This note has no headings.", tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
		})
		return layout.Dimensions{Size: region}
	}
	if doc := v.cur.document(); doc != nil {
		first := doc.Position().First
		at, chosen := v.choice.stands(doc, first)
		if !chosen {
			at = outlineActive(entries, first)
		}
		if at != v.marked {
			v.marked = at
			v.outlineList.Select(at)
			// A chosen entry was pressed, so it is already in view; only a
			// mark the document moved under the reader has to be brought
			// there.
			if at >= 0 && !chosen {
				v.outlineList.Reveal(at)
			}
		}
	}
	// Return activates the selected entry. The list consumes the arrows;
	// activation is the caller's semantics, filtered on the list's own
	// focus tag so it cannot answer for the document or the folder rail.
	for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
		for {
			e, ok := gtx.Event(key.Filter{Focus: v.outlineList.Focus(), Name: name})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				if sel := v.outlineList.Selected(); sel >= 0 && sel < len(entries) {
					v.seek(gtx, entries[sel])
				}
			}
		}
	}
	for len(v.outlineClick) < len(entries) {
		v.outlineClick = append(v.outlineClick, &widget.Clickable{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	return list.LayoutSelectableScrollbar(gtx, v.outlineList, asideIndicator(tok), list.Occupy, entries,
		func(gtx layout.Context, e outlineEntry, selected bool) layout.Dimensions {
			click := v.outlineClick[e.Idx]
			if click.Clicked(gtx) {
				v.outlineList.Select(e.Idx)
				v.seek(gtx, e)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				// Two states, the sidebar's two: the heading the reader is
				// inside wears the "you are here" fill the open note wears
				// in the rail, and a row merely arrowed onto in this pane
				// wears the neutral one. They part because the reader can
				// move the mark off where they are actually reading.
				switch {
				case e.Idx == v.marked:
					asidePill(gtx, size, tok.col.Ramps.Primary.Step(300))
				case selected:
					asidePill(gtx, size, tok.col.Ramps.Neutral.Step(300))
				}
				semantic.LabelOp(e.Title).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				// The pill's edge, then the pad, then the heading's own
				// depth: a top-level heading stands a pad inside its own
				// fill the way every filled row in this window does, and
				// each level below it steps in from there. The pad is on
				// the trailing edge too, so a title long enough to be cut
				// is cut inside the pill rather than at it.
				lead := asideRowPadDp + max(e.Level-1, 0)*asideIndentDp
				layout.Inset{
					Left: unit.Dp(lead), Right: asideRowPadDp,
					Top: asideRowInsetDp / 2, Bottom: asideRowInsetDp / 2,
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					// A first-level heading is the note's own title level
					// and is set apart from what hangs under it.
					style := tok.typ.BodyMedium
					ink := tok.col.Text
					if e.Level > 1 {
						ink = tok.col.Ramps.Neutral.Step(800)
					}
					return drawLabel(gtx, tok.shaper, e.Title, style, ink)
				})
				return layout.Dimensions{Size: size}
			})
		})
}

// seek takes the reader to an entry's heading by moving the document that
// is already open — never by navigating to it, which would push a history
// entry for a note nobody left. The frame is invalidated because the note
// column laid out before this one did: the move it was just asked for
// shows on the next frame, and without the invalidation there need not be
// one.
func (v *asideView) seek(gtx layout.Context, e outlineEntry) {
	doc := v.cur.document()
	if doc == nil {
		return
	}
	doc.ScrollToBlock(e.Block)
	// The mark goes with the reader immediately rather than waiting for
	// the document to report its new leading block, which it cannot do
	// until it has laid out again. The choice is what keeps it there: the
	// document may not be able to put this heading at the top, and where it
	// does come to rest reads as some earlier section.
	v.choice.take(doc, e.Idx)
	v.marked = e.Idx
	v.outlineList.Select(e.Idx)
	gtx.Execute(op.InvalidateCmd{})
}

// backlinkPane lays out the citing notes: the note title leading, its
// folder as the quiet trailing annotation when it has one.
func (v *asideView) backlinkPane(gtx layout.Context, tok themeTokens, rows []backlinkRow) layout.Dimensions {
	if len(rows) == 0 {
		// Released from the flex child's minimum height, so the line sits
		// under its own header instead of being centred in the whole empty
		// column — where a fresh reviewer read it as belonging to nothing.
		gtx.Constraints.Min.Y = 0
		region := gtx.Constraints.Max
		layout.Inset{Right: asideBarLane(tok)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawText(gtx, tok.shaper, "No notes link here.", tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
		})
		return layout.Dimensions{Size: region}
	}
	for len(v.rowClicks) < len(rows) {
		v.rowClicks = append(v.rowClicks, &widget.Clickable{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	return list.LayoutSelectableScrollbar(gtx, v.list, asideIndicator(tok), list.Occupy, rows,
		func(gtx layout.Context, row backlinkRow, selected bool) layout.Dimensions {
			click := v.rowClicks[row.Idx]
			if click.Clicked(gtx) {
				v.list.Select(row.Idx)
				mvu.MessageOp{Message: Navigate{Path: row.Path}}.Add(gtx.Ops)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				if selected {
					asidePill(gtx, size, tok.col.Ramps.Neutral.Step(300))
				}
				semantic.LabelOp(row.Title).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				// The rows below the rule take the same pad the rows above
				// it do, for the same reason: they wear the same pill.
				complayout.InsetXY(asideRowPadDp, asideRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return drawLabel(gtx, tok.shaper, row.Title, tok.typ.BodyMedium, tok.col.Text)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if row.Folder == "" {
								return layout.Dimensions{}
							}
							return drawLabel(gtx, tok.shaper, row.Folder, tok.typ.BodySmall, tok.col.Ramps.Neutral.Step(700))
						}),
					)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
				return layout.Dimensions{Size: size}
			})
		})
}

// docCursor is the one document the note column has on screen, shared
// with the aside. The note column writes it as it lays the document out;
// the outline reads it to say where in the note the reader is, and writes
// through it to take them somewhere else in the same note. Both run on
// the frame goroutine and nothing else touches it.
//
// It is a shared pointer rather than a round trip through the model
// because what it carries is not model state: the scroll position is the
// widget's, it changes on every frame of a wheel gesture, and a note that
// went through the update loop to be scrolled would be a note reloaded.
type docCursor struct{ doc *markdown.Document }

// document is the note column's current document, nil before the first
// note is laid out and whenever a message stands in place of one.
func (c *docCursor) document() *markdown.Document {
	if c == nil {
		return nil
	}
	return c.doc
}

// show records the document the note column is laying out this frame.
func (c *docCursor) show(d *markdown.Document) {
	if c != nil {
		c.doc = d
	}
}
