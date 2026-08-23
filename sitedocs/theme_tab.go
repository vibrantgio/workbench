// theme_tab.go composes the Theme tab: the themer's palette section —
// the ramps grid and the named picks, drawn by theme_palette.go's copy of
// that presentation — following the live theme, as a scrolling column
// filling the panel under the tab strip, the same frame the Gallery tab
// uses.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// themeTabLayer is the Theme tab's content stream: the freshly themed
// palette column on every token emission, over one long-lived scroll
// state.
func themeTabLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	st := list.NewState()

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })

	return rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) layout.Widget {
		col, typ := t.First, t.Second
		return galleryColumn(st, col, themePaletteRows(typ.Shaper(), typ, col))
	})
}

// themePaletteRows resolves the palette section's inputs from the live
// tokens: the section's own furniture palette and type roles, the
// counterpart scheme the inverse pair's rules name, and which side of the
// pair is on screen — all read off the tokens themselves, so the section
// follows whatever theme the window is wearing.
func themePaletteRows(shaper *text.Shaper, typo tokens.Typography, c tokens.ColorTokens) []layout.Widget {
	return PaletteRows(PaletteFrom(c), c, schemeCounterpart(c), TypeFrom(shaper, typo), isDark(c))
}

// renderThemeTab is the static counterpart of the Theme tab's content
// used by goldens and review captures: a fresh top-scrolled section laid
// out once from pre-resolved tokens with no event processing.
func renderThemeTab(shaper *text.Shaper, colors tokens.ColorTokens, typo tokens.Typography) layout.Widget {
	return galleryColumn(list.NewState(), colors, themePaletteRows(shaper, typo, colors))
}
