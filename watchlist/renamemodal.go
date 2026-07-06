// renamemodal.go composes the rename-watchlist modal: a cadence/modal whose
// Body is a cadence/card wrapping (optionally) a cadence/alert and a single
// prism/input textfield (the new name) plus a prism/button submit. It is the
// symbol modal scaled down to one field — and it reuses the SAME uncontrolled-
// field pre-population workaround (epoch rebuild + placeholder seed + "empty
// keeps seed"; full rationale and its clear-to-empty limitation are in
// modal.go's header and FEEDBACK-G5.3.md). Rename is the same pre-population
// problem, so it inherits the same limitation.
//
// The disk write lives in the submit CALLBACK (the reducer is pure and run()'s
// Scan discards Commands): it reads a model mirror, validates (non-empty,
// non-duplicate), applies the SAME pure renameWatchlistTo the reducer uses,
// writes the full Document atomically (with the renamed selection), and toasts.

package main

import (
	"image"
	"strings"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/alert"
	"github.com/vibrantgio/cadence/card"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/theme"
)

// renameTarget is the per-open rename-modal seed: epoch drives the field
// rebuild, target is the watchlist being renamed, seed is its current name
// (shown as the field placeholder).
type renameTarget struct {
	epoch  int
	target string
	seed   string
}

const (
	renameFieldHDp = 48
	renameBtnHDp   = 48
	renameAlertHDp = 56
	renameGapDp    = 12
)

// renameWatchlistModal builds the rename-watchlist modal stream.
func renameWatchlistModal(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	storePath string,
	modelMirrorObs rx.Observable[Model],
	openObs rx.Observable[bool],
	errorObs rx.Observable[bool],
	editObs rx.Observable[renameTarget],
) rx.Observable[layout.Widget] {
	var modelCell atomic.Value
	modelCell.Store(Model{editIndex: -1})
	_ = modelMirrorObs.Subscribe(func(m Model, _ error, done bool) {
		if !done {
			modelCell.Store(m)
		}
	}, rx.Goroutine)

	// nameCell holds the latest field text (the textfield is uncontrolled),
	// re-seeded on each open so an untouched field submits the current name.
	var nameCell atomic.Value
	nameCell.Store("")
	// targetCell mirrors the watchlist being renamed (the callback needs it).
	var targetCell atomic.Value
	targetCell.Store("")

	nameField := rx.SwitchMap(editObs, func(e renameTarget) rx.Observable[layout.Widget] {
		nameCell.Store(e.seed)
		targetCell.Store(e.target)
		placeholder := e.seed
		if placeholder == "" {
			placeholder = "Watchlist name"
		}
		return input.TextField(th, input.TextFieldProps{
			Placeholder: placeholder,
			Description: "Watchlist name",
			Shaper:      shaper,
			OnChange:    func(_ layout.Context, txt string) { nameCell.Store(txt) },
		})
	})

	var submitClick widget.Clickable
	submit := button.Button(th, button.Props{
		Label:     "Rename",
		Clickable: &submitClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			name := strings.TrimSpace(loadStr(&nameCell))
			target, _ := targetCell.Load().(string)
			m, _ := modelCell.Load().(Model)
			// Validate exactly as the reducer does (non-empty, non-duplicate)
			// so the callback never writes a name the reducer rejects.
			if name != "" && !nameTaken(m.watchlists, name, target) {
				next := renameWatchlistTo(m.watchlists, target, name)
				selected := m.selected
				if selected == target {
					selected = name
				}
				if err := saveStore(storePath, documentOf(next, selected)); err == nil {
					toast.Notify(toast.Success, "Watchlist renamed")
				} else {
					toast.Notify(toast.Error, "Rename failed")
				}
			}
			mvu.MessageOp{Message: SubmitRenameWatchlist{Name: name}}.Add(gtx.Ops)
		},
	})

	alertObs := alert.Alert(th, alert.Props{
		Variant: alert.Error,
		Title:   "Name must be unique and non-empty",
		Shaper:  shaper,
	})

	var nameFC, submitFC, alertFC atomic.Value
	cellSlot := func(c *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := c.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	var errorCell atomic.Bool
	_ = errorObs.Subscribe(func(v bool, _ error, done bool) {
		if !done {
			errorCell.Store(v)
		}
	}, rx.Goroutine)

	cardBody := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(unit.Dp(renameGapDp))
		fieldH := gtx.Dp(unit.Dp(renameFieldHDp))
		btnH := gtx.Dp(unit.Dp(renameBtnHDp))
		alertH := gtx.Dp(unit.Dp(renameAlertHDp))
		y := 0
		place := func(slot layout.Widget, h int) {
			stk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			cg := gtx
			cg.Constraints = layout.Exact(image.Pt(w, h))
			slot(cg)
			stk.Pop()
			y += h + gap
		}
		if errorCell.Load() {
			place(cellSlot(&alertFC), alertH)
		}
		place(cellSlot(&nameFC), fieldH)
		place(cellSlot(&submitFC), btnH)
		y -= gap
		return layout.Dimensions{Size: image.Pt(w, y)}
	}

	cardObs := card.Card(th, card.Props{Body: cardBody})
	var cardCell atomic.Value
	modalBody := func(gtx layout.Context) layout.Dimensions {
		if w, ok := cardCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	modalObs := modal.Modal(th, modal.Props{
		Open:   openObs,
		Title:  "Rename watchlist",
		Body:   modalBody,
		Shaper: shaper,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseRenameWatchlist{}}.Add(gtx.Ops)
		},
	})

	return rx.Map(
		rx.CombineLatest5(modalObs, cardObs, nameField, submit, alertObs),
		func(n rx.Tuple5[layout.Widget, layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			cardCell.Store(n.Second)
			nameFC.Store(n.Third)
			submitFC.Store(n.Fourth)
			alertFC.Store(n.Fifth)
			return n.First
		},
	)
}
