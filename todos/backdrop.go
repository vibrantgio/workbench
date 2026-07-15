package main

import (
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// BackdropLayer fills the window with the theme's background colour; it is
// the bottom layer and re-emits whenever the OS colour scheme changes.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		return backdrop.Widget(c.Background)
	})
}
