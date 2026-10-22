// shortcut.go is the key area one accelerator is laid out as. Which
// accelerators the window answers, and what each of them posts, is menu.go's
// table.
package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// shortcutArea returns a layout.Widget that invokes cb on every press of name
// the platform shortcut modifier held. Lay it out over the area the shortcut
// should be live in — for a global accelerator, the whole window, FIRST, so
// it sits at the bottom of the hit stack under the content.
//
// The area exists only to receive KEY events, so it is wrapped in
// pointer.PassOp: gio input areas occlude POINTER events by default, and a
// window-sized area laid over the content would swallow every click in the
// app.
//
// A focused component that claims the same chord first — a text editor's own
// Cmd-Z, say — receives it and this area does not: the accelerator is the
// app's fallback, not an override.
func shortcutArea(name key.Name, cb func(gtx layout.Context)) layout.Widget {
	tag := new(int)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer pointer.PassOp{}.Push(gtx.Ops).Pop()
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
