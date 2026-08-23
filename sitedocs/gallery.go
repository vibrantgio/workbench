// gallery.go composes the Gallery tab: the design system's published
// surface — foundations, components, patterns and the reading sample — as
// one scrolling column of live controls. The catalogue is
// components/gallery/inventory, the same one the themer embeds: the rows
// are the real widgets (buttons press, fields take type, toggles flip),
// never pictures of them, and this file adds no second inventory — it asks
// the published one for its rows and puts a viewport in front of them.
//
// The tab is a ThreeColumn shell with a zero-width leading column: the
// navbar spans the top and the column fills everything under it. The
// inventory is built once, on the first theme emission, and outlives every
// palette after it — a theme change is a new set of row values over the
// same parsed documents and scroll positions, exactly the economy the
// inventory is designed around. The theme-driven stream that rebuilds the
// rows rides in the shell's Sidebar slot (measuring zero) so a theme
// emission re-emits the shell and repaints on the same frame; the static
// Main slot reads the latest column widget through an atomic cell, the
// same layer-boundary hand-off the Docs shell uses.

package main

import (
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// galleryShellLayer composes the Gallery tab. The shell is ThreeColumn so
// the navbar spans the full width; the leading column is the theme-driven
// zero-size widget described in the file comment, and Main is the
// inventory column.
func galleryShellLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	// Built on the first emission and kept: the inventory holds the parsed
	// reading sample and its sections' scroll state, st the column's own
	// scroll position. A palette change rebuilds row values, never these.
	var inv *inventory.Inventory
	st := list.NewState()

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })

	// mainCell bridges the layer boundary: the sidebar-slot stream below
	// stores the freshly themed column synchronously, and the static Main
	// slot reads it at frame time.
	var mainCell atomic.Value
	sidebarDriven := rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) layout.Widget {
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
		mainCell.Store(galleryColumn(st, col, inv.Items(col)))
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
		Navbar:  navbarProps(mirrorTokens(th), pageGallery),
		Main:    mainSlot,
	})
}

// zeroSidebar measures nothing, so the shell's Main slot takes the whole
// row width. It exists because the theme-driven rebuild stream has to live
// in some shell input for emissions to repaint, and the sidebar slot is
// the one that accepts a stream.
func zeroSidebar(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{}
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
