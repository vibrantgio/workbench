package main

import (
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// BackdropLayer fills the window with the theme's Background pin; it is
// the ground under the wireframe field and re-emits whenever the OS
// colour scheme changes.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		return backdrop.Widget(c.Background)
	})
}

// FieldLayer is the launcher triangle field drawn as a single-colour
// wireframe. The Field is subscription-scoped state, so it lives in an
// rx.Defer factory; each theme emission re-keys the one stroke colour
// in place — the field itself is built exactly once per subscription.
func FieldLayer(win *app.Window, th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		field := NewField(win, unit.Dp(windowW), unit.Dp(windowH))
		return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
			field.SetColors(c)
			return field.Widget()
		})
	})
}
