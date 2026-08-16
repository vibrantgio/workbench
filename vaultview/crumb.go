// crumb.go is the app's breadcrumb row: a horizontal trail of segments
// separated by chevrons, ancestors clickable and the trailing segment the
// current location. Both trails in the app wear it — the picker's
// directory path and the note header's vault / folder / title trail.
//
// It is an app-local composition rather than the vocabulary's breadcrumb
// row for one reason: that row fixes its items and their clickables when
// the stream is built, and both trails here change with every model
// emission. The visual language is the same — the trailing segment in the
// text colour, ancestors in the low-contrast neutral step. Colour follows
// position and interactivity follows the message, so the two can disagree
// without either lying.

package main

import (
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/widget"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu"
)

// Crumb layout constants.
const crumbGapDp = 4

// crumb is one segment of a trail: its label and the message clicking it
// emits. A nil message leaves the segment inert.
type crumb struct {
	label string
	msg   mvu.Message
}

// crumbRow holds a trail's per-segment clickables, pointer-stable across
// frames. The slice grows to the longest trail the row has drawn.
type crumbRow struct {
	clicks []*widget.Clickable
}

// layout draws the trail and emits a clicked segment's message on the same
// frame.
func (r *crumbRow) layout(gtx layout.Context, tok themeTokens, items []crumb) layout.Dimensions {
	for len(r.clicks) < len(items) {
		r.clicks = append(r.clicks, &widget.Clickable{})
	}
	var children []layout.FlexChild
	for i, item := range items {
		item := item
		if i > 0 {
			children = append(children,
				layout.Rigid(complayout.HSpacer(crumbGapDp)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLabel(gtx, tok.shaper, "›", tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
				}),
				layout.Rigid(complayout.HSpacer(crumbGapDp)),
			)
		}
		fg := tok.col.Ramps.Neutral.Step(700)
		if i == len(items)-1 {
			fg = tok.col.Text
		}
		label := func(gtx layout.Context) layout.Dimensions {
			return drawLabel(gtx, tok.shaper, item.label, tok.typ.TitleSmall, fg)
		}
		if item.msg == nil {
			children = append(children, layout.Rigid(label))
			continue
		}
		click := r.clicks[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				mvu.MessageOp{Message: item.msg}.Add(gtx.Ops)
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(item.label).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				return label(gtx)
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}
