package main

import (
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/theme/theme"
)

// buildLayers returns the layer-builder the theme window renders: a
// surface-fill backdrop and a content layer, both reacting to the live theme.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// ContentLayer is the single modelObs consumer counted by modelObsConsumers
// in main.go. The scaffold draws nothing on top of the surface fill.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	return rx.Map(rx.CombineLatest2(th, modelObs),
		func(next rx.Tuple2[theme.Theme, Model]) layout.Widget {
			return View(next.Second)
		})
}

// View builds the page widget for one model. The landing sections are not
// here yet; the window is the surface fill the traffic lights sit on.
func View(model Model) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}
