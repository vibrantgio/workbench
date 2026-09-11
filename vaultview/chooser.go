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

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
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
) rx.Observable[layout.Widget] {
	openObs := rx.Map(modelObs, func(m Model) bool { return m.ChooserOpen() }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	// Per-candidate clickables, pointer-stable across frames; the slice
	// grows to the widest candidate list seen.
	var rowClicks []*widget.Clickable

	body := func(gtx layout.Context) layout.Dimensions {
		m := loadModel()
		tok := loadTok()
		for len(rowClicks) < len(m.ChooserCandidates) {
			rowClicks = append(rowClicks, &widget.Clickable{})
		}
		ref := ParseRef(m.ChooserBody)
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lead := fmt.Sprintf("%q matches %d notes:", ref.File, len(m.ChooserCandidates))
				return drawText(gtx, tok.shaper, lead, tok.typ.BodyMedium,
					vgcolor.Flatten(tok.col.SecondaryLabel, tok.col.WindowBackground))
			}),
			layout.Rigid(complayout.VSpacer(chooserGapDp)),
		}
		for i, cand := range m.ChooserCandidates {
			cand := cand
			click := rowClicks[i]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if click.Clicked(gtx) {
					mvu.MessageOp{Message: ChooseCandidate{Path: cand}}.Add(gtx.Ops)
				}
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(chooserRowHDp)))
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := gtx.Constraints.Max
					// A pick list answers the pointer the way the platform's
					// menus do — the row under it wears the selection colour
					// — rather than with an overlay tint, which the platform
					// draws on a toolbar button and nowhere else.
					fill := tok.col.WindowBackground
					if click.Hovered() {
						fill = tok.col.SelectedContentBackground
						paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					}
					semantic.LabelOp(cand).Add(gtx.Ops)
					pointer.CursorPointer.Add(gtx.Ops)
					complayout.Inset(chooserRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := tok.col.Label
								if click.Hovered() {
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
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}

	return modal.Modal(th, modal.Props{
		Open:  openObs,
		Title: "Choose a note",
		Body:  body,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseChooser{}}.Add(gtx.Ops)
		},
	})
}
