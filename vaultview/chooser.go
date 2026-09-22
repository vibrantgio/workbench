// chooser.go is the ambiguous-link chooser: a patterns/modal panel that
// opens when a wikilink's file part matches more than one note. The
// resolver already refused with the candidate list; this surface only
// asks which one was meant. Choosing navigates there (carrying the
// link's own heading path or block id); Escape, the close affordance and
// a scrim click all dismiss without navigating.

package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/modal"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
)

// Chooser layout constants.
const (
	chooserRowHDp     = 36
	chooserRowInsetDp = 8
	chooserGapDp      = 8
)

// chooserLayer builds the chooser modal stream. Open state and the
// candidate list live in the model; the modal is pure view over them.
// The body reads the model and token snapshots at frame time; repaints
// on model change are driven by the open observable's re-emission and
// the frames the routed layer requests.
func chooserLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	loadModel func() Model,
	loadTok func() themeTokens,
	arb *modal.Arbiter,
) rx.Observable[layout.Widget] {
	openObs := rx.Map(modelObs, func(m Model) bool { return m.ChooserOpen() }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	// Per-candidate clickables, pointer-stable across frames; the slice
	// grows to the widest candidate list seen.
	var rowClicks []*widget.Clickable

	// The candidates are a components/list, so the chooser's rows are the
	// dialog's focusable: the list takes the keyboard when the chooser
	// opens, wears the halo on its own box and opens with its first row
	// selected. shown is the refused body the list on screen was built for,
	// so a second ambiguous link opens on its own first row rather than on
	// the row the reader left behind.
	rows := list.NewState()
	shown := ""

	body := func(gtx layout.Context) layout.Dimensions {
		m := loadModel()
		tok := loadTok()
		for len(rowClicks) < len(m.ChooserCandidates) {
			rowClicks = append(rowClicks, &widget.Clickable{})
		}
		if shown != m.ChooserBody && len(m.ChooserCandidates) > 0 {
			shown = m.ChooserBody
			rows.Select(0)
			rows.Reveal(0)
		}
		choose := func(gtx layout.Context, i int) {
			if i >= 0 && i < len(m.ChooserCandidates) {
				mvu.MessageOp{Message: ChooseCandidate{Path: m.ChooserCandidates[i]}}.Add(gtx.Ops)
			}
		}
		// Return chooses the selected candidate. The list consumes the
		// arrows; activation is the caller's semantics, filtered on the
		// list's focus tag, as the folder browser filters it.
		for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
			for {
				e, ok := gtx.Event(key.Filter{Focus: rows.Focus(), Name: name})
				if !ok {
					break
				}
				if ke, ok := e.(key.Event); ok && ke.State == key.Press {
					choose(gtx, rows.Selected())
				}
			}
		}
		ref := ParseRef(m.ChooserBody)
		idx := make([]int, len(m.ChooserCandidates))
		for i := range idx {
			idx[i] = i
		}
		rowsPx := len(idx) * gtx.Dp(chooserRowHDp)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lead := fmt.Sprintf("%q matches %d notes:", ref.File, len(m.ChooserCandidates))
				return drawText(gtx, tok.shaper, lead, tok.typ.BodyMedium,
					vgcolor.Flatten(tok.col.SecondaryLabel, tok.col.WindowBackground))
			}),
			layout.Rigid(complayout.VSpacer(chooserGapDp)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowsPx))
				// Every fill a row paints under the band, not the
				// selection's alone: a row under the pointer wears the same
				// selection fill here, and the band's sides must land on
				// whichever of them the row they cross actually paints.
				rowFill := func(i int) color.NRGBA {
					if i == rows.Selected() || (i < len(rowClicks) && rowClicks[i].Hovered()) {
						return tok.col.SelectedContentBackground
					}
					return color.NRGBA{}
				}
				return list.Halo(gtx, rows, tok.col, tok.col.WindowBackground, rowFill, func(gtx layout.Context) layout.Dimensions {
					return list.LayoutSelectable(gtx, rows, idx,
						func(gtx layout.Context, i int, selected bool) layout.Dimensions {
							return chooserRow(gtx, tok, m.ChooserCandidates[i], rowClicks[i], rows, i, selected, choose)
						})
				})
			}),
		)
	}

	return modal.Modal(th, modal.Props{
		Open:    openObs,
		Title:   "Choose a note",
		Body:    body,
		Arbiter: arb,
		// The list is the body's first — and only — focusable, so the
		// chooser opens on its rows rather than on the header's close mark.
		DynamicFocusTags: func() []event.Tag { return []event.Tag{rows.Focus()} },
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseChooser{}}.Add(gtx.Ops)
		},
	})
}

// chooserRow draws one candidate row: the platform's selection fill under
// the row the keyboard stands on or the pointer is over, and the note's path
// on it.
func chooserRow(
	gtx layout.Context,
	tok themeTokens,
	cand string,
	click *widget.Clickable,
	rows *list.State,
	i int,
	selected bool,
	choose func(gtx layout.Context, i int),
) layout.Dimensions {
	if click.Clicked(gtx) {
		rows.Select(i)
		gtx.Execute(key.FocusCmd{Tag: rows.Focus()})
		choose(gtx, i)
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(chooserRowHDp)))
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		// A pick list answers the pointer the way the platform's
		// menus do — the row under it wears the selection colour
		// — rather than with an overlay tint, which the platform
		// draws on a toolbar button and nowhere else. The row the
		// keyboard stands on wears the same fill: one selection,
		// whichever hand moved it.
		fill := tok.col.WindowBackground
		if selected || click.Hovered() {
			fill = tok.col.SelectedContentBackground
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
		}
		semantic.LabelOp(cand).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		complayout.Inset(chooserRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := tok.col.Label
					if selected || click.Hovered() {
						label = tok.col.AlternateSelectedControlText
					}
					return drawLabel(gtx, tok.shaper, cand, tok.typ.BodyMedium,
						vgcolor.Flatten(label, fill))
				}),
			)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		return layout.Dimensions{Size: size}
	})
}
