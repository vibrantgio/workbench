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
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// markJitter is how far the middle of a drawn disc may sit from the colour it
// was filled with: the rasteriser's own last level or two of antialiasing, and
// nothing like the distance between a mark and the step under it.
const markJitter = 4

// The geometry of the palette section inside the window, from the same
// constants it lays out with. The Theme tab — the tab the window opens on —
// leads with the seed section, so the grid stands one section down from the
// strip and the geometry is measured from the model rather than from a
// constant: how tall the seed section is depends on whether the seed and the
// colour it was lifted into are one cell or two.
func rampGridTop(m Model, os tokens.ColorTokens) int {
	return tabTop() + seedSectionH(m, os) + int(palette.SectionHeadH) + int(inventory.SectionPadY)
}

// seedSectionH is the height the seed section takes at the head of the tab:
// its own heading band, the body's margins, and a cell per colour the row has
// to tell apart.
func seedSectionH(m Model, os tokens.ColorTokens) int {
	c, _ := derived(m, os)
	seed, picked := m.Seed()
	h := int(palette.SectionHeadH) + 2*int(inventory.SectionPadY)
	for _, cell := range seedCells(c, seed, picked) {
		h += int(cell.Height())
	}
	return h
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
	return (rampContentW() - int(palette.RampLabelW) - int(palette.RampPinGap) - int(palette.RampPinW)) / palette.RampSteps
}

// rampPinCentre is the middle of the chip at the end of row i, which is where
// that role's pinned base is drawn. The chips are ranged against the grid's
// trailing edge, so the middle of one is half a chip in from the far side of
// the content the section's body is laid out in — not one gap past step 900,
// which is where they would stand if the cells were the thing setting the
// grid's width.
func rampPinCentre(m Model, os tokens.ColorTokens, i int) image.Point {
	x := rampLabelLeft() + rampContentW() - int(palette.RampPinW)/2
	y := rampGridTop(m, os) + int(palette.RampHeadH) + i*int(palette.RampRowH) + int(palette.RampRowH)/2
	return image.Pt(x, y)
}

// rampCellCentre is the middle of the cell holding step n+1 of ramp row i,
// which is where a claimed rung's mark is, and rampCellColour a point in the
// same cell that no mark reaches — a quarter of the way in, against a mark of
// six points in a cell of ninety.
func rampCellCentre(m Model, os tokens.ColorTokens, i, n int) image.Point {
	x := rampLabelLeft() + int(palette.RampLabelW) + n*rampCellW() + rampCellW()/2
	y := rampGridTop(m, os) + int(palette.RampHeadH) + i*int(palette.RampRowH) + int(palette.RampRowH)/2
	return image.Pt(x, y)
}

func rampCellColour(m Model, os tokens.ColorTokens, i, n int) image.Point {
	at := rampCellCentre(m, os, i, n)
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

// TestTheWindowShowsOnePalette: the catalogue's own palette sections are on
// no tab of this window. They are a definition of the roles; the Theme tab
// shows a specimen of one seed's, with where each colour came from, and a
// reader given both would have no way of telling which of the two answers the
// question they arrived with.
//
// It also asserts the two are still there to leave out. A rename upstream
// would otherwise be silent, and the window would go back to showing the
// palette twice with nothing failing.
func TestTheWindowShowsOnePalette(t *testing.T) {
	e := newEmbed()
	inv := e.catalogue(pinned().Shaper, tokens.DefaultTypography, highlight.DefaultBases())
	light, _ := tokens.FromSeed(fixtureBlue)
	sections := map[string]bool{}
	for _, s := range inv.Foundations(light) {
		sections[s.Name] = true
	}
	for _, name := range []string{"foundations-roles", "foundations-ramps"} {
		if !sections[name] {
			t.Errorf("the catalogue has no section named %q for this window to leave out", name)
		}
	}
	for tab := TabComponents; tab < TabCount; tab++ {
		if rows := inv.TabItems(light, TabGroups[tab]); len(rows) == 0 {
			t.Errorf("the %s tab shows nothing", TabLabels[tab])
		}
	}
}

// TestTheRampsAreEveryRampWithNeutralLast: eight rows, each the theme's own
// ramp, in the order the grid is read. Neutral last is the whole of the
// ordering decision — it is the row the seed has least to say about — and a
// change that quietly put it back on top would be a change nobody notices until
// they are looking at the wrong row.
func TestTheRampsAreEveryRampWithNeutralLast(t *testing.T) {
	c, _ := tokens.FromSeed(fixtureBlue)
	rows := palette.RampRows(c)
	want := []struct {
		name string
		ramp tokens.Ramp
	}{
		{palette.PrimaryName, c.Ramps.Primary},
		{palette.SecondaryName, c.Ramps.Secondary},
		{palette.TertiaryName, c.Ramps.Tertiary},
		{palette.ErrorName, c.Ramps.Error},
		{palette.SuccessName, c.Ramps.Success},
		{palette.WarningName, c.Ramps.Warning},
		{palette.InfoName, c.Ramps.Info},
		{palette.NeutralName, c.Ramps.Neutral},
	}
	if len(rows) != len(want) {
		t.Fatalf("the grid has %d rows, want one per ramp (%d)", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].Name != w.name {
			t.Errorf("row %d is %q, want %q", i, rows[i].Name, w.name)
		}
		if rows[i].Ramp != w.ramp {
			t.Errorf("row %q does not carry the theme's own %s ramp", rows[i].Name, w.name)
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
		for _, g := range palette.Groups(sc.c, sc.other, sc.dark) {
			for _, cell := range g.Cells {
				add := func(name string, col stdcolor.NRGBA) {
					if _, seen := shown[name]; seen {
						t.Errorf("%s: %s is listed twice", sc.name, name)
					}
					shown[name] = col
				}
				add(cell.Base.Name, cell.Fill)
				if cell.Paired() {
					add(cell.Ink.Name, cell.On)
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
	out := map[string]stdcolor.NRGBA{palette.WhitePick: tokens.White, palette.BlackPick: tokens.Black}
	for _, r := range statusRoles() {
		out[r.name+palette.ContainerPick] = c.StatusContainer(r.id)
		out[r.name+palette.MarkPick] = c.OnStatusContainer(r.id)
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
		{palette.ErrorName, tokens.RoleError, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Error }},
		{palette.SuccessName, tokens.RoleSuccess, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Success }},
		{palette.WarningName, tokens.RoleWarning, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Warning }},
		{palette.InfoName, tokens.RoleInfo, func(c tokens.ColorTokens) tokens.Ramp { return c.Ramps.Info }},
	}
}

// TestABaseAndItsInkAreOneCell: the seven pinned roles, the page and its text,
// and the inverse pair are each one cell — one swatch, the ink written on it —
// because each is one decision. Surface and Divider stand alone, the theme
// naming no ink for either.
func TestABaseAndItsInkAreOneCell(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		want := map[string]string{
			palette.BackgroundPick:     palette.TextPick,
			palette.InverseSurfacePick: palette.OnInverseSurfacePick,
			palette.PrimaryName:        "OnPrimary",
			palette.SecondaryName:      "OnSecondary",
			palette.TertiaryName:       "OnTertiary",
			palette.ErrorName:          "OnError",
			palette.SuccessName:        "OnSuccess",
			palette.WarningName:        "OnWarning",
			palette.InfoName:           "OnInfo",
		}
		// A container and the mark read on it are one cell for the reason a base
		// and its ink are: the mark was measured over that exact ground.
		for _, r := range statusRoles() {
			want[r.name+palette.ContainerPick] = r.name + palette.MarkPick
		}
		// The two ends of the axis stand alone. They are what an ink turned out
		// to be, not a ground anything is written on, and writing letters on
		// either would be this section inventing a pairing the theme never made.
		//
		// The reserved highlighter stands alone for a reason of its own: it
		// marks content rather than being a surface content is written on, and
		// the theme names no ink for it.
		alone := map[string]bool{
			palette.SurfacePick: true, palette.DividerPick: true,
			palette.WhitePick: true, palette.BlackPick: true,
			palette.HighlightPick: true,
		}
		for _, g := range palette.Groups(sc.c, sc.other, sc.dark) {
			for _, cell := range g.Cells {
				ink, paired := want[cell.Base.Name]
				switch {
				case paired && !cell.Paired():
					t.Errorf("%s: %s is a swatch on its own, want %s written on it", sc.name, cell.Base.Name, ink)
				case paired && cell.Ink.Name != ink:
					t.Errorf("%s: %s carries %s, want %s", sc.name, cell.Base.Name, cell.Ink.Name, ink)
				case alone[cell.Base.Name] && cell.Paired():
					t.Errorf("%s: %s carries an ink, and the theme names none for it", sc.name, cell.Base.Name)
				case !paired && !alone[cell.Base.Name]:
					t.Errorf("%s: %s is a cell nothing accounts for", sc.name, cell.Base.Name)
				}
				// The title names both members, in the order their rules are
				// written under them.
				if cell.Paired() && cell.Title() != cell.Base.Name+palette.PickPairSep+cell.Ink.Name {
					t.Errorf("%s: the cell is titled %q, want both names in the order the rules are in", sc.name, cell.Title())
				}
				delete(want, cell.Base.Name)
				delete(alone, cell.Base.Name)
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
			palette.BackgroundPick: "Neutral 100",
			palette.TextPick:       "Neutral 900",
			palette.SurfacePick:    "Neutral 200",
			palette.DividerPick:    "Neutral 300",
			palette.PrimaryName:    fmt.Sprintf(palette.PickSeedNear, palette.PrimaryName, 700),
			palette.SecondaryName:  fmt.Sprintf(palette.PickJustOff, palette.SecondaryName, 700),
			palette.TertiaryName:   fmt.Sprintf(palette.PickJustOff, palette.TertiaryName, 700),
			palette.ErrorName:      "Error 700",
			palette.SuccessName:    "Success 700",
			palette.WarningName:    "Warning 700",
			palette.InfoName:       "Info 700",
		},
		"dark": {
			palette.BackgroundPick: "Neutral 100",
			palette.TextPick:       "Neutral 900",
			palette.SurfacePick:    "Neutral 200",
			palette.DividerPick:    "Neutral 300",
			palette.PrimaryName:    "Primary 700",
			palette.SecondaryName:  "Secondary 700",
			palette.TertiaryName:   "Tertiary 700",
			palette.ErrorName:      "Error 700",
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
		rules := rulesOf(palette.Groups(sc.c, sc.other, sc.dark))
		for name, rule := range want[sc.name] {
			if rules[name] != rule {
				t.Errorf("%s: %s says %q, want %q", sc.name, name, rules[name], rule)
			}
		}
		// The inverse pair names the other side by name, and names the role it
		// is over there rather than a step of the ramp on this side.
		if got, w := rules[palette.InverseSurfacePick], sc.inverseSide+" "+palette.PickSurfaceRole; got != w {
			t.Errorf("%s: %s says %q, want %q", sc.name, palette.InverseSurfacePick, got, w)
		}
		if got, w := rules[palette.OnInverseSurfacePick], sc.inverseSide+" "+palette.PickTextRole; got != w {
			t.Errorf("%s: %s says %q, want %q", sc.name, palette.OnInverseSurfacePick, got, w)
		}
		// Every ink says it was measured, because every one of them was, and
		// that is the half of the answer the colour alone does not give. An ink
		// whose own name says which role the cell is about — a rung of that
		// role's ramp — may say "the base"; one that does not, white and black
		// belonging to no role, has to name it, or a light scheme's seven cells
		// carry one sentence between them.
		for _, role := range []string{palette.PrimaryName, palette.SecondaryName, palette.TertiaryName,
			palette.ErrorName, palette.SuccessName, palette.WarningName, palette.InfoName} {
			rule := rules["On"+role]
			named := strings.HasSuffix(rule, fmt.Sprintf(palette.PickMeasuredOver, role))
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
		for _, role := range []string{palette.PrimaryName, palette.SecondaryName, palette.TertiaryName,
			palette.ErrorName, palette.SuccessName, palette.WarningName, palette.InfoName} {
			rule := rules["On"+role]
			if first, seen := said[rule]; seen {
				t.Errorf("%s: On%s and On%s both say %q", sc.name, first, role, rule)
			}
			said[rule] = role
		}
		// And an ink's rule names what was actually kept: one of the two ends
		// of the tonal axis, or the role's own deepest rung.
		if got := rules["OnPrimary"]; sc.dark {
			if !strings.HasPrefix(got, palette.PrimaryName+" ") && !strings.HasPrefix(got, palette.PickWhite) {
				t.Errorf("dark: OnPrimary says %q, want its own ramp's step or white", got)
			}
		} else if !strings.HasPrefix(got, palette.PickWhite) && !strings.HasPrefix(got, palette.PickBlack) {
			t.Errorf("light: OnPrimary says %q, want white or black", got)
		}
	}
}

// TestNoRuleIsEmpty: a colour with no rule under it is a swatch with a name,
// and a swatch with a name is not what this section says about a palette.
func TestNoRuleIsEmpty(t *testing.T) {
	for _, sc := range schemesUnderTest(t) {
		for _, g := range palette.Groups(sc.c, sc.other, sc.dark) {
			if g.Name == "" {
				t.Errorf("%s: a family of cells has no name", sc.name)
			}
			for _, cell := range g.Cells {
				for _, part := range []palette.Part{cell.Base, cell.Ink} {
					if part.Name == "" {
						continue
					}
					if part.Rule == "" {
						t.Errorf("%s: %s has no rule under it", sc.name, part.Name)
					}
				}
				if cell.Fill.A == 0 {
					t.Errorf("%s: %s draws a transparent swatch", sc.name, cell.Base.Name)
				}
				if cell.Paired() && cell.On.A == 0 {
					t.Errorf("%s: %s is written in a transparent ink", sc.name, cell.Ink.Name)
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
		groups := palette.Groups(sc.c, sc.other, sc.dark)
		claims := palette.Claims(groups)
		for claim := range claims {
			if claim.Step%100 != 0 || claim.Step < 100 || claim.Step > 900 {
				t.Errorf("%s: %s claims step %d, which is not a rung", sc.name, claim.Role, claim.Step)
			}
		}
		for _, g := range groups {
			for _, cell := range g.Cells {
				for _, part := range []palette.Part{cell.Base, cell.Ink} {
					if part.Step == 0 {
						continue
					}
					// A rule names the rung its mark is on, whether the colour
					// is that rung or is merely indistinguishable from it.
					want := fmt.Sprintf("%s %d", part.Role, part.Step)
					if !strings.Contains(part.Rule, want) {
						t.Errorf("%s: %s marks %s and its rule says %q", sc.name, part.Name, want, part.Rule)
					}
					if !claims[palette.Claim{Role: part.Role, Step: part.Step}] {
						t.Errorf("%s: %s names %s and the grid marks nothing there", sc.name, part.Name, want)
					}
				}
			}
		}
		// A light scheme's inks are the ends of the tonal axis, which are on no
		// ramp, so they claim nothing; a dark scheme's come off their own role's
		// ramp and claim its deepest rung. That difference is the derivation
		// being visible, and it is worth failing if it stops being true.
		rules := rulesOf(groups)
		for _, role := range []string{palette.PrimaryName, palette.ErrorName, palette.InfoName} {
			ink := rules["On"+role]
			if sc.dark && !strings.HasPrefix(ink, role+" ") {
				t.Errorf("%s: On%s says %q, want a rung of its own ramp", sc.name, role, ink)
			}
			if !sc.dark && !strings.HasPrefix(ink, palette.PickWhite) && !strings.HasPrefix(ink, palette.PickBlack) {
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
				{palette.PrimaryName, sc.Primary, sc.Ramps.Primary},
				{palette.SecondaryName, sc.Secondary, sc.Ramps.Secondary},
				{palette.TertiaryName, sc.Tertiary, sc.Ramps.Tertiary},
				{palette.ErrorName, sc.Error, sc.Ramps.Error},
				{palette.SuccessName, sc.Success, sc.Ramps.Success},
				{palette.WarningName, sc.Warning, sc.Ramps.Warning},
				{palette.InfoName, sc.Info, sc.Ramps.Info},
			} {
				step := palette.NearestStep(pin.ramp, pin.col)
				if step == 0 {
					t.Errorf("%s: the %s pin sits at no rung the grid can mark", hexOf(seed), pin.name)
					continue
				}
				worstMatch = max(worstMatch, palette.OKLabDistance(pin.ramp.Step(step), pin.col))
			}
			for _, r := range palette.RampRows(sc) {
				for n := range palette.RampSteps - 1 {
					closestRungs = min(closestRungs,
						palette.OKLabDistance(r.Ramp.Step((n+1)*100), r.Ramp.Step((n+2)*100)))
				}
			}
		}
	}
	t.Logf("the worst pin sits %.4f from its rung; the closest two rungs are %.4f apart; the tolerance is %.4f",
		worstMatch, closestRungs, palette.RungTolerance)
	if worstMatch >= palette.RungTolerance {
		t.Errorf("a pin sits %.4f from its own rung and the tolerance is %.4f — a pick the grid should mark goes unmarked",
			worstMatch, palette.RungTolerance)
	}
	// Two rungs a whole tolerance apart on either side of one colour is the
	// case that would let a colour be within reach of both, so the gap has to
	// beat twice the tolerance rather than merely exceed it.
	if closestRungs <= 2*palette.RungTolerance {
		t.Errorf("two rungs stand %.4f apart against a tolerance of %.4f — a mark cannot say which rung it means",
			closestRungs, palette.RungTolerance)
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
		for i, row := range palette.RampRows(c) {
			for n := range palette.RampSteps {
				at := rampCellColour(m, os, i, n)
				want := row.Ramp.Step((n + 1) * 100)
				got := img.RGBAAt(at.X, at.Y)
				if got.R != want.R || got.G != want.G || got.B != want.B {
					t.Errorf("%s %d at %v drew %v, want the ramp's %v",
						row.Name, (n+1)*100, at, got, want)
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
		claims := palette.Claims(palette.Groups(c, other, m.Dark(os)))
		if len(claims) == 0 {
			t.Fatal("no pick claims a rung, so the grid marks nothing")
		}
		img := page(t, m, os)
		for i, row := range palette.RampRows(c) {
			for n := range palette.RampSteps {
				at := rampCellCentre(m, os, i, n)
				step := row.Ramp.Step((n + 1) * 100)
				marked := claims[palette.Claim{Role: row.Name, Step: (n + 1) * 100}]
				want := step
				if marked {
					want = palette.MarkInkOn(step)
				}
				got := img.RGBAAt(at.X, at.Y)
				// A mark is a disc, so its own middle carries the last level or
				// two of the rasteriser's antialiasing; a cell that carries none
				// is a flat fill and is compared exactly. The tolerance is far
				// under the distance between a step and the end of the axis
				// chosen to stand out on it, which is what is being asserted.
				off := max(apart(got.R, want.R), max(apart(got.G, want.G), apart(got.B, want.B)))
				if (marked && off > markJitter) || (!marked && off != 0) {
					t.Errorf("%s %d at %v drew %v, want %v", row.Name, (n+1)*100, at, got, want)
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
		groups := palette.Groups(sc.c, sc.other, sc.dark)
		rules := rulesOf(groups)
		for _, end := range []struct {
			name, text string
			col        stdcolor.NRGBA
		}{
			{palette.WhitePick, palette.PickAxisLight, tokens.White},
			{palette.BlackPick, palette.PickAxisDark, tokens.Black},
		} {
			used := false
			for _, g := range groups {
				for _, cell := range g.Cells {
					if cell.Paired() && cell.On == end.col {
						used = true
					}
				}
			}
			want := fmt.Sprintf(palette.PickAxisNoInk, end.text)
			if used {
				want = fmt.Sprintf(palette.PickAxisInk, end.text)
			}
			if got := rules[end.name]; got != want {
				t.Errorf("%s: %s says %q, want %q", sc.name, end.name, got, want)
			}
		}
		// And the case the rule exists for: a light scheme does write in white
		// and a dark scheme writes in neither end, so a rule that said one thing
		// on both sides would be saying nothing on either.
		if !sc.dark {
			if got, want := rules[palette.WhitePick], fmt.Sprintf(palette.PickAxisInk, palette.PickAxisLight); got != want {
				t.Errorf("%s: %s says %q, and this scheme's inks are white", sc.name, palette.WhitePick, got)
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
// third of the scale reads at 2.7:1 in white and 7.8:1 in black. The status
// containers name their marks on those rungs, on four cells of every dark
// scheme.
func TestEveryMarkOnTheGridReadsOnTheStepItStandsOn(t *testing.T) {
	worst, at := math.Inf(1), ""
	for _, sc := range schemesUnderTest(t) {
		claims := palette.Claims(palette.Groups(sc.c, sc.other, sc.dark))
		for _, row := range palette.RampRows(sc.c) {
			for n := range palette.RampSteps {
				if !claims[palette.Claim{Role: row.Name, Step: (n + 1) * 100}] {
					continue
				}
				step := row.Ramp.Step((n + 1) * 100)
				if got := vgcolor.ContrastRatio(palette.MarkInkOn(step), step); got < worst {
					worst, at = got, fmt.Sprintf("%s %s %d", sc.name, row.Name, (n+1)*100)
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

// TestEverySwatchIsBoundedByItsEdgeOrByItsOwnFill: the section frames every
// swatch in one colour per scheme — the inverse of the page — and every fill it
// frames is told from the ground it stands on either by that edge or by being
// that far from the ground itself.
//
// One colour, because a frame chosen per swatch turns its polarity over in the
// middle of every ramp and the flip is louder than the edges it buys. What
// stands in for a per-swatch floor is the pair of readings: an edge that fades
// as a fill leaves the page tone behind is an edge handing the boundary to the
// fill, and the two multiply out to the contrast between the page and its
// inverse, so they cannot both be weak. The range is logged in full and the
// soft end named.
func TestEverySwatchIsBoundedByItsEdgeOrByItsOwnFill(t *testing.T) {
	// Per side, because the two sides are two different questions: one edge is
	// near-black over fills running up from a near-white page, the other is
	// near-white over fills running down from a near-black one.
	type span struct {
		lo, hi     float64
		soft, hard string
	}
	// The pale side first, so the two lines of the log read in the order the
	// account of this choice writes them.
	sides := [2]struct {
		name string
		span
	}{{name: "light"}, {name: "dark"}}
	worst, at, bounds := math.Inf(1), "", ""
	for i := range sides {
		sides[i].lo = math.Inf(1)
	}
	for _, sc := range schemesUnderTest(t) {
		edge := palette.EdgeIn(sc.c)
		// The single colour, asserted as the token the account names rather
		// than as bytes: a section framed in something else is a section whose
		// edge no longer turns over with the switch.
		if edge != sc.c.InverseSurface {
			t.Errorf("%s: the section's edge is %v, want the page's inverse %v", sc.name, edge, sc.c.InverseSurface)
		}
		side := &sides[0].span
		if sc.dark {
			side = &sides[1].span
		}
		selfFramed := ""
		for name, fill := range sectionFills(sc.c, sc.other, sc.dark) {
			// The recorded exception: one swatch on the board is the edge
			// colour, so it wears no edge at all and is bounded by being the
			// furthest thing from the page there is. Any second swatch framed
			// in itself would be a colour the section is claiming twice.
			if fill == edge {
				if selfFramed != "" {
					t.Errorf("%s: %s and %s are both framed in themselves", sc.name, selfFramed, name)
				}
				selfFramed = name
			}
			got := vgcolor.ContrastRatio(edge, fill)
			if got < side.lo {
				side.lo, side.soft = got, sc.name+" "+name
			}
			if got > side.hi {
				side.hi, side.hard = got, sc.name+" "+name
			}
			// Bounded: by the edge over the fill, or by the fill over the page.
			// A swatch needs one of the two and the section guarantees it can
			// never be short of both.
			page := vgcolor.ContrastRatio(fill, sc.c.Background)
			if b := math.Max(got, page); b < worst {
				worst, at = b, sc.name+" "+name
				bounds = fmt.Sprintf("%.2f:1 in edge, %.2f:1 against the page", got, page)
			}
		}
		if want := palette.InverseSurfacePick + palette.PickPairSep + palette.OnInverseSurfacePick; selfFramed != want {
			t.Errorf("%s: the swatch framed in itself is %q, want the section's own edge colour on the board, %q",
				sc.name, selfFramed, want)
		}
	}
	for _, s := range sides {
		t.Logf("the %s schemes' edge runs from %.2f:1 on %s to %.2f:1 on %s",
			s.name, s.lo, s.soft, s.hi, s.hard)
	}
	t.Logf("the least-bounded swatch in the section is %s, at %s", at, bounds)
	if worst < markGraphicFloor {
		t.Errorf("%s is bounded by neither its edge nor its own fill (%s), under the %.1f:1 a graphic owes its ground",
			at, bounds, markGraphicFloor)
	}
}

// sectionFills is every colour this section paints a swatch in, named as the
// section names it: the seventy-two rungs of the grid, the base each role
// pinned, and the fill of every cell on the picks board.
func sectionFills(c, other tokens.ColorTokens, dark bool) map[string]stdcolor.NRGBA {
	fills := map[string]stdcolor.NRGBA{}
	for _, row := range palette.RampRows(c) {
		for n := range palette.RampSteps {
			fills[fmt.Sprintf("%s %d", row.Name, (n+1)*100)] = row.Ramp.Step((n + 1) * 100)
		}
		if row.Pin.A != 0 {
			fills[row.Name+" base"] = row.Pin
		}
	}
	for _, g := range palette.Groups(c, other, dark) {
		for _, cell := range g.Cells {
			fills[cell.Title()] = cell.Fill
		}
	}
	return fills
}

// TestTheGridDrawsOneEdgeColourAcrossTheRow: read off a render, every boundary
// along a row of the grid is drawn in the section's one edge colour.
//
// The grid is where a frame chosen per swatch would show itself, being the one
// place the whole ramp is on screen at once: the boundary between cell four and
// cell five in one polarity and the boundary between five and six in the other
// reads as two rows. Asserted off the pixels rather than off [palette.EdgeIn]
// alone, because what a reader sees is what was drawn. The Neutral row is
// scanned because its own fills are the greys the edge is nearest, so a row
// that holds here holds everywhere.
func TestTheGridDrawsOneEdgeColourAcrossTheRow(t *testing.T) {
	for _, sc := range schemesUnderTest(t)[:4] {
		img := paletteSectionW(t, sectionWidths[0], sc.c, sc.other, sc.dark)
		row := -1
		for i, r := range palette.RampRows(sc.c) {
			if r.Name == palette.NeutralName {
				row = i
			}
		}
		if row < 0 {
			t.Fatalf("%s: no %s row on the grid", sc.name, palette.NeutralName)
		}
		// A point below the cell's top, so what is read is the vertical
		// boundary between two cells and not the row's own gap. The leading
		// edge of the first cell stands against the label column rather than
		// against another cell, so the scan starts at the second.
		edge, mid := palette.EdgeIn(sc.c), sectionRowY(row)
		for n := 1; n < palette.RampSteps; n++ {
			x := sectionCellX(sectionWidths[0], n)
			if got := pixelAt(img, x, mid); got != edge {
				t.Errorf("%s: the boundary before %s %d drew %v, want the section's edge %v",
					sc.name, palette.NeutralName, (n+1)*100, got, edge)
			}
		}
		t.Logf("%s: all %d boundaries of the %s row drew %v",
			sc.name, palette.RampSteps-1, palette.NeutralName, edge)
	}
}

// sectionCellX is the leading edge of the n-th step cell inside a
// [paletteSectionW] capture of the given width.
func sectionCellX(width, n int) int {
	inset := sectionInset()
	content := width - 2*inset
	cellW := (content - int(palette.RampLabelW) - int(palette.RampPinGap) - int(palette.RampPinW)) / palette.RampSteps
	return inset + int(palette.RampLabelW) + n*cellW
}

// TestEveryPinnedBaseStandsAtTheEndOfItsOwnRow: the colour a role was actually
// pinned is drawn beside the nine it was pinned against, read off the render.
//
// A light scheme's Primary is the chosen seed at the seed's own depth and its
// Secondary and Tertiary are pinned off their own 700, so a grid of rungs alone
// shows nine colours a role might have been and not the one it is — and the
// seed, which is the colour the window exists to judge, is nowhere in the
// window.
func TestEveryPinnedBaseStandsAtTheEndOfItsOwnRow(t *testing.T) {
	m := seeded(t)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		c, _ := derived(m, os)
		img := page(t, m, os)
		for i, row := range palette.RampRows(c) {
			at := rampPinCentre(m, os, i)
			got := img.RGBAAt(at.X, at.Y)
			if row.Pin.A == 0 {
				// Neutral pins no solid fill, so its slot carries the mark that
				// says so and no chip: something is drawn in the middle of it,
				// and the slot around that something is the section's own
				// ground rather than a colour standing in for a pin.
				if got.R == c.Background.R && got.G == c.Background.G && got.B == c.Background.B {
					t.Errorf("%s pins nothing and its slot at %v is empty, want the mark that says so", row.Name, at)
				}
				off := image.Pt(at.X-int(palette.RampPinW)/3, at.Y)
				beside := img.RGBAAt(off.X, off.Y)
				if want := c.Background; beside.R != want.R || beside.G != want.G || beside.B != want.B {
					t.Errorf("%s pins nothing and its slot at %v drew %v, want the ground %v",
						row.Name, off, beside, want)
				}
				continue
			}
			if got.R != row.Pin.R || got.G != row.Pin.G || got.B != row.Pin.B {
				t.Errorf("%s's chip at %v drew %v, want the pinned base %v",
					row.Name, at, got, row.Pin)
			}
		}
		// And the seed itself, which is the first chip in the grid on the side
		// that pins it: a light scheme's Primary is the seed, lifted.
		if !m.Dark(os) {
			at := rampPinCentre(m, os, 0)
			got := img.RGBAAt(at.X, at.Y)
			if want := c.Primary; got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("the first chip drew %v, want the lifted seed %v", got, want)
			}
		}
	}
}

// fixtureMagenta is a seed whose lifted light Primary sits between two rungs
// of its own ramp: its depth lands between the tones of steps 500 and 600, and
// the pin sits 0.0575 in OKLab from the nearest rung — over three times
// [palette.RungTolerance] — so nearest-rung matching honestly claims nothing.
// Found by scanning the seed cube for the widest such margin at a vivid
// mid-scale colour. It is the case the chip dot exists for: the light scheme pins the
// seed at the seed's own depth, and this seed's depth is no rung's.
var fixtureMagenta = stdcolor.NRGBA{R: 0xf8, G: 0x00, B: 0xd8, A: 0xff}

// offRampSeeded is a window showing the theme fixtureMagenta generates: the
// one fixture whose light pin claims no rung and whose dark pin is a rung
// exactly, which are the two sides of the chip-dot question in one seed.
func offRampSeeded(t *testing.T) Model {
	t.Helper()
	m := dropped(t)
	m.Candidates = []imageseed.Candidate{candidate(fixtureMagenta, 1)}
	m.Selected = 0
	return m
}

// TestTheOffRampFixtureSitsBetweenRungs: the fixture the chip dot is judged
// with honestly is the case it exists for. The lifted seed is on no rung and
// indistinguishable from none, with margin; its depth falls between two
// adjacent steps of the scale rather than off either end; its rule says the
// seed was lifted and nothing else; and no pick claims a Primary rung, which is
// the row the chip's own dot keeps from reading as unused. The dark side of the
// same seed pins a rung exactly, which is the chip that has to stay undotted.
func TestTheOffRampFixtureSitsBetweenRungs(t *testing.T) {
	light, dark := tokens.FromSeed(fixtureMagenta)
	if n := palette.StepIn(light.Ramps.Primary, light.Primary); n != 0 {
		t.Fatalf("the light pin is rung %d exactly, want a pin between rungs", n)
	}
	if n := palette.NearestStep(light.Ramps.Primary, light.Primary); n != 0 {
		t.Fatalf("the light pin is indistinguishable from rung %d, want a pin between rungs", n)
	}
	nearest := math.Inf(1)
	for n := range palette.RampSteps {
		nearest = min(nearest, palette.OKLabDistance(light.Ramps.Primary.Step((n+1)*100), light.Primary))
	}
	t.Logf("the lifted seed sits %.4f from its nearest rung, against a tolerance of %.4f", nearest, palette.RungTolerance)
	if nearest < 2*palette.RungTolerance {
		t.Errorf("the lifted seed sits %.4f from a rung against a tolerance of %.4f — too near to hold the between-rungs case",
			nearest, palette.RungTolerance)
	}
	// Between two adjacent steps of the light scale — which runs pale to deep —
	// with daylight on both sides, not past either end of it.
	l, _, _ := vgcolor.LabFromNRGBA(light.Primary)
	above, _, _ := vgcolor.LabFromNRGBA(light.Ramps.Primary.Step(500))
	below, _, _ := vgcolor.LabFromNRGBA(light.Ramps.Primary.Step(600))
	t.Logf("the lifted seed's depth is L* %.2f, between step 500 at %.2f and step 600 at %.2f", l, above, below)
	if l > above-3 || l < below+3 {
		t.Errorf("the lifted seed's depth L* %.2f is not between steps 500 (%.2f) and 600 (%.2f) with margin",
			l, above, below)
	}
	groups := palette.Groups(light, dark, false)
	if got := rulesOf(groups)[palette.PrimaryName]; got != palette.PickSeed {
		t.Errorf("the light Primary rule says %q, want %q", got, palette.PickSeed)
	}
	for claim := range palette.Claims(groups) {
		if claim.Role == palette.PrimaryName {
			t.Errorf("a pick claims %s %d, so the row was never the one with no dot", claim.Role, claim.Step)
		}
	}
	if palette.StepIn(dark.Ramps.Primary, dark.Primary) == 0 {
		t.Error("the dark pin is on no rung, want the rung-exact pin whose chip stays undotted")
	}
}

// TestTheChipDotAgreesWithTheRule: the chip carries a dot exactly where the
// rule under the pick says the pin is on no rung. [palette.PinRung] asks the two
// questions [palette.BasePart] resolves a base's rule by, and this holds the two
// answers together — a chip dotted beside a rule naming a rung, or a bare chip
// beside a rule claiming none, would be the section disagreeing with itself in
// the two places it is read.
func TestTheChipDotAgreesWithTheRule(t *testing.T) {
	for _, seed := range []stdcolor.NRGBA{fixtureBlue, fixtureRed, fixtureGrey, tokens.DefaultSeed, fixtureMagenta} {
		light, dark := tokens.FromSeed(seed)
		for _, sc := range []tokens.ColorTokens{light, dark} {
			for _, row := range palette.RampRows(sc) {
				if row.Pin.A == 0 {
					continue // Neutral pins nothing: a dash, and never a dot
				}
				// The near and off wordings differ per role and play no part in
				// which rung the rule claims, which is the half under test.
				part := palette.BasePart(row.Name, row.Ramp, row.Pin, palette.PickJustOff, palette.PickPinned)
				if dotted, claimed := palette.PinRung(row.Ramp, row.Pin) == 0, part.Step != 0; dotted == claimed {
					t.Errorf("%s %s: the chip dot says the pin claims no rung (%t) and the rule claims step %d",
						hexOf(seed), row.Name, dotted, part.Step)
				}
			}
		}
	}
}

// TestAnOffRampBaseCarriesTheDotItself: a pinned base indistinguishable from
// no rung carries the dot on its own chip, in the ink measured over the chip
// the way every cell's dot is measured over its step — so the row reads as
// used and placed rather than as a role nothing picked. The dark side of the
// same seed is the control: a rung-exact pin keeps its rung dot and an
// undotted chip. Both read off the render.
func TestAnOffRampBaseCarriesTheDotItself(t *testing.T) {
	m := offRampSeeded(t)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		c, _ := derived(m, os)
		img := page(t, m, os)
		at := rampPinCentre(m, os, 0) // Primary leads the grid
		got := img.RGBAAt(at.X, at.Y)
		if !m.Dark(os) {
			// The dot, in the middle of the chip, in the measured ink.
			want := palette.MarkInkOn(c.Primary)
			if off := max(apart(got.R, want.R), max(apart(got.G, want.G), apart(got.B, want.B))); off > markJitter {
				t.Errorf("the chip centre at %v drew %v, want the dot ink %v", at, got, want)
			}
			// Beside the dot the chip is still the pin.
			side := img.RGBAAt(at.X-int(palette.RampPinW)/3, at.Y)
			if side.R != c.Primary.R || side.G != c.Primary.G || side.B != c.Primary.B {
				t.Errorf("beside the dot the chip drew %v, want the pinned base %v", side, c.Primary)
			}
			// And no cell of the row carries one: the pin claims no rung, and
			// the dot the row owes its reader is the chip's.
			for n := range palette.RampSteps {
				cell := rampCellCentre(m, os, 0, n)
				step := c.Ramps.Primary.Step((n + 1) * 100)
				pix := img.RGBAAt(cell.X, cell.Y)
				if pix.R != step.R || pix.G != step.G || pix.B != step.B {
					t.Errorf("%s %d at %v drew %v, want the bare step %v — the pin claims no rung",
						palette.PrimaryName, (n+1)*100, cell, pix, step)
				}
			}
			continue
		}
		// The rung-exact side: the chip stays the pin, whole, and the dot is on
		// the rung the pin is.
		if got.R != c.Primary.R || got.G != c.Primary.G || got.B != c.Primary.B {
			t.Errorf("the rung-exact chip centre at %v drew %v, want the undotted pin %v", at, got, c.Primary)
		}
		n := palette.StepIn(c.Ramps.Primary, c.Primary)
		if n == 0 {
			t.Fatal("the dark pin claims no rung, want the rung-exact control case")
		}
		cell := rampCellCentre(m, os, 0, n/100-1)
		step := c.Ramps.Primary.Step(n)
		want := palette.MarkInkOn(step)
		pix := img.RGBAAt(cell.X, cell.Y)
		if off := max(apart(pix.R, want.R), max(apart(pix.G, want.G), apart(pix.B, want.B))); off > markJitter {
			t.Errorf("%s %d at %v drew %v, want its dot %v", palette.PrimaryName, n, cell, pix, want)
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
		rules := rulesOf(palette.Groups(sc.c, sc.other, sc.dark))
		for _, r := range statusRoles() {
			ramp := r.ramp(sc.c)
			ground := sc.c.StatusContainer(r.id)
			step := palette.ToneStep(ramp, ground)
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
			if got, want := rules[r.name+palette.ContainerPick], fmt.Sprintf(palette.PickContainerRule, r.name, step); got != want {
				t.Errorf("%s: %s says %q, want %q", sc.name, r.name+palette.ContainerPick, got, want)
			}
			// And the mark on it is a rung of the role's own ramp, named as one.
			mark := sc.c.OnStatusContainer(r.id)
			n := palette.StepIn(ramp, mark)
			if n == 0 {
				t.Errorf("%s: the %s mark %v is on no rung of its own ramp", sc.name, r.name, mark)
				continue
			}
			if got, want := rules[r.name+palette.MarkPick], fmt.Sprintf(palette.PickMarkRule, r.name, n); got != want {
				t.Errorf("%s: %s says %q, want %q", sc.name, r.name+palette.MarkPick, got, want)
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
			tone, _, _ := vgcolor.LabFromNRGBA(ramp.Step(palette.ToneStep(ramp, ground)))
			worst = max(worst, math.Abs(held-tone))
			for n := range palette.RampSteps - 1 {
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
	on := ReduceModel(m, SetScheme{Dark: true})
	c, _ := derived(on, tokens.DefaultLight)
	for i, row := range palette.RampRows(c) {
		at := rampCellColour(on, tokens.DefaultLight, i, palette.RampSteps-1)
		want := row.Ramp.Step(palette.RampSteps * 100)
		got := darkImg.RGBAAt(at.X, at.Y)
		if got.R != want.R || got.G != want.G || got.B != want.B {
			t.Errorf("with the switch on dark, %s 900 drew %v, want the dark ramp's %v", row.Name, got, want)
		}
	}
	top := rampGridTop(on, tokens.DefaultLight)
	if moved := bandChange(lightImg, darkImg, top, top+int(palette.RampHeadH)+8*int(palette.RampRowH)); moved < schemeBandFloor {
		t.Errorf("the switch moved %.2f%% of the grid, want the whole of it", moved)
	}
}

// TestThePicksSpreadOverAWideWindowAndStackOnANarrowOne: the board is as many
// columns as the window is wide enough for, and never one so narrow that a name
// in it has to be cut.
func TestThePicksSpreadOverAWideWindowAndStackOnANarrowOne(t *testing.T) {
	gap, narrowest := int(palette.PickColGap), boardNarrowest(t)
	for _, tc := range []struct{ width, want int }{
		{rampContentW(), palette.PickMaxCols},
		{2*narrowest + gap, 2},
		{narrowest, 1},
		{narrowest - 1, 1},
		{0, 1},
	} {
		if got := palette.Columns(tc.width, gap, narrowest); got != tc.want {
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
	return palette.Narrowest(measuring(), pinned().story(), palette.Groups(light, dark, false))
}

// TestTheBoardGivesUpAColumnRatherThanCutAName: at the widths the section is
// judged at, the board takes only as many columns as leave every identifier
// whole — two at a window of nine hundred.
//
// A column minimum written down as a constant goes stale the way a number
// describing a vocabulary does: the vocabulary grows. Measuring the names keeps
// the answer true when a role is renamed or a colour is named for the first
// time, so this names no width in points — it asks the board what it needs and
// then asks the columns to hold it.
func TestTheBoardGivesUpAColumnRatherThanCutAName(t *testing.T) {
	gtx, ty := measuring(), pinned()
	gap := int(palette.PickColGap)
	// The widths the section is judged at, and the window's own on top of them:
	// it is the one width a reader is guaranteed to meet, since it is what the
	// window opens on.
	widths := append([]int{windowW - 2*int(Pad)}, sectionWidths...)
	for _, sc := range schemesUnderTest(t) {
		groups := palette.Groups(sc.c, sc.other, sc.dark)
		narrowest := palette.Narrowest(gtx, ty.story(), groups)
		for _, section := range widths {
			width := section - 2*sectionInset()
			cols := palette.Columns(width, gap, narrowest)
			colW := (width - (cols-1)*gap) / cols
			if colW < narrowest {
				t.Errorf("%s at %d: %d columns of %d, under the %d a column needs to hold its names",
					sc.name, section, cols, colW, narrowest)
			}
			room := colW - int(palette.PickSwatchW) - int(palette.PickGap)
			for _, g := range groups {
				if got := palette.FitLine(gtx, ty.Shaper, ty.Head, g.Name, colW); got != g.Name {
					t.Errorf("%s at %d: the family heading reads %q, want %q whole", sc.name, section, got, g.Name)
				}
				for _, cell := range g.Cells {
					if got := palette.FitLine(gtx, ty.Shaper, ty.Body, cell.Title(), room); got != cell.Title() {
						t.Errorf("%s at %d: a cell is titled %q, want %q whole", sc.name, section, got, cell.Title())
					}
				}
			}
		}
	}
	// And the window's own width takes the third column: a board that gave one
	// up everywhere would be a board that never spread.
	light, dark := tokens.FromSeed(fixtureBlue)
	groups := palette.Groups(light, dark, false)
	if got := palette.Columns(rampContentW(), gap, palette.Narrowest(gtx, ty.story(), groups)); got != palette.PickMaxCols {
		t.Errorf("the window's own width deals %d columns, want the board spread over %d", got, palette.PickMaxCols)
	}
	t.Logf("a column needs %d points; at the section widths %v the board takes %v columns",
		palette.Narrowest(gtx, ty.story(), groups), sectionWidths, columnsAt(gtx, ty, groups))
}

// columnsAt is how many columns the board takes at each width it is judged at,
// for the log line.
func columnsAt(gtx layout.Context, ty Type, groups []palette.Group) []int {
	out := make([]int, 0, len(sectionWidths))
	for _, section := range sectionWidths {
		out = append(out, palette.Columns(section-2*sectionInset(), int(palette.PickColGap), palette.Narrowest(gtx, ty.story(), groups)))
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
	groups := palette.Groups(light, dark, false)
	kept := map[string]bool{}
	for _, g := range groups {
		for _, cell := range g.Cells {
			for _, line := range []struct {
				style textdraw.TextStyle
				text  string
			}{{ty.Body, cell.Title()}, {ty.Small, cell.Base.Rule}, {ty.Small, cell.Ink.Rule}} {
				if line.text == "" {
					continue
				}
				for room := natural(gtx, ty.Shaper, line.style, line.text); room > 0; room -= 7 {
					got := palette.FitLine(gtx, ty.Shaper, line.style, line.text, room)
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
	if got := palette.FitLine(gtx, ty.Shaper, ty.Small, rule, room); got != head {
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
	groups := palette.Groups(light, dark, false)
	total := 0
	for _, g := range groups {
		total += palette.Load(g)
	}
	for n := 1; n <= palette.PickMaxCols; n++ {
		cols := palette.Pack(groups, n)
		if len(cols) != n {
			t.Fatalf("%d columns asked for, %d dealt", n, len(cols))
		}
		read, tallest := []string{}, 0
		for _, col := range cols {
			load := 0
			for _, g := range col {
				read = append(read, g.Name)
				load += palette.Load(g)
			}
			tallest = max(tallest, load)
		}
		if len(read) != len(groups) {
			t.Errorf("%d columns hold %d families, want all %d", n, len(read), len(groups))
		}
		for i, g := range groups {
			if i < len(read) && read[i] != g.Name {
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
func best(groups []palette.Group, n int) int {
	if n <= 1 || len(groups) == 0 {
		total := 0
		for _, g := range groups {
			total += palette.Load(g)
		}
		return total
	}
	shortest := -1
	var walk func(from, left, tallest int)
	walk = func(from, left, tallest int) {
		if left == 1 {
			load := 0
			for _, g := range groups[from:] {
				load += palette.Load(g)
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
				load += palette.Load(groups[cut])
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
	groups := palette.Groups(light, dark, false)
	for n := 1; n <= palette.PickMaxCols; n++ {
		for _, col := range palette.Pack(groups, n) {
			for i, g := range col {
				if g.Name != palette.PickInverseGroup {
					continue
				}
				if i == 0 || col[i-1].Name != palette.PickPageGroup {
					t.Errorf("%d columns: %s stands under %v, want it under %s",
						n, palette.PickInverseGroup, names(col), palette.PickPageGroup)
				}
			}
		}
	}
}

func names(col []palette.Group) []string {
	out := make([]string, len(col))
	for i, g := range col {
		out[i] = g.Name
	}
	return out
}

// rulesOf is every cell's rules by the name they stand under.
func rulesOf(groups []palette.Group) map[string]string {
	out := map[string]string{}
	for _, g := range groups {
		for _, cell := range g.Cells {
			out[cell.Base.Name] = cell.Base.Rule
			if cell.Paired() {
				out[cell.Ink.Name] = cell.Ink.Rule
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
	// And the seed whose light pin claims no rung, which is the one grid where
	// the dot is on a chip rather than a cell — with its own dark side, where
	// the pin is a rung exactly and the chip stays bare.
	oLight, oDark := tokens.FromSeed(fixtureMagenta)
	for _, sc := range []struct {
		name     string
		c, other tokens.ColorTokens
		dark     bool
	}{{"light", oLight, oDark, false}, {"dark", oDark, oLight, true}} {
		path := filepath.Join(*dumpDir, "themer-palette-offramp-"+sc.name+".png")
		if err := golden.Save(path, paletteSectionW(t, sectionWidths[1], sc.c, sc.other, sc.dark)); err != nil {
			t.Fatalf("themer: save %s: %v", path, err)
		}
		t.Logf("wrote %s at %d points wide", path, sectionWidths[1])
	}
}

// sectionRowY is the middle of ramp row i inside a [paletteSectionW] capture,
// and sectionInset the margin the section's body and its heading bar both keep
// from either edge.
func sectionRowY(i int) int {
	return int(palette.SectionHeadH) + int(inventory.SectionPadY) + int(palette.RampHeadH) + i*int(palette.RampRowH) + int(palette.RampRowH)/2
}

func sectionInset() int { return int(inventory.SectionPadX) }

// sectionWidths are the widths the section is judged at: one narrow enough
// that the caption has to give a clause up, the window's own, and one far
// wider than either, where a grid with a capped cell width would stop hundreds
// of points short of the heading bar over it.
var sectionWidths = []int{900 - 2*int(Pad), 1120 - 2*int(Pad), 1440 - 2*int(Pad)}

// TestTheGridEndsWhereTheHeadingBarDoes: the ramps grid and the bar above it
// keep one trailing edge at every width.
//
// The cells divide the whole row and the chips are ranged against the trailing
// margin, which is what holds both ends of that. Cells capped at ninety-six
// points leave a wide window's grid with width it cannot spend — 342 points of
// it at 1440 — under a heading bar and over a picks board that both run the
// full width; putting that surplus in the gap before the trailing chip instead
// opens a hole four cells wide that takes the chips out of the rows they end.
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
			if gap < int(palette.RampPinGap)-strokeBleed {
				t.Errorf("%s at %d: %d points of air between step 900 and the pinned base, want at least %d — nine steps and a pin read as ten steps",
					sc.name, width, gap, int(palette.RampPinGap)-strokeBleed)
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
// The clauses are what the caption is made of and the only places it can be
// cut without saying something else. Truncating by character puts "100 nearest
// the pa…" on the bar — two characters lost off a word, and an ellipsis saying
// only that the line was clipped.
func TestACaptionTooWideForItsBarLosesWholeClauses(t *testing.T) {
	gtx, ty := measuring(), pinned()
	for _, hint := range []string{palette.RampsHint, palette.PicksHint} {
		full := natural(gtx, ty.Shaper, ty.Small, hint)
		for room := 0; room <= full+40; room++ {
			fit := fitHint(gtx, ty, hint, room)
			if fit == "" {
				continue
			}
			if fit != hint && !strings.HasPrefix(hint, fit+palette.HintSep) {
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
	clauses := strings.Split(palette.RampsHint, palette.HintSep)
	if len(clauses) < 2 {
		t.Fatalf("the ramps caption %q does not split on %q", palette.RampsHint, palette.HintSep)
	}
	for _, clause := range clauses {
		if strings.TrimSpace(clause) != clause || clause == "" {
			t.Errorf("clause %q is not a clause of its own", clause)
		}
	}
	if got := strings.Join(clauses, palette.HintSep); got != palette.RampsHint {
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
// The quiet neutral step every hint in this window uses does not hold its
// register across the schemes: measured against the bar it stands on it reaches
// 9.91:1 in a dark scheme, where its neighbours reach 13.71, and 5.46:1 in a
// light one, where they reach 15.16. A caption is
// a legend — the leading clause of this one is the only thing on the screen
// that says what the dots on the grid below mean — and a legend that is quiet
// in one scheme and faint in the other is a legend nobody reads in either.
func TestTheSectionCaptionReadsInItsNeighboursRegister(t *testing.T) {
	width := 1440 - 2*int(Pad)
	for _, sc := range schemesUnderTest(t)[:4] {
		img := paletteSectionW(t, width, sc.c, sc.other, sc.dark)
		p := PaletteFrom(sc.c)
		gtx, ty := measuring(), pinned()
		bar := image.Rect(0, 2, width, int(palette.SectionHeadH)-4)
		titleBand := bar
		titleBand.Min.X = sectionInset()
		titleBand.Max.X = sectionInset() + natural(gtx, ty.Shaper, ty.Label, palette.RampsLabel)
		captionBand := bar
		captionBand.Max.X = width - sectionInset()
		captionBand.Min.X = captionBand.Max.X - natural(gtx, ty.Shaper, ty.Small, palette.RampsHint)

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
