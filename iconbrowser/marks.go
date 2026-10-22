package main

import (
	"image"
	"image/color"
	"strconv"
	"strings"

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

// ChevronDownMark is the mark a row whose disclosure is open draws: the set's
// own chevron-down, named by what it depicts. A closed row draws chevron-right
// and an open one draws this drawing, because one picture has one name.
const ChevronDownMark = marks.ChevronDown

// MarkSizeNote says what a cell's row of drawings is, in the sizes themselves,
// and — only when chevron-down's cell is on screen — what that mark is drawn
// for. The clause is conditional because the section is filtered: a query
// that keeps no chevron-down would otherwise be described wrongly.
//
// It stays a clause rather than a sentence: the note is drawn at the trailing
// edge of a section label beside a sibling reading "961 icons".
func MarkSizeNote(names []marks.Name) string {
	sizes := make([]string, len(MarkSizes))
	for i, size := range MarkSizes {
		sizes[i] = strconv.Itoa(int(size))
	}
	note := "one mark at " + strings.Join(sizes, ", ") + " dp"
	for _, name := range names {
		if name == ChevronDownMark {
			return note + "; " + string(ChevronDownMark) + ", for a row whose disclosure is open"
		}
	}
	return note
}

// Heading draws one section heading on its own line, with a muted note against
// the far edge — what the section is on the left, what it holds on the right.
// The note is the only place either section says what its cells are showing,
// so a row of one mark at three sizes does not read as three marks.
func Heading(t themed, heading, note string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(HeadingH))
		rect := image.Rectangle{Max: size}
		textdraw.FillText(gtx, t.typ.Shaper, t.typ.Section, rect, 0.0, 0.5, t.palette.Text, heading)
		textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, rect, 1.0, 0.5, t.palette.Muted, note)
		return layout.Dimensions{Size: size}
	}
}

// MarkGrid lays the design system's own marks out in the catalogue's cell
// rhythm, wrapping into as many rows as the width needs. Every cell draws one
// mark through the icons package at each of MarkSizes and captions it with the
// name a call site writes.
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
// name underneath.
//
// Every size is centred on the band's own line rather than stood on a common
// foot: a mark is drawn centred in its square, so centring the squares puts
// every drawing's middle on one line. How far a drawing's paint reaches into
// its square differs between marks, so a common foot would not align them.
//
// A painter is a lookup and a closure over the set's shared cache, so building
// one per frame is cheap. Mark returns nil for a name the set does not carry;
// the cell then captions the name over empty space rather than drawing somebody
// else's mark.
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
		paintSquare(gtx, paintMark, image.Pt(x, middle-px/2), px, t.palette.Text)
		x += px + gap
	}

	caption := image.Rect(cell.Min.X, band+gtx.Dp(MarkBand)+gtx.Dp(4), cell.Max.X, cell.Max.Y)
	textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, caption, 0.5, 0.0, t.palette.Text, string(name))
}

// paintSquare draws one mark into a square of px at at.
func paintSquare(gtx layout.Context, paint marks.Painter, at image.Point, px int, c color.NRGBA) {
	if paint == nil {
		return
	}
	defer op.Offset(at).Push(gtx.Ops).Pop()
	paint(gtx, px, c)
}
