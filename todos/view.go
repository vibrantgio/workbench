package main

import (
	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// buildLayers returns the layer-builder the theme window renders: a
// backdrop layer and a content layer, both reacting to the live theme. The
// backdrop is full-bleed, the title-bar strip included; only the resting page
// starts below it.
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

// View builds the page widget for one (theme, model) pair, held down past the
// native title-bar strip the full-size-content window opens the top of itself
// into. Everything here is reconstructed per emission; per-interaction state
// (the editor, the clickables) lives inside the widgets for exactly one
// route's lifetime.
func View(th themed, model Model) layout.Widget {
	return view(th, model, desktop.TopInset)
}

// view is View over a stated strip height rather than the window's own — the
// same split desktop.InsetTop's own height parameter already makes — so a test
// can state a strip it has no window to measure.
//
// The strip carries no fill of its own, and that is R6 satisfied rather than
// skipped: the region this band caps is the window's own ground — the
// Background pin BackdropLayer fills the whole window with — so the region's
// fill reaches the top edge without anything being painted twice. A band drawn
// here would be furniture this window does not have; the list is the content
// ground and the only other thing standing on it is a floating button. What
// has to stay clear of the strip is the resting page, which starts below it.
//
// The modal does not. A scrim isolates the window it covers, and a scrim with
// a strip-shaped hole in its top edge isolates nothing — so the dialog and its
// cover are laid out in the window's own coordinates, over the strip as over
// everything else, which is also where R7's walk puts them: transient
// surfaces lie over the resting window rather than in it.
func view(th themed, model Model, strip func() unit.Dp) layout.Widget {
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

	// The resting page — the list on the ground and the button floating over
	// it — capped by the strip. desktop.InsetTop reports the size it was given
	// rather than the inset one, so the button still lands on the window's
	// bottom-trailing corner and only the top edge moves.
	resting := func(gtx layout.Context) layout.Dimensions {
		layout.UniformInset(Padding).Layout(gtx, list)
		return layout.UniformInset(Padding).Layout(gtx, fab)
	}
	capped := dragUnderStrip(strip, resting)

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max

		if route == "" {
			capped(gtx)
		} else {
			capped(gtx.Disabled())
			dialog(gtx)
			// And the strip is given back its drag, on top of what was just
			// drawn over it. The modal's Escape catcher spans the whole window
			// so that Escape closes the dialog wherever the pointer is, and a
			// whole-window input region recorded after the band shadows it —
			// leaving a window that cannot be moved for as long as a text field
			// is open, which is not a thing any window on this platform does. A
			// second claim over the same strip settles it; the dialog is
			// centred and 200 dp tall, so nothing of its own stands in there to
			// be swallowed.
			desktop.DragTop(gtx, strip)
		}

		return layout.Dimensions{Size: size}
	}
}

// dragUnderStrip pads a widget down by a native title-bar strip's height and
// claims that same strip for the window's own drag.
//
// The height is read at frame time rather than taken as a value because
// desktop.TopInset is measured from the live window: it reports 0 until the
// first frame, in headless renders, and on every platform but macOS, so away
// from the full-size-content treatment the wrapper is an exact no-op.
//
// desktop.DragTop is the other half of R6. The native drag leaves with the
// native strip, and the strip here carries paint but no widget of its own, so
// without this claim the window could not be moved by its top edge at all.
func dragUnderStrip(height func() unit.Dp, w layout.Widget) layout.Widget {
	inset := desktop.InsetTop(height, w)
	return func(gtx layout.Context) layout.Dimensions {
		desktop.DragTop(gtx, height)
		return inset(gtx)
	}
}
