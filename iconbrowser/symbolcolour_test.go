package main

// The colour every symbol is drawn in, read off the pixels: an icon on the
// content is drawn in the text colour, and the accent is passed only to a
// selected one, which this window has none of.

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	marks "github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/list"
)

// blendTolerance is how far, in summed channel distance, a pixel may sit off
// the line from the fill to the text colour and still count as an
// anti-aliased step of a symbol in the text colour. It covers rounding in
// the rasteriser's coverage and the sRGB encoding, and is far short of the
// distance between the text colour and the accent in either scheme.
const blendTolerance = 12

// offBlend is how far c sits from the nearest colour on the line from fill
// to the drawn colour, in summed channel distance.
func offBlend(c, fill, drawn color.NRGBA) int {
	best := -1
	for a := 0; a <= 255; a++ {
		mix := func(f, d uint8) uint8 { return uint8((int(f)*(255-a) + int(d)*a + 127) / 255) }
		m := color.NRGBA{R: mix(fill.R, drawn.R), G: mix(fill.G, drawn.G), B: mix(fill.B, drawn.B), A: 255}
		if d := channelDistance(c, m); best < 0 || d < best {
			best = d
		}
	}
	return best
}

// checkSymbolColour fails when the box draws nothing in the text colour, or
// draws any pixel off the line from the fill to the text colour, or any pixel
// nearer the line from the fill to the accent than that line.
func checkSymbolColour(t *testing.T, img *image.RGBA, box image.Rectangle, fill, text, accent color.NRGBA) {
	t.Helper()
	full, off, accented := 0, 0, 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := pixelAt(img, image.Pt(x, y))
			if c == fill {
				continue
			}
			if channelDistance(c, text) <= blendTolerance {
				full++
			}
			if offBlend(c, fill, text) > blendTolerance {
				off++
			}
			if offBlend(c, fill, accent) < offBlend(c, fill, text) {
				accented++
			}
		}
	}
	if full == 0 {
		t.Errorf("no pixel of the symbol is the text colour %v", text)
	}
	if off > 0 {
		t.Errorf("%d pixels of the symbol are not a step from the fill to the text colour", off)
	}
	if accented > 0 {
		t.Errorf("%d pixels of the symbol are steps from the fill to the accent %v", accented, accent)
	}
}

// TestACatalogueSymbolIsDrawnInTheTextColour renders one catalogue cell, the
// first icon of the table, through the grid the window draws, and reads its
// symbol's square.
func TestACatalogueSymbolIsDrawnInTheTextColour(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			tok := staticThemed(t, tc.c)
			p := tok.palette
			paintBackdrop := backdrop.Widget(p.Backdrop)
			grid := Grid(tok, []int{0}, "", list.NewState())
			size := image.Pt(int(CellW)+2*int(Padding), int(CellH)+int(Padding))
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				paintBackdrop(gtx)
				return grid(gtx)
			})
			x := int(Padding) + (int(CellW)-int(IconSize))/2
			box := image.Rect(x, 8, x+int(IconSize), 8+int(IconSize))
			checkSymbolColour(t, img, box, p.Backdrop, p.Text, tc.c.ControlAccent)
		})
	}
}

// TestAMarkIsDrawnInTheTextColour reads the largest square of one mark's cell
// in the marks section.
func TestAMarkIsDrawnInTheTextColour(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			img := markCellFrame(t, tc.c, marks.Names()[0])
			box := squareAt(false, len(MarkSizes)-1)
			checkSymbolColour(t, img, box, p.Backdrop, p.Text, tc.c.ControlAccent)
		})
	}
}
