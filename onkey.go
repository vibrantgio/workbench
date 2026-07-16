package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// OnShortcutKey returns a widget that invokes cb on every press of the given
// key with the platform shortcut modifier (Cmd on macOS, Ctrl elsewhere).
// Lay it out over the area the shortcut should be live in — typically the
// whole window. A focused widget that claims the same chord (e.g. a text
// editor's own Cmd-Z undo) receives it first; that is correct layering, not
// a conflict. (Pattern: todos/onescapekey.go.)
func OnShortcutKey(name key.Name, cb func(gtx layout.Context)) layout.Widget {
	tag := new(int)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, tag)
		for {
			e, ok := gtx.Event(key.Filter{Name: name, Required: key.ModShortcut})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				cb(gtx)
			}
		}
		return layout.Dimensions{Size: size}
	}
}
