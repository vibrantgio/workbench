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
	"github.com/vibrantgio/cadence/card"
	"github.com/vibrantgio/cadence/hero"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/prism/button"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

// Static layout dimensions; these do not vary with the colour scheme.
const (
	CardW    unit.Dp = 300 // one app card
	CardH    unit.Dp = 190
	ButtonW  unit.Dp = 110 // fixed launch-button width, so labels don't resize it
	IconSize unit.Dp = 28
	RowGap   float32 = 16 // dp between cards, and between the two rows
	perRow           = 3  // cards per grid row
)

// buildLayers returns the layer-builder the spectrum window renders, back to
// front: the theme background fill, the animated seen triangle field, and the
// hero + launch-card content floating on top.
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

// FieldLayer is the animated seen triangle field. The Field (its scene,
// animation loop and palette) is subscription-scoped state, so it lives in an
// rx.Defer factory (llms.txt rule 2); each theme emission re-keys its palette
// in place — the field itself is built exactly once per subscription.
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

// themed is one theme emission resolved to the token snapshot the view
// consumes, alongside the emission itself: the snapshot's fields are all
// rx.Of, so the theme-driven components built from prism resolve
// synchronously via First().
type themed struct {
	prism   theme.Theme
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	typ     tokens.Typography
	shaper  *text.Shaper // the theme's cached shaper (Typography.Shaper())
}

// ContentLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a widget. This is the single modelObs consumer
// counted by modelObsConsumers in main.go. The launch buttons' clickables are
// subscription-scoped (rx.Defer) so press/hover/focus state survives the
// per-message view rebuilds; the hero is its own theme-driven component
// observable, rebuilt only when the theme changes.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(
			rx.CombineLatest3(t.Color, t.Spacing, t.Typography),
			func(n rx.Tuple3[tokens.ColorTokens, tokens.SpacingScale, tokens.Typography]) themed {
				typ := n.Third
				return themed{prism: t, color: n.First, spacing: n.Second, typ: typ, shaper: typ.Shaper()}
			},
		)
	})
	heroObs := hero.Hero(th, hero.Props{
		Eyebrow:  "VIBRANTGIO",
		Title:    "Workbench",
		Subtitle: "Six complete example apps built on mvu, prism, spectrum and cadence — floating on a live seen 3D field.",
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		clicks := make([]widget.Clickable, len(Apps))
		return rx.Map(rx.CombineLatest3(resolved, heroObs, modelObs),
			func(next rx.Tuple3[themed, layout.Widget, Model]) layout.Widget {
				return View(next.First, next.Second, clicks, next.Third)
			})
	})
}

// View builds the page widget for one (theme, model) pair: a hero title block
// over a two-row grid of app cards, the whole column centred on the field.
func View(tok themed, heroW layout.Widget, clicks []widget.Clickable, model Model) layout.Widget {
	cards := make([]layout.Widget, len(Apps))
	for i, app := range Apps {
		cards[i] = appCard(tok, app, &clicks[i], model.StatusOf(app.Name))
	}
	var rows []layout.Widget
	for i := 0; i < len(cards); i += perRow {
		rows = append(rows, cardRow(cards[i:min(i+perRow, len(cards))]))
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{layout.Rigid(heroW), layout.Rigid(pllayout.VSpacer(RowGap))}
			for i, row := range rows {
				if i > 0 {
					children = append(children, layout.Rigid(pllayout.VSpacer(RowGap)))
				}
				children = append(children, layout.Rigid(row))
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		return layout.Dimensions{Size: size}
	}
}

// cardRow lays out one row of fixed-size cards with RowGap gaps, centred.
func cardRow(cells []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var children []layout.FlexChild
		for i, cell := range cells {
			if i > 0 {
				children = append(children, layout.Rigid(pllayout.HSpacer(RowGap)))
			}
			children = append(children, layout.Rigid(cell))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// appCard is one launchable app as an elevated cadence card: icon + name
// header, blurb body, and a footer with the launch button and a status line.
// The card and button are the theme-driven components, built from the
// emission's static snapshot; text colours come off the Neutral ramp — 900
// for the name, 700 for the low-contrast blurb (ADR-007).
func appCard(tok themed, app App, click *widget.Clickable, status Status) layout.Widget {
	thObs := rx.Of(tok.prism)

	icon, err := raster.Widget(app.Icon, IconSize, IconSize, raster.WithColors(tok.color.Primary))
	if err != nil {
		icon = func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}

	header := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
			layout.Rigid(label(tok.shaper, app.Name, tok.typ.TitleMedium, tok.color.Ramps.Neutral.Step(900), 1)),
		)
	}
	body := label(tok.shaper, app.Blurb, tok.typ.BodySmall, tok.color.Ramps.Neutral.Step(700), 3)
	footer := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(launchButton(thObs, app, click, status)),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
			layout.Flexed(1, statusLine(tok, status)),
		)
	}

	// thObs is a static snapshot (rx.Of), so First() resolves synchronously.
	inner, _ := card.Card(thObs, card.Props{Header: header, Body: body, Footer: footer, Elevated: true}).First()
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(CardW), gtx.Dp(CardH)))
		return inner(gtx)
	}
}

// launchButton is the theme-driven prism button wired to the caller-owned
// clickable (so press/hover/focus state survives the per-message view
// rebuilds) and emits Launch into the MVU loop on activation. While the app
// is starting or running it renders disabled and emits nothing; the reducer
// guards again anyway.
func launchButton(th rx.Observable[theme.Theme], app App, click *widget.Clickable, status Status) layout.Widget {
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

// label renders a colour-materialised widget.Label in one Typography role —
// typeface, weight, size and line height all come from the theme — capped at
// maxLines.
func label(shaper *text.Shaper, txt string, style tokens.TextStyle, col color.NRGBA, maxLines int) layout.Widget {
	f := font.Font{Typeface: font.Typeface(style.Typeface)}
	if style.Weight != 0 {
		f.Weight = tokens.FontWeight(style.Weight)
	}
	wl := widget.Label{MaxLines: maxLines}
	if style.LineHeight != 0 {
		wl.LineHeight = unit.Sp(style.LineHeight)
		wl.LineHeightScale = 1
	}
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: col}.Add(gtx.Ops)
		material := m.Stop()
		gtx.Constraints.Min = image.Point{}
		return wl.Layout(gtx, shaper, f, unit.Sp(style.Size), txt, material)
	}
}
