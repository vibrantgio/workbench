// landing.go composes the four Cadence marketing patterns — Hero, Feature,
// Pricing, Testimonial — into the sitedocs Home page. The runtime entry
// point (landingMain) combines the four pattern observables into a single
// rx.Observable[layout.Widget] via CombineLatest4; the static entry point
// (renderLanding) is used by the golden test and skips all subscription work.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/feature"
	"github.com/vibrantgio/cadence/hero"
	"github.com/vibrantgio/cadence/pricing"
	"github.com/vibrantgio/cadence/testimonial"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// sectionGapDp is the vertical gap inserted between adjacent sections.
const sectionGapDp float32 = 24

// landingMain returns the Home page as an rx.Observable[layout.Widget]. Each
// pattern is constructed as an rx observable so its visuals re-emit on theme
// change; CombineLatest4 combines them into a single emission. A single
// layout.List captures scroll position across emissions.
func landingMain(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	gotoDocs func(gtx layout.Context),
) rx.Observable[layout.Widget] {
	heroObs := hero.Hero(th, heroContent(shaper, gotoDocs))
	featObs := feature.Feature(th, featureContent())
	priceObs := pricing.Pricing(th, pricingContent(shaper))
	testObs := testimonial.Testimonial(th, testimonialContent(shaper))

	list := &layout.List{Axis: layout.Vertical}
	gap := pllayout.VSpacer(sectionGapDp)

	combined := rx.CombineLatest4(heroObs, featObs, priceObs, testObs)
	return rx.Map(combined, func(t rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
		heroW, featW, priceW, testW := t.First, t.Second, t.Third, t.Fourth
		sections := []layout.Widget{heroW, featW, priceW, testW}
		return func(gtx layout.Context) layout.Dimensions {
			return list.Layout(gtx, len(sections), func(gtx layout.Context, i int) layout.Dimensions {
				w := sections[i]
				if i == 0 {
					return w(gtx)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(gap),
					layout.Rigid(w),
				)
			})
		}
	})
}

// renderLanding composes the four patterns' Render() forms vertically with
// sectionGapDp gaps. No scroll, no event handling — intended for the
// golden test and static demonstrations. The runtime path is landingMain.
func renderLanding(
	shaper *text.Shaper,
	hp hero.Props,
	fp feature.Props,
	pp pricing.Props,
	tp testimonial.Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	ts tokens.TypeScale,
) layout.Widget {
	sections := []layout.Widget{
		hero.Render(shaper, hp, colors, sp, rad, ts),
		feature.Render(shaper, fp, colors, sp, ts),
		pricing.Render(shaper, pp, colors, sp, rad, ts),
		testimonial.Render(shaper, tp, colors, sp, rad, ts),
	}
	gap := pllayout.VSpacer(sectionGapDp)
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 2*len(sections)-1)
		for i, s := range sections {
			if i > 0 {
				children = append(children, layout.Rigid(gap))
			}
			children = append(children, layout.Rigid(s))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
