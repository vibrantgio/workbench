package main

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/composite"
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// buildLayers returns the layer-builder the theme window renders: a
// backdrop layer and a content layer, both reacting to the live theme. The
// backdrop is full-bleed, the title-bar strip included; only the resting page
// starts below it.
//
// bars is the platform's "Show scroll bars" setting, read once where the
// window is composed.
func buildLayers(modelObs rx.Observable[Model], bars desktop.ScrollBarSetting) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs, bars),
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
	platform   tokens.PlatformColors
	// typography is the theme's type set, which the list's empty state is
	// set from.
	typography tokens.Typography
	// density is the theme's density, which the toolbar's button is sized
	// from.
	density tokens.Density
	// scrollBars is the platform's "Show scroll bars" setting, read once
	// when the window is composed and passed here, so that no frame asks
	// the platform and no still render depends on the machine it runs on.
	scrollBars desktop.ScrollBarSetting
}

// listBar is the bar the list draws at its trailing edge: the fading
// overlay bar the colours alone give, over the window's own plane the list
// is drawn on, standing without fading where the reader asked for scroll
// bars always.
func (t themed) listBar() scrollbar.Style {
	s := scrollbar.FromTokens(t.platform, t.palette.Backdrop)
	if t.scrollBars == desktop.ScrollBarsAlways {
		return s.Always()
	}
	return s
}

// ContentLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a layout.Widget. This is the window's only consumer
// of the model stream; the backdrop layer is theme-only.
//
// The list's scroll state is kept here, at subscription scope, and passed to
// the page each emission builds, so the list keeps its scroll position across
// changes to a todo and to the theme.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], bars desktop.ScrollBarSetting) rx.Observable[layout.Widget] {
	listState := complist.NewState()
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest3(t.Platform, t.Typography, t.Density),
			func(n rx.Tuple3[tokens.PlatformColors, tokens.Typography, tokens.Density]) themed {
				return themed{components: t, palette: PaletteFrom(n.First), typ: TypeFrom(n.Second),
					platform: n.First, typography: n.Second, density: n.Third, scrollBars: bars}
			})
	})
	return rx.Map(rx.CombineLatest2(themes, modelObs),
		func(next rx.Tuple2[themed, Model]) layout.Widget {
			return View(next.First, next.Second, listState)
		})
}

// View builds the page's layout.Widget for one (theme, model) pair. The
// toolbar takes the window's top band, as deep as the platform's toolbar or
// the native title-bar strip, whichever is deeper, and the list starts below
// it. Everything here is reconstructed per emission except the list's scroll
// state, which is passed in; per-interaction state (the editor, the
// clickables) is kept inside the controls for exactly one route's lifetime.
func View(th themed, model Model, listState *complist.State) layout.Widget {
	return view(th, model, listState, func() unit.Dp { return shell.ChromeTop(shell.ToolbarDepth) })
}

// view is View over a stated band depth rather than the window's own, so a
// test can state a band it has no window to measure.
//
// The band is the toolbar's, and it carries no fill of its own: the window's
// own plane is under it, which BackdropLayer already fills full-bleed, so the
// fill reaches the top edge without anything being painted twice. Only the
// list has to stay clear of the band.
//
// The dialog does not. A scrim with a strip-shaped hole in its top edge
// isolates nothing, so the dialog and its cover are laid out in the window's
// own coordinates: transient surfaces lie over the resting window rather than
// in it.
func view(th themed, model Model, listState *complist.State, band func() unit.Dp) layout.Widget {
	thObs := rx.Of(th.components)
	p := th.palette

	add := func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Route: "add.todo"}}.Add(gtx.Ops)
	}
	toolbar := Toolbar(th.platform, th.density, new(widget.Clickable), band, add)
	addKey := OnAddKey(add)
	list := List(th.typ, thObs, p, th.density, th.listBar(), listState, model)

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

	// The list is the region that scrolls and takes the frame below the
	// band: its padding is inside it, so its bar is at the window's trailing
	// edge.
	page := desktop.InsetTop(band, list)

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		// The window's own plane, declared before anything is offset, so the
		// dialog's scrim composites over the page rather than blending with
		// it. The backdrop layer beneath has already painted into these ops.
		composite.Frame(gtx)

		// A list with nothing in it draws its empty state under the page,
		// so the dialog's scrim covers it as it covers the rows.
		if len(model.List) == 0 {
			EmptyList(gtx, th)
		}

		if route == "" {
			page(gtx)
			toolbar(gtx)
			addKey(gtx)
		} else {
			page(gtx.Disabled())
			toolbar(gtx.Disabled())
			dialog(gtx)
			// The band is given back its drag on top of what was just drawn
			// over it. The dialog's Escape catcher spans the whole window so
			// Escape closes the dialog wherever the pointer is, and a
			// whole-window input region recorded after the band would shadow
			// the drag claim, leaving a window that cannot be moved while a
			// text field is open. The dialog is centred and 200 dp tall, so
			// nothing of its own stands in the band to be swallowed.
			desktop.DragTop(gtx, band)
		}

		return layout.Dimensions{Size: size}
	}
}
