// landing.go composes the four Cadence marketing patterns — Hero, Feature,
// Pricing, Testimonial — into the Home page. The runtime entry point
// (homeShellLayer) mounts them as Sections of a cadence/shell StackedPage:
// the shell pins the full-width navbar, owns the scroll region, and
// re-emits whenever any section stream emits. The static entry point
// (renderLanding) is used by the golden test and skips all subscription
// work.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/feature"
	"github.com/vibrantgio/cadence/hero"
	"github.com/vibrantgio/cadence/pricing"
	"github.com/vibrantgio/cadence/shell"
	"github.com/vibrantgio/cadence/testimonial"
	"github.com/vibrantgio/mvu"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// sectionGapDp is the vertical gap inserted between adjacent sections.
const sectionGapDp float32 = 24

// homeShellLayer returns the Home page as a StackedPage shell: pinned
// full-width navbar, the marketing patterns as scrolling sections, and a
// theme-aware footer as the final section so it scrolls with the content.
func homeShellLayer(th rx.Observable[theme.Theme], shaper *text.Shaper) rx.Observable[layout.Widget] {
	gotoDocs := func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Page: pageDocsDefault}}.Add(gtx.Ops)
	}
	gotoAbout := func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Page: pageAbout}}.Add(gtx.Ops)
	}
	gap := rx.Of[layout.Widget](pllayout.VSpacer(sectionGapDp))
	return shell.Shell(th, shell.Props{
		Layout: shell.StackedPage,
		Navbar: navbarProps(th, shaper, pageHome),
		Sections: []rx.Observable[layout.Widget]{
			hero.Hero(th, heroContent(shaper, gotoDocs, gotoAbout)),
			gap,
			feature.Feature(th, featureContent()),
			gap,
			pricing.Pricing(th, pricingContent(shaper)),
			gap,
			testimonial.Testimonial(th, testimonialContent(shaper)),
			gap,
			footerSection(th, shaper),
		},
	})
}

// footerSection is the landing page's end-cap: a single muted line that
// scrolls with the content (StackedPage appends sections; it does not
// pin them to the viewport). Built as a section stream so it re-renders
// on theme change like every other section.
func footerSection(th rx.Observable[theme.Theme], shaper *text.Shaper) rx.Observable[layout.Widget] {
	type pair struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	combined := rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale]) pair {
		return pair{col: t.First, typ: t.Second}
	})
	return rx.Map(combined, func(p pair) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := pllayout.Inset(sectionGapDp)
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return paragraphWidget(shaper,
					"Vibrant Gio — built with Gio. MIT licensed.",
					p.col.OnSurfaceVariant, p.typ)(gtx)
			})
		}
	})
}

// renderLanding composes the four patterns' Render() forms vertically with
// sectionGapDp gaps. No scroll, no event handling — intended for the
// golden test and static demonstrations. The runtime path is homeShellLayer.
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
