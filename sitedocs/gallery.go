// gallery.go composes the Gallery tab: the design system's published
// surface — foundations, components, patterns and the reading sample — as
// one scrolling column of live controls. The catalogue is
// components/gallery/inventory, the same one the themer embeds: the rows
// are the real widgets (buttons press, fields take type, toggles flip),
// never pictures of them, and this file adds no second inventory — it asks
// the published one for its rows and puts a viewport in front of them.
//
// The tab's content is the inventory column filling the panel under the
// tab strip. The inventory is built once, on the first theme emission,
// and outlives every palette after it — a theme change is a new set of
// row values over the same parsed documents and scroll positions, exactly
// the economy the inventory is designed around.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// galleryTabLayer is the Gallery tab's content stream: the freshly themed
// inventory column on every token emission, over the one long-lived
// inventory and scroll state.
func galleryTabLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	// Built on the first emission and kept: the inventory holds the parsed
	// reading sample and its sections' scroll state, st the column's own
	// scroll position. A palette change rebuilds row values, never these.
	var inv *inventory.Inventory
	st := list.NewState()

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })

	return rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) layout.Widget {
		col, typ := t.First, t.Second
		shaper := typ.Shaper()
		if inv == nil {
			inv = inventory.New(shaper)
		} else {
			// A typography change needs the matching collection; the parsed
			// documents stay.
			inv.SetShaper(shaper)
		}
		inv.SetTypography(typ)
		return galleryColumn(st, col, inv.Items(col))
	})
}

// galleryColumn is the scrolling inventory column: the rows in a virtual
// list — only what shows is laid out — with an overlay scrollbar drawn
// from the same tokens the rows are, floating over the rows rather than
// cutting a gutter out of families shown at their own widths.
func galleryColumn(st *list.State, c tokens.ColorTokens, items []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		gtx.Constraints = layout.Exact(size)
		list.LayoutScrollbar(gtx, st, scrollbar.FromTokens(c), list.Overlay, items,
			func(gtx layout.Context, w layout.Widget) layout.Dimensions {
				return w(gtx)
			})
		return layout.Dimensions{Size: size}
	}
}

// renderGalleryTab is the static counterpart of the Gallery tab's content
// used by goldens and review captures: a fresh top-scrolled inventory
// column laid out once from pre-resolved tokens, with the control marks
// pinned to one platform so the same bytes come out on any machine.
func renderGalleryTab(shaper *text.Shaper, colors tokens.ColorTokens, typo tokens.Typography) layout.Widget {
	inv := inventory.NewForOS(shaper, "darwin")
	inv.SetTypography(typo)
	return galleryColumn(list.NewState(), colors, inv.Items(colors))
}
