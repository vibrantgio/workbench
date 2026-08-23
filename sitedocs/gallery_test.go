package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// galleryCanvasSize is the Gallery tab's content area at the app's default
// window: the first screen of the inventory column, which is what the
// goldens pin.
var galleryCanvasSize = image.Pt(1180, 760)

// TestGalleryTabGolden pins the Gallery tab's first screen in both
// schemes: the inventory's group banner, the palette sections and the top
// of the type ladder, laid out by the same column widget the app scrolls.
func TestGalleryTabGolden(t *testing.T) {
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
			w := renderGalleryTab(shaper, tc.colors, tokens.DefaultTypography)
			golden.Render(t, "gallery-tab-"+tc.name, galleryCanvasSize, scene(w, tc.bg))
		})
	}
}

// TestGalleryTabFollowsScheme is the standing hunt for a Gallery surface
// drawn from something other than the tokens it was handed: the same
// column in the two schemes must not come out the same bytes.
func TestGalleryTabFollowsScheme(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	a := golden.Capture(t, galleryCanvasSize, scene(renderGalleryTab(shaper, tokens.DefaultLight, tokens.DefaultTypography), bg))
	b := golden.Capture(t, galleryCanvasSize, scene(renderGalleryTab(shaper, tokens.DefaultDark, tokens.DefaultTypography), bg))
	if golden.PixelDiff(a, b) == 0 {
		t.Fatal("gallery tab renders identically in light and dark — the column is not following its tokens")
	}
}

// TestGalleryColumnIsTheWholeInventory guards the tab against quietly
// showing a slice of the catalogue: the column has a row for every row the
// published inventory builds — every group's banner and every section's
// heading and body, the closing line included.
func TestGalleryColumnIsTheWholeInventory(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.DefaultLight
	want := 1 // the closing PageEnd row
	for _, grp := range inv.Groups(c) {
		want += 1 + 2*len(grp.Sections)
	}
	if got := len(inv.Items(c)); got != want {
		t.Fatalf("inventory column has %d rows, want %d — a group or section is missing from the tab", got, want)
	}
}
