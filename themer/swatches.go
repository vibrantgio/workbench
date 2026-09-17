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
// edge as the box it stands in instead of stopping short of it. The bounds
// keep a row of two from becoming two enormous panels and a row on a narrow
// window from becoming unreadable slivers.
const (
	CellMinW unit.Dp = 108 // narrower than this and the hex no longer fits
	CellMaxW unit.Dp = 190 // wider than this a swatch stops reading as a swatch
	CellGap  unit.Dp = 12
	CellPad  unit.Dp = 6  // card edge to the swatch inside it
	SwatchH  unit.Dp = 34 // the colour itself
	CaptionH unit.Dp = 16 // the colour written out under it
	InnerR   unit.Dp = 8  // the swatch's corners, and the picture's mat

	// MarkRing and MarkGutter are how the platform marks a chosen thumbnail,
	// MEASURED off the Appearance pane's own row in
	// `system-settings-grouped-box-light.png`: the ring round the selected
	// thumbnail spans x 455–457 and y 62–64, three points of the accent, and
	// one point of the box's own fill stands between it and the thumbnail at
	// x 458. Nothing else about the thumbnail changes when it is chosen.
	MarkRing   unit.Dp = 3
	MarkGutter unit.Dp = 1
)

// RowHint says what the cards are, at the trailing end of the picture
// group's title row. It states the ordering and what the number under each
// swatch is, because those are two different things and a row that says only
// "most prominent first" reads as broken the moment a vivid tenth of the
// picture outranks a drab half of it.
const RowHint = "Vivid colours first, not the largest areas; the percentage is each colour's share of the picture."

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
// with no picture behind it has no shares in it to explain, and what it says
// instead is how a picture gets here: the window is the drop target and has no
// edges of its own to say so.
func RowHintFor(m Model) string {
	if len(m.Candidates) == 0 {
		return "Drop a picture on this window and its colours come back here."
	}
	return RowHint
}

// systemSlot is the position the platform's own colour takes in the row: the
// leading one, because it is the default — the colour an application wears
// having chosen none.
const systemSlot = 0

// SwatchRow draws the colours on offer as a row of cards, filling the run of
// the picture group's box that the mat left: the platform's accent colour
// where the platform reports one, then every colour the picture gave, with the
// chosen one marked. A click on a card makes that colour the theme colour.
func SwatchRow(p Palette, ty Type, m Model, clicks []gesture.Click) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		room := gtx.Constraints.Max
		cells := rowCells(m)
		gap := gtx.Dp(CellGap)
		size := image.Pt(gtx.Dp(unit.Dp(cellWidth(room.X, gap, len(cells)))), room.Y)
		for i, cell := range cells {
			x := i * (size.X + gap)
			if x+size.X > room.X {
				break // a window too narrow for the whole row shows what fits
			}
			at(gtx, image.Pt(x, 0), func(gtx layout.Context) {
				Cell(gtx, p, ty, cell, &clicks[i], size)
			})
		}
		return layout.Dimensions{Size: room}
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
// The chosen card is marked by a RING round its swatch and by nothing else —
// the swatch keeps its size and its place, and the caption keeps its colour
// and its place. That is how the platform marks a chosen thumbnail, and it is
// what this row needs: a card filled with the platform's selection put a
// second large flat colour beside the six the row exists to compare, and read
// cold it made the tile twitch, "the caption jumped from below the tile to
// inside it".
//
// Under the pointer a card takes the platform's hover overlay: a swatch is a
// graphic operated by pointing at it, which is where the platform lays one.
//
// The card carries no resting hairline. It stands inside a grouped box, and
// the platform's box carries none — what tells one card from the next is the
// air between them and the swatch's own edge.
func Cell(gtx layout.Context, p Palette, ty Type, c cell, click *gesture.Click, size image.Point) {
	card := image.Rectangle{Max: size}
	if click.Hovered() {
		fillRRect(gtx, card, gtx.Dp(InnerR), p.Hover)
	}

	inner := card.Inset(gtx.Dp(CellPad))
	swatch := image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+gtx.Dp(SwatchH))
	caption := image.Rect(inner.Min.X, inner.Max.Y-gtx.Dp(CaptionH), inner.Max.X, inner.Max.Y)

	if c.chosen {
		ring, gutter := gtx.Dp(MarkRing), gtx.Dp(MarkGutter)
		strokeRRect(gtx, swatch.Inset(-(ring + gutter)), gtx.Dp(InnerR)+ring+gutter, ring, p.Accent)
	}

	// The swatch carries a hairline of its own because a picture's palest
	// colour is a legal choice, and a near-white swatch on a near-white card
	// with no boundary reads as a card that failed to draw rather than as the
	// colour it is.
	fillRRect(gtx, swatch, gtx.Dp(InnerR), c.col)
	strokeRRect(gtx, swatch, gtx.Dp(InnerR), gtx.Dp(Hairline), p.Edge)
	textdraw.FillText(gtx, ty.Shaper, ty.Small, caption, 0.5, 0.5, p.CardText, c.label)

	// The clickable area is the card, registered after the paint so the hover
	// state read above is the one the previous frame recorded.
	area := clip.UniformRRect(card, gtx.Dp(InnerR)).Push(gtx.Ops)
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
