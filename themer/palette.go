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
//
// # Why the pinned bases stand at the end of their own rows
//
// A grid of seventy-two rungs beside a list saying "Primary — the seed, lifted"
// is a window telling a reader that the colour their window is actually painted
// with is somewhere near a cell up there. It is not in the grid: a light
// scheme's Primary is the chosen seed at the seed's own depth, and its
// Secondary and Tertiary are pinned a unit of lightness off their own 700. The
// ramps were the only picture of the palette, and the picture was missing the
// three colours a person came to this window to judge.
//
// So each row ends with the base that role pinned, past a gap and under a word
// of its own, drawn as a chip rather than a cell so that nothing reads it as a
// tenth step. Beside it, on the same row, are the nine rungs, and in the row is
// the dot on the rung the pin is indistinguishable from — which is the relation
// between the two, shown rather than asserted. Where the pin is a rung exactly,
// which is every role in a dark scheme and the four status roles in a light one,
// the chip and the marked cell are the same colour, and that is the answer to
// "is this pin on the ramp" rather than a repetition: a column that appeared on
// one side of the switch and vanished on the other would leave a reader unable
// to tell a role with no pin from a pin the window declined to show. Neutral's
// chip is the one that is absent, because Neutral is the one role the theme
// pins no solid fill for.
//
// # Why the containers and the two ends of the axis are cells of their own
//
// Both are colours a widget is painted with at rest and neither is on a rung.
// A status container is its role's own hue realized at one rung's tone with the
// chroma pulled down to the container dial, which is a colour no cell of the
// grid holds; the two ends of the tonal axis are white and black, which belong
// to no ramp at all and which almost every ink in a light scheme literally is.
// Before they were cells, the only place either appeared was inside somebody
// else's swatch — the axis ends as the two letters written on a base, the
// containers nowhere — and a colour visible only as a letterform is a colour a
// reader cannot judge.
//
// The containers are drawn as what they are: a ground with the mark that reads
// on it, and the mark is a glyph of its own rather than the two letters the
// other pairs carry, because a mark is not text and is not measured against a
// text floor. Writing "Aa" on a container would be this section claiming a
// legibility the derivation never promised.
//
// Each container stands under the role it belongs to rather than in a family of
// containers, which is a decision about repetition. In a light scheme the mark
// on Error's container is Error's own 700 — the same colour as the solid fill
// the role puts a label on — and shown as two families those two landed in two
// columns, four rows each, aligned, with the same colour under two names and
// nothing joining them. One under the other, it reads as what it is.
//
// # What the section leaves out, and why
//
// Three families of colour a component can be painted with are deliberately not
// here. The state walks — hover, pressed, selected and dragged, which walk one
// or two rungs toward the deep end of a ramp — are not colours the palette
// publishes, they are what a component does to a colour it was given while a
// pointer is over it. The disabled colours are the same colour at a fraction of
// its alpha, which is a transparency rather than a member of the palette. The
// focus ring is Neutral 500, so it is already a cell of the grid and already
// marked wherever some other pick took that rung.
//
// The line between what is in and what is out is rest: everything the window
// shows is a colour some widget is painted with while nothing is happening to
// it. Interaction states are a second axis over the whole of this palette — a
// hover for every one of these cells — and drawing them here would quadruple
// the section to say one rule eight times.
package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"math"
	"strconv"
	"strings"

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
	// A cell has no maximum: the nine steps divide whatever the row has left
	// once the names at one end and the chip at the other are reserved, so the
	// row fills its width at every window.
	//
	// It used to stop at ninety-six points, and the judgement is worth writing
	// down because the cap looked like the careful choice. A capped table in a
	// container wider than the cap has width it cannot spend, and there are only
	// two places to put it: past the last cell, which is a ragged right edge
	// under a heading bar and over a picks board that both run the full width;
	// or into the gap before the trailing chip, which was tried and measured —
	// at a 1440-point window it opened a hole of 356 points, nearly four cells
	// wide, and eight chips across a hole that size stop reading as the ends of
	// eight rows and start reading as a strip of their own. Both of those break
	// something the row is, to prevent a swatch from being wide. A wide swatch
	// is a wide swatch; a row that comes apart in the middle is a table that has
	// stopped being one. So the cells stretch.
	//
	// RampMark is the dot on a rung a pick sits at. It is a dot and not a ring, a
	// label or a heavier frame: a fifth of the cells carry one, and anything with
	// an edge of its own at that count turns a table of colour into a table of
	// marks.
	RampMark unit.Dp = 7

	// RampPinW is the chip at the trailing end of a row holding the base that
	// role pinned, RampPinGap the least air there may ever be between it and
	// step 900, and RampPinInset how far it stands off the row's top and bottom.
	// The gap is wider than any space inside the grid and the inset makes the
	// chip shorter than a cell, which between them are what stop nine steps and
	// a pin from reading as ten steps. The chip is the width of the swatch a
	// pick carries below, because it is the same colour shown twice in one
	// section and two sizes would read as two different claims.
	//
	// A least and not a fixed distance because the chips are ranged against the
	// grid's trailing edge rather than set one gap past step 900: nine cells of
	// a whole number of points rarely divide the row exactly, and the eight
	// points or fewer left over land here. Eight points is a gap a hair wider,
	// which is a gap; the same slack left past the chip would be the section
	// ending in a different place from the bar above it.
	RampPinW     unit.Dp = 44
	RampPinGap   unit.Dp = 14
	RampPinInset unit.Dp = 3

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
// And the mark's own clause says what a dot is, in the words anybody reads it
// with. Some picks are not on the rung they are marked at — a light scheme's
// accents are pinned a hair off their own 700, by three parts in 255 — but the
// caption is the wrong place to carry that: hedging it there costs every reader
// a sentence they have to stop and parse, to buy a distinction that is invisible
// on the grid it describes. The cells below say it per colour, beside the colour
// it is about, and a reader who wants to know how a pick was pinned is already
// reading the line that tells them. So the caption is left doing a legend's job.
const (
	RampsLabel = "Palette Ramps"
	RampsHint  = "a dot marks each pick's step · nine steps a role · 100 nearest the page · each row ends with its role's pinned base, and Neutral pins none"
	PicksLabel = "Palette Picks"
	PicksHint  = "every colour the theme names, and where it came from"
	// HintSep joins one clause of a caption to the next, and is therefore where
	// a caption too wide for its bar is cut — see [fitHint]. It is written down
	// because it is two things at once: the mark a reader sees between the
	// facts, and the seam the layout takes them apart at. A caption whose
	// clauses were joined by a character the cut did not know about would
	// truncate to nothing and vanish, silently, at exactly the widths this is
	// meant to serve.
	HintSep = " · "
	// RampPinHead stands over the chips at the ends of the rows, where the step
	// numbers stand over the columns. It is the word the rules under the picks
	// already use for a pinned colour — an ink says it was measured over the
	// base — so a reader meets one name for one thing in both halves of the
	// section. One word over a column is a label and not an explanation, which
	// is why the caption gained a clause naming the column as well; the clause
	// is last, so it is the first thing a narrow window gives up and the legend
	// the dot needs is never the clause that dies.
	RampPinHead = "base"
	// RampPinNone stands where a role pins nothing, which is Neutral and only
	// Neutral. An empty slot at the end of one row of eight is a chip that
	// failed to draw as far as anybody looking can tell, and the one thing the
	// column must not do is look unfinished in the row where the answer is that
	// there is nothing to draw. The caption says which row that is, because a
	// dash is an answer only to a reader who already knows there was a question:
	// told that every row ends with a base, they read the dash as the row that
	// disagrees with the caption rather than as the row the caption is about.
	RampPinNone = "—"
)

// The families the cells are read in. Page and surfaces first because it is the
// ground everything else stands on, and the inverse pair straight after it:
// those two are surfaces as well, borrowed whole from the other side of the
// scheme, and a reader looking for a surface should not have to pass the accents
// and the status roles to find the last two. Then the accents the seed rotates,
// and the status roles it may only tint.
//
// The containers have no family of their own: each stands under the role it
// belongs to, inside Status. The ends of the tonal axis come last — they are
// what the inks in every family above them turned out to be, so they are the
// footnote the rest of the board points at rather than a family a reader goes
// looking for.
const (
	PickPageGroup    = "Page and surfaces"
	PickInverseGroup = "Inverse"
	PickAccentGroup  = "Accents"
	PickStatusGroup  = "Status"
	PickAxisGroup    = "Ink ends"
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
	// PickContainerRule is a status container: its role's own rung, kept at that
	// rung's tone and hue with the chroma pulled down to the one the containers
	// share. Naming the rung is what makes it findable — the container is not
	// that cell of the grid and is the only colour in the section derived from
	// one without being it — and naming what was done to it is what says why the
	// two are not the same swatch.
	PickContainerRule = "%s %d, held at the container chroma"
	// PickMarkRule is the mark read on a container: a rung of the role's own
	// ramp, chosen against the container it stands on rather than against a page.
	// It ends the way an ink's rule ends, because it is the same measurement done
	// to the same end — a colour that has to clear the ground it is drawn on.
	PickMarkRule = "%s %d, measured over the container"
	// PickMarkOff is what a mark says when it is not a rung of its own ramp,
	// which no derivation shipping today produces: the rule the mark is chosen by
	// walks that ramp and can return nothing else.
	PickMarkOff = "measured over the container"
	// The two ends of the tonal axis, each named for the end it is and then
	// answered for: does anything on this board turn out to be it.
	//
	// The second half is the half that earns the cell. These two are the only
	// colours here a reader cannot point at in the grid above — they are on no
	// ramp — so a cell that said what they are and stopped would read as a
	// legend that wandered into a listing of colours the palette uses, and in a
	// dark scheme, where every ink comes off its own role's ramp and neither end
	// is written anywhere, it would be a legend for nothing. Whether the scheme
	// on screen writes in it is read off that scheme's own inks, so the answer
	// turns over with the switch, which is the fact worth having: white is the
	// ink over almost everything on one side and over nothing on the other.
	PickAxisLight = "the tonal axis's light end"
	PickAxisDark  = "the tonal axis's dark end"
	PickAxisInk   = "%s, an ink here"
	PickAxisNoInk = "%s, no ink here"
)

// The token names the cells carry, which are the names in the theme's own
// source. They are the vocabulary a person reads the palette in, and renaming
// them for the sake of a prettier caption would mean this section describes a
// palette nobody can look up.
//
// Two of them are names this section builds rather than reads. The theme holds
// no field for a container — it derives one from a role when it is asked — and
// states the two ends of the tonal axis as colours of the package rather than
// of a palette. So the containers are named out of the words the theme uses for
// them: a role and the container it fills, a role and the mark read on it.
//
// The mark is named a mark rather than an On-colour, which is the one place
// this section departs from the theme's own On-something convention, and it
// departs deliberately. An On-colour is the text a base is read in and is
// measured against a text floor; a mark is a graphic — an icon, a leading edge,
// a rule — and is measured against a lower one. Calling it OnErrorContainer
// would put it in the same category as the six inks above it in the same
// column, at the same moment the cell is drawing it as a shape to say it is
// not.
const (
	BackgroundPick       = "Background"
	TextPick             = "Text"
	SurfacePick          = "Surface"
	DividerPick          = "Divider"
	InverseSurfacePick   = "InverseSurface"
	OnInverseSurfacePick = "OnInverseSurface"
	ContainerPick        = "Container"
	MarkPick             = "Mark"
	WhitePick            = "White"
	BlackPick            = "Black"
)

// rampRow is one row of the ramps grid: a role's name, the nine steps generated
// for it, and the base the derivation pinned for it — which is a colour of its
// own and not always one of the nine. A row whose role has no pinned base
// carries a transparent one, and a transparent chip is not drawn.
type rampRow struct {
	name string
	ramp tokens.Ramp
	pin  stdcolor.NRGBA
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
//
// Primary leads for the same reason again, and now with a second thing to say:
// its chip is the seed itself, lifted, and a reader who wants to know what the
// colour they chose became looks at the first chip in the grid.
func rampRows(c tokens.ColorTokens) []rampRow {
	return []rampRow{
		{PrimaryName, c.Ramps.Primary, c.Primary},
		{SecondaryName, c.Ramps.Secondary, c.Secondary},
		{TertiaryName, c.Ramps.Tertiary, c.Tertiary},
		{ErrorName, c.Ramps.Error, c.Error},
		{SuccessName, c.Ramps.Success, c.Success},
		{WarningName, c.Ramps.Warning, c.Warning},
		{InfoName, c.Ramps.Info, c.Info},
		// Neutral pins no solid fill, so its chip is the one the grid leaves
		// empty rather than a colour invented to fill the column.
		{NeutralName, c.Ramps.Neutral, stdcolor.NRGBA{}},
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
//
// mark says the second colour is a mark and not an ink: it is drawn as a shape
// over the fill rather than as two letters, because it was chosen against the
// non-text floor and letters would claim a legibility nothing measured.
type pickCell struct {
	base, ink pickPart
	fill, on  stdcolor.NRGBA
	mark      bool
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
	groups := []pickGroup{
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
		// Each status role twice over: the solid fill it puts a label on, and
		// under it the ground it fills a whole band with and the mark read on
		// that. Under it, and not in a family of containers of its own, because
		// in a light scheme the mark on Error's container is Error's own 700 —
		// the same colour as the solid fill directly above it — and two columns
		// of four rows each, aligned, both ending in "measured over", read as
		// one table split in half with the same colour twice in it and nothing
		// saying why. Beside its own role the repeat stops being a coincidence
		// and becomes the fact it is: a role marks its quiet ground in the same
		// colour it fills its loud one with.
		{PickStatusGroup, []pickCell{
			pinnedCell(ErrorName, c.Ramps.Error, c.Error, c.OnError, PickJustOff, PickPinned),
			containerCell(ErrorName, c, tokens.RoleError, c.Ramps.Error),
			pinnedCell(SuccessName, c.Ramps.Success, c.Success, c.OnSuccess, PickJustOff, PickPinned),
			containerCell(SuccessName, c, tokens.RoleSuccess, c.Ramps.Success),
			pinnedCell(WarningName, c.Ramps.Warning, c.Warning, c.OnWarning, PickJustOff, PickPinned),
			containerCell(WarningName, c, tokens.RoleWarning, c.Ramps.Warning),
			pinnedCell(InfoName, c.Ramps.Info, c.Info, c.OnInfo, PickJustOff, PickPinned),
			containerCell(InfoName, c, tokens.RoleInfo, c.Ramps.Info),
		}},
	}
	// And the two colours every ink above was chosen between, at last shown as
	// colours rather than as letterforms — and each told whether this scheme
	// wrote anything in it, which is read off the families already built rather
	// than asserted, so the answer is the board's own inks answering for
	// themselves.
	return append(groups, pickGroup{PickAxisGroup, []pickCell{
		alone(axisPart(WhitePick, PickAxisLight, tokens.White, groups), tokens.White),
		alone(axisPart(BlackPick, PickAxisDark, tokens.Black, groups), tokens.Black),
	}})
}

// axisPart is one end of the tonal axis as a cell carries it: which end it is,
// and whether anything above it on the board is written in it.
func axisPart(name, end string, col stdcolor.NRGBA, groups []pickGroup) pickPart {
	rule := PickAxisNoInk
	if inkedWith(groups, col) {
		rule = PickAxisInk
	}
	return pickPart{name: name, rule: fmt.Sprintf(rule, end)}
}

// inkedWith reports whether any cell of these families is written in col.
func inkedWith(groups []pickGroup, col stdcolor.NRGBA) bool {
	for _, g := range groups {
		for _, cell := range g.cells {
			if cell.paired() && cell.on == col {
				return true
			}
		}
	}
	return false
}

// containerCell is one status role's tonal container and the mark read on it:
// one ground, one mark, and the rule each was derived by. The pair is one cell
// for the reason a base and its ink are — the mark is chosen against this exact
// ground and cannot be understood apart from it — and the mark is drawn as a
// disc because that is the kind of thing it was measured to be.
func containerCell(role string, c tokens.ColorTokens, id tokens.Role, r tokens.Ramp) pickCell {
	ground, mark := c.StatusContainer(id), c.OnStatusContainer(id)
	return pickCell{
		base: containerPart(role, r, ground),
		ink:  markPart(role, r, mark),
		fill: ground, on: mark, mark: true,
	}
}

// containerPart is a container as a cell carries it: the rung it was realized
// at, and what was done to that rung to get it.
//
// The rung is found by tone rather than by colour, which is the one place in
// this section a colour is identified by something other than itself. A
// container keeps its rung's lightness and its hue and gives up chroma, so
// comparing bytes finds nothing and measuring a distance finds whichever rung
// happens to be nearest in a space the difference is not in. What survives the
// derivation intact is the tone, so the tone is what says which rung this was.
//
// It claims that rung, and claims it whether or not the container came out
// close enough to be mistaken for it. The dot on the grid marks the step a
// pick's rule names — that is what it has meant since the light accents, which
// are pinned off their rung and marked on it — and a rule that names Error 300
// beside a grid with nothing at Error 300 leaves a reader unable to say which
// of the two is lying. How close the container lands varies with how much
// chroma the rung had to give, so marking only the close ones would put dots
// under two of four rows whose rules are word for word the same.
func containerPart(role string, r tokens.Ramp, ground stdcolor.NRGBA) pickPart {
	step := toneStep(r, ground)
	return pickPart{
		name: role + ContainerPick,
		rule: fmt.Sprintf(PickContainerRule, role, step),
		role: role,
		step: step,
	}
}

// markPart is the mark read on a container: a rung of the role's own ramp,
// chosen against the container rather than against a page, so it claims that
// rung and the grid marks it.
func markPart(role string, r tokens.Ramp, mark stdcolor.NRGBA) pickPart {
	part := pickPart{name: role + MarkPick, role: role}
	if n := stepIn(r, mark); n != 0 {
		part.rule, part.step = fmt.Sprintf(PickMarkRule, role, n), n
		return part
	}
	part.rule = PickMarkOff
	return part
}

// toneStep is the rung of r a colour was realized at, read off the lightness
// the two share. See [containerPart] for why lightness is the question.
func toneStep(r tokens.Ramp, col stdcolor.NRGBA) int {
	tone, _, _ := vgcolor.LabFromNRGBA(col)
	best, at := math.Inf(1), 0
	for i := range r {
		l, _, _ := vgcolor.LabFromNRGBA(r[i])
		if d := math.Abs(l - tone); d < best {
			best, at = d, (i+1)*100
		}
	}
	return at
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
		//
		// In the ink the title is in, and not a rung quieter. The caption's
		// leading clause is the only legend the mark on the grid below has —
		// nothing else on the screen says what a dot on a swatch means — and
		// the numbers over that grid are set in this same ink for the same
		// reason: a legend drawn fainter than the thing it explains is a legend
		// somebody has to go looking for. It is the ink that keeps a caption in
		// the register of the words around it on both sides of the switch, too,
		// which a step of the neutral ramp does not: the quiet step reads at
		// two-thirds of its neighbours' contrast against a dark ground and a
		// third of it against a light one, so a caption set in it is quiet in
		// one scheme and faint in the other.
		if lead := box.Min.X + natural(gtx, ty.Shaper, ty.Label, title) + gtx.Dp(Gap); lead < box.Max.X {
			if fit := fitHint(gtx, ty, hint, box.Max.X-lead); fit != "" {
				textdraw.FillText(gtx, ty.Shaper, ty.Small,
					image.Rect(lead, 0, box.Max.X, size.Y), 1, 0.5, p.Text, fit)
			}
		}
		return layout.Dimensions{Size: size}
	}
}

// fitHint is a section's caption cut to the room it has, at the boundaries the
// caption is written in.
//
// A caption is a list of clauses joined by [HintSep], and what a narrow window
// takes off it is whole clauses from the tail. The shaper's own truncation is
// what this replaces, and it is the wrong cut here for two reasons. It cuts
// mid-word — the caption arrived at a width reading "100 nearest the pa…",
// which is not a shorter caption but a caption that failed to finish — and the
// ellipsis it leaves says the text was clipped, when the fact is that a window
// this wide gets the three clauses that fit and the fourth was never going to
// be read. Cut at the clauses, every word on the bar is a whole word and every
// clause a whole fact, and the line ends where a list ends.
//
// Nothing marks the cut. A list that stops is a list of what fits; an ellipsis
// on the end of it is an invitation to go looking for a clause that was written
// to be the one nobody has to read. The order is what does the work here — the
// caption constants say why their clauses stand in the order they do, and it is
// tail-droppable on purpose — and the cut only has to respect it.
//
// With room for not even the leading clause the caption is dropped whole. Half
// a sentence in a heading bar is not a shorter caption: it is a caption that
// says something else, and the bar already knows how to carry a title alone.
func fitHint(gtx layout.Context, ty Type, hint string, room int) string {
	if natural(gtx, ty.Shaper, ty.Small, hint) <= room {
		return hint
	}
	clauses := strings.Split(hint, HintSep)
	for n := len(clauses) - 1; n > 0; n-- {
		if cut := strings.Join(clauses[:n], HintSep); natural(gtx, ty.Shaper, ty.Small, cut) <= room {
			return cut
		}
	}
	return ""
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
		// The chips are reserved out of the width before the cells are measured,
		// and given up only when reserving them would leave the nine steps under
		// a point each. A grid too narrow to hold both is a grid too narrow to
		// read, and the steps are the thing the chips are read against.
		pinW, pinGap := gtx.Dp(RampPinW), gtx.Dp(RampPinGap)
		if width-labelW-pinGap-pinW < RampSteps {
			pinW, pinGap = 0, 0
		}
		cellW := max(0, width-labelW-pinGap-pinW) / RampSteps
		if cellW <= 0 {
			return total
		}
		// Ranged against the trailing edge, which is where the heading bar's own
		// caption ends and where the picks board below ends: the names stand at
		// the leading margin, the chips at the trailing one, and the section has
		// one right edge at every width instead of a grid that stops wherever
		// nine whole-point cells happen to run out. The width reserved above is
		// what guarantees the chips cannot reach step 900 — the cells were
		// divided out of a width the chip and its least gap had already been
		// taken from, so the slack that lands here is never less than
		// [RampPinGap] and never more than eight points past it.
		pinX := width - pinW
		// The numbers are set in the ink the names are, not a rung quieter. They
		// are the table's only legend — every cell under them is a colour with
		// no other way of saying which step it is — and a legend drawn fainter
		// than the thing it explains is a legend somebody has to go looking for.
		for n := range RampSteps {
			box := image.Rect(labelW+n*cellW, 0, labelW+(n+1)*cellW, head)
			textdraw.FillText(gtx, ty.Shaper, ty.Small, box, 0.5, 0.5, p.Text,
				strconv.Itoa((n+1)*100))
		}
		// A word rather than a tenth number, because the chips under it are not
		// a step and the header row is where a reader finds out.
		if pinW > 0 {
			textdraw.FillText(gtx, ty.Shaper, ty.Small,
				image.Rect(pinX, 0, pinX+pinW, head), 0.5, 0.5, p.Text, RampPinHead)
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
			if pinW > 0 {
				slot := image.Rect(pinX, y+gtx.Dp(RampPinInset), pinX+pinW, y+rowH-gtx.Dp(RampPinInset))
				if r.pin.A != 0 {
					markPin(gtx, p, slot, r.pin)
				} else {
					textdraw.FillText(gtx, ty.Shaper, ty.Small, slot, 0.5, 0.5, p.Muted, RampPinNone)
				}
			}
		}
		return total
	}
}

// markPin draws the base a role pinned, at the end of that role's row.
//
// It is a rounded chip with a frame rather than a square butted against its
// neighbours, which is the whole of what separates it from the nine cells it
// stands beside — that and the gap. The frame is the one every other swatch in
// the window wears, and it is here for the reason it is there: a pinned base
// can be any colour a seed produces, pale ones included, and a pale chip on a
// pale ground with no boundary reads as a chip that failed to draw.
func markPin(gtx layout.Context, p Palette, box image.Rectangle, pin stdcolor.NRGBA) {
	if box.Empty() {
		return
	}
	radius := gtx.Dp(InnerR) / 2
	fillRRect(gtx, box, radius, pin)
	strokeRRect(gtx, box, radius, gtx.Dp(Hairline), p.CardEdge)
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

// markInkOn is the ink a mark takes over one step: whichever end of the tonal
// axis reads better on it, measured.
//
// Measured, and not decided by asking whether the step is dark. Half-way up the
// luminance scale is the answer to "which side of a scheme is this", and it is
// the wrong question here: a step at a luminance of a third is called dark by
// that test and carries white at under three to one, while black on the same
// step reads at nearly eight. That band is where the mid rungs of a saturated
// hue live — a light red, a mid amber — and this window puts marks on them the
// moment a status role's container names its mark. So the two candidates are
// tried and the better one kept, which is what the derivation itself does when
// it picks an on-colour, and the mark is doing the same job on the same ground.
func markInkOn(step stdcolor.NRGBA) stdcolor.NRGBA {
	if vgcolor.ContrastRatio(tokens.White, step) > vgcolor.ContrastRatio(tokens.Black, step) {
		return tokens.White
	}
	return tokens.Black
}

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
	switch {
	case cell.mark:
		// In the colour the derivation chose, over the ground it chose it
		// against: this cell is the specimen of that pairing, and a mark drawn
		// in anything else would be a claim nobody could check by looking.
		markGlyph(gtx, box, cell.on)
	case cell.paired():
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

// markGlyph draws a status role's mark on its own container: a square, which is
// the plainest thing that is a graphic and not a letter.
//
// A square and not a disc, though a disc is plainer still, because the grid
// above already spends a disc on something else — a dot there says a pick came
// off this rung — and one shape carrying two unrelated meanings in one section
// is a legend a reader has to hold two entries for. The square is nearly the
// same weight of ink and cannot be confused with the marker.
//
// The size is a share of the swatch rather than a number of its own. A mark
// measured against the non-text floor is legible at the size a graphic is drawn
// at and not at the size text is, and a mark small enough to pass for a full
// stop would understate a contrast the derivation actually achieved.
func markGlyph(gtx layout.Context, box image.Rectangle, mark stdcolor.NRGBA) {
	d := min(box.Dx(), box.Dy()) / 2
	if d <= 0 {
		return
	}
	mid := image.Pt((box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2)
	fillRRect(gtx, image.Rect(mid.X-d/2, mid.Y-d/2, mid.X-d/2+d, mid.Y-d/2+d), gtx.Dp(Hairline), mark)
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

// packPicks deals the families into n columns so that the tallest column is as
// short as it can be, each family whole and none of them out of order.
//
// Whole, because a family cut across a column boundary is two families as far as
// anybody reading is concerned. In order, because the board is read down one
// column and then down the next, and a deal free to put the fourth family in the
// first column would give the same board two different reading orders at two
// window widths — a reader dragging the window watches a family change
// neighbours.
//
// So a deal is not an assignment of families to columns, it is a run of
// boundaries in the reading order, and the search is over where the boundaries
// fall. Which is the difference between a handful of arrangements and a number
// that grows by a factor of three every time the board gains a family: this
// section stood at four families and grew to six the moment the containers and
// the ends of the axis joined it, and enumerating assignments would have gone
// from eighty-one arrangements a frame to seven hundred and twenty-nine, to
// find what twenty-eight runs of boundaries answer.
func packPicks(groups []pickGroup, n int) [][]pickGroup {
	if n < 1 {
		n = 1
	}
	cols, cuts := make([][]pickGroup, n), bestCuts(groups, n)
	at := 0
	for i, g := range groups {
		for at < len(cuts) && i >= cuts[at] {
			at++
		}
		cols[at] = append(cols[at], g)
	}
	return cols
}

// bestCuts is where the column boundaries fall in the evenest in-order deal:
// n-1 indices into groups, never going backwards, cut[j] naming the first
// family of column j+1. A boundary at the end of the run leaves its column
// empty, which is what a board with fewer families than columns comes to.
//
// Ties go to the first arrangement found, and the walk starts each boundary as
// far along the run as it can go, so the first arrangement found — and the one
// a tie keeps — is the one that fills the leftmost column first.
func bestCuts(groups []pickGroup, n int) []int {
	cuts, best, tallest := make([]int, n-1), make([]int, n-1), -1
	var walk func(j, from int)
	walk = func(j, from int) {
		if j == len(cuts) {
			if got := cutsTallest(groups, cuts); tallest < 0 || got < tallest {
				tallest = got
				copy(best, cuts)
			}
			return
		}
		for cut := len(groups); cut >= from; cut-- {
			cuts[j] = cut
			walk(j+1, cut)
		}
	}
	walk(0, 0)
	return best
}

// cutsTallest is the height of the tallest column one run of boundaries deals.
func cutsTallest(groups []pickGroup, cuts []int) int {
	tallest, load, at := 0, 0, 0
	for i, g := range groups {
		for at < len(cuts) && i >= cuts[at] {
			at, load = at+1, 0
		}
		load += pickLoad(g)
		tallest = max(tallest, load)
	}
	return tallest
}
