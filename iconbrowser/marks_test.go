package main

// What the marks section shows, read off the pixels of one cell.
//
// The cell is captured on its own rather than out of the whole-window frame:
// MarkGrid is handed an exact CellW × MarkCellH canvas, so the one cell in it
// IS the frame and every square's position is arithmetic on the layout
// constants instead of a hunt through 700 rows for a band. The window render
// next door still draws the same cells inside the real page — this is the same
// composition read closer.

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
// pinned typography. The prebuilt Material widgets staticThemed carries are the
// catalogue grid's, and the marks section draws none of them — it paints
// through the icons package.
func markThemed(c tokens.ColorTokens) themed {
	return themed{palette: PaletteFrom(c), typ: staticTypo(tokens.DefaultTypography)}
}

// markCellFrame renders one mark's cell alone, on the window's own ground so
// that what is not the mark can be told from what is.
func markCellFrame(t *testing.T, c tokens.ColorTokens, name marks.Name) *image.RGBA {
	t.Helper()
	tok := markThemed(c)
	ground := backdrop.Widget(tok.palette.Backdrop)
	cell := MarkGrid(tok, []marks.Name{name})
	return golden.Capture(t, image.Pt(int(CellW), int(MarkCellH)), func(gtx layout.Context) layout.Dimensions {
		ground(gtx)
		return cell(gtx)
	})
}

// markRow is where markCell puts the row of squares in a cell of CellW: the
// leading edge of the first square and the row's whole width, with the turned
// square counted when the cell is the one that shows it. The arithmetic is the
// cell's own, restated here to say which pixels the assertions below are about
// — what they then assert is what the pixels are, which the arithmetic cannot
// tell them.
func markRow(turn bool) (x0, width int) {
	for i, size := range MarkSizes {
		if i > 0 {
			width += int(MarkGap)
		}
		width += int(size)
	}
	if turn {
		width += int(MarkGap) + int(MarkBand)
	}
	return (int(CellW) - width) / 2, width
}

// bandRows is the vertical span every square is centred in.
func bandRows() (top, bottom int) { return 8, 8 + int(MarkBand) }

// squareAt is the rectangle of the i'th square of the row, counting the turned
// one as the last.
func squareAt(turn bool, i int) image.Rectangle {
	x, _ := markRow(turn)
	top, _ := bandRows()
	middle := top + int(MarkBand)/2
	sizes := sizesPx()
	if turn {
		sizes = append(sizes, int(MarkBand))
	}
	for n, px := range sizes {
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

// inkMask reads r as a grid of ink-or-ground decisions, row-major. A pixel
// counts as ink when it has travelled more than half the way from the ground to
// the glyph colour, which is what puts an anti-aliased edge on one side or the
// other without a per-scheme threshold.
func inkMask(img *image.RGBA, r image.Rectangle, ground, ink color.NRGBA) []bool {
	full := channelDistance(ink, ground)
	mask := make([]bool, 0, r.Dx()*r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			mask = append(mask, full > 0 && 2*channelDistance(pixelAt(img, image.Pt(x, y)), ground) > full)
		}
	}
	return mask
}

// channelDistance is the summed per-channel distance between two opaque
// colours: a scale on which "half way from the ground to the glyph" means the
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

func inkCount(mask []bool) int {
	n := 0
	for _, on := range mask {
		if on {
			n++
		}
	}
	return n
}

// agreement is the fraction of the two masks' cells that decide alike. Both
// must describe a square of side n.
func agreement(a, b []bool, n int) float64 {
	same := 0
	for i := range a {
		if a[i] == b[i] {
			same++
		}
	}
	return float64(same) / float64(n*n)
}

// turnedClockwise is the mask a quarter turn about the square's centre produces:
// the pixel at (x, y) lands at (n-1-y, x). Gio's y axis points down, so the
// positive rotation markCell applies is the clockwise one on screen — which is
// the turn that takes a mark pointing at the row it would open into one
// pointing at the children it has opened.
func turnedClockwise(mask []bool, n int) []bool {
	out := make([]bool, len(mask))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			out[x*n+(n-1-y)] = mask[y*n+x]
		}
	}
	return out
}

// inkColumns reports the first and last column of r carrying ink.
func inkColumns(img *image.RGBA, r image.Rectangle, ground, ink color.NRGBA) (first, last int) {
	first, last = -1, -1
	for x := r.Min.X; x < r.Max.X; x++ {
		col := image.Rect(x, r.Min.Y, x+1, r.Max.Y)
		if inkCount(inkMask(img, col, ground, ink)) > 0 {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return first, last
}

// TestTheTurnedCellDrawsTheOpenRendition is the phase's whole claim read off
// the frame: the cell that shows a turn draws, beside its closed row, that same
// drawing turned a quarter turn about its own square's centre.
//
// It is checked as a rotation rather than as "a fourth drawing appeared",
// because a second registered mark would satisfy the weaker reading. The masks
// have to agree once the closed one is turned, and they have to DISagree when
// they are not — a mark that came out the same either way would make the first
// half of this test vacuous, and a cell that forgot the transform would pass it.
func TestTheTurnedCellDrawsTheOpenRendition(t *testing.T) {
	n := int(MarkBand)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := markCellFrame(t, tc.c, TurnedMark)
			p := PaletteFrom(tc.c)

			closed := inkMask(img, squareAt(true, len(MarkSizes)-1), p.Backdrop, p.Icon)
			turned := inkMask(img, squareAt(true, len(MarkSizes)), p.Backdrop, p.Icon)
			if got := inkCount(closed); got < n {
				t.Fatalf("the closed drawing at %d dp inks %d pixels; the cell is not drawing it", n, got)
			}
			if got := inkCount(turned); got < n {
				t.Fatalf("the turned drawing at %d dp inks %d pixels; the cell is not drawing it", n, got)
			}
			if got := agreement(turnedClockwise(closed, n), turned, n); got < 0.95 {
				t.Errorf("the extra drawing agrees with the quarter-turned mark on %.1f%% of its square; it is not that mark turned", got*100)
			}
			if got := agreement(closed, turned, n); got >= 0.95 {
				t.Errorf("the extra drawing agrees with the unturned mark on %.1f%% of its square; nothing was turned", got*100)
			}
		})
	}
}

// TestTheTurnedCellKeepsTheCellsGutter guards the layout decision the turn was
// fitted into: the row grew by a gap and a band, it is still centred in the
// cell, and it still ends inside it — which is what leaves the gutter between
// one cell and the next wider than any gap inside either.
func TestTheTurnedCellKeepsTheCellsGutter(t *testing.T) {
	img := markCellFrame(t, tokens.DefaultLight, TurnedMark)
	p := PaletteFrom(tokens.DefaultLight)
	top, bottom := bandRows()
	x0, width := markRow(true)

	first, last := inkColumns(img, image.Rect(0, top, int(CellW), bottom), p.Backdrop, p.Icon)
	if first < x0 || last >= x0+width {
		t.Errorf("the row inks columns %d..%d, outside the %d..%d it is laid out in", first, last, x0, x0+width-1)
	}
	closed := squareAt(true, len(MarkSizes)-1)
	if last <= closed.Max.X {
		t.Errorf("the row's last ink is column %d and its closed drawings end at %d; there is no second rendition", last, closed.Max.X)
	}
	// The gutter is what is left over on both sides of one cell, and cells
	// abut: two of those margins is the distance between neighbours.
	if 2*x0 <= int(MarkGap) {
		t.Errorf("the turned row leaves %d dp each side of a cell, against the %d dp gap inside it; the turn has eaten the gutter that groups a cell",
			x0, int(MarkGap))
	}
}

// TestPlainMarkCellsAreUnchanged is the other half: every other mark's cell
// still draws its three sizes and nothing else, centred on the closed row's own
// arithmetic. The turn belongs to the one mark whose set doc gives it two
// states, not to the section.
func TestPlainMarkCellsAreUnchanged(t *testing.T) {
	p := PaletteFrom(tokens.DefaultLight)
	top, bottom := bandRows()
	x0, width := markRow(false)
	for _, name := range marks.Names() {
		if name == TurnedMark {
			continue
		}
		t.Run(string(name), func(t *testing.T) {
			img := markCellFrame(t, tokens.DefaultLight, name)
			first, last := inkColumns(img, image.Rect(0, top, int(CellW), bottom), p.Backdrop, p.Icon)
			if first < x0 || last >= x0+width {
				t.Errorf("%s inks columns %d..%d, outside the closed row's %d..%d", name, first, last, x0, x0+width-1)
			}
		})
	}
}

// TestTheNoteSaysWhatTheTurnedDrawingIs is the caption side of the same claim.
// The section's note is the only place either set says what its cells hold, and
// a cell showing four drawings of a set of four names has to be accounted for
// there — otherwise the turned drawing reads as a mark called "open", which is
// a name no call site can write.
func TestTheNoteSaysWhatTheTurnedDrawingIs(t *testing.T) {
	plain := MarkSizeNote([]marks.Name{marks.HistoryBack, marks.HistoryForward})
	if strings.Contains(plain, "turned") {
		t.Errorf("a section with no turn on screen notes one: %q", plain)
	}
	full := MarkSizeNote(marks.Names())
	if !strings.Contains(full, "turned") || !strings.Contains(full, string(TurnedMark)) {
		t.Errorf("the note does not say what the turned drawing is: %q", full)
	}
	if !strings.HasPrefix(full, plain) || len(full) == len(plain) {
		t.Errorf("the note %q is no longer the size note %q plus what the turn adds", full, plain)
	}
}
