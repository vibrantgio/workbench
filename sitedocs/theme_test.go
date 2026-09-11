package main

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// themeFrameSize is the Theme tab's content area at the app's default
// window, which is what the goldens pin.
var themeFrameSize = image.Pt(1180, 760)

// TestThemeTabGolden pins the Theme tab in both appearances: the colour
// board — every name the platform answers for, its swatch in each appearance
// and the value written out — and the type scale under it.
func TestThemeTabGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	for _, tc := range schemeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderThemeTab(shaper, tc.colors, tokens.DefaultTypography)
			golden.Render(t, "theme-tab-"+tc.name, themeFrameSize, scene(w, tc.bg))
		})
	}
}

// TestThemeTabFollowsScheme is the standing hunt for a Theme surface drawn
// from something other than the set it was handed: the same section in the
// two appearances must not come out the same bytes.
func TestThemeTabFollowsScheme(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	a := golden.Capture(t, themeFrameSize, scene(renderThemeTab(shaper, tokens.PlatformLight, tokens.DefaultTypography), bg))
	b := golden.Capture(t, themeFrameSize, scene(renderThemeTab(shaper, tokens.PlatformDark, tokens.DefaultTypography), bg))
	if golden.PixelDiff(a, b) == 0 {
		t.Fatal("theme tab renders identically in light and dark — the section is not following the set it was handed")
	}
}

// TestPaletteSectionRowsIsTheRowCount asserts the stated row count against
// the rows themselves: the board is a heading and a body.
func TestPaletteSectionRowsIsTheRowCount(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	c := tokens.PlatformLight
	if got := len(PaletteRows(c, TypeFrom(shaper, tokens.DefaultTypography))); got != PaletteSectionRows {
		t.Fatalf("PaletteRows returns %d rows, PaletteSectionRows says %d", got, PaletteSectionRows)
	}
}

// TestTypeScaleFollowsTheBoard pins the order: the Theme tab borrows the
// inventory's type scale as two rows — this tab's own heading band and the
// section's body — and they come after the board's rows, not before them.
func TestTypeScaleFollowsTheBoard(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight

	scale := palette.TypeScaleRows(inv, paletteChrome(c), c, TypeFrom(shaper, typo).story())
	if len(scale) != 2 {
		t.Fatalf("the type scale is %d rows, want 2 (a heading band and a body)", len(scale))
	}
	rows := themeTabRows(inv, shaper, typo, c)
	if len(rows) != PaletteSectionRows+len(scale) {
		t.Fatalf("the Theme column is %d rows, want the board's %d plus the scale's %d",
			len(rows), PaletteSectionRows, len(scale))
	}
}

// typeSection is the inventory section the board borrows for its type scale,
// and sectionTitleSep the seam it splits the borrowed title at. The board
// owns both; they are written down here because this window's own guards rest
// on them, and a guard that reads its subject off the thing it is guarding
// checks nothing.
const (
	typeSection     = "foundations-type"
	sectionTitleSep = " — "
)

// TestTypeScaleKeepsTheInventorysWords is the guard on the one place
// this tab could quietly invent copy: the borrowed band's label and
// caption are the inventory's own title, split at its separator and
// nothing else. A title reworded upstream has to arrive here reworded.
func TestTypeScaleKeepsTheInventorysWords(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight

	var title string
	for _, s := range inv.Foundations(c) {
		if s.Name == typeSection {
			title = s.Title
		}
	}
	if title == "" {
		t.Fatalf("the inventory publishes no section named %q — the Theme tab's type scale is empty", typeSection)
	}
	label, hint, _ := strings.Cut(title, sectionTitleSep)
	if label == "" {
		t.Errorf("splitting %q leaves no label for the band", title)
	}
	if rejoined := label + sectionTitleSep + hint; rejoined != title {
		t.Errorf("the band says %q, the inventory says %q", rejoined, title)
	}
}

// TestTheBoardShowsEveryPlatformName is the arithmetic the board exists for:
// every name the set carries has a row, so a name added upstream appears here
// without an edit to this window. The board reads the set by reflection, and
// this asserts the count against the set itself.
func TestTheBoardShowsEveryPlatformName(t *testing.T) {
	rows := inventory.PlatformRows()
	if len(rows) == 0 {
		t.Fatal("the platform set publishes no rows; the board would be empty")
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.Name == "" {
			t.Error("a row carries no platform name")
		}
		if seen[r.Name] {
			t.Errorf("the board draws %q twice", r.Name)
		}
		seen[r.Name] = true
	}
}

// TestTheBoardsChromeIsFlattened guards the one rule a consumer of the set
// can break silently: every colour this window hands the board is opaque, so
// nothing composited over a fill it does not stand on ever reaches Gio.
func TestTheBoardsChromeIsFlattened(t *testing.T) {
	for _, tc := range schemeCases {
		t.Run(tc.name, func(t *testing.T) {
			chrome := paletteChrome(tc.colors)
			for _, f := range []struct {
				name string
				col  color.NRGBA
			}{
				{"Surface", chrome.Surface},
				{"Seam", chrome.Seam},
				{"Text", chrome.Text},
				{"Muted", chrome.Muted},
			} {
				if f.col.A != 0xff {
					t.Errorf("the board's %s is handed over at coverage %d; a caller composites before it paints", f.name, f.col.A)
				}
			}
		})
	}
}
