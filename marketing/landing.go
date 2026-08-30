// The four marketing patterns — Hero, Feature, Pricing, Testimonial —
// composed into a scrolling column, with no navbar and no shell page.

package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/patterns/feature"
	"github.com/vibrantgio/patterns/hero"
	"github.com/vibrantgio/patterns/pricing"
	"github.com/vibrantgio/patterns/testimonial"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// sectionGapDp is the ordinary gap (hero to features). The stretch
// between the feature row and the testimonials is larger: pricing sits
// in that interval, so afterFeaturesGapDp and afterPricingGapDp are
// what actually separate those two bands.
const (
	sectionGapDp       float32 = 24
	afterFeaturesGapDp float32 = 40
	afterPricingGapDp  float32 = 48
	pageBottomInsetDp  float32 = 24
)

func gapAfter(i int) float32 {
	switch i {
	case 1:
		return afterFeaturesGapDp
	case 2:
		return afterPricingGapDp
	default:
		return sectionGapDp
	}
}

// contentMaxWidthDp matches sitedocs' landing column so the same
// patterns sit at the same measure.
const contentMaxWidthDp = 1100

// pricingSection is the layout.List index of the pricing block
// (hero, features, pricing, testimonials). Primary CTA scrolls here.
const pricingSection = 2

// pageLayer is the runtime page: the four pattern streams stacked in a
// shell-less vertical list, re-emitting on theme change. modelObs is consumed
// so AutoConnect in main stays balanced, though the model has no fields the
// page reads. Scroll position lives in this subscription.
func pageLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	return rx.Defer(func() rx.Observable[layout.Widget] {
		state := list.NewState()
		seePlans := func(layout.Context) {
			state.ScrollTo(pricingSection)
		}
		sections := rx.Map(
			rx.CombineLatest4(
				hero.Hero(th, heroContent(seePlans)),
				feature.Feature(th, featureContent()),
				pricing.Pricing(th, pricingContent()),
				testimonial.Testimonial(th, testimonialContent()),
			),
			func(n rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) []layout.Widget {
				return []layout.Widget{n.First, n.Second, n.Third, n.Fourth}
			},
		)
		colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
			return t.Color
		})
		return rx.Map(rx.CombineLatest3(sections, modelObs, colors),
			func(next rx.Tuple3[[]layout.Widget, Model, tokens.ColorTokens]) layout.Widget {
				return scrollingPage(next.First, state, next.Third)
			})
	})
}

// scrollingPage lays sections in a vertical list, clamping each child
// to contentMaxWidthDp and centering it. An overlay scrollbar (not
// Occupy) sits on the trailing edge so the 1100 dp column does not
// jump when the bar appears. The Background pin and the wireframe
// field live in layers behind this one, so the page does not paint a
// ground of its own.
func scrollingPage(sections []layout.Widget, state *list.State, colors tokens.ColorTokens) layout.Widget {
	children := make([]layout.Widget, 0, len(sections)+1)
	for i, s := range sections {
		if i < len(sections)-1 {
			children = append(children, stackSection(s, complayout.VSpacer(gapAfter(i))))
		} else {
			children = append(children, stackSection(s, complayout.VSpacer(pageBottomInsetDp)))
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if len(children) == 0 {
			return layout.Dimensions{Size: size}
		}
		contentW := size.X
		if px := gtx.Dp(contentMaxWidthDp); px > 0 && px < contentW {
			contentW = px
		}
		margin := (size.X - contentW) / 2
		list.LayoutScrollbar(gtx, state, scrollbar.FromTokens(colors), list.Overlay, children,
			func(gtx layout.Context, w layout.Widget) layout.Dimensions {
				if margin == 0 {
					return w(gtx)
				}
				cgtx := gtx
				cgtx.Constraints.Min.X = contentW
				cgtx.Constraints.Max.X = contentW
				off := op.Offset(image.Pt(margin, 0)).Push(gtx.Ops)
				dims := w(cgtx)
				off.Pop()
				return layout.Dimensions{
					Size:     image.Pt(size.X, dims.Size.Y),
					Baseline: dims.Baseline,
				}
			})
		return layout.Dimensions{Size: size}
	}
}

func stackSection(w, gap layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(w),
			layout.Rigid(gap),
		)
	}
}

// renderLanding composes the four patterns' Render() forms vertically with
// sectionGapDp gaps, with no scroll and no event handling. The runtime path is
// pageLayer.
//
// All four sections spend several type roles apiece, so each takes the whole
// Typography rather than a single style; hero and pricing size controls, so
// they also take the Density.
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
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 2*len(sections))
		for i, s := range sections {
			if i > 0 {
				children = append(children, layout.Rigid(complayout.VSpacer(gapAfter(i-1))))
			}
			children = append(children, layout.Rigid(s))
		}
		children = append(children, layout.Rigid(complayout.VSpacer(pageBottomInsetDp)))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
