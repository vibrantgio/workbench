// The state the moved tabs keep across emissions, and the type scale the
// Typography tab shows.
//
// The colour set and the type scale are on two tabs because a tab's title
// names one kind of thing, and a colour set and a type scale are two kinds.
// The inventory's own Foundations sections are on no tab of their own: the
// board on the Colours tab is the one telling of the colour set, and the
// type scale is the other Foundations section.

package main

import (
	"gioui.org/layout"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/list"
)

// themePages is what the Colours, Typography, Components, Patterns and
// Markdown tabs keep across emissions: each column's scroll position, the
// inventories, and the Typography tab's own handlers for the two code faces.
// A handler made again on every emission loses the press it is in the middle
// of, so these are at subscription scope.
type themePages struct {
	colours    *list.State
	typography *list.State
	scale      *inventory.Inventory
	faces      *faceChooser
	groups     [tabCount]*groupPage
}

func newThemePages() *themePages {
	tp := &themePages{
		colours:    list.NewState(),
		typography: list.NewState(),
		faces:      newFaceChooser(),
	}
	for tab := range tabGroups {
		tp.groups[tab] = &groupPage{st: list.NewState()}
	}
	return tp
}

// TypeScaleRows is the inventory's type scale in the judged theme: a heading
// band and a body, framed in the colours and type roles the board on the
// Colours tab uses.
func TypeScaleRows(j judged, tp *themePages) []layout.Widget {
	inv := j.prepare(&tp.scale)
	return palette.TypeScaleRows(inv, colourChrome(j.col), j.col, boardType(j.shaper, j.typ))
}
