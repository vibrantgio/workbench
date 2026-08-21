package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
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

// rampCellW is the width one step cell takes when the grid has the whole of the
// panel's content width, which is what is left of it once the names at one end
// and the chips at the other have been reserved.
func rampCellW() int {
	return (rampContentW() - int(RampLabelW) - int(RampPinGap) - int(RampPinW)) / RampSteps
}

// rampPinCentre is the middle of the chip at the end of row i, which is where
// that role's pinned base is drawn. The chips are ranged against the grid's
// trailing edge, so the middle of one is half a chip in from the far side of
// the content the section's body is laid out in — not one gap past step 900,
// which is where they would stand if the cells were the thing setting the
// grid's width.
func rampPinCentre(i int) image.Point {
	x := rampLabelLeft() + rampContentW() - int(RampPinW)/2
	y := rampGridTop() + int(RampHeadH) + i*int(RampRowH) + int(RampRowH)/2
	return image.Pt(x, y)
}

// rampCellCentre is the middle of the cell holding step n+1 of ramp row i,
// which is where a claimed rung's mark is, and rampCellColour a point in the
// same cell that no mark reaches — a quarter of the way in, against a mark of
// six points in a cell of ninety.
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
		// And the colours the theme publishes without a field to hold them: the
		// two ends of the tonal axis, which are package colours, and the four
		// status containers with their marks, which the theme derives from a role
		// when it is asked. They are checked the way the fields are — the listing
		// has to carry each, at the theme's own value — because a colour a widget
		// is painted with at rest is a colour this window claims to show, and
		// whether the theme keeps it in a struct is not the reader's problem.
		for name, want := range publishedBeyondTheFields(sc.c) {
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

// publishedBeyondTheFields is every resting colour the theme publishes that is
// not a field of ColorTokens, by the name the listing carries it under.
func publishedBeyondTheFields(c tokens.ColorTokens) map[string]stdcolor.NRGBA {
	out := map[string]stdcolor.NRGBA{WhitePick: tokens.White, BlackPick: tokens.Black}
	for _, r := range statusRoles() {
		out[r.name+ContainerPick] = c.StatusContainer(r.id)
		out[r.name+MarkPick] = c.OnStatusContainer(r.id)
	}
	return out
}

// statusRoles is the four roles that publish a container, each with the way to
// reach its own ramp.
func statusRoles() []struct {
	name string
	id   tokens.Role
	ramp func(tokens.ColorTokens) tokens.Ramp
} {
	return []struct {
		name string
		id   tokens.Role
		ramp func(tokens.ColorTokens) tokens.Ramp
	}{
		{ErrorName, tokens.RoleError, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Error }},
		{SuccessName, tokens.RoleSuccess, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Success }},
		{WarningName, tokens.RoleWarning, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Warning }},
		{InfoName, tokens.RoleInfo, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Info }},
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
		// A container and the mark read on it are one cell for the reason a base
		// and its ink are: the mark was measured over that exact ground.
		for _, r := range statusRoles() {
			want[r.name+ContainerPick] = r.name + MarkPick
		}
		// The two ends of the axis stand alone. They are what an ink turned out
		// to be, not a ground anything is written on, and writing letters on
		// either would be this section inventing a pairing the theme never made.
		alone := map[string]bool{
			SurfacePick: true, DividerPick: true,
			WhitePick: true, BlackPick: true,
		}
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

// TestTheAxisEndsSayWhetherThisSchemeWritesInThem: the two ends of the tonal
// axis are on no ramp, so the only thing that puts either in the picture is its
// own cell, and the only thing that makes the cell worth reading is whether the
// scheme on screen turned out to use it.
//
// The answer is read off the board's own inks rather than written down, and it
// is not the same answer on both sides: a light scheme writes almost every ink
// in white, and a dark scheme takes every ink off the role's own ramp and
// writes in neither end.
func TestTheAxisEndsSayWhetherThisSchemeWritesInThem(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		groups := paletteGroups(sc.c, sc.other, sc.dark)
		rules := rulesOf(groups)
		for _, end := range []struct {
			name, text string
			col        stdcolor.NRGBA
		}{
			{WhitePick, PickAxisLight, tokens.White},
			{BlackPick, PickAxisDark, tokens.Black},
		} {
			used := false
			for _, g := range groups {
				for _, cell := range g.cells {
					if cell.paired() && cell.on == end.col {
						used = true
					}
				}
			}
			want := fmt.Sprintf(PickAxisNoInk, end.text)
			if used {
				want = fmt.Sprintf(PickAxisInk, end.text)
			}
			if got := rules[end.name]; got != want {
				t.Errorf("%s: %s says %q, want %q", sc.name, end.name, got, want)
			}
		}
		// And the case the rule exists for: a light scheme does write in white
		// and a dark scheme writes in neither end, so a rule that said one thing
		// on both sides would be saying nothing on either.
		if !sc.dark {
			if got, want := rules[WhitePick], fmt.Sprintf(PickAxisInk, PickAxisLight); got != want {
				t.Errorf("%s: %s says %q, and this scheme's inks are white", sc.name, WhitePick, got)
			}
		}
	}
}

// markGraphicFloor is what a mark on the grid owes the step it stands on: WCAG
// 1.4.11's 3:1 for a non-text graphic, which is what a dot is.
const markGraphicFloor = 3.0

// TestEveryMarkOnTheGridReadsOnTheStepItStandsOn: a marker nobody can see is
// not a marker, and the grid puts them on seventy-two possible grounds running
// from the page itself to nearly black.
//
// The ink is chosen by measuring both ends of the tonal axis over the step and
// keeping the better, and this is why it cannot be chosen by asking whether the
// step is dark instead: the mid rungs of a saturated hue sit under half the
// luminance scale and still take black far better than white — a light red at a
// third of the scale reads at 2.7:1 in white and 7.8:1 in black. Nothing marked
// those rungs until the status containers named their marks, so the question
// never came up; it comes up now, on four cells of every dark scheme.
func TestEveryMarkOnTheGridReadsOnTheStepItStandsOn(t *testing.T) {
	worst, at := math.Inf(1), ""
	for _, sc := range schemesUnderTest(t) {
		claims := rampClaims(paletteGroups(sc.c, sc.other, sc.dark))
		for _, row := range rampRows(sc.c) {
			for n := range RampSteps {
				if !claims[rampClaim{row.name, (n + 1) * 100}] {
					continue
				}
				step := row.ramp.Step((n + 1) * 100)
				if got := vgcolor.ContrastRatio(markInkOn(step), step); got < worst {
					worst, at = got, fmt.Sprintf("%s %s %d", sc.name, row.name, (n+1)*100)
				}
			}
		}
	}
	t.Logf("the faintest mark on the grid reads at %.2f:1, on %s", worst, at)
	if worst < markGraphicFloor {
		t.Errorf("a mark reads at %.2f:1 over its own step (%s), under the %.1f:1 a graphic owes its ground",
			worst, at, markGraphicFloor)
	}
}

// TestNoSwatchEdgeIsItsOwnFill: every frame in this section is a colour the
// thing inside it is not, and reads against it.
//
// The grid's frame was the neutral ramp's 400 for every one of its cells, and
// the grid draws that ramp: the Neutral row's own 400 cell was framed in itself,
// byte for byte, so it had no edge at all — and the whole 400 column sits at
// that step's luminance, which put the other seven rows at 1.00:1 as well. The
// frame is measured over the fill now, and what this holds is both halves of
// that: never the fill itself, and never so near it that the edge is a claim
// only a colour picker can check.
func TestNoSwatchEdgeIsItsOwnFill(t *testing.T) {
	worst, at := math.Inf(1), ""
	// Per side as well as over the pair, because the two sides are two
	// different questions — the frames are chosen off one lightness scale but
	// the fills sit on opposite ends of it — and the numbers are what the
	// drawing code's own account of this choice cites.
	side := map[bool]float64{false: math.Inf(1), true: math.Inf(1)}
	for _, sc := range schemesUnderTest(t) {
		fills := map[string]stdcolor.NRGBA{}
		for _, row := range rampRows(sc.c) {
			for n := range RampSteps {
				fills[fmt.Sprintf("%s %d", row.name, (n+1)*100)] = row.ramp.Step((n + 1) * 100)
			}
			if row.pin.A != 0 {
				fills[row.name+" base"] = row.pin
			}
		}
		for _, g := range paletteGroups(sc.c, sc.other, sc.dark) {
			for _, cell := range g.cells {
				fills[cell.title()] = cell.fill
			}
		}
		for name, fill := range fills {
			edge := edgeOn(sc.c, fill)
			if edge == fill {
				t.Errorf("%s: %s is framed in itself (%v), which is a swatch with no edge", sc.name, name, fill)
			}
			got := vgcolor.ContrastRatio(edge, fill)
			side[sc.dark] = math.Min(side[sc.dark], got)
			if got < worst {
				worst, at = got, sc.name+" "+name
			}
		}
	}
	t.Logf("the faintest edge in the section reads at %.2f:1, on %s — %.2f:1 the faintest of any light scheme, %.2f:1 of any dark one",
		worst, at, side[false], side[true])
	if worst < markGraphicFloor {
		t.Errorf("a swatch's edge reads at %.2f:1 over its own fill (%s), under the %.1f:1 a graphic owes its ground",
			worst, at, markGraphicFloor)
	}
}

// oldEdgeStep is the step of the neutral ramp the section's frames used to be
// drawn in, and so the one cell on the grid that was framed in its own colour.
const oldEdgeStep = 400

// TestTheNeutralRowsOwnEdgeStepIsDrawnWithAnEdge: read off a render, the cell
// this whole change is about — the step the old frame was the colour of — has a
// boundary between its fill and the row it stands in.
func TestTheNeutralRowsOwnEdgeStepIsDrawnWithAnEdge(t *testing.T) {
	for _, sc := range schemesUnderTest(t)[:4] {
		img := paletteSectionW(t, sectionWidths[0], sc.c, sc.other, sc.dark)
		rows := rampRows(sc.c)
		row := -1
		for i, r := range rows {
			if r.name == NeutralName {
				row = i
			}
		}
		if row < 0 {
			t.Fatalf("%s: no %s row on the grid", sc.name, NeutralName)
		}
		// The old frame was Neutral 400, so its cell is where an edge either
		// exists or does not. The scan crosses the boundary between that cell
		// and the one before it, a point below the cell's top so it is the
		// vertical edge being read and not the row's own gap.
		fill := sc.c.Ramps.Neutral.Step(oldEdgeStep)
		mid := sectionRowY(row)
		cell := sectionCellX(sectionWidths[0], oldEdgeStep/100-1)
		seen := stdcolor.NRGBA{}
		for x := cell - 3; x <= cell+1; x++ {
			if got := pixelAt(img, x, mid); got != fill {
				seen = got
			}
		}
		if seen == (stdcolor.NRGBA{}) {
			t.Errorf("%s: nothing but the fill %v either side of the %s %d cell's leading edge — the cell has no edge",
				sc.name, fill, NeutralName, oldEdgeStep)
			continue
		}
		if got := vgcolor.ContrastRatio(seen, fill); got < markGraphicFloor {
			t.Errorf("%s: the %s %d cell's edge drew %v over %v, which reads at %.2f:1",
				sc.name, NeutralName, oldEdgeStep, seen, fill, got)
		} else {
			t.Logf("%s: the %s %d cell's edge drew %v over its fill %v at %.2f:1",
				sc.name, NeutralName, oldEdgeStep, seen, fill, got)
		}
	}
}

// sectionCellX is the leading edge of the n-th step cell inside a
// [paletteSectionW] capture of the given width.
func sectionCellX(width, n int) int {
	inset := sectionInset()
	content := width - 2*inset
	cellW := (content - int(RampLabelW) - int(RampPinGap) - int(RampPinW)) / RampSteps
	return inset + int(RampLabelW) + n*cellW
}

// TestEveryPinnedBaseStandsAtTheEndOfItsOwnRow: the colour a role was actually
// pinned is drawn beside the nine it was pinned against, read off the render.
//
// This is the whole of what the grid was missing. A light scheme's Primary is
// the chosen seed at the seed's own depth and its Secondary and Tertiary are
// pinned off their own 700, so a grid of rungs alone showed nine colours a role
// might have been and not the one it is — and the seed, which is the colour the
// window exists to judge, was in the window nowhere.
func TestEveryPinnedBaseStandsAtTheEndOfItsOwnRow(t *testing.T) {
	m := seeded(t)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		c, _ := derived(m, os)
		img := page(t, m, os)
		for i, row := range rampRows(c) {
			at := rampPinCentre(i)
			got := img.RGBAAt(at.X, at.Y)
			if row.pin.A == 0 {
				// Neutral pins no solid fill, so its slot carries the mark that
				// says so and no chip: something is drawn in the middle of it,
				// and the slot around that something is the section's own
				// ground rather than a colour standing in for a pin.
				if got.R == c.Background.R && got.G == c.Background.G && got.B == c.Background.B {
					t.Errorf("%s pins nothing and its slot at %v is empty, want the mark that says so", row.name, at)
				}
				off := image.Pt(at.X-int(RampPinW)/3, at.Y)
				beside := img.RGBAAt(off.X, off.Y)
				if want := c.Background; beside.R != want.R || beside.G != want.G || beside.B != want.B {
					t.Errorf("%s pins nothing and its slot at %v drew %v, want the ground %v",
						row.name, off, beside, want)
				}
				continue
			}
			if got.R != row.pin.R || got.G != row.pin.G || got.B != row.pin.B {
				t.Errorf("%s's chip at %v drew %v, want the pinned base %v",
					row.name, at, got, row.pin)
			}
		}
		// And the seed itself, which is the first chip in the grid on the side
		// that pins it: a light scheme's Primary is the seed, lifted.
		if !m.Dark(os) {
			at := rampPinCentre(0)
			got := img.RGBAAt(at.X, at.Y)
			if want := c.Primary; got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("the first chip drew %v, want the lifted seed %v", got, want)
			}
		}
	}
}

// TestEachContainerIsItsRungHeldAtLessChroma: the rule under a status container
// says which rung it was realized at and what was done to that rung, and both
// halves are checked against the colour itself.
//
// The rung is named by tone because tone is what a container keeps: it gives up
// chroma, so no comparison of colours finds the cell it came from. Rebuilding
// the container out of the named rung's tone and hue at the container's own
// chroma has to produce the container back, to within the byte the chroma was
// rounded into on the way out, or the rule names a rung the derivation did not
// use.
func TestEachContainerIsItsRungHeldAtLessChroma(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		rules := rulesOf(paletteGroups(sc.c, sc.other, sc.dark))
		for _, r := range statusRoles() {
			ramp := r.ramp(sc.c)
			ground := sc.c.StatusContainer(r.id)
			step := toneStep(ramp, ground)
			rung := ramp.Step(step)
			tone, _, _ := vgcolor.LabFromNRGBA(rung)
			_, chroma, hue := vgcolor.OKLChFromNRGBA(rung)
			_, held, _ := vgcolor.OKLChFromNRGBA(ground)
			// Within a part in 255 a channel: the chroma the container is
			// rebuilt at is read back out of eight bits a channel, so the last
			// bit of it was rounded away before this test could ask for it.
			got := vgcolor.NRGBAFromToneChromaHue(tone, held, hue)
			if off := max(apart(got.R, ground.R), max(apart(got.G, ground.G), apart(got.B, ground.B))); off > 1 {
				t.Errorf("%s: the %s container is %v, and %s %d's tone and hue at that chroma is %v",
					sc.name, r.name, ground, r.name, step, got)
			}
			if held >= chroma {
				t.Errorf("%s: the %s container carries chroma %.4f against its rung's %.4f, and the rule says it was pulled down",
					sc.name, r.name, held, chroma)
			}
			if got, want := rules[r.name+ContainerPick], fmt.Sprintf(PickContainerRule, r.name, step); got != want {
				t.Errorf("%s: %s says %q, want %q", sc.name, r.name+ContainerPick, got, want)
			}
			// And the mark on it is a rung of the role's own ramp, named as one.
			mark := sc.c.OnStatusContainer(r.id)
			n := stepIn(ramp, mark)
			if n == 0 {
				t.Errorf("%s: the %s mark %v is on no rung of its own ramp", sc.name, r.name, mark)
				continue
			}
			if got, want := rules[r.name+MarkPick], fmt.Sprintf(PickMarkRule, r.name, n); got != want {
				t.Errorf("%s: %s says %q, want %q", sc.name, r.name+MarkPick, got, want)
			}
		}
	}
}

// TestTheContainersToneNamesOneRungAndNoOther: the tone a container shares with
// its rung is closer to that rung than half the distance to the rung's
// neighbour, so reading the rung off the tone cannot land on the wrong one.
//
// It is the container's answer to the tolerance the pins are marked by: a rule
// that names a step has to name the step the derivation used, and a measurement
// that could be within reach of two steps at once would name whichever came
// first in a loop.
func TestTheContainersToneNamesOneRungAndNoOther(t *testing.T) {
	worst, closest := 0.0, math.Inf(1)
	for _, sc := range schemesUnderTest(t) {
		for _, r := range statusRoles() {
			ramp := r.ramp(sc.c)
			ground := sc.c.StatusContainer(r.id)
			held, _, _ := vgcolor.LabFromNRGBA(ground)
			tone, _, _ := vgcolor.LabFromNRGBA(ramp.Step(toneStep(ramp, ground)))
			worst = max(worst, math.Abs(held-tone))
			for n := range RampSteps - 1 {
				a, _, _ := vgcolor.LabFromNRGBA(ramp.Step((n + 1) * 100))
				b, _, _ := vgcolor.LabFromNRGBA(ramp.Step((n + 2) * 100))
				closest = min(closest, math.Abs(a-b))
			}
		}
	}
	t.Logf("the worst container sits %.4f from its rung's tone; the closest two rungs stand %.4f apart", worst, closest)
	if 2*worst >= closest {
		t.Errorf("a container sits %.4f from its rung's tone and two rungs stand %.4f apart — the tone names two rungs",
			worst, closest)
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
// columns as the window is wide enough for, and never one so narrow that a name
// in it has to be cut.
func TestThePicksSpreadOverAWideWindowAndStackOnANarrowOne(t *testing.T) {
	gap, narrowest := int(PickColGap), boardNarrowest(t)
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

// boardNarrowest is the least a column of this board may be, measured off the
// names it draws — which is the number the layout itself works from, so a test
// asserting column counts asserts them against the same measurement and not
// against a copy of it.
func boardNarrowest(t *testing.T) int {
	t.Helper()
	light, dark := tokens.FromSeed(fixtureBlue)
	return pickNarrowest(measuring(), pinned(), paletteGroups(light, dark, false))
}

// TestTheBoardGivesUpAColumnRatherThanCutAName: at the widths the section is
// judged at, the board takes only as many columns as leave every identifier
// whole — two at a window of nine hundred, where three used to fit by a
// written-down minimum and cut two of them in half.
//
// The minimum used to be a constant, and the constant went stale the way a
// number describing a vocabulary does: the vocabulary grew. Measuring the names
// is what keeps the answer true when a role is renamed or a colour is named for
// the first time, and this is the assertion that says so — it names no width in
// points, it asks the board what it needs and then asks the columns to hold it.
func TestTheBoardGivesUpAColumnRatherThanCutAName(t *testing.T) {
	gtx, ty := measuring(), pinned()
	gap := int(PickColGap)
	// The widths the section is judged at, and the window's own on top of them:
	// it is the one width a reader is guaranteed to meet, since it is what the
	// window opens on.
	widths := append([]int{windowW - 2*int(Pad)}, sectionWidths...)
	for _, sc := range schemesUnderTest(t) {
		groups := paletteGroups(sc.c, sc.other, sc.dark)
		narrowest := pickNarrowest(gtx, ty, groups)
		for _, section := range widths {
			width := section - 2*sectionInset()
			cols := pickColumns(width, gap, narrowest)
			colW := (width - (cols-1)*gap) / cols
			if colW < narrowest {
				t.Errorf("%s at %d: %d columns of %d, under the %d a column needs to hold its names",
					sc.name, section, cols, colW, narrowest)
			}
			room := colW - int(PickSwatchW) - int(PickGap)
			for _, g := range groups {
				if got := fitLine(gtx, ty.Shaper, ty.Head, g.name, colW); got != g.name {
					t.Errorf("%s at %d: the family heading reads %q, want %q whole", sc.name, section, got, g.name)
				}
				for _, cell := range g.cells {
					if got := fitLine(gtx, ty.Shaper, ty.Body, cell.title(), room); got != cell.title() {
						t.Errorf("%s at %d: a cell is titled %q, want %q whole", sc.name, section, got, cell.title())
					}
				}
			}
		}
	}
	// And the window's own width takes the third column: a board that gave one
	// up everywhere would be a board that never spread.
	light, dark := tokens.FromSeed(fixtureBlue)
	groups := paletteGroups(light, dark, false)
	if got := pickColumns(rampContentW(), gap, pickNarrowest(gtx, ty, groups)); got != PickMaxCols {
		t.Errorf("the window's own width deals %d columns, want the board spread over %d", got, PickMaxCols)
	}
	t.Logf("a column needs %d points; at the section widths %v the board takes %v columns",
		pickNarrowest(gtx, ty, groups), sectionWidths, columnsAt(gtx, ty, groups))
}

// columnsAt is how many columns the board takes at each width it is judged at,
// for the log line that records what this task changed.
func columnsAt(gtx layout.Context, ty Type, groups []pickGroup) []int {
	out := make([]int, 0, len(sectionWidths))
	for _, section := range sectionWidths {
		out = append(out, pickColumns(section-2*sectionInset(), int(PickColGap), pickNarrowest(gtx, ty, groups)))
	}
	return out
}

// TestALineTooWideForItsColumnIsCutAtItsOwnBoundaries: nothing the board draws
// is ever cut inside a word, at any width, and what a cut line keeps is the
// identifier at the front of it.
//
// The rules are the lines this bites on, because a rule is a sentence about a
// name and is longer than the name by design. "Success 300, held at the
// container chroma" ended "at the container chro…" on a board three columns
// wide; cut at its own comma it reads "Success 300", which is the half a reader
// came for and a true sentence besides.
func TestALineTooWideForItsColumnIsCutAtItsOwnBoundaries(t *testing.T) {
	gtx, ty := measuring(), pinned()
	light, dark := tokens.FromSeed(fixtureBlue)
	groups := paletteGroups(light, dark, false)
	kept := map[string]bool{}
	for _, g := range groups {
		for _, cell := range g.cells {
			for _, line := range []struct {
				style textdraw.TextStyle
				text  string
			}{{ty.Body, cell.title()}, {ty.Small, cell.base.rule}, {ty.Small, cell.ink.rule}} {
				if line.text == "" {
					continue
				}
				for room := natural(gtx, ty.Shaper, line.style, line.text); room > 0; room -= 7 {
					got := fitLine(gtx, ty.Shaper, line.style, line.text, room)
					head := strings.TrimSuffix(got, Ellipsis)
					if head == line.text {
						continue
					}
					if !wholeWords(line.text, head) {
						t.Errorf("at %d points %q was cut to %q, which ends inside a word",
							room, line.text, got)
						break
					}
					if natural(gtx, ty.Shaper, line.style, got) > room {
						t.Errorf("at %d points %q was cut to %q, which is still too wide",
							room, line.text, got)
						break
					}
					if got != head && !strings.HasSuffix(got, Ellipsis) {
						t.Errorf("at %d points %q was cut to %q with no mark on the end", room, line.text, got)
					}
					kept[line.text] = kept[line.text] || head != ""
				}
			}
		}
	}
	if len(kept) == 0 {
		t.Fatal("no line on the board was ever cut, so nothing here was tested")
	}
	// The clause cut is the one that matters, so it is named: the container
	// rules are the longest lines on the board and their first clause is the
	// rung a reader is looking for.
	rule, longest := "", 0
	for _, r := range rulesOf(groups) {
		if w := natural(gtx, ty.Shaper, ty.Small, r); w > longest {
			rule, longest = r, w
		}
	}
	head := strings.SplitN(rule, ", ", 2)[0]
	if head == rule {
		t.Fatalf("the longest rule on the board is %q, which has no clause to cut at", rule)
	}
	room := natural(gtx, ty.Shaper, ty.Small, rule) - 1
	if got := fitLine(gtx, ty.Shaper, ty.Small, rule, room); got != head {
		t.Errorf("a hair too narrow for %q the board shows %q, want its first clause %q", rule, got, head)
	}
	t.Logf("%q cut to its room reads %q", rule, head)
}

// wholeWords reports whether head is line cut at a word boundary: a prefix of
// the line's own words, with nothing kept that the line does not have whole.
func wholeWords(line, head string) bool {
	if head == "" {
		return true
	}
	if !strings.HasPrefix(line, head) {
		return false
	}
	rest := line[len(head):]
	return rest == "" || rest[0] == ' ' || rest[0] == ',' || rest[0] == '/'
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

// paletteSectionW is the palette section on its own, at a width of the
// caller's choosing: the four rows the column would stack, on the ground the
// column would stand them on. The section is asserted here rather than inside
// a window capture because what is under test is how it answers to the width
// it is handed, and the widths worth trying are wider than the window the rest
// of this file measures.
func paletteSectionW(t *testing.T, width int, c, other tokens.ColorTokens, dark bool) *image.RGBA {
	t.Helper()
	rows := PaletteRows(PaletteFrom(c), c, other, pinned(), dark)
	size := image.Pt(width, paletteCaptureH)
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, size, c.Background)
		children := make([]layout.FlexChild, 0, len(rows))
		for _, row := range rows {
			children = append(children, layout.Rigid(row))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// paletteCaptureH is tall enough for the whole section at the widths tried
// below, so that nothing under test is clipped by the capture rather than by
// the layout. The narrowest of those widths is the tallest section: it is the
// one the board deals into two columns rather than three.
const paletteCaptureH = 1120

// TestPaletteSectionDump writes the section on its own at the narrowest width
// it is judged at and at the widest, in both schemes, for the same reason the
// window dumps write theirs: looking at it is a review step and not a test. It
// skips unless -themer.dump names a directory.
func TestPaletteSectionDump(t *testing.T) {
	if *dumpDir == "" {
		t.Skip("themer: pass -themer.dump=DIR to write the section out")
	}
	light, dark := tokens.FromSeed(fixtureBlue)
	for _, w := range []struct {
		name  string
		width int
	}{{"narrow", sectionWidths[0]}, {"wide", sectionWidths[len(sectionWidths)-1]}} {
		for _, sc := range []struct {
			name     string
			c, other tokens.ColorTokens
			dark     bool
		}{{"light", light, dark, false}, {"dark", dark, light, true}} {
			path := filepath.Join(*dumpDir, "themer-palette-"+w.name+"-"+sc.name+".png")
			if err := golden.Save(path, paletteSectionW(t, w.width, sc.c, sc.other, sc.dark)); err != nil {
				t.Fatalf("themer: save %s: %v", path, err)
			}
			t.Logf("wrote %s at %d points wide", path, w.width)
		}
	}
}

// sectionRowY is the middle of ramp row i inside a [paletteSectionW] capture,
// and sectionInset the margin the section's body and its heading bar both keep
// from either edge.
func sectionRowY(i int) int {
	return int(PaletteHeadH) + int(inventory.SectionPadY) + int(RampHeadH) + i*int(RampRowH) + int(RampRowH)/2
}

func sectionInset() int { return int(inventory.SectionPadX) }

// sectionWidths are the widths the section is judged at: one narrow enough
// that the caption has to give a clause up, the window's own, and one far
// wider than either — which is where the grid used to stop hundreds of points
// short of the heading bar over it.
var sectionWidths = []int{900 - 2*int(Pad), 1120 - 2*int(Pad), 1440 - 2*int(Pad)}

// TestTheGridEndsWhereTheHeadingBarDoes: the ramps grid and the bar above it
// keep one trailing edge at every width.
//
// The cells used to stop growing at ninety-six points, which left a wide
// window's grid with width it could not spend — 342 points of it at 1440 — and
// a ragged edge under a heading bar and over a picks board that both run the
// full width. Putting that surplus in the gap before the trailing chip instead
// only moved the defect: a hole four cells wide took the chips out of the rows
// they end. So the cells divide the whole row, and the chips are ranged
// against the trailing margin, and this is what holds both ends of that.
func TestTheGridEndsWhereTheHeadingBarDoes(t *testing.T) {
	for _, sc := range schemesUnderTest(t)[:4] {
		for _, width := range sectionWidths {
			img := paletteSectionW(t, width, sc.c, sc.other, sc.dark)
			ground := stdcolor.NRGBA(sc.c.Background)
			y := sectionRowY(0) // Primary, which every derivation pins
			edge := width - sectionInset()
			right := -1
			for x := edge - 1; x >= 0; x-- {
				if pixelAt(img, x, y) != ground {
					right = x
					break
				}
			}
			if right < 0 {
				t.Fatalf("%s at %d: nothing drawn in the first ramp row", sc.name, width)
			}
			if edge-1-right > 1 {
				t.Errorf("%s at %d: the row's last ink is at x=%d, %d points short of the section's trailing edge at %d — the grid and its heading bar disagree on where the section ends",
					sc.name, width, right, edge-1-right, edge-1)
			}
			// And the chip is still a chip at the end of a row rather than a
			// tenth step: the air before it is never less than the least the
			// grid states, however much surplus lands in it.
			left := right
			for left >= 0 && pixelAt(img, left, y) != ground {
				left--
			}
			gap := 0
			for x := left; x >= 0 && pixelAt(img, x, y) == ground; x-- {
				gap++
			}
			if gap < int(RampPinGap)-strokeBleed {
				t.Errorf("%s at %d: %d points of air between step 900 and the pinned base, want at least %d — nine steps and a pin read as ten steps",
					sc.name, width, gap, int(RampPinGap)-strokeBleed)
			}
			t.Logf("%s at %d: row ends at x=%d (edge %d), chip %d wide, gap %d",
				sc.name, width, right, edge-1, right-left, gap)
		}
	}
}

// strokeBleed is how far the rounded chip's hairline frame spreads past the
// rectangle it was asked for: the rasteriser's own antialiasing, a pixel on
// either side, which a scan reading colour boundaries off a capture counts as
// ink.
const strokeBleed = 2

// pixelAt is one pixel of a capture as an opaque colour.
func pixelAt(img *image.RGBA, x, y int) stdcolor.NRGBA {
	c := img.RGBAAt(x, y)
	return stdcolor.NRGBA{R: c.R, G: c.G, B: c.B, A: 0xff}
}

// TestACaptionTooWideForItsBarLosesWholeClauses: at every width there is, what
// the heading bar shows of a caption is a whole number of that caption's own
// clauses.
//
// The shaper's truncator used to do this and did it by character, which put
// "100 nearest the pa…" on the bar — two characters lost off a word and an
// ellipsis saying the line was clipped. The clauses are what the caption is
// made of and the only places it can be cut without saying something else.
func TestACaptionTooWideForItsBarLosesWholeClauses(t *testing.T) {
	gtx, ty := measuring(), pinned()
	for _, hint := range []string{RampsHint, PicksHint} {
		full := natural(gtx, ty.Shaper, ty.Small, hint)
		for room := 0; room <= full+40; room++ {
			fit := fitHint(gtx, ty, hint, room)
			if fit == "" {
				continue
			}
			if fit != hint && !strings.HasPrefix(hint, fit+HintSep) {
				t.Fatalf("at %d points of room the caption was cut to %q, which is not a run of its clauses", room, fit)
			}
			if w := natural(gtx, ty.Shaper, ty.Small, fit); w > room {
				t.Fatalf("at %d points of room the caption was cut to %q, which wants %d", room, fit, w)
			}
		}
		// And what fits, fits whole: nothing is dropped from a caption the bar
		// has room for.
		if fit := fitHint(gtx, ty, hint, full); fit != hint {
			t.Errorf("with room for the whole caption the bar shows %q", fit)
		}
	}
}

// TestTheCaptionsClausesAreTheOnesItIsWrittenIn: the separator the caption is
// cut at is the separator it is joined with. They are one string in one place
// and this is what says so — a caption joined by anything else would not split
// and would vanish whole at the first width it did not fit.
func TestTheCaptionsClausesAreTheOnesItIsWrittenIn(t *testing.T) {
	clauses := strings.Split(RampsHint, HintSep)
	if len(clauses) < 2 {
		t.Fatalf("the ramps caption %q does not split on %q", RampsHint, HintSep)
	}
	for _, clause := range clauses {
		if strings.TrimSpace(clause) != clause || clause == "" {
			t.Errorf("clause %q is not a clause of its own", clause)
		}
	}
	if got := strings.Join(clauses, HintSep); got != RampsHint {
		t.Errorf("the clauses rejoin to %q, want the caption itself", got)
	}
}

// captionRegister is how close a caption's contrast may fall to the contrast
// of the words beside it on the same bar before it reads as a different class
// of text. The caption is set in the heading's own ink, so what is measured is
// the antialiasing of twelve points against fourteen and nothing else.
const captionRegister = 0.85

// TestTheSectionCaptionReadsInItsNeighboursRegister: the caption on a section's
// heading bar is read at the contrast the rest of the section is read at, on
// both sides of the switch.
//
// It used to be set in the quiet neutral step every hint in this window uses,
// and that step does not hold its register across the schemes: measured against
// the bar it stands on it reaches 9.91:1 in a dark scheme, where its neighbours
// reach 13.71, and 5.46:1 in a light one, where they reach 15.16. A caption is
// a legend — the leading clause of this one is the only thing on the screen
// that says what the dots on the grid below mean — and a legend that is quiet
// in one scheme and faint in the other is a legend nobody reads in either.
func TestTheSectionCaptionReadsInItsNeighboursRegister(t *testing.T) {
	width := 1440 - 2*int(Pad)
	for _, sc := range schemesUnderTest(t)[:4] {
		img := paletteSectionW(t, width, sc.c, sc.other, sc.dark)
		p := PaletteFrom(sc.c)
		gtx, ty := measuring(), pinned()
		bar := image.Rect(0, 2, width, int(PaletteHeadH)-4)
		titleBand := bar
		titleBand.Min.X = sectionInset()
		titleBand.Max.X = sectionInset() + natural(gtx, ty.Shaper, ty.Label, RampsLabel)
		captionBand := bar
		captionBand.Max.X = width - sectionInset()
		captionBand.Min.X = captionBand.Max.X - natural(gtx, ty.Shaper, ty.Small, RampsHint)

		titleGround, titleInk := inkOn(img, titleBand)
		captionGround, captionInk := inkOn(img, captionBand)
		title := vgcolor.ContrastRatio(titleInk, titleGround)
		caption := vgcolor.ContrastRatio(captionInk, captionGround)
		t.Logf("%s: title %v on %v %.2f:1, caption %v on %v %.2f:1",
			sc.name, titleInk, titleGround, title, captionInk, captionGround, caption)
		if titleGround != stdcolor.NRGBA(p.Surface) || captionGround != stdcolor.NRGBA(p.Surface) {
			t.Errorf("%s: the heading bar is not the surface under both runs of text: title on %v, caption on %v, want %v",
				sc.name, titleGround, captionGround, p.Surface)
		}
		if caption < captionRegister*title {
			t.Errorf("%s: the caption reads at %.2f:1 beside a heading at %.2f:1 — %.0f%% of it, under the %.0f%% that keeps them one register",
				sc.name, caption, title, 100*caption/title, 100*captionRegister)
		}
	}
}
