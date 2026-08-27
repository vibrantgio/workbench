package main

import (
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// BackdropLayer fills the window with the content ground — the Background pin
// the palette resolves for level 0. It is the bottom layer, it re-emits
// whenever the OS colour scheme changes, and it is the only fill the resting
// window has: the list above it rests on this, raising nothing.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		return backdrop.Widget(PaletteFrom(c).Backdrop)
	})
}
