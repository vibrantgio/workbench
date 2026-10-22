package main

// The content column standing with nothing chosen, held against the
// platform's empty state: the line is drawn by patterns/shell in the
// column's FULL frame — the main slot grown upward by the toolbar's share —
// and it spends the platform's tertiary label over the column's own fill.
//
// It is read off a whole-window render rather than off the column alone,
// because the frame the line has to centre in is the arrangement's and not
// the slot's: a line centred in the slot it is handed stands half a
// toolbar lower, which only a frame holding both can show.

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// unchosenModel is the window as it opens: a feed listed and no article
// chosen out of it, which is the state the content column stands empty in.
func unchosenModel() Model { return initialModel() }

// TestEmptyStateStandsInTheColumnsFullFrame holds the line's placement
// against the arrangement the window actually laid out: its glyphs centre
// on the centre of the content column's full frame, the toolbar's share
// included, and not on the centre of the row below the toolbar.
//
// The glyph box is the painted glyphs' own and the line box is what the
// pattern centres, so the two centres differ by what the descenders reach
// below the box's middle: a pixel across and two down at this size. A line
// placed any other way in this column is out by tens of pixels at least.
func TestEmptyStateStandsInTheColumnsFullFrame(t *testing.T) {
	g := layoutFrame(t, unchosenModel())
	if g.Main.Empty() || g.Toolbar.Empty() {
		t.Fatalf("the arrangement laid out no content column: main %v, toolbar %v", g.Main, g.Toolbar)
	}
	full := g.Toolbar.Union(g.Main)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			box := emptyStateGlyphBox(t, tc.c, full)
			if box.Empty() {
				t.Fatal("the column is bare, want the line the empty state draws")
			}
			if got, want := box.Min.X+box.Max.X, full.Min.X+full.Max.X; abs(got-want) > 2 {
				t.Errorf("the line's glyphs centre at %.1f across, want the column's %.1f",
					float64(got)/2, float64(want)/2)
			}
			if got, want := box.Min.Y+box.Max.Y, full.Min.Y+full.Max.Y; abs(got-want) > 4 {
				t.Errorf("the line's glyphs centre at %.1f down, want the full frame's %.1f",
					float64(got)/2, float64(want)/2)
			}
			// Where the row below the toolbar would have centred it, which
			// is the placement this assertion exists to refuse.
			if slot := g.Main.Min.Y + g.Main.Max.Y; abs(box.Min.Y+box.Max.Y-slot) <= 4 {
				t.Errorf("the line centres on the slot below the toolbar at %.1f, not on the column's frame",
					float64(slot)/2)
			}
		})
	}
}

// TestEmptyStateSpendsTertiaryLabel pins the colour the line's glyphs
// plateau at: the platform's tertiary label flattened over the content
// column's own fill. A fully covered pixel is painted the flattened colour
// exactly, so this is an equality and not a tolerance.
func TestEmptyStateSpendsTertiaryLabel(t *testing.T) {
	g := layoutFrame(t, unchosenModel())
	full := g.Toolbar.Union(g.Main)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			fill := tc.c.ControlBackground
			img := renderUnchosen(t, tc.c)
			want := vgcolor.Flatten(tc.c.TertiaryLabel, fill)
			got := emptyStatePlateau(img, fill, scanOf(full))
			if got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("the line's glyphs plateau at #%02x%02x%02x, want TertiaryLabel over the column, #%02x%02x%02x",
					got.R, got.G, got.B, want.R, want.G, want.B)
			}
		})
	}
}

// renderUnchosen draws the whole window with nothing chosen in it. Two
// frames are drawn and the second kept, for the reason renderWindow does.
func renderUnchosen(t *testing.T, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	w := frame(t, c, tokens.Comfortable, unchosenModel())
	golden.Capture(t, windowSize, w)
	return golden.Capture(t, windowSize, w)
}

// scanOf is the part of the content column's frame the line is looked for
// in: the frame less the toolbar's own strip, which carries this window's
// Share button and so holds a symbol that is not the empty state's.
func scanOf(full image.Rectangle) image.Rectangle {
	scan := full
	scan.Min.Y = full.Min.Y + int(shell.ToolbarDepth)
	return scan
}

// emptyStateGlyphBox is the box the line's glyphs occupy in a render of the
// unchosen window: every pixel inside the scan region standing at least a
// fifth of the way from the column's own fill to the colour the glyphs
// plateau at.
//
// It is a share of that plateau and not simply "not the fill" because the
// Share button casts the toolbar control's own drop shadow, and that shadow
// reaches past the strip onto the column: MEASURED off this render, it leaves
// the fill 27 of 765 behind in the scan region against the line's own 198,
// so a reading that counted every pixel the fill does not account for would
// take the shadow's ramp for a glyph and put the line's box out by the width
// of the column. A fifth stands clear of both.
func emptyStateGlyphBox(t *testing.T, c tokens.PlatformColors, full image.Rectangle) image.Rectangle {
	t.Helper()
	img := renderUnchosen(t, c)
	fill := c.ControlBackground
	scan := scanOf(full)
	least := fromFill(emptyStatePlateau(img, fill, scan), fill) / 5
	box := image.Rectangle{}
	first := true
	for y := scan.Min.Y; y < scan.Max.Y; y++ {
		for x := scan.Min.X; x < scan.Max.X; x++ {
			if fromFill(at(img, x, y), fill) < least {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// emptyStatePlateau is the painted colour furthest from the fill inside the
// scan region, which is the colour of the pixels the glyphs cover wholly.
func emptyStatePlateau(img *image.RGBA, fill color.NRGBA, scan image.Rectangle) color.NRGBA {
	var found color.NRGBA
	far := -1
	for y := scan.Min.Y; y < scan.Max.Y; y++ {
		for x := scan.Min.X; x < scan.Max.X; x++ {
			c := at(img, x, y)
			d := fromFill(c, fill)
			if d > far {
				far, found = d, c
			}
		}
	}
	return found
}

// fromFill is how far a painted pixel stands from the fill it was painted
// over, summed across the three channels.
func fromFill(c, fill color.NRGBA) int {
	return abs(int(c.R)-int(fill.R)) + abs(int(c.G)-int(fill.G)) + abs(int(c.B)-int(fill.B))
}
