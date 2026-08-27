package main

import (
	"gioui.org/app"
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
)

// buildLayers returns the layer-builder the theme window renders, back
// to front: the Background pin, the wireframe triangle field, and the
// marketing page. The field is full-bleed, including under the title-bar
// strip; only the page is inset.
func buildLayers(win *app.Window, modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			FieldLayer(win, th),
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
//
// The strip also carries the window's own drag: it holds paint but no widget
// of its own, so without a claim over it the window could not be moved by its
// top edge at all. desktop.CapTop is the two together — the claim over the
// strip and the page held down past it, over one measured height.
func underTitleBar(pageObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(pageObs, func(w layout.Widget) layout.Widget {
		return desktop.CapTop(desktop.TopInset, w)
	})
}
