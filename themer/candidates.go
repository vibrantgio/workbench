package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"runtime"

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
//
// There are two hints because there are two things a row can be made of. The
// share under a swatch is a fraction of whatever the colours were counted out
// of, and after a style card is clicked that is the style's own colours and not
// a photograph: a line explaining a percentage of a picture, on a window where
// no picture was ever dropped, describes a route the reader did not take and
// leaves the number in front of them with nothing to be a fraction of.
const (
	RowLabel     = "Seed candidates"
	RowHint      = "vivid first, not largest. The % is how much of the picture. Click to apply."
	RowHintStyle = "vivid first, not largest. The % is how much of the palette. Click to apply."
)

// SystemCaption is what the last cell says it is: the platform's own word
// for the setting it is showing, under the colour that setting is on. It is
// the platform's word and not one of this window's, because a reader who
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

// RowHintFor is the hint for the row the window is actually showing. It
// names the last cell where there is one, because that cell's caption is a
// setting's name and not a share of anything the hint has just described.
func RowHintFor(m Model) string {
	hint := RowHint
	if m.Style != "" {
		hint = RowHintStyle
	}
	if m.Platform.A != 0 {
		hint += " The last is the " + SystemCaption() + "."
	}
	return hint
}

// systemClick is the handler for that cell, which is the last of the row's
// slots: the ones before it belong to the candidate at the same position,
// and this one belongs to a colour that is not a candidate.
func systemClick(clicks []gesture.Click) *gesture.Click { return &clicks[len(clicks)-1] }

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
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, RowHintFor(m))

		top := labelH + gtx.Dp(RowTop)
		gap := gtx.Dp(CellGap)
		cell := image.Pt(gtx.Dp(unit.Dp(cellWidth(width, gap, len(pairs)))), gtx.Dp(CellH))
		draw := func(i int, col stdcolor.NRGBA, label string, chosen bool, click *gesture.Click, choose mvu.Message) bool {
			x := i * (cell.X + gap)
			if x+cell.X > width {
				return false // a window too narrow for the whole row shows what fits
			}
			at(gtx, image.Pt(x, top), func(gtx layout.Context) {
				Cell(gtx, p, ty, col, pairs[i], label, chosen, click, cell, choose)
			})
			return true
		}
		for i, c := range m.Candidates {
			if !draw(i, c.Color, fmt.Sprintf("%s · %s", hexOf(c.Color), share(c.Share)), !m.Follows && i == m.Selected, &clicks[i], SelectCandidate{Index: i}) {
				return layout.Dimensions{Size: image.Pt(width, top+cell.Y)}
			}
		}
		if m.Platform.A != 0 {
			draw(len(m.Candidates), m.Platform, hexOf(m.Platform)+" · "+platformName(), m.Follows, systemClick(clicks), FollowSystem{})
		}
		return layout.Dimensions{Size: image.Pt(width, top+cell.Y)}
	}
}

// Cell draws one card of the row at the origin and makes it clickable. The
// card carries three things stacked: the colour itself, the primary pair
// derived from it with its own on-colour proving the pair is legible, and
// the label under it — the colour written out with the share of the picture
// it stands for, or with the name of the setting it came off.
//
// choose is the message the card sends when it is clicked, because what a
// card offers is not always a candidate: the last one offers the colour the
// platform reports, which has no index in any list.
func Cell(gtx layout.Context, p Palette, ty Type, c stdcolor.NRGBA, pair tokens.ColorTokens, label string, chosen bool, click *gesture.Click, size image.Point, choose mvu.Message) {
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

	// The swatch is one band in the same frame the style cards wear, and it
	// wears it for the same reason: a picture's palest colour is a legal
	// candidate, and a near-white swatch on a near-white card with no
	// boundary of its own reads as a card that failed to draw rather than as
	// the colour it is.
	SwatchBands(gtx, swatch, gtx.Dp(InnerR), []imageseed.Candidate{{Color: c}}, p.Edge)
	fillRRect(gtx, chip, gtx.Dp(InnerR), pair.Primary)
	textdraw.FillText(gtx, ty.Shaper, ty.Label, chip, 0.5, 0.5, pair.OnPrimary, "Aa")

	tone := p.Muted
	if chosen {
		tone = p.Text
	}
	textdraw.FillText(gtx, ty.Shaper, ty.Small, caption, 0.5, 0.5, tone, label)

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
			mvu.MessageOp{Message: choose}.Add(gtx.Ops)
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
