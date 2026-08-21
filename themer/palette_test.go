package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/tokens"
)

// columnBannerH is the band the embedded page puts over each group of its
// sections, which the palette section stands directly under. It is the
// inventory's number and not this window's, so it is stated here — where the
// pixel assertions below disagree with it loudly — rather than copied into the
// drawing code, which has no business knowing it.
const columnBannerH = 44

// markJitter is how far the middle of a drawn disc may sit from the colour it
// was filled with: the rasteriser's own last level or two of antialiasing, and
// nothing like the distance between a mark and the step under it.
const markJitter = 4

// The geometry of the palette section inside the window, from the same
// constants it lays out with. The section stands where the embedded page's own
// palette sections stood, which is the first thing under the foundations band.
func rampGridTop() int {
	return galleryTop() + columnBannerH + int(PaletteHeadH) + int(inventory.SectionPadY)
}

// rampLabelLeft is the x the ramp names are ranged against, and rampCellW the
// width one step cell takes when the grid has the whole of the panel's content
// width.
func rampLabelLeft() int { return int(Pad) + int(inventory.SectionPadX) }

func rampContentW() int { return windowW - 2*int(Pad) - 2*int(inventory.SectionPadX) }

func rampCellW() int {
	return min(int(RampCellMax), (rampContentW()-int(RampLabelW))/RampSteps)
}

// rampCellCentre is the middle of the cell holding step n+1 of ramp row i,
// which is where a claimed rung's mark is, and rampCellColour a point in the
// same cell that no mark reaches — a quarter of the way in, against a mark of
// six points in a cell of ninety-six.
func rampCellCentre(i, n int) image.Point {
	x := rampLabelLeft() + int(RampLabelW) + n*rampCellW() + rampCellW()/2
	y := rampGridTop() + int(RampHeadH) + i*int(RampRowH) + int(RampRowH)/2
	return image.Pt(x, y)
}

func rampCellColour(i, n int) image.Point {
	at := rampCellCentre(i, n)
	return image.Pt(at.X-rampCellW()/4, at.Y)
}

// seeded is a window showing a theme generated from one seed, which is what
// every assertion about the palette section is made against: the section
// describes a derivation, and a derivation needs something to derive from.
func seeded(t *testing.T) Model {
	t.Helper()
	m := contrasting(t)
	m.Selected = 1 // the blue, which puts every role a long way from the default
	return m
}

// derived is the pair the window resolves for a model, and the side it shows.
func derived(m Model, os tokens.ColorTokens) (shown, other tokens.ColorTokens) {
	return SchemePair(os, m)
}

// TestPaletteSectionRowsIsTheRowCount: the count the column is addressed by has
// to be the number of rows the section actually is, or the scroll that brings
// the code specimen into view lands short of it.
func TestPaletteSectionRowsIsTheRowCount(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureBlue)
	rows := PaletteRows(PaletteFrom(light), light, dark, pinned(), false)
	if len(rows) != PaletteSectionRows {
		t.Errorf("the palette section is %d rows and PaletteSectionRows says %d", len(rows), PaletteSectionRows)
	}
}

// TestTheWindowShowsOnePalette: the embedded page's own palette sections are
// not in this window's column. They are a definition of the roles; this window
// shows a specimen of one seed's, with where each colour came from, and a reader
// scrolled past both would have no way of telling which of the two answers the
// question they arrived with.
//
// It also asserts the two sections are still there to replace. A rename upstream
// takes them out of this list silently, and the window would go back to showing
// the palette twice with nothing failing.
func TestTheWindowShowsOnePalette(t *testing.T) {
	e := newEmbed()
	shaper := pinned().Shaper
	light, dark := tokens.FromSeed(fixtureBlue)
	palette := PaletteRows(PaletteFrom(light), light, dark, pinned(), false)
	column := e.items(shaper, light, highlight.DefaultBases(), palette)
	for _, name := range swapped {
		if row := e.inv.ItemIndex(light, name); row < 0 {
			t.Errorf("the embedded page has no section named %q to stand in the place of", name)
		}
	}
	at, rows := e.paletteSpan(light)
	if want := 2 * len(swapped); rows != want {
		t.Errorf("the sections replaced take %d rows, want a heading and a body each (%d)", rows, want)
	}
	if first := e.inv.ItemIndex(light, swapped[0]); at != first {
		t.Errorf("the swap starts at row %d, want the first replaced section's own row %d", at, first)
	}
	whole := e.inv.Items(light)
	if want := len(whole) - rows + len(palette); len(column) != want {
		t.Errorf("the column is %d rows against an inventory of %d: %d out, %d in makes %d",
			len(column), len(whole), rows, len(palette), want)
	}
	// And the code specimen is still found where it now stands, since the swap
	// moved every row behind it.
	if got, want := e.codeColumnRow(), e.codeRow()+len(palette)-rows; got != want {
		t.Errorf("the code specimen is addressed at row %d, want %d", got, want)
	}
}

// TestTheRampsAreEveryRampWithNeutralLast: eight rows, each the theme's own
// ramp, in the order the grid is read. Neutral last is the whole of the
// ordering decision — it is the row the seed has least to say about — and a
// change that quietly put it back on top would be a change nobody notices until
// they are looking at the wrong row.
func TestTheRampsAreEveryRampWithNeutralLast(t *testing.T) {
	c, _ := tokens.FromSeed(fixtureBlue)
	rows := rampRows(c)
	want := []struct {
		name string
		ramp tokens.Ramp
	}{
		{PrimaryName, c.Ramps.Primary},
		{SecondaryName, c.Ramps.Secondary},
		{TertiaryName, c.Ramps.Tertiary},
		{ErrorName, c.Ramps.Error},
		{SuccessName, c.Ramps.Success},
		{WarningName, c.Ramps.Warning},
		{InfoName, c.Ramps.Info},
		{NeutralName, c.Ramps.Neutral},
	}
	if len(rows) != len(want) {
		t.Fatalf("the grid has %d rows, want one per ramp (%d)", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].name != w.name {
			t.Errorf("row %d is %q, want %q", i, rows[i].name, w.name)
		}
		if rows[i].ramp != w.ramp {
			t.Errorf("row %q does not carry the theme's own %s ramp", rows[i].name, w.name)
		}
	}
}

// TestEveryColourTokenIsPicked: the listing is every colour the theme names,
// with no name invented and none left out.
//
// The token set is walked by reflection rather than written out again here.
// Restating it would mean the test agrees with the section because both were
// typed from the same list; walking the struct means a role added to the theme
// fails this until the window shows it, which is the only way a listing that
// claims to be complete stays complete.
func TestEveryColourTokenIsPicked(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		shown := map[string]stdcolor.NRGBA{}
		for _, g := range paletteGroups(sc.c, sc.other, sc.dark) {
			for _, cell := range g.cells {
				add := func(name string, col stdcolor.NRGBA) {
					if _, seen := shown[name]; seen {
						t.Errorf("%s: %s is listed twice", sc.name, name)
					}
					shown[name] = col
				}
				add(cell.base.name, cell.fill)
				if cell.paired() {
					add(cell.ink.name, cell.on)
				}
			}
		}
		v := reflect.ValueOf(sc.c)
		for i := range v.NumField() {
			name := v.Type().Field(i).Name
			if name == "Ramps" {
				continue
			}
			want := v.Field(i).Interface().(stdcolor.NRGBA)
			got, ok := shown[name]
			if !ok {
				t.Errorf("%s: the listing has no %s", sc.name, name)
				continue
			}
			if got != want {
				t.Errorf("%s: %s is shown as %v, want the theme's own %v", sc.name, name, got, want)
			}
			delete(shown, name)
		}
		for name := range shown {
			t.Errorf("%s: the listing shows %q, which is not a colour token", sc.name, name)
		}
	}
}

// TestABaseAndItsInkAreOneCell: the seven pinned roles, the page and its text,
// and the inverse pair are each one cell — one swatch, the ink written on it —
// because each is one decision. Surface and Divider stand alone, the theme
// naming no ink for either.
func TestABaseAndItsInkAreOneCell(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		want := map[string]string{
			BackgroundPick:     TextPick,
			InverseSurfacePick: OnInverseSurfacePick,
			PrimaryName:        "OnPrimary",
			SecondaryName:      "OnSecondary",
			TertiaryName:       "OnTertiary",
			ErrorName:          "OnError",
			SuccessName:        "OnSuccess",
			WarningName:        "OnWarning",
			InfoName:           "OnInfo",
		}
		alone := map[string]bool{SurfacePick: true, DividerPick: true}
		for _, g := range paletteGroups(sc.c, sc.other, sc.dark) {
			for _, cell := range g.cells {
				ink, paired := want[cell.base.name]
				switch {
				case paired && !cell.paired():
					t.Errorf("%s: %s is a swatch on its own, want %s written on it", sc.name, cell.base.name, ink)
				case paired && cell.ink.name != ink:
					t.Errorf("%s: %s carries %s, want %s", sc.name, cell.base.name, cell.ink.name, ink)
				case alone[cell.base.name] && cell.paired():
					t.Errorf("%s: %s carries an ink, and the theme names none for it", sc.name, cell.base.name)
				case !paired && !alone[cell.base.name]:
					t.Errorf("%s: %s is a cell nothing accounts for", sc.name, cell.base.name)
				}
				// The title names both members, in the order their rules are
				// written under them.
				if cell.paired() && cell.title() != cell.base.name+PickPairSep+cell.ink.name {
					t.Errorf("%s: the cell is titled %q, want both names in the order the rules are in", sc.name, cell.title())
				}
				delete(want, cell.base.name)
				delete(alone, cell.base.name)
			}
		}
		for name := range want {
			t.Errorf("%s: no cell carries %s", sc.name, name)
		}
		for name := range alone {
			t.Errorf("%s: no cell carries %s", sc.name, name)
		}
	}
}

// TestPickRulesNameWhereTheColourCameFrom: every rule is read off the colours,
// so every rule has to be the truth about them.
//
// The claims asserted here are the ones a reader would otherwise have to take
// on trust: the neutral resolutions, the status pins that are their own ramp's
// 700 in both schemes, the light primary that is on no ramp at all, and the
// light secondary and tertiary that are near a rung without being on it.
func TestPickRulesNameWhereTheColourCameFrom(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureBlue)
	want := map[string]map[string]string{
		"light": {
			BackgroundPick: "Neutral 100",
			TextPick:       "Neutral 900",
			SurfacePick:    "Neutral 200",
			DividerPick:    "Neutral 300",
			PrimaryName:    fmt.Sprintf(PickSeedNear, PrimaryName, 700),
			SecondaryName:  fmt.Sprintf(PickJustOff, SecondaryName, 700),
			TertiaryName:   fmt.Sprintf(PickJustOff, TertiaryName, 700),
			ErrorName:      "Error 700",
			SuccessName:    "Success 700",
			WarningName:    "Warning 700",
			InfoName:       "Info 700",
		},
		"dark": {
			BackgroundPick: "Neutral 100",
			TextPick:       "Neutral 900",
			SurfacePick:    "Neutral 200",
			DividerPick:    "Neutral 300",
			PrimaryName:    "Primary 700",
			SecondaryName:  "Secondary 700",
			TertiaryName:   "Tertiary 700",
			ErrorName:      "Error 700",
		},
	}
	for _, sc := range []struct {
		name        string
		c, other    tokens.ColorTokens
		dark        bool
		inverseSide string
	}{
		{"light", light, dark, false, "the dark scheme's"},
		{"dark", dark, light, true, "the light scheme's"},
	} {
		rules := rulesOf(paletteGroups(sc.c, sc.other, sc.dark))
		for name, rule := range want[sc.name] {
			if rules[name] != rule {
				t.Errorf("%s: %s says %q, want %q", sc.name, name, rules[name], rule)
			}
		}
		// The inverse pair names the other side by name, and names the role it
		// is over there rather than a step of the ramp on this side.
		if got, w := rules[InverseSurfacePick], sc.inverseSide+" "+PickSurfaceRole; got != w {
			t.Errorf("%s: %s says %q, want %q", sc.name, InverseSurfacePick, got, w)
		}
		if got, w := rules[OnInverseSurfacePick], sc.inverseSide+" "+PickTextRole; got != w {
			t.Errorf("%s: %s says %q, want %q", sc.name, OnInverseSurfacePick, got, w)
		}
		// Every ink says it was measured, because every one of them was, and
		// that is the half of the answer the colour alone does not give. An ink
		// whose own name says which role the cell is about — a rung of that
		// role's ramp — may say "the base"; one that does not, white and black
		// belonging to no role, has to name it, or a light scheme's seven cells
		// carry one sentence between them.
		for _, role := range []string{PrimaryName, SecondaryName, TertiaryName,
			ErrorName, SuccessName, WarningName, InfoName} {
			rule := rules["On"+role]
			named := strings.HasSuffix(rule, fmt.Sprintf(PickMeasuredOver, role))
			switch {
			case !strings.Contains(rule, "measured over"):
				t.Errorf("%s: On%s says %q, which does not say what it was measured over", sc.name, role, rule)
			case !named && !strings.HasPrefix(rule, role+" "):
				t.Errorf("%s: On%s says %q, which names neither the base nor its own role", sc.name, role, rule)
			}
		}
		// No two inks may say the same thing, which is what naming the base
		// buys: it is the difference between seven rules and one repeated.
		said := map[string]string{}
		for _, role := range []string{PrimaryName, SecondaryName, TertiaryName,
			ErrorName, SuccessName, WarningName, InfoName} {
			rule := rules["On"+role]
			if first, seen := said[rule]; seen {
				t.Errorf("%s: On%s and On%s both say %q", sc.name, first, role, rule)
			}
			said[rule] = role
		}
		// And an ink's rule names what was actually kept: one of the two ends
		// of the tonal axis, or the role's own deepest rung.
		if got := rules["OnPrimary"]; sc.dark {
			if !strings.HasPrefix(got, PrimaryName+" ") && !strings.HasPrefix(got, PickWhite) {
				t.Errorf("dark: OnPrimary says %q, want its own ramp's step or white", got)
			}
		} else if !strings.HasPrefix(got, PickWhite) && !strings.HasPrefix(got, PickBlack) {
			t.Errorf("light: OnPrimary says %q, want white or black", got)
		}
	}
}

// TestNoRuleIsEmpty: a colour with no rule under it is a swatch with a name, and
// a swatch with a name is what this section was added to stop being the whole
// of what the window says about its palette.
func TestNoRuleIsEmpty(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		for _, g := range paletteGroups(sc.c, sc.other, sc.dark) {
			if g.name == "" {
				t.Errorf("%s: a family of cells has no name", sc.name)
			}
			for _, cell := range g.cells {
				for _, part := range []pickPart{cell.base, cell.ink} {
					if part.name == "" {
						continue
					}
					if part.rule == "" {
						t.Errorf("%s: %s has no rule under it", sc.name, part.name)
					}
				}
				if cell.fill.A == 0 {
					t.Errorf("%s: %s draws a transparent swatch", sc.name, cell.base.name)
				}
				if cell.paired() && cell.on.A == 0 {
					t.Errorf("%s: %s is written in a transparent ink", sc.name, cell.ink.name)
				}
			}
		}
	}
}

// TestAClaimedRungIsTheRuleItNames: the mark on the grid and the rule under the
// pick are resolved from one answer, so a cell marked is a cell some rule names
// and no rule names a cell that is not marked.
func TestAClaimedRungIsTheRuleItNames(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		groups := paletteGroups(sc.c, sc.other, sc.dark)
		claims := rampClaims(groups)
		for claim := range claims {
			if claim.step%100 != 0 || claim.step < 100 || claim.step > 900 {
				t.Errorf("%s: %s claims step %d, which is not a rung", sc.name, claim.role, claim.step)
			}
		}
		for _, g := range groups {
			for _, cell := range g.cells {
				for _, part := range []pickPart{cell.base, cell.ink} {
					if part.step == 0 {
						continue
					}
					// A rule names the rung its mark is on, whether the colour
					// is that rung or is merely indistinguishable from it.
					want := fmt.Sprintf("%s %d", part.role, part.step)
					if !strings.Contains(part.rule, want) {
						t.Errorf("%s: %s marks %s and its rule says %q", sc.name, part.name, want, part.rule)
					}
					if !claims[rampClaim{part.role, part.step}] {
						t.Errorf("%s: %s names %s and the grid marks nothing there", sc.name, part.name, want)
					}
				}
			}
		}
		// A light scheme's inks are the ends of the tonal axis, which are on no
		// ramp, so they claim nothing; a dark scheme's come off their own role's
		// ramp and claim its deepest rung. That difference is the derivation
		// being visible, and it is worth failing if it stops being true.
		rules := rulesOf(groups)
		for _, role := range []string{PrimaryName, ErrorName, InfoName} {
			ink := rules["On"+role]
			if sc.dark && !strings.HasPrefix(ink, role+" ") {
				t.Errorf("%s: On%s says %q, want a rung of its own ramp", sc.name, role, ink)
			}
			if !sc.dark && !strings.HasPrefix(ink, PickWhite) && !strings.HasPrefix(ink, PickBlack) {
				t.Errorf("%s: On%s says %q, want an end of the tonal axis", sc.name, role, ink)
			}
		}
	}
}

// TestTheRungToleranceStandsBetweenItsTwoMeasurements: the tolerance that
// decides whether a pick sits at a rung is set by measurement, and this is the
// measurement.
//
// Below it, every pin that ought to match its rung has to match: the light
// accents are pinned a unit of lightness off their own 700 and a reader looking
// at the grid sees them on it. Above it, no rung may be within reach of another,
// or a mark would be ambiguous about which one it means — and worse, a colour
// could be marked at two rungs at once.
func TestTheRungToleranceStandsBetweenItsTwoMeasurements(t *testing.T) {
	worstMatch, closestRungs := 0.0, 1.0
	for _, seed := range []stdcolor.NRGBA{fixtureBlue, fixtureRed, fixtureGrey, tokens.DefaultSeed} {
		light, dark := tokens.FromSeed(seed)
		for _, sc := range []tokens.ColorTokens{light, dark} {
			for _, pin := range []struct {
				name string
				col  stdcolor.NRGBA
				ramp tokens.Ramp
			}{
				{PrimaryName, sc.Primary, sc.Ramps.Primary},
				{SecondaryName, sc.Secondary, sc.Ramps.Secondary},
				{TertiaryName, sc.Tertiary, sc.Ramps.Tertiary},
				{ErrorName, sc.Error, sc.Ramps.Error},
				{SuccessName, sc.Success, sc.Ramps.Success},
				{WarningName, sc.Warning, sc.Ramps.Warning},
				{InfoName, sc.Info, sc.Ramps.Info},
			} {
				step := nearestStep(pin.ramp, pin.col)
				if step == 0 {
					t.Errorf("%s: the %s pin sits at no rung the grid can mark", hexOf(seed), pin.name)
					continue
				}
				worstMatch = max(worstMatch, oklabDistance(pin.ramp.Step(step), pin.col))
			}
			for _, r := range rampRows(sc) {
				for n := range RampSteps - 1 {
					closestRungs = min(closestRungs,
						oklabDistance(r.ramp.Step((n+1)*100), r.ramp.Step((n+2)*100)))
				}
			}
		}
	}
	t.Logf("the worst pin sits %.4f from its rung; the closest two rungs are %.4f apart; the tolerance is %.4f",
		worstMatch, closestRungs, rungTolerance)
	if worstMatch >= rungTolerance {
		t.Errorf("a pin sits %.4f from its own rung and the tolerance is %.4f — a pick the grid should mark goes unmarked",
			worstMatch, rungTolerance)
	}
	// Two rungs a whole tolerance apart on either side of one colour is the
	// case that would let a colour be within reach of both, so the gap has to
	// beat twice the tolerance rather than merely exceed it.
	if closestRungs <= 2*rungTolerance {
		t.Errorf("two rungs stand %.4f apart against a tolerance of %.4f — a mark cannot say which rung it means",
			closestRungs, rungTolerance)
	}
}

// TestTheGridDrawsTheThemesOwnRampSteps: every cell of the grid is the colour
// the theme's own ramp holds at that step, read off the render.
//
// It is asserted from pixels and not from the values the grid was built with,
// because a grid that resolved the right colours and drew the wrong ones — a row
// off by one, a step read from the ramp beside it — would pass every assertion
// made anywhere else in this file.
func TestTheGridDrawsTheThemesOwnRampSteps(t *testing.T) {
	m := seeded(t)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		c, _ := derived(m, os)
		img := page(t, m, os)
		for i, row := range rampRows(c) {
			for n := range RampSteps {
				at := rampCellColour(i, n)
				want := row.ramp.Step((n + 1) * 100)
				got := img.RGBAAt(at.X, at.Y)
				if got.R != want.R || got.G != want.G || got.B != want.B {
					t.Errorf("%s %d at %v drew %v, want the ramp's %v",
						row.name, (n+1)*100, at, got, want)
				}
			}
		}
	}
}

// TestTheGridMarksTheRungsThePicksTook: the two halves of the section point at
// each other. A rung some pick took carries a mark in the middle of its cell,
// every other cell is the colour and nothing else, and the mark's ink is
// measured over the step it stands on — the same pair of candidates the
// derivation itself chooses an on-colour from, because it is the same job.
func TestTheGridMarksTheRungsThePicksTook(t *testing.T) {
	m := seeded(t)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		c, other := derived(m, os)
		claims := rampClaims(paletteGroups(c, other, m.Dark(os)))
		if len(claims) == 0 {
			t.Fatal("no pick claims a rung, so the grid marks nothing")
		}
		img := page(t, m, os)
		for i, row := range rampRows(c) {
			for n := range RampSteps {
				at := rampCellCentre(i, n)
				step := row.ramp.Step((n + 1) * 100)
				marked := claims[rampClaim{row.name, (n + 1) * 100}]
				want := step
				if marked {
					want = markInkOn(step)
				}
				got := img.RGBAAt(at.X, at.Y)
				// A mark is a disc, so its own middle carries the last level or
				// two of the rasteriser's antialiasing; a cell that carries none
				// is a flat fill and is compared exactly. The tolerance is far
				// under the distance between a step and the end of the axis
				// chosen to stand out on it, which is what is being asserted.
				off := max(apart(got.R, want.R), max(apart(got.G, want.G), apart(got.B, want.B)))
				if (marked && off > markJitter) || (!marked && off != 0) {
					t.Errorf("%s %d at %v drew %v, want %v", row.name, (n+1)*100, at, got, want)
				}
			}
		}
	}
}

// TestThePaletteSectionFollowsTheSchemeSwitch: the grid is drawn from the side
// of the pair the window is showing, so pressing the switch redraws it. A
// section resolved once and kept would go on showing the palette nobody is
// looking at.
func TestThePaletteSectionFollowsTheSchemeSwitch(t *testing.T) {
	m := seeded(t)
	e := newEmbed()
	lightImg := pageOn(t, e, ReduceModel(m, SetScheme{Dark: false}), tokens.DefaultLight)
	darkImg := pageOn(t, e, ReduceModel(m, SetScheme{Dark: true}), tokens.DefaultLight)
	c, _ := derived(ReduceModel(m, SetScheme{Dark: true}), tokens.DefaultLight)
	for i, row := range rampRows(c) {
		at := rampCellColour(i, RampSteps-1)
		want := row.ramp.Step(RampSteps * 100)
		got := darkImg.RGBAAt(at.X, at.Y)
		if got.R != want.R || got.G != want.G || got.B != want.B {
			t.Errorf("with the switch on dark, %s 900 drew %v, want the dark ramp's %v", row.name, got, want)
		}
	}
	if moved := bandChange(lightImg, darkImg, rampGridTop(), rampGridTop()+int(RampHeadH)+8*int(RampRowH)); moved < schemeBandFloor {
		t.Errorf("the switch moved %.2f%% of the grid, want the whole of it", moved)
	}
}

// TestThePicksSpreadOverAWideWindowAndStackOnANarrowOne: the board is as many
// columns as the window is wide enough for, and never one so narrow that the
// rule under a name is an ellipsis.
func TestThePicksSpreadOverAWideWindowAndStackOnANarrowOne(t *testing.T) {
	gap, narrowest := int(PickColGap), int(PickMinW)
	for _, tc := range []struct{ width, want int }{
		{rampContentW(), PickMaxCols},
		{2*narrowest + gap, 2},
		{narrowest, 1},
		{narrowest - 1, 1},
		{0, 1},
	} {
		if got := pickColumns(tc.width, gap, narrowest); got != tc.want {
			t.Errorf("a board %d wide took %d columns, want %d", tc.width, got, tc.want)
		}
	}
}

// TestTheFamiliesAreDealtWholeInOrderAndEvenly: no family is split across a
// column boundary, every family is dealt exactly once, no family overtakes
// another, and the tallest column is as short as an in-order deal can make it.
//
// The order matters as much as the balance. A deal free to move a family to a
// column of its own choosing gives the same board two reading orders at two
// window widths, and a reader dragging the window watches a family change
// neighbours — which is the one thing a board of labelled families must not do.
func TestTheFamiliesAreDealtWholeInOrderAndEvenly(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureBlue)
	groups := paletteGroups(light, dark, false)
	total := 0
	for _, g := range groups {
		total += pickLoad(g)
	}
	for n := 1; n <= PickMaxCols; n++ {
		cols := packPicks(groups, n)
		if len(cols) != n {
			t.Fatalf("%d columns asked for, %d dealt", n, len(cols))
		}
		read, tallest := []string{}, 0
		for _, col := range cols {
			load := 0
			for _, g := range col {
				read = append(read, g.name)
				load += pickLoad(g)
			}
			tallest = max(tallest, load)
		}
		if len(read) != len(groups) {
			t.Errorf("%d columns hold %d families, want all %d", n, len(read), len(groups))
		}
		for i, g := range groups {
			if i < len(read) && read[i] != g.name {
				t.Errorf("%d columns: read in order the board says %v, want the families in their own order", n, read)
				break
			}
		}
		if tallest > best(groups, n) {
			t.Errorf("%d columns: the tallest stands %d against a best in-order deal of %d", n, tallest, best(groups, n))
		}
	}
}

// best is the shortest tallest column any in-order deal of these families into
// n columns achieves, found by trying every run of boundaries.
func best(groups []pickGroup, n int) int {
	if n <= 1 || len(groups) == 0 {
		total := 0
		for _, g := range groups {
			total += pickLoad(g)
		}
		return total
	}
	shortest := -1
	var walk func(from, left, tallest int)
	walk = func(from, left, tallest int) {
		if left == 1 {
			load := 0
			for _, g := range groups[from:] {
				load += pickLoad(g)
			}
			if got := max(tallest, load); shortest < 0 || got < shortest {
				shortest = got
			}
			return
		}
		load := 0
		for cut := from; cut <= len(groups); cut++ {
			walk(cut, left-1, max(tallest, load))
			if cut < len(groups) {
				load += pickLoad(groups[cut])
			}
		}
	}
	walk(0, n, 0)
	return shortest
}

// TestTheInverseFamilyKeepsItsNeighbourAtEveryWidth: the inverse pair is two
// surfaces borrowed from the other side of the scheme, and it stays beside the
// window's own surfaces however many columns the board is dealt into. A family
// that changes neighbours with the window's width is a family a reader has to
// find again after every resize.
func TestTheInverseFamilyKeepsItsNeighbourAtEveryWidth(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureBlue)
	groups := paletteGroups(light, dark, false)
	for n := 1; n <= PickMaxCols; n++ {
		for _, col := range packPicks(groups, n) {
			for i, g := range col {
				if g.name != PickInverseGroup {
					continue
				}
				if i == 0 || col[i-1].name != PickPageGroup {
					t.Errorf("%d columns: %s stands under %v, want it under %s",
						n, PickInverseGroup, names(col), PickPageGroup)
				}
			}
		}
	}
}

func names(col []pickGroup) []string {
	out := make([]string, len(col))
	for i, g := range col {
		out[i] = g.name
	}
	return out
}

// rulesOf is every cell's rules by the name they stand under.
func rulesOf(groups []pickGroup) map[string]string {
	out := map[string]string{}
	for _, g := range groups {
		for _, cell := range g.cells {
			out[cell.base.name] = cell.base.rule
			if cell.paired() {
				out[cell.ink.name] = cell.ink.rule
			}
		}
	}
	return out
}

// schemesUnderTest is the pair a palette assertion is made against, each side
// with the counterpart the inverse rules are read from.
func schemesUnderTest(t *testing.T) []struct {
	name     string
	c, other tokens.ColorTokens
	dark     bool
} {
	t.Helper()
	out := []struct {
		name     string
		c, other tokens.ColorTokens
		dark     bool
	}{}
	for _, seed := range []stdcolor.NRGBA{fixtureBlue, fixtureRed, fixtureGrey, tokens.DefaultSeed} {
		light, dark := tokens.FromSeed(seed)
		out = append(out,
			struct {
				name     string
				c, other tokens.ColorTokens
				dark     bool
			}{fmt.Sprintf("%s light", hexOf(seed)), light, dark, false},
			struct {
				name     string
				c, other tokens.ColorTokens
				dark     bool
			}{fmt.Sprintf("%s dark", hexOf(seed)), dark, light, true},
		)
	}
	return out
}
