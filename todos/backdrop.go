package main

import (
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// BackdropLayer fills the window with the window's own plane. It is the
// bottom layer, re-emits whenever the OS appearance changes, and is the only
// fill the resting window has: the list above it stands on this and paints
// nothing of its own.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
		return backdrop.Widget(PaletteFrom(c).Backdrop)
	})
}
