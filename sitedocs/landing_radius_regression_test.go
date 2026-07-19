package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/cadence/hero"
	"github.com/vibrantgio/cadence/pricing"
	"github.com/vibrantgio/prism/tokens"
)

// TestPricingHighlightedTierDoesNotFloodCanvas exercises the Pro (Highlighted)
// tier with the real production radius scale (tokens.Radius, where Full is
// 9999 dp). The "Popular" chip's pill uses radius.Full; clip.RRect does not
// clamp corner radii to the rect, so an unclamped Full radius would spray
// Primary paint across the entire canvas — invisible to the structural
// golden, which uses a zero-valued radius scale.
//
// The test renders the Pro tier in a small canvas, then samples a pixel
// well outside the card's bounds. With the radius clamp in place, that
// pixel must be the canvas background (Surface). Without the clamp it
// renders Primary.
func TestPricingHighlightedTierDoesNotFloodCanvas(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	size := image.Pt(400, 500)
	colors := tokens.DefaultDark
	bg := colors.Surface

	full := pricingContent(shaper)
	var pro pricing.Tier
	for _, tt := range full.Tiers {
		if tt.Highlighted {
			pro = tt
			break
		}
	}
	if pro.Name == "" {
		t.Fatalf("no Highlighted tier in pricingContent")
	}

	single := pricing.Props{Shaper: shaper, Tiers: []pricing.Tier{pro}}
	w := pricing.Render(shaper, single, colors, tokens.Spacing, tokens.Radius, tokens.DefaultTypeScale)

	img := renderToImage(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	})

	// Sample a pixel far below the card — must be Surface.
	sx, sy := 20, size.Y-50
	off := sy*img.Stride + sx*4
	got := color.NRGBA{R: img.Pix[off], G: img.Pix[off+1], B: img.Pix[off+2], A: img.Pix[off+3]}
	if got != bg {
		t.Fatalf("background pixel at (%d,%d) = %#v, want Surface %#v — the Pro tier flooded outside its bounds (likely an unclamped corner radius on the Popular chip)", sx, sy, got, bg)
	}
}

// TestHeroEyebrowDoesNotFloodCanvas exercises the hero's eyebrow pill —
// which shares the popular-chip pattern of an unclamped `tokens.Radius.Full`
// passed to `clip.RRect`. The hero's bug is visually masked because the
// pill uses `tintColor(Primary, Surface)` (≈12%/88% blend), which is
// almost indistinguishable from Surface; but the pixel value is not
// exactly Surface, so the regression is still detectable.
//
// Pre-fix: a pixel well below the eyebrow band paints as
// tintColor(Primary, Surface) instead of Surface. Post-fix: Surface.
func TestHeroEyebrowDoesNotFloodCanvas(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	size := image.Pt(400, 600)
	colors := tokens.DefaultDark
	bg := colors.Surface

	hp := heroContent(shaper, func(_ layout.Context) {}, func(_ layout.Context) {})
	w := hero.Render(shaper, hp, colors, tokens.Spacing, tokens.Radius, tokens.DefaultTypeScale)

	img := renderToImage(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	})

	// Sample a pixel far below the eyebrow band — must be Surface.
	sx, sy := 20, size.Y-50
	off := sy*img.Stride + sx*4
	got := color.NRGBA{R: img.Pix[off], G: img.Pix[off+1], B: img.Pix[off+2], A: img.Pix[off+3]}
	if got != bg {
		t.Fatalf("background pixel at (%d,%d) = %#v, want Surface %#v — the hero eyebrow pill flooded outside its bounds (likely an unclamped corner radius)", sx, sy, got, bg)
	}
}

func renderToImage(t *testing.T, size image.Point, w layout.Widget) *image.RGBA {
	t.Helper()
	hw, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("headless: %v", err)
		return nil
	}
	defer hw.Release()

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)
	if err := hw.Frame(&ops); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := hw.Screenshot(img); err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	return img
}
