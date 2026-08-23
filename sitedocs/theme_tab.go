// theme_tab.go composes the Theme tab: the whole theme in one column —
// the colour the palette grew from (theme_seed.go), then the themer's
// palette section (the ramps grid and the named picks, drawn by
// theme_palette.go's copy of that presentation), then the inventory's
// type ladder, following the live theme, in the same scrolling frame the
// group tabs use.
//
// The seed leads because the two sections after it are both derivations
// and the tab showed them without ever showing their input: a reader
// could read every rule on the page and still not know which colour the
// theme was made from.
//
// The ladder is here rather than on a tab of its own because a theme is a
// palette and a typeface: the type roles are generated from the same
// theme the ramps are, so the tab that answers "what is this theme" has
// to answer both halves of it. It is the inventory's own foundations-type
// section — its body, drawn under this tab's own heading band rather than
// the inventory's.
//
// Which band it wears is not cosmetic. A fresh-eyes reviewer reading this
// tab found it inconsistent with itself: two sections banded one way and
// a third banded another, in a column whose whole subject is that a theme
// is coherent. So the borrowed section is dressed the way its neighbours
// are — and the inventory's own words are kept, split at the em dash its
// titles are already written with, so this file states no copy of its own
// and a title reworded upstream arrives here reworded.
//
// The inventory's other two Foundations sections — foundations-roles and
// foundations-ramps — are on no tab at all. This window has one palette
// story and it is the one above, with the derivation in it; rendering
// both would put the same token names in front of a reader twice, and the
// second telling, being a definition rather than this theme's own
// specimen, answers nothing the first did not.

package main

import (
	"image"
	stdcolor "image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// typeSection is the inventory section this tab borrows: the whole type
// ladder, every role a surface reads in.
const typeSection = "foundations-type"

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
	return append(rows, typeLadderRows(inv, PaletteFrom(c), c, TypeFrom(shaper, typo))...)
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

// sectionTitleSep is the seam an inventory section's title is written
// with: what the section is, then how to read it. The palette's own bands
// are built from exactly that pair — a label at the leading edge and a
// caption at the trailing one — so a borrowed title splits into a band
// with nothing reworded.
const sectionTitleSep = " — "

// typeLadderRows is the inventory's type ladder as the two rows that
// follow the palette: this tab's own heading band over the section's own
// body. A section whose title carries no separator lands with the whole
// title as the label and no caption, which is what the band does with an
// empty hint anyway.
func typeLadderRows(inv *inventory.Inventory, p Palette, c tokens.ColorTokens, ty Type) []layout.Widget {
	for _, s := range inv.Foundations(c) {
		if s.Name != typeSection {
			continue
		}
		label, hint, _ := strings.Cut(s.Title, sectionTitleSep)
		return []layout.Widget{
			paletteHeading(p, c, ty, label, hint),
			paletteBody(c, ladderBody(s)),
		}
	}
	return nil
}

// ladderBody adapts an inventory section's body to the palette body's
// shape: the palette measures its content and reports the height, while a
// section body is laid out in a slot of the height the section states. So
// the slot is stated here — bounded, because the type ladder measures
// nothing of its own and an unbounded one would take the column with it —
// and handed back as the height the band wraps.
func ladderBody(s inventory.Section) func(gtx layout.Context, width int) int {
	return func(gtx layout.Context, width int) int {
		h := gtx.Dp(s.Height)
		gtx.Constraints = layout.Constraints{Max: image.Pt(width, h)}
		s.Body(gtx)
		return h
	}
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
