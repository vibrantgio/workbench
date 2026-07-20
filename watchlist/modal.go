// modal.go composes the add/edit-symbol modal: a cadence/modal whose Body is a
// cadence/card wrapping (optionally) a cadence/alert, four prism/input
// textfields (Symbol, Exchange, Timeframe, Notes), and a prism/button submit.
// It is structurally the feeds addFeedModal scaled to four fields, plus the
// machinery G5.3b needs that feeds did not: pre-population of an uncontrolled
// TextField, and a real atomic disk write on save.
//
// Pre-population (the task's hardest part — fully logged in FEEDBACK-G5.3.md):
// prism/input.TextField is UNCONTROLLED — its widget.Editor lives in the
// component's rx.Defer scope and there is NO initial-value prop, so the row's
// current values cannot be injected into the live editor. The workaround:
//   - The model carries an incrementing modalEpoch (bumped on every open) and
//     the edited row in editSeed. editObs streams {epoch, seed}.
//   - Each field is rebuilt via SwitchMap keyed on the epoch: a fresh epoch
//     re-subscribes the TextField, giving a FRESH (empty) editor every open —
//     without this the editor persists across opens (open row A, type, close,
//     open row B → the field still shows A's text). Keyed on epoch, not
//     editIndex, so reopening the SAME row after a cancel still rebuilds.
//   - The fresh editor is EMPTY, so the row's current value is shown as the
//     field PLACEHOLDER and the field's text cell is seeded to that value too.
//   - On submit, an EMPTY field keeps the seeded value (cell unchanged); typing
//     replaces it. So "edit one field, the others survive" works.
//
// The honest limitation (logged): "empty keeps the seed" makes it impossible to
// CLEAR a previously-set optional field (e.g. Notes "foo" → ""). And the
// placeholder hides on focus, so the original is not visible while typing.
//
// Save side-effect placement (logged): the disk write lives in the submit
// CALLBACK by design, not in a reducer-returned mvu.Command (mvu.Loop in
// run() does execute those; saves stay in the callback so the write is
// synchronous with the confirming click). The callback reads a
// model mirror (fed by modelObs), applies the SAME pure applyEdit the reducer
// uses to build the full Document, writes it atomically, and fires the toast.
// The reducer stays pure; the callback and reducer never diverge because both
// route through applyEdit.

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

// editTarget is the per-open modal seed: epoch drives the field rebuild, seed
// carries the row's current values (zero Symbol in add mode).
type editTarget struct {
	epoch int
	seed  Symbol
}

// Body-row geometry. Each field is one row; the alert (empty-Symbol submit)
// gets its own band above the fields.
const (
	symFieldHDp = 48
	symBtnHDp   = 48
	symAlertHDp = 56
	symGapDp    = 12
)

// addSymbolModal builds the add/edit-symbol modal stream. modelMirrorObs feeds
// the submit callback's model snapshot (for the disk write); modalOpenObs and
// modalErrorObs drive the modal Open and the alert band; editObs drives the
// per-field epoch rebuild + placeholder seeding.
func addSymbolModal(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	storePath string,
	modelMirrorObs rx.Observable[Model],
	modalOpenObs rx.Observable[bool],
	modalErrorObs rx.Observable[bool],
	editObs rx.Observable[editTarget],
) rx.Observable[layout.Widget] {
	// Model mirror for the submit callback (the disk write needs the full
	// current watchlists/selection/editIndex, which the four form cells do not
	// carry). Fed by modelMirrorObs.
	var modelCell atomic.Value
	modelCell.Store(Model{editIndex: -1})
	_ = modelMirrorObs.Subscribe(rx.GoroutineContext(), func(m Model, _ error, done bool) {
		if !done {
			modelCell.Store(m)
		}
	})

	// Per-field text cells (latest editor text; the textfield is uncontrolled).
	// Seeded from the row on each open so an untouched field keeps its value.
	var symCell, exchCell, tfCell, notesCell atomic.Value
	symCell.Store("")
	exchCell.Store("")
	tfCell.Store("")
	notesCell.Store("")

	// field builds one epoch-rebuilt textfield: a fresh (empty) TextField per
	// open, with the seed value as Placeholder, and the cell re-seeded so an
	// untouched field submits the seed.
	field := func(label string, get func(Symbol) string, cell *atomic.Value) rx.Observable[layout.Widget] {
		return rx.SwitchMap(editObs, func(e editTarget) rx.Observable[layout.Widget] {
			seedVal := get(e.seed)
			cell.Store(seedVal)
			placeholder := seedVal
			if placeholder == "" {
				placeholder = label
			}
			return input.TextField(th, input.TextFieldProps{
				Placeholder: placeholder,
				Description: label,
				Shaper:      shaper,
				OnChange: func(_ layout.Context, txt string) {
					cell.Store(txt)
				},
			})
		})
	}

	symField := field("Symbol", func(s Symbol) string { return s.Symbol }, &symCell)
	exchField := field("Exchange", func(s Symbol) string { return s.Exchange }, &exchCell)
	tfField := field("Timeframe", func(s Symbol) string { return s.Timeframe }, &tfCell)
	notesField := field("Notes", func(s Symbol) string { return s.Notes }, &notesCell)

	var submitClick widget.Clickable
	submit := button.Button(th, button.Props{
		Label:     "Save",
		Clickable: &submitClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			sym := strings.TrimSpace(loadStr(&symCell))
			exch := strings.TrimSpace(loadStr(&exchCell))
			tf := strings.TrimSpace(loadStr(&tfCell))
			notes := strings.TrimSpace(loadStr(&notesCell))
			if sym != "" {
				// Side-effect placement (see file header): apply the SAME pure
				// helper the reducer uses to the mirrored model, write the full
				// Document atomically, and confirm with a toast. Empty Symbol
				// fires no write/toast — the reducer raises the alert instead.
				m, _ := modelCell.Load().(Model)
				next := applyEdit(m.watchlists, m.selected, m.editIndex, Symbol{
					Symbol: sym, Exchange: exch, Timeframe: tf, Notes: notes,
				})
				if err := saveStore(storePath, documentOf(next, m.selected)); err == nil {
					toast.Notify(toast.Success, "Saved")
				} else {
					toast.Notify(toast.Error, "Save failed")
				}
			}
			mvu.MessageOp{Message: SubmitSymbol{
				Symbol: sym, Exchange: exch, Timeframe: tf, Notes: notes,
			}}.Add(gtx.Ops)
		},
	})

	alertObs := alert.Alert(th, alert.Props{
		Variant: alert.Error,
		Title:   "Symbol is required",
		Shaper:  shaper,
	})

	// Layer-boundary cells for the static modal/card slots.
	var symFC, exchFC, tfFC, notesFC, submitFC, alertFC atomic.Value
	cellSlot := func(c *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := c.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// errorCell mirrors modalError so the static card body decides whether to
	// draw the alert band at frame time.
	var errorCell atomic.Bool
	_ = modalErrorObs.Subscribe(rx.GoroutineContext(), func(v bool, _ error, done bool) {
		if !done {
			errorCell.Store(v)
		}
	})

	// The card body: optional alert band, the four fields, then Save.
	cardBody := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(unit.Dp(symGapDp))
		fieldH := gtx.Dp(unit.Dp(symFieldHDp))
		btnH := gtx.Dp(unit.Dp(symBtnHDp))
		alertH := gtx.Dp(unit.Dp(symAlertHDp))
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
		place(cellSlot(&symFC), fieldH)
		place(cellSlot(&exchFC), fieldH)
		place(cellSlot(&tfFC), fieldH)
		place(cellSlot(&notesFC), fieldH)
		place(cellSlot(&submitFC), btnH)
		y -= gap // last row added a trailing gap
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
		Open:   modalOpenObs,
		Title:  "Symbol",
		Body:   modalBody,
		Shaper: shaper,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseModal{}}.Add(gtx.Ops)
		},
	})

	// Fold every live component widget into the modal stream so the modal
	// re-emits when any re-emits; store the latest into its cell. CombineLatest8
	// is not available, so two CombineLatest5s are merged through one more Map.
	fieldsObs := rx.Map(
		rx.CombineLatest4(symField, exchField, tfField, notesField),
		func(n rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) [4]layout.Widget {
			return [4]layout.Widget{n.First, n.Second, n.Third, n.Fourth}
		},
	)
	return rx.Map(
		rx.CombineLatest5(modalObs, cardObs, fieldsObs, submit, alertObs),
		func(n rx.Tuple5[layout.Widget, layout.Widget, [4]layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			cardCell.Store(n.Second)
			symFC.Store(n.Third[0])
			exchFC.Store(n.Third[1])
			tfFC.Store(n.Third[2])
			notesFC.Store(n.Third[3])
			submitFC.Store(n.Fourth)
			alertFC.Store(n.Fifth)
			return n.First
		},
	)
}

func loadStr(c *atomic.Value) string {
	s, _ := c.Load().(string)
	return s
}
