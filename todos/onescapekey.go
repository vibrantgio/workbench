package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

func OnEscapeKey(cb func(gtx layout.Context)) layout.Widget {
	esc := new(int)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, esc)
		for {
			e, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if e, ok := e.(key.Event); ok && e.State == key.Press {
				cb(gtx)
			}
		}
		return layout.Dimensions{Size: size}
	}
}
