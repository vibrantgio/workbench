package main

import (
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
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
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
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

// themed is one theme emission resolved to the concrete token snapshot the
// static Render forms consume.
type themed struct {
	color   tokens.ColorTokens
	spacing tokens.SpacingScale
	radius  tokens.RadiusScale
	typ     tokens.TypeScale
}

// ContentLayer renders the page: the latest theme snapshot combined with the
// latest Model, mapped to a widget. This is the single modelObs consumer
// counted by modelObsConsumers in main.go. The launch buttons' clickables are
// subscription-scoped (rx.Defer) so press/hover/focus state survives the
// per-message view rebuilds.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	resolved := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(
			rx.CombineLatest4(t.Color, t.Spacing, t.Radius, t.Type),
			func(n rx.Tuple4[tokens.ColorTokens, tokens.SpacingScale, tokens.RadiusScale, tokens.TypeScale]) themed {
				return themed{color: n.First, spacing: n.Second, radius: n.Third, typ: n.Fourth}
			},
		)
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		shaper := text.NewShaper(text.WithCollection(style.FontFaces()))
		clicks := make([]widget.Clickable, len(Apps))
		return rx.Map(rx.CombineLatest2(resolved, modelObs),
			func(next rx.Tuple2[themed, Model]) layout.Widget {
				return View(shaper, next.First, clicks, next.Second)
			})
	})
}

// View builds the page widget for one (theme, model) pair: a hero title block
// over a two-row grid of app cards, the whole column centred on the field.
func View(shaper *text.Shaper, tok themed, clicks []widget.Clickable, model Model) layout.Widget {
	heroW := hero.Render(shaper, hero.Props{
		Eyebrow:  "VIBRANTGIO",
		Title:    "Workbench",
		Subtitle: "Five complete example apps built on mvu, prism, spectrum and cadence — floating on a live seen 3D field.",
		Shaper:   shaper,
	}, tok.color, tok.spacing, tok.radius, tok.typ)

	cards := make([]layout.Widget, len(Apps))
	for i, app := range Apps {
		cards[i] = appCard(shaper, tok, app, &clicks[i], model.StatusOf(app.Name))
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
func appCard(shaper *text.Shaper, tok themed, app App, click *widget.Clickable, status Status) layout.Widget {
	icon, err := raster.Widget(app.Icon, IconSize, IconSize, raster.WithColors(tok.color.Primary))
	if err != nil {
		icon = func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}

	header := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
			layout.Rigid(label(shaper, app.Name, tok.typ.TitleMedium, font.Font{Weight: font.SemiBold}, tok.color.OnSurface, 1)),
		)
	}
	body := label(shaper, app.Blurb, tok.typ.BodySmall, font.Font{}, tok.color.OnSurfaceVariant, 3)
	footer := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(launchButton(shaper, tok, app, click, status)),
			layout.Rigid(pllayout.HSpacer(tok.spacing.S3)),
			layout.Flexed(1, statusLine(shaper, tok, status)),
		)
	}

	inner := card.Render(card.Props{Header: header, Body: body, Footer: footer, Elevated: true},
		tok.color, tok.spacing, tok.radius)
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(CardW), gtx.Dp(CardH)))
		return inner(gtx)
	}
}

// launchButton renders the prism button visual driven by the caller-owned
// clickable (the hero-CTA wiring pattern) and emits Launch into the MVU loop
// on activation. While the app is starting or running it renders disabled and
// emits nothing; the reducer guards again anyway.
func launchButton(shaper *text.Shaper, tok themed, app App, click *widget.Clickable, status Status) layout.Widget {
	busy := status.State == Starting || status.State == Running
	text := "Launch"
	switch status.State {
	case Starting:
		text = "Starting"
	case Running:
		text = "Running"
	case Failed:
		text = "Relaunch"
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(ButtonW)
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		if !busy && click.Clicked(gtx) {
			mvu.MessageOp{Message: Launch{Name: app.Name}}.Add(gtx.Ops)
		}
		rendered := button.Render(shaper, text, tok.color, tok.spacing, tok.radius, tok.typ, button.RenderState{
			Hovered:  !busy && click.Hovered(),
			Pressed:  !busy && click.Pressed(),
			Disabled: busy,
		})
		if busy {
			return rendered(gtx)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Launch " + app.Name).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return rendered(gtx)
		})
	}
}

// statusLine is the small caption beside the launch button: the failure
// detail in Error red, or a quiet lifecycle note.
func statusLine(shaper *text.Shaper, tok themed, status Status) layout.Widget {
	txt, col := "", tok.color.OnSurfaceVariant
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
	return label(shaper, txt, tok.typ.LabelMedium, font.Font{}, col, 2)
}

// label renders a colour-materialised widget.Label capped at maxLines.
func label(shaper *text.Shaper, txt string, sizeDp float32, fnt font.Font, col color.NRGBA, maxLines int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		paint.ColorOp{Color: col}.Add(gtx.Ops)
		material := m.Stop()
		gtx.Constraints.Min = image.Point{}
		wl := widget.Label{MaxLines: maxLines}
		return wl.Layout(gtx, shaper, fnt, unit.Sp(sizeDp), txt, material)
	}
}
