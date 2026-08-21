// The palette section: what the seed on screen actually generated, said in
// two parts, in the place on the embedded page where the palette was already
// spoken about.
//
// # Why the window shows this at all
//
// Everything else in this window answers "what does this colour look like".
// The row of candidates offers colours, the page below draws the whole design
// system in the one that was chosen, and between those two there is a step
// nobody sees: a seed does not become a window, it becomes a palette, and the
// palette is what the window is drawn from. A person judging a seed by the page
// alone can tell that something is wrong without being able to say what — the
// dividers are too faint, the warning is too close to the error — because the
// thing that decided both is not on screen anywhere.
//
// So it is put on screen, and in the order the derivation works in. First the
// ramps, which are what there is to pick from: eight roles, nine steps each, on
// one shared lightness scale. Then the picks, which are the colours the theme
// actually names, each beside the rule that chose it. Ramps above picks, because
// a pick that says "Neutral 300" means nothing until the row it came out of is
// the thing directly above it.
//
// # Where it stands, and what it replaced
//
// In the embedded page's own foundations, in the place its two palette sections
// stood, and those two are gone from this window's column — see [embed.items].
// Not in front of them: a section of provenance immediately above a section
// showing the same tokens again is the same page saying one thing twice, and a
// reader has no way of telling which of the two is the answer. The window shows
// one palette story and it is this one.
//
// It is a section of that column rather than a band of the window for a second
// reason. The page under the candidate row is what a seed is judged on and is
// the biggest thing in the window by design; a fixed band above it would come
// out of that page, and this section is a third of a screen tall. In the column
// it costs the page nothing and lands in the causal order: the seed, the ramps,
// the picks, then everything drawn from them.
//
// # Why the picks carry a rule and not just a colour
//
// A swatch labelled "Divider" is a colour. A swatch labelled "Divider — Neutral
// 300" is a colour and an explanation, and the explanation is what makes the
// grid above it useful: it says which of the seventy-two cells up there this
// one came out of. The rules are read off the colours themselves rather than
// written down here — a pin is compared against its own ramp, an ink against
// the two ends of the tonal axis and its role's deepest step — so a derivation
// that changes what it pins changes what this section says about it, in the same
// build, rather than leaving a caption behind that is quietly no longer true.
//
// Some of them are not a rung and still have to point at one. A light scheme's
// accents are pinned a unit of lightness off their own 700 — three parts in 255,
// which no eye and no display can show — and a light Primary is the chosen seed
// lifted, sitting at whatever depth that seed has. Comparing bytes, none of the
// three is on the ramp; looking at the grid, all three plainly are. So the rule
// says both: how the colour was pinned, and the rung it is indistinguishable
// from, which is the rung the grid marks. Saying only the first left three rows
// of the grid unmarked beside four that were marked, and the only reading
// available for that is that Primary is not in the table.
//
// # Why a base and its ink are one cell
//
// Because they are one decision. The derivation does not pick a primary and
// then, separately, pick something to write on it: it pins the base and then
// measures both ends of the tonal axis over that exact colour and keeps the
// better one, which is why the ink cannot be understood apart from the fill it
// was measured against. Two cells side by side would be the window claiming they
// are two facts about two colours, and would leave the ink drawn on a ground
// nobody said it was for.
//
// So the pair is one swatch: the base as the fill, the ink as two letters on it,
// and under the pair of names the two rules in the order the names are in. The
// specimen is the claim and the claim is checkable by looking. Surface and
// Divider stand alone because nothing is written on them — they are grounds and
// borders, and the theme names no ink for either.
package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"math"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The section's dimensions.
//
// The heading rows are the column's own furniture at the column's own sizes:
// this section stands among a page of labelled sections, and a heading a few
// points off the ones around it reads as a heading from somewhere else.
// Everything inside them is measured to what it holds.
const (
	// PaletteHeadH is a section heading, at the height the embedded page gives
	// its own.
	PaletteHeadH unit.Dp = 32

	// RampSteps is how many steps a ramp has. It is the length of the row and
	// the count of the numbers over it, and it is one name rather than a 9
	// written down in three places.
	RampSteps = 9
	// RampLabelW is the column the ramp names stand in, at the left of the
	// grid, and RampGutter what stands between the longest of them and the first
	// swatch. The names are ranged against the grid, so the gutter is a fixed
	// distance rather than whatever a name's own length leaves over.
	RampLabelW unit.Dp = 88
	RampGutter unit.Dp = 14
	// RampRowH is one ramp, RampHeadH the line of step numbers above the
	// columns. A row is a band rather than a tile: nine of them stacked is the
	// whole scale at once, and height spent on any one of them is height the
	// picks below have to be scrolled to.
	RampRowH  unit.Dp = 24
	RampHeadH unit.Dp = 18
	// RampCellMax stops a wide window from stretching nine swatches into nine
	// panels. Past it the grid keeps its size and the row simply ends, which is
	// what a table does.
	RampCellMax unit.Dp = 96
	// RampMark is the dot on a rung a pick sits at. It is a dot and not a ring, a
	// label or a heavier frame: a fifth of the cells carry one, and anything with
	// an edge of its own at that count turns a table of colour into a table of
	// marks.
	RampMark unit.Dp = 7

	// PickSwatchW and PickSwatchH are one cell's colour. It is wide enough to
	// carry two letters at the size the rest of the window sets its specimens
	// in, because most of these swatches carry an ink and an ink shown smaller
	// than the claim next to it is a claim nobody can check — and tall enough
	// that the letters have air above and below rather than reaching the edge,
	// which on a chip this small is the difference between a specimen and a
	// stamp.
	PickSwatchW unit.Dp = 44
	PickSwatchH unit.Dp = 26
	// PickPairH is a base and its ink, which carry three lines: the two names,
	// and a rule for each. PickCellH is a colour that has no ink, which carries
	// two. The difference between either and the lines inside it is the air one
	// cell keeps from the next.
	PickPairH unit.Dp = 62
	PickCellH unit.Dp = 44
	// PickTitleH is the line the names are on and PickRuleH one rule under them.
	PickTitleH unit.Dp = 18
	PickRuleH  unit.Dp = 15
	// PickHeadH is the name over one family of cells and PickHeadGap the air
	// under the line that follows it. PickGroupGap is what stands above the
	// name, and it is the larger of the two by a long way: a heading equidistant
	// between the family it ends and the family it starts belongs to neither.
	PickHeadH    unit.Dp = 20
	PickHeadGap  unit.Dp = 8
	PickGroupGap unit.Dp = 22
	// PickGap is swatch to text.
	PickGap unit.Dp = 10
	// PickColGap is between one column of cells and the next, and PickMinW the
	// narrowest a column is worth having: under it the rule truncates into an
	// ellipsis and the column stops explaining anything.
	PickColGap unit.Dp = 24
	PickMinW   unit.Dp = 250
	// PickMaxCols is as wide as the board spreads. Four families across a
	// window is a row of short lists rather than a board.
	PickMaxCols = 3
)

// What the section says about itself. The two names are the two halves of the
// derivation named for what they are: a ramp is a scale to pick from and a pick
// is a colour taken off one.
//
// The ramps' line leads with the mark and not with the table. A caption is
// truncated from its tail, and at a width where only one clause survives the one
// worth keeping is the one nothing else on screen says: the column headers
// already print 100 to 900, and which end of the scale is nearest the page is
// visible the moment anybody looks — while a dot is a mark with no other legend
// anywhere. Ordered the other way round, the clause that died at the tight width
// was the only one that was doing any work.
//
// What the line says about the scale is still worth saying, because it turns
// over between the schemes: step 100 is the palest rung in a light theme and the
// deepest in a dark one, and it is the same step in both — the one nearest the
// page — which is the whole reason a component asking for 100 gets a tint on
// either side. Everybody double-takes at the first press of the switch.
//
// And the mark's own clause says indistinguishable from rather than sitting on,
// because that is what the grid marks: a light scheme's accents are pinned a
// hair off their rung and are marked there, so a caption promising the rung was
// taken would be a caption the picks below it quietly contradict.
const (
	RampsLabel = "Palette Ramps"
	RampsHint  = "a dot marks the rung a pick is indistinguishable from · nine steps a role · 100 nearest the page"
	PicksLabel = "Palette Picks"
	PicksHint  = "every colour the theme names, and where it came from"
)

// The families the cells are read in. Page and surfaces first because it is the
// ground everything else stands on, and the inverse pair straight after it:
// those two are surfaces as well, borrowed whole from the other side of the
// scheme, and a reader looking for a surface should not have to pass the accents
// and the status roles to find the last two. Then the accents the seed rotates,
// and the status roles it may only tint.
const (
	PickPageGroup    = "Page and surfaces"
	PickInverseGroup = "Inverse"
	PickAccentGroup  = "Accents"
	PickStatusGroup  = "Status"
)

// The role names, said once. They are the ramp rows' labels and the cells'
// names both, and the same role named two ways in one section would read as two
// roles.
const (
	NeutralName   = "Neutral"
	PrimaryName   = "Primary"
	SecondaryName = "Secondary"
	TertiaryName  = "Tertiary"
	ErrorName     = "Error"
	SuccessName   = "Success"
	WarningName   = "Warning"
	InfoName      = "Info"
)

// The rules, as they are written under a pair of names.
const (
	PickGlyph = "Aa"
	// PickPairSep joins the two names a cell carries, in the order their rules
	// are written under them: the fill first, then what is written on it.
	PickPairSep = " / "
	// PickMeasured is how an ink's rule ends when the ink's own name has already
	// said which role the cell is about — a rung of the role's own ramp, which is
	// what a dark scheme's inks are.
	//
	// PickMeasuredOver is how it ends when the ink's name has not: white and
	// black belong to no role, so seven cells of a light scheme would otherwise
	// carry one identical sentence between them, saying nothing per cell that the
	// first of them had not already said. Naming the base makes every line its
	// own, and it is on the ink's line rather than in the section's caption
	// because a cell has to derive both the colours it carries.
	PickMeasured     = ", measured over the base"
	PickMeasuredOver = ", measured over %s"
	PickWhite        = "white"
	PickBlack        = "black"
	// PickMeasuredOn is what an ink says when it is neither end of the axis nor
	// a rung of its own ramp, which no derivation shipping today produces.
	PickMeasuredOn = "measured over the base"
	// PickSeed is the light scheme's Primary where it fell between rungs: the
	// colour that was chosen, lifted onto the palette's own chroma and pinned at
	// its own depth, which is a depth no scale has a say in.
	PickSeed = "the seed, lifted"
	// PickJustOff is a base that landed beside a rung rather than on it — the
	// light scheme's accents, which are pinned one unit of lightness off their
	// own 700. It names the rung because a base a reader cannot find on the grid
	// above is a provenance that provenances nothing, and because the rung is
	// where the grid marks it: the two are one answer and are written from one.
	PickJustOff = "pinned just off %s %d"
	// PickSeedNear is the light scheme's Primary where the seed it was lifted
	// from landed beside a rung. Which rung that is depends on the seed's own
	// depth and is worth saying: it is where on its own ramp the colour somebody
	// chose actually sits.
	PickSeedNear = "the seed, lifted, just off %s %d"
	// PickPinned is a base near no rung at all, which the derivations shipping
	// today do not produce for any role but Primary.
	PickPinned = "pinned off the ramp"
	// PickOffRamp is what a neutral resolution says when it is not on the
	// neutral ramp, which no derivation shipping today produces. It is here so
	// that one which did would say so rather than say nothing.
	PickOffRamp = "off the neutral ramp"
	// The side of the scheme the window is not showing, which is where the
	// inverse pair comes from, filled in with that side's own role name: a light
	// window's inverse surface is the dark scheme's Surface, exactly, and saying
	// "Neutral 200" instead would name a step of the wrong ramp.
	PickOtherLight = "the light scheme's %s"
	PickOtherDark  = "the dark scheme's %s"
	PickOtherSide  = "the other scheme's %s"
	// PickSurfaceRole and PickTextRole are the counterpart roles the inverse
	// pair resolves from.
	PickSurfaceRole = "Surface"
	PickTextRole    = "Text"
)

// The token names the cells carry, which are the names in the theme's own
// source. They are the vocabulary a person reads the palette in, and renaming
// them for the sake of a prettier caption would mean this section describes a
// palette nobody can look up.
const (
	BackgroundPick       = "Background"
	TextPick             = "Text"
	SurfacePick          = "Surface"
	DividerPick          = "Divider"
	InverseSurfacePick   = "InverseSurface"
	OnInverseSurfacePick = "OnInverseSurface"
)

// rampRow is one row of the ramps grid: a role's name and the nine steps
// generated for it.
type rampRow struct {
	name string
	ramp tokens.Ramp
}

// rampRows is the grid, in the order it is read.
//
// The seed's own roles lead, because they are the ones a choice on this window
// moves: Primary is the colour that was picked, Secondary and Tertiary are
// rotated off it, and a reader watching a candidate change watches these three.
// The status roles follow — anchored hues the seed may only tint a few degrees,
// so they barely move and belong under the ones that do. Neutral is last and
// not first: it is the largest of the eight in the window, carrying every
// surface, border and line of text there is, and precisely because it is
// everywhere it is the one nobody is choosing. A grid that opens on it opens on
// the row the seed has least to say about.
func rampRows(c tokens.ColorTokens) []rampRow {
	return []rampRow{
		{PrimaryName, c.Ramps.Primary},
		{SecondaryName, c.Ramps.Secondary},
		{TertiaryName, c.Ramps.Tertiary},
		{ErrorName, c.Ramps.Error},
		{SuccessName, c.Ramps.Success},
		{WarningName, c.Ramps.Warning},
		{InfoName, c.Ramps.Info},
		{NeutralName, c.Ramps.Neutral},
	}
}

// rampClaim is one rung a pick took: the row it is on and the step it is.
type rampClaim struct {
	role string
	step int
}

// rampClaims is every rung the picks below the grid took, which is what the
// grid marks.
//
// It is read off the picks rather than written down beside them, so the two
// halves of this section cannot drift: a rule saying "Error 700" and a marker
// on any other cell would be one section disagreeing with itself in the two
// places it is read. What claims nothing is as informative as what claims
// something — a light scheme's inks are literally white and its accents are
// pinned off their rung, so the light grid is marked in far fewer places than
// the dark one, and that is the derivation being visible rather than a gap.
func rampClaims(groups []pickGroup) map[rampClaim]bool {
	out := map[rampClaim]bool{}
	for _, g := range groups {
		for _, cell := range g.cells {
			for _, part := range [2]pickPart{cell.base, cell.ink} {
				if part.role != "" && part.step != 0 {
					out[rampClaim{part.role, part.step}] = true
				}
			}
		}
	}
	return out
}

// pickPart is one colour token in a cell: what the theme calls it, what chose
// it, and — where what chose it was a rung of a ramp — which row and which step,
// so the grid above can mark the rung this pick took.
//
// The rung and the rule are resolved together and never separately. A rule that
// says "Error 700" and a marker on a different cell would be the section
// disagreeing with itself in the two places it is read.
type pickPart struct {
	name, rule string
	role       string // the ramp row the rule names, empty when it names none
	step       int    // the rung on that row, 0 when the colour is on none
}

// pickCell is one thing the palette decided. It is a base and the ink measured
// over it — one swatch, two names, two rules — or, where the theme names no ink
// for a colour, that colour on its own.
type pickCell struct {
	base, ink pickPart
	fill, on  stdcolor.NRGBA
}

// paired reports whether this cell carries an ink as well as a fill.
func (c pickCell) paired() bool { return c.ink.name != "" }

// title is the cell's names, in the order their rules are written under them.
func (c pickCell) title() string {
	if !c.paired() {
		return c.base.name
	}
	return c.base.name + PickPairSep + c.ink.name
}

// height is the slot this cell takes: three lines for a pair and two for a
// colour standing on its own.
func (c pickCell) height() unit.Dp {
	if c.paired() {
		return PickPairH
	}
	return PickCellH
}

// pickGroup is one family of cells under its own name.
type pickGroup struct {
	name  string
	cells []pickCell
}

// paletteGroups is every colour token the theme names, grouped as they are
// read, with each rule resolved against the colours themselves.
//
// c is the side of the pair the window is drawing and other is the side it is
// not — the inverse pair is the only thing here that comes from across the
// scheme, and it is handed the counterpart rather than deriving one, so this
// section costs the window no palette of its own.
func paletteGroups(c, other tokens.ColorTokens, dark bool) []pickGroup {
	n := c.Ramps.Neutral
	alone := func(base pickPart, fill stdcolor.NRGBA) pickCell {
		return pickCell{base: base, fill: fill}
	}
	return []pickGroup{
		{PickPageGroup, []pickCell{
			// The page and the ink it is read in: the one pair in the theme
			// that is two pins rather than a pin and a measurement, and still
			// one decision — the ink is chosen for that ground.
			{
				base: neutralPart(BackgroundPick, n, c.Background),
				ink:  neutralPart(TextPick, n, c.Text),
				fill: c.Background, on: c.Text,
			},
			alone(neutralPart(SurfacePick, n, c.Surface), c.Surface),
			alone(neutralPart(DividerPick, n, c.Divider), c.Divider),
		}},
		{PickInverseGroup, []pickCell{{
			base: inversePart(InverseSurfacePick, c.InverseSurface, other.Surface, PickSurfaceRole, dark),
			ink:  inversePart(OnInverseSurfacePick, c.OnInverseSurface, other.Text, PickTextRole, dark),
			fill: c.InverseSurface, on: c.OnInverseSurface,
		}}},
		{PickAccentGroup, []pickCell{
			pinnedCell(PrimaryName, c.Ramps.Primary, c.Primary, c.OnPrimary, PickSeedNear, PickSeed),
			pinnedCell(SecondaryName, c.Ramps.Secondary, c.Secondary, c.OnSecondary, PickJustOff, PickPinned),
			pinnedCell(TertiaryName, c.Ramps.Tertiary, c.Tertiary, c.OnTertiary, PickJustOff, PickPinned),
		}},
		{PickStatusGroup, []pickCell{
			pinnedCell(ErrorName, c.Ramps.Error, c.Error, c.OnError, PickJustOff, PickPinned),
			pinnedCell(SuccessName, c.Ramps.Success, c.Success, c.OnSuccess, PickJustOff, PickPinned),
			pinnedCell(WarningName, c.Ramps.Warning, c.Warning, c.OnWarning, PickJustOff, PickPinned),
			pinnedCell(InfoName, c.Ramps.Info, c.Info, c.OnInfo, PickJustOff, PickPinned),
		}},
	}
}

// pinnedCell is one role's cell: the base the derivation pinned and the ink it
// measured over that exact colour, named for it — the token is called OnPrimary
// because Primary is what it has to clear. near is what the base's rule says
// when it landed beside a rung rather than on one, and off what it says when it
// landed beside none.
func pinnedCell(role string, r tokens.Ramp, base, ink stdcolor.NRGBA, near, off string) pickCell {
	return pickCell{
		base: basePart(role, r, base, near, off),
		ink:  inkPart(role, r, ink),
		fill: base, on: ink,
	}
}

// stepIn reports which step of r the colour is, or 0 when it is not on the ramp
// at all. It compares bytes rather than measuring a distance: a pin that is a
// rung is that rung exactly, and a pin that is near one is a different colour,
// which is the whole distinction this section exists to draw.
func stepIn(r tokens.Ramp, col stdcolor.NRGBA) int {
	for i := range r {
		if r[i] == col {
			return (i + 1) * 100
		}
	}
	return 0
}

// basePart is a pinned base as a cell carries it, in three cases: the rung it
// landed on, the rung it is indistinguishable from and how it came to be beside
// rather than on it (near, which takes the role and the rung), and — where no
// rung is near — how it was pinned and nothing else (off).
func basePart(role string, r tokens.Ramp, col stdcolor.NRGBA, near, off string) pickPart {
	if n := stepIn(r, col); n != 0 {
		return pickPart{name: role, rule: fmt.Sprintf("%s %d", role, n), role: role, step: n}
	}
	if n := nearestStep(r, col); n != 0 {
		return pickPart{name: role, rule: fmt.Sprintf(near, role, n), role: role, step: n}
	}
	return pickPart{name: role, rule: off, role: role}
}

// nearestStep is the rung of r a colour is indistinguishable from, or 0 when it
// is distinguishable from all nine.
//
// It exists because "on the ramp" and "the same colour as the ramp" are not the
// same question, and the grid answers the second. Every light-scheme accent is
// pinned one unit of lightness off its own 700 rung — three parts in 255, which
// is nothing an eye or a display can show — and comparing bytes left three rows
// of the grid unmarked beside four that were marked, whose only possible reading
// is that Primary is not in the table at all.
//
// The distance is measured in OKLab, where a step is a step wherever it is
// taken, and the tolerance is set by measurement rather than by taste. It has a
// floor and a ceiling and sits between them: the pins that ought to match sit up
// to sixteen thousandths from their rung, so anything under that misses a match;
// and the closest two rungs of any ramp this derivation builds are forty-one
// thousandths apart, so anything over half of that would put a colour within
// reach of two rungs at once and leave a mark unable to say which it meant.
// Eighteen is the point that is as far from one bound as from the other, and the
// two measurements are asserted in this package's tests rather than trusted.
func nearestStep(r tokens.Ramp, col stdcolor.NRGBA) int {
	best, at := rungTolerance, 0
	for i := range r {
		if d := oklabDistance(r[i], col); d < best {
			best, at = d, (i+1)*100
		}
	}
	return at
}

// rungTolerance is how far from a rung a colour may sit and still be that
// rung as far as anybody looking is concerned. See [nearestStep] for the two
// measurements it stands between.
const rungTolerance = 0.018

// oklabDistance is how far apart two colours are, perceptually.
func oklabDistance(a, b stdcolor.NRGBA) float64 {
	l1, a1, b1 := vgcolor.OKLabFromNRGBA(a)
	l2, a2, b2 := vgcolor.OKLabFromNRGBA(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// neutralPart is a surface, a border or the body ink: all of them resolutions of
// one neutral step.
func neutralPart(name string, n tokens.Ramp, col stdcolor.NRGBA) pickPart {
	part := basePart(NeutralName, n, col, PickJustOff, PickOffRamp)
	part.name = name
	return part
}

// inkPart names what the derivation put over the base and kept: one of the two
// ends of the tonal axis, named for the base it was measured over, or — where
// the base is a dark scheme's, and white would be the wrong half of the pair —
// the role's own deepest rung, which names the role itself.
//
// The ends of the axis are on no ramp, so an ink that is one of them claims no
// rung and the grid above marks nothing for it. That is the fact, and it is
// worth being able to see: under a light theme almost every ink is literally
// white, and under a dark one every one of them comes off its own role's ramp.
func inkPart(role string, r tokens.Ramp, ink stdcolor.NRGBA) pickPart {
	part := pickPart{name: "On" + role, role: role}
	switch ink {
	case tokens.White:
		part.rule = PickWhite + fmt.Sprintf(PickMeasuredOver, role)
		return part
	case tokens.Black:
		part.rule = PickBlack + fmt.Sprintf(PickMeasuredOver, role)
		return part
	}
	if n := stepIn(r, ink); n != 0 {
		part.rule, part.step = fmt.Sprintf("%s %d%s", role, n, PickMeasured), n
		return part
	}
	part.rule = PickMeasuredOn
	return part
}

// inversePart is one member of the inverse pair. It claims no rung: the colour
// is a step of the counterpart scheme's neutral ramp, and that ramp is not one
// of the eight on screen.
func inversePart(name string, col, counterpart stdcolor.NRGBA, role string, dark bool) pickPart {
	return pickPart{name: name, rule: inverseRule(col, counterpart, role, dark)}
}

// inverseRule names the counterpart role an inverse colour is, in the words of
// the side it came from. The claim is checked rather than asserted: where the
// colour is that side's role byte for byte it is named as it, and where it is
// not the rule falls back to naming the side alone.
func inverseRule(col, counterpart stdcolor.NRGBA, role string, dark bool) string {
	if col != counterpart {
		return fmt.Sprintf(PickOtherSide, role)
	}
	if dark {
		return fmt.Sprintf(PickOtherLight, role)
	}
	return fmt.Sprintf(PickOtherDark, role)
}

// PaletteSectionRows is how many rows [PaletteRows] returns: a heading and a
// body for each of the two sections. The column is addressed by row number in
// one place — see [embed.codeColumnRow] — and the count is asserted against the
// rows themselves rather than trusted.
const PaletteSectionRows = 4

// PaletteRows is the section as rows of the embedded page's column: the ramps
// under their own heading, and the picks under theirs.
func PaletteRows(p Palette, c, other tokens.ColorTokens, ty Type, dark bool) []layout.Widget {
	groups := paletteGroups(c, other, dark)
	return []layout.Widget{
		paletteHeading(p, c, ty, RampsLabel, RampsHint),
		paletteBody(c, RampGrid(p, c, ty, rampClaims(groups))),
		paletteHeading(p, c, ty, PicksLabel, PicksHint),
		paletteBody(c, PickBoard(p, ty, groups)),
	}
}

// paletteHeading labels one of the two sections, with what it is at the leading
// edge and how to read it at the trailing one — the same two-part label the
// window's own rows carry, on the ground the column's headings stand on.
func paletteHeading(p Palette, c tokens.ColorTokens, ty Type, title, hint string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(PaletteHeadH))
		paint.FillShape(gtx.Ops, p.Surface, clip.Rect{Max: size}.Op())
		line := gtx.Dp(Hairline)
		paint.FillShape(gtx.Ops, c.Divider,
			clip.Rect(image.Rect(0, size.Y-line, size.X, size.Y)).Op())
		pad := gtx.Dp(inventory.SectionPadX)
		box := image.Rect(pad, 0, max(pad, size.X-pad), size.Y)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, box, 0, 0.5, p.Text, title)
		// The caption takes what the title leaves rather than the whole bar, so
		// a narrow window truncates it instead of running it into the title:
		// two runs of text laid out from opposite ends of one box meet in the
		// middle the moment the box is narrower than the two of them.
		if lead := box.Min.X + natural(gtx, ty.Shaper, ty.Label, title) + gtx.Dp(Gap); lead < box.Max.X {
			textdraw.FillText(gtx, ty.Shaper, ty.Small,
				image.Rect(lead, 0, box.Max.X, size.Y), 1, 0.5, p.Muted, hint)
		}
		return layout.Dimensions{Size: size}
	}
}

// paletteBody lays one section's content out on the embedded page's own ground,
// inside the margin every other body in that column keeps.
//
// The content is drawn before the ground is painted and replayed over it. The
// height is the content's — a grid of eight rows and a board of however many
// columns the window is wide enough for are two different heights, and neither
// is a number worth writing down beside them — so the ground cannot be filled
// until the content has been measured, and the content cannot be drawn under a
// ground that is not there yet.
func paletteBody(c tokens.ColorTokens, body func(gtx layout.Context, width int) int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		padX, padY := gtx.Dp(inventory.SectionPadX), gtx.Dp(inventory.SectionPadY)
		width := max(0, gtx.Constraints.Max.X-2*padX)
		macro := op.Record(gtx.Ops)
		h := 0
		at(gtx, image.Pt(padX, padY), func(gtx layout.Context) { h = body(gtx, width) })
		content := macro.Stop()
		size := image.Pt(gtx.Constraints.Max.X, h+2*padY)
		paint.FillShape(gtx.Ops, c.Background, clip.Rect{Max: size}.Op())
		content.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	}
}

// RampGrid draws the eight ramps as a table: the step numbers standing over the
// columns once, and under them a row per role — its name at the leading edge
// and its nine steps beside it, each in its own cell.
//
// The numbers are headers rather than captions under every swatch. Nine numbers
// repeated eight times is seventy-two numbers on a grid whose whole point is the
// colour, and the column a cell is in is what says which step it is.
//
// A rung one of the picks took carries a mark. Without one the two halves of
// this section sit next to each other describing the same eight roles and never
// point at each other: the picks say "Error 700", the grid has an Error row and
// a 700 column, and joining them is left to a reader holding a number in their
// head while they count columns. The marks join them, and they cost one dot.
func RampGrid(p Palette, c tokens.ColorTokens, ty Type, claims map[rampClaim]bool) func(gtx layout.Context, width int) int {
	rows := rampRows(c)
	return func(gtx layout.Context, width int) int {
		head, rowH := gtx.Dp(RampHeadH), gtx.Dp(RampRowH)
		total := head + len(rows)*rowH
		labelW := min(gtx.Dp(RampLabelW), width)
		cellW := min(gtx.Dp(RampCellMax), max(0, width-labelW)/RampSteps)
		if cellW <= 0 {
			return total
		}
		// The numbers are set in the ink the names are, not a rung quieter. They
		// are the table's only legend — every cell under them is a colour with
		// no other way of saying which step it is — and a legend drawn fainter
		// than the thing it explains is a legend somebody has to go looking for.
		for n := range RampSteps {
			box := image.Rect(labelW+n*cellW, 0, labelW+(n+1)*cellW, head)
			textdraw.FillText(gtx, ty.Shaper, ty.Small, box, 0.5, 0.5, p.Text,
				strconv.Itoa((n+1)*100))
		}
		gutter := gtx.Dp(RampGutter)
		line := gtx.Dp(Hairline)
		for i, r := range rows {
			y := head + i*rowH
			// Ranged against the grid rather than off the margin: set flush left
			// the eight names end wherever their own length puts them, and half
			// of them float a word away from the row they belong to.
			textdraw.FillText(gtx, ty.Shaper, ty.Small,
				image.Rect(0, y, max(0, labelW-gutter), y+rowH), 1, 0.5, p.Text, r.name)
			for n := range RampSteps {
				// A point off the row top and bottom, so two ramps a rung apart
				// are two rows rather than one block of colour.
				cell := image.Rect(labelW+n*cellW, y+1, labelW+(n+1)*cellW, y+rowH-1)
				// The frame is a fill with the colour inset into it, and not a
				// stroke over it, for the reason the window's other swatches
				// wear the same one: a one-point stroke is centred on the
				// boundary and lands as two rows of half-strength antialiasing,
				// which is a line between two colours that differ and nothing at
				// all between a step and a page it is within a shade of. That
				// case is not the exception here, it is the first column — step
				// 100 of every ramp is the ground, near enough — and a grid
				// whose first column dissolves is a grid claiming nine steps and
				// showing eight.
				step := r.ramp.Step((n + 1) * 100)
				paint.FillShape(gtx.Ops, p.CardEdge, clip.Rect(cell).Op())
				if in := cell.Inset(line); !in.Empty() {
					paint.FillShape(gtx.Ops, step, clip.Rect(in).Op())
				}
				if claims[rampClaim{r.name, (n + 1) * 100}] {
					markRung(gtx, cell, step)
				}
			}
		}
		return total
	}
}

// markRung puts the dot on a cell a pick took.
//
// Its ink is measured over the step it stands on rather than taken from the
// page: this mark lands on seventy-two possible grounds running from the page
// itself to nearly black, and one ink chosen for the page would be invisible on
// a third of them. The two candidates are the ends of the tonal axis, which is
// the same pair the derivation itself chooses an on-colour from — the mark is
// doing the same job on the same ground.
func markRung(gtx layout.Context, cell image.Rectangle, step stdcolor.NRGBA) {
	d := min(gtx.Dp(RampMark), min(cell.Dx(), cell.Dy())/2)
	if d <= 0 {
		return
	}
	mid := image.Pt((cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2)
	dot := image.Rect(mid.X-d/2, mid.Y-d/2, mid.X-d/2+d, mid.Y-d/2+d)
	fillRRect(gtx, dot, d/2, markInkOn(step))
}

// markInkOn is the ink a mark takes over one step.
func markInkOn(step stdcolor.NRGBA) stdcolor.NRGBA {
	if vgcolor.RelativeLuminance(step) < markFloor {
		return tokens.White
	}
	return tokens.Black
}

// markFloor is the luminance at which the mark turns over from black to white.
// It is the same half-way point the window uses to tell one side of a scheme
// from the other, and for the same reason: it is the answer to "is this ground
// dark".
const markFloor = 0.5

// PickBoard draws every colour the theme names, in families, across as many
// columns as the window is wide enough for.
//
// A board rather than a list: eleven cells down one column is a column taller
// than the window, and the families are short enough that side by side they can
// all be read at once — which is the comparison worth having, since the question
// a reader brings here is usually about two roles rather than one.
func PickBoard(p Palette, ty Type, groups []pickGroup) func(gtx layout.Context, width int) int {
	return func(gtx layout.Context, width int) int {
		gap := gtx.Dp(PickColGap)
		cols := packPicks(groups, pickColumns(width, gap, gtx.Dp(PickMinW)))
		colW := (width - (len(cols)-1)*gap) / len(cols)
		if colW <= 0 {
			return 0
		}
		tallest := 0
		for i, col := range cols {
			x, y := i*(colW+gap), 0
			for gi, g := range col {
				if gi > 0 {
					y += gtx.Dp(PickGroupGap)
				}
				y += drawFamily(gtx, p, ty, g.name, x, y, colW)
				for _, cell := range g.cells {
					h := gtx.Dp(cell.height())
					drawCell(gtx, p, ty, cell, image.Rect(x, y, x+colW, y+h))
					y += h
				}
			}
			tallest = max(tallest, y)
		}
		return tallest
	}
}

// drawFamily draws the name over one family of cells and the line under it, and
// answers how much of the column the pair took.
//
// The line is what makes the name out-rank the cells below it. The names in
// those cells are set at the size a name is read at, and a family heading a
// point or two larger than them is not a level of its own — it is the same level
// slightly emphasised, which is worse than no level at all. A rule across the
// column is unmistakably a break, costs one point of height, and binds the name
// to what is under it rather than leaving it floating between two families.
func drawFamily(gtx layout.Context, p Palette, ty Type, name string, x, y, w int) int {
	head := gtx.Dp(PickHeadH)
	textdraw.FillText(gtx, ty.Shaper, ty.Head, image.Rect(x, y, x+w, y+head), 0, 0.5, p.Text, name)
	line := gtx.Dp(Hairline)
	paint.FillShape(gtx.Ops, p.Divider, clip.Rect(image.Rect(x, y+head, x+w, y+head+line)).Op())
	return head + line + gtx.Dp(PickHeadGap)
}

// drawCell draws one cell in the slot it was given: the colour, with the ink
// written on it where there is one, and beside them the names over their rules.
//
// The rules are a rung quieter than the names. They are two different kinds of
// thing — one is what the theme calls this colour, the other is where it came
// from — and a reader scanning for a token name has to be able to skip the
// halves that are not names.
func drawCell(gtx layout.Context, p Palette, ty Type, cell pickCell, r image.Rectangle) {
	if r.Dx() <= 0 {
		return
	}
	sw, sh := min(gtx.Dp(PickSwatchW), r.Dx()), min(gtx.Dp(PickSwatchH), r.Dy())
	top := r.Min.Y + (r.Dy()-sh)/2
	box := image.Rect(r.Min.X, top, r.Min.X+sw, top+sh)
	radius := gtx.Dp(InnerR) / 2
	fillRRect(gtx, box, radius, cell.fill)
	// The same frame the window's other swatches wear, for the same reason: a
	// Background swatch on the page it is the background of has no boundary of
	// its own, and without one it reads as a swatch that failed to draw.
	strokeRRect(gtx, box, radius, gtx.Dp(Hairline), p.CardEdge)
	if cell.paired() {
		textdraw.FillText(gtx, ty.Shaper, ty.Label, box, 0.5, 0.5, cell.on, PickGlyph)
	}
	text := box.Max.X + gtx.Dp(PickGap)
	if text >= r.Max.X {
		return
	}
	// The lines stand in a block shorter than the slot, which is where the air
	// between one cell and the next comes from. Set at the slot's full height
	// they came out on an even pitch from the top of the column to the bottom,
	// and an even pitch is what a list of thirty lines looks like rather than
	// eleven cells: nothing in the spacing said which rule belonged to which
	// name.
	title, rule := gtx.Dp(PickTitleH), gtx.Dp(PickRuleH)
	rules := 1
	if cell.paired() {
		rules = 2
	}
	block := min(title+rules*rule, r.Dy())
	y := r.Min.Y + (r.Dy()-block)/2
	textdraw.FillText(gtx, ty.Shaper, ty.Body,
		image.Rect(text, y, r.Max.X, y+title), 0, 0.5, p.Text, cell.title())
	y += title
	textdraw.FillText(gtx, ty.Shaper, ty.Small,
		image.Rect(text, y, r.Max.X, y+rule), 0, 0.5, p.Muted, cell.base.rule)
	if cell.paired() {
		y += rule
		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(text, y, r.Max.X, y+rule), 0, 0.5, p.Muted, cell.ink.rule)
	}
}

// pickColumns is how many columns of cells fit in width px, gap px apart, at no
// less than narrowest px each — at least one, however narrow the window is, and
// never more than the board is worth spreading over.
func pickColumns(width, gap, narrowest int) int {
	if width <= 0 || narrowest <= 0 {
		return 1
	}
	return max(1, min(PickMaxCols, (width+gap)/(narrowest+gap)))
}

// pickLoad is how tall one family stands, in dp: its name, the line under it,
// the air below that, and a slot per cell. It is what the columns are balanced
// by, and it is arithmetic on the constants rather than a measurement, so the
// deal is the same deal before a frame is drawn.
func pickLoad(g pickGroup) int {
	h := int(PickHeadH) + int(PickHeadGap)
	for _, c := range g.cells {
		h += int(c.height())
	}
	return h
}

// pickPackLimit bounds the search below. The board has four families and at most
// three columns, so the whole space is eighty-one arrangements; the guard is
// there so that a fifth family cannot quietly turn a frame into a search.
const pickPackLimit = 256

// packPicks deals the families into n columns so that the tallest column is as
// short as it can be, each family whole and none of them out of order.
//
// Whole, because a family cut across a column boundary is two families as far as
// anybody reading is concerned. In order, because the board is read down one
// column and then down the next, and a deal free to put the fourth family in the
// first column would give the same board two different reading orders at two
// window widths — a reader dragging the window watches a family change
// neighbours. So the arrangements tried are the ones whose column numbers never
// go backwards, and among those the evenest wins; ties go to the first one
// found, which is the one that fills the leftmost column first.
func packPicks(groups []pickGroup, n int) [][]pickGroup {
	if n < 1 {
		n = 1
	}
	cols := make([][]pickGroup, n)
	total := 1
	for range groups {
		total *= n
		if total > pickPackLimit {
			return dealPicks(cols, groups, n, -1)
		}
	}
	best, worst := -1, 0
	for deal := range total {
		load, tallest, back := make([]int, n), 0, false
		for i, g := range groups {
			at := pickColumn(deal, i, len(groups), n)
			if i > 0 && at < pickColumn(deal, i-1, len(groups), n) {
				back = true
				break
			}
			load[at] += pickLoad(g)
			tallest = max(tallest, load[at])
		}
		if back {
			continue
		}
		if best < 0 || tallest < worst {
			best, worst = deal, tallest
		}
	}
	return dealPicks(cols, groups, n, best)
}

// dealPicks lays the families into the columns one arrangement names, or — with
// no arrangement, which is the guard's answer — one family per column and the
// rest onto the last.
func dealPicks(cols [][]pickGroup, groups []pickGroup, n, deal int) [][]pickGroup {
	for i, g := range groups {
		at := min(i, n-1)
		if deal >= 0 {
			at = pickColumn(deal, i, len(groups), n)
		}
		cols[at] = append(cols[at], g)
	}
	return cols
}

// pickColumn reads the column family i is in out of one arrangement: the digits
// of deal in base n, most significant first, so counting up from zero walks the
// arrangements in the order the families are read.
func pickColumn(deal, i, groups, n int) int {
	for range groups - 1 - i {
		deal /= n
	}
	return deal % n
}
