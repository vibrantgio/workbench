package main

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// hintCell is one settings cell as the anchor of its tooltip: the tooltip is
// placed against the cell while the cell is hovered anywhere or one of its
// controls holds the keyboard.
type hintCell struct {
	// hov tracks the pointer over the whole cell. Its area is registered
	// over the cell's controls in pass-through mode, so the controls still
	// get their own events.
	hov gesture.Hover
	// tags reports the focus tags of the controls in the cell, read each
	// frame: a field's editor is known only once the field is laid out.
	tags func() []event.Tag
	// anchor is the cell's rectangle while it is hovered or holds the
	// keyboard and the empty rectangle otherwise, in the cell's own
	// coordinates. The cell's tooltip reads it as its Props.Anchor.
	anchor image.Rectangle
}

// hintCells are a window's hint cells by slot key, made at subscription
// scope with the tooltips that read them.
type hintCells map[string]*hintCell

// add makes the cell for key, whose controls hold the focus tags tags
// reports, and returns its anchor for the cell's tooltip.
func (hc hintCells) add(key string, tags func() []event.Tag) func(layout.Context) image.Rectangle {
	c := &hintCell{tags: tags}
	hc[key] = c
	return func(layout.Context) image.Rectangle { return c.anchor }
}

// wrap lays w out as the hint cell key and then the cell's tooltip, the slot
// "hint."+key, in the cell's own box. The tooltip is anchored to the cell
// while the pointer is anywhere over it or one of its controls holds the
// keyboard.
func (hc hintCells) wrap(key string, slots slotSet, w layout.Widget) layout.Widget {
	c, ok := hc[key]
	if !ok {
		return w
	}
	return func(gtx layout.Context) layout.Dimensions {
		hovered := c.hov.Update(gtx.Source)
		focused := false
		for _, tag := range c.tags() {
			if tag != nil && gtx.Focused(tag) {
				focused = true
				break
			}
		}
		dims := w(gtx)
		area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		c.hov.Add(gtx.Ops)
		pass.Pop()
		area.Pop()

		c.anchor = image.Rectangle{}
		if hovered || focused {
			c.anchor = image.Rectangle{Max: dims.Size}
		}
		tgtx := gtx
		tgtx.Constraints = layout.Exact(dims.Size)
		slots.get("hint." + key)(tgtx)
		return dims
	}
}
