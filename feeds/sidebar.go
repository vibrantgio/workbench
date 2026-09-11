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
	"github.com/vibrantgio/components/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/accordion"
	"github.com/vibrantgio/patterns/notifications"
	"github.com/vibrantgio/patterns/popover"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	feedsSidebarWidthDp = 192
	trashColWDp         = 24 // trailing trash-icon hit area, hover-revealed
)

// feedsRowLabelPadDp is the gap between the pill's leading edge and the
// label's first mark: a pill whose text starts on its own left edge is a bar
// with a rounded corner. The row's height, the pill's inset from the rail's
// edges and its corner are patterns/sidebar's — a list of feeds is a chrome
// rail, so its rows are the sidebar's rows and not the platform's list rows.
const feedsRowLabelPadDp = 8

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
// The feed tree lives in the Model, so add/delete mutate it; feedEntryListBody
// reads the live slice each frame and keys its per-entry view state by
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
	// The cell is written in the fold below, BEFORE the emitted layout.Widget
	// can be laid out, so no frame paints last selection's pill.
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
	// before the accordion's layout.Widget is returned, so a delete/add re-emits
	// this layer — driving theme/window's Invalidate() and the same-frame
	// repaint, the same way the open-section map drives it.
	colorsObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	// The density is here for the band alone: it is what the navbar's height
	// on the other side of the window's top edge is pinned to, and therefore
	// what the sidebar has to hold open on this side (see windowBandDp).
	densityObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Density] {
		return t.Density
	})
	return rx.Map(
		rx.CombineLatest5(accObs, feedsObs, colorsObs, selectedFeedObs, densityObs),
		func(n rx.Tuple5[layout.Widget, []feedGroup, tokens.PlatformColors, FeedID, tokens.Density]) layout.Widget {
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
// included — the band wears the fill of the region it caps, which here is
// the sidebar's own chrome fill — and only the accordion starts below it,
// clear of the buttons.
//
// Nothing is drawn in the band on this side. The window's name is already the
// navbar's brand on the other side of the seam, and a second copy of it under
// the buttons would be the same word twice across one strip.
func drawFeedsSidebar(
	gtx layout.Context,
	accW layout.Widget,
	colors tokens.PlatformColors,
	band unit.Dp,
) layout.Dimensions {
	w := gtx.Dp(unit.Dp(feedsSidebarWidthDp))
	h := gtx.Constraints.Max.Y
	size := image.Pt(w, h)
	// A sidebar is chrome, so it wears the platform's chrome material —
	// the fill a sidebar, a toolbar and a status strip share.
	paint.FillShape(gtx.Ops, colors.SidebarMaterial, clip.Rect{Max: size}.Op())
	top := min(max(gtx.Dp(band), 0), h)
	defer op.Offset(image.Pt(0, top)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(w, h-top))
	if accW != nil {
		accW(gtx)
	}
	return layout.Dimensions{Size: size}
}

// feedEntryListBody returns the body layout.Widget for one accordion section.
// entriesFn yields the section's CURRENT entries each frame (read from the
// per-section model cell). Entry clicks emit SelectFeed; hovering a row
// reveals a trash icon whose click toggles a per-row delete-confirm popover,
// whose confirm fires ConfirmDelete + a "Feed deleted" toast.
//
// All per-entry view state (the row clickable, the trash clickable, the
// confirm clickable, the per-row open flag) is keyed by FeedID so add/delete
// never re-binds state to the wrong row.
func feedEntryListBody(
	th rx.Observable[theme.Theme],
	entriesFn func() []feedEntry,
	selectedFn func() FeedID,
	popArb *popover.Arbiter,
) layout.Widget {
	loadTok := mirrorTokens(th)

	// Per-FeedID view state, stable across list mutation.
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
	// layout through Props.OpenNow. They all share this window's Arbiter, so
	// opening one row's confirm dismisses whichever row had it open.
	popovers := keyed.Defer(func(id FeedID) *deleteConfirm {
		return newDeleteConfirm(th, id, trashClicks.For(id), confirmClicks.For(id), popArb)
	})

	return func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		entries := entriesFn()
		selected := selectedFn()
		size := gtx.Constraints.Max
		rowH := gtx.Dp(patsidebar.RowHeight)
		trashW := gtx.Dp(unit.Dp(trashColWDp))

		for _, e := range entries {
			rc := rowClicks.For(e.ID)
			if rc.Clicked(gtx) {
				mvu.MessageOp{Message: SelectFeed{Feed: e.ID}}.Add(gtx.Ops)
			}
		}

		for i, e := range entries {
			stk := op.Offset(image.Pt(0, i*rowH)).Push(gtx.Ops)
			rowGtx := gtx
			rowGtx.Constraints = layout.Exact(image.Pt(size.X, rowH))
			drawFeedEntryRow(rowGtx, s, e, e.ID == selected, rowClicks.For(e.ID),
				hovers.For(e.ID), popovers.For(e.ID), trashW)
			stk.Pop()
		}
		return layout.Dimensions{Size: size}
	}
}

// drawFeedEntryRow paints one feed row: the selected feed's pill (under
// everything), the label (left) and a hover-revealed trash icon +
// delete-confirm popover (right). hover holds the row's pointer hover state;
// the trash icon paints only while hovered (or while its confirm popover is
// open, so the popover never floats over an un-hovered row).
//
// Only the open feed — the one whose articles the table is listing — takes a
// fill, and it is patterns/sidebar's pill. A sidebar row does not tint under
// the pointer on this platform, which the stored reference captures measure,
// so the hover state reveals the trash gutter and paints nothing.
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

	// Hover tracking spans the whole row but registers hover Enter/Leave only
	// (gesture.Hover), so it never claims the select press. Register the hover
	// area first so it sits under the label/trash content.
	hovered := hover.Update(gtx.Source) || dc.open
	hoverClip := clip.Rect{Max: size}.Push(gtx.Ops)
	hover.Add(gtx.Ops)
	hoverClip.Pop()

	if selected {
		patsidebar.PaintSelection(gtx, size, tok.col, false)
	}

	// Everything the row draws lives between the pill's two edges: the label
	// padded in from the leading one, the trash gutter measured back from the
	// trailing one. Laying either out against the ROW's edges instead puts a
	// label flush on a fill and an icon half off it.
	inset := gtx.Dp(patsidebar.SelectionInset)
	pad := inset + gtx.Dp(unit.Dp(feedsRowLabelPadDp))
	trail := inset

	// Label fills the pill minus the trash gutter; the label area is the
	// SelectFeed click target. It wears the foreground the platform pairs
	// with whatever the row is filled with, in the theme's BodySmall role.
	labelW := size.X - trail - trashW - pad
	if labelW < 0 {
		labelW = 0
	}
	lbStk := op.Offset(image.Pt(pad, 0)).Push(gtx.Ops)
	labelGtx := gtx
	labelGtx.Constraints = layout.Exact(image.Pt(labelW, size.Y))
	drawFeedEntry(labelGtx, tok, e.Label, selected, click)
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

// feedRowForeground is what a feed row's marks wear: the foreground the
// platform pairs with the selection pill on the open feed, and the ordinary
// label on the chrome material everywhere else. Both are flattened onto the
// fill they actually land on.
func feedRowForeground(c tokens.PlatformColors, selected bool) color.NRGBA {
	if selected {
		return vgcolor.Flatten(patsidebar.SelectionLabel(c, false), patsidebar.SelectionFill(c, false))
	}
	return vgcolor.Flatten(c.Label, c.SidebarMaterial)
}

func drawFeedEntry(
	gtx layout.Context,
	tok themeTokens,
	label string,
	selected bool,
	click *widget.Clickable,
) layout.Dimensions {
	size := gtx.Constraints.Max
	inner := func(gtx layout.Context) layout.Dimensions {
		labelGtx := gtx
		labelGtx.Constraints.Min = image.Point{}
		labelGtx.Constraints.Max = size
		mLabel := op.Record(gtx.Ops)
		labelDims := drawLabel(labelGtx, tok.shaper, label, tok.typ.BodySmall, feedRowForeground(tok.col, selected))
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
// through Props.OpenNow. Nothing outside the frame ever asks whether a row's
// confirm is open, so nothing outside the frame holds a copy of the answer.
// The trash click toggles it; the confirm click fires ConfirmDelete + a toast
// and closes; OnDismiss closes. The remaining atomic cell carries the THEME's
// re-emissions, which do arrive from another goroutine.
//
// The popover is wrapped in an Exact space (the trash gutter) so its anchor
// centres on the trash icon and the confirm surface sits below it — the same
// anchor-to-space coupling as the Share popover.
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
			drawTrashIcon(gtx, sz, feedRowForeground(s.col, false))
			return layout.Dimensions{Size: sz}
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if confirmClick.Clicked(gtx) {
			notifications.Notify(gtx, toast.Success, "Feed deleted")
			mvu.MessageOp{Message: ConfirmDelete{Feed: dc.id}}.Add(gtx.Ops)
			dc.close()
		}
		// Override the incoming half-space constraints: the popover sized
		// the anchor space to the tiny trash gutter, so half of it cannot
		// hold a confirm prompt. Size the content ourselves; popover pads it.
		w := gtx.Dp(unit.Dp(deleteConfirmWDp))
		promptH := gtx.Dp(unit.Dp(deleteConfirmRowHDp))
		btnH := gtx.Dp(unit.Dp(deleteConfirmRowHDp))
		drawLabel(gtx, s.shaper, "Delete this feed?", s.typ.BodyMedium,
			vgcolor.Flatten(s.col.Label, s.col.WindowBackground))
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm delete").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			drawLabel(gtx, s.shaper, "Delete", s.typ.LabelLarge, s.col.SystemRed)
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
// When hovered/open the popover renders the trash anchor (and, while open,
// the confirm surface) inside the gutter's Exact space.
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
