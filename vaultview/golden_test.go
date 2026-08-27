package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

const (
	// noteCanvasW matches the runtime main-slot budget: the 1100 dp
	// window less the sidebar column, which the content area now butts
	// straight against, the divider's grab area and the backlinks aside.
	noteCanvasW = 1100 - treeWidthDp - frameDividerDp - frameAsideDp
	// noteCanvasH is the golden viewport height. The document scrolls, so
	// the goldens capture the top of the note — header row, properties
	// panel, headings, prose with its wikilinks, list and code block.
	noteCanvasH = 700
	// treeCanvasH gives the rail the same height as the note viewport.
	treeCanvasH = 700

	// goldenLeading is the window-button trailing edge the static renders
	// lay out from. The live pane measures this from the window; a stored
	// image may not, because off a frame the measurement is zero and on
	// one it is whatever this machine's macOS answers — two different
	// windows, neither reproducible elsewhere. So the golden states a
	// value: a real macOS measurement, recorded once and frozen as a
	// fixture. It is not a constant the application uses.
	//
	// One value, for both rail states and for the same reason the window
	// itself has one: the buttons are anchored to the window's own edges
	// and nothing the application draws under them moves them. The pin was
	// briefly a pair, when hiding the pane handed the buttons back to a
	// different geometry; that a single number serves again is the
	// arrangement saying so.
	goldenLeading = 79
)

var (
	noteCanvasSize = image.Pt(noteCanvasW, noteCanvasH)
	treeCanvasSize = image.Pt(treeWidthDp, treeCanvasH)
	// windowCanvasSize is the size the app's window opens at. The window
	// goldens are recorded there and nowhere else: a composition is only
	// worth a picture at a size somebody actually looks at it in.
	windowCanvasSize = image.Pt(windowW, windowH)
	// Sharp corners keep the goldens deterministic: anti-aliased rounded
	// corners vary slightly between GPU contexts, breaking pixel-exact
	// diffs. The pattern goldens upstream do the same.
	sharpRadius = tokens.RadiusScale{}
)

// goldenNoteSource is the note the goldens render: frontmatter for the
// properties panel, a heading hierarchy, prose carrying a plain and an
// aliased wikilink, a list, a fenced code block whose wikilink must NOT
// become a link, and a block-id tail that must not show.
const goldenNoteSource = `---
title: Reading list
tags:
  - reading
  - vault
status: open
---

# Reading list

The shelf I keep coming back to. See [[Design/Principles|the principles]]
for why, and [[Sources]] for where these came from.

## Open questions

- Does a viewer owe the reader the vault's real state?
- What does a link mean when two notes share a name?

Answers get stamped so they can be cited. ^answers-1

## A sample

	code

` + "```go" + `
// A wikilink inside code is a code sample, not navigation:
// [[Design/Principles]]
func main() {}
` + "```" + `
`

// goldenModel is the vault-screen model the note golden renders: the
// reading-list note current, reached from another note so Back is live
// and Forward is not, properties expanded.
func goldenModel() Model {
	m := Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1, PropsOpen: true}
	m.Index = treeIndex(
		"Sources.md",
		"Design/Principles.md",
		"Design/notes/Colour.md",
		"guide/Reading list.md",
	)
	m = cacheNote(m, noteFromSource("guide/Reading list.md", goldenNoteSource))
	m.Current = "guide/Reading list.md"
	m.History = []HistEntry{{Path: "Sources.md", Anchor: -1}, {Path: "guide/Reading list.md", Anchor: -1}}
	m.Cursor = 1
	m.Folds = map[string]bool{"Design": true, "guide": true}
	return m
}

// themeCases are the two appearance modes every golden records.
var themeCases = []struct {
	name   string
	colors tokens.ColorTokens
	bg     color.NRGBA
}{
	{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
	{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
}

// TestNotePageGolden records or diffs the rendered note in light and
// dark, through the same composition the runtime main slot lays out:
// header row, breadcrumb trail, properties panel from the frontmatter
// split, and the parsed body as a markdown document with chroma
// highlighting. Rendering pins DeterministicShaper — Roboto and Roboto
// Mono, system fonts off — so the rasterisation cannot depend on which
// faces the host carries. The theme's own Shaper() is the application
// default and resolves system fallbacks; a golden must never take it.
//
// No mark in the window is typeset: the history controls and the
// disclosures are drawn from the design system's set, so the goldens
// record the same ink the runtime shows whatever faces the host carries.
func TestNotePageGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderNotePage(shaper, m, tc.colors, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
			golden.Render(t, "note-"+tc.name, noteCanvasSize, scene(w, tc.bg))
		})
	}
}

// longNoteSource is a note far taller than the viewport: forty numbered
// sections, each a heading and a paragraph. It exists so the note column
// can be photographed part way down a document, which is the only state
// where the scroll indicator has anything to say.
func longNoteSource() string {
	var b strings.Builder
	b.WriteString("# A long note\n\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "## Section %d\n\nParagraph %d of a note that runs well past the "+
			"bottom of any window it is read in, so the reader needs telling where "+
			"in it they are.\n\n", i, i)
	}
	return b.String()
}

// longNoteModel is goldenModel's note replaced by the long one, seated at
// the given block so the render lands mid-document.
func longNoteModel(first int) Model {
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Long note.md", longNoteSource()))
	m.Current = "guide/Long note.md"
	m.CurAnchor = first
	return m
}

// plainNoteSource is a note with no headings of any kind: the case the
// outline pane has nothing to list for, and the one where the column had
// better still be worth its width — this note is cited, and its citations
// are all the column has to show.
const plainNoteSource = `The sources these notes came from, kept as a plain
list because it has never needed sections.

- A shelf of books.
- A folder of papers.
- Two decades of margin notes.
`

// plainNoteModel makes the cited, heading-less note current: the Reading
// list note links to it, so its backlinks pane has a row while its
// outline has none.
func plainNoteModel() Model {
	m := goldenModel()
	// The scanned index the tree golden shares carries no link bodies, so
	// the reverse edges have to be put there for the pane to have a row.
	// Two citations, one of them from a folder, so the pane shows both of
	// the shapes a backlink row has.
	idx := *m.Index
	idx.Files = append([]FileScan(nil), idx.Files...)
	for i := range idx.Files {
		switch idx.Files[i].Path {
		case "guide/Reading list.md", "Design/Principles.md":
			idx.Files[i].Links = []string{"Sources"}
		}
	}
	m.Index = &idx
	m = cacheNote(m, noteFromSource("Sources.md", plainNoteSource))
	m.Current = "Sources.md"
	m.History = []HistEntry{{Path: "guide/Reading list.md", Anchor: -1}, {Path: "Sources.md", Anchor: -1}}
	m.Cursor = 1
	return m
}

// taskNoteSource is a note whose body is a GFM task list: checked, open,
// [X], and a nested item. It exists so the goldens record the marks a
// click will write, in both schemes, rather than only the notes that
// have none.
const taskNoteSource = `# Tasks

What is still open, and what is not.

- [x] Record the marker offset
- [ ] Write the character
- [X] Keep the reader where they were
  - [ ] nested under a done item
- [ ] Brackets stay put
`

func taskNoteModel() Model {
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Tasks.md", taskNoteSource))
	m.Current = "guide/Tasks.md"
	m.History = []HistEntry{{Path: "guide/Reading list.md", Anchor: -1}, {Path: "guide/Tasks.md", Anchor: -1}}
	m.Cursor = 1
	m.PropsOpen = false
	return m
}

// TestNoteTasksGolden records the note column in both schemes for a note
// that has task boxes. The idle marks must match the file; a click is
// not this picture's job.
func TestNoteTasksGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := taskNoteModel()
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderNotePage(shaper, m, tc.colors, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
			golden.Render(t, "note-tasks-"+tc.name, noteCanvasSize, scene(w, tc.bg))
		})
	}
}

// TestNoteScrollbarGolden records the note column part way down a long
// note. The indicator sits in the column's trailing gutter, away from both
// ends of its track and a fraction of its length — position and proportion,
// which is the whole of what it is for.
func TestNoteScrollbarGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := longNoteModel(30)
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderNotePage(shaper, m, tc.colors, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
			golden.Render(t, "note-scrollbar-"+tc.name, noteCanvasSize, scene(w, tc.bg))
		})
	}
}

// TestNoteScrollbarOnlyWhenTheNoteOverflows is the exit condition as an
// assertion: a long note read at its end draws an indicator at the foot of
// the column's trailing gutter, and a note that fits, in the same viewport,
// draws none. The probe is pixels rather than dimensions, because the gutter
// is reserved either way — that is what stops the prose reflowing when the
// bar fades.
//
// The note that fits is the plain one rather than the golden note: at the
// reading rhythm the renderer now sets, the golden note's frontmatter panel,
// headings and code block no longer fit a 700 dp viewport, so it has become a
// second long note and can no longer say anything about a note that does not
// scroll.
//
// The gutter is the column's last ten dp — the bar reaches the edge, where
// the platform puts it, and every other row stops a reading margin short of
// it — and the probe takes the foot of that strip, which no note's own ink
// can reach.
func TestNoteScrollbarOnlyWhenTheNoteOverflows(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	shot := func(m Model) *image.RGBA {
		w := renderNotePage(shaper, m, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
		return golden.Capture(t, noteCanvasSize, scene(w, bg))
	}
	ground := tokens.DefaultLight.Background
	footInk := func(img *image.RGBA) int {
		n := 0
		for y := noteCanvasH - 100; y < noteCanvasH-noteInsetDp; y++ {
			for x := noteCanvasW - 10; x < noteCanvasW; x++ {
				c := img.RGBAAt(x, y)
				if c.R != ground.R || c.G != ground.G || c.B != ground.B {
					n++
				}
			}
		}
		return n
	}

	if n := footInk(shot(longNoteModel(118))); n == 0 {
		t.Error("a note taller than the viewport drew no scroll indicator")
	}
	if n := footInk(shot(plainNoteModel())); n != 0 {
		t.Errorf("a note that fits drew %d indicator pixels, want none", n)
	}
}

// TestTreeGolden records or diffs the tree rail: the find field above
// the rows, folders with their disclosure marks, the current note
// marked active — and the same rail with a filter typed in, where the
// hierarchy gives way to the matching notes and their folders.
func TestTreeGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	base := goldenModel()
	filtered := base
	filtered.Filter = "o"

	cases := []struct {
		name  string
		model Model
	}{
		{"tree", base},
		{"tree-filtered", filtered},
	}
	for _, c := range cases {
		for _, tc := range themeCases {
			name := c.name + "-" + tc.name
			t.Run(name, func(t *testing.T) {
				w := renderTree(shaper, c.model, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable, goldenLeading)
				golden.Render(t, name, treeCanvasSize, scene(w, tc.bg))
			})
		}
	}
}

// TestVaultWindowGolden records or diffs the whole vault window, which is
// the one thing every other golden in this package cannot see. A rail
// rendered on its own at 240 dp and a note column rendered on its own at
// its runtime width are both correct pictures of a window that spends
// eighty dp on empty chrome above them: the defect is in the composition,
// and only a picture of the composition carries it.
//
// Four images, because hiding the rail is not a change to one slot — the
// pane goes, the note column reflows from the window's own edge, and the
// control that brings the pane back moves into the chrome row — and
// because every appearance the app ships is an appearance the
// composition can be wrong in.
//
// The pane's corners are anti-aliased and the goldens carry those pixels.
// That is the composition telling the truth; the sharp-radius trick the
// slot goldens use covers the components' own radii, not a rounded clip
// the frame draws itself.
func TestVaultWindowGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true

	cases := []struct {
		name  string
		model Model
	}{
		{"window", shown},
		{"window-hidden", hidden},
		// The three shapes the trailing column has to hold. A forty-heading
		// note read part way down fills the outline pane and marks the
		// section the reader is inside — and the backlinks are still below
		// it, which is the whole reason the column is two panes. A note
		// with no headings at all says so and gives the rest of the column
		// to the notes citing it. And a note cited far more often than the
		// pane will show stops at the cap and scrolls the rest, which is
		// the state both panes' indicators are in at once.
		{"window-outline", longNoteModel(30)},
		{"window-plain", plainNoteModel()},
		{"window-cited", citedModel("guide/Long note.md", longNoteSource(), 20)},
	}
	for _, c := range cases {
		for _, tc := range themeCases {
			name := c.name + "-" + tc.name
			t.Run(name, func(t *testing.T) {
				// One leading pin for every case, the way the live
				// measurement is one: the buttons stand at the window's own
				// inset whether the pane is under them or not.
				w, _ := renderWindow(shaper, c.model, tc.colors, tokens.Spacing, sharpRadius,
					tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
				golden.Render(t, name, windowCanvasSize, scene(w, tc.bg))
			})
		}
	}
}

// TestTheTopBandStandsOnTheButtonLine reads the rendered window and
// requires everything along the top of it to be on one line: the line the
// window's control buttons centre on. The vault's name, the toggle in the
// pane's own strip, and the toggle the chrome row shows once the pane is
// away — all three are measured as ink, off the composed image, rather
// than computed from the constants that placed them. A placement that
// agrees with its own arithmetic and not with the picture is the defect
// this is here for.
//
// The buttons themselves are the platform's and draw nothing headlessly,
// so the line is the number the window states to it — the same number
// the placement call is given, and the one a live capture was measured
// against.
//
// The two toggle marks are also required to occupy exactly the same rows.
// They are the two halves of one switch, and a reader working the pane
// back and forth must see one mark stay put, not a mark that hops as the
// half it is showing changes.
//
// A dp of slack, and no more: the label is a line box centred on the
// line, and a line box reserves room under the baseline for descenders
// that "Second Brain" does not spend, which puts its ink one row high of
// the marks beside it. That much is invisible; anything the marks do is
// not.
func TestTheTopBandStandsOnTheButtonLine(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			shot := func(m Model) (*image.RGBA, *frameState) {
				w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, sharpRadius,
					tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
				return golden.Capture(t, windowCanvasSize, scene(w, tc.bg)), st
			}
			level := func(what string, top, bot int) {
				t.Helper()
				if top < 0 {
					t.Fatalf("%s leaves no ink in the window's top band", what)
				}
				// Ink rows are counted inclusive, so the middle of a span
				// is half a row past its last row's top edge.
				line := float64(windowButtons.Center)
				if c := float64(top+bot+1) / 2; c < line-1 || c > line+1 {
					t.Errorf("%s centres on %.1f, the window buttons on %.1f — the top of the window is not one line",
						what, c, line)
				}
			}

			img, st := shot(shown)
			// The note's first ink is a full margin below the chrome row,
			// so the band above that is the row's alone to have marked. The
			// row starts at the same place in both rail states, which is
			// its own assertion elsewhere.
			band := st.geom.rowTop + noteInsetDp
			nameX := st.geom.contentX + noteInsetDp
			top, bot := inkRows(img, tc.colors.Background, nameX, nameX+400, 0, band)
			level("the vault's name", top, bot)

			// The pane's own toggle stands on the pane's surface, in the
			// square at the trailing end of its strip. The last few columns
			// of that square are left out: the pane's rounded corner is
			// there, and the ground showing round it is not the toggle.
			strip := st.geom.pane.Min.Y + paneStripDp
			toggleX := st.geom.pane.Max.X - railMarginDp - treeHideBoxDp
			paneTop, paneBot := inkRows(img, tc.colors.Surface, toggleX, toggleX+treeHideBoxDp-4,
				st.geom.pane.Min.Y, strip)
			level("the pane's toggle", paneTop, paneBot)

			img, st = shot(hidden)
			// With the pane away the row leads with the toggle, in the
			// span between the window buttons' measured edge and the
			// vault's name.
			markX := goldenLeading + frameGapDp
			rowTop, rowBot := inkRows(img, tc.colors.Background, markX, markX+railToggleMarkDp, 0, band)
			level("the chrome row's toggle", rowTop, rowBot)
			if rowTop != paneTop || rowBot != paneBot {
				t.Errorf("the chrome row's toggle marks rows %d..%d and the pane's %d..%d; one switch, one line",
					rowTop, rowBot, paneTop, paneBot)
			}

			nameX = markX + railToggleMarkDp + int(tokens.Spacing.S3)
			top, bot = inkRows(img, tc.colors.Background, nameX, nameX+400, 0, band)
			level("the vault's name with the pane away", top, bot)
		})
	}
}

// TestTheTrailingColumnKeepsOneEdge reads the trailing column off the
// composed window and requires everything down it to agree on where the
// column's ink stops. A marked row's fill, and the hairline parting the
// two panes, are measured as pixels rather than computed from the
// constants that placed them: the column kept three edges within eight dp
// of each other — a fill at 1075, a bar at 1081, a hairline at 1083 of an
// 1100 dp frame — and every one of them agreed with its own arithmetic.
//
// The bar is measured against the note's, which is the same bar: the two
// stand at the trailing edge of the two columns a reader reads between,
// and a window where one hugs its edge while the other floats eighteen dp
// off is a window with two ideas about where a scrollbar goes. The
// distance measured is to the ground each one stands on — the note's
// paper gives way to this column's surface, and this column's surface
// gives way to the window's own edge.
func TestTheTrailingColumnKeepsOneEdge(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	// A long note read part way down: the outline holds the mark and more
	// entries than its pane can show, so the fill and the bar are both on
	// screen to be measured against each other.
	m := longNoteModel(30)

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
				sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, sharpRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowCanvasSize, scene(w, tc.bg))

			asideX := windowW - frameAsideDp
			lane := int(asideBarLane(tok))
			top, bot := st.geom.rowTop, st.geom.footTop
			is := func(c color.RGBA, want color.NRGBA) bool {
				return c.R == want.R && c.G == want.G && c.B == want.B
			}
			// span answers the leading and trailing columns, inside the box,
			// of every pixel the predicate accepts.
			span := func(x0, x1 int, pick func(color.RGBA) bool) (int, int) {
				lo, hi := -1, -1
				for y := top; y < bot; y++ {
					for x := x0; x < x1; x++ {
						if !pick(img.RGBAAt(x, y)) {
							continue
						}
						if lo < 0 {
							lo = x
						}
						lo, hi = min(lo, x), max(hi, x)
					}
				}
				return lo, hi
			}

			// The mark's own fill: one colour, laid down as a fill, so its
			// own pixels are the only ones that carry it exactly.
			pillLo, pillHi := span(asideX, windowW, func(c color.RGBA) bool {
				return is(c, tc.colors.Ramps.Primary.Step(300))
			})
			if pillLo < 0 {
				t.Fatal("the outline drew no marked row to measure")
			}
			// The hairline is the row that runs two hundred dp of the
			// divider's own colour — a length no glyph's antialiasing and no
			// other fill in the column can be mistaken for.
			ruleLo, ruleHi := -1, -1
			for y := top; y < bot && ruleLo < 0; y++ {
				run := 0
				for x := asideX; x < windowW; x++ {
					if is(img.RGBAAt(x, y), tc.colors.Divider) {
						if run == 0 {
							ruleLo = x
						}
						run++
						ruleHi = x
						continue
					}
					if run >= 200 {
						break
					}
					run, ruleLo, ruleHi = 0, -1, -1
				}
				if run < 200 {
					ruleLo, ruleHi = -1, -1
				}
			}
			if ruleLo < 0 {
				t.Fatal("the column drew no hairline between its panes")
			}

			if want := asideX + asideInsetDp; pillLo != want || ruleLo != want {
				t.Errorf("the column leads with the mark at %d and the hairline at %d; one ink margin, at %d",
					pillLo, ruleLo, want)
			}
			if pillHi != ruleHi {
				t.Errorf("the mark ends at %d and the hairline at %d; the column keeps one right edge", pillHi, ruleHi)
			}

			// The bar's lane, and nothing else in the column: the rows and
			// the hairline stop where it starts.
			barLo, barHi := span(windowW-lane, windowW, func(c color.RGBA) bool {
				return !is(c, tc.colors.Surface)
			})
			if barLo < 0 {
				t.Fatal("an outline with more entries than its pane can show drew no bar")
			}
			if got := barLo - pillHi - 1; got != railMarginDp {
				t.Errorf("the bar stands %d px off the mark beside it, want %d", got, railMarginDp)
			}
			// The note's own bar, in the margin its column keeps: the only
			// ink out there past the prose, which stops a whole page inset
			// short of the columns scanned.
			noteLo, noteHi := span(asideX-noteInsetDp, asideX, func(c color.RGBA) bool {
				return !is(c, tc.colors.Background)
			})
			if noteLo < 0 {
				t.Fatal("the note column drew no bar to measure against")
			}
			if a, b := barHi-barLo, noteHi-noteLo; a != b {
				t.Errorf("the column's bar is %d px wide and the note's %d; they are the same bar", a+1, b+1)
			}
			if a, b := windowW-barHi-1, asideX-noteHi-1; a != b {
				t.Errorf("the column's bar stands %d px off the window's edge and the note's %d px off this column's; one distance",
					a, b)
			}
		})
	}
}

// inkRows answers the first and last row inside the given box that carry
// anything other than the ground colour, or -1, -1 for a box of bare
// ground. Alpha is left out of the comparison: what is drawn over the
// ground is opaque by the time it is captured.
func inkRows(img *image.RGBA, ground color.NRGBA, x0, x1, y0, y1 int) (int, int) {
	top, bot := -1, -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if c := img.RGBAAt(x, y); c.R != ground.R || c.G != ground.G || c.B != ground.B {
				if top < 0 {
					top = y
				}
				bot = y
				break
			}
		}
	}
	return top, bot
}

// TestThePaneEdgeIsCleanBesideTheToggle reads the pixels immediately
// past the pane's trailing edge and requires every one of them to be the
// ground the window is painted on. Nothing the pane draws — its tint,
// its shadow, its strip or the toggle at the end of that strip — may
// leave a mark outside the pane's own fill.
//
// This is the defect the exit condition names, as an assertion. The
// pane's cast shadow used to reach three pixels past that edge, and the
// note column's own ground, painted after the pane, wiped all of it
// below the chrome row: what survived was a nine-row stub of grey beside
// the pane's toggle, an inch of edging that stopped dead. Reading the
// whole column rather than the stub's rows catches both halves — ink
// where there should be none, and ink that stops where nothing changes.
//
// Both appearances, because the shadow is an alpha over whatever is
// under it and light is only where it shows first; and either side of a
// round trip through the hidden state, because the pane the toggle
// brings back has to be the pane that left.
func TestThePaneEdgeIsCleanBesideTheToggle(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shown := goldenModel()
	hidden := shown
	hidden.SidebarHidden = true
	// Three pixels is the reach of the elevation this pane floats at,
	// which is as far as anything it draws could carry.
	const past = 3

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
				sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}
			// One frame state across the three frames, the way the running
			// window keeps one while the streams re-emit around it.
			f := &frameState{asideW: frameAsideDp, leading: func() unit.Dp { return goldenLeading }}
			cur := &docCursor{}
			av := newAsideView(cur)
			shot := func(m Model) *image.RGBA {
				sb := renderTree(shaper, m, tc.colors, tokens.Spacing, sharpRadius,
					tokens.DefaultTypography, tokens.Comfortable, goldenLeading)
				main := renderNotePageInto(cur, shaper, m, tc.colors, tokens.Spacing,
					tokens.DefaultTypography, tokens.Comfortable)
				as := func(gtx layout.Context) layout.Dimensions { return av.layout(gtx, m, tok) }
				w := func(gtx layout.Context) layout.Dimensions { return f.layout(gtx, m, tok, sb, as, main) }
				return golden.Capture(t, windowCanvasSize, scene(w, tc.bg))
			}
			ground := tc.colors.Background
			check := func(when string, img *image.RGBA) {
				edge := f.geom.pane.Max.X
				if edge <= 0 {
					t.Fatalf("%s: the pane has no trailing edge to read", when)
				}
				for x := edge; x < edge+past && x < windowW; x++ {
					for y := 0; y < windowH; y++ {
						if c := img.RGBAAt(x, y); c.R != ground.R || c.G != ground.G || c.B != ground.B {
							t.Errorf("%s: (%d,%d) is %v, one column past the pane's edge at x=%d; want the ground %v",
								when, x, y, c, edge, ground)
							return
						}
					}
				}
			}

			check("with the pane shown", shot(shown))
			shot(hidden)
			check("with the pane brought back", shot(shown))
		})
	}
}

// TestNotePageLightDarkDiffer confirms swapping the colour token set
// changes the rendered note — the guard that a golden pair recorded in
// one appearance cannot silently stand in for both.
func TestNotePageLightDarkDiffer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	m := goldenModel()

	light := renderNotePage(shaper, m, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
	dark := renderNotePage(shaper, m, tokens.DefaultDark, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
	a := golden.Capture(t, noteCanvasSize, scene(light, bg))
	b := golden.Capture(t, noteCanvasSize, scene(dark, bg))
	if n := golden.PixelDiff(a, b); n == 0 {
		t.Error("light and dark render the note identically; expected colour differences across breadcrumb, properties and prose")
	}
}

// TestNotePageConstructs is the smoke test for the states the main slot
// stands in for the document: scanning, a failed scan, and an empty
// vault must each lay out a frame without panicking.
func TestNotePageConstructs(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	cases := []struct {
		name  string
		model Model
	}{
		{"note", goldenModel()},
		{"scanning", Model{Screen: screenVault, Vault: "/v", Scanning: true, CurAnchor: -1}},
		{"scan failed", Model{Screen: screenVault, Vault: "/v", ScanErr: "permission denied", CurAnchor: -1}},
		{"empty vault", Model{Screen: screenVault, Vault: "/v", Index: &Index{}, CurAnchor: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderNotePage(shaper, tc.model, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable)
			dims := drawOnce(t, noteCanvasSize, w)
			if dims.Size.X == 0 || dims.Size.Y == 0 {
				t.Errorf("note page produced zero dimensions: %v", dims)
			}
		})
	}
}

// ---- headless test helpers ---------------------------------------------

// scene paints a ground behind the widget, so a transparent render is
// visible in the stored image rather than reading as black.
func scene(w layout.Widget, bg color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// drawOnce lays one frame out at size, without a GPU.
func drawOnce(t *testing.T, size image.Point, w layout.Widget) layout.Dimensions {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	return w(gtx)
}
