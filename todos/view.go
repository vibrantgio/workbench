package main

import (
	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
)

// buildLayers returns the layer-builder the spectrum window renders: a
// backdrop layer and a content layer, both reacting to the live theme.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// themed pairs one theme emission with the palette derived from it. Each
// LiveTheme emission is a static snapshot (every field is an rx.Of), so the
// palette derives synchronously.
type themed struct {
	prism   theme.Theme
	palette Palette
}

// ContentLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a widget. This is the single modelObs consumer
// counted by modelObsConsumers in main.go.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	shaper := text.NewShaper(text.WithCollection(style.FontFaces()))
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(t.Color, func(c tokens.ColorTokens) themed {
			return themed{prism: t, palette: PaletteFrom(c)}
		})
	})
	return rx.Map(rx.CombineLatest2(themes, modelObs),
		func(next rx.Tuple2[themed, Model]) layout.Widget {
			return View(shaper, next.First, next.Second)
		})
}

// View builds the page widget for one (theme, model) pair. Everything here
// is reconstructed per emission; per-interaction state (the editor, the
// clickables) lives inside the widgets for exactly one route's lifetime.
func View(shaper *text.Shaper, th themed, model Model) layout.Widget {
	thObs := rx.Of(th.prism)
	p := th.palette

	add, err := raster.Widget(icons.ContentAddCircle, 40, 40, raster.WithColors(p.Icon))
	if err != nil {
		panic(err)
	}
	fab := Fab(add, 1.0, 1.0, 48, 48, true, func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Route: "add.todo"}}.Add(gtx.Ops)
	})
	list := List(shaper, thObs, p, model)

	// The dialogs are constructed HERE, once per emission, and reused across
	// frames: the editor's text and caret live inside the dialog closure, so
	// constructing it per frame would discard every keystroke.
	route := model.Route
	var dialog layout.Widget
	switch route {
	case "add.todo":
		dialog = UpsertDialog(shaper, thObs, p, Todo{Id: -1})
	case "edit.todo":
		if selected, ok := model.List.Find(model.Selected); ok {
			dialog = UpsertDialog(shaper, thObs, p, selected)
		} else {
			// The edit target was deleted out from under the route;
			// fall back to the list rather than editing a zero Todo.
			route = ""
		}
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max

		if route == "" {
			layout.UniformInset(Padding).Layout(gtx, list)
			layout.UniformInset(Padding).Layout(gtx, fab)
		} else {
			layout.UniformInset(Padding).Layout(gtx.Disabled(), list)
			layout.UniformInset(Padding).Layout(gtx.Disabled(), fab)
			dialog(gtx)
		}

		return layout.Dimensions{Size: size}
	}
}
