// preferences.go composes the Preferences PANEL — the reference
// implementation of the dialog grammar's panel half, and the other end of the
// accelerator shortcut.go binds.
//
// It is a panel because of what its contents ARE, not because of a flag:
// rows-per-page and unread-only apply the instant they change, the table
// repaginates underneath the open panel, and there is consequently nothing to
// confirm and nothing to cancel. Leaving costs nothing, so every cheap exit is
// offered — and all three of them (the ghost X, Escape, a backdrop click) come
// from cadence/modal for free, because Props.Decision is nil. That single
// absence is the whole declaration: no HideClose, no dismiss-on-scrim boolean,
// no Return binding. The affordances travel with the intent.
//
// Contrast the Add-feed modal next door in app.go, which asks a question and
// answers it with a submit.
//
// The body also puts prism/button's emphasis axis to work as a state display:
// the selected page size is TONAL and the rest are GHOST. Nothing here is
// Filled, and that is the point — a panel of preferences has no one loud
// action, no thing the screen is about. Filled would be a lie about what the
// surface is for.
package main

import (
	"image"
	"strconv"
	"sync/atomic"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/theme/theme"
)

// Geometry of the panel body's two preference rows. The row height is the
// 44 dp pointer floor prism/button guarantees in every emphasis register, so
// a ghost control's hit area never reaches into the row above or below it.
const (
	prefsRowHDp     = 44
	prefsRowGapDp   = 12
	prefsBtnGapDp   = 8
	prefsSizeBtnW   = 56
	prefsToggleBtnW = 72
)

// preferencesPanel builds the Preferences panel stream. Open state and both
// preferences are model-derived, so the panel is pure view over them and the
// reducer owns every transition — including the one the accelerator lands.
func preferencesPanel(
	th rx.Observable[theme.Theme],
	prefsOpenObs rx.Observable[bool],
	rowsPerPageObs rx.Observable[int],
	unreadOnlyObs rx.Observable[bool],
	modalArb *modal.Arbiter,
) rx.Observable[layout.Widget] {
	loadTok := mirrorTokens(th)

	// Clickables are owned here and outlive the rebuilds below: the buttons
	// are re-BUILT whenever a preference changes (their emphasis is derived
	// from it), but their focus tags and press state must not be, or Tab
	// order and hover would reset on every click.
	sizeClicks := make([]widget.Clickable, len(rowsPerPageChoices))
	var unreadClick widget.Clickable

	// One SwitchMap over both preferences rebuilds the row of buttons with
	// the emphasis the new state implies. This is the same rebuild-on-model
	// -change shape articles.go uses for the pagination row.
	buttonsObs := rx.SwitchMap(
		rx.CombineLatest2(rowsPerPageObs, unreadOnlyObs),
		func(t rx.Tuple2[int, bool]) rx.Observable[[]layout.Widget] {
			rows, unread := t.First, t.Second
			emphasis := func(on bool) button.Emphasis {
				if on {
					return button.Tonal
				}
				return button.Ghost
			}
			built := make([]rx.Observable[layout.Widget], 0, len(rowsPerPageChoices)+1)
			for i, n := range rowsPerPageChoices {
				n := n
				built = append(built, button.Button(th, button.Props{
					Label:     strconv.Itoa(n),
					Emphasis:  emphasis(n == rows),
					Clickable: &sizeClicks[i],
					OnClick: func(gtx layout.Context) {
						mvu.MessageOp{Message: SetRowsPerPage{Rows: n}}.Add(gtx.Ops)
					},
				}))
			}
			label := "Off"
			if unread {
				label = "On"
			}
			built = append(built, button.Button(th, button.Props{
				Label:     label,
				Emphasis:  emphasis(unread),
				Clickable: &unreadClick,
				OnClick: func(gtx layout.Context) {
					mvu.MessageOp{Message: ToggleUnreadOnly{}}.Add(gtx.Ops)
				},
			}))
			return rx.CombineLatest(built...)
		},
	)

	// Layer-boundary cell: modal.Props.Body is a static slot, the buttons are
	// observables. Same hand-off as addFeedModal's cells.
	var buttonCell atomic.Value
	body := func(gtx layout.Context) layout.Dimensions {
		tok := loadTok()
		btns, _ := buttonCell.Load().([]layout.Widget)
		w := gtx.Constraints.Max.X
		rowH := gtx.Dp(unit.Dp(prefsRowHDp))
		gap := gtx.Dp(unit.Dp(prefsRowGapDp))

		y := prefsRow(gtx, tok, 0, w, rowH, "Rows per page",
			take(btns, 0, len(rowsPerPageChoices)), gtx.Dp(unit.Dp(prefsSizeBtnW)))
		y += gap
		y = prefsRow(gtx, tok, y, w, rowH, "Unread only",
			take(btns, len(rowsPerPageChoices), len(rowsPerPageChoices)+1), gtx.Dp(unit.Dp(prefsToggleBtnW)))
		return layout.Dimensions{Size: image.Pt(w, y)}
	}

	modalObs := modal.Modal(th, modal.Props{
		Open:  prefsOpenObs,
		Title: "Preferences",
		Body:  body,
		// This window's modal stack: the panel and the Add-feed modal share
		// it, so whichever is opened last is the one that takes input.
		Arbiter: modalArb,
		// Props.Decision stays nil. That is the panel intent, and with it come
		// the ghost close X, the dismissing backdrop, and Escape — none of
		// which is configured here because none of them is this app's choice
		// to make once it has said what kind of dialog this is.
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: ClosePreferences{}}.Add(gtx.Ops)
		},
		// The preference controls live in the Body, not the (absent) footer,
		// so they join the Tab cycle through DynamicFocusTags. The close X
		// still leads it and takes initial focus on open.
		DynamicFocusTags: func() []event.Tag {
			tags := make([]event.Tag, 0, len(sizeClicks)+1)
			for i := range sizeClicks {
				tags = append(tags, &sizeClicks[i])
			}
			return append(tags, &unreadClick)
		},
	})

	return rx.Map(rx.CombineLatest2(modalObs, buttonsObs),
		func(n rx.Tuple2[layout.Widget, []layout.Widget]) layout.Widget {
			buttonCell.Store(n.Second)
			return n.First
		},
	)
}

// prefsRow draws one preference row at vertical offset y — its caption on the
// left, vertically centred, and its controls right-aligned in equal boxes —
// and returns the offset just past it.
func prefsRow(
	gtx layout.Context,
	tok themeTokens,
	y, w, rowH int,
	caption string,
	btns []layout.Widget,
	btnW int,
) int {
	labelGtx := gtx
	labelGtx.Constraints.Min = image.Point{}
	labelGtx.Constraints.Max = image.Pt(w, rowH)
	rec := op.Record(gtx.Ops)
	dims := drawLabel(labelGtx, tok.shaper, caption, tok.typ.BodyMedium, tok.col.Text)
	call := rec.Stop()
	off := op.Offset(image.Pt(0, y+(rowH-dims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()

	btnGap := gtx.Dp(unit.Dp(prefsBtnGapDp))
	x := w - (len(btns)*btnW + (len(btns)-1)*btnGap)
	for _, b := range btns {
		if b != nil {
			st := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			bg := gtx
			bg.Constraints = layout.Exact(image.Pt(btnW, rowH))
			b(bg)
			st.Pop()
		}
		x += btnW + btnGap
	}
	return y + rowH
}

// take returns ws[lo:hi] clipped to what ws actually holds — the button cell
// is empty on the first frame, before the button streams have emitted.
func take(ws []layout.Widget, lo, hi int) []layout.Widget {
	if lo >= len(ws) {
		return make([]layout.Widget, hi-lo)
	}
	if hi > len(ws) {
		hi = len(ws)
	}
	return ws[lo:hi]
}
