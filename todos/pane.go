package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func Pane(gtx layout.Context, r image.Rectangle, radius int, c color.NRGBA) {
	if radius == 0 {
		paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
	} else {
		paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, radius).Op(gtx.Ops))
	}
}
