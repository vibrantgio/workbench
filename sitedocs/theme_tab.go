// theme_tab.go composes the Theme tab: the whole theme in one column —
// the colour the palette grew from (theme_seed.go), then the shared
// palette story (the ramps grid and the named picks), then the
// inventory's type ladder, following the live theme, in the same
// scrolling frame the group tabs use.
//
// The seed leads because the two sections after it are both derivations
// and the tab showed them without ever showing their input: a reader
// could read every rule on the page and still not know which colour the
// theme was made from.
//
// The ladder rides with the story rather than standing on a tab of its
// own, and it is the story that draws it — see [palette.TypeLadderRows],
// which carries why the ladder belongs here and which band it wears.
//
// The inventory's other two Foundations sections — foundations-roles and
// foundations-ramps — are on no tab at all. This window has one palette
// story and it is the one above, with the derivation in it; rendering
// both would put the same token names in front of a reader twice, and the
// second telling, being a definition rather than this theme's own
// specimen, answers nothing the first did not.

package main

import (
	stdcolor "image/color"

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
// palette column with the type ladder under it on every token emission,
// over one long-lived scroll state and one long-lived inventory.
//
// seed is the colour this window believes its palette was grown from. It
// is carried rather than derived because a dark scheme's tokens do not
// hold it (theme_seed.go), and it is a candidate rather than a fact: the
// seed row checks it against the palette it is drawing.
func themeTabLayer(th rx.Observable[theme.Theme], seed stdcolor.NRGBA) rx.Observable[layout.Widget] {
	st := list.NewState()
	var inv *inventory.Inventory

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })

	return rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) layout.Widget {
		col, typ := t.First, t.Second
		shaper := typ.Shaper()
		if inv == nil {
			inv = inventory.New(shaper)
		} else {
			inv.SetShaper(shaper)
		}
		inv.SetTypography(typ)
		return scrollingColumn(st, col, themeTabRows(inv, shaper, typ, col, seed))
	})
}

// themeTabRows is the whole Theme column: the palette story, then the
// type ladder under it, in that order — the palette first because the
// ladder is drawn in the colours the palette just accounted for.
func themeTabRows(inv *inventory.Inventory, shaper *text.Shaper, typo tokens.Typography, c tokens.ColorTokens, seed stdcolor.NRGBA) []layout.Widget {
	rows := themePaletteRows(shaper, typo, c, seed)
	return append(rows, palette.TypeLadderRows(inv, PaletteFrom(c).story(), c, TypeFrom(shaper, typo).story())...)
}

// themePaletteRows resolves the palette section's inputs from the live
// tokens: the section's own furniture palette and type roles, the
// counterpart scheme the inverse pair's rules name, and which side of the
// pair is on screen — all read off the tokens themselves, so the section
// follows whatever theme the window is wearing.
//
// The seed row comes first: the ramps and the picks are both derivations,
// and a story tells its input before it tells what was made of it.
func themePaletteRows(shaper *text.Shaper, typo tokens.Typography, c tokens.ColorTokens, seed stdcolor.NRGBA) []layout.Widget {
	p, ty := PaletteFrom(c), TypeFrom(shaper, typo)
	return append(seedRows(p, c, ty, seed), PaletteRows(p, c, schemeCounterpart(c), ty, isDark(c))...)
}

// renderThemeTab is the static counterpart of the Theme tab's content
// used by goldens and review captures: a fresh top-scrolled column laid
// out once from pre-resolved tokens with no event processing. The seed
// is handed in beside the tokens for the reason the live layer takes one.
func renderThemeTab(shaper *text.Shaper, colors tokens.ColorTokens, typo tokens.Typography, seed stdcolor.NRGBA) layout.Widget {
	inv := inventory.NewForOS(shaper, "darwin")
	inv.SetTypography(typo)
	return scrollingColumn(list.NewState(), colors, themeTabRows(inv, shaper, typo, colors, seed))
}
