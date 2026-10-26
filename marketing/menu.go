// menu.go declares the application menu bar. Every action this window
// offers through a control also stands in the bar.
package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// MenuItems returns the menu bar's declaration for desktop.NewMenuBar. The
// bar holds only the platform's application menu. The hero's "See plans"
// button scrolls the page, and the scroll position belongs to the page's
// own subscription, not to the model, so no message can carry it. The
// window binds no chord.
func MenuItems() []desktop.MenuItem {
	return nil
}
