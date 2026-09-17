// theme_tabs.go composes the two tabs the theme is cut into: Colours — the
// shared colour board, every name the platform answers for with the value it
// carries in each appearance — and Typography — the inventory's type scale,
// every typography role with its name and size. Both follow the live theme,
// and both scroll in the same frame the group tabs use.
//
// They are two tabs rather than one because a strip cell names what its tab
// holds, and a colour set and a type scale are two different kinds of thing:
// a cell reading "Theme" named neither, and a reader after the scale had to
// know it was filed under the colours and scroll past the whole board to
// reach it.
//
// The inventory's own Foundations sections are on no tab of their own. This
// window has one telling of the colour set and it is the board the Colours
// tab draws; rendering both would put the same names in front of a reader
// twice, and the second telling, being a definition rather than this theme's
// own specimen, answers nothing the first did not. The scale is the other
// Foundations section, and the Typography tab is where it is told.

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

// colourTabLayer is the Colours tab's content stream: the freshly themed
// colour board on every token emission, over one long-lived scroll state.
// It asks the inventory for nothing — the board is the shared section and
// reads the set it is handed — so this tab builds none.
func colourTabLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	st := list.NewState()
	return rx.Map(themeTokenStream(th), func(t themeTokens) layout.Widget {
		return scrollingColumn(st, t.col, ColourRows(t.col, TypeFrom(t.shaper, t.typ)))
	})
}

// typographyTabLayer is the Typography tab's content stream: the freshly
// themed type scale on every token emission, over one long-lived scroll
// state and one long-lived inventory. The scale is the inventory's own
// section, drawn in the frame this window resolved for the board beside it,
// so the two tabs read as one theme told in two parts.
func typographyTabLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	st := list.NewState()
	var inv *inventory.Inventory
	return rx.Map(themeTokenStream(th), func(t themeTokens) layout.Widget {
		if inv == nil {
			inv = inventory.New(t.shaper)
		} else {
			inv.SetShaper(t.shaper)
		}
		inv.SetTypography(t.typ)
		return scrollingColumn(st, t.col, typeScaleRows(inv, t.shaper, t.typ, t.col))
	})
}

// themeTokenStream is the colour/typography snapshot both theme-cut tabs
// draw from: the two token streams combined, with the theme's own cached
// shaper resolved once per emission.
func themeTokenStream(th rx.Observable[theme.Theme]) rx.Observable[themeTokens] {
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	return rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.PlatformColors, tokens.Typography]) themeTokens {
		return themeTokens{col: t.First, typ: t.Second, shaper: t.Second.Shaper()}
	})
}

// typeScaleRows is the Typography tab's whole column: the inventory's type
// scale, framed in the colours and type roles this window resolved for the
// board.
func typeScaleRows(inv *inventory.Inventory, shaper *text.Shaper, typo tokens.Typography, c tokens.PlatformColors) []layout.Widget {
	return palette.TypeScaleRows(inv, colourChrome(c), c, TypeFrom(shaper, typo).story())
}

// renderColourTab is the static counterpart of the Colours tab's content
// used by goldens and review captures: a fresh top-scrolled column laid
// out once from pre-resolved tokens with no event processing.
func renderColourTab(shaper *text.Shaper, colors tokens.PlatformColors, typo tokens.Typography) layout.Widget {
	return scrollingColumn(list.NewState(), colors, ColourRows(colors, TypeFrom(shaper, typo)))
}

// renderTypographyTab is the static counterpart of the Typography tab's
// content, with the inventory pinned to one platform so the same bytes come
// out on any machine.
func renderTypographyTab(shaper *text.Shaper, colors tokens.PlatformColors, typo tokens.Typography) layout.Widget {
	inv := inventory.NewForOS(shaper, "darwin")
	inv.SetTypography(typo)
	return scrollingColumn(list.NewState(), colors, typeScaleRows(inv, shaper, typo, colors))
}
