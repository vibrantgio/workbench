package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/alert"
	"github.com/vibrantgio/components/badge"
	"github.com/vibrantgio/components/button"
	pllayout "github.com/vibrantgio/components/layout"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/group"
	"github.com/vibrantgio/patterns/hero"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Static layout dimensions; these do not vary with the colour scheme.
const (
	CellW    unit.Dp = 300 // one app group
	CellH    unit.Dp = 190
	ButtonW  unit.Dp = 110 // fixed launch-button width, so titles don't resize it
	IconSize unit.Dp = 28
	RowGap   float32 = 16 // dp between groups, and between the rows
	perRow           = 4  // groups per grid row; the last row holds the rest
)

// buildLayers returns the layer-builder the theme window renders, back to
// front: the theme background fill, the animated seen triangle field, and the
// hero + launch-grid content floating on top. The background fill and the
// field are full-bleed, the title-bar strip included; only the page is inset
// below it.
func buildLayers(win *app.Window, modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			FieldLayer(win, th),
			ContentLayer(th, modelObs),
		}
	}
}

// BackdropLayer fills the window with its own plane; it is the bottom layer
// and re-emits whenever the OS colour scheme changes.
//
// The workbench is one plane edge to edge — the page of app groups stands
// straight on it, with the field between — so the fill is the window's own
// and no region paints a band of its own over it.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
		return backdrop.Widget(c.WindowBackground)
	})
}

// FieldLayer is the animated seen triangle field. The Field — its scene,
// animation loop and palette — is subscription-scoped state, so it lives in an
// rx.Defer factory: each theme emission re-keys its palette in place, and the
// field itself is built exactly once per subscription.
func FieldLayer(win *app.Window, th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		field := NewField(win, winW, winH)
		return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
			field.SetColors(c)
			return field.Widget()
		})
	})
}

// HeroProps is the page's title block, stated once so a render made outside
// the running window puts the same words on the page as the window does.
var HeroProps = hero.Props{
	Eyebrow: "Vibrant Gio",
	Title:   "Workbench",
	// The subtitle names no count of the apps: a number written here is one
	// nobody updates when an app is added.
	Subtitle: "Complete example apps built on mvu, components, theme, patterns and seen.",
}

// windowPlane is the fill this window's page stands on: the window's own
// plane, which the workbench carries edge to edge — the title-bar strip
// included — since nothing here paints a band of its own. Every
// alpha-carrying platform name the page draws is flattened onto it before it
// reaches Gio.
//
// It is a function rather than a field on themed so this window's own tests
// can name the same fill the window draws.
func windowPlane(c tokens.PlatformColors) color.NRGBA { return c.WindowBackground }

// tileMaterial is the fill a cell of the grid stands on: the window
// background colour at the platform's measured coverage, which is what the
// platform paints over a painted backdrop so what is behind shows through.
// Here that is the animated field, and it shows through lighter in the light
// appearance and darker in the dark one.
//
// The coverage handed to the rasterizer is the linear-light fit, because Gio
// mixes a translucent fill in linear light where the platform mixes the
// encoded byte. What the tile's material tends toward is windowPlane, and
// every foreground on the cell is flattened onto that, which is why nothing
// on the cell changes colour when the fill arrives under it.
func tileMaterial(c tokens.PlatformColors) color.NRGBA {
	return vgcolor.Fade(windowPlane(c), vgcolor.LinearCoverage(tokens.BackdropMaterialCoverage))
}

// themed is one theme emission resolved to the token snapshot the view
// consumes, alongside the emission itself: the snapshot's fields are all
// rx.Of, so the theme-driven components built from components resolve
// synchronously via First().
type themed struct {
	components theme.Theme
	color      tokens.PlatformColors
	spacing    tokens.SpacingScale
	radius     tokens.RadiusScale
	typ        tokens.Typography
	shaper     *text.Shaper // the theme's cached shaper (Typography.Shaper())
}

// ContentLayer is the page, held down past the native title-bar strip. It is
// this window's only consumer of the model stream; the backdrop and field
// layers are theme-only.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	return underTitleBar(pageLayer(th, modelObs))
}

// underTitleBar pads the page down by the native title-bar strip's measured
// height on a full-size-content window. desktop.TopInset is read at frame
// time: it reports 0 until the window's first frame, in headless renders, and
// on every platform but macOS, so away from the treatment the wrapper is an
// exact no-op.
//
// The strip carries no fill of its own: the background fill and the field
// beneath it are already full-bleed, so the region reaches the top edge
// without anything being painted twice. Only the page has to stay clear of
// the strip.
// desktop.CapTop also claims the strip for the window's own drag — without
// that claim the window could not be moved by its top edge.
func underTitleBar(pageObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(pageObs, func(w layout.Widget) layout.Widget {
		return desktop.CapTop(desktop.TopInset, w)
	})
}

// pageLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a layout.Widget. The launch buttons' clickables are
// subscription-scoped (rx.Defer) so press/hover/focus state survives the
// per-message view rebuilds; the hero is its own theme-driven component
// observable, rebuilt only when the theme changes.
func pageLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(
			rx.CombineLatest4(t.Platform, t.Spacing, t.Radius, t.Typography),
			func(n rx.Tuple4[tokens.PlatformColors, tokens.SpacingScale, tokens.RadiusScale, tokens.Typography]) themed {
				typ := n.Fourth
				return themed{components: t, color: n.First, spacing: n.Second, radius: n.Third, typ: typ, shaper: typ.Shaper()}
			},
		)
	})
	heroObs := hero.Hero(th, HeroProps)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		clicks := make([]widget.Clickable, len(Apps))
		return rx.Map(rx.CombineLatest3(resolved, heroObs, modelObs),
			func(next rx.Tuple3[themed, layout.Widget, Model]) layout.Widget {
				return View(next.First, next.Second, clicks, next.Third)
			})
	})
}

// View builds the page's layout.Widget for one (theme, model) pair: a hero
// title block over a grid of app groups, the whole column centred on the
// field. A roster that outgrows a row wraps onto a further row rather than
// shrinking a cell.
func View(tok themed, heroW layout.Widget, clicks []widget.Clickable, model Model) layout.Widget {
	cells := make([]layout.Widget, len(Apps))
	for i, app := range Apps {
		cells[i] = appGroup(tok, app, &clicks[i], model.StatusOf(app.Name))
	}
	var rows []layout.Widget
	for i := 0; i < len(cells); i += perRow {
		rows = append(rows, gridRow(cells[i:min(i+perRow, len(cells))]))
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// The hero pads itself by one S6 step; the rows are given the
			// same padding so that the two share a leading edge instead of
			// ragging by the width of the hero's inset.
			gutter := layout.Inset{Left: unit.Dp(tok.spacing.S6), Right: unit.Dp(tok.spacing.S6)}
			children := []layout.FlexChild{layout.Rigid(heroW), layout.Rigid(pllayout.VSpacer(RowGap))}
			for i, row := range rows {
				if i > 0 {
					children = append(children, layout.Rigid(pllayout.VSpacer(RowGap)))
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return gutter.Layout(gtx, row)
				}))
			}
			// Start, not Middle: every row is already GridW wide, so the
			// column's leading edge is the grid's first column and the hero
			// lands on it too. Centring each child on its own width would make
			// the title's position depend on the length of the sentence under
			// it.
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
		})
		return layout.Dimensions{Size: size}
	}
}

// GridW is the width of a full row. Every row is laid out at this
// width whatever it holds, so a last row with fewer cells than the rest
// starts in the first column instead of centring itself half a cell off it.
const GridW unit.Dp = perRow*CellW + (perRow-1)*unit.Dp(RowGap)

// gridRow lays out one row of fixed-size cells with RowGap gaps, from the
// leading edge of a full-width row.
func gridRow(cells []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var children []layout.FlexChild
		for i, cell := range cells {
			if i > 0 {
				children = append(children, layout.Rigid(pllayout.HSpacer(RowGap)))
			}
			children = append(children, layout.Rigid(cell))
		}
		gtx.Constraints.Min.X = gtx.Dp(GridW)
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		// Start: a card that grows under an alert keeps its top edge on the
		// row's top edge, and so do the cards beside it.
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
	}
}

// appGroup is one launchable app as a patterns group standing on the tile's
// material over the painted backdrop: icon + name row, blurb, and the launch
// button. The group and button are
// theme-driven components built from the emission's static snapshot; the
// name is the platform's label and the blurb its secondary strength, both
// flattened onto what that fill tends toward, the window's own plane.
//
// A group and not a card, because the roster is the page dividing itself:
// every app on the workbench is a peer of every other, and nothing here is
// singled out. A translucent material raises nothing — it is the window's
// own plane at a coverage — and the hairline stays the whole of what chunks
// one app from the next.
func appGroup(tok themed, app App, click *widget.Clickable, status Status) layout.Widget {
	thObs := rx.Of(tok.components)
	plane := windowPlane(tok.color)

	blurb := textWidget(tok.shaper, app.Blurb, tok.typ.Subheadline, vgcolor.Flatten(tok.color.SecondaryLabel, plane), 3)
	launchRow := launchButton(thObs, tok.shaper, app, click, status, plane)

	// The app's own name is already the first row, so the group takes no
	// title of its own: a second copy of it above the icon would name the
	// group twice.
	//
	// The group paints nothing inside itself, so the fill its hairline is
	// flattened onto is what the cell carries under it — the tile's
	// material, whose own colour is the plane the page stands on.
	//
	// thObs is a static snapshot (rx.Of), so First() resolves synchronously.
	//
	// The alert of a failed launch is a slot of its own under the name row,
	// and the launch button is a later slot: the button is never inside the
	// alert. The group drops a nil slot, so a card with no failure has none.
	failure := failureAlert(tok, app, status)
	inner, _ := group.Group(thObs, group.Props{
		Content: []layout.Widget{nameRow(tok, app, status), failure, blurb, launchRow},
		Surface: plane,
	}).First()
	material := tileMaterial(tok.color)
	radius := tok.radius.Lg
	return func(gtx layout.Context) layout.Dimensions {
		height := gtx.Dp(CellH)
		if failure != nil {
			height += failureHeight(gtx, tok, failure)
		}
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(CellW), height))
		// The tile's material goes down first, at the group's own radius,
		// so the hairline the group draws lands on its edge rather than
		// beside it.
		r := gtx.Dp(unit.Dp(radius))
		rrect := clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Max}, SE: r, SW: r, NE: r, NW: r}
		paint.FillShape(gtx.Ops, material, rrect.Op(gtx.Ops))
		return inner(gtx)
	}
}

// failureTitle is the title of the alert on a card whose launch failed, in
// the platform's wording. The process that could not be opened and the
// process that ended each have their own wording, and both put the name
// between the platform's curly quotation marks.
func failureTitle(app App, status Status) string {
	if status.Ended {
		return fmt.Sprintf("“%s” quit unexpectedly.", app.Name)
	}
	return fmt.Sprintf("The application “%s” can’t be opened.", app.Name)
}

// failureAlert is the Error alert of a failed launch, or nil when the last
// launch did not fail. Its body is the error text as the process gave it, on
// as many lines as that text takes: no line cap and no ellipsis.
func failureAlert(tok themed, app App, status Status) layout.Widget {
	if status.State != Failed {
		return nil
	}
	// The body is on the alert's own fill, so its colour is flattened onto
	// that fill and not onto the window's plane.
	body := textWidget(tok.shaper, status.Detail, tok.typ.Subheadline, vgcolor.Flatten(tok.color.Label, tok.color.ControlBackground), 0)
	// The theme is a static snapshot (rx.Of), so First() resolves
	// synchronously.
	w, _ := alert.Alert(rx.Of(tok.components), alert.Props{
		Status:  alert.Error,
		Title:   failureTitle(app, status),
		Body:    body,
		Shaper:  tok.shaper,
		Surface: windowPlane(tok.color),
	}).First()
	return w
}

// failureHeight is the height a card adds for its alert: the alert's own
// depth at the width the group gives its slots, and the group's gap above
// it. The alert is laid out on scratch ops, so the measure draws nothing.
func failureHeight(gtx layout.Context, tok themed, failure layout.Widget) int {
	measure := gtx
	measure.Ops = new(op.Ops)
	width := gtx.Dp(CellW) - 2*gtx.Dp(unit.Dp(tok.spacing.S4))
	measure.Constraints = layout.Constraints{Max: image.Pt(width, 1<<24)}
	return failure(measure).Size.Y + gtx.Dp(unit.Dp(tok.spacing.S3))
}

// textColour is the colour the name and the icon on a cell are drawn in:
// the platform's label flattened onto the window's own plane, the fill the
// cell's material tends toward.
func textColour(c tokens.PlatformColors) color.NRGBA {
	return vgcolor.Flatten(c.Label, windowPlane(c))
}

// appIcon is an application's icon, drawn in the text colour. The icon is on
// the content and marks nothing as selected, so it is never drawn in the
// accent.
func appIcon(tok themed, app App) layout.Widget {
	icon, err := raster.Widget(app.Icon, IconSize, IconSize, raster.WithColors(textColour(tok.color)))
	if err != nil {
		return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	return icon
}

// launchButton is the theme-driven components button wired to the caller-owned
// clickable — so press/hover/focus state survives the per-message view
// rebuilds — emitting Launch into the MVU loop on activation. While the app is
// starting or running it renders disabled and emits nothing; the reducer
// guards again anyway.
//
// It is the Filled button, the one action its cell is about: its fill is read
// from the platform's default button fill token and its title is drawn in the
// colour that token is paired with, so no fill is pinned here.
//
// It shapes with the shaper the rest of the cell uses rather than reaching for
// the theme's own, so one page has one shaper and a render made outside the
// window can say which.
func launchButton(th rx.Observable[theme.Theme], shaper *text.Shaper, app App, click *widget.Clickable, status Status, surface color.NRGBA) layout.Widget {
	busy := status.State == Starting || status.State == Running
	txt := "Launch"
	switch status.State {
	case Starting:
		txt = "Starting"
	case Running:
		txt = "Running"
	case Failed:
		txt = "Relaunch"
	}
	// th is a static snapshot (rx.Of), so First() resolves synchronously.
	btn, _ := button.Button(th, button.Props{
		Title:       txt,
		Description: "Launch " + app.Name,
		Disabled:    rx.Of(busy),
		Clickable:   click,
		Message:     Launch{Name: app.Name},
		Shaper:      shaper,
		Surface:     surface,
	}).First()
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(ButtonW)
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		return btn(gtx)
	}
}

// nameRow is the app's icon and name, and while the app runs a Neutral badge
// with the text "Running" after the name. Running is the system's word about
// the app while it is true, so it is a badge with no status: the system grey
// with white text, never the accent, which marks a selection.
func nameRow(tok themed, app App, status Status) layout.Widget {
	children := []layout.FlexChild{
		layout.Rigid(appIcon(tok, app)),
		layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
		layout.Rigid(textWidget(tok.shaper, app.Name, tok.typ.Title3, textColour(tok.color), 1)),
	}
	if status.State == Running {
		// The theme is a static snapshot (rx.Of), so First() resolves
		// synchronously.
		running, _ := badge.Badge(rx.Of(tok.components), badge.Props{
			Text:    "Running",
			Status:  badge.Neutral,
			Surface: windowPlane(tok.color),
			Shaper:  tok.shaper,
		}).First()
		children = append(children,
			layout.Rigid(pllayout.HSpacer(tok.spacing.S2)),
			layout.Rigid(running),
		)
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// textWidget defers a run of text in one Typography role, capped at maxLines,
// as the layout.Widget every row of this window is assembled from. The minimum
// is relaxed before the draw, so what comes back is the text's own size rather
// than the slot it was offered.
func textWidget(shaper *text.Shaper, txt string, style tokens.TextStyle, col color.NRGBA, maxLines int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return typeset.Text(gtx, shaper, txt, style, col, maxLines)
	}
}
