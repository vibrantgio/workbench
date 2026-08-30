package main

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/textdraw"
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
	if got := len(themePaletteRows(shaper, tokens.DefaultTypography, c, tokens.DefaultSeed)); got != palette.SeedSectionRows+PaletteSectionRows {
		t.Fatalf("the palette story is %d rows, want the seed's %d plus the section's %d",
			got, palette.SeedSectionRows, PaletteSectionRows)
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

	ladder := palette.TypeLadderRows(inv, PaletteFrom(c).story(), c, TypeFrom(shaper, typo).story())
	if len(ladder) != 2 {
		t.Fatalf("the type ladder is %d rows, want 2 (a heading band and a body)", len(ladder))
	}
	rows := themeTabRows(inv, shaper, typo, c, tokens.DefaultSeed)
	if len(rows) != palette.SeedSectionRows+PaletteSectionRows+len(ladder) {
		t.Fatalf("the Theme column is %d rows, want the seed's %d plus the palette's %d plus the ladder's %d",
			len(rows), palette.SeedSectionRows, PaletteSectionRows, len(ladder))
	}
}

// typeSection is the inventory section the palette story borrows for its
// type ladder, and sectionTitleSep the seam it splits the borrowed title
// at. The story owns both; they are written down here because this
// window's own guards rest on them, and a guard that reads its subject
// off the thing it is guarding checks nothing.
const (
	typeSection     = "foundations-type"
	sectionTitleSep = " — "
)

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
			groups := palette.Groups(tc.c, tc.o, tc.dark)
			claims := palette.Claims(groups)
			if len(claims) == 0 {
				t.Fatal("no pick claims any rung — the grid would carry no marks at all")
			}
			for claim := range claims {
				if claim.Role == "" || claim.Step < 100 || claim.Step > 900 || claim.Step%100 != 0 {
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
			palette.SeedHex(moved))
	}
	grown := tokens.DefaultLight.Primary
	// A pick already past the dial comes back byte for byte, which is the
	// projection the whole recovery rests on.
	vivid := color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	vividLight, vividDark := tokens.FromSeed(vivid)
	if vividLight.Primary != vivid {
		t.Fatalf("fixture %s is moved by the accent dial; this test needs one it leaves alone", palette.SeedHex(vivid))
	}
	hcLight, _ := tokens.FromSeedHighContrast(tokens.DefaultSeed)

	for _, tc := range []struct {
		name  string
		c     tokens.ColorTokens
		seed  color.NRGBA
		cells []palette.SeedCell
	}{
		{"light, a pick the dial moved", tokens.DefaultLight, moved, []palette.SeedCell{
			{Col: moved, Name: palette.SeedName, Rule: palette.SeedPickRule, HandedIn: true},
			{Col: grown, Name: palette.SeedLiftedName, Rule: palette.SeedLiftedRule},
		}},
		{"dark, a pick the dial moved", tokens.DefaultDark, moved, []palette.SeedCell{
			{Col: moved, Name: palette.SeedName, Rule: palette.SeedPickRule, HandedIn: true},
			{Col: grown, Name: SeedLiftedNameDark, Rule: palette.SeedLiftedRuleDark},
		}},
		{"high contrast", hcLight, moved, []palette.SeedCell{
			{Col: moved, Name: palette.SeedName, Rule: palette.SeedPickRule, HandedIn: true},
			{Col: hcLight.Primary, Name: palette.SeedLiftedName, Rule: palette.SeedLiftedRule},
		}},
		{"light, the pick itself", vividLight, vivid, []palette.SeedCell{
			{Col: vivid, Name: palette.SeedName, Rule: palette.SeedKeptRule},
		}},
		{"dark, the pick itself", vividDark, vivid, []palette.SeedCell{
			{Col: vivid, Name: palette.SeedName, Rule: palette.SeedKeptRuleDark},
		}},
		{"light, not this seed's palette", tokens.DefaultLight, vivid, []palette.SeedCell{
			{Col: tokens.DefaultLight.Primary, Name: SeedPinName, Rule: SeedFromBase},
		}},
		{"dark, not this seed's palette", tokens.DefaultDark, vivid, []palette.SeedCell{
			{Col: tokens.DefaultDark.Primary, Name: SeedPinName, Rule: SeedNotHeld},
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
						i, palette.SeedHex(got[i].Col), got[i].Name, got[i].Rule,
						palette.SeedHex(want.Col), want.Name, want.Rule)
				}
			}
		})
	}

	// The one thing the row may never do: show a colour that is not the
	// pick under the rule that says it is.
	for _, c := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		for _, cell := range seedCells(c, moved) {
			if cell.Rule == palette.SeedPickRule && cell.Col != moved {
				t.Errorf("a cell showing %s is captioned %q, and the pick is %s",
					palette.SeedHex(cell.Col), cell.Rule, palette.SeedHex(moved))
			}
		}
	}
}

// TestSeedRulesNameOneColourEach is the guard on how this row broke
// once. The first draft told the pick and the colour grown from it apart
// inside one sentence; [palette.FitLine] cuts a line at its commas and
// marks nothing when it does, so the sentence cut to "the colour this
// grew from — #6750A4 picked" and asserted, unmarked, the one thing the
// row exists to deny. The fix was structural: two colours are two cells,
// each swatch carrying its own value on its own line, so no rule names a
// colour at all and no cut can fuse two of them into one claim. This
// keeps it that way.
func TestSeedRulesNameOneColourEach(t *testing.T) {
	for _, rule := range seedRules {
		if strings.Contains(rule, "#") {
			t.Errorf("rule %q names a colour value; values belong on the cell's own value line, "+
				"where a truncation cannot attach one colour's value to another colour's claim", rule)
		}
	}
	// And the two colours really are two cells whenever they are two
	// colours: no cell may carry both.
	for _, c := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		cells := seedCells(c, tokens.DefaultSeed)
		if len(cells) != 2 {
			t.Fatalf("the default pick is moved by the dial but the row draws %d cell(s)", len(cells))
		}
		if cells[0].Col == cells[1].Col {
			t.Errorf("both cells show %s; the row is meant to be showing two colours", palette.SeedHex(cells[0].Col))
		}
	}
}

// seedRules is every rule the row can put under a swatch, and seedNames
// every name it can put over one — the two sets the truncation guards
// below are run over. The story's own lines are in them as well as this
// window's: the guards below measure what a cut does in this window's type
// roles at this window's widths, which is a fact about neither set on its
// own.
var (
	seedRules = []string{
		palette.SeedPickRule, palette.SeedLiftedRule, palette.SeedLiftedRuleDark,
		palette.SeedKeptRule, palette.SeedKeptRuleDark, SeedFromBase, SeedNotHeld,
	}
	seedNames = []string{palette.SeedName, palette.SeedLiftedName, SeedLiftedNameDark, SeedPinName}
)

// TestSeedTextTakesNoUnmarkedCut is the guard the first fix did not put
// in. [palette.FitLine] has two ways to shorten a line: at a clause seam
// — a comma, " ·" or " /" — with nothing at all marking the cut, and at
// a word boundary with an ellipsis. The rework of this row moved its
// honesty disclosure past a comma, where the unmarked cut shed it whole
// at any window under about 586px and handed the reader back the exact
// claim the row exists to deny.
//
// The fix is structural rather than editorial: no string this row draws
// carries a clause seam at all, so the unmarked path has nothing to cut
// at and every cut a reader is ever shown ends in an ellipsis. This
// checks both halves — that the seams are absent, and that the whole
// range of rooms a window can give produces nothing but the string
// itself or a marked prefix of it.
func TestSeedTextTakesNoUnmarkedCut(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	ty := TypeFrom(shaper, tokens.DefaultTypography)
	gtx := measuringContext()
	for _, str := range append(append([]string{}, seedRules...), seedNames...) {
		t.Run(str, func(t *testing.T) {
			if heads := palette.LineHeads(str, true); len(heads) != 0 {
				t.Fatalf("%q carries a clause seam, so the story can cut it to %q with nothing marking the cut",
					str, heads[0])
			}
			for _, style := range []textdraw.TextStyle{ty.Small, ty.Body} {
				full := natural(gtx, shaper, style, str)
				for room := 0; room <= full+8; room++ {
					got := palette.FitLine(gtx, shaper, style, str, room)
					switch {
					case got == str:
						// Whole, or the last-resort fallback the shaper
						// clips itself; either way nothing was dropped
						// silently.
					case strings.HasSuffix(got, Ellipsis) &&
						strings.HasPrefix(str, strings.TrimSuffix(got, Ellipsis)):
						// A marked cut, and a prefix of what it cut.
					default:
						t.Fatalf("at room %d, %q comes back as %q — a cut that is neither whole nor marked",
							room, str, got)
					}
				}
			}
		})
	}
}

// TestSeedTextSurvivesTheNarrowWindow puts a number on it. The finding
// this task answers was measured at a window: under about 586px the dark
// rule lost its disclosure. Every string the row draws now stands whole
// in the room a 560px window leaves — the width the verification pass
// captures at.
//
// This is a width, at the one text scale the goldens are drawn at. Room
// is measured in dp and text in sp, so a reader who has turned the OS
// text scale up meets these cuts at a wider window than this test asks
// about; what holds for them is the guard above, that no cut of any of
// these strings goes unmarked at any scale.
func TestSeedTextSurvivesTheNarrowWindow(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	ty := TypeFrom(shaper, tokens.DefaultTypography)
	gtx := measuringContext()
	// The room drawSeedCell hands a line at this content width: the body's
	// margins off both edges, then the swatch slot and the air beside it.
	const narrowContent = 540 // a 560px window, less the shell's own edge
	room := narrowContent - 2*gtx.Dp(inventory.SectionPadX) - gtx.Dp(palette.PickSwatchW) - gtx.Dp(palette.PickGap)
	for _, rule := range seedRules {
		if got := palette.FitLine(gtx, shaper, ty.Small, rule, room); got != rule {
			t.Errorf("at a %dpx window the row draws %q, cut from %q — it wants %d of the %d it has",
				narrowContent+20, got, rule, natural(gtx, shaper, ty.Small, rule), room)
		}
	}
	for _, name := range seedNames {
		if got := palette.FitLine(gtx, shaper, ty.Body, name, room); got != name {
			t.Errorf("at a %dpx window the row draws the name %q, cut from %q", narrowContent+20, got, name)
		}
	}
}

// TestSeedSaysWhatThePaletteGrewFrom is the second finding. A section
// titled Palette Seed that identifies no seed is worth less than no
// section, and on the matched path — the one the goldens photograph —
// every line the previous draft drew was about the pick or about the
// base, and none of them said which colour the palette grew from.
//
// It has to be said in a clause no cut can shed, so it is said first:
// [palette.FitLine] takes words off the tail, and a claim that leads
// survives every cut down to the point where the shaper is clipping
// single words.
func TestSeedSaysWhatThePaletteGrewFrom(t *testing.T) {
	// Every path where the row can prove what the palette grew from.
	vivid := color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	vividLight, vividDark := tokens.FromSeed(vivid)
	hcLight, hcDark := tokens.FromSeedHighContrast(tokens.DefaultSeed)
	for _, tc := range []struct {
		name string
		c    tokens.ColorTokens
		seed color.NRGBA
	}{
		{"light", tokens.DefaultLight, tokens.DefaultSeed},
		{"dark", tokens.DefaultDark, tokens.DefaultSeed},
		{"high contrast light", hcLight, tokens.DefaultSeed},
		{"high contrast dark", hcDark, tokens.DefaultSeed},
		{"light, the pick itself", vividLight, vivid},
		{"dark, the pick itself", vividDark, vivid},
		{"light, no candidate matched", tokens.DefaultLight, vivid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			said := false
			for _, cell := range seedCells(tc.c, tc.seed) {
				if strings.HasPrefix(cell.Rule, palette.SeedGrewFrom) {
					said = true
				}
			}
			if !said {
				t.Errorf("no cell opens with %q, so nothing on screen says which colour the palette grew from",
					palette.SeedGrewFrom)
			}
		})
	}
	// And the one path where it cannot be proved must not claim it: a
	// dark palette handed a candidate that is not its own has no seed to
	// name, and says so.
	for _, cell := range seedCells(tokens.DefaultDark, vivid) {
		if strings.Contains(cell.Rule, palette.SeedGrewFrom) {
			t.Errorf("the unmatched dark row claims %q, and it has no seed to claim", cell.Rule)
		}
	}
}

// TestSeedDarkRuleDisclosesItsScheme is the first finding's other half.
// A dark palette draws a re-toned accent, so the colour this row shows
// as the one the palette grew from is a colour nowhere in the ramps
// under it. The rule has to say so, and has to say so in the same clause
// that makes the claim — the previous draft put it after the rule's only
// comma, which is exactly where [palette.FitLine] takes things off.
func TestSeedDarkRuleDisclosesItsScheme(t *testing.T) {
	const disclosure = "re-toned"
	for _, rule := range []string{palette.SeedLiftedRuleDark, palette.SeedKeptRuleDark} {
		if !strings.Contains(rule, disclosure) {
			t.Errorf("dark rule %q does not disclose that this scheme re-tones the colour", rule)
		}
		if strings.Contains(rule, ",") {
			t.Errorf("dark rule %q carries a comma, and its disclosure is past it", rule)
		}
	}
	// The rule the dark row actually draws is one of those two, whichever
	// case the candidate falls in.
	for _, seed := range []color.NRGBA{tokens.DefaultSeed, {R: 0xff, A: 0xff}} {
		cells := seedCells(tokens.DefaultDark, seed)
		last := cells[len(cells)-1]
		if grown, ok := grownFrom(tokens.DefaultDark, seed); ok {
			if last.Col != grown {
				t.Fatalf("the dark row's last cell shows %s, not the colour grown %s", palette.SeedHex(last.Col), palette.SeedHex(grown))
			}
			if !strings.Contains(last.Rule, disclosure) {
				t.Errorf("the dark row shows %s under %q with no word about this scheme re-toning it",
					palette.SeedHex(last.Col), last.Rule)
			}
		}
	}
}

// TestSeedDarkDisclosureOutlivesItsRule is the answer to the half of the
// finding a single line cannot give. Two facts have to survive here —
// which colour the palette grew from, and that a dark scheme does not
// draw it — and [palette.FitLine] takes words off the tail, so two facts
// on one line have an order and the second one goes first. They are
// therefore on two lines, which are cut independently: the rule opens
// with the claim, and the name carries the scheme. This checks that the
// name really is the one that lasts, at every room down to nothing.
func TestSeedDarkDisclosureOutlivesItsRule(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	ty := TypeFrom(shaper, tokens.DefaultTypography)
	gtx := measuringContext()
	// narrowest is the least room from which on up the words are always
	// drawn: the width the reader stops being told this at. Rooms too
	// small for even one word are not asked about — the story hands the
	// whole line back there and the shaper clips it, so the words are
	// "present" in a line nobody can read.
	narrowest := func(style textdraw.TextStyle, str, words string) int {
		full := natural(gtx, shaper, style, str)
		for room := full; room > 0; room-- {
			if !strings.Contains(palette.FitLine(gtx, shaper, style, str, room), words) {
				return room + 1
			}
		}
		return 1
	}
	rule := narrowest(ty.Small, palette.SeedLiftedRuleDark, "re-toned")
	name := narrowest(ty.Body, SeedLiftedNameDark, "light scheme")
	if name >= rule {
		t.Errorf("the name holds the scheme down to %ddp and the rule down to %ddp; "+
			"the disclosure is meant to outlive the line that makes the claim", name, rule)
	}
	// And the claim itself outlives nothing but the shaper: it leads the
	// rule, so it is the last thing on that line to go.
	if claim := narrowest(ty.Small, palette.SeedLiftedRuleDark, "grew from"); claim >= rule {
		t.Errorf("the claim holds down to %ddp and the disclosure to %ddp; the claim is meant to lead", claim, rule)
	}
}

// TestSeedNamesOnlyPicksSeed keeps the row's one word for the one thing.
// A cell called Seed is a colour somebody picked; where the row cannot
// prove a pick — a palette wearing an accent nobody told this app about
// — it names the token it is showing instead and leaves the claim to the
// rule, which is careful about it.
func TestSeedNamesOnlyPicksSeed(t *testing.T) {
	vivid := color.NRGBA{R: 0xff, A: 0xff}
	for _, tc := range []struct {
		name string
		c    tokens.ColorTokens
		seed color.NRGBA
	}{
		{"light", tokens.DefaultLight, tokens.DefaultSeed},
		{"dark", tokens.DefaultDark, tokens.DefaultSeed},
		{"light, no candidate matched", tokens.DefaultLight, vivid},
		{"dark, no candidate matched", tokens.DefaultDark, vivid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, cell := range seedCells(tc.c, tc.seed) {
				if cell.Name != palette.SeedName {
					continue
				}
				if cell.Col != tc.seed {
					t.Errorf("a cell showing %s is called %q, and the colour picked is %s",
						palette.SeedHex(cell.Col), cell.Name, palette.SeedHex(tc.seed))
				}
			}
		})
	}
}

// TestSeedPairIsToldApartWithoutChroma is the third finding. The two
// colours are one hue at two chromas: measured, the default pair stands
// at 1.00:1 luminance and four greyscale levels apart, which is one
// swatch drawn twice to anybody whose display or eyes take the chroma
// out. The row answers with size — the smaller swatch is the colour the
// palette only took in — and this checks the answer the way the finding
// was made, on the pixels with the colour taken out.
func TestSeedPairIsToldApartWithoutChroma(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	for _, tc := range schemeCases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.colors
			cells := seedCells(c, tokens.DefaultSeed)
			if len(cells) != 2 {
				t.Fatalf("the default seed draws %d cell(s); this test needs the pair", len(cells))
			}
			p, ty := PaletteFrom(c), TypeFrom(shaper, tokens.DefaultTypography)
			rows := seedRows(p, c, ty, tokens.DefaultSeed)
			pairH, padY := int(palette.PickPairH), int(inventory.SectionPadY)
			size := image.Pt(themeCanvasSize.X, 2*pairH+2*padY)
			img := golden.Capture(t, size, scene(rows[1], tc.bg))
			// Only the swatch column: the lines beside it differ in words,
			// which would carry this test whatever the swatches did.
			right := int(inventory.SectionPadX) + int(palette.PickSwatchW)
			worst := 0
			for y := 0; y < pairH; y++ {
				for x := 0; x < right; x++ {
					a := grey(img.At(x, padY+y))
					b := grey(img.At(x, padY+pairH+y))
					worst = max(worst, abs(a-b))
				}
			}
			// The chroma-only pair the finding measured came to four.
			if worst < 24 {
				t.Errorf("with the colour taken out the two swatches differ by at most %d of 255; "+
					"they are one swatch drawn twice", worst)
			}
			// A difference is not the difference. A one-pixel inset also
			// puts a ring of ground between the two rows and clears the
			// bar above while being invisible, so the size the channel is
			// actually drawn at is measured here rather than assumed: the
			// swatch of the colour handed in is the story's own inset
			// smaller on every side than the one the realized colour
			// fills.
			pick := swatchBox(img, c.Background, image.Rect(0, padY, right, padY+pairH))
			grown := swatchBox(img, c.Background, image.Rect(0, padY+pairH, right, padY+2*pairH))
			if pick.Empty() || grown.Empty() {
				t.Fatalf("found %v and %v where two swatches should be", pick, grown)
			}
			// Within a pixel of it: the frame is stroked with a
			// half-width inset and its corners are round, so the outermost
			// column of a swatch can be one antialiased pixel wide.
			inset := 2 * int(palette.SeedHandedInset)
			if got := grown.Dx() - pick.Dx(); abs(got-inset) > 1 {
				t.Errorf("the swatches are %dpx apart in width, want %d — %v against %v",
					got, inset, pick, grown)
			}
			if got := grown.Dy() - pick.Dy(); abs(got-inset) > 1 {
				t.Errorf("the swatches are %dpx apart in height, want %d — %v against %v",
					got, inset, pick, grown)
			}
		})
	}
	// And the difference is the one the caption promises.
	for _, c := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		cells := seedCells(c, tokens.DefaultSeed)
		if !cells[0].HandedIn || cells[1].HandedIn {
			t.Errorf("the smaller swatch is not the colour picked: handedIn is %v then %v",
				cells[0].HandedIn, cells[1].HandedIn)
		}
		if !strings.Contains(palette.SeedHint(cells), palette.SeedHintPair) {
			t.Error("the pair is drawn at two sizes and the caption does not say what the sizes mean")
		}
	}
	// A row with nothing to tell apart does not promise a difference.
	single := seedCells(tokens.DefaultDark, color.NRGBA{R: 0xff, A: 0xff})
	if strings.Contains(palette.SeedHint(single), palette.SeedHintPair) {
		t.Error("a one-cell row's caption points at a smaller swatch that is not drawn")
	}
}

// swatchBox is the rectangle a swatch covers inside the region handed
// in: everything there that is not the ground it stands on. The seed
// cells put nothing but a swatch in the column this is asked about, so
// the extent of what is not ground is the extent of the swatch.
func swatchBox(img image.Image, ground color.NRGBA, region image.Rectangle) image.Rectangle {
	box := image.Rectangle{}
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == ground.R && uint8(g>>8) == ground.G && uint8(b>>8) == ground.B {
				continue
			}
			at := image.Rect(x, y, x+1, y+1)
			if box.Empty() {
				box = at
			} else {
				box = box.Union(at)
			}
		}
	}
	return box
}

// grey is the luminance of a pixel, which is what is left of it to a
// reader the chroma is gone for.
func grey(c color.Color) int {
	r, g, b, _ := c.RGBA()
	return int((299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// natural is how wide a string wants to be, unconstrained by the room it
// is about to be given.
func natural(gtx layout.Context, shaper *text.Shaper, style textdraw.TextStyle, str string) int {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	return textdraw.MeasureText(gtx, shaper, style, str).X
}

// measuringContext is a context good for asking how wide a string wants
// to be and what [palette.FitLine] does with the room it has: one pixel
// to the dp, so a room in this test is a room on a default display.
func measuringContext() layout.Context {
	return layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
}

// TestSeedRowIsTheHeadOfTheStory pins the order the tab tells it in: the
// input first, then the ramps and picks made of it.
func TestSeedRowIsTheHeadOfTheStory(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	c := tokens.DefaultLight
	p, ty := PaletteFrom(c), TypeFrom(shaper, tokens.DefaultTypography)
	story := themePaletteRows(shaper, tokens.DefaultTypography, c, tokens.DefaultSeed)
	head := seedRows(p, c, ty, tokens.DefaultSeed)
	if len(head) != palette.SeedSectionRows {
		t.Fatalf("seedRows returns %d rows, palette.SeedSectionRows says %d", len(head), palette.SeedSectionRows)
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
