package main

import (
	"fmt"
	"image"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// The candidate row's dimensions. The cards SHARE the row's width rather
// than each taking a fixed slice of it, so a full row reaches the same right
// margin as everything above it instead of stopping short of it with a slab
// of dead space wide enough to look like a card that failed to load. The
// bounds keep a row of two from becoming two enormous panels and a row on a
// narrow window from becoming unreadable slivers.
const (
	CellMinW unit.Dp = 108 // narrower than this and the hex no longer fits
	CellMaxW unit.Dp = 190 // wider than this a swatch stops reading as a swatch
	CellH    unit.Dp = 140
	CellGap  unit.Dp = 12
	CellPad  unit.Dp = 12 // card edge to the swatch inside it
	SwatchH  unit.Dp = 56 // the candidate colour itself
	ChipH    unit.Dp = 28 // the primary pair generated from it
	CaptionH unit.Dp = 18 // the colour written out under the pair
	InnerR   unit.Dp = 8  // swatch and chip corners
	RowTop   unit.Dp = 8  // label to cards
)

// RowLabel and RowHint head the candidate row. The hint states the ordering
// and what the number under each swatch is, because those are two different
// things and a row that says only "most prominent first" reads as broken the
// moment a vivid tenth of the picture outranks a drab half of it.
const (
	RowLabel = "Seed candidates"
	RowHint  = "vivid first, not largest. The % is how much of the picture. Click to apply."
)

// cellWidth is the width one card takes when n of them share width dp of
// row, gap dp apart.
func cellWidth(width, gap, n int) int {
	if n <= 0 {
		return 0
	}
	return min(int(CellMaxW), max(int(CellMinW), (width-(n-1)*gap)/n))
}

// CandidateRow draws the extracted seeds as a row of cards, each showing the
// colour and the primary pair a palette derivation makes of it, with the
// chosen one ringed. A click on a card selects it.
func CandidateRow(p Palette, ty Type, m Model, pairs []tokens.ColorTokens, clicks []gesture.Click) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, width, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, RowLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, RowHint)

		top := labelH + gtx.Dp(RowTop)
		gap := gtx.Dp(CellGap)
		cell := image.Pt(gtx.Dp(unit.Dp(cellWidth(width, gap, len(m.Candidates)))), gtx.Dp(CellH))
		for i := range m.Candidates {
			x := i * (cell.X + gap)
			if x+cell.X > width {
				break // a window too narrow for the whole row shows what fits
			}
			at(gtx, image.Pt(x, top), func(gtx layout.Context) {
				Cell(gtx, p, ty, m.Candidates[i], pairs[i], i, i == m.Selected, &clicks[i], cell)
			})
		}
		return layout.Dimensions{Size: image.Pt(width, top+cell.Y)}
	}
}

// Cell draws one candidate card at the origin and makes it clickable. The
// card carries three things stacked: the colour as it occurs in the picture,
// the primary pair derived from it with its own on-colour proving the pair
// is legible, and the colour written out with the share of the image it
// stands for.
func Cell(gtx layout.Context, p Palette, ty Type, c imageseed.Candidate, pair tokens.ColorTokens, index int, chosen bool, click *gesture.Click, size image.Point) {
	card := image.Rectangle{Max: size}
	fill, edge, width := p.Surface, p.CardEdge, gtx.Dp(Hairline)
	if click.Hovered() {
		fill = p.Selection
	}
	if chosen {
		fill, edge, width = p.Selection, p.Accent, gtx.Dp(Ring)
	}
	fillRRect(gtx, card, gtx.Dp(Radius), fill)
	strokeRRect(gtx, card, gtx.Dp(Radius), width, edge)

	pad := gtx.Dp(CellPad)
	inner := card.Inset(pad)
	swatch := image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+gtx.Dp(SwatchH))
	chip := image.Rect(inner.Min.X, swatch.Max.Y+gtx.Dp(8), inner.Max.X, swatch.Max.Y+gtx.Dp(8)+gtx.Dp(ChipH))
	caption := image.Rect(inner.Min.X, chip.Max.Y+gtx.Dp(6), inner.Max.X, chip.Max.Y+gtx.Dp(6)+gtx.Dp(CaptionH))

	fillRRect(gtx, swatch, gtx.Dp(InnerR), c.Color)
	fillRRect(gtx, chip, gtx.Dp(InnerR), pair.Primary)
	textdraw.FillText(gtx, ty.Shaper, ty.Label, chip, 0.5, 0.5, pair.OnPrimary, "Aa")

	label, tone := hexOf(c.Color), p.Muted
	if chosen {
		tone = p.Text
	}
	textdraw.FillText(gtx, ty.Shaper, ty.Small, caption, 0.5, 0.5, tone,
		fmt.Sprintf("%s · %s", label, share(c.Share)))

	// The clickable area is the card, registered after the paint so the
	// hover state read above is the one the previous frame recorded.
	area := clip.UniformRRect(card, gtx.Dp(Radius)).Push(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: SelectCandidate{Index: index}}.Add(gtx.Ops)
		}
	}
}

// share writes a candidate's coverage as a percentage, with a floor so a
// vivid sliver of an image reads as "under 1%" rather than as "0%".
func share(f float64) string {
	pct := f * 100
	if pct < 1 {
		return "<1%"
	}
	return fmt.Sprintf("%.0f%%", pct)
}
