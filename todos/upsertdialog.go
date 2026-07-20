package main

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/workbench/todos/internal/place"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/textdraw"
)

// UpsertDialog is the modal used for both adding (item.Id == -1) and editing
// a todo. It shows a single-line editor over a scrim; Escape cancels, Enter
// or the Add/Save button submits. Submission emits AddTodo or UpdateTodo —
// the reducer assigns new Ids — followed by SetRoute home.
func UpsertDialog(shaper *text.Shaper, th rx.Observable[theme.Theme], p Palette, item Todo) layout.Widget {
	edit := widget.Editor{SingleLine: true, Submit: true, InputHint: key.HintText}
	edit.SetText(item.Text)
	edit.SetCaret(len(edit.Text()), len(edit.Text()))
	focusRequested := false

	navigateHome := func(gtx layout.Context) { mvu.MessageOp{Message: SetRoute{}}.Add(gtx.Ops) }
	escape := OnEscapeKey(navigateHome)

	submit := func(gtx layout.Context, entered string) {
		if item.Id == -1 {
			mvu.MessageOp{Message: AddTodo{Text: entered}}.Add(gtx.Ops)
		} else {
			mvu.MessageOp{Message: UpdateTodo{Id: item.Id, Text: entered}}.Add(gtx.Ops)
		}
		mvu.MessageOp{Message: SetRoute{}}.Add(gtx.Ops)
	}

	// Prism buttons fill the width they are given and are at least 44 dp
	// tall, so each one is laid out inside a fixed-width box.
	sized := func(w unit.Dp, widget layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			gtx.Constraints.Max.X = gtx.Dp(w)
			return widget(gtx)
		}
	}

	// th is a static snapshot (rx.Of), so First() resolves synchronously.
	cancelBtn, _ := button.Button(th, button.Props{
		Label:   "Cancel",
		Message: SetRoute{},
	}).First()

	label := "Save"
	if item.Id == -1 {
		label = "Add"
	}
	var submitClicked bool
	submitWidget, _ := button.Button(th, button.Props{
		Label:   label,
		OnClick: func(_ layout.Context) { submitClicked = true },
	}).First()
	submitBtn := func(gtx layout.Context) layout.Dimensions {
		dims := submitWidget(gtx)
		if submitClicked {
			submitClicked = false
			submit(gtx, edit.Text())
		}
		return dims
	}

	return func(gtx layout.Context) layout.Dimensions {
		if !focusRequested {
			gtx.Execute(key.FocusCmd{Tag: &edit})
			focusRequested = true
		}
		max := gtx.Constraints.Max

		escape(gtx)

		// Dialog modal fullscreen scrim over the disabled page behind it.
		rect := image.Rectangle{Max: max}
		Pane(gtx, rect, 0, p.Cover)

		// Allow the dialog to be smaller than max.
		gtx.Constraints.Min = image.Point{}

		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: p.Text}.Add(gtx.Ops)
		textMaterial := m.Stop()

		m = op.Record(gtx.Ops)
		paint.ColorOp{Color: p.Select}.Add(gtx.Ops)
		selectMaterial := m.Stop()

		return layout.UniformInset(Padding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// Dialog pane surface, centred.
			size := image.Pt(gtx.Dp(ModalWidth), gtx.Dp(ModalHeight))
			max := gtx.Constraints.Constrain(size)
			rect = place.Place(image.Rectangle{Max: gtx.Constraints.Max}, max, 0.5, 0.5)
			Pane(gtx, rect, gtx.Dp(BorderRadius), p.Pane)
			defer op.Offset(rect.Min).Push(gtx.Ops).Pop()
			gtx.Constraints.Max = rect.Size()
			return layout.UniformInset(Padding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				max := gtx.Constraints.Max

				b := gtx.Dp(BorderWidth)
				pad := gtx.Dp(Padding) / 2
				t := textdraw.MeasureText(gtx, shaper, H5, "W").Y
				r := gtx.Dp(BorderRadius)

				// Bordered text-entry field: accent border, field fill.
				rect := image.Rect(0, 0, max.X, t+2*(pad+b))
				Pane(gtx, rect, r, p.Icon)
				rect = rect.Inset(b)
				Pane(gtx, rect, r, p.Edit)
				rect = rect.Inset(pad)

				for {
					ev, ok := edit.Update(gtx)
					if !ok {
						break
					}
					if se, ok := ev.(widget.SubmitEvent); ok {
						submit(gtx, se.Text)
					}
				}

				if edit.Text() == "" {
					textdraw.FillText(gtx, shaper, H6, rect, 0.0, 0.5, p.Select, "What needs to be done?")
				}
				func(gtx layout.Context) {
					defer op.Offset(rect.Min).Push(gtx.Ops).Pop()
					gtx.Constraints = layout.Exact(rect.Size())
					edit.Layout(gtx, shaper, H5.Font, H5.Size, textMaterial, selectMaterial)
				}(gtx)

				// Cancel / submit row along the dialog's bottom edge, tall
				// enough for the 44 dp button height.
				rect = image.Rect(0, max.Y-gtx.Dp(48), max.X, max.Y)
				cs := op.Offset(rect.Min).Push(gtx.Ops)
				gtx.Constraints = layout.Exact(rect.Size())
				layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(sized(100, cancelBtn)),
					layout.Rigid(layout.Spacer{Width: Padding}.Layout),
					layout.Rigid(sized(100, submitBtn)))
				cs.Pop()

				return layout.Dimensions{Size: max}
			})
		})
	}
}
