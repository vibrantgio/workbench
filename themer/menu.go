// menu.go declares the application menu bar. Every action this window
// offers through a control also stands in the bar. The window binds no
// chord, so no item has a key equivalent.
package main

import (
	"github.com/vibrantgio/mvu/desktop"
)

// MenuItems returns the menu bar's declaration for desktop.NewMenuBar. Each
// item posts the message of its control: Keep the keep control's, Light and
// Dark the messages of the scheme control's two segments. The window opens
// a picture by a drop or from the command line and has no control for it,
// so the bar has no Open item.
func MenuItems() []desktop.MenuItem {
	return []desktop.MenuItem{
		{Menu: "File", Title: "Keep", Msg: KeepColor{}},
		{Menu: "View", Title: "Light", Msg: SetScheme{Dark: false}},
		{Menu: "View", Title: "Dark", Msg: SetScheme{Dark: true}},
	}
}
