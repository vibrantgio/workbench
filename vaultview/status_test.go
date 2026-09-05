package main

import (
	"image"
	stdcolor "image/color"
	"strings"
	"testing"

	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// TestSourceLinesCountsTheFilesOwnLines pins the definition the bar
// reports against, edge by edge: a line ends at a newline, a last line
// written without one still counts, an empty file has nothing to count,
// and the frontmatter is part of the file like every other line of it.
func TestSourceLinesCountsTheFilesOwnLines(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"an empty note", "", 0},
		{"one line, ended", "title\n", 1},
		{"one line, unended", "title", 1},
		{"two lines, ended", "title\nbody\n", 2},
		{"two lines, unended", "title\nbody", 2},
		{"a blank line counts", "title\n\n", 2},
		{"a file of blank lines", "\n\n\n", 3},
		{"only frontmatter", "---\ntitle: Card\n---\n", 3},
		{"frontmatter and a body", "---\ntitle: Card\n---\n\n# Card\n", 5},
		{"lines ended the other way", "title\r\nbody\r\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sourceLines([]byte(c.src)); got != c.want {
				t.Errorf("sourceLines(%q) = %d, want %d", c.src, got, c.want)
			}
		})
	}
}

// TestTheCountIsTheFilesLinesAndNotTheWindows: what the bar reports is a
// property of the file, so it cannot move when the window does. One
// paragraph of four hundred words is one line of source however many rows
// the column wraps it into — at the width the window opens at and at a
// third of it.
func TestTheCountIsTheFilesLinesAndNotTheWindows(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Wide.md", strings.Repeat("word ", 400)+"\n"))
	m.Current = "guide/Wide.md"

	if got := m.CurrentNote().Lines; got != 1 {
		t.Fatalf("a note of one wrapped paragraph counts %d lines, want 1", got)
	}
	for _, w := range []int{windowW, windowW / 3} {
		size := image.Pt(w, windowH)
		widget, st := renderWindow(shaper, m, tokens.DefaultLight, tokens.Spacing, goldenRadius,
			tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
		drawOnce(t, size, widget)
		if st.geom.footTop >= size.Y {
			t.Errorf("no status bar in a %d dp window", w)
		}
		if got, want := statusLine(m), "1 line"; got != want {
			t.Errorf("laid out at %d dp the bar says %q, want %q", w, got, want)
		}
	}
}

// TestTheBarNamesWhatItCounted asserts the wording: one line is not
// "1 lines", and a note with nothing in it says so rather than nothing.
func TestTheBarNamesWhatItCounted(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{{0, "0 lines"}, {1, "1 line"}, {2, "2 lines"}, {132, "132 lines"}}
	for _, c := range cases {
		if got := lineCountLabel(c.n); got != c.want {
			t.Errorf("lineCountLabel(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// TestTheBarFollowsTheNoteOnScreen drives the bar through the model: it
// reads the note the window is showing rather than the one it opened with,
// whichever way the reader arrived — a landing or back through the
// history.
func TestTheBarFollowsTheNoteOnScreen(t *testing.T) {
	m := goldenModel()
	m = cacheNote(m, noteFromSource("Sources.md", "one\ntwo\nthree\n"))

	first := statusLine(m)
	if want := lineCountLabel(m.CurrentNote().Lines); first != want {
		t.Fatalf("the bar opens saying %q, want %q", first, want)
	}

	m, _ = Update(m, Navigate{Path: "Sources.md"})
	if got, want := statusLine(m), "3 lines"; got != want {
		t.Errorf("with a three-line note open the bar says %q, want %q", got, want)
	}
	if statusLine(m) == first {
		t.Errorf("the bar says %q for both notes; it is not following the note on screen", first)
	}

	m, _ = Update(m, GoBack{})
	if got := statusLine(m); got != first {
		t.Errorf("back at the first note the bar says %q, want %q", got, first)
	}
}

// TestTheBarSaysNothingBeforeAnyNoteIsOpen holds the bar to what it knows:
// scanning a vault, a scan that failed and a vault with no notes are all
// states with no note to measure.
func TestTheBarSaysNothingBeforeAnyNoteIsOpen(t *testing.T) {
	for _, c := range []struct {
		name  string
		model Model
	}{
		{"scanning", Model{Screen: screenVault, Vault: "/v", Scanning: true, CurAnchor: -1}},
		{"scan failed", Model{Screen: screenVault, Vault: "/v", ScanErr: "permission denied", CurAnchor: -1}},
		{"empty vault", Model{Screen: screenVault, Vault: "/v", Index: &Index{}, CurAnchor: -1}},
	} {
		if got := statusLine(c.model); got != "" {
			t.Errorf("with %s the bar says %q, want nothing", c.name, got)
		}
	}
}

// TestTheBarsInkIsLegibleOnItsGround measures the quiet neutral step the
// count is drawn in against the paper it stands on, in both appearances
// the app ships, logging the ratios.
//
// The band the bar claims runs past the document and over the trailing
// panel's own surface, so that ground is measured too: no ink is drawn out
// there today, but a bar with room to grow must not grow onto a pairing
// nobody measured.
func TestTheBarsInkIsLegibleOnItsGround(t *testing.T) {
	// The floor for body-sized text. The bar's role is smaller than body
	// text, which asks for more rather than less, so this is the weakest
	// claim worth making.
	const floor = 4.5
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			ink := tc.colors.Ramps.Neutral.Step(statusInkStep)
			for _, g := range []struct {
				name   string
				ground stdcolor.NRGBA
			}{
				{"the note's paper", tc.colors.Background},
				{"the trailing panel's floor", chromeSurface(tc.colors)},
			} {
				ratio := color.ContrastRatio(ink, g.ground)
				t.Logf("the bar's ink on %s: %.2f:1", g.name, ratio)
				if ratio < floor {
					t.Errorf("the bar's ink reads %.2f:1 on %s, under the %.1f:1 floor", ratio, g.name, floor)
				}
			}
		})
	}
}

// TestTheBarStandsInTheFootItWasGiven reads the composed window and
// requires the count's ink to be inside the band the frame reserved: below
// the document's own column, clear of the window's bottom edge, and on the
// note column's reading margin rather than anywhere else across the foot.
//
// Measured off the picture: a bar that agrees with the arithmetic that
// placed it and not with the pixels is the defect worth catching.
func TestTheBarStandsInTheFootItWasGiven(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			m := goldenModel()
			w, st := renderWindow(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
			img := golden.Capture(t, windowCanvasSize, windowScene(w, tc.colors))

			foot := st.geom.footTop
			if foot >= windowH {
				t.Fatal("the frame reserved no foot for the status bar")
			}
			x0 := st.geom.contentX + noteInsetDp
			top, bot := inkRows(img, tc.colors.Background, x0, x0+200, foot, windowH)
			if top < 0 {
				t.Fatalf("no ink on the reading margin between y=%d and the window's foot; the count is not being drawn", foot)
			}
			if bot >= windowH-1 {
				t.Errorf("the count's ink reaches row %d of a %d dp window; the window's edge is cutting it", bot, windowH)
			}
			// Nothing of the count may stand above the band: the document
			// column ends where the band begins, and ink over that line
			// would be the bar reaching back into the note.
			if above, _ := inkRows(img, tc.colors.Background, x0, x0+200, foot-4, foot); above >= 0 {
				t.Errorf("ink at row %d, above the band the bar was given at y=%d", above, foot)
			}
		})
	}
}

// TestTheFootRedrawsOnANoteSwitch is the update path in pixels: one window
// showing two notes of different lengths must not paint the same foot, so
// the string reaches the glass and not only the model.
func TestTheFootRedrawsOnANoteSwitch(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	first := goldenModel()
	second := cacheNote(first, noteFromSource("Sources.md", plainNoteSource))
	second.Current = "Sources.md"
	if statusLine(first) == statusLine(second) {
		t.Fatalf("both notes report %q; the probe cannot tell them apart", statusLine(first))
	}

	shot := func(m Model) *image.RGBA {
		w, _ := renderWindow(shaper, m, tokens.DefaultLight, tokens.Spacing, goldenRadius,
			tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
		return golden.Capture(t, windowCanvasSize, windowScene(w, themeCases[0].colors))
	}
	a, b := shot(first), shot(second)

	// The probe is the band, in the columns the note column owns: the
	// sidebar's card reaches into these rows too, and a difference over
	// there would be some other part of the window changing its mind.
	foot := windowH - int(statusBarHeight(goldenTokens()))
	diff := 0
	for y := foot; y < windowH; y++ {
		for x := treeWidthDp + railMarginDp; x < windowW-frameAsideDp; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Error("the window's foot is pixel-identical for two notes of different lengths")
	}
}
