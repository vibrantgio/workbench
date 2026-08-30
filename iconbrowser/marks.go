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
// the library draws an icon at. A mark is drawn on one grid but is not
// pixel-exact at all three, so the section shows all three rather than one.
var MarkSizes = []unit.Dp{16, 20, 24}

// TurnedMark is the one mark of the set whose single drawing serves two states.
// The icons package draws disclosure as the row stands closed, and a control
// showing an open row turns that same drawing a quarter turn rather than
// reaching for a second name, so this cell shows the turn as well.
//
// It is a name the set carries, not a name of its own: everything the cell adds
// is drawn by the painter marks.Mark(TurnedMark) returns.
const TurnedMark = marks.Disclosure

// MarkSizeNote says what a cell's row of drawings is, in the sizes themselves,
// and — only when the turned mark's cell is on screen — what the extra drawing
// in it is. The clause is conditional because the section is filtered: a query
// that keeps no turned mark would otherwise be described wrongly.
//
// It stays a clause rather than a sentence: the note hangs off the trailing
// edge of a section label beside a sibling reading "961 icons". Naming the mark
// as its subject is what keeps the turned drawing from reading as a second mark
// called "open".
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
// name a call site writes. TurnedMark's cell draws that same mark once more,
// turned; the caption underneath is still the one name, because that is all a
// call site can write.
//
// Unlike the Material grid this one does not scroll: the set is meant to be
// seen at a glance.
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
// every drawing's middle on one line. How far a drawing's ink reaches into its
// square differs between marks, so a common foot would not align them.
//
// The turned drawing is shown once, at MarkBand, rather than beside every size:
// three sizes doubled with a gap between each is 180 dp against a 160 dp CellW,
// so a pair per size either overflows the cell or crushes the gutter between
// cells. It sits at the row's trailing end under the plain MarkGap — widening
// that gap would narrow the gutter between cells by the same amount, since what
// is left over centres the row.
//
// A painter is a lookup and a closure over the set's shared cache, so building
// one per frame is cheap. Mark returns nil for a name the set does not carry;
// the cell then captions the name over empty space rather than drawing somebody
// else's mark.
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
// drawing. The mark occupies the whole square, so the centre it turns about is
// the square's, which keeps the turned drawing on the band's line and inside
// its own slot.
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
