// theme_palette.go is the themer's palette section, copied: the ramps grid
// (roles × steps 100–900, colour names) and the named picks with the rule
// that chose each one, every rule read off the colours themselves so the
// section cannot drift from the palette it describes.
//
// Copied from workbench/themer (palette.go and the helpers it leans on in
// theme.go and view.go) rather than called or re-invented: the section
// lives in that app's main package, so there is nothing to import, and the
// palette story deliberately has one telling — this file reproduces it
// instead of writing a third. The full design rationale for every choice
// here — why picks carry rules, why a base and its ink are one cell, why
// the chips stand past the grid, the measured tolerances — is in
// workbench/themer/palette.go's comments; the copies here keep only what a
// maintainer needs at the point of use. The one deliberate deviation:
// TypeFrom takes the shaper explicitly, so goldens can hand in the
// deterministic one instead of the theme's cached system shaper.

package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The section's own furniture measurements, at the sizes the themer's page
// gives them.
const (
	Gap      unit.Dp = 14 // between stacked parts
	Hairline unit.Dp = 1  // resting outlines
	InnerR   unit.Dp = 8  // swatch and chip corners
)

// The section's dimensions. See themer/palette.go for the measurements
// behind each number.
const (
	// PaletteHeadH is a section heading, at the height the inventory's own
	// section headers stand at.
	PaletteHeadH unit.Dp = 32

	// RampSteps is how many steps a ramp has.
	RampSteps = 9
	// RampLabelW is the column the ramp names stand in and RampGutter what
	// stands between the longest of them and the first swatch.
	RampLabelW unit.Dp = 88
	RampGutter unit.Dp = 14
	// RampRowH is one ramp, RampHeadH the line of step numbers above the
	// columns. Cells have no maximum width: the nine steps divide whatever
	// the row has left, so the row fills its width at every window.
	RampRowH  unit.Dp = 24
	RampHeadH unit.Dp = 18
	// RampMark is the dot on a rung a pick sits at.
	RampMark unit.Dp = 7

	// RampPinW is the chip at the trailing end of a row holding the base
	// that role pinned, RampPinGap the least air between it and step 900,
	// and RampPinInset how far it stands off the row's top and bottom —
	// between them what stops nine steps and a pin reading as ten steps.
	RampPinW     unit.Dp = 44
	RampPinGap   unit.Dp = 14
	RampPinInset unit.Dp = 3

	// PickSwatchW and PickSwatchH are one cell's colour: wide enough to
	// carry two letters at specimen size with air above and below them.
	PickSwatchW unit.Dp = 44
	PickSwatchH unit.Dp = 26
	// PickPairH is a base and its ink (three lines); PickCellH a colour
	// with no ink (two).
	PickPairH unit.Dp = 62
	PickCellH unit.Dp = 44
	// PickTitleH is the line the names are on and PickRuleH one rule.
	PickTitleH unit.Dp = 18
	PickRuleH  unit.Dp = 15
	// PickHeadH is the name over one family, PickHeadGap the air under its
	// line, PickGroupGap what stands above the next family's name.
	PickHeadH    unit.Dp = 20
	PickHeadGap  unit.Dp = 8
	PickGroupGap unit.Dp = 22
	// PickGap is swatch to text; PickColGap between columns of cells.
	PickGap    unit.Dp = 10
	PickColGap unit.Dp = 24
	// PickMaxCols is as wide as the board spreads.
	PickMaxCols = 3
)

// What the section says about itself. The captions are clause lists joined
// by HintSep, tail-droppable on purpose — see fitHint.
const (
	RampsLabel = "Palette Ramps"
	RampsHint  = "a dot marks where each pick lives · nine steps a role · 100 nearest the page · each row ends with its role's pinned base, and Neutral pins none"
	PicksLabel = "Palette Picks"
	PicksHint  = "every colour the theme names, and where it came from"
	// HintSep joins one clause of a caption to the next, and is the seam a
	// caption too wide for its bar is cut at.
	HintSep = " · "
	// RampPinHead stands over the chips; RampPinNone where a role pins
	// nothing, which is Neutral and only Neutral.
	RampPinHead = "base"
	RampPinNone = "—"
)

// The families the cells are read in.
const (
	PickPageGroup    = "Page and surfaces"
	PickInverseGroup = "Inverse"
	PickAccentGroup  = "Accents"
	PickStatusGroup  = "Status"
	PickAxisGroup    = "Ink ends"
)

// The role names, said once: the ramp rows' labels and the cells' names both.
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

// The rules, as they are written under a pair of names. See
// themer/palette.go for why each is worded the way it is.
const (
	PickGlyph        = "Aa"
	PickPairSep      = " / "
	PickMeasured     = ", measured over the base"
	PickMeasuredOver = ", measured over %s"
	PickWhite        = "white"
	PickBlack        = "black"
	PickMeasuredOn   = "measured over the base"
	PickSeed         = "the seed, lifted"
	PickJustOff      = "pinned just off %s %d"
	PickSeedNear     = "the seed, lifted, just off %s %d"
	PickPinned       = "pinned off the ramp"
	PickOffRamp      = "off the neutral ramp"
	PickOtherLight   = "the light scheme's %s"
	PickOtherDark    = "the dark scheme's %s"
	PickOtherSide    = "the other scheme's %s"
	PickSurfaceRole  = "Surface"
	PickTextRole     = "Text"

	PickContainerRule = "%s %d, held at the container chroma"
	PickMarkRule      = "%s %d, measured over the container"
	PickMarkOff       = "measured over the container"

	PickAxisLight = "the tonal axis's light end"
	PickAxisDark  = "the tonal axis's dark end"
	PickAxisInk   = "%s, an ink here"
	PickAxisNoInk = "%s, no ink here"
)

// The token names the cells carry — the names in the theme's own source.
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

// edgeFloor is WCAG 1.4.11's contrast floor for a graphic that carries
// meaning without being text — 3:1. An edge is exactly that: it is the whole
// of what says where one plane ends and the next begins, so it is not
// decoration and owes its ground this much.
const edgeFloor = 3.0

// Palette is this section's view of the colour tokens: every colour it
// draws its own furniture with, named for what it draws.
type Palette struct {
	Backdrop stdcolor.NRGBA
	Surface  stdcolor.NRGBA
	Divider  stdcolor.NRGBA
	CardEdge stdcolor.NRGBA
	Edge     stdcolor.NRGBA
	Text     stdcolor.NRGBA
	Muted    stdcolor.NRGBA
	Accent   stdcolor.NRGBA
	OnAccent stdcolor.NRGBA
	Problem  stdcolor.NRGBA
}

// PaletteFrom resolves the palette in the token vocabulary.
func PaletteFrom(c tokens.ColorTokens) Palette {
	return Palette{
		Backdrop: c.Background,
		Surface:  c.Ramps.Neutral.Step(200),
		Divider:  c.Ramps.Neutral.Step(300),
		CardEdge: c.Ramps.Neutral.Step(400),
		// The heaviest edge, derived rather than named: the neutral rung the
		// ramp measures as reaching the graphic floor against Surface — the
		// level-1 storey this section's cards fill at, and the ground any
		// edge in this palette is drawn on. Named at step 500 it meant two
		// different weights, 2.35:1 there in the light scheme against 5.94:1
		// in the dark, from a line that looks scheme-neutral.
		Edge:     c.MarkOn(tokens.RoleNeutral, c.SurfaceAt(tokens.Level1), edgeFloor),
		Text:     c.Text,
		Muted:    c.Ramps.Neutral.Step(700),
		Accent:   c.Primary,
		OnAccent: c.OnPrimary,
		Problem:  c.Error,
	}
}

// Type is this section's view of the theme's Typography: the roles it
// draws directly, as textdraw styles, plus the shaper it draws them with.
type Type struct {
	Shaper *text.Shaper
	Head   textdraw.TextStyle // TitleSmall: family names on the picks board
	Label  textdraw.TextStyle // LabelLarge: section labels, the pair's "Aa"
	Body   textdraw.TextStyle // BodyMedium: a cell's names
	Small  textdraw.TextStyle // BodySmall: rules, step numbers, captions
}

// TypeFrom resolves the roles from the typography, drawing with the shaper
// handed in — the theme's cached one in the app, the deterministic one in
// goldens. (The themer's original reads the shaper off the typography; the
// explicit parameter is this copy's one deviation.)
func TypeFrom(shaper *text.Shaper, t tokens.Typography) Type {
	return Type{
		Shaper: shaper,
		Head:   palTextStyle(t.TitleSmall),
		Label:  palTextStyle(t.LabelLarge),
		Body:   palTextStyle(t.BodyMedium),
		Small:  palTextStyle(t.BodySmall),
	}
}

// Ellipsis is the mark a run of text wears when it was cut short.
const Ellipsis = "…"

// palTextStyle converts one Typography role to a single-line textdraw style.
func palTextStyle(ts tokens.TextStyle) textdraw.TextStyle {
	f := font.Font{Typeface: font.Typeface(ts.Typeface)}
	if ts.Weight != 0 {
		f.Weight = tokens.FontWeight(ts.Weight)
	}
	return textdraw.TextStyle{Font: f, Alignment: textdraw.Start, Size: unit.Sp(ts.Size), MaxLines: 1, Truncator: Ellipsis}
}

// isDark reports whether a palette is the dark side of its pair, by the
// luminance of the surface everything else is drawn on.
func isDark(c tokens.ColorTokens) bool {
	return vgcolor.RelativeLuminance(c.Background) < 0.5
}

// schemeCounterpart is the side of the theme the window is not showing,
// derived the way the themer derives it with nothing chosen: from the
// brand base the palette on screen pins — c.Primary, which is the seed
// itself on the light side and the nearest thing to it a dark palette says
// about itself. The counterpart feeds only the inverse pair's rules.
func schemeCounterpart(c tokens.ColorTokens) tokens.ColorTokens {
	light, dark := tokens.FromSeed(c.Primary)
	if isDark(c) {
		return light
	}
	return dark
}

// natural is how wide a string wants to be, unconstrained by the room it
// is about to be given.
func natural(gtx layout.Context, shaper *text.Shaper, style textdraw.TextStyle, str string) int {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	return textdraw.MeasureText(gtx, shaper, style, str).X
}

// fillRRect paints a rounded rectangle.
func fillRRect(gtx layout.Context, r image.Rectangle, radius int, c stdcolor.NRGBA) {
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// strokeRRect outlines a rounded rectangle, inset by half the stroke width
// so the whole line lands inside the rectangle rather than half outside it.
func strokeRRect(gtx layout.Context, r image.Rectangle, radius, width int, c stdcolor.NRGBA) {
	if width <= 0 {
		return
	}
	half := float32(width) / 2
	inner := image.Rect(r.Min.X+width/2, r.Min.Y+width/2, r.Max.X-width/2, r.Max.Y-width/2)
	path := clip.UniformRRect(inner, max(0, radius-width/2)).Path(gtx.Ops)
	defer clip.Stroke{Path: path, Width: half * 2}.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// at offsets the operations w records to origin, leaving the caller's
// coordinate system untouched.
func at(gtx layout.Context, origin image.Point, w func(gtx layout.Context)) {
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	w(gtx)
}

// rampRow is one row of the ramps grid: a role's name, the nine steps
// generated for it, and the base the derivation pinned for it — which is a
// colour of its own and not always one of the nine. A row whose role has
// no pinned base carries a transparent one, and a transparent chip is not
// drawn.
type rampRow struct {
	name string
	ramp tokens.Ramp
	pin  stdcolor.NRGBA
}

// rampRows is the grid, in the order it is read: the seed's own roles
// lead, the status roles follow, and Neutral — the row the seed has least
// to say about — is last.
func rampRows(c tokens.ColorTokens) []rampRow {
	return []rampRow{
		{PrimaryName, c.Ramps.Primary, c.Primary},
		{SecondaryName, c.Ramps.Secondary, c.Secondary},
		{TertiaryName, c.Ramps.Tertiary, c.Tertiary},
		{ErrorName, c.Ramps.Error, c.Error},
		{SuccessName, c.Ramps.Success, c.Success},
		{WarningName, c.Ramps.Warning, c.Warning},
		{InfoName, c.Ramps.Info, c.Info},
		// Neutral pins no solid fill, so its chip is the one the grid
		// leaves empty rather than a colour invented to fill the column.
		{NeutralName, c.Ramps.Neutral, stdcolor.NRGBA{}},
	}
}

// rampClaim is one rung a pick took: the row it is on and the step it is.
type rampClaim struct {
	role string
	step int
}

// rampClaims is every rung the picks below the grid took, which is what
// the grid marks. It is read off the picks rather than written down beside
// them, so the two halves of this section cannot drift.
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

// pickPart is one colour token in a cell: what the theme calls it, what
// chose it, and — where what chose it was a rung of a ramp — which row and
// which step, so the grid above can mark the rung this pick took.
type pickPart struct {
	name, rule string
	role       string // the ramp row the rule names, empty when it names none
	step       int    // the rung on that row, 0 when the colour is on none
}

// pickCell is one thing the palette decided: a base and the ink measured
// over it — one swatch, two names, two rules — or, where the theme names
// no ink for a colour, that colour on its own. mark says the second colour
// is a mark and not an ink: drawn as a shape, not letters.
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
// read, with each rule resolved against the colours themselves. c is the
// side of the pair on screen and other the side it is not — the inverse
// pair is the only thing here that comes from across the scheme.
func paletteGroups(c, other tokens.ColorTokens, dark bool) []pickGroup {
	n := c.Ramps.Neutral
	alone := func(base pickPart, fill stdcolor.NRGBA) pickCell {
		return pickCell{base: base, fill: fill}
	}
	groups := []pickGroup{
		{PickPageGroup, []pickCell{
			// The page and the ink it is read in: two pins, still one
			// decision — the ink is chosen for that ground.
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
		// Each status role twice over: the solid fill it puts a label on,
		// and under it the container it fills a band with and the mark
		// read on that — under its own role so the repeated colour reads
		// as the fact it is.
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
	// The two colours every ink above was chosen between, each told
	// whether this scheme wrote anything in it — read off the families
	// already built rather than asserted.
	return append(groups, pickGroup{PickAxisGroup, []pickCell{
		alone(axisPart(WhitePick, PickAxisLight, tokens.White, groups), tokens.White),
		alone(axisPart(BlackPick, PickAxisDark, tokens.Black, groups), tokens.Black),
	}})
}

// axisPart is one end of the tonal axis as a cell carries it: which end it
// is, and whether anything above it on the board is written in it.
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

// containerCell is one status role's tonal container and the mark read on
// it: one ground, one mark, and the rule each was derived by.
func containerCell(role string, c tokens.ColorTokens, id tokens.Role, r tokens.Ramp) pickCell {
	ground, mark := c.StatusContainer(id), c.OnStatusContainer(id)
	return pickCell{
		base: containerPart(role, r, ground),
		ink:  markPart(role, r, mark),
		fill: ground, on: mark, mark: true,
	}
}

// containerPart is a container as a cell carries it: the rung it was
// realized at — found by tone, the one thing the derivation keeps intact —
// and what was done to that rung to get it.
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
// chosen against the container rather than against a page.
func markPart(role string, r tokens.Ramp, mark stdcolor.NRGBA) pickPart {
	part := pickPart{name: role + MarkPick, role: role}
	if n := stepIn(r, mark); n != 0 {
		part.rule, part.step = fmt.Sprintf(PickMarkRule, role, n), n
		return part
	}
	part.rule = PickMarkOff
	return part
}

// toneStep is the rung of r a colour was realized at, read off the
// lightness the two share.
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

// pinnedCell is one role's cell: the base the derivation pinned and the
// ink it measured over that exact colour.
func pinnedCell(role string, r tokens.Ramp, base, ink stdcolor.NRGBA, near, off string) pickCell {
	return pickCell{
		base: basePart(role, r, base, near, off),
		ink:  inkPart(role, r, ink),
		fill: base, on: ink,
	}
}

// stepIn reports which step of r the colour is, or 0 when it is not on the
// ramp at all — comparing bytes, because a pin near a rung is a different
// colour, which is the distinction this section exists to draw.
func stepIn(r tokens.Ramp, col stdcolor.NRGBA) int {
	for i := range r {
		if r[i] == col {
			return (i + 1) * 100
		}
	}
	return 0
}

// basePart is a pinned base as a cell carries it, in three cases: the rung
// it landed on, the rung it is indistinguishable from and how it came to
// be beside rather than on it (near), and — where no rung is near — how it
// was pinned and nothing else (off).
func basePart(role string, r tokens.Ramp, col stdcolor.NRGBA, near, off string) pickPart {
	if n := stepIn(r, col); n != 0 {
		return pickPart{name: role, rule: fmt.Sprintf("%s %d", role, n), role: role, step: n}
	}
	if n := nearestStep(r, col); n != 0 {
		return pickPart{name: role, rule: fmt.Sprintf(near, role, n), role: role, step: n}
	}
	return pickPart{name: role, rule: off, role: role}
}

// nearestStep is the rung of r a colour is indistinguishable from, or 0
// when it is distinguishable from all nine. Measured in OKLab; the
// tolerance's floor and ceiling are measured in the themer's tests.
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
// rung as far as anybody looking is concerned.
const rungTolerance = 0.018

// pinRung is the rung a pinned base claims: the step it is exactly, else
// the one it is indistinguishable from, else 0 — asked in the order
// basePart asks, so the grid and the rule cannot disagree.
func pinRung(r tokens.Ramp, pin stdcolor.NRGBA) int {
	if n := stepIn(r, pin); n != 0 {
		return n
	}
	return nearestStep(r, pin)
}

// oklabDistance is how far apart two colours are, perceptually.
func oklabDistance(a, b stdcolor.NRGBA) float64 {
	l1, a1, b1 := vgcolor.OKLabFromNRGBA(a)
	l2, a2, b2 := vgcolor.OKLabFromNRGBA(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// neutralPart is a surface, a border or the body ink: all of them
// resolutions of one neutral step.
func neutralPart(name string, n tokens.Ramp, col stdcolor.NRGBA) pickPart {
	part := basePart(NeutralName, n, col, PickJustOff, PickOffRamp)
	part.name = name
	return part
}

// inkPart names what the derivation put over the base and kept: one of the
// two ends of the tonal axis, named for the base it was measured over, or
// the role's own deepest rung.
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

// inversePart is one member of the inverse pair. It claims no rung: the
// colour is a step of the counterpart scheme's neutral ramp, and that ramp
// is not one of the eight on screen.
func inversePart(name string, col, counterpart stdcolor.NRGBA, role string, dark bool) pickPart {
	return pickPart{name: name, rule: inverseRule(col, counterpart, role, dark)}
}

// inverseRule names the counterpart role an inverse colour is, in the
// words of the side it came from — checked byte for byte rather than
// asserted, falling back to naming the side alone.
func inverseRule(col, counterpart stdcolor.NRGBA, role string, dark bool) string {
	if col != counterpart {
		return fmt.Sprintf(PickOtherSide, role)
	}
	if dark {
		return fmt.Sprintf(PickOtherLight, role)
	}
	return fmt.Sprintf(PickOtherDark, role)
}

// PaletteSectionRows is how many rows PaletteRows returns: a heading and a
// body for each of the two sections.
const PaletteSectionRows = 4

// PaletteRows is the section as rows of a column: the ramps under their
// own heading, and the picks under theirs.
func PaletteRows(p Palette, c, other tokens.ColorTokens, ty Type, dark bool) []layout.Widget {
	groups := paletteGroups(c, other, dark)
	return []layout.Widget{
		paletteHeading(p, c, ty, RampsLabel, RampsHint),
		paletteBody(c, RampGrid(p, c, ty, rampClaims(groups))),
		paletteHeading(p, c, ty, PicksLabel, PicksHint),
		paletteBody(c, PickBoard(p, c, ty, groups)),
	}
}

// paletteHeading labels one of the two sections, with what it is at the
// leading edge and how to read it at the trailing one. The caption is in
// the title's own ink — it is the only legend the mark on the grid has —
// and takes what the title leaves rather than the whole bar.
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
		if lead := box.Min.X + natural(gtx, ty.Shaper, ty.Label, title) + gtx.Dp(Gap); lead < box.Max.X {
			if fit := fitHint(gtx, ty, hint, box.Max.X-lead); fit != "" {
				textdraw.FillText(gtx, ty.Shaper, ty.Small,
					image.Rect(lead, 0, box.Max.X, size.Y), 1, 0.5, p.Text, fit)
			}
		}
		return layout.Dimensions{Size: size}
	}
}

// fitHint is a section's caption cut to the room it has, at the clause
// boundaries the caption is written in — whole clauses off the tail, and
// nothing marking the cut. With room for not even the leading clause the
// caption is dropped whole.
func fitHint(gtx layout.Context, ty Type, hint string, room int) string {
	if natural(gtx, ty.Shaper, ty.Small, hint) <= room {
		return hint
	}
	clauses := strings.Split(hint, HintSep)
	heads := make([]string, 0, len(clauses))
	for n := len(clauses) - 1; n > 0; n-- {
		heads = append(heads, strings.Join(clauses[:n], HintSep))
	}
	return longestHead(gtx, ty.Shaper, ty.Small, heads, "", room)
}

// fitLine is one line of the picks board cut to the room its column has:
// clauses first with nothing marking the cut, words second with an
// ellipsis, and — with room for not even the first word — the shaper's own
// truncation.
func fitLine(gtx layout.Context, shaper *text.Shaper, style textdraw.TextStyle, line string, room int) string {
	if room <= 0 || natural(gtx, shaper, style, line) <= room {
		return line
	}
	if cut := longestHead(gtx, shaper, style, lineHeads(line, true), "", room); cut != "" {
		return cut
	}
	if cut := longestHead(gtx, shaper, style, lineHeads(line, false), Ellipsis, room); cut != "" {
		return cut
	}
	return line
}

// lineHeads is every head this line can be cut down to, longest first: the
// head at each of its clause boundaries when clauses is set, and at each
// of its word boundaries when it is not. The separator itself is taken off
// the head.
func lineHeads(line string, clauses bool) []string {
	var heads []string
	for i := len(line) - 1; i > 0; i-- {
		if line[i] != ' ' {
			continue
		}
		if clauses && !strings.HasSuffix(line[:i], ",") &&
			!strings.HasSuffix(line[:i], " ·") && !strings.HasSuffix(line[:i], " /") {
			continue
		}
		if head := strings.TrimRight(line[:i], " ,·/"); head != "" {
			heads = append(heads, head)
		}
	}
	return heads
}

// longestHead is the first of these heads that fits the room with the tail
// on the end of it, and "" when none of them does.
func longestHead(gtx layout.Context, shaper *text.Shaper, style textdraw.TextStyle, heads []string, tail string, room int) string {
	for _, head := range heads {
		if natural(gtx, shaper, style, head+tail) <= room {
			return head + tail
		}
	}
	return ""
}

// paletteBody lays one section's content out on the page's own ground,
// inside the margin the inventory's bodies keep. The content is drawn
// before the ground is painted and replayed over it, because the height is
// the content's own.
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

// RampGrid draws the eight ramps as a table: the step numbers standing
// over the columns once, and under them a row per role — its name at the
// leading edge, its nine steps beside it, and the base it pinned as a chip
// past the trailing gap. A rung one of the picks took carries a mark.
func RampGrid(p Palette, c tokens.ColorTokens, ty Type, claims map[rampClaim]bool) func(gtx layout.Context, width int) int {
	rows := rampRows(c)
	return func(gtx layout.Context, width int) int {
		head, rowH := gtx.Dp(RampHeadH), gtx.Dp(RampRowH)
		total := head + len(rows)*rowH
		labelW := min(gtx.Dp(RampLabelW), width)
		// The chips are reserved out of the width before the cells are
		// measured, and given up only when reserving them would leave the
		// nine steps under a point each.
		pinW, pinGap := gtx.Dp(RampPinW), gtx.Dp(RampPinGap)
		if width-labelW-pinGap-pinW < RampSteps {
			pinW, pinGap = 0, 0
		}
		cellW := max(0, width-labelW-pinGap-pinW) / RampSteps
		if cellW <= 0 {
			return total
		}
		// The chips are ranged against the grid's trailing edge, so the
		// section has one right edge at every width.
		pinX := width - pinW
		// The numbers are the table's only legend, in the ink the names
		// are — a legend drawn fainter than the thing it explains is a
		// legend somebody has to go looking for.
		for n := range RampSteps {
			box := image.Rect(labelW+n*cellW, 0, labelW+(n+1)*cellW, head)
			textdraw.FillText(gtx, ty.Shaper, ty.Small, box, 0.5, 0.5, p.Text,
				strconv.Itoa((n+1)*100))
		}
		// A word rather than a tenth number, because the chips under it
		// are not a step.
		if pinW > 0 {
			textdraw.FillText(gtx, ty.Shaper, ty.Small,
				image.Rect(pinX, 0, pinX+pinW, head), 0.5, 0.5, p.Text, RampPinHead)
		}
		gutter := gtx.Dp(RampGutter)
		line := gtx.Dp(Hairline)
		for i, r := range rows {
			y := head + i*rowH
			// Ranged against the grid rather than off the margin, so no
			// name floats a word away from the row it belongs to.
			textdraw.FillText(gtx, ty.Shaper, ty.Small,
				image.Rect(0, y, max(0, labelW-gutter), y+rowH), 1, 0.5, p.Text, r.name)
			for n := range RampSteps {
				// A point off the row top and bottom, so two ramps a rung
				// apart are two rows rather than one block of colour.
				cell := image.Rect(labelW+n*cellW, y+1, labelW+(n+1)*cellW, y+rowH-1)
				// The frame is a fill with the colour inset into it — a
				// centred stroke dissolves exactly where the frame is
				// needed, on a step within a shade of the page. Its ink is
				// one colour for the whole section; see edgeIn.
				step := r.ramp.Step((n + 1) * 100)
				paint.FillShape(gtx.Ops, edgeIn(c), clip.Rect(cell).Op())
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
					markPin(gtx, c, slot, r.pin)
					// A pin that claims no rung has no cell to dot; the
					// dot lands on the chip instead — the pinned colour
					// lives here.
					if pinRung(r.ramp, r.pin) == 0 {
						markRung(gtx, slot, r.pin)
					}
				} else {
					textdraw.FillText(gtx, ty.Shaper, ty.Small, slot, 0.5, 0.5, p.Muted, RampPinNone)
				}
			}
		}
		return total
	}
}

// markPin draws the base a role pinned, at the end of that role's row: a
// rounded chip wearing the same frame the cells in its row wear.
func markPin(gtx layout.Context, c tokens.ColorTokens, box image.Rectangle, pin stdcolor.NRGBA) {
	if box.Empty() {
		return
	}
	radius := gtx.Dp(InnerR) / 2
	fillRRect(gtx, box, radius, pin)
	strokeRRect(gtx, box, radius, gtx.Dp(Hairline), edgeIn(c))
}

// markRung puts the dot on the ground a pick lives on: the cell of a rung
// it took, or — for a pin that claims no rung — the chip at the end of its
// row. Its ink is measured over the ground it stands on.
func markRung(gtx layout.Context, cell image.Rectangle, step stdcolor.NRGBA) {
	d := min(gtx.Dp(RampMark), min(cell.Dx(), cell.Dy())/2)
	if d <= 0 {
		return
	}
	mid := image.Pt((cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2)
	dot := image.Rect(mid.X-d/2, mid.Y-d/2, mid.X-d/2+d, mid.Y-d/2+d)
	fillRRect(gtx, dot, d/2, markInkOn(step))
}

// markInkOn is the ink a mark takes over one step: whichever end of the
// tonal axis reads better on it, measured — the same choice the derivation
// itself makes when it picks an on-colour.
func markInkOn(step stdcolor.NRGBA) stdcolor.NRGBA {
	if vgcolor.ContrastRatio(tokens.White, step) > vgcolor.ContrastRatio(tokens.Black, step) {
		return tokens.White
	}
	return tokens.Black
}

// edgeIn is the frame every swatch of this section wears: the inverse of
// the page — one voice for the whole section, strongest exactly where an
// edge is needed, on fills near the tone of the page they stand on.
func edgeIn(c tokens.ColorTokens) stdcolor.NRGBA { return c.InverseSurface }

// PickBoard draws every colour the theme names, in families, across as
// many columns as the window is wide enough for — see pickNarrowest for
// how many that is.
func PickBoard(p Palette, c tokens.ColorTokens, ty Type, groups []pickGroup) func(gtx layout.Context, width int) int {
	return func(gtx layout.Context, width int) int {
		gap := gtx.Dp(PickColGap)
		cols := packPicks(groups, pickColumns(width, gap, pickNarrowest(gtx, ty, groups)))
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
					drawCell(gtx, p, c, ty, cell, image.Rect(x, y, x+colW, y+h))
					y += h
				}
			}
			tallest = max(tallest, y)
		}
		return tallest
	}
}

// drawFamily draws the name over one family of cells and the line under
// it, and answers how much of the column the pair took.
func drawFamily(gtx layout.Context, p Palette, ty Type, name string, x, y, w int) int {
	head := gtx.Dp(PickHeadH)
	textdraw.FillText(gtx, ty.Shaper, ty.Head, image.Rect(x, y, x+w, y+head), 0, 0.5, p.Text,
		fitLine(gtx, ty.Shaper, ty.Head, name, w))
	line := gtx.Dp(Hairline)
	paint.FillShape(gtx.Ops, p.Divider, clip.Rect(image.Rect(x, y+head, x+w, y+head+line)).Op())
	return head + line + gtx.Dp(PickHeadGap)
}

// drawCell draws one cell in the slot it was given: the colour, with the
// ink written on it where there is one, and beside them the names over
// their rules — the rules a rung quieter than the names.
func drawCell(gtx layout.Context, p Palette, c tokens.ColorTokens, ty Type, cell pickCell, r image.Rectangle) {
	if r.Dx() <= 0 {
		return
	}
	sw, sh := min(gtx.Dp(PickSwatchW), r.Dx()), min(gtx.Dp(PickSwatchH), r.Dy())
	top := r.Min.Y + (r.Dy()-sh)/2
	box := image.Rect(r.Min.X, top, r.Min.X+sw, top+sh)
	radius := gtx.Dp(InnerR) / 2
	fillRRect(gtx, box, radius, cell.fill)
	// The one frame the grid above uses, and here for the reason it is
	// there: a Background swatch on the page it is the background of has
	// no boundary of its own.
	strokeRRect(gtx, box, radius, gtx.Dp(Hairline), edgeIn(c))
	switch {
	case cell.mark:
		markGlyph(gtx, box, cell.on)
	case cell.paired():
		textdraw.FillText(gtx, ty.Shaper, ty.Label, box, 0.5, 0.5, cell.on, PickGlyph)
	}
	lines := box.Max.X + gtx.Dp(PickGap)
	if lines >= r.Max.X {
		return
	}
	room := r.Max.X - lines
	// The lines stand in a block shorter than the slot, which is where the
	// air between one cell and the next comes from.
	title, rule := gtx.Dp(PickTitleH), gtx.Dp(PickRuleH)
	rules := 1
	if cell.paired() {
		rules = 2
	}
	block := min(title+rules*rule, r.Dy())
	y := r.Min.Y + (r.Dy()-block)/2
	textdraw.FillText(gtx, ty.Shaper, ty.Body,
		image.Rect(lines, y, r.Max.X, y+title), 0, 0.5, p.Text,
		fitLine(gtx, ty.Shaper, ty.Body, cell.title(), room))
	y += title
	textdraw.FillText(gtx, ty.Shaper, ty.Small,
		image.Rect(lines, y, r.Max.X, y+rule), 0, 0.5, p.Muted,
		fitLine(gtx, ty.Shaper, ty.Small, cell.base.rule, room))
	if cell.paired() {
		y += rule
		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(lines, y, r.Max.X, y+rule), 0, 0.5, p.Muted,
			fitLine(gtx, ty.Shaper, ty.Small, cell.ink.rule, room))
	}
}

// markGlyph draws a status role's mark on its own container: a square,
// because the grid already spends a disc on the rung marker, at a share of
// the swatch rather than a size of its own.
func markGlyph(gtx layout.Context, box image.Rectangle, mark stdcolor.NRGBA) {
	d := min(box.Dx(), box.Dy()) / 2
	if d <= 0 {
		return
	}
	mid := image.Pt((box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2)
	fillRRect(gtx, image.Rect(mid.X-d/2, mid.Y-d/2, mid.X-d/2+d, mid.Y-d/2+d), gtx.Dp(Hairline), mark)
}

// pickNarrowest is the narrowest a column is worth having: a swatch, the
// air beside it, and the longest name the board is about to draw, whole —
// measured off the names rather than written down as a number, so it moves
// when the vocabulary does.
func pickNarrowest(gtx layout.Context, ty Type, groups []pickGroup) int {
	lead, narrowest := gtx.Dp(PickSwatchW)+gtx.Dp(PickGap), 0
	for _, g := range groups {
		narrowest = max(narrowest, natural(gtx, ty.Shaper, ty.Head, g.name))
		for _, cell := range g.cells {
			narrowest = max(narrowest, lead+natural(gtx, ty.Shaper, ty.Body, cell.title()))
		}
	}
	return narrowest
}

// pickColumns is how many columns of cells fit in width px, gap px apart,
// at no less than narrowest px each — at least one, never more than the
// board is worth spreading over.
func pickColumns(width, gap, narrowest int) int {
	if width <= 0 || narrowest <= 0 {
		return 1
	}
	return max(1, min(PickMaxCols, (width+gap)/(narrowest+gap)))
}

// pickLoad is how tall one family stands, in dp: arithmetic on the
// constants rather than a measurement.
func pickLoad(g pickGroup) int {
	h := int(PickHeadH) + int(PickHeadGap)
	for _, c := range g.cells {
		h += int(c.height())
	}
	return h
}

// packPicks deals the families into n columns so that the tallest column
// is as short as it can be, each family whole and none of them out of
// order — a deal is a run of boundaries in the reading order, not an
// assignment of families to columns.
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

// bestCuts is where the column boundaries fall in the evenest in-order
// deal: n-1 indices into groups, never going backwards. Ties keep the
// arrangement that fills the leftmost column first.
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
