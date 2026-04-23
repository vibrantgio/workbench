package main

import (
	"golang.org/x/exp/shiny/materialdesign/colornames"

	"gioui.org/f32"
	"gioui.org/layout"

	"github.com/reactivego/gio"
	"github.com/reactivego/rx"
)

func Backdrop() rx.Observable[layout.Widget] {
	return rx.Of(gio.LinearGradient(f32.Pt(0, 0), colornames.BlueGrey500, f32.Pt(1, 1), colornames.BlueGrey300))
}
