package main

import (
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/button"
	pllayout "github.com/vibrantgio/components/layout"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/group"
	"github.com/vibrantgio/patterns/hero"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Static layout dimensions; these do not vary with the colour scheme.
const (
	CellW    unit.Dp = 300 // one app group
	CellH    unit.Dp = 190
	ButtonW  unit.Dp = 110 // fixed launch-button width, so labels don't resize it
	IconSize unit.Dp = 28
	RowGap   float32 = 16 // dp between groups, and between the rows
	perRow           = 4  // groups per grid row; the last row holds the rest
)

// buildLayers returns the layer-builder the theme window renders, back to
// front: the theme background fill, the animated seen triangle field, and the
// hero + launch-grid content floating on top. The ground and the field are
// full-bleed, the title-bar strip included; only the page is inset below it.
func buildLayers(win *app.Window, modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			FieldLayer(win, th),
			ContentLayer(th, modelObs),
		}
	}
}

// BackdropLayer fills the window with the theme's background colour; it is
// the bottom layer and re-emits whenever the OS colour scheme changes.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		return backdrop.Widget(c.Background)
	})
}

// FieldLayer is the animated seen triangle field. The Field — its scene,
// animation loop and palette — is subscription-scoped state, so it lives in an
// rx.Defer factory: each theme emission re-keys its palette in place, and the
// field itself is built exactly once per subscription.
func FieldLayer(win *app.Window, th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		field := NewField(win, winW, winH)
		return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
			field.SetColors(c)
			return field.Widget()
		})
	})
}

// HeroProps is the page's title block, stated once so a render made outside
// the running window puts the same words on the page as the window does.
var HeroProps = hero.Props{
	Eyebrow: "VIBRANTGIO",
	Title:   "Workbench",
	// The subtitle names no count of the apps: a number written here is one
	// nobody updates when an app is added.
	Subtitle: "Complete example apps built on mvu, components, theme, patterns and seen.",
}

// themed is one theme emission resolved to the token snapshot the view
// consumes, alongside the emission itself: the snapshot's fields are all
// rx.Of, so the theme-driven components built from components resolve
// synchronously via First().
type themed struct {
	components theme.Theme
	color      tokens.ColorTokens
	spacing    tokens.SpacingScale
	typ        tokens.Typography
	shaper     *text.Shaper // the theme's cached shaper (Typography.Shaper())
}

// ContentLayer is the page, held down past the native title-bar strip. It is
// the single modelObs consumer counted by modelObsConsumers in main.go.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	return underTitleBar(pageLayer(th, modelObs))
}

// underTitleBar pads the page down by the native title-bar strip's measured
// height on a full-size-content window. desktop.TopInset is read at frame
// time: it reports 0 until the window's first frame, in headless renders, and
// on every platform but macOS, so away from the treatment the wrapper is an
// exact no-op.
//
// The strip carries no fill of its own: the ground and the field beneath it
// are already full-bleed, so the region reaches the top edge without anything
// being painted twice. Only the page has to stay clear of the strip.
// desktop.CapTop also claims the strip for the window's own drag — without
// that claim the window could not be moved by its top edge.
func underTitleBar(pageObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(pageObs, func(w layout.Widget) layout.Widget {
		return desktop.CapTop(desktop.TopInset, w)
	})
}

// pageLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a widget. The launch buttons' clickables are
// subscription-scoped (rx.Defer) so press/hover/focus state survives the
// per-message view rebuilds; the hero is its own theme-driven component
// observable, rebuilt only when the theme changes.
func pageLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(
			rx.CombineLatest3(t.Color, t.Spacing, t.Typography),
			func(n rx.Tuple3[tokens.ColorTokens, tokens.SpacingScale, tokens.Typography]) themed {
				typ := n.Third
				return themed{components: t, color: n.First, spacing: n.Second, typ: typ, shaper: typ.Shaper()}
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

// View builds the page widget for one (theme, model) pair: a hero title block
// over a grid of app groups, the whole column centred on the field. A roster
// that outgrows a row wraps onto a further row rather than shrinking a cell.
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
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// appGroup is one launchable app as a patterns group: icon + name row, blurb,
// and a row holding the launch button and a status line. The group and
// button are theme-driven components built from the emission's static
// snapshot; text colours come off the Neutral ramp — 900 for the name, 700 for
// the low-contrast blurb.
//
// A group and not a card, because the roster is the page dividing itself:
// every app on the launcher is a peer of every other, and nothing here is
// singled out. The grid raises nothing off the page it stands on — the
// hairline at the page's own level is the whole of what chunks one app from
// the next.
func appGroup(tok themed, app App, click *widget.Clickable, status Status) layout.Widget {
	thObs := rx.Of(tok.components)

	icon, err := raster.Widget(app.Icon, IconSize, IconSize, raster.WithColors(tok.color.Primary))
	if err != nil {
		icon = func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}

	nameRow := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
			layout.Rigid(label(tok.shaper, app.Name, tok.typ.TitleMedium, tok.color.Ramps.Neutral.Step(900), 1)),
		)
	}
	blurb := label(tok.shaper, app.Blurb, tok.typ.BodySmall, tok.color.Ramps.Neutral.Step(700), 3)
	launchRow := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(launchButton(thObs, tok.shaper, app, click, status)),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
			layout.Flexed(1, statusLine(tok, status)),
		)
	}

	// The app's own name is already the first row, so the group takes no
	// label of its own: a second copy of it above the icon would name the
	// group twice.
	//
	// thObs is a static snapshot (rx.Of), so First() resolves synchronously.
	inner, _ := group.Group(thObs, group.Props{
		Content: []layout.Widget{nameRow, blurb, launchRow},
	}).First()
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(CellW), gtx.Dp(CellH)))
		return inner(gtx)
	}
}

// launchButton is the theme-driven components button wired to the caller-owned
// clickable — so press/hover/focus state survives the per-message view
// rebuilds — emitting Launch into the MVU loop on activation. While the app is
// starting or running it renders disabled and emits nothing; the reducer
// guards again anyway.
//
// It shapes with the shaper the rest of the cell uses rather than reaching for
// the theme's own, so one page has one shaper and a render made outside the
// window can say which.
func launchButton(th rx.Observable[theme.Theme], shaper *text.Shaper, app App, click *widget.Clickable, status Status) layout.Widget {
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
		Label:       txt,
		Description: "Launch " + app.Name,
		Disabled:    rx.Of(busy),
		Clickable:   click,
		Message:     Launch{Name: app.Name},
		Shaper:      shaper,
	}).First()
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(ButtonW)
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		return btn(gtx)
	}
}

// statusLine is the small caption beside the launch button: the failure
// detail in Error red, or a quiet lifecycle note.
func statusLine(tok themed, status Status) layout.Widget {
	txt, col := "", tok.color.Ramps.Neutral.Step(700)
	switch status.State {
	case Starting:
		txt = "compiling…"
	case Running:
		txt, col = "running", tok.color.Primary
	case Failed:
		txt, col = status.Detail, tok.color.Error
	}
	if txt == "" {
		return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	return label(tok.shaper, txt, tok.typ.LabelMedium, col, 2)
}

// label renders a colour-materialised label in one Typography role —
// typeface, weight, size and line height all come from the theme — capped at
// maxLines. It draws through theme/typeset so the role's LineHeight is the
// height of the line box, which widget.Label alone does not give a capped
// label.
func label(shaper *text.Shaper, txt string, style tokens.TextStyle, col color.NRGBA, maxLines int) layout.Widget {
	f := typeset.Font(style, font.Normal)
	wl := typeset.Label(style, maxLines)
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: col}.Add(gtx.Ops)
		material := m.Stop()
		gtx.Constraints.Min = image.Point{}
		return typeset.Layout(gtx, shaper, wl, f, unit.Sp(style.Size), txt, material)
	}
}
