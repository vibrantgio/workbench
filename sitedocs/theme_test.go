package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// themeCanvasSize is the Theme tab's content area at the app's default
// window, which is what the goldens pin.
var themeCanvasSize = image.Pt(1180, 760)

// TestThemeTabGolden pins the Theme tab in both schemes: the ramps grid
// with its step numbers and pinned-base chips, and the picks board with
// the rule that chose each colour.
func TestThemeTabGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderThemeTab(shaper, tc.colors, tokens.DefaultTypography)
			golden.Render(t, "theme-tab-"+tc.name, themeCanvasSize, scene(w, tc.bg))
		})
	}
}

// TestThemeTabFollowsScheme is the standing hunt for a Theme surface
// drawn from something other than the tokens it was handed: the same
// section in the two schemes must not come out the same bytes.
func TestThemeTabFollowsScheme(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	a := golden.Capture(t, themeCanvasSize, scene(renderThemeTab(shaper, tokens.DefaultLight, tokens.DefaultTypography), bg))
	b := golden.Capture(t, themeCanvasSize, scene(renderThemeTab(shaper, tokens.DefaultDark, tokens.DefaultTypography), bg))
	if golden.PixelDiff(a, b) == 0 {
		t.Fatal("theme tab renders identically in light and dark — the section is not following its tokens")
	}
}

// TestPaletteSectionRowsIsTheRowCount asserts the stated row count
// against the rows themselves, the way the themer's own tests do.
func TestPaletteSectionRowsIsTheRowCount(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	if got := len(themePaletteRows(shaper, tokens.DefaultTypography, tokens.DefaultLight)); got != PaletteSectionRows {
		t.Fatalf("PaletteRows returns %d rows, PaletteSectionRows says %d", got, PaletteSectionRows)
	}
}

// TestTheGridMarksTheRungsThePicksTook keeps the copied section's two
// halves honest against each other: every rung a pick's rule names is
// claimed, in both schemes, and the claims carry real roles and steps.
func TestTheGridMarksTheRungsThePicksTook(t *testing.T) {
	for _, tc := range []struct {
		name string
		c, o tokens.ColorTokens
		dark bool
	}{
		{"light", tokens.DefaultLight, tokens.DefaultDark, false},
		{"dark", tokens.DefaultDark, tokens.DefaultLight, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups := paletteGroups(tc.c, tc.o, tc.dark)
			claims := rampClaims(groups)
			if len(claims) == 0 {
				t.Fatal("no pick claims any rung — the grid would carry no marks at all")
			}
			for claim := range claims {
				if claim.role == "" || claim.step < 100 || claim.step > 900 || claim.step%100 != 0 {
					t.Fatalf("claim %+v names no rung the grid has", claim)
				}
			}
		})
	}
}
