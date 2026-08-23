package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
)

// buildLayers returns the layer-builder the theme window renders: a
// surface-fill backdrop and the marketing page, both reacting to the
// live theme.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// ContentLayer is the single modelObs consumer counted by modelObsConsumers
// in main.go. The page is inset by desktop.TopInset() so the Hero sits
// below the traffic lights; that inset is 0 off macOS and in goldens.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	return underTitleBar(pageLayer(th, modelObs))
}

// underTitleBar pads the page down by the native title-bar strip's
// measured height on a full-size-content window. desktop.TopInset is
// read at frame time: it reports 0 until the window's first frame, in
// headless tests, and on every platform but macOS.
func underTitleBar(pageObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(pageObs, func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := gtx.Dp(desktop.TopInset())
			if inset <= 0 {
				return w(gtx)
			}
			size := gtx.Constraints.Max
			defer op.Offset(image.Pt(0, inset)).Push(gtx.Ops).Pop()
			gtx.Constraints.Max.Y -= inset
			if gtx.Constraints.Min.Y > gtx.Constraints.Max.Y {
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			}
			w(gtx)
			return layout.Dimensions{Size: size}
		}
	})
}
