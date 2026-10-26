package main

// What the marks section shows, read off the pixels of one cell.
//
// The cell is captured on its own rather than out of the whole-window frame:
// MarkGrid is handed an exact CellW × CellH frame, so the one cell in it is
// the frame. The mark's square is arithmetic on the layout constants.

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	marks "github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/theme/tokens"
)

// markThemed is the snapshot MarkGrid needs, and no more: the palette and the
// pinned typography. The prebuilt Material icons staticThemed carries are the
// catalogue grid's, and the marks section draws none of them — it paints
// through the icons package.
func markThemed(c tokens.PlatformColors) themed {
	return themed{palette: PaletteFrom(c), typ: staticTypo(tokens.DefaultTypography)}
}

// markCellFrame renders one mark's cell alone, on the window's own fill so
// that what is not the mark can be told from what is.
func markCellFrame(t *testing.T, c tokens.PlatformColors, name marks.Name) *image.RGBA {
	t.Helper()
	tok := markThemed(c)
	paintBackdrop := backdrop.Widget(tok.palette.Backdrop)
	cell := MarkGrid(tok, []marks.Name{name})
	return golden.Capture(t, image.Pt(int(CellW), int(CellH)), func(gtx layout.Context) layout.Dimensions {
		paintBackdrop(gtx)
		return cell(gtx)
	})
}

// markSquare is the square markCell draws the mark in: IconSize wide,
// centred across the cell, 8 dp below its top, as a catalogue cell's symbol.
func markSquare() image.Rectangle {
	x := (int(CellW) - int(IconSize)) / 2
	return image.Rect(x, 8, x+int(IconSize), 8+int(IconSize))
}

// drawnMask reads r as a grid of symbol-or-fill decisions, row-major. A pixel
// counts as symbol when it has travelled more than half the way from the fill
// to the symbol colour, which is what puts an anti-aliased edge on one side or
// the other without a per-scheme threshold.
func drawnMask(img *image.RGBA, r image.Rectangle, fill, symbol color.NRGBA) []bool {
	full := channelDistance(symbol, fill)
	mask := make([]bool, 0, r.Dx()*r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			mask = append(mask, full > 0 && 2*channelDistance(pixelAt(img, image.Pt(x, y)), fill) > full)
		}
	}
	return mask
}

// channelDistance is the summed per-channel distance between two opaque
// colours: a scale on which "half way from the fill to the symbol" means the
// same thing in either scheme.
func channelDistance(a, b color.NRGBA) int {
	d := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		if p[0] > p[1] {
			d += int(p[0] - p[1])
		} else {
			d += int(p[1] - p[0])
		}
	}
	return d
}

func drawnCount(mask []bool) int {
	n := 0
	for _, on := range mask {
		if on {
			n++
		}
	}
	return n
}

// agreement is how much of the two masks' DRAWN cells decide alike: the cells
// both set, over the cells either sets. Counting the empty square with them
// would make any two sparse drawings agree — the icon set's one measured band
// puts a chevron at 24 dp on a little over a dozen cells of five hundred and
// seventy-six, so emptiness alone would answer 96 per cent for two marks that
// share nothing.
func agreement(a, b []bool, _ int) float64 {
	both, either := 0, 0
	for i := range a {
		switch {
		case a[i] && b[i]:
			both++
			either++
		case a[i] || b[i]:
			either++
		}
	}
	if either == 0 {
		return 0
	}
	return float64(both) / float64(either)
}

// drawnColumns reports the first and last column of r carrying symbol pixels.
func drawnColumns(img *image.RGBA, r image.Rectangle, fill, symbol color.NRGBA) (first, last int) {
	first, last = -1, -1
	for x := r.Min.X; x < r.Max.X; x++ {
		col := image.Rect(x, r.Min.Y, x+1, r.Max.Y)
		if drawnCount(drawnMask(img, col, fill, symbol)) > 0 {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return first, last
}

// markAlone paints one mark into an n×n square on the window's own fill, the
// way the cell paints each of its squares.
func markAlone(t *testing.T, c tokens.PlatformColors, name marks.Name, n int) *image.RGBA {
	t.Helper()
	p := PaletteFrom(c)
	paintBackdrop := backdrop.Widget(p.Backdrop)
	paint := marks.Mark(name)
	if paint == nil {
		t.Fatalf("the set carries no %s", name)
	}
	return golden.Capture(t, image.Pt(n, n), func(gtx layout.Context) layout.Dimensions {
		paintBackdrop(gtx)
		paint(gtx, n, p.Text)
		return layout.Dimensions{Size: image.Pt(n, n)}
	})
}

// TestTheChevronDownCellDrawsTheSetsChevronDown reads off the frame that the
// mark a row whose disclosure is open draws is the set's own chevron-down at
// the catalogue's size.
func TestTheChevronDownCellDrawsTheSetsChevronDown(t *testing.T) {
	n := int(IconSize)
	// The floor is "the cell drew the mark at all", not a count the drawing
	// owes. Half the square's side is well under what a chevron at 40 dp
	// paints past half way, and well over nothing.
	drawn := n / 2
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			sq := image.Rect(0, 0, n, n)
			img := markCellFrame(t, tc.c, ChevronDownMark)
			cell := drawnMask(img, markSquare(), p.Backdrop, p.Text)
			down := drawnMask(markAlone(t, tc.c, marks.ChevronDown, n), sq, p.Backdrop, p.Text)
			if got := drawnCount(cell); got < drawn {
				t.Fatalf("the cell's drawing at %d dp paints %d pixels; the cell is not drawing it", n, got)
			}
			if got := agreement(cell, down, n); got < 0.95 {
				t.Errorf("the cell's drawing agrees with the set's chevron-down on %.1f%% of its square; it is not that mark", got*100)
			}
		})
	}
}

// TestEachMarkIsDrawnOnceAtTheCataloguesCellSize: every mark's cell draws
// the mark once, in the catalogue's IconSize square. The square agrees with
// the mark drawn alone at IconSize. The cell paints nothing else above its
// caption.
func TestEachMarkIsDrawnOnceAtTheCataloguesCellSize(t *testing.T) {
	p := PaletteFrom(tokens.PlatformLight)
	n := int(IconSize)
	sq := markSquare()
	above := image.Rect(0, 0, int(CellW), sq.Max.Y)
	for _, name := range marks.Names() {
		t.Run(string(name), func(t *testing.T) {
			img := markCellFrame(t, tokens.PlatformLight, name)
			alone := drawnMask(markAlone(t, tokens.PlatformLight, name, n), image.Rect(0, 0, n, n), p.Backdrop, p.Text)
			cell := drawnMask(img, sq, p.Backdrop, p.Text)
			if got := agreement(cell, alone, n); got < 0.95 {
				t.Errorf("the cell's square agrees with %s drawn at %d dp on %.1f%%; it is not that mark at the catalogue's size", name, n, got*100)
			}
			if inSquare, all := drawnCount(cell), drawnCount(drawnMask(img, above, p.Backdrop, p.Text)); all != inSquare {
				t.Errorf("%s paints %d pixels above its caption, %d of them outside its %d dp square", name, all, all-inSquare, n)
			}
		})
	}
}

// TestTheNoteNamesTheShippedSizes: the section's note names the three sizes
// the set ships each mark at.
func TestTheNoteNamesTheShippedSizes(t *testing.T) {
	const want = "the set ships each mark at 16, 20 and 24 dp"
	if got := MarkSizeNote(nil); got != want {
		t.Errorf("the note reads %q, want %q", got, want)
	}
}

// TestTheNoteSaysWhatChevronDownIsFor: the note says what chevron-down is
// drawn for whenever that mark's cell is on screen. It says nothing of it
// when a query keeps no chevron-down.
func TestTheNoteSaysWhatChevronDownIsFor(t *testing.T) {
	const clause = "chevron-down, for a row whose disclosure is open"
	plain := MarkSizeNote([]marks.Name{marks.ChevronLeft, marks.ChevronRight})
	if strings.Contains(plain, "disclosure") {
		t.Errorf("a section with no chevron-down on screen notes one: %q", plain)
	}
	full := MarkSizeNote(marks.Names())
	if !strings.HasSuffix(full, "; "+clause) {
		t.Errorf("the note %q does not end with %q", full, clause)
	}
	if !strings.HasPrefix(full, plain) || len(full) == len(plain) {
		t.Errorf("the note %q is no longer the size note %q plus what chevron-down is for", full, plain)
	}
}
