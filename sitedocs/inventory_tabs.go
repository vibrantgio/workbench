// inventory_tabs.go composes the three tabs cut from the published
// inventory's own groups — Components, Patterns, Markdown — as one
// scrolling column of live controls each. The catalogue is
// components/gallery/inventory, the same one the themer embeds: the rows
// are the real widgets (buttons press, fields take type, toggles flip),
// never pictures of them, and this file adds no second inventory — it asks
// the published one for its rows and puts a viewport in front of them.
//
// One group per tab, which is the cut the inventory publishes as TabItems:
// the group's sections without the banner that would only repeat the strip
// cell clicked to reach it, closing line included. The reasoning is stated
// where the cut lives; what this window decides is which groups get a tab.
//
// The Foundations group is on no tab: its colour sections are the Theme
// tab's telling, and its type ladder moved there too (theme_tab.go).
//
// Each tab's inventory is built once, on the first theme emission, and
// outlives every palette after it — a theme change is a new set of row
// values over the same parsed documents and scroll positions, exactly the
// economy the inventory is designed around. The tabs hold one Inventory
// each rather than sharing one, because each stream is subscribed
// separately and a shared instance would be mutated from more than one of
// them; what that costs is three parses of the reading sample at startup.

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

// The inventory's own group names, as Groups() spells them. They are the
// lookup key rather than an index, so a group reordered upstream still
// lands on the tab that names it — and TabItems returning nothing is what
// TestEveryTabNamesALiveGroup turns into a failure.
const (
	groupComponents  = "Components"
	groupPatterns    = "Patterns"
	groupMarkdown    = "Markdown"
	groupFoundations = "Foundations"
)

// tabGroups maps each inventory tab's route identifier to the group it
// shows. The Docs and Theme tabs are not in it: they are not cut from the
// inventory.
var tabGroups = map[string]string{
	pageComponents: groupComponents,
	pagePatterns:   groupPatterns,
	pageMarkdown:   groupMarkdown,
}

// groupTabLayer is one inventory tab's content stream: the freshly themed
// column of that group's sections on every token emission, over the one
// long-lived inventory and scroll state.
func groupTabLayer(th rx.Observable[theme.Theme], group string) rx.Observable[layout.Widget] {
	// Built on the first emission and kept: the inventory holds the parsed
	// reading sample and its sections' scroll state, st the column's own
	// scroll position. A palette change rebuilds row values, never these.
	// st is per tab, which is what gives each tab its own scroll position.
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
		return scrollingColumn(st, col, inv.TabItems(col, group))
	})
}

// scrollingColumn is the scrolling column every inventory-fed tab shows —
// the three group tabs and the Theme tab both: the rows in a virtual list
// — only what shows is laid out — with an overlay scrollbar drawn from the
// same tokens the rows are, floating over the rows rather than cutting a
// gutter out of families shown at their own widths.
func scrollingColumn(st *list.State, c tokens.ColorTokens, items []layout.Widget) layout.Widget {
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

// renderGroupTab is the static counterpart of an inventory tab's content
// used by goldens and review captures: a fresh top-scrolled column laid
// out once from pre-resolved tokens, with the control marks pinned to one
// platform so the same bytes come out on any machine.
func renderGroupTab(shaper *text.Shaper, group string, colors tokens.ColorTokens, typo tokens.Typography) layout.Widget {
	inv := inventory.NewForOS(shaper, "darwin")
	inv.SetTypography(typo)
	return scrollingColumn(list.NewState(), colors, inv.TabItems(colors, group))
}
