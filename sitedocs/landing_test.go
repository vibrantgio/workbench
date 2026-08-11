package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/feature"
	"github.com/vibrantgio/cadence/hero"
	"github.com/vibrantgio/cadence/pricing"
	"github.com/vibrantgio/cadence/testimonial"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	landingCanvasW = 960
	landingCanvasH = 1100
)

var (
	landingCanvasSize = image.Pt(landingCanvasW, landingCanvasH)
	// Sharp corner radius keeps the goldens deterministic — anti-aliased
	// rounded corners and the eyebrow / chip Full radii vary slightly
	// between GPU contexts, breaking pixel-exact diffs. The pattern
	// goldens upstream do the same.
	sharpRadius = tokens.RadiusScale{}
)

// TestLandingGolden records or diffs the home-page composition in light
// and dark themes. Text labels in the patterns are intentionally blank /
// single-space; structural variations (hero CTA pair, feature row, pricing
// "Popular" border, testimonial card chrome) drive the visual difference.
// The runtime path in homeShellLayer uses landing_content.go for real copy.
func TestLandingGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	hp := structuralHeroProps(shaper)
	fp := structuralFeatureProps(shaper)
	pp := structuralPricingProps(shaper)
	tp := structuralTestimonialProps(shaper)

	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light-home", tokens.DefaultLight, lightBG},
		{"dark-home", tokens.DefaultDark, darkBG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderLanding(shaper, hp, fp, pp, tp, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
			golden.Render(t, tc.name, landingCanvasSize, scene(w, tc.bg))
		})
	}
}

// TestLandingLightDarkDiffer confirms swapping the colour token set
// changes the rendered output of the home page composition.
func TestLandingLightDarkDiffer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	hp := structuralHeroProps(shaper)
	fp := structuralFeatureProps(shaper)
	pp := structuralPricingProps(shaper)
	tp := structuralTestimonialProps(shaper)

	light := renderLanding(shaper, hp, fp, pp, tp, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
	dark := renderLanding(shaper, hp, fp, pp, tp, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
	a := golden.Capture(t, landingCanvasSize, scene(light, bg))
	b := golden.Capture(t, landingCanvasSize, scene(dark, bg))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("light and dark landing render identically; expected colour differences across the four sections")
	}
}

// TestHomeShellLayerConstructs verifies that the runtime composition —
// the StackedPage shell whose Sections are the four pattern observables
// plus the footer — wires up and emits a usable widget. The
// rx.Of(theme.Default()) source delivers values synchronously, so the
// combined emission arrives before the test collects.
func TestHomeShellLayerConstructs(t *testing.T) {
	obs := homeShellLayer(rx.Of(theme.Default()))
	w, err := collectOne(obs)
	if err != nil {
		t.Fatalf("homeShellLayer subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("homeShellLayer produced no widget")
	}
	// Drive one frame at the runtime canvas size so the widget executes
	// its layout path; failure to compose returns either a panic or zero
	// dims.
	dims := drawOnce(t, landingCanvasSize, w)
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Errorf("homeShellLayer widget produced zero dimensions: %v", dims)
	}
}

// Routing is now exercised by TestDocsShellLayerReEmitsOnModelChange in
// sitedocs_test.go, which asserts the shell layer re-emits on a model change
// (the same-frame repaint contract) rather than poking a standalone router.

// ---- structural prop builders -------------------------------------------

// structuralHeroProps returns a hero.Props with empty/space text labels.
// Text-bearing fields collapse to no visible pixels so the golden depends
// on structure (eyebrow pill, dual-CTA row) rather than font rasterisation.
func structuralHeroProps(shaper *text.Shaper) hero.Props {
	return hero.Props{
		Eyebrow:      " ",
		PrimaryCTA:   &hero.CTA{Label: ""},
		SecondaryCTA: &hero.CTA{Label: ""},
		Shaper:       shaper,
	}
}

func structuralFeatureProps(shaper *text.Shaper) feature.Props {
	// iconFill is the same structural stand-in feature_test.go uses: a
	// solid-coloured rectangle filling its cell so the grid has visible
	// mass in the goldens without depending on a vector asset.
	iconFill := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, color.NRGBA{R: 60, G: 110, B: 200, A: 255}, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
	item := feature.Item{Icon: iconFill}
	return feature.Props{
		Columns: 3,
		Shaper:  shaper,
		Items:   []feature.Item{item, item, item},
	}
}

func structuralPricingProps(shaper *text.Shaper) pricing.Props {
	tier := func(highlighted bool) pricing.Tier {
		return pricing.Tier{
			Features:    []string{"", "", ""},
			CTA:         &pricing.CTA{Label: ""},
			Highlighted: highlighted,
		}
	}
	return pricing.Props{
		Shaper: shaper,
		Tiers:  []pricing.Tier{tier(false), tier(true), tier(false)},
	}
}

func structuralTestimonialProps(shaper *text.Shaper) testimonial.Props {
	item := testimonial.Item{}
	return testimonial.Props{
		Variant: testimonial.Grid,
		Shaper:  shaper,
		Items:   []testimonial.Item{item, item, item},
	}
}

// ---- headless test helpers ---------------------------------------------

func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

func drawOnce(t *testing.T, size image.Point, w layout.Widget) layout.Dimensions {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	return w(gtx)
}
