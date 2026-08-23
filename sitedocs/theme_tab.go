// theme_tab.go composes the Theme tab: the themer's palette section —
// the ramps grid and the named picks, drawn by theme_palette.go's copy of
// that presentation — following the live theme, in the same shell frame
// the Gallery tab uses: a ThreeColumn shell with a zero-width leading
// column, the navbar across the top, and the section as a scrolling
// column filling the rest.

package main

import (
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// themeShellLayer composes the Theme tab. The theme-driven stream that
// rebuilds the section rows rides in the shell's Sidebar slot (measuring
// zero) so a theme emission re-emits the shell and repaints on the same
// frame; the static Main slot reads the latest column through an atomic
// cell — the same layer-boundary hand-off the Docs and Gallery shells use.
func themeShellLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	st := list.NewState()

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })

	var mainCell atomic.Value
	sidebarDriven := rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) layout.Widget {
		col, typ := t.First, t.Second
		mainCell.Store(galleryColumn(st, col, themePaletteRows(typ.Shaper(), typ, col)))
		return zeroSidebar
	})

	mainSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := mainCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	return shell.Shell(th, shell.Props{
		Layout:  shell.ThreeColumn,
		Sidebar: sidebarDriven,
		Navbar:  navbarProps(mirrorTokens(th), pageTheme),
		Main:    mainSlot,
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
