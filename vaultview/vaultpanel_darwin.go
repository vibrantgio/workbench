//go:build darwin

package main

import "gioui.org/app"

// viewHandle reads the NSView out of a view event. An event that is not
// valid carries no live handle — the view left its window — and answers
// zero, which is what a panel presented then stands on its own for.
func viewHandle(e app.ViewEvent) uintptr {
	if v, ok := e.(app.AppKitViewEvent); ok && v.Valid() {
		return v.View
	}
	return 0
}
