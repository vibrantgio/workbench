package main

import (
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// BackdropLayer fills the window with the platform's window plane; it is the
// bottom layer and re-emits whenever the OS colour scheme changes.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
		return backdrop.Widget(c.WindowBackground)
	})
}
