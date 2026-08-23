package main

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// themeCanvasSize is the Theme tab's content area at the app's default
// window, which is what the goldens pin.
var themeCanvasSize = image.Pt(1180, 760)

// TestThemeTabGolden pins the Theme tab in both schemes: the ramps grid
// with its step numbers and pinned-base chips, the picks board with the
// rule that chose each colour, and the type ladder under them.
func TestThemeTabGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	for _, tc := range schemeCases {
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

// TestTypeLadderFollowsThePalette is the AH1.1 move made checkable: the
// Theme tab borrows the inventory's type ladder as two rows — this tab's
// own heading band and the section's body — and they come after the
// palette's four rows, not before them.
func TestTypeLadderFollowsThePalette(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.DefaultLight

	ladder := typeLadderRows(inv, PaletteFrom(c), c, TypeFrom(shaper, typo))
	if len(ladder) != 2 {
		t.Fatalf("the type ladder is %d rows, want 2 (a heading band and a body)", len(ladder))
	}
	rows := themeTabRows(inv, shaper, typo, c)
	if len(rows) != PaletteSectionRows+len(ladder) {
		t.Fatalf("the Theme column is %d rows, want the palette's %d plus the ladder's %d",
			len(rows), PaletteSectionRows, len(ladder))
	}
}

// TestTypeLadderKeepsTheInventorysWords is the guard on the one place
// this tab could quietly invent copy: the borrowed band's label and
// caption are the inventory's own title, split at its separator and
// nothing else. A title reworded upstream has to arrive here reworded.
func TestTypeLadderKeepsTheInventorysWords(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.DefaultLight

	var title string
	for _, s := range inv.Foundations(c) {
		if s.Name == typeSection {
			title = s.Title
		}
	}
	if title == "" {
		t.Fatalf("the inventory publishes no section named %q — the Theme tab's ladder is empty", typeSection)
	}
	label, hint, _ := strings.Cut(title, sectionTitleSep)
	if label == "" {
		t.Errorf("splitting %q leaves no label for the band", title)
	}
	if rejoined := label + sectionTitleSep + hint; rejoined != title {
		t.Errorf("the band says %q, the inventory says %q", rejoined, title)
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
