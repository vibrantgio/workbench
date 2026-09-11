// theme_tab.go composes the Theme tab: the whole theme in one column — the
// shared colour board, every name the platform answers for with the value it
// carries in each appearance, and under it the inventory's type scale,
// following the live theme, in the same scrolling frame the group tabs use.
//
// The scale rides with the board rather than standing on a tab of its own,
// and it is the board that draws it — [palette.TypeScaleRows].
//
// The inventory's other Foundations sections are on no tab at all. This
// window has one telling of the colour set and it is the board above;
// rendering both would put the same names in front of a reader twice, and the
// second telling, being a definition rather than this theme's own specimen,
// answers nothing the first did not.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// themeTabLayer is the Theme tab's content stream: the freshly themed
// colour board with the type scale under it on every token emission, over
// one long-lived scroll state and one long-lived inventory.
func themeTabLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	st := list.NewState()
	var inv *inventory.Inventory

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })

	return rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.PlatformColors, tokens.Typography]) layout.Widget {
		col, typ := t.First, t.Second
		shaper := typ.Shaper()
		if inv == nil {
			inv = inventory.New(shaper)
		} else {
			inv.SetShaper(shaper)
		}
		inv.SetTypography(typ)
		return scrollingColumn(st, col, themeTabRows(inv, shaper, typ, col))
	})
}

// themeTabRows is the whole Theme column: the colour board, then the type
// scale under it, in that order — the board first because the scale is drawn
// in the colours the board just accounted for.
func themeTabRows(inv *inventory.Inventory, shaper *text.Shaper, typo tokens.Typography, c tokens.PlatformColors) []layout.Widget {
	ty := TypeFrom(shaper, typo)
	rows := PaletteRows(c, ty)
	return append(rows, palette.TypeScaleRows(inv, paletteChrome(c), c, ty.story())...)
}

// renderThemeTab is the static counterpart of the Theme tab's content
// used by goldens and review captures: a fresh top-scrolled column laid
// out once from pre-resolved tokens with no event processing.
func renderThemeTab(shaper *text.Shaper, colors tokens.PlatformColors, typo tokens.Typography) layout.Widget {
	inv := inventory.NewForOS(shaper, "darwin")
	inv.SetTypography(typo)
	return scrollingColumn(list.NewState(), colors, themeTabRows(inv, shaper, typo, colors))
}
