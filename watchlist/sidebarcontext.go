// sidebarcontext.go owns one watchlist row's right-click context menu: a
// cadence/popover with "Rename" and "Delete" entries. Rename lands
// OpenRenameWatchlist (a small modal opens); Delete confirms inline (a second
// "Confirm delete" row) then writes the file (deleteWatchlistNamed), toasts,
// lands DeleteWatchlist, and closes. Open state is EPHEMERAL per-row
// interaction state (a per-row rx.Subject), keyed by name — the feeds idiom
// (logged in FEEDBACK-G5.3.md), NOT model state.
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
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

const (
	ctxMenuWDp   = 160
	ctxMenuRowDp = 32
)

// sidebarContext is one watchlist row's context menu (anchor + popover).
type sidebarContext struct {
	name        string
	openCh      rx.Observer[bool]
	openVal     atomic.Bool
	confirmShow atomic.Bool // inline "confirm delete" expansion
	cell        atomic.Value
}

func newSidebarContext(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	name string,
	storePath string,
	renameClick, deleteClick, confirmClick *widget.Clickable,
	loadModel func() Model,
) *sidebarContext {
	sc := &sidebarContext{name: name}
	send, openObs := rx.Subject[bool](0, 1)
	send.Next(false)
	sc.openCh = send

	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	var tokenCell atomic.Value
	tokenCell.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	_ = rx.CombineLatest2(colObs, typObs).Subscribe(func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			tokenCell.Store(tokenState{col: t.First, typ: t.Second})
		}
	}, rx.Goroutine)

	// The anchor is invisible (zero-painted) but must report a small size so the
	// popover has an anchor rect to place the surface against.
	anchor := func(gtx layout.Context) layout.Dimensions {
		sz := image.Pt(gtx.Dp(unit.Dp(1)), gtx.Dp(unit.Dp(1)))
		return layout.Dimensions{Size: sz}
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := tokenCell.Load().(tokenState)
		if renameClick.Clicked(gtx) {
			mvu.MessageOp{Message: OpenRenameWatchlist{Name: sc.name}}.Add(gtx.Ops)
			sc.close()
		}
		if !sc.confirmShow.Load() {
			if deleteClick.Clicked(gtx) {
				sc.confirmShow.Store(true)
			}
		} else if confirmClick.Clicked(gtx) {
			m := loadModel()
			next := deleteWatchlistNamed(m.watchlists, sc.name)
			selected := m.selected
			if selected == sc.name {
				selected = firstWatchlistName(next)
			}
			if err := saveStore(storePath, documentOf(next, selected)); err == nil {
				toast.Notify(toast.Success, "Watchlist deleted")
			} else {
				toast.Notify(toast.Error, "Delete failed")
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
			drawLabel(gtx, shaper, "Rename", unit.Sp(s.typ.BodyMedium), s.col.OnSurface)
			return layout.Dimensions{Size: image.Pt(w, rowH)}
		})
		rStk.Pop()
		y += rowH
		// Delete (or Confirm delete) entry.
		dStk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		dGtx := gtx
		dGtx.Constraints = layout.Exact(image.Pt(w, rowH))
		if !sc.confirmShow.Load() {
			deleteClick.Layout(dGtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp("Delete watchlist").Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				drawLabel(gtx, shaper, "Delete", unit.Sp(s.typ.BodyMedium), s.col.Error)
				return layout.Dimensions{Size: image.Pt(w, rowH)}
			})
		} else {
			confirmClick.Layout(dGtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp("Confirm delete watchlist").Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				drawLabel(gtx, shaper, "Confirm delete", unit.Sp(s.typ.LabelLarge), s.col.Error)
				return layout.Dimensions{Size: image.Pt(w, rowH)}
			})
		}
		dStk.Pop()
		y += rowH
		return layout.Dimensions{Size: image.Pt(w, y)}
	}

	popObs := popover.Popover(th, popover.Props{
		Open:      openObs,
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Right,
		OnDismiss: func(layout.Context) { sc.close() },
	})
	sc.cell.Store(layout.Widget(nil))
	_ = popObs.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			sc.cell.Store(w)
		}
	}, rx.Goroutine)
	return sc
}

func (sc *sidebarContext) openMenu() {
	sc.confirmShow.Store(false)
	if !sc.openVal.Swap(true) {
		sc.openCh.Next(true)
	}
}

func (sc *sidebarContext) close() {
	sc.confirmShow.Store(false)
	if sc.openVal.Swap(false) {
		sc.openCh.Next(false)
	}
}

// layout renders the context-menu popover widget (the invisible anchor always,
// the menu surface while open) inside the row's canvas.
func (sc *sidebarContext) layout(gtx layout.Context) layout.Dimensions {
	if w, ok := sc.cell.Load().(layout.Widget); ok && w != nil {
		w(gtx)
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
