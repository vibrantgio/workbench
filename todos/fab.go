package main

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/workbench/todos/internal/place"
)

func Fab(icon layout.Widget, ax, ay float32, dx, dy unit.Dp, focus bool, cb func(gtx layout.Context)) layout.Widget {
	clickable := &widget.Clickable{}
	return func(gtx layout.Context) layout.Dimensions {
		r := place.Place(image.Rectangle{Max: gtx.Constraints.Max}, image.Pt(gtx.Dp(dx), gtx.Dp(dy)), ax, ay)
		defer op.Offset(r.Min).Push(gtx.Ops).Pop()

		gtx.Constraints = layout.Exact(r.Size())
		clicked := clickable.Clicked(gtx)
		dims := clickable.Layout(gtx, icon)

		if clicked {
			cb(gtx)
		}
		if clickable.Hovered() {
			pointershape.OverSize(gtx.Ops, dims.Size, pointer.CursorPointer)
		}
		dims.Size = dims.Size.Add(r.Min)
		return dims
	}
}
