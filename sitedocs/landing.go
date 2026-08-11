// landing.go composes the four Patterns marketing patterns — Hero, Feature,
// Pricing, Testimonial — into the Home page. The runtime entry point
// (homeShellLayer) mounts them as Sections of a patterns/shell StackedPage:
// the shell pins the full-width navbar, owns the scroll region, and
// re-emits whenever any section stream emits. The static entry point
// (renderLanding) is used by the golden test and skips all subscription
// work.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/patterns/feature"
	"github.com/vibrantgio/patterns/hero"
	"github.com/vibrantgio/patterns/pricing"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/testimonial"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// sectionGapDp is the vertical gap inserted between adjacent sections.
const sectionGapDp float32 = 24

// contentMaxWidthDp clamps the landing sections to a centered reading
// column; on wider windows the shell fills the side margins with the
// page background instead of stretching the sections edge to edge.
const contentMaxWidthDp = 1100

// homeShellLayer returns the Home page as a StackedPage shell: pinned
// full-width navbar, the marketing patterns as scrolling sections, and a
// theme-aware footer as the final section so it scrolls with the content.
func homeShellLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	gotoDocs := func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Page: pageDocsDefault}}.Add(gtx.Ops)
	}
	gotoAbout := func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Page: pageAbout}}.Add(gtx.Ops)
	}
	gap := rx.Of[layout.Widget](complayout.VSpacer(sectionGapDp))
	return shell.Shell(th, shell.Props{
		Layout:          shell.StackedPage,
		ContentMaxWidth: contentMaxWidthDp,
		Navbar:          navbarProps(mirrorTokens(th), pageHome),
		Sections: []rx.Observable[layout.Widget]{
			hero.Hero(th, heroContent(gotoDocs, gotoAbout)),
			gap,
			feature.Feature(th, featureContent()),
			gap,
			pricing.Pricing(th, pricingContent()),
			gap,
			testimonial.Testimonial(th, testimonialContent()),
			gap,
			footerSection(th),
		},
	})
}

// footerSection is the landing page's end-cap: a single muted line that
// scrolls with the content (StackedPage appends sections; it does not
// pin them to the viewport). Built as a section stream so it re-renders
// on theme change like every other section. The muted line sits in
// BodyMedium on the low-contrast Neutral 700 text step (replacing the
// deprecated OnSurfaceVariant alias), shaped with the theme's shaper.
func footerSection(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	combined := rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themeTokens {
		typ := t.Second
		return themeTokens{col: t.First, typ: typ, shaper: typ.Shaper()}
	})
	return rx.Map(combined, func(p themeTokens) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := complayout.Inset(sectionGapDp)
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return paragraphWidget(p.shaper,
					"Vibrant Gio — built with Gio. MIT licensed.",
					p.col.Ramps.Neutral.Step(700), p.typ.BodyMedium)(gtx)
			})
		}
	})
}

// renderLanding composes the four patterns' Render() forms vertically with
// sectionGapDp gaps. No scroll, no event handling — intended for the
// golden test and static demonstrations. The runtime path is homeShellLayer.
//
// All four sections spend several type roles apiece, so each takes the whole
// Typography rather than a single style; hero and pricing size controls, so
// they also take the Density. Pass tokens.DefaultTypography and
// tokens.Comfortable for the default desktop look.
func renderLanding(
	shaper *text.Shaper,
	hp hero.Props,
	fp feature.Props,
	pp pricing.Props,
	tp testimonial.Props,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	d tokens.Density,
) layout.Widget {
	sections := []layout.Widget{
		hero.Render(shaper, hp, colors, sp, rad, typo, d),
		feature.Render(shaper, fp, colors, sp, typo),
		pricing.Render(shaper, pp, colors, sp, rad, typo, d),
		testimonial.Render(shaper, tp, colors, sp, rad, typo),
	}
	gap := complayout.VSpacer(sectionGapDp)
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
