// split.go is the feeds-local articles/detail split pane. The plan suggested
// cadence/shell.Shell(SplitPane), but that component's per-subscription
// dragState is written by the emission projector (rx scheduler goroutine)
// and read by the emitted widget at frame time — a data race go test -race
// flags on every model-driven SplitRatio emission (see FEEDBACK-G5.2.md).
//
// This MVU-pure replacement has no shared mutable state across goroutines:
// the emitted widget closes over the ratio value carried by the model
// emission, divider drags land SetSplitRatio messages through the mvu loop,
// and the transient drag tracker is written and read only during layout on
// the frame goroutine.
package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// Divider geometry and ratio clamp, mirroring cadence/shell's SplitPane
// constants so the visuals match the component this replaces.
const (
	splitDividerDp = 6
	splitMinRatio  = 0.05
	splitMaxRatio  = 0.95
)

// splitTag is a non-zero-size type so its address is a unique event tag for
// the divider's pointer hit area.
type splitTag struct{ _ byte }

// splitDrag tracks an in-progress divider drag. Touched ONLY during layout
// (frame goroutine): Press records the grab point and the committed ratio,
// Drag emits SetSplitRatio from those plus the event position. The committed
// ratio always flows back in via the model, never via this struct.
type splitDrag struct {
	tag    splitTag
	pressX float32
	startR float32
	active bool
}

// feedsSplitPane lays left and right side-by-side with a draggable vertical
// divider at ratioObs (model-derived). Drags land SetSplitRatio messages, so
// the divider position survives theme re-emissions and is replayable like
// every other piece of app state.
func feedsSplitPane(
	th rx.Observable[theme.Theme],
	ratioObs rx.Observable[float32],
	left, right layout.Widget,
) rx.Observable[layout.Widget] {
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	ds := &splitDrag{} // one tracker per pane; feedsSplitPane is called once.
	return rx.Map(
		rx.CombineLatest2(colorObs, ratioObs),
		func(n rx.Tuple2[tokens.ColorTokens, float32]) layout.Widget {
			colors, ratio := n.First, clampSplitRatio(n.Second)
			return func(gtx layout.Context) layout.Dimensions {
				return drawFeedsSplit(gtx, ds, ratio, left, right, colors)
			}
		},
	)
}

func clampSplitRatio(r float32) float32 {
	if r < splitMinRatio {
		return splitMinRatio
	}
	if r > splitMaxRatio {
		return splitMaxRatio
	}
	return r
}

// drawFeedsSplit processes pending divider drags, then lays the two panes
// around the divider. Pane widths derive from the committed (model) ratio;
// a drag updates the model via messages, so this frame's layout and the
// next frame's both read a single source of truth.
func drawFeedsSplit(
	gtx layout.Context,
	ds *splitDrag,
	ratio float32,
	left, right layout.Widget,
	colors tokens.ColorTokens,
) layout.Dimensions {
	size := gtx.Constraints.Max
	processSplitDrag(gtx, ds, ratio)

	dividerW := gtx.Dp(unit.Dp(splitDividerDp))
	if dividerW > size.X {
		dividerW = size.X
	}
	leftW := int(ratio * float32(size.X-dividerW))
	rightW := size.X - dividerW - leftW

	lgtx := gtx
	lgtx.Constraints = layout.Exact(image.Pt(leftW, size.Y))
	left(lgtx)

	divRect := image.Rect(leftW, 0, leftW+dividerW, size.Y)
	paint.FillShape(gtx.Ops, colors.Outline, clip.Rect(divRect).Op())
	area := clip.Rect(divRect).Push(gtx.Ops)
	event.Op(gtx.Ops, &ds.tag)
	pointer.CursorColResize.Add(gtx.Ops)
	area.Pop()

	st := op.Offset(image.Pt(leftW+dividerW, 0)).Push(gtx.Ops)
	rgtx := gtx
	rgtx.Constraints = layout.Exact(image.Pt(rightW, size.Y))
	right(rgtx)
	st.Pop()

	return layout.Dimensions{Size: size}
}

// processSplitDrag drains the divider's pointer events. Press grabs the
// committed ratio; each Drag emits a SetSplitRatio computed from the grab
// point and the event position — no incremental local ratio, so a missed
// frame cannot accumulate drift.
func processSplitDrag(gtx layout.Context, ds *splitDrag, committed float32) {
	totalW := float32(gtx.Constraints.Max.X)
	if totalW <= 0 {
		return
	}
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: &ds.tag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			ds.active = true
			ds.pressX = pe.Position.X
			ds.startR = committed
		case pointer.Drag:
			if !ds.active {
				continue
			}
			r := clampSplitRatio(ds.startR + (pe.Position.X-ds.pressX)/totalW)
			mvu.MessageOp{Message: SetSplitRatio{Ratio: r}}.Add(gtx.Ops)
		case pointer.Release, pointer.Cancel:
			ds.active = false
		}
	}
}
