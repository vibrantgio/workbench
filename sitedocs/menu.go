// menu.go declares the application menu bar. Every action this window
// offers through a control also stands in the bar.
package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// MenuItems returns the menu bar's declaration for desktop.NewMenuBar: View
// holds one item for each tab, in the strip's order, and each item posts
// the message its tab posts. The window binds no chord, so no item has a
// key equivalent. The outline's rows and disclosure marks act on one row
// each and have no item.
func MenuItems() []desktop.MenuItem {
	items := make([]desktop.MenuItem, len(tabPages))
	for i, page := range tabPages {
		items[i] = desktop.MenuItem{Menu: "View", Title: tabTitles[i], Msg: SetRoute{Page: page}}
	}
	return items
}
