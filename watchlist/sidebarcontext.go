// sidebarcontext.go owns one watchlist row's right-click context menu: a
// cadence/popover with "Rename" and "Delete" entries. Rename lands
// OpenRenameWatchlist (a small modal opens); Delete confirms inline (a second
// "Confirm delete" row) then writes the file (deleteWatchlistNamed), toasts,
// lands DeleteWatchlist, and closes. Open state is EPHEMERAL per-row
// interaction state, keyed by name and NOT model state (logged in
// FEEDBACK-G5.3.md): plain bools this file owns, written and read during
// layout on the frame goroutine, which cadence/popover reads back through
// Props.OpenNow — ADR-008 destination 2. Every row's menu shares the window's
// Arbiter, so right-clicking one row closes whichever row's menu was up.
//
// Until G0C.4 the open flag was a per-row rx.Subject with an atomic.Bool
// mirror beside it, and the inline confirm expansion was a second atomic
// beside that. Both are plain fields now; the atomic cell that remains
// carries the THEME's re-emissions, which really do arrive from another
// goroutine.
//
// Opening is driven by a SECONDARY (right) pointer press on the row, registered
// by the sidebar in front of the select clickable inside a pointer.PassOp (see
// sidebar.go). The popover anchor itself is invisible (the row already draws);
// the menu surface is positioned by cadence/popover relative to the anchor
// (canvas centre) — it cannot open at the cursor, a logged limitation.

package main

import (
	"image"
	"sync/atomic"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
)

const (
	ctxMenuWDp   = 160
	ctxMenuRowDp = 32
)

// sidebarContext is one watchlist row's context menu (anchor + popover).
// open and confirmShow are frame state: only layout writes them and only
// layout reads them.
type sidebarContext struct {
	name        string
	open        bool
	confirmShow bool // inline "confirm delete" expansion
	cell        atomic.Value
}

func newSidebarContext(
	th rx.Observable[theme.Theme],
	name string,
	storePath string,
	renameClick, deleteClick, confirmClick *widget.Clickable,
	loadModel func() Model,
	popArb *popover.Arbiter,
) *sidebarContext {
	sc := &sidebarContext{name: name}

	loadTok := mirrorTokens(th)

	// The anchor is invisible (zero-painted) but must report a small size so the
	// popover has an anchor rect to place the surface against.
	anchor := func(gtx layout.Context) layout.Dimensions {
		sz := image.Pt(gtx.Dp(unit.Dp(1)), gtx.Dp(unit.Dp(1)))
		return layout.Dimensions{Size: sz}
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if renameClick.Clicked(gtx) {
			mvu.MessageOp{Message: OpenRenameWatchlist{Name: sc.name}}.Add(gtx.Ops)
			sc.close()
		}
		if !sc.confirmShow {
			if deleteClick.Clicked(gtx) {
				sc.confirmShow = true
			}
		} else if confirmClick.Clicked(gtx) {
			m := loadModel()
			next := deleteWatchlistNamed(m.watchlists, sc.name)
			selected := m.selected
			if selected == sc.name {
				selected = firstWatchlistName(next)
			}
			if err := saveStore(storePath, documentOf(next, selected)); err == nil {
				toast.Notify(gtx, toast.Success, "Watchlist deleted")
			} else {
				toast.Notify(gtx, toast.Error, "Delete failed")
			}
			mvu.MessageOp{Message: DeleteWatchlist{Name: sc.name}}.Add(gtx.Ops)
			sc.close()
		}

		w := gtx.Dp(unit.Dp(ctxMenuWDp))
		rowH := gtx.Dp(unit.Dp(ctxMenuRowDp))
		y := 0
		// Rename entry.
		rStk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		rGtx := gtx
		rGtx.Constraints = layout.Exact(image.Pt(w, rowH))
		renameClick.Layout(rGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Rename watchlist").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			drawLabel(gtx, s.shaper, "Rename", s.typ.BodyMedium, s.col.Ramps.Neutral.Step(900))
			return layout.Dimensions{Size: image.Pt(w, rowH)}
		})
		rStk.Pop()
		y += rowH
		// Delete (or Confirm delete) entry.
		dStk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		dGtx := gtx
		dGtx.Constraints = layout.Exact(image.Pt(w, rowH))
		if !sc.confirmShow {
			deleteClick.Layout(dGtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp("Delete watchlist").Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				drawLabel(gtx, s.shaper, "Delete", s.typ.BodyMedium, s.col.Error)
				return layout.Dimensions{Size: image.Pt(w, rowH)}
			})
		} else {
			confirmClick.Layout(dGtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp("Confirm delete watchlist").Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				drawLabel(gtx, s.shaper, "Confirm delete", s.typ.LabelLarge, s.col.Error)
				return layout.Dimensions{Size: image.Pt(w, rowH)}
			})
		}
		dStk.Pop()
		y += rowH
		return layout.Dimensions{Size: image.Pt(w, y)}
	}

	popObs := popover.Popover(th, popover.Props{
		OpenNow:   func() bool { return sc.open },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Right,
		Arbiter:   popArb,
		OnDismiss: func(layout.Context) { sc.close() },
	})
	sc.cell.Store(layout.Widget(nil))
	_ = popObs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			sc.cell.Store(w)
		}
	})
	return sc
}

// openMenu and close run during layout — from the row's secondary-press
// handler, from the menu entries, and from the arbiter's OnDismiss — all on
// the frame goroutine, which is what lets these be plain fields. Each opening
// starts with the inline confirm collapsed, so a menu reopened after a
// half-finished delete never comes back already armed.
func (sc *sidebarContext) openMenu() {
	sc.confirmShow = false
	sc.open = true
}

func (sc *sidebarContext) close() {
	sc.confirmShow = false
	sc.open = false
}

// layout renders the context-menu popover widget (the invisible anchor always,
// the menu surface while open) inside the row's canvas.
func (sc *sidebarContext) layout(gtx layout.Context) layout.Dimensions {
	if w, ok := sc.cell.Load().(layout.Widget); ok && w != nil {
		w(gtx)
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
