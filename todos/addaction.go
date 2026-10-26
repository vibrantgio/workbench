package main

import (
	"image"

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
