package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/keyed"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/accordion"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/patterns/toast"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	feedsSidebarWidthDp = 192
	feedsEntryRowHDp    = 28
	feedsEntryIndentDp  = 24
	trashColWDp         = 24 // trailing trash-icon hit area, hover-revealed
)

// feedsSidebar returns the accordion-grouped feeds sidebar observable.
// openSectionsObs streams the current open-section map from the MVU model;
// feedsObs streams the current (mutable) feed tree. The accordion section
// count is fixed — added feeds join an existing group and deletions leave the
// (possibly empty) section in place — so Sections is built once and each
// section's Body renders the CURRENT entries from a per-section atomic cell
// updated by every feedsObs emission. OnToggle emits a ToggleSection message;
// entry clicks emit SelectFeed; the hover-revealed trash icon opens a
// per-row delete-confirm popover whose confirm fires ConfirmDelete + a toast.
//
// G5.2d note: the feed tree was a static fixture before this goal. It now
// lives in the Model so add/delete mutate it; feedEntryListBody therefore
// reads the live slice each frame and keys its per-entry widget state by
// FeedID via components/keyed so add/delete never re-binds a clickable to the
// wrong row.
func feedsSidebar(
	th rx.Observable[theme.Theme],
	openSectionsObs rx.Observable[map[int]bool],
	feedsObs rx.Observable[[]feedGroup],
	popArb *popover.Arbiter,
) rx.Observable[layout.Widget] {
	// The fixed section count drives the per-section entry cells. The set of
	// groups (titles, count) never changes; only their Entries do.
	groups := hardCodedGroups()
	sectionCells := make([]atomic.Value, len(groups))
	for i := range sectionCells {
		sectionCells[i].Store(groups[i].Entries)
	}

	accSections := make([]accordion.Section, len(groups))
	for i, g := range groups {
		cell := &sectionCells[i]
		accSections[i] = accordion.Section{
			Title: g.Title,
			Body: feedEntryListBody(th, func() []feedEntry {
				if e, ok := cell.Load().([]feedEntry); ok {
					return e
				}
				return nil
			}, popArb),
		}
	}

	// SingleOpen is false: the patterns accordion emits exactly one
	// ToggleSection per click, and feeds.Update owns the single-open invariant
	// (opening a section closes its peers). One message per click keeps the
	// model update — and the same-frame repaint it drives — to a single hop,
	// rather than the N+1 OnToggle calls SingleOpen mode fires.
	accObs := accordion.Accordion(th, accordion.Props{
		Sections: accSections,
		Open:     openSectionsObs,
		OnToggle: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: ToggleSection{Idx: idx}}.Add(gtx.Ops)
		},
		SingleOpen: false,
	})

	// Fold the accordion, the live feed tree, and a theme token together. The
	// feeds emission updates the per-section cells (read by the bodies above)
	// before the accordion widget is returned, so a delete/add re-emits this
	// layer — driving theme/window's Invalidate() and the same-frame
	// repaint, the same way the open-section map drives it.
	colorsObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(
		rx.CombineLatest3(accObs, feedsObs, colorsObs),
		func(n rx.Tuple3[layout.Widget, []feedGroup, tokens.ColorTokens]) layout.Widget {
			accW, feeds, c := n.First, n.Second, n.Third
			for i := range sectionCells {
				if i < len(feeds) {
					sectionCells[i].Store(feeds[i].Entries)
				} else {
					sectionCells[i].Store([]feedEntry(nil))
				}
			}
			return func(gtx layout.Context) layout.Dimensions {
				return drawFeedsSidebar(gtx, accW, c)
			}
		},
	)
}

func drawFeedsSidebar(
	gtx layout.Context,
	accW layout.Widget,
	colors tokens.ColorTokens,
) layout.Dimensions {
	w := gtx.Dp(unit.Dp(feedsSidebarWidthDp))
	h := gtx.Constraints.Max.Y
	size := image.Pt(w, h)
	paint.FillShape(gtx.Ops, colors.Surface, clip.Rect{Max: size}.Op())
	gtx.Constraints = layout.Exact(size)
	if accW != nil {
		accW(gtx)
	}
	return layout.Dimensions{Size: size}
}

// feedEntryListBody returns the body widget for a single accordion section.
// entriesFn yields the section's CURRENT entries each frame (read from the
// per-section model cell). Entry clicks emit SelectFeed; hovering a row
// reveals a trash icon whose click toggles a per-row delete-confirm popover,
// whose confirm fires ConfirmDelete + a "Feed deleted" toast.
//
// All per-entry widget state (the row clickable, the trash clickable, the
// confirm clickable, the per-row open flag) is keyed by FeedID so add/delete
// never re-binds state to the wrong row.
func feedEntryListBody(
	th rx.Observable[theme.Theme],
	entriesFn func() []feedEntry,
	popArb *popover.Arbiter,
) layout.Widget {
	loadTok := mirrorTokens(th)

	// Per-FeedID widget state, stable across list mutation.
	rowClicks := keyed.Defer(func(FeedID) *widget.Clickable { return &widget.Clickable{} })
	trashClicks := keyed.Defer(func(FeedID) *widget.Clickable { return &widget.Clickable{} })
	confirmClicks := keyed.Defer(func(FeedID) *widget.Clickable { return &widget.Clickable{} })
	// hover is per-row pointer hover state; ephemeral, lives in the closure.
	// gesture.Hover only filters Enter/Leave (never Press), so it does NOT
	// swallow the select press the way a full-row widget.Clickable would —
	// click-to-select stays intact under the hover-reveal trash gutter.
	hovers := keyed.Defer(func(FeedID) *gesture.Hover { return &gesture.Hover{} })
	// The per-row delete-confirm popovers. Each holds its open flag as a
	// plain bool on the frame goroutine — ephemeral interaction state, not
	// model state, keyed by FeedID — and patterns/popover reads it during
	// layout through Props.OpenNow (ADR-008 destination 2). They all share
	// this window's Arbiter, so opening one row's confirm dismisses whichever
	// row had it open.
	popovers := keyed.Defer(func(id FeedID) *deleteConfirm {
		return newDeleteConfirm(th, id, trashClicks.For(id), confirmClicks.For(id), popArb)
	})

	return func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		entries := entriesFn()
		size := gtx.Constraints.Max
		rowH := gtx.Dp(unit.Dp(feedsEntryRowHDp))
		indent := gtx.Dp(unit.Dp(feedsEntryIndentDp))
		trashW := gtx.Dp(unit.Dp(trashColWDp))

		for _, e := range entries {
			rc := rowClicks.For(e.ID)
			if rc.Clicked(gtx) {
				mvu.MessageOp{Message: SelectFeed{Feed: e.ID}}.Add(gtx.Ops)
			}
		}

		for i, e := range entries {
			off := image.Pt(indent, i*rowH)
			stk := op.Offset(off).Push(gtx.Ops)
			rowGtx := gtx
			rowGtx.Constraints = layout.Exact(image.Pt(size.X-indent, rowH))
			drawFeedEntryRow(rowGtx, s, e, rowClicks.For(e.ID),
				hovers.For(e.ID), popovers.For(e.ID), trashW)
			stk.Pop()
		}
		return layout.Dimensions{Size: size}
	}
}

// drawFeedEntryRow paints one feed row: the label (left) and a hover-revealed
// trash icon + delete-confirm popover (right). hover holds the row's pointer
// hover state; the trash icon paints only while hovered (or while its confirm
// popover is open, so the popover never floats over an un-hovered row).
func drawFeedEntryRow(
	gtx layout.Context,
	tok themeTokens,
	e feedEntry,
	click *widget.Clickable,
	hover *gesture.Hover,
	dc *deleteConfirm,
	trashW int,
) layout.Dimensions {
	size := gtx.Constraints.Max

	// Hover tracking spans the whole row but registers ONLY Enter/Leave (via
	// gesture.Hover), so it never claims the select press. Register the hover
	// area first so it sits under the label/trash content.
	hovered := hover.Update(gtx.Source) || dc.open
	hoverClip := clip.Rect{Max: size}.Push(gtx.Ops)
	hover.Add(gtx.Ops)
	hoverClip.Pop()

	// Label fills the row minus the trash gutter; the label area is the
	// SelectFeed click target. Body text sits on the Neutral ramp's 900
	// step (ADR-007), in the theme's BodySmall role.
	labelW := size.X - trashW
	if labelW < 0 {
		labelW = 0
	}
	labelGtx := gtx
	labelGtx.Constraints = layout.Exact(image.Pt(labelW, size.Y))
	drawFeedEntry(labelGtx, tok, e.Label, click)

	// Trash gutter + confirm popover, right-aligned.
	trStk := op.Offset(image.Pt(size.X-trashW, 0)).Push(gtx.Ops)
	trGtx := gtx
	trGtx.Constraints = layout.Exact(image.Pt(trashW, size.Y))
	dc.layout(trGtx, hovered)
	trStk.Pop()

	return layout.Dimensions{Size: size}
}

func drawFeedEntry(
	gtx layout.Context,
	tok themeTokens,
	label string,
	click *widget.Clickable,
) layout.Dimensions {
	size := gtx.Constraints.Max
	inner := func(gtx layout.Context) layout.Dimensions {
		labelGtx := gtx
		labelGtx.Constraints.Min = image.Point{}
		labelGtx.Constraints.Max = size
		mLabel := op.Record(gtx.Ops)
		labelDims := drawLabel(labelGtx, tok.shaper, label, tok.typ.BodySmall, tok.col.Ramps.Neutral.Step(900))
		labelCall := mLabel.Stop()
		offY := (size.Y - labelDims.Size.Y) / 2
		if offY < 0 {
			offY = 0
		}
		stk := op.Offset(image.Pt(0, offY)).Push(gtx.Ops)
		labelCall.Add(gtx.Ops)
		stk.Pop()
		return layout.Dimensions{Size: size}
	}
	gtx.Constraints = layout.Exact(size)
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		return inner(gtx)
	})
}

// deleteConfirm owns one feed row's delete-confirm popover: the trash-icon
// anchor and a "Delete this feed?" confirm surface. Open state is ephemeral
// per-row interaction state — a plain bool this struct owns, written and read
// during layout on the frame goroutine, which patterns/popover reads back
// through Props.OpenNow (ADR-008 destination 2). Nothing outside the frame
// ever asks whether a row's confirm is open, so nothing outside the frame
// holds a copy of the answer. The trash click toggles it; the confirm click
// fires ConfirmDelete + a toast and closes; OnDismiss closes.
//
// Until G0C.4 the flag lived in a per-row rx.Subject with an atomic.Bool
// mirror beside it, and the flip crossed to the rx goroutine and back before
// any frame could see it. The Subject and the mirror are both gone; the
// remaining atomic cell carries the THEME's re-emissions, which really do
// arrive from another goroutine.
//
// The popover is wrapped in an Exact canvas (the trash gutter) so its anchor
// centres on the trash icon and the confirm surface sits below it — the same
// canvas-coupling workaround as the Share popover (logged in FEEDBACK-G5.2.md).
type deleteConfirm struct {
	id FeedID
	// open is frame state: only layout writes it and only layout reads it.
	open bool
	cell atomic.Value // latest popover layout.Widget
}

func newDeleteConfirm(
	th rx.Observable[theme.Theme],
	id FeedID,
	trashClick *widget.Clickable,
	confirmClick *widget.Clickable,
	popArb *popover.Arbiter,
) *deleteConfirm {
	dc := &deleteConfirm{id: id}

	loadTok := mirrorTokens(th)

	anchor := func(gtx layout.Context) layout.Dimensions {
		if trashClick.Clicked(gtx) {
			dc.toggle()
		}
		return trashClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := loadTok()
			semantic.LabelOp("Delete feed").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			sz := gtx.Constraints.Max
			drawTrashIcon(gtx, sz, s.col.Ramps.Neutral.Step(900))
			return layout.Dimensions{Size: sz}
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if confirmClick.Clicked(gtx) {
			toast.Notify(gtx, toast.Success, "Feed deleted")
			mvu.MessageOp{Message: ConfirmDelete{Feed: dc.id}}.Add(gtx.Ops)
			dc.close()
		}
		// Override the incoming canvas/2 constraints: the popover sized the
		// anchor canvas to the tiny trash gutter, so half of it cannot hold a
		// confirm prompt. Size the content ourselves; popover pads it.
		w := gtx.Dp(unit.Dp(deleteConfirmWDp))
		promptH := gtx.Dp(unit.Dp(deleteConfirmRowHDp))
		btnH := gtx.Dp(unit.Dp(deleteConfirmRowHDp))
		drawLabel(gtx, s.shaper, "Delete this feed?", s.typ.BodyMedium, s.col.Ramps.Neutral.Step(900))
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm delete").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			drawLabel(gtx, s.shaper, "Delete", s.typ.LabelLarge, s.col.Error)
			return layout.Dimensions{Size: image.Pt(w, btnH)}
		})
		btnStk.Pop()
		return layout.Dimensions{Size: image.Pt(w, promptH+btnH)}
	}

	popObs := popover.Popover(th, popover.Props{
		OpenNow:   func() bool { return dc.open },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		Arbiter:   popArb,
		OnDismiss: func(layout.Context) { dc.close() },
	})
	dc.cell.Store(layout.Widget(nil))
	_ = popObs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			dc.cell.Store(w)
		}
	})
	return dc
}

// toggle and close run during layout, from the anchor click, the confirm
// click and the arbiter's OnDismiss — all three on the frame goroutine, which
// is what lets open be a plain bool.
func (dc *deleteConfirm) toggle() { dc.open = !dc.open }

func (dc *deleteConfirm) close() { dc.open = false }

// layout draws the trash gutter for one row. When the row is not hovered (and
// the confirm popover is closed), nothing is painted and the gutter is inert.
// When hovered/open the popover widget renders the trash anchor (and, while
// open, the confirm surface) inside the gutter's Exact canvas.
func (dc *deleteConfirm) layout(gtx layout.Context, visible bool) layout.Dimensions {
	size := gtx.Constraints.Max
	if !visible {
		return layout.Dimensions{Size: size}
	}
	if w, ok := dc.cell.Load().(layout.Widget); ok && w != nil {
		w(gtx)
	}
	return layout.Dimensions{Size: size}
}

const (
	deleteConfirmWDp    = 132
	deleteConfirmRowHDp = 28
)

// drawTrashIcon paints a minimal trash glyph (a lid line + a body box) into a
// square the size of the gutter, centred, in colour col. clip.Path/Stroke
// only, so it stays golden-deterministic like the other feeds glyphs.
func drawTrashIcon(gtx layout.Context, box image.Point, col color.NRGBA) {
	side := box.X
	if box.Y < side {
		side = box.Y
	}
	pad := gtx.Dp(unit.Dp(6))
	x0 := (box.X-side)/2 + pad
	x1 := box.X - (box.X-side)/2 - pad
	y0 := (box.Y-side)/2 + pad
	y1 := box.Y - (box.Y-side)/2 - pad
	stroke := float32(gtx.Dp(unit.Dp(1)))
	if stroke < 1 {
		stroke = 1
	}
	lidY := y0 + (y1-y0)/5
	// Lid line.
	rect(gtx, image.Rect(x0, lidY, x1, lidY+int(stroke)+1), col)
	// Body outline (four thin rects).
	rect(gtx, image.Rect(x0, lidY, x0+int(stroke)+1, y1), col)
	rect(gtx, image.Rect(x1-int(stroke)-1, lidY, x1, y1), col)
	rect(gtx, image.Rect(x0, y1-int(stroke)-1, x1, y1), col)
}

func rect(gtx layout.Context, r image.Rectangle, col color.NRGBA) {
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}
