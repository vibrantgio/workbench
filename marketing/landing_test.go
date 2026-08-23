package main

import (
	"context"
	"image"
	"image/color"
	"testing"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/feature"
	"github.com/vibrantgio/patterns/hero"
	"github.com/vibrantgio/patterns/pricing"
	"github.com/vibrantgio/patterns/testimonial"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

const (
	pageCanvasW = 960
	pageCanvasH = 1100
)

var (
	pageCanvasSize = image.Pt(pageCanvasW, pageCanvasH)
	// Sharp corner radius keeps the goldens deterministic — anti-aliased
	// rounded corners and the eyebrow / chip Full radii vary slightly
	// between GPU contexts, breaking pixel-exact diffs. The pattern
	// goldens upstream do the same.
	sharpRadius = tokens.RadiusScale{}
)

// TestPageGolden records or diffs the page composition in light and dark
// themes: Background pin, rest-pose wireframe field, then the landing
// column. Text labels in the patterns are intentionally blank / single-
// space; structural variations (hero CTA pair, feature row, pricing
// "Popular" border, testimonial card chrome) drive the visual difference.
// The runtime path in pageLayer uses landing_content.go for real copy.
func TestPageGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()

	hp := structuralHeroProps(shaper)
	fp := structuralFeatureProps(shaper)
	pp := structuralPricingProps(shaper)
	tp := structuralTestimonialProps(shaper)

	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light-page", tokens.DefaultLight},
		{"dark-page", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderLanding(shaper, hp, fp, pp, tp, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
			golden.Render(t, tc.name, pageCanvasSize, scene(w, tc.colors))
		})
	}
}

// TestPageLightDarkDiffer confirms swapping the colour token set
// changes the rendered output of the page composition.
func TestPageLightDarkDiffer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()

	hp := structuralHeroProps(shaper)
	fp := structuralFeatureProps(shaper)
	pp := structuralPricingProps(shaper)
	tp := structuralTestimonialProps(shaper)

	light := renderLanding(shaper, hp, fp, pp, tp, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
	dark := renderLanding(shaper, hp, fp, pp, tp, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
	a := golden.Capture(t, pageCanvasSize, scene(light, tokens.DefaultLight))
	b := golden.Capture(t, pageCanvasSize, scene(dark, tokens.DefaultDark))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("light and dark page render identically; expected colour differences across the four sections")
	}
}

// TestContentLayerConstructs verifies that the runtime composition —
// the four pattern observables in a scrolling column — wires up and
// emits a usable widget.
func TestContentLayerConstructs(t *testing.T) {
	obs := ContentLayer(rx.Of(theme.Default()), rx.Of(Model{}))
	w, err := collectOne(obs)
	if err != nil {
		t.Fatalf("ContentLayer subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("ContentLayer produced no widget")
	}
	dims := drawOnce(t, pageCanvasSize, w)
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Errorf("ContentLayer widget produced zero dimensions: %v", dims)
	}
}

func TestSimpleAppsCopy(t *testing.T) {
	hp := heroContent(nil)
	if hp.Eyebrow != "simpleapps.eu · coming soon" {
		t.Errorf("hero eyebrow = %q", hp.Eyebrow)
	}
	if hp.Title != "SimpleApps" {
		t.Errorf("hero title = %q", hp.Title)
	}
	if hp.Subtitle != "Tools for knowing where a sentence came from. Provenance, authenticity and source custody — so work still stands when anyone can generate a paragraph." {
		t.Errorf("hero subtitle = %q", hp.Subtitle)
	}
	if hp.PrimaryCTA == nil || hp.PrimaryCTA.Label != "See plans" {
		t.Errorf("hero primary CTA = %+v", hp.PrimaryCTA)
	}
	if hp.SecondaryCTA == nil || hp.SecondaryCTA.Label != "Learn more" || hp.SecondaryCTA.OnClick != nil {
		t.Errorf("hero secondary CTA = %+v", hp.SecondaryCTA)
	}

	fp := featureContent()
	if fp.Columns != 3 || len(fp.Items) != 3 {
		t.Fatalf("features = %d columns / %d items", fp.Columns, len(fp.Items))
	}
	wantFeat := [][2]string{
		{"Provenance", "Every claim keeps the page, the quote and the date it came from. A rewrite does not drop the trail."},
		{"Authenticity", "Mark what you wrote, what a model drafted, and what a source attested. The distinction travels with the file."},
		{"Custody", "Sources stay on your device. Export a signed bundle, not a screenshot of a chat. No account, no feed."},
	}
	for i, want := range wantFeat {
		if fp.Items[i].Title != want[0] || fp.Items[i].Body != want[1] {
			t.Errorf("feature %d = %q / %q", i, fp.Items[i].Title, fp.Items[i].Body)
		}
	}

	pp := pricingContent()
	if len(pp.Tiers) != 3 {
		t.Fatalf("pricing tiers = %d", len(pp.Tiers))
	}
	if pp.Tiers[0].Name != "Free" || pp.Tiers[0].Price != "€0" || pp.Tiers[0].Cadence != "once" {
		t.Errorf("free tier = %+v", pp.Tiers[0])
	}
	if pp.Tiers[1].Name != "Pro" || pp.Tiers[1].Price != "€29" || !pp.Tiers[1].Highlighted {
		t.Errorf("pro tier = %+v", pp.Tiers[1])
	}
	if pp.Tiers[2].Name != "Studio" || pp.Tiers[2].Price != "€79" {
		t.Errorf("studio tier = %+v", pp.Tiers[2])
	}
	for i, tier := range pp.Tiers {
		if tier.CTA == nil || tier.CTA.OnClick != nil {
			t.Errorf("pricing CTA %d = %+v; want a visual-only button", i, tier.CTA)
		}
	}

	tp := testimonialContent()
	if tp.Variant != testimonial.Grid || len(tp.Items) != 3 {
		t.Fatalf("testimonials variant=%v n=%d", tp.Variant, len(tp.Items))
	}
	if tp.Items[0].AuthorName != "Kees de Wit" || tp.Items[1].AuthorName != "Amira Haddad" || tp.Items[2].AuthorName != "Jonah Eller" {
		t.Errorf("testimonial authors = %q, %q, %q", tp.Items[0].AuthorName, tp.Items[1].AuthorName, tp.Items[2].AuthorName)
	}
}

// ---- structural prop builders -------------------------------------------

func structuralHeroProps(shaper *text.Shaper) hero.Props {
	return hero.Props{
		Eyebrow:      " ",
		PrimaryCTA:   &hero.CTA{Label: ""},
		SecondaryCTA: &hero.CTA{Label: ""},
		Shaper:       shaper,
	}
}

func structuralFeatureProps(shaper *text.Shaper) feature.Props {
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

// scene is the golden composition: Background pin, a rest-pose
// wireframe field, then the landing column. The live field is
// clock-driven; goldens store the un-noised mesh so the frame is
// one frame.
func scene(w layout.Widget, colors tokens.ColorTokens) layout.Widget {
	field := newField(new(app.Window), unit.Dp(pageCanvasW), unit.Dp(pageCanvasH))
	field.SetColors(colors)
	field.applyPending()
	back := backdrop.Widget(colors.Background)
	return func(gtx layout.Context) layout.Dimensions {
		back(gtx)
		field.Widget()(gtx)
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

func collectOne(obs rx.Observable[layout.Widget]) (layout.Widget, error) {
	var got layout.Widget
	err := obs.Subscribe(context.Background(), func(v layout.Widget, _ error, done bool) {
		if !done && got == nil {
			got = v
		}
	}).Wait()
	return got, err
}
