package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// addTitle is the add action's title. The button shows the plus symbol, and
// assistive technology reads this title.
const addTitle = "New Todo"

// addKeyName is the add action's key equivalent with the shortcut modifier:
// Command-N, the platform's New chord.
const addKeyName key.Name = "N"

// Toolbar draws the window's toolbar across a band of the given depth. The
// add action is a bordered toolbar button at the band's trailing end, with
// the toolbar's trail between it and the window's edge. The button shows
// the plus symbol in the toolbar's label colour. The band paints no fill:
// the window's own plane is under it.
//
// The run over the window's control buttons carries no drag, so the drag
// does not take their presses. The rest of the band, the button excepted,
// moves the window.
func Toolbar(c tokens.PlatformColors, den tokens.Density, click *widget.Clickable, depth func() unit.Dp, add func(gtx layout.Context)) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if click.Clicked(gtx) {
			add(gtx)
		}
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(depth()))
		gtx.Constraints = layout.Exact(size)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		buttons := gtx.Dp(desktop.LeadingInset())
		layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(buttons, size.Y)}
			}),
			layout.Flexed(1, pane.DragFill),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return shell.ToolbarControl(gtx, c, tokens.Radius, den, click, icons.Mark(icons.Plus), addTitle, false)
			}),
			layout.Rigid(pane.DragSpacer(shell.PlatformToolbar.Trail)),
		)
		return layout.Dimensions{Size: size}
	}
}

// OnAddKey calls add when the add action's key equivalent is pressed. Its
// area spans the frame it is given and lets pointer input pass through.
func OnAddKey(add func(gtx layout.Context)) layout.Widget {
	tag := new(int)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer pointer.PassOp{}.Push(gtx.Ops).Pop()
		defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, tag)
		for {
			e, ok := gtx.Event(key.Filter{Name: addKeyName, Required: key.ModShortcut})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				add(gtx)
			}
		}
		return layout.Dimensions{Size: size}
	}
}
