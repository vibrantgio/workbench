package main

import (
	"image"
	"image/color"
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
	// window less the 240 dp tree rail, the 6 dp divider and the 320 dp
	// backlinks aside.
	noteCanvasW = 1100 - treeWidthDp - 6 - 320
	// noteCanvasH is the golden viewport height. The document scrolls, so
	// the goldens capture the top of the note — header row, properties
	// panel, headings, prose with its wikilinks, list and code block.
	noteCanvasH = 700
	// treeCanvasH gives the rail the same height as the note viewport.
	treeCanvasH = 700
)

var (
	noteCanvasSize = image.Pt(noteCanvasW, noteCanvasH)
	treeCanvasSize = image.Pt(treeWidthDp, treeCanvasH)
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
// One consequence is visible in the stored images: the arrow and chevron
// glyphs the header and the tree draw with are not in Roboto, so they
// record as fallback boxes here. At runtime the application shaper
// resolves them from the system faces. Only their positions are pinned
// by these goldens, not their shapes.
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

// TestTreeGolden records or diffs the tree rail: the find field above
// the rows, folders with their disclosure chevrons, the current note
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
				w := renderTree(shaper, c.model, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable)
				golden.Render(t, name, treeCanvasSize, scene(w, tc.bg))
			})
		}
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
