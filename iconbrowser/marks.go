package main

import (
	"image"
	"strconv"
	"strings"

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

// MarkSizeNote says what a cell's row of drawings is, in the sizes themselves
// rather than a typed-out copy of them.
func MarkSizeNote() string {
	sizes := make([]string, len(MarkSizes))
	for i, size := range MarkSizes {
		sizes[i] = strconv.Itoa(int(size))
	}
	return "one mark at " + strings.Join(sizes, ", ") + " dp"
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
// the names.
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
// name underneath.
//
// Every size is centred on the band's own line rather than stood on a common
// foot: a mark is drawn centred in its square, so centring the squares puts
// every drawing's middle on one line — which is what the eye reads as aligned.
// Standing them on a foot does not, because how far a drawing's ink reaches
// into its square is the drawing's business and differs between marks.
//
// A painter is a lookup and a closure over the set's shared cache, so building
// one per frame costs nothing worth avoiding. Mark returns nil for a name the
// set does not carry; the cell then captions the name over empty space rather
// than drawing somebody else's mark.
func markCell(gtx layout.Context, t themed, name marks.Name, cell image.Rectangle) {
	paintMark := marks.Mark(name)
	gap := gtx.Dp(MarkGap)

	row := 0
	for i, size := range MarkSizes {
		if i > 0 {
			row += gap
		}
		row += gtx.Dp(size)
	}

	band := cell.Min.Y + gtx.Dp(8)
	middle := band + gtx.Dp(MarkBand)/2
	x := cell.Min.X + (cell.Dx()-row)/2
	for _, size := range MarkSizes {
		px := gtx.Dp(size)
		if paintMark != nil {
			off := op.Offset(image.Pt(x, middle-px/2)).Push(gtx.Ops)
			paintMark(gtx, px, t.palette.Icon)
			off.Pop()
		}
		x += px + gap
	}

	caption := image.Rect(cell.Min.X, band+gtx.Dp(MarkBand)+gtx.Dp(4), cell.Max.X, cell.Max.Y)
	textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, caption, 0.5, 0.0, t.palette.Text, string(name))
}
