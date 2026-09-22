//go:build !darwin

package main

import "gioui.org/app"

// viewHandle answers zero away from macOS: no platform here presents a
// panel of this application's own (openpanel.Available says so), so nothing
// asks for a view to attach one to.
func viewHandle(app.ViewEvent) uintptr { return 0 }
