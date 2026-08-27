package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	marks "github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/textdraw"
)

// MarkSizes are the sizes every mark is shown at, ascending: the three sizes
// the library draws an icon at. A mark is drawn on one grid but it is not
// pixel-exact at all three, so the section shows all three rather than one —
// what an author sees here is what a control ships.
var MarkSizes = []unit.Dp{16, 20, 24}

// TurnedMark is the one mark of the set whose single drawing serves two states.
// The icons package draws disclosure as the row stands closed, and a control
// showing an open row turns that same drawing a quarter turn rather than
// reaching for a second name — vaultview's disclosure rows are the precedent.
// So its cell here shows the turn too: an author who reads this section as
// "what a control ships" would otherwise never see the state the set does not
// carry a name for.
//
// It is a name the set carries, not a name of its own: everything the cell adds
// is drawn by the painter marks.Mark(TurnedMark) returns.
const TurnedMark = marks.Disclosure

// MarkSizeNote says what a cell's row of drawings is, in the sizes themselves
// rather than a typed-out copy of them — and, when the turned mark's cell is on
// screen, what the extra drawing in it is. The clause is conditional because
// the section is filtered: a query that keeps only the history marks shows no
// turn, and a note describing one would be describing the wrong screen.
//
// It is a clause and not a sentence on purpose. This note is shipped chrome
// hanging off the trailing edge of a section label, next to a sibling that says
// "961 icons", and prose in that slot reads as a design note somebody left in
// the window. So it names the mark, says it is turned, and says what the turn
// is for — the mark's own name is the subject, which is what keeps the turned
// drawing from reading as a second mark called "open".
func MarkSizeNote(names []marks.Name) string {
	sizes := make([]string, len(MarkSizes))
	for i, size := range MarkSizes {
		sizes[i] = strconv.Itoa(int(size))
	}
	note := "one mark at " + strings.Join(sizes, ", ") + " dp"
	for _, name := range names {
		if name == TurnedMark {
			return note + "; " + string(TurnedMark) + " also turned, for an open row"
		}
	}
	return note
}

// Heading draws one section label on its own line, with a muted note against
// the far edge — what the section is on the left, what it holds on the right.
// The note is the only place either section says what its cells are showing,
// so a row of one mark at three sizes does not read as three marks.
func Heading(t themed, label, note string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(HeadingH))
		rect := image.Rectangle{Max: size}
		textdraw.FillText(gtx, t.typ.Shaper, t.typ.Section, rect, 0.0, 0.5, t.palette.Text, label)
		textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, rect, 1.0, 0.5, t.palette.Muted, note)
		return layout.Dimensions{Size: size}
	}
}

// MarkGrid lays the design system's own marks out in the catalogue's cell
// rhythm, wrapping into as many rows as the width needs. Every cell draws one
// mark through the icons package at each of MarkSizes and captions it with the
// name a call site writes — not a picture's description, which is the point of
// the names. TurnedMark's cell draws that same mark once more, turned; the
// caption underneath is still the one name, because that is still all a call
// site can write.
//
// Unlike the Material grid this one does not scroll: the set is small, and its
// whole purpose here is to be seen at a glance before somebody draws a mark
// that already exists.
func MarkGrid(t themed, names []marks.Name) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		cellW, cellH := gtx.Dp(CellW), gtx.Dp(MarkCellH)
		cols := max(1, width/cellW)
		rows := (len(names) + cols - 1) / cols

		for i, name := range names {
			cell := image.Rect((i%cols)*cellW, (i/cols)*cellH, (i%cols+1)*cellW, (i/cols+1)*cellH)
			markCell(gtx, t, name, cell)
		}
		return layout.Dimensions{Size: image.Pt(width, rows*cellH)}
	}
}

// markCell paints one mark at every size in MarkSizes, ascending, with the
// name underneath — and for TurnedMark that same mark once more at the band's
// own size, turned a quarter turn.
//
// Every size is centred on the band's own line rather than stood on a common
// foot: a mark is drawn centred in its square, so centring the squares puts
// every drawing's middle on one line — which is what the eye reads as aligned.
// Standing them on a foot does not, because how far a drawing's ink reaches
// into its square is the drawing's business and differs between marks. The turn
// keeps that rule: it is the same square, so its middle is on the same line.
//
// The turned drawing is shown once, at MarkBand — the largest size, where the
// quarter turn is most legible — rather than beside every size. That is the
// cell's width deciding: three sizes doubled with a gap between each is 180 dp
// against a 160 dp CellW, so a pair per size either overflows the cell or
// crushes the gutter that separates one cell from the next. One turn costs a
// gap and a band, which the row's own slack covers, and the sizes are what the
// three closed drawings are already there to show.
//
// It sits at the row's trailing end under the plain MarkGap rather than a wider
// separation of its own, and that is grouping rather than thrift: what is left
// over centres the row, so widening the gap inside the cell narrows the gutter
// between cells by the same amount, and past a point the turned drawing reads
// as belonging to the mark on its right. Nothing is lost by it — the drawing is
// turned, which no size in an ascending row is, and it repeats a size rather
// than continuing the ascent.
//
// A painter is a lookup and a closure over the set's shared cache, so building
// one per frame costs nothing worth avoiding. Mark returns nil for a name the
// set does not carry; the cell then captions the name over empty space rather
// than drawing somebody else's mark.
func markCell(gtx layout.Context, t themed, name marks.Name, cell image.Rectangle) {
	paintMark := marks.Mark(name)
	gap := gtx.Dp(MarkGap)
	turn := name == TurnedMark

	row := 0
	for i, size := range MarkSizes {
		if i > 0 {
			row += gap
		}
		row += gtx.Dp(size)
	}
	if turn {
		row += gap + gtx.Dp(MarkBand)
	}

	band := cell.Min.Y + gtx.Dp(8)
	middle := band + gtx.Dp(MarkBand)/2
	x := cell.Min.X + (cell.Dx()-row)/2
	for _, size := range MarkSizes {
		px := gtx.Dp(size)
		paintSquare(gtx, paintMark, image.Pt(x, middle-px/2), px, t.palette.Icon, false)
		x += px + gap
	}
	if turn {
		px := gtx.Dp(MarkBand)
		paintSquare(gtx, paintMark, image.Pt(x, middle-px/2), px, t.palette.Icon, true)
	}

	caption := image.Rect(cell.Min.X, band+gtx.Dp(MarkBand)+gtx.Dp(4), cell.Max.X, cell.Max.Y)
	textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, caption, 0.5, 0.0, t.palette.Text, string(name))
}

// paintSquare draws one mark into a square of px at at, optionally turned a
// quarter turn about that square's own centre.
//
// The turn is a paint-time affine over the one registered drawing, not a second
// drawing: it is vaultview's own disclosure transform, applied here to the same
// painter the closed drawings in the row use. The mark occupies the whole
// square, so the centre it turns about is the square's — which is what keeps
// the turned drawing on the band's line and inside its own slot.
func paintSquare(gtx layout.Context, paint marks.Painter, at image.Point, px int, c color.NRGBA, turn bool) {
	if paint == nil {
		return
	}
	defer op.Offset(at).Push(gtx.Ops).Pop()
	if turn {
		half := float32(px) / 2
		defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), math.Pi/2)).Push(gtx.Ops).Pop()
	}
	paint(gtx, px, c)
}
