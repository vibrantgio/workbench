// menu.go declares the application menu bar. Every action this window
// offers through a control also stands in the bar.
package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// MenuItems returns the menu bar's declaration for desktop.NewMenuBar. The
// bar holds only the platform's application menu. Each card has a Launch
// button of its own, and the window has no selected application, so the
// bar has no Launch item. The window binds no chord.
func MenuItems() []desktop.MenuItem {
	return nil
}
