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

// ShippedSizes are the three sizes the set ships each mark at, ascending.
// The section draws each mark once at the catalogue's IconSize; the note
// names these sizes.
var ShippedSizes = []unit.Dp{16, 20, 24}

// ChevronDownMark is the mark a row whose disclosure is open draws: the set's
// own chevron-down, named by what it depicts. A closed row draws chevron-right
// and an open one draws this drawing, because one picture has one name.
const ChevronDownMark = marks.ChevronDown

// MarkSizeNote names the sizes the set ships each mark at. It also says
// what chevron-down is drawn for, but only when that mark's cell is on
// screen. The section is filtered, and a query can keep no chevron-down.
//
// The note is a clause, not a sentence: it is drawn at the trailing edge of
// a section label, beside a sibling reading "961 icons".
func MarkSizeNote(names []marks.Name) string {
	sizes := make([]string, len(ShippedSizes))
	for i, size := range ShippedSizes {
		sizes[i] = strconv.Itoa(int(size))
	}
	last := len(sizes) - 1
	note := "the set ships each mark at " + strings.Join(sizes[:last], ", ") + " and " + sizes[last] + " dp"
	for _, name := range names {
		if name == ChevronDownMark {
			return note + "; " + string(ChevronDownMark) + ", for a row whose disclosure is open"
		}
	}
	return note
}

// Heading draws one section heading on its own line, with a muted note against
// the far edge — what the section is on the left, what it holds on the right.
func Heading(t themed, heading, note string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(HeadingH))
		rect := image.Rectangle{Max: size}
		textdraw.FillText(gtx, t.typ.Shaper, t.typ.Section, rect, 0.0, 0.5, t.palette.Text, heading)
		textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, rect, 1.0, 0.5, t.palette.Muted, note)
		return layout.Dimensions{Size: size}
	}
}

// MarkGrid lays the design system's own marks out in the catalogue's cells,
// wrapping into as many rows as the width needs. Every cell draws its mark
// once through the icons package and captions it with the name a call site
// writes.
//
// Unlike the Material grid this one does not scroll: the set is meant to be
// seen at a glance.
func MarkGrid(t themed, names []marks.Name) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		cellW, cellH := gtx.Dp(CellW), gtx.Dp(CellH)
		cols := max(1, width/cellW)
		rows := (len(names) + cols - 1) / cols

		for i, name := range names {
			cell := image.Rect((i%cols)*cellW, (i/cols)*cellH, (i%cols+1)*cellW, (i/cols+1)*cellH)
			markCell(gtx, t, name, cell)
		}
		return layout.Dimensions{Size: image.Pt(width, rows*cellH)}
	}
}

// markCell paints one mark at IconSize with the name underneath, at the
// positions a catalogue cell uses.
//
// Mark returns nil for a name the set does not carry. The cell then captions
// the name over empty space; it never draws another mark.
func markCell(gtx layout.Context, t themed, name marks.Name, cell image.Rectangle) {
	px := gtx.Dp(IconSize)
	top := cell.Min.Y + gtx.Dp(8)
	paintSquare(gtx, marks.Mark(name), image.Pt(cell.Min.X+(cell.Dx()-px)/2, top), px, t.palette.Text)

	caption := image.Rect(cell.Min.X, top+px+gtx.Dp(4), cell.Max.X, cell.Max.Y)
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
