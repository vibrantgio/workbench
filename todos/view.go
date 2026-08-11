package main

import (
	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// buildLayers returns the layer-builder the theme window renders: a
// backdrop layer and a content layer, both reacting to the live theme.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// themed pairs one theme emission with the palette and typography derived
// from it. Each LiveTheme emission is a static snapshot (every field is an
// rx.Of), so both derive synchronously.
type themed struct {
	components theme.Theme
	palette    Palette
	typ        Type
}

// ContentLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a widget. This is the single modelObs consumer
// counted by modelObsConsumers in main.go.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography),
			func(n rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themed {
				return themed{components: t, palette: PaletteFrom(n.First), typ: TypeFrom(n.Second)}
			})
	})
	return rx.Map(rx.CombineLatest2(themes, modelObs),
		func(next rx.Tuple2[themed, Model]) layout.Widget {
			return View(next.First, next.Second)
		})
}

// View builds the page widget for one (theme, model) pair. Everything here
// is reconstructed per emission; per-interaction state (the editor, the
// clickables) lives inside the widgets for exactly one route's lifetime.
func View(th themed, model Model) layout.Widget {
	thObs := rx.Of(th.components)
	p := th.palette

	add, err := raster.Widget(icons.ContentAddCircle, 40, 40, raster.WithColors(p.Icon))
	if err != nil {
		panic(err)
	}
	fab := Fab(add, 1.0, 1.0, 48, 48, true, func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Route: "add.todo"}}.Add(gtx.Ops)
	})
	list := List(th.typ, thObs, p, model)

	// The dialogs are constructed HERE, once per emission, and reused across
	// frames: the editor's text and caret live inside the dialog closure, so
	// constructing it per frame would discard every keystroke.
	route := model.Route
	var dialog layout.Widget
	switch route {
	case "add.todo":
		dialog = UpsertDialog(th.typ, thObs, p, Todo{Id: -1})
	case "edit.todo":
		if selected, ok := model.List.Find(model.Selected); ok {
			dialog = UpsertDialog(th.typ, thObs, p, selected)
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
