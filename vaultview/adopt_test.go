package main

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	"github.com/vibrantgio/theme/tokens"
)

// harbourRed is the kept brand these tests adopt: nothing like the default
// seed, so a surface that failed to follow it is visible as itself.
var harbourRed = color.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}

// fixedAppearance is a desktop that always reports the same thing, so what
// these tests assert cannot depend on the machine they run on.
type fixedAppearance struct{ a specsystem.Appearance }

func (f fixedAppearance) Read() (specsystem.Appearance, error) { return f.a, nil }

// TestAKeptBrandDressesTheWholeWindow is the adoption proof. It builds the
// theme stream from exactly the expression the application builds its own
// from — a kept brand's options over the live bridge — renders the whole
// window in what that stream emits, and requires the default seed's accent
// to be gone from every pixel: a window that adopts a brand adopts it
// everywhere, or the adoption is a decoration on one panel.
//
// Both sides are checked because a kept brand pins a pair, not a colour,
// and the desktop still chooses between them.
func TestAKeptBrandDressesTheWholeWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := brand.SaveTo(path, brand.Brand{Seed: harbourRed, Source: "harbour.jpg"}); err != nil {
		t.Fatalf("keep: %v", err)
	}
	opts := brand.KeptFrom(path).Options()

	for _, tc := range []struct {
		name     string
		desktop  specsystem.Appearance
		fallback tokens.ColorTokens
	}{
		{"light", specsystem.Appearance{}, tokens.DefaultLight},
		{"dark", specsystem.Appearance{Dark: true}, tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th, err := specsystem.FromSourceTheme(fixedAppearance{tc.desktop}, time.Hour, opts...).First()
			if err != nil {
				t.Fatalf("theme: %v", err)
			}
			adopted, err := th.Color.First()
			if err != nil {
				t.Fatalf("colours: %v", err)
			}
			if adopted == tc.fallback {
				t.Fatal("the stream emitted the default palette with a brand kept")
			}

			before := window(t, tc.fallback)
			after := window(t, adopted)
			if pixels(before, tc.fallback.Primary) == 0 {
				t.Fatal("this window shows none of its accent, so it cannot show that it changed")
			}
			if n := pixels(after, tc.fallback.Primary); n != 0 {
				t.Errorf("%d pixels are still the default seed's accent while a brand is kept", n)
			}
			if pixels(after, adopted.Primary) == 0 {
				t.Error("the kept brand's accent is nowhere in the window")
			}
		})
	}
}

// TestWithNothingKeptTheWindowIsTheOneItAlwaysWas: adoption is optional, and
// a machine that never chose a brand must render what it always rendered.
func TestWithNothingKeptTheWindowIsTheOneItAlwaysWas(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "theme.json")
	th, err := specsystem.FromSourceTheme(fixedAppearance{}, time.Hour, brand.KeptFrom(absent).Options()...).First()
	if err != nil {
		t.Fatalf("theme: %v", err)
	}
	colors, err := th.Color.First()
	if err != nil {
		t.Fatalf("colours: %v", err)
	}
	if colors != tokens.DefaultLight {
		t.Fatal("with nothing kept the stream did not emit the default palette")
	}
	if golden.PixelDiff(window(t, colors), window(t, tokens.DefaultLight)) != 0 {
		t.Error("with nothing kept the window is not the one it always was")
	}
}

// window renders the whole vault window in one palette, through the same
// composition the window goldens record. It captures rather than stores:
// what is asserted here is which colours reach which surfaces, and the
// stored goldens stay on the canonical palette because adoption happens at
// runtime and is not baked into the application.
func window(t *testing.T, colors tokens.ColorTokens) *image.RGBA {
	t.Helper()
	w, _ := renderWindow(tokens.DefaultTypography.DeterministicShaper(), goldenModel(), colors,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
	return golden.Capture(t, windowCanvasSize, scene(w, colors.Background))
}

// pixels counts how many pixels of img are exactly c, alpha ignored: a
// colour that survives compositing unblended is a surface painted in it.
func pixels(img *image.RGBA, c color.NRGBA) int {
	if img == nil {
		return 0
	}
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px := img.RGBAAt(x, y)
			if px.R == c.R && px.G == c.G && px.B == c.B {
				n++
			}
		}
	}
	return n
}
