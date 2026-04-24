package main

import (
	"golang.org/x/exp/shiny/materialdesign/colornames"

	"gioui.org/f32"
	"gioui.org/layout"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/gradient"
)

func Backdrop() rx.Observable[layout.Widget] {
	return rx.Of(gradient.LinearGradient(f32.Pt(0, 0), colornames.BlueGrey500, f32.Pt(1, 1), colornames.BlueGrey300))
}
