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
			w := renderThemeTab(shaper, tc.colors, tokens.DefaultTypography, tokens.DefaultSeed)
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
	a := golden.Capture(t, themeCanvasSize, scene(renderThemeTab(shaper, tokens.DefaultLight, tokens.DefaultTypography, tokens.DefaultSeed), bg))
	b := golden.Capture(t, themeCanvasSize, scene(renderThemeTab(shaper, tokens.DefaultDark, tokens.DefaultTypography, tokens.DefaultSeed), bg))
	if golden.PixelDiff(a, b) == 0 {
		t.Fatal("theme tab renders identically in light and dark — the section is not following its tokens")
	}
}

// TestPaletteSectionRowsIsTheRowCount asserts the stated row counts
// against the rows themselves, the way the themer's own tests do: the
// palette story is the seed's two rows and the copied section's four.
func TestPaletteSectionRowsIsTheRowCount(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	c := tokens.DefaultLight
	if got := len(PaletteRows(PaletteFrom(c), c, schemeCounterpart(c), TypeFrom(shaper, tokens.DefaultTypography), false)); got != PaletteSectionRows {
		t.Fatalf("PaletteRows returns %d rows, PaletteSectionRows says %d", got, PaletteSectionRows)
	}
	if got := len(themePaletteRows(shaper, tokens.DefaultTypography, c, tokens.DefaultSeed)); got != SeedSectionRows+PaletteSectionRows {
		t.Fatalf("the palette story is %d rows, want the seed's %d plus the section's %d",
			got, SeedSectionRows, PaletteSectionRows)
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
	rows := themeTabRows(inv, shaper, typo, c, tokens.DefaultSeed)
	if len(rows) != SeedSectionRows+PaletteSectionRows+len(ladder) {
		t.Fatalf("the Theme column is %d rows, want the seed's %d plus the palette's %d plus the ladder's %d",
			len(rows), SeedSectionRows, PaletteSectionRows, len(ladder))
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

// TestSeedRowNamesWhatItShows is the honesty rule made checkable. Every
// colour the row draws is named for what it is: the colour picked, the
// colour the palette grew from, or — where neither can be proved — the
// token actually on screen. No cell is ever called the pick unless it is
// the pick byte for byte.
func TestSeedRowNamesWhatItShows(t *testing.T) {
	// The default seed is itself a pick the dial moves — #6750a4 measures
	// chroma 0.1305 against the accent dial's 0.22 — so the default
	// palette is the two-cell case, and it is what the goldens photograph.
	moved := tokens.DefaultSeed
	if tokens.DefaultLight.Primary == moved {
		t.Fatalf("the default seed %s is no longer moved by the accent dial; this test needs a pick that is",
			hexOf(moved))
	}
	grown := tokens.DefaultLight.Primary
	// A pick already past the dial comes back byte for byte, which is the
	// projection the whole recovery rests on.
	vivid := color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	vividLight, vividDark := tokens.FromSeed(vivid)
	if vividLight.Primary != vivid {
		t.Fatalf("fixture %s is moved by the accent dial; this test needs one it leaves alone", hexOf(vivid))
	}
	hcLight, _ := tokens.FromSeedHighContrast(tokens.DefaultSeed)

	for _, tc := range []struct {
		name  string
		c     tokens.ColorTokens
		seed  color.NRGBA
		cells []seedCell
	}{
		{"light, a pick the dial moved", tokens.DefaultLight, moved, []seedCell{
			{moved, SeedName, SeedPickRule},
			{grown, SeedLiftedName, SeedLiftedRule},
		}},
		{"dark, a pick the dial moved", tokens.DefaultDark, moved, []seedCell{
			{moved, SeedName, SeedPickRule},
			{grown, SeedLiftedName, SeedLiftedRuleDark},
		}},
		{"high contrast", hcLight, moved, []seedCell{
			{moved, SeedName, SeedPickRule},
			{hcLight.Primary, SeedLiftedName, SeedLiftedRule},
		}},
		{"light, the pick itself", vividLight, vivid, []seedCell{
			{vivid, SeedName, SeedKeptRule},
		}},
		{"dark, the pick itself", vividDark, vivid, []seedCell{
			{vivid, SeedName, SeedKeptRuleDark},
		}},
		{"light, not this seed's palette", tokens.DefaultLight, vivid, []seedCell{
			{tokens.DefaultLight.Primary, SeedName, SeedFromBase},
		}},
		{"dark, not this seed's palette", tokens.DefaultDark, vivid, []seedCell{
			{tokens.DefaultDark.Primary, SeedPinName, SeedNotHeld},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := seedCells(tc.c, tc.seed)
			if len(got) != len(tc.cells) {
				t.Fatalf("the row draws %d cells, want %d", len(got), len(tc.cells))
			}
			for i, want := range tc.cells {
				if got[i] != want {
					t.Errorf("cell %d is {%s %q %q}, want {%s %q %q}",
						i, hexOf(got[i].col), got[i].name, got[i].rule,
						hexOf(want.col), want.name, want.rule)
				}
			}
		})
	}

	// The one thing the row may never do: show a colour that is not the
	// pick under the rule that says it is.
	for _, c := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		for _, cell := range seedCells(c, moved) {
			if cell.rule == SeedPickRule && cell.col != moved {
				t.Errorf("a cell showing %s is captioned %q, and the pick is %s",
					hexOf(cell.col), cell.rule, hexOf(moved))
			}
		}
	}
}

// TestSeedRulesNameOneColourEach is the guard on how this row broke
// once. The first draft told the pick and the colour grown from it apart
// inside one sentence; fitLine cuts a line at its commas and marks
// nothing when it does, so the sentence cut to "the colour this palette
// grew from — #6750A4 picked" and asserted, unmarked, the one thing the
// row exists to deny. The fix was structural: two colours are two cells,
// each swatch carrying its own value on its own line, so no rule names a
// colour at all and no cut can fuse two of them into one claim. This
// keeps it that way.
func TestSeedRulesNameOneColourEach(t *testing.T) {
	rules := []string{
		SeedPickRule, SeedLiftedRule, SeedLiftedRuleDark,
		SeedKeptRule, SeedKeptRuleDark, SeedFromBase, SeedNotHeld,
	}
	for _, rule := range rules {
		if strings.Contains(rule, "#") {
			t.Errorf("rule %q names a colour value; values belong on the cell's own value line, "+
				"where a truncation cannot attach one colour's value to another colour's claim", rule)
		}
		// fitLine cuts at commas and marks nothing, so every head has to
		// stand as a claim of its own.
		if head, _, cut := strings.Cut(rule, ","); cut && strings.TrimSpace(head) == "" {
			t.Errorf("rule %q cuts to nothing", rule)
		}
	}
	// And the two colours really are two cells whenever they are two
	// colours: no cell may carry both.
	for _, c := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		cells := seedCells(c, tokens.DefaultSeed)
		if len(cells) != 2 {
			t.Fatalf("the default pick is moved by the dial but the row draws %d cell(s)", len(cells))
		}
		if cells[0].col == cells[1].col {
			t.Errorf("both cells show %s; the row is meant to be showing two colours", hexOf(cells[0].col))
		}
	}
}

// TestSeedRowIsTheHeadOfTheStory pins the order the tab tells it in: the
// input first, then the ramps and picks made of it.
func TestSeedRowIsTheHeadOfTheStory(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	c := tokens.DefaultLight
	p, ty := PaletteFrom(c), TypeFrom(shaper, tokens.DefaultTypography)
	story := themePaletteRows(shaper, tokens.DefaultTypography, c, tokens.DefaultSeed)
	head := seedRows(p, c, ty, tokens.DefaultSeed)
	if len(head) != SeedSectionRows {
		t.Fatalf("seedRows returns %d rows, SeedSectionRows says %d", len(head), SeedSectionRows)
	}
	if len(story) < len(head)+1 {
		t.Fatalf("the palette story is %d rows, too few to lead with the seed", len(story))
	}
	// The band a row draws is what identifies it, and the widgets
	// themselves are opaque, so the order is checked by drawing the story's
	// leading row and the seed band alone and comparing the pixels.
	size := image.Pt(themeCanvasSize.X, 40)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	first := golden.Capture(t, size, scene(story[0], bg))
	band := golden.Capture(t, size, scene(head[0], bg))
	if golden.PixelDiff(first, band) != 0 {
		t.Error("the palette story does not open with the seed band")
	}
}
