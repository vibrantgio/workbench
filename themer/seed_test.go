package main

import (
	"image"
	stdcolor "image/color"
	"strings"
	"testing"

	"gioui.org/layout"
	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/golden"

	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// fixtureLifted is a seed the derivation does not keep: the palette's accent
// chroma dial realizes it more chromatic than it was handed over, so the
// colour picked and the colour the palette grew from are two colours and the
// row has two cells. Every fixture the rest of this package uses is saturated
// past the dial and comes back unchanged, which is the one-cell case.
var fixtureLifted = tokens.DefaultSeed

// liftedSeeded is a window whose pick is lifted, which is the case the two
// cells exist for.
func liftedSeeded(t *testing.T) Model {
	t.Helper()
	m := dropped(t)
	m.Candidates = []imageseed.Candidate{candidate(fixtureLifted, 1)}
	m.Selected = 0
	return m
}

// seedSectionTop is the y of the seed body's first cell inside the window: the
// section leads the Theme tab, so it stands directly under the strip.
func seedSectionTop() int {
	return tabTop() + int(palette.SectionHeadH) + int(inventory.SectionPadY)
}

// seedSwatchCentre is the middle of cell i's swatch, in the slot every cell of
// this row gives its colour whether it fills the slot or stands inside it.
func seedSwatchCentre(i int) image.Point {
	return image.Pt(rampLabelLeft()+int(palette.PickSwatchW)/2,
		seedSectionTop()+i*int(palette.PickPairH)+int(palette.PickPairH)/2)
}

// seedSectionOn captures the seed section on its own, on the ground the column
// would stand it on, for the assertions about what it draws where nothing is
// picked — a state the window has no gallery in, so it cannot be read off a
// window render.
func seedSectionOn(t *testing.T, c tokens.ColorTokens, seed stdcolor.NRGBA, picked bool) *image.RGBA {
	t.Helper()
	rows := SeedRows(PaletteFrom(c), c, pinned(), seed, picked)
	size := image.Pt(windowW-2*int(Pad), 240)
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, size, c.Background)
		children := make([]layout.FlexChild, 0, len(rows))
		for _, row := range rows {
			children = append(children, layout.Rigid(row))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// TestSeedSectionIsTwoRows: the band and its body, which is the shape every
// section of this column has and what the tab's geometry is measured with.
func TestSeedSectionIsTwoRows(t *testing.T) {
	c, _ := tokens.FromSeed(fixtureLifted)
	if rows := SeedRows(PaletteFrom(c), c, pinned(), fixtureLifted, true); len(rows) != SeedSectionRows {
		t.Fatalf("the seed section is %d rows, SeedSectionRows says %d", len(rows), SeedSectionRows)
	}
}

// TestSeedTellsThePickFromWhatGrewFromIt: the colour handed over and the
// colour the derivation made of it are two cells, each with its own swatch and
// its own name, and the pick is the one drawn smaller.
//
// One sentence relating the two is the draft this row does not have: its only
// clause boundary sits between the colours, so a narrow window would cut it to
// a line naming one of them and claiming the other's fact.
func TestSeedTellsThePickFromWhatGrewFromIt(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureLifted)
	if light.Primary == fixtureLifted {
		t.Fatalf("the fixture seed %s is kept whole, want the lifted case", hexOf(fixtureLifted))
	}
	for _, sc := range []struct {
		name string
		c    tokens.ColorTokens
	}{{"light", light}, {"dark", dark}} {
		cells := seedCells(sc.c, fixtureLifted, true)
		if len(cells) != 2 {
			t.Fatalf("%s: the row draws %d cells, want the pick and what grew from it", sc.name, len(cells))
		}
		if cells[0].col != fixtureLifted || !cells[0].handedIn {
			t.Errorf("%s: the first cell is %v handedIn=%v, want the pick %v drawn smaller",
				sc.name, cells[0].col, cells[0].handedIn, fixtureLifted)
		}
		if cells[0].rule != SeedPickRule {
			t.Errorf("%s: the pick's rule is %q, want %q — the palette did not grow from it",
				sc.name, cells[0].rule, SeedPickRule)
		}
		if cells[1].col != light.Primary || cells[1].handedIn {
			t.Errorf("%s: the second cell is %v handedIn=%v, want the realized colour %v at full size",
				sc.name, cells[1].col, cells[1].handedIn, light.Primary)
		}
		if !strings.HasPrefix(cells[1].rule, SeedGrewFrom) {
			t.Errorf("%s: the realized colour's rule is %q, want it to open with %q",
				sc.name, cells[1].rule, SeedGrewFrom)
		}
	}
}

// TestSeedRowChecksItsClaimAgainstThePaletteOnScreen: what the row names is
// read off the palette it is standing on, and in the scheme that does not draw
// the colour it says so in the same clause that names it.
//
// A dark palette does not carry its seed: the light base is the seed realized
// and the dark side is re-toned to the dark scale's own depth, so the swatch
// beside the pick is a colour the ramps and the chips below it draw nowhere.
func TestSeedRowChecksItsClaimAgainstThePaletteOnScreen(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureLifted)
	lightCells, darkCells := seedCells(light, fixtureLifted, true), seedCells(dark, fixtureLifted, true)
	grown := lightCells[1].col
	if grown != light.Primary {
		t.Errorf("the light row names %v, want the Primary base the palette on screen pins %v", grown, light.Primary)
	}
	if grown == dark.Primary {
		t.Fatalf("the dark palette pins %v, want a re-toned base the row has to disclose", dark.Primary)
	}
	// Two facts, on two lines, because a cell's lines are cut at two different
	// widths: which colour the palette grew from, and that this scheme does not
	// draw it. Both must be in the row, and neither may lean on the other.
	if darkCells[1].name == lightCells[1].name {
		t.Errorf("the dark cell is named %q, the same as the light one — the name carries the scheme", darkCells[1].name)
	}
	if darkCells[1].rule == lightCells[1].rule {
		t.Errorf("the dark cell's rule is %q, the same as the light one — the rule carries the re-toning", darkCells[1].rule)
	}
	if !strings.HasPrefix(darkCells[1].rule, SeedGrewFrom) {
		t.Errorf("the dark rule is %q, want the claim at the front where no cut takes it off", darkCells[1].rule)
	}
}

// TestSeedKeptWholeIsOneCell: where the dial left the pick alone there is
// nothing to tell apart, so the row draws one colour and says both things
// about it.
func TestSeedKeptWholeIsOneCell(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureBlue)
	if light.Primary != fixtureBlue {
		t.Fatalf("the fixture seed %s is lifted, want the kept case", hexOf(fixtureBlue))
	}
	for _, sc := range []struct {
		name string
		c    tokens.ColorTokens
	}{{"light", light}, {"dark", dark}} {
		cells := seedCells(sc.c, fixtureBlue, true)
		if len(cells) != 1 {
			t.Fatalf("%s: the row draws %d cells for a seed kept whole, want one", sc.name, len(cells))
		}
		if cells[0].col != fixtureBlue || cells[0].handedIn {
			t.Errorf("%s: the cell is %v handedIn=%v, want the seed at full size", sc.name, cells[0].col, cells[0].handedIn)
		}
		if !strings.HasPrefix(cells[0].rule, SeedGrewFrom) || !strings.Contains(cells[0].rule, SeedPickRule) {
			t.Errorf("%s: the rule is %q, want it to say both that it grew the palette and that it was picked",
				sc.name, cells[0].rule)
		}
	}
	if a, b := seedCells(light, fixtureBlue, true)[0].rule, seedCells(dark, fixtureBlue, true)[0].rule; a == b {
		t.Errorf("both schemes say %q, want the dark one to disclose the re-toning", a)
	}
}

// TestSeedNamesNoSeedItCannotProve: a palette that is not this seed's gets no
// claim about what it grew from, only the one thing the window knows
// first-hand — which colour was clicked.
func TestSeedNamesNoSeedItCannotProve(t *testing.T) {
	stranger, _ := tokens.FromSeed(fixtureRed)
	if _, ok := grownFrom(stranger, fixtureBlue); ok {
		t.Fatal("a palette grown from another seed checked out as this one's")
	}
	cells := seedCells(stranger, fixtureBlue, true)
	if len(cells) != 1 || cells[0].col != fixtureBlue {
		t.Fatalf("the unproven row draws %d cells, want the pick alone", len(cells))
	}
	if strings.Contains(cells[0].rule, SeedGrewFrom) {
		t.Errorf("the unproven rule is %q, want no claim about what the palette grew from", cells[0].rule)
	}
	// And the palette that is this seed's checks out on both sides of the pair,
	// which is what makes the claim worth checking rather than a formality.
	light, dark := tokens.FromSeed(fixtureBlue)
	for _, c := range []tokens.ColorTokens{light, dark} {
		if grown, ok := grownFrom(c, fixtureBlue); !ok || grown != light.Primary {
			t.Errorf("the seed's own palette checked out %v/%v, want the realized %v", grown, ok, light.Primary)
		}
	}
}

// TestSeedRowSaysWhenNothingIsPicked: with no candidate chosen the row says so
// rather than showing a seed that is not there — no swatch, and a caption that
// says what the bar will hold rather than describing a derivation that is not
// on screen.
func TestSeedRowSaysWhenNothingIsPicked(t *testing.T) {
	c := tokens.DefaultLight
	cells := seedCells(c, stdcolor.NRGBA{}, false)
	if len(cells) != 1 || !cells[0].wordsOnly {
		t.Fatalf("the unpicked row draws %d cells, want one made of words", len(cells))
	}
	if cells[0].name != SeedNoneName {
		t.Errorf("the unpicked row is named %q, want %q", cells[0].name, SeedNoneName)
	}
	if got := seedHint(cells); got != SeedHintNone {
		t.Errorf("the unpicked caption is %q, want %q", got, SeedHintNone)
	}
	// And no swatch is drawn: the slot a cell gives its colour is the ground.
	img := seedSectionOn(t, c, stdcolor.NRGBA{}, false)
	top := int(palette.SectionHeadH) + int(inventory.SectionPadY)
	at := image.Pt(int(inventory.SectionPadX)+int(palette.PickSwatchW)/2, top+int(palette.PickCellH)/2)
	got := img.RGBAAt(at.X, at.Y)
	if want := c.Background; got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("the unpicked row drew %v where a swatch would stand at %v, want the ground %v", got, at, want)
	}
}

// TestSeedLinesCarryNoUnmarkedSeam: every line this row can draw is one
// clause, so [palette.FitLine] has no unmarked cut to make on any of them
// and every
// cut a reader is shown ends in an ellipsis.
//
// This is the guard the two cells rest on. A line with a clause seam in it is
// a line a narrow window can shorten into a different claim without saying
// that it did, and there is no wording of a comma a reader can be relied on to
// supply back.
func TestSeedLinesCarryNoUnmarkedSeam(t *testing.T) {
	lines := []string{
		SeedName, SeedLiftedName, SeedLiftedNameDark, SeedNoneName,
		SeedGrewFrom, SeedPickRule, SeedLiftedRule, SeedLiftedRuleDark,
		SeedKeptRule, SeedKeptRuleDark, SeedUnprovenRule, SeedNoneRule,
	}
	for _, line := range lines {
		if heads := palette.LineHeads(line, true); len(heads) > 0 {
			t.Errorf("%q can be cut to %q with nothing marking the cut", line, heads[0])
		}
	}
	// The caption is the one thing here written as clauses, and is cut at the
	// separator the cut knows about.
	for _, clause := range []string{SeedHintPair, SeedHintHue, SeedHintChroma, SeedHintStatus, SeedHintNone} {
		if strings.Contains(clause, palette.HintSep) {
			t.Errorf("the caption clause %q carries the separator its own list is strung on", clause)
		}
	}
}

// TestSeedCaptionSizesItselfToWhatIsDrawn: the legend for the two sizes is
// only true where two cells are drawn, so it is only said there.
func TestSeedCaptionSizesItselfToWhatIsDrawn(t *testing.T) {
	light, _ := tokens.FromSeed(fixtureLifted)
	pair := seedHint(seedCells(light, fixtureLifted, true))
	if !strings.HasPrefix(pair, SeedHintPair) {
		t.Errorf("the two-cell caption is %q, want it to lead with the legend for the sizes", pair)
	}
	kept, _ := tokens.FromSeed(fixtureBlue)
	one := seedHint(seedCells(kept, fixtureBlue, true))
	if strings.Contains(one, SeedHintPair) {
		t.Errorf("the one-cell caption is %q, want no legend for a size distinction it does not draw", one)
	}
	if !strings.HasPrefix(one, SeedHintHue) {
		t.Errorf("the one-cell caption is %q, want the derivation it does describe", one)
	}
}

// TestSeedRowOpensTheThemeTab: the seed is the first thing the tab shows, read
// off the window's own pixels — the pick's swatch and the colour it grew into,
// in that order, above the ramps that were derived from them.
func TestSeedRowOpensTheThemeTab(t *testing.T) {
	m := liftedSeeded(t)
	light, _ := tokens.FromSeed(fixtureLifted)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		img := page(t, m, os)
		for i, want := range []stdcolor.NRGBA{fixtureLifted, light.Primary} {
			at := seedSwatchCentre(i)
			got := img.RGBAAt(at.X, at.Y)
			if got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("cell %d at %v drew %v, want %v", i, at, got, want)
			}
		}
		// And the ramps stand under it rather than where they used to: the
		// first cell of the first row is the step it always was, one section
		// further down the column.
		at := rampCellColour(m, os, 0, 0)
		c, _ := derived(m, os)
		want := c.Ramps.Primary.Step(100)
		if got := img.RGBAAt(at.X, at.Y); got.R != want.R || got.G != want.G || got.B != want.B {
			t.Errorf("the grid's first cell at %v drew %v, want %v — the seed section moved the grid", at, got, want)
		}
	}
}
