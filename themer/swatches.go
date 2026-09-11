package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"runtime"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/textdraw"
)

// The swatch row's dimensions. The cards SHARE the row's width rather than
// each taking a fixed slice of it, so a full row reaches the same trailing
// margin as everything above it instead of stopping short of it. The bounds
// keep a row of two from becoming two enormous panels and a row on a narrow
// window from becoming unreadable slivers.
const (
	CellMinW unit.Dp = 108 // narrower than this and the hex no longer fits
	CellMaxW unit.Dp = 190 // wider than this a swatch stops reading as a swatch
	CellH    unit.Dp = 104
	CellGap  unit.Dp = 12
	CellPad  unit.Dp = 12 // card edge to the swatch inside it
	SwatchH  unit.Dp = 52 // the colour itself
	CaptionH unit.Dp = 18 // the colour written out under it
	InnerR   unit.Dp = 8  // the swatch's corners
	RowTop   unit.Dp = 8  // label to cards
)

// RowLabel and RowHint head the swatch row. The hint states the ordering and
// what the number under each swatch is, because those are two different things
// and a row that says only "most prominent first" reads as broken the moment a
// vivid tenth of the picture outranks a drab half of it.
const (
	RowLabel = "Theme colour"
	RowHint  = "vivid first, not largest. The % is how much of the picture. Click to apply."
)

// SystemCaption is what the platform's own cell says it is: the platform's
// word for the setting it is showing, under the colour that setting is on. It
// is the platform's word and not one of this window's, because a reader who
// wants to change it changes it there.
func SystemCaption() string { return platformName() + " accent colour" }

// platformName is what the platform calls itself where a reader would go
// looking for the setting. A desktop that is not one of the two named is
// called what its own settings call the family of them.
func platformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	}
	return "desktop"
}

// RowHintFor is the hint for the row the window is actually showing. A row
// with no picture behind it has no shares in it to explain.
func RowHintFor(m Model) string {
	if len(m.Candidates) == 0 {
		return "the colour the platform is set to. Drop a picture for more."
	}
	return RowHint
}

// systemSlot is the position the platform's own colour takes in the row: the
// leading one, because it is the default — the colour an application wears
// having chosen none.
const systemSlot = 0

// SwatchRow draws the colours on offer as a row of cards: the platform's
// accent colour where the platform reports one, then every colour the picture
// gave, with the chosen one marked. A click on a card makes that colour the
// theme colour.
func SwatchRow(p Palette, ty Type, m Model, clicks []gesture.Click) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, width, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, RowLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, RowHintFor(m))

		cells := rowCells(m)
		top := labelH + gtx.Dp(RowTop)
		gap := gtx.Dp(CellGap)
		size := image.Pt(gtx.Dp(unit.Dp(cellWidth(width, gap, len(cells)))), gtx.Dp(CellH))
		for i, cell := range cells {
			x := i * (size.X + gap)
			if x+size.X > width {
				break // a window too narrow for the whole row shows what fits
			}
			at(gtx, image.Pt(x, top), func(gtx layout.Context) {
				Cell(gtx, p, ty, cell, &clicks[i], size)
			})
		}
		return layout.Dimensions{Size: image.Pt(width, top+size.Y)}
	}
}

// cell is one card of the row: a colour, what to call it, whether it is the
// one in force, and what choosing it says.
type cell struct {
	col    stdcolor.NRGBA
	label  string
	chosen bool
	choose mvu.Message
}

// rowCells is the row as the window draws it: the platform's own colour where
// there is one, then the picture's, in the order the extraction returned them.
func rowCells(m Model) []cell {
	out := make([]cell, 0, len(m.Candidates)+1)
	if m.Platform.A != 0 {
		out = append(out, cell{
			col:    m.Platform,
			label:  hexOf(m.Platform) + " · " + platformName(),
			chosen: m.Follows(),
			choose: FollowSystem{},
		})
	}
	for i, c := range m.Candidates {
		out = append(out, cell{
			col:    c.Color,
			label:  fmt.Sprintf("%s · %s", hexOf(c.Color), share(c.Share)),
			chosen: m.From == FromImage && i == m.Selected,
			choose: SelectCandidate{Index: i},
		})
	}
	return out
}

// cellWidth is the width one card takes when n of them share width dp of row,
// gap dp apart.
func cellWidth(width, gap, n int) int {
	if n <= 0 {
		return 0
	}
	return min(int(CellMaxW), max(int(CellMinW), (width-(n-1)*gap)/n))
}

// Cell draws one card of the row at the origin and makes it clickable: the
// colour itself over the label that names it, on the platform's box.
//
// The chosen card wears the platform's selection under the label the platform
// pairs with it, which is how a list says which of its rows is the one in
// force. Under the pointer a card takes the platform's hover overlay: a swatch
// is a graphic operated by pointing at it, which is where the platform lays
// one.
func Cell(gtx layout.Context, p Palette, ty Type, c cell, click *gesture.Click, size image.Point) {
	card := image.Rectangle{Max: size}
	fill, edge, foreground := p.Surface, p.Edge, p.CardMuted
	switch {
	case c.chosen:
		fill, edge, foreground = p.Selection, p.Selection, p.OnAccent
	case click.Hovered():
		fill = p.Hover
	}
	fillRRect(gtx, card, gtx.Dp(Radius), fill)
	strokeRRect(gtx, card, gtx.Dp(Radius), gtx.Dp(Hairline), edge)

	pad := gtx.Dp(CellPad)
	inner := card.Inset(pad)
	swatch := image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+gtx.Dp(SwatchH))
	caption := image.Rect(inner.Min.X, swatch.Max.Y+gtx.Dp(8), inner.Max.X, swatch.Max.Y+gtx.Dp(8)+gtx.Dp(CaptionH))

	// The swatch carries a hairline of its own because a picture's palest
	// colour is a legal choice, and a near-white swatch on a near-white card
	// with no boundary reads as a card that failed to draw rather than as the
	// colour it is.
	fillRRect(gtx, swatch, gtx.Dp(InnerR), c.col)
	strokeRRect(gtx, swatch, gtx.Dp(InnerR), gtx.Dp(Hairline), p.Edge)
	textdraw.FillText(gtx, ty.Shaper, ty.Small, caption, 0.5, 0.5, foreground, c.label)

	// The clickable area is the card, registered after the paint so the hover
	// state read above is the one the previous frame recorded.
	area := clip.UniformRRect(card, gtx.Dp(Radius)).Push(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: c.choose}.Add(gtx.Ops)
		}
	}
}

// share writes a colour's coverage as a percentage, with a floor so a vivid
// sliver of an image reads as "under 1%" rather than as "0%".
func share(f float64) string {
	pct := f * 100
	if pct < 1 {
		return "<1%"
	}
	return fmt.Sprintf("%.0f%%", pct)
}
