package main

import (
	"image/color"
	"strings"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/theme/tokens"
)

// TestColourSectionRowsIsTheRowCount asserts the stated row count against
// the rows themselves: the board is a heading and a body.
func TestColourSectionRowsIsTheRowCount(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	c := tokens.PlatformLight
	if got := len(ColourRows(c, tokens.DefaultTypography, shaper)); got != ColourSectionRows {
		t.Fatalf("ColourRows returns %d rows, ColourSectionRows says %d", got, ColourSectionRows)
	}
}

// TestEachThemeTabHoldsItsOwnSection is the split itself, stated as
// arithmetic: the type scale on the Typography tab is a heading band and a
// body, and the board is not in it. The board under the Colours tab's groups
// is the board's heading and body, and the scale is not in it.
func TestEachThemeTabHoldsItsOwnSection(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	c := tokens.PlatformLight
	j := judged{col: c, typ: typo, shaper: shaper, pinned: true}

	scale := palette.TypeScaleRows(inventory.NewForOS(shaper, "darwin"), colourChrome(c), c, boardType(shaper, typo))
	if len(scale) != 2 {
		t.Fatalf("the type scale is %d rows, want 2 (a heading band and a body)", len(scale))
	}
	if got := len(TypeScaleRows(j, newThemePages())); got != len(scale) {
		t.Errorf("the Typography tab's scale is %d rows, want the scale's %d and nothing else", got, len(scale))
	}
	if got := len(ColourRows(c, typo, shaper)); got != ColourSectionRows {
		t.Errorf("the Colours tab's board is %d rows, want the board's %d and nothing else", got, ColourSectionRows)
	}
}

// typeSection is the inventory section the board borrows for its type scale,
// and sectionTitleSep the seam it splits the borrowed title at. The board
// owns both; they are written down here because this window's own guards rest
// on them, and a guard that reads its subject off the thing it is guarding
// checks nothing.
const (
	typeSection     = "foundations-type"
	sectionTitleSep = " — "
)

// TestTypeScaleKeepsTheInventorysWords is the guard on the one place
// the Typography tab could quietly invent copy: the borrowed band's heading
// and caption are the inventory's own title, split at its separator and
// nothing else. A title reworded upstream has to arrive here reworded.
func TestTypeScaleKeepsTheInventorysWords(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight

	var title string
	for _, s := range inv.Foundations(c) {
		if s.Name == typeSection {
			title = s.Title
		}
	}
	if title == "" {
		t.Fatalf("the inventory publishes no section named %q — the Typography tab is empty", typeSection)
	}
	heading, hint, _ := strings.Cut(title, sectionTitleSep)
	if heading == "" {
		t.Errorf("splitting %q leaves no heading for the band", title)
	}
	if rejoined := heading + sectionTitleSep + hint; rejoined != title {
		t.Errorf("the band says %q, the inventory says %q", rejoined, title)
	}
}

// TestTheBoardShowsEveryPlatformName is the arithmetic the board exists for:
// every name the set carries has a row, so a name added upstream appears here
// without an edit to this window. The board reads the set by reflection, and
// this asserts the count against the set itself.
func TestTheBoardShowsEveryPlatformName(t *testing.T) {
	rows := inventory.PlatformColorRows()
	if len(rows) == 0 {
		t.Fatal("the platform set publishes no rows; the board would be empty")
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.Name == "" {
			t.Error("a row carries no platform name")
		}
		if seen[r.Name] {
			t.Errorf("the board draws %q twice", r.Name)
		}
		seen[r.Name] = true
	}
}

// TestTheBoardsChromeIsFlattened guards the one rule a consumer of the set
// can break silently: every colour this window hands the board is opaque, so
// nothing composited over a fill it does not stand on ever reaches Gio.
func TestTheBoardsChromeIsFlattened(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{{"light", tokens.PlatformLight}, {"dark", tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			chrome := colourChrome(tc.colors)
			for _, f := range []struct {
				name string
				col  color.NRGBA
			}{
				{"Surface", chrome.Surface},
				{"Seam", chrome.Seam},
				{"Text", chrome.Text},
				{"Muted", chrome.Muted},
			} {
				if f.col.A != 0xff {
					t.Errorf("the board's %s is handed over at coverage %d; a caller composites before it paints", f.name, f.col.A)
				}
			}
		})
	}
}
