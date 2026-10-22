package main

// What the marks section shows, read off the pixels of one cell.
//
// The cell is captured on its own rather than out of the whole-window frame:
// MarkGrid is handed an exact CellW × MarkCellH frame, so the one cell in it
// is the frame and every square's position is arithmetic on the layout
// constants rather than a hunt through 700 rows for a band.

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
	return golden.Capture(t, image.Pt(int(CellW), int(MarkCellH)), func(gtx layout.Context) layout.Dimensions {
		paintBackdrop(gtx)
		return cell(gtx)
	})
}

// markRow is where markCell puts the row of squares in a cell of CellW: the
// leading edge of the first square and the row's whole width.
func markRow() (x0, width int) {
	for i, size := range MarkSizes {
		if i > 0 {
			width += int(MarkGap)
		}
		width += int(size)
	}
	return (int(CellW) - width) / 2, width
}

// bandRows is the vertical span every square is centred in.
func bandRows() (top, bottom int) { return 8, 8 + int(MarkBand) }

// squareAt is the rectangle of the i'th square of the row.
func squareAt(i int) image.Rectangle {
	x, _ := markRow()
	top, _ := bandRows()
	middle := top + int(MarkBand)/2
	for n, px := range sizesPx() {
		if n == i {
			return image.Rect(x, middle-px/2, x+px, middle+px-px/2)
		}
		x += px + int(MarkGap)
	}
	panic("no such square")
}

func sizesPx() []int {
	px := make([]int, len(MarkSizes))
	for i, size := range MarkSizes {
		px[i] = int(size)
	}
	return px
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

// turnedClockwise is the mask a quarter turn about the square's centre produces:
// the pixel at (x, y) lands at (n-1-y, x). Gio's y axis points down, so the
// positive rotation is the clockwise one on screen.
func turnedClockwise(mask []bool, n int) []bool {
	out := make([]bool, len(mask))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			out[x*n+(n-1-y)] = mask[y*n+x]
		}
	}
	return out
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
// the band's size, and not chevron-right turned a quarter turn.
//
// It is checked against both drawings rather than as "a down-pointing chevron
// appeared", because chevron-right turned would satisfy the weaker reading.
// The masks must agree with chevron-down and disagree with the turned
// chevron-right; the second half is only a check while the two drawings
// differ, which the test asserts first.
func TestTheChevronDownCellDrawsTheSetsChevronDown(t *testing.T) {
	n := int(MarkBand)
	// The floor is "the cell drew the mark at all", not a count the drawing
	// owes: half the square's side is well under the dozen or so pixels a
	// chevron at 24 dp paints past half way, and well over nothing.
	drawn := n / 2
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			sq := image.Rect(0, 0, n, n)
			img := markCellFrame(t, tc.c, ChevronDownMark)
			cell := drawnMask(img, squareAt(len(MarkSizes)-1), p.Backdrop, p.Text)
			down := drawnMask(markAlone(t, tc.c, marks.ChevronDown, n), sq, p.Backdrop, p.Text)
			turned := turnedClockwise(drawnMask(markAlone(t, tc.c, marks.ChevronRight, n), sq, p.Backdrop, p.Text), n)
			if got := drawnCount(cell); got < drawn {
				t.Fatalf("the cell's drawing at %d dp paints %d pixels; the cell is not drawing it", n, got)
			}
			if got := agreement(down, turned, n); got >= 0.95 {
				t.Fatalf("chevron-down and chevron-right turned agree on %.1f%% of the square; this test cannot tell them apart", got*100)
			}
			if got := agreement(cell, down, n); got < 0.95 {
				t.Errorf("the cell's drawing agrees with the set's chevron-down on %.1f%% of its square; it is not that mark", got*100)
			}
			if got := agreement(cell, turned, n); got >= 0.95 {
				t.Errorf("the cell's drawing agrees with chevron-right turned on %.1f%% of its square; it is the turned mark", got*100)
			}
		})
	}
}

// TestEveryMarkCellDrawsItsSizesAndNothingElse: every mark's cell draws its
// three sizes and nothing else, centred on the row's own arithmetic. No cell
// draws a second rendition of its mark; a row whose disclosure is open draws
// chevron-down's own cell.
func TestEveryMarkCellDrawsItsSizesAndNothingElse(t *testing.T) {
	p := PaletteFrom(tokens.PlatformLight)
	top, bottom := bandRows()
	x0, width := markRow()
	for _, name := range marks.Names() {
		t.Run(string(name), func(t *testing.T) {
			img := markCellFrame(t, tokens.PlatformLight, name)
			first, last := drawnColumns(img, image.Rect(0, top, int(CellW), bottom), p.Backdrop, p.Text)
			if first < x0 || last >= x0+width {
				t.Errorf("%s paints columns %d..%d, outside the row's %d..%d", name, first, last, x0, x0+width-1)
			}
		})
	}
}

// TestTheNoteSaysWhatChevronDownIsFor: the section's note is the only place
// either set says what its cells hold, so it says what chevron-down is drawn
// for whenever that mark's cell is on screen, and says nothing of it when a
// query keeps no chevron-down.
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
