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
	feedsEntryIndentDp  = 16
	trashColWDp         = 24 // trailing trash-icon hit area, hover-revealed
)

// The chosen-feed pill (ADR-021 R5). The fill is inset equally from both
// edges of the sidebar and padded vertically, so it reads as a pill on the
// Surface rather than a full-bleed bar — the shape vaultview's tree and
// sitedocs' outline already draw. Its leading edge is the row's own origin
// and the label is padded in from there: a pill whose text starts on its
// left edge is not a pill, it is a bar with a rounded corner, which is what
// a review of the first draft called out. The indent and the trail are equal
// for the same reason, so the highlight is centred in the rail rather than
// hanging off one side of it, and the trash gutter is measured from the
// pill's trailing edge rather than the row's so the icon stays on the fill.
const (
	feedsPillTrailDp   = 16
	feedsPillVPadDp    = 2
	feedsPillRadiusDp  = 4
	feedsRowLabelPadDp = 8
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
	selectedFeedObs rx.Observable[FeedID],
	popArb *popover.Arbiter,
) rx.Observable[layout.Widget] {
	// The fixed section count drives the per-section entry cells. The set of
	// groups (titles, count) never changes; only their Entries do.
	groups := hardCodedGroups()
	sectionCells := make([]atomic.Value, len(groups))
	for i := range sectionCells {
		sectionCells[i].Store(groups[i].Entries)
	}

	// The open feed, mirrored for the section bodies. accordion.Section.Body
	// is a static layout.Widget slot, so the entry rows cannot be handed the
	// selection in-band; they read it from this cell at frame time, the same
	// layer-boundary hand-off the entry list already uses for its entries.
	// The cell is written in the fold below, BEFORE the emitted widget can be
	// laid out, so no frame paints last selection's pill.
	var selectedCell atomic.Value
	selectedCell.Store(FeedID(""))
	loadSelected := func() FeedID {
		id, _ := selectedCell.Load().(FeedID)
		return id
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
			}, loadSelected, popArb),
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
	// The density is here for the band alone: it is what the navbar's height
	// on the other side of the window's top edge is pinned to, and therefore
	// what the sidebar has to hold open on this side (see windowBandDp).
	densityObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Density] {
		return t.Density
	})
	return rx.Map(
		rx.CombineLatest5(accObs, feedsObs, colorsObs, selectedFeedObs, densityObs),
		func(n rx.Tuple5[layout.Widget, []feedGroup, tokens.ColorTokens, FeedID, tokens.Density]) layout.Widget {
			accW, feeds, c := n.First, n.Second, n.Third
			band := windowBandDp(n.Fifth)
			selectedCell.Store(n.Fourth)
			for i := range sectionCells {
				if i < len(feeds) {
					sectionCells[i].Store(feeds[i].Entries)
				} else {
					sectionCells[i].Store([]feedEntry(nil))
				}
			}
			return func(gtx layout.Context) layout.Dimensions {
				return drawFeedsSidebar(gtx, accW, c, band)
			}
		},
	)
}

// drawFeedsSidebar fills the sidebar's column and lays the accordion out
// below the window's title band.
//
// The band is the sidebar's own top rows rather than something drawn over
// them: this window paints its own title bar, so the sidebar reaches the
// window's top edge and the platform's three control buttons stand in its
// top-leading corner. The fill therefore runs the whole column, band
// included — the band wears the ground of the region it caps, which here is
// the sidebar's Surface — and only the accordion starts below it. Before the
// treatment the first section's header sat at the very corner, exactly where
// the buttons land, which is the collision the window audit read off this
// window.
//
// Nothing is drawn in the band on this side. The window's name is already the
// navbar's brand on the other side of the seam, and a second copy of it under
// the buttons would be the same word twice across one strip.
func drawFeedsSidebar(
	gtx layout.Context,
	accW layout.Widget,
	colors tokens.ColorTokens,
	band unit.Dp,
) layout.Dimensions {
	w := gtx.Dp(unit.Dp(feedsSidebarWidthDp))
	h := gtx.Constraints.Max.Y
	size := image.Pt(w, h)
	// ADR-021 R2: the sidebar is chrome furniture, so it stands exactly one
	// rung above the content it frames — the semantic Surface over the
	// window's Background ground. The token never moved; what moved is the
	// ground under it, which used to be Surface as well.
	paint.FillShape(gtx.Ops, colors.Surface, clip.Rect{Max: size}.Op())
	top := min(max(gtx.Dp(band), 0), h)
	defer op.Offset(image.Pt(0, top)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(w, h-top))
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
	selectedFn func() FeedID,
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
		selected := selectedFn()
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
			drawFeedEntryRow(rowGtx, s, e, e.ID == selected, rowClicks.For(e.ID),
				hovers.For(e.ID), popovers.For(e.ID), trashW)
			stk.Pop()
		}
		return layout.Dimensions{Size: size}
	}
}

// drawFeedEntryRow paints one feed row: the state pill (under everything),
// the label (left) and a hover-revealed trash icon + delete-confirm popover
// (right). hover holds the row's pointer hover state; the trash icon paints
// only while hovered (or while its confirm popover is open, so the popover
// never floats over an un-hovered row).
//
// ADR-021 R5 governs the pill's two inks and keeps them apart: the OPEN feed
// — the one whose articles the table is listing — is Primary-tinted, and
// hover is a neutral step walked from the sidebar's own ground (Surface is
// neutral 200, so the walk lands on 300). A reader must be able to see which
// row the pointer is over and which row the window is showing at the same
// time, which one ink cannot say.
func drawFeedEntryRow(
	gtx layout.Context,
	tok themeTokens,
	e feedEntry,
	selected bool,
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

	drawFeedEntryPill(gtx, tok, size, selected, hovered)

	// Everything the row draws lives between the pill's two edges: the label
	// padded in from the leading one, the trash gutter measured back from the
	// trailing one. Laying either out against the ROW's edges instead is what
	// puts a label flush on a fill and an icon half off it.
	pad := gtx.Dp(unit.Dp(feedsRowLabelPadDp))
	trail := gtx.Dp(unit.Dp(feedsPillTrailDp))

	// Label fills the pill minus the trash gutter; the label area is the
	// SelectFeed click target. Body text sits on the Neutral ramp's 900
	// step (ADR-007), in the theme's BodySmall role.
	labelW := size.X - trail - trashW - pad
	if labelW < 0 {
		labelW = 0
	}
	lbStk := op.Offset(image.Pt(pad, 0)).Push(gtx.Ops)
	labelGtx := gtx
	labelGtx.Constraints = layout.Exact(image.Pt(labelW, size.Y))
	drawFeedEntry(labelGtx, tok, e.Label, click)
	lbStk.Pop()

	// Trash gutter + confirm popover, against the pill's trailing edge.
	trX := size.X - trail - trashW
	if trX < 0 {
		trX = 0
	}
	trStk := op.Offset(image.Pt(trX, 0)).Push(gtx.Ops)
	trGtx := gtx
	trGtx.Constraints = layout.Exact(image.Pt(trashW, size.Y))
	dc.layout(trGtx, hovered)
	trStk.Pop()

	return layout.Dimensions{Size: size}
}

// drawFeedEntryPill fills the row's state pill, or nothing when the row is
// neither open nor hovered. Selection wins over hover: a hovered open feed
// keeps its tint, because the tint is the answer to "which one am I reading"
// and the hover is only "the pointer is here".
func drawFeedEntryPill(
	gtx layout.Context,
	tok themeTokens,
	size image.Point,
	selected, hovered bool,
) {
	var fill color.NRGBA
	switch {
	case selected:
		fill = tok.col.Ramps.Primary.Step(300)
	case hovered:
		fill = tok.col.Ramps.Neutral.Step(300)
	default:
		return
	}
	trail := gtx.Dp(unit.Dp(feedsPillTrailDp))
	vp := gtx.Dp(unit.Dp(feedsPillVPadDp))
	r := gtx.Dp(unit.Dp(feedsPillRadiusDp))
	right := size.X - trail
	if right <= 0 {
		return
	}
	pill := clip.RRect{
		Rect: image.Rect(0, vp, right, size.Y-vp),
		NE:   r, NW: r, SE: r, SW: r,
	}
	paint.FillShape(gtx.Ops, fill, pill.Op(gtx.Ops))
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
