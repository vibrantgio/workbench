// The embedded page: the whole published surface, drawn in the palette the
// chosen candidate generates.
//
// This is what the application is for. A swatch says what a colour is; three
// dozen families drawn in it say what it does, which is the only question a
// brand colour actually raises. The page below the candidate row is the
// answer, and picking another swatch redraws all of it on the next frame.
//
// The page is a tab strip and four surfaces rather than one column several
// screens tall. Theme is what the window opens on — the seed itself, then the
// palette it derived with the provenance in it, and the type ladder under
// them — and the three after it are the published catalogue's own groups, one
// to a tab. A reader judging a colour on buttons does not have to scroll past
// the ramps to reach them, and the tab they were on is still the tab they are
// on after the next pick.
package main

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// The embedded page's own furniture.
const (
	GalleryLabel = "Everything in this theme"
	GalleryHint  = "the whole published surface, a tab at a time"
)

// The page's tabs, in the order the strip lays them out. Theme leads because
// it is the answer to the question the window was opened with — what this
// colour makes — and the three after it are what that answer is spent on.
const (
	TabTheme = iota
	TabComponents
	TabPatterns
	TabMarkdown
	TabCount
)

// TabLabels are the strip's cells, in tab order.
var TabLabels = [TabCount]string{"Theme", "Components", "Patterns", "Markdown"}

// TabGroups is the inventory group each of the three catalogue tabs shows,
// by name rather than by position: a group reordered upstream still lands on
// the tab that names it, and a group renamed upstream shows nothing at all —
// which is what [TestEveryGroupTabNamesALiveGroup] turns into a failure
// rather than a blank surface somebody has to notice.
//
// The inventory's Foundations group is on no tab of its own. Its colour
// sections are the Theme tab's telling, in this window's own words and with
// this seed's provenance, and its type ladder stands under them there.
var TabGroups = map[int]string{
	TabComponents: "Components",
	TabPatterns:   "Patterns",
	TabMarkdown:   "Markdown",
}

// TabGap is the air between the strip and the surface under it: eight times
// the underline the selected cell is marked with. Without it the underline is
// the top edge of whatever the tab opens on, and a mark that doubles as a
// border is not a mark. It exposes the panel's own ground on both sides of
// the line, so the line reads as a line.
var TabGap = unit.Dp(tokens.Spacing.S4)

// embed is what the page keeps across frames and across picks: where each tab
// is scrolled to, the catalogue with its rasterised icons, and the reading
// sample parsed from its source.
//
// It is built once and never rebuilt for a palette, which is what makes a
// pick cheap. Parsing the reading sample is the expensive half of this page
// and colour is no part of it: the document takes its style at layout time,
// so choosing another candidate re-styles a document already parsed rather
// than reading one again. What a pick costs is one palette derivation, one
// pass building the section values, and the frame that draws them.
//
// One scroll state per tab, because a tab a reader has scrolled into is a
// place they mean to come back to. One catalogue for all of them, because
// every tab is a cut of the same catalogue and the parse is worth doing once.
type embed struct {
	sts    [TabCount]*list.State
	shaper *text.Shaper
	inv    *inventory.Inventory
	bases  highlight.BasePair
	// code is the row of the Markdown tab the specimen's body is drawn on,
	// or -1 before the first call has looked. It is a fact about the group's
	// shape, which no palette moves.
	code int
}

func newEmbed() *embed {
	e := &embed{code: -1}
	for i := range e.sts {
		e.sts[i] = list.NewState()
	}
	return e
}

// state is where the given tab is scrolled to.
func (e *embed) state(tab int) *list.State { return e.sts[tab] }

// catalogue returns the inventory every tab is cut from, in the given
// typography and with its code drawn in the given syntax bases — the pair, so
// the appearance the palette is on picks its own member and a flip of the
// scheme puts the other one's whole plate on the specimen, its ground
// included.
//
// It is built on the first call — before anything has been dropped, so the
// parse is behind us by the time a pick has to be quick — and kept. A
// code-face change needs the matching collection, so the shaper is replaced;
// the parsed document stays and is only restyled.
func (e *embed) catalogue(shaper *text.Shaper, typ tokens.Typography, bases highlight.BasePair) *inventory.Inventory {
	if e.inv == nil {
		e.inv, e.shaper = inventory.New(shaper), shaper
		e.bases, e.code = highlight.BasePair{}, -1
	} else if e.shaper != shaper {
		e.inv.SetShaper(shaper)
		e.shaper = shaper
	}
	e.inv.SetTypography(typ)
	if e.bases != bases {
		e.bases = bases
		e.inv.SetCodeBases(bases)
	}
	return e.inv
}

// codeRow is the row of the Markdown tab the code specimen's body is drawn
// on, or -1 when that group carries no such section — which is a wiring fault
// and not a state any build ships.
//
// It is the number the base selector is seated by. The rows of a tab are the
// group's sections, a heading and a body each, so the body of section i is
// row 2i+1; the banner [inventory.Inventory.GroupItems] leads with is not
// there, the strip cell having said the word already.
func (e *embed) codeRow(c tokens.ColorTokens) int {
	if e.code >= 0 || e.inv == nil {
		return e.code
	}
	for _, grp := range e.inv.Groups(c) {
		if grp.Name != TabGroups[TabMarkdown] {
			continue
		}
		for i, s := range grp.Sections {
			if s.Name == inventory.CodeSectionName() {
				e.code = 2*i + 1
				return e.code
			}
		}
	}
	return -1
}

// TypeLadderRows is the inventory's type ladder as the two rows that close
// the Theme tab: this window's own heading band over the section's own body.
//
// The ladder is here rather than on a tab of its own because a theme is a
// palette and a typeface: the type roles are generated from the same theme
// the ramps are, so the tab that answers "what is this theme" has to answer
// both halves of it. It wears the band its neighbours wear, and the
// inventory's own words are kept — split at the em dash its titles are
// already written with, so nothing is reworded here and a title reworded
// upstream arrives reworded.
func TypeLadderRows(inv *inventory.Inventory, p Palette, c tokens.ColorTokens, ty Type) []layout.Widget {
	for _, s := range inv.Foundations(c) {
		if s.Name != TypeSection {
			continue
		}
		label, hint, _ := strings.Cut(s.Title, SectionTitleSep)
		return []layout.Widget{
			paletteHeading(p, c, ty, label, hint),
			paletteBody(c, ladderBody(s)),
		}
	}
	return nil
}

// TypeSection is the inventory section the Theme tab borrows: the whole type
// ladder, every role a surface reads in.
const TypeSection = "foundations-type"

// SectionTitleSep is the seam an inventory section's title is written with:
// what the section is, then how to read it. The palette's own bands are built
// from exactly that pair — a label at the leading edge and a caption at the
// trailing one — so a borrowed title splits into a band with nothing
// reworded.
const SectionTitleSep = " — "

// ladderBody adapts an inventory section's body to the palette body's shape:
// the palette measures its content and reports the height, while a section
// body is laid out in a slot of the height the section states. So the slot is
// stated here — bounded, because the type ladder measures nothing of its own
// and an unbounded one would take the column with it — and handed back as the
// height the band wraps.
func ladderBody(s inventory.Section) func(gtx layout.Context, width int) int {
	return func(gtx layout.Context, width int) int {
		h := gtx.Dp(s.Height)
		gtx.Constraints = layout.Constraints{Max: image.Pt(width, h)}
		s.Body(gtx)
		return h
	}
}

// GalleryColumns is what the four tabs show for one emission, in tab order:
// the Theme tab's own palette story with the type ladder under it, and the
// three catalogue groups, one scrolling column each.
//
// The columns are built here rather than per frame, because the rows are a
// function of the palette and the chosen syntax base, and both change on an
// emission and not on a frame. All four are built whichever one is on screen:
// switching tabs is then a frame and not an emission, and the cost of the
// three nobody is looking at is the row values, the catalogue behind them
// being the same catalogue.
//
// The base selector and the two-name face plate ride in the code specimen's
// own row on the Markdown tab rather than standing beside the page: the
// choice belongs next to its consequence, and nowhere else on the page is it
// worth a column. Changing the base moves nothing — whoever picked one is
// already looking at what it did, and a column that jumped under them would
// be taking a view they had set themselves.
func GalleryColumns(t themed, m Model, page *embed, sel *baseSelector, faces *faceSelector) []layout.Widget {
	c, other := SchemePair(t.os, m)
	p, dark := PaletteFrom(c), m.Dark(t.os)
	applied, shaper := t.codeType(m)
	inv := page.catalogue(shaper, applied, m.AppliedBases())

	cols := make([]layout.Widget, TabCount)
	// The seed leads the palette story because it is what the story is a
	// story about: the ramps, the picks and the bases under it are all
	// derivations of this one colour, and the tab showed every derivation and
	// never the input until this row.
	seed, picked := m.Seed()
	theme := SeedRows(p, c, t.typ, seed, picked)
	theme = append(theme, PaletteRows(p, c, other, t.typ, dark)...)
	theme = append(theme, TypeLadderRows(inv, p, c, t.typ)...)
	cols[TabTheme] = ScrollingColumn(page.state(TabTheme), c, theme)
	for tab := TabComponents; tab < TabCount; tab++ {
		rows := inv.TabItems(c, TabGroups[tab])
		if tab == TabMarkdown {
			if row := page.codeRow(c); row >= 0 && row < len(rows) {
				rows[row] = BesideTheCode(p, c, t.typ, m, dark, sel, faces, page.state(tab), rows[row])
			}
		}
		cols[tab] = ScrollingColumn(page.state(tab), c, rows)
	}
	return cols
}

// ScrollingColumn is the surface every tab shows: the rows in a virtual list
// — only what shows is laid out — under the air the strip's underline needs,
// with an overlay scrollbar drawn from the same tokens the rows are.
//
// The bar floats over the rows rather than reserving a gutter: the column
// below is the design system at its own widths, and a gutter cut out of it
// would be the application editing what it is showing. It is drawn from the
// embedded palette, because it is furniture of the page and not of the
// window.
func ScrollingColumn(st *list.State, c tokens.ColorTokens, items []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: TabGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(gtx.Constraints.Max)
			list.LayoutScrollbar(gtx, st, scrollbar.FromTokens(c), list.Overlay, items,
				func(gtx layout.Context, w layout.Widget) layout.Dimensions {
					return w(gtx)
				})
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}
}

// Gallery draws the embedded page: a label saying what it is and which colour
// it is rendered from, and under it the tabbed surface on its own panel.
//
// The panel is filled with the embedded palette's own background, not the
// window's — for a moment after a pick those are the same thing, and that is
// the point: the page is not a preview beside the theme, it is the theme. The
// strip is drawn in that same palette for the same reason: it is furniture of
// the page, not of the window.
//
// shell is the tab strip with the selected tab's surface under it, handed in
// because the strip's own handlers outlive an emission and this function is
// called on every one of them.
func Gallery(p Palette, c tokens.ColorTokens, ty Type, seed string, shell layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, size.X, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, GalleryLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, seed)

		top := labelH + gtx.Dp(RowTop)
		if top >= size.Y {
			return layout.Dimensions{Size: size}
		}
		panel := image.Rectangle{Max: image.Pt(size.X, size.Y-top)}
		at(gtx, image.Pt(0, top), func(gtx layout.Context) {
			fillRRect(gtx, panel, gtx.Dp(Radius), c.Background)
			// Clipped to the panel: a column is taller than the window
			// several times over, and a row drawn past the bottom edge would
			// land on the candidate row above it on the next frame.
			defer clip.UniformRRect(panel, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
			gtx.Constraints = layout.Exact(panel.Max)
			shell(gtx)
			strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline), p.Divider)
		})
		return layout.Dimensions{Size: size}
	}
}

// GalleryHintFor writes what the page is rendered from: the chosen colour and
// the syntax base its code is coloured with under the appearance on screen, or
// the standing invitation while there is none.
//
// The base is named here as well as marked beside the specimen because the
// specimen is on one tab of four. Everywhere else on the page, this line is
// the only thing saying what the code is coloured with — and it stays on
// screen, where the specimen does not. It names one base and not the pair for
// the same reason: it says what is on screen, and what is on screen is one
// appearance's.
func GalleryHintFor(m Model, dark bool) string {
	if seed, ok := m.Seed(); ok {
		return "rendered from " + hexOf(seed) + " · syntax base " + m.Base(dark)
	}
	return GalleryHint
}
