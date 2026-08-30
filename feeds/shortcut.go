// shortcut.go holds the app's keyboard accelerators — the INVOCATION half of
// the dialog grammar.
//
// A modal cannot own how you arrived. patterns/modal owns dismissal (a panel's
// ghost X, Escape, the backdrop; a decision's Escape-to-Cancel and
// Return-to-default) because those are affordances the dialog itself draws or
// traps while it is on screen. Arrival is app chrome: the accelerator has to
// be live when NO dialog exists, which is precisely the state the modal is not
// in. So the binding lives here, one function, and patterns carries no notion
// of it.
//
// The platform-correct modifier is Gio's, not ours: key.ModShortcut is Cmd on
// darwin and Ctrl everywhere else. Testing runtime.GOOS here would be a
// second copy of a decision the toolkit already made.
package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// prefsAccelerator is the settings accelerator every desktop app on both
// platforms binds: ⌘, on macOS, Ctrl-, elsewhere. It opens the Preferences
// panel and nothing else — closing it is the panel's job.
const prefsAccelerator = ","

// shortcutArea returns a widget that invokes cb on every press of name with
// the platform shortcut modifier held. Lay it out over the area the shortcut
// should be live in — for a global accelerator, the whole window, FIRST, so
// it sits at the bottom of the hit stack under the content.
//
// The area exists only to receive KEY events, so it is wrapped in
// pointer.PassOp: gio input areas occlude POINTER events by default, and a
// window-sized area laid over the content would swallow every click in the
// app.
//
// A focused widget that claims the same chord first — a text editor's own
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
