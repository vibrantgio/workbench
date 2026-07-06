package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
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
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

var goldenUpdate = flag.Bool("golden.update", false, "overwrite golden images with current output")

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
// The runtime path in landingMain uses landing_content.go for real copy.
func TestLandingGolden(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	hp := structuralHeroProps(shaper)
	fp := structuralFeatureProps()
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
			w := renderLanding(shaper, hp, fp, pp, tp, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypeScale)
			renderGolden(t, tc.name, landingCanvasSize, scene(w, tc.bg))
		})
	}
}

// TestLandingLightDarkDiffer confirms swapping the colour token set
// changes the rendered output of the home page composition.
func TestLandingLightDarkDiffer(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	hp := structuralHeroProps(shaper)
	fp := structuralFeatureProps()
	pp := structuralPricingProps(shaper)
	tp := structuralTestimonialProps(shaper)

	light := renderLanding(shaper, hp, fp, pp, tp, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypeScale)
	dark := renderLanding(shaper, hp, fp, pp, tp, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypeScale)
	a := capture(t, landingCanvasSize, scene(light, bg))
	b := capture(t, landingCanvasSize, scene(dark, bg))
	if a == nil || b == nil {
		return
	}
	if n := pixelDiff(a, b); n == 0 {
		t.Error("light and dark landing render identically; expected colour differences across the four sections")
	}
}

// TestLandingMainConstructs verifies that the runtime composition wires
// the four pattern observables and emits a usable widget. The
// rx.Of(theme.Default()) source delivers values synchronously, so the
// CombineLatest4 emission arrives before the test collects.
func TestLandingMainConstructs(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	calls := 0
	gotoDocs := func(_ layout.Context) { calls++ }
	obs := landingMain(rx.Of(theme.Default()), shaper, gotoDocs)
	w, err := collectOne(obs)
	if err != nil {
		t.Fatalf("landingMain subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("landingMain produced no widget")
	}
	// Drive one frame at the runtime canvas size so the widget executes
	// its layout path; failure to compose returns either a panic or zero
	// dims.
	dims := drawOnce(t, landingCanvasSize, w)
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Errorf("landingMain widget produced zero dimensions: %v", dims)
	}
	// gotoDocs is only invoked on hero CTA click; no input is driven here
	// so it must remain at zero.
	if calls != 0 {
		t.Errorf("gotoDocs fired during static layout: %d times", calls)
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

func structuralFeatureProps() feature.Props {
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

// ---- golden harness (inlined; prism/internal/golden is not importable) --

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

func capture(t *testing.T, size image.Point, draw layout.Widget) *image.RGBA {
	t.Helper()
	hw, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("headless rendering not supported: %v", err)
		return nil
	}
	defer hw.Release()

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	draw(gtx)
	if err := hw.Frame(&ops); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := hw.Screenshot(img); err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	return img
}

func renderGolden(t *testing.T, name string, size image.Point, draw layout.Widget) {
	t.Helper()
	img := capture(t, size, draw)
	if img == nil {
		return
	}
	path := filepath.Join("testdata", "golden", name+".png")

	if *goldenUpdate {
		if err := saveImage(path, img); err != nil {
			t.Fatalf("save %s: %v", path, err)
		}
		return
	}
	stored, err := loadImage(path)
	if os.IsNotExist(err) {
		t.Fatalf("%s not found; run go test -golden.update to create", path)
		return
	}
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
		return
	}
	if n := pixelDiff(stored, img); n > 0 {
		actualPath := strings.TrimSuffix(path, ".png") + ".actual.png"
		_ = saveImage(actualPath, img)
		t.Fatalf("%q: %d pixel(s) differ (actual saved to %s)", name, n, actualPath)
	}
}

func pixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	bounds := a.Bounds()
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := (y-bounds.Min.Y)*a.Stride + (x-bounds.Min.X)*4
			if a.Pix[off] != b.Pix[off] ||
				a.Pix[off+1] != b.Pix[off+1] ||
				a.Pix[off+2] != b.Pix[off+2] ||
				a.Pix[off+3] != b.Pix[off+3] {
				n++
			}
		}
	}
	return n
}

func saveImage(path string, img *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	nrgba := &image.NRGBA{Pix: img.Pix, Stride: img.Stride, Rect: img.Rect}
	return png.Encode(f, nrgba)
}

func loadImage(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	switch v := decoded.(type) {
	case *image.RGBA:
		return v, nil
	case *image.NRGBA:
		return &image.RGBA{Pix: v.Pix, Stride: v.Stride, Rect: v.Rect}, nil
	default:
		bounds := decoded.Bounds()
		rgba := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				rgba.Set(x, y, decoded.At(x, y))
			}
		}
		return rgba, nil
	}
}
