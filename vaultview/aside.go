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
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
)

// Aside layout constants.
const (
	asideInsetDp     = 16
	asideHeaderGapDp = 8
	asideRowInsetDp  = 8
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

// asideOutlineShare is the largest share of the column the outline pane
// may take. The backlinks keep the rest whatever the note's shape: a
// forty-heading note may not bury them, and a note with three headings
// does not hold space it has no use for, because the pane asks only for
// what its rows need and this is a ceiling, not a size.
const asideOutlineShare = 2

// backlinkRow is one citing note in the lower pane.
type backlinkRow struct {
	Idx    int    // position in the row slice
	Path   string // vault-relative path Navigate opens
	Title  string // the note's display name
	Folder string // the citing note's folder, "" at the vault root
}

// asideGeom is the column's internal stacking as one layout arranged it:
// the band each pane's rows occupy, in column coordinates. The two meet
// and do not overlap, which is what keeps each pane's scroll its own.
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
// below. The outline pane takes the height its rows need and no more, up
// to a share of the column; everything left over is the backlinks'. That
// is the split's whole rule, and it is a ceiling rather than a division
// so that the ordinary note — a handful of headings and a handful of
// citations — spends the column on rows rather than on two half-empty
// halves.
func (v *asideView) layout(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	entries := v.headings(m)
	rows := v.backlinks(m)
	size := gtx.Constraints.Max
	complayout.Inset(asideInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		inner := gtx.Constraints.Max
		rowH := gtx.Dp(list.RowHeight(tok.den))
		outlineH := rowH // the empty state's own line
		if len(entries) > 0 {
			outlineH = len(entries) * rowH
		}
		if ceiling := inner.Y / asideOutlineShare; outlineH > ceiling {
			outlineH = max(ceiling, 0)
		}

		var y, outlineTop, outlineRows, backTop int
		rigid := func(w layout.Widget) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d := w(gtx)
				y += d.Size.Y
				return d
			})
		}
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			rigid(func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "Outline", tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
			}),
			rigid(complayout.VSpacer(asideHeaderGapDp)),
			rigid(func(gtx layout.Context) layout.Dimensions {
				outlineTop = y
				// Never more than the flex has left to give, whatever the
				// ceiling said: a window squeezed to a few rows tall must
				// still show that there is a second pane under this one.
				h := min(outlineH, max(gtx.Constraints.Max.Y, 0))
				gtx.Constraints = layout.Exact(image.Pt(inner.X, h))
				d := v.outlinePane(gtx, tok, entries)
				outlineRows = d.Size.Y
				return d
			}),
			rigid(complayout.VSpacer(asideGroupGapDp)),
			rigid(func(gtx layout.Context) layout.Dimensions {
				return asideRule(gtx, tok)
			}),
			rigid(complayout.VSpacer(asideGroupGapDp)),
			rigid(func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "Backlinks", tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
			}),
			rigid(complayout.VSpacer(asideHeaderGapDp)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				backTop = y
				return v.backlinkPane(gtx, tok, rows)
			}),
		)
		off := gtx.Dp(unit.Dp(asideInsetDp))
		v.geom = asideGeom{
			outline:   image.Rect(off, off+outlineTop, off+inner.X, off+outlineTop+outlineRows),
			backlinks: image.Rect(off, off+backTop, off+inner.X, off+inner.Y),
		}
		return layout.Dimensions{Size: inner}
	})
	return layout.Dimensions{Size: size}
}

// asidePill fills a row's mark: a rounded pill held off the column's ink
// margin, never a bar running edge to edge. It is the sidebar's fill in
// the sidebar's geometry, so one window has one way of saying a row is
// spoken for.
func asidePill(gtx layout.Context, size image.Point, fill color.NRGBA) {
	ins := gtx.Dp(unit.Dp(asideRowInsetDp))
	vp := gtx.Dp(unit.Dp(asidePillVPadDp))
	r := gtx.Dp(unit.Dp(asidePillRadiusDp))
	rect := image.Rect(ins, vp, size.X-ins, size.Y-vp)
	if rect.Empty() {
		return
	}
	pill := clip.RRect{Rect: rect, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, fill, pill.Op(gtx.Ops))
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
func (v *asideView) outlinePane(gtx layout.Context, tok themeTokens, entries []outlineEntry) layout.Dimensions {
	if len(entries) == 0 {
		gtx.Constraints.Min.Y = 0
		drawText(gtx, tok.shaper, "This note has no headings.", tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	if doc := v.cur.document(); doc != nil {
		if at := outlineActive(entries, doc.Position().First); at != v.marked {
			v.marked = at
			v.outlineList.Select(at)
			if at >= 0 {
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
	return list.LayoutSelectable(gtx, v.outlineList, entries,
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
				// The indent is the heading's depth; the top level sits on
				// the same ink margin the pane's own headings do, so the
				// column reads as one grid.
				lead := asideRowInsetDp + max(e.Level-1, 0)*asideIndentDp
				layout.Inset{
					Left: unit.Dp(lead), Right: asideRowInsetDp,
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
	// until it has laid out again — and it is where the document is about
	// to be, so the next frame's reading agrees and writes nothing.
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
		drawText(gtx, tok.shaper, "No notes link here.", tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	for len(v.rowClicks) < len(rows) {
		v.rowClicks = append(v.rowClicks, &widget.Clickable{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	return list.LayoutSelectable(gtx, v.list, rows,
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
				complayout.Inset(asideRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
