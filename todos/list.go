package main

import (
	"image"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	prismlist "github.com/vibrantgio/prism/list"
	"github.com/vibrantgio/prism/theme"
)

// List renders the todos inside a rounded pane using the prism virtual list.
func List(th rx.Observable[theme.Theme], p Palette, model Model) layout.Widget {
	listState := prismlist.NewState()
	rows := make([]layout.Widget, len(model.List))
	for i := range model.List {
		rows[i] = Row(th, p, model.List[i])
	}
	return func(gtx layout.Context) layout.Dimensions {
		Pane(gtx, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(BorderRadius), p.Pane)
		return prismlist.Layout(gtx, listState, rows, func(gtx layout.Context, row layout.Widget) layout.Dimensions {
			return layout.UniformInset(Padding).Layout(gtx, row)
		})
	}
}

// Row is one todo line: a prism checkbox toggling completion, a prism button
// whose label opens the edit dialog, and a delete icon. All three events
// route through mvu.MessageOp — the checkbox and the routing messages via
// the Message prop / explicit ops, so the reducers are the only state
// writers.
func Row(th rx.Observable[theme.Theme], p Palette, item Todo) layout.Widget {
	row := layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}

	// th is a static snapshot (rx.Of), so First() resolves synchronously.
	cb, _ := input.Checkbox(th, input.CheckboxProps{
		Description: "completed",
		Checked:     item.Completed,
		Message:     ToggleTodo{Id: item.Id},
	}).First()

	editClicked := false
	editWidget, _ := button.Button(th, button.Props{
		Label:   item.Text,
		OnClick: func(_ layout.Context) { editClicked = true },
	}).First()

	clearIcon, err := raster.Widget(icons.ContentClear, 40, 40, raster.WithColors(p.Icon))
	if err != nil {
		panic(err)
	}
	var deleteClick widget.Clickable

	return func(gtx layout.Context) layout.Dimensions {
		return row.Layout(gtx,
			layout.Rigid(cb),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				dims := editWidget(gtx)
				if editClicked {
					editClicked = false
					mvu.MessageOp{Message: SelectTodo{Id: item.Id}}.Add(gtx.Ops)
					mvu.MessageOp{Message: SetRoute{Route: "edit.todo"}}.Add(gtx.Ops)
				}
				return dims
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if deleteClick.Clicked(gtx) {
					mvu.MessageOp{Message: DeleteTodo{Id: item.Id}}.Add(gtx.Ops)
				}
				dims := deleteClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, clearIcon)
				})
				if deleteClick.Hovered() {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				return dims
			}),
		)
	}
}
