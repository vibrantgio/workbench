package main

// The readout block: the SK150's own display, in the device's own colours
// measured off the photograph of it. Each line is big mono digits with a
// half-size capital unit letter baseline-aligned in a right column; while the
// output is regulating, the line of the quantity in control wears the mode
// badge above its unit letter — CV on the volt line in the volt colour, CC on
// the amp line in the amp colour, each a lit pill with the panel's black cut
// out of it, as the device draws them. Output state lives in the header's
// bolt + ON/OFF cluster beside the power toggle, not on the readout block.
//
// Everything aligns with the digits' GLYPHS, not their line box: the box
// carries the font's leading, and trim hung off its edges floats
// visibly too far from the glyphs.

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
)

// boltDp is the construction size of the output bolt; the header's output
// cluster hands the bolt exact constraints at draw time, so this is only
// its default.
const boltDp unit.Dp = 26

// Roboto Mono's vertical metrics as fractions of the line box: where the
// digit glyphs' top (ascent minus cap height) and the baseline sit. The
// digits and unit letters are caps and numerals, so paint runs from capTop to
// the baseline exactly.
const (
	capTopFrac = 0.256
	baseFrac   = 0.795
)

// voltRow is the voltage line: zero-padded digits, and — while the output
// is on and voltage is the quantity in control — the CV badge above the
// unit letter.
func voltRow(t themed, r Reading) layout.Widget {
	badgeTxt := ""
	if r.On && !r.CC {
		badgeTxt = "CV"
	}
	return panelRow(t, t.palette.DisplayVolt, fmt.Sprintf("%05.2f", r.VOut), "V", badgeTxt, t.palette.DisplayVolt)
}

// presetBadge is the active-preset pill ("M2"): the unit letters' style on
// the accent, on the line above the readout panel and so outside it.
func presetBadge(t themed, active int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !isGroup(active) {
			return layout.Dimensions{}
		}
		typ := t.typ
		txt := fmt.Sprintf("M%d", active)
		sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Unit, txt)
		r := image.Rect(0, 0, sz.X+2*gtx.Dp(8), sz.Y+2*gtx.Dp(2))
		badgeBoxStyled(gtx, t, typ.Unit, txt, t.palette.Accent, t.palette.FilledText, true, r)
		return layout.Dimensions{Size: r.Max}
	}
}

// readoutWidth is the natural width of the readout block: the widest of
// the three digit strings, the gap, and the unit column.
func readoutWidth(gtx layout.Context, t themed, r Reading) int {
	typ := t.typ
	digits := 0
	for _, s := range []string{fmt.Sprintf("%05.2f", r.VOut), fmt.Sprintf("%5.3f", r.IOut), fmt.Sprintf("%.3f", r.Power)} {
		digits = max(digits, textdraw.MeasureText(gtx, typ.Shaper, typ.Digits, s).X)
	}
	units := 0
	for _, u := range []string{"V", "A", "W"} {
		units = max(units, textdraw.MeasureText(gtx, typ.Shaper, typ.Unit, u).X)
	}
	return digits + gtx.Dp(8) + units
}

// ampRow is the current line — while the output is current-limited it
// wears the CC badge above the unit letter.
func ampRow(t themed, r Reading) layout.Widget {
	badgeTxt := ""
	if r.On && r.CC {
		badgeTxt = "CC"
	}
	return panelRow(t, t.palette.DisplayAmp, fmt.Sprintf("%5.3f", r.IOut), "A", badgeTxt, t.palette.DisplayAmp)
}

// wattRow is the power line, three decimals like the device.
func wattRow(t themed, r Reading) layout.Widget {
	return panelRow(t, t.palette.DisplayWatt, fmt.Sprintf("%.3f", r.Power), "W", "", color.NRGBA{})
}

// setLimitLine is what one of the two boxes at the panel's foot holds: the
// title the device gives the pair, and the pair's two values with their
// units.
type setLimitLine struct {
	Caption string
	Volts   string
	Amps    string
}

// The stand-ins shown while the live group has not been read: one dash per
// digit, so the lines hold the width the values will take.
const (
	dashVolts = "--.-- V"
	dashAmps  = "-.--- A"
)

// setLimitLines is what the two boxes hold: the live group's two
// setpoints, and the two protections that cut the output.
func setLimitLines(m Model) [2]setLimitLine {
	if !m.HaveS {
		return [2]setLimitLine{{"Set", dashVolts, dashAmps}, {"Limit", dashVolts, dashAmps}}
	}
	return [2]setLimitLine{
		{"Set", fmt.Sprintf(FVSet.spec().Format, m.S.Value(FVSet)), fmt.Sprintf(FISet.spec().Format, m.S.Value(FISet))},
		{"Limit", fmt.Sprintf(FOVP.spec().Format, m.S.Value(FOVP)), fmt.Sprintf(FOCP.spec().Format, m.S.Value(FOCP))},
	}
}

// setLimitBoxes is the panel's bottom block: the two bordered boxes the
// OWON SPE6103 carries across the foot of its display, Set at the left and
// Limit at the right, read off reference/sk150-display-2026-09-23.jpeg
// upright. Each box holds its title on the first line and its volts and its
// amps side by side on the second, so a box is read as one setting of the
// supply and not as four loose numbers.
//
// The geometry is the photograph's, in its own proportions:
//
//   - The boxes take equal halves of the width they are given. On the
//     photograph they are 929 px and 930 px wide inside a display 2250 px
//     across, so they fill it edge to edge.
//   - The gap between them is 8 px there against a 929 px box, under a
//     hundredth of the box; at this panel's width that rounds to a crack
//     between two hairlines, so the gap is 8 dp — the panel's own gap
//     between a digit column and its unit letter, the smallest separation
//     the panel already reads as a gap.
//   - The box edges are square on the photograph and the recessed value
//     area inside them is chamfered by about 5 px on a 929 px box. The two
//     shapes are drawn here as one, so the corner takes that chamfer: 2 dp,
//     barely rounded.
//   - The titles are centred over their boxes, as the photograph centres
//     Set and Limit over theirs, and not left-aligned: a title centred over
//     a box names the whole box, which is what a two-value box needs.
//   - Each value is centred in its half of the box. On the photograph the
//     left box's volts centre 6 px from the centre of its half and its amps
//     15 px from the centre of theirs, on a 929 px box. Both boxes are the
//     same width, so the four values stand at the same two offsets inside
//     their boxes and line up between them.
//   - Nothing pads the lines away from the rims: the mono face's own
//     leading stands 5 px above the capitals at this size against the
//     photograph's 5.4 px scaled to it, and the box is exactly as tall as
//     the two lines and its two rims.
//
// The volts take the volt line's green and the amps the amp line's yellow,
// the two the readings above are lit in, so a value in a box is the same
// colour as the reading it governs. The photograph lights both its own
// values in one yellow-green, #BDE45E over 22982 glyph-core pixels at
// x 420-1260 and x 1380-2200, y 1945-2015 upright; that is the OWON's
// colour for the pair and not this display's, which has a colour per
// quantity. The titles are words rather than values and carry no hue.
func setLimitBoxes(t themed, lines [2]setLimitLine) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		p, typ := t.palette, t.typ
		rim := max(1, gtx.Dp(1))
		h := textdraw.MeasureText(gtx, typ.Shaper, typ.Set, "0").Y
		boxH := 2*h + 2*rim
		room := gtx.Constraints.Max.X
		gap := gtx.Dp(8)
		boxW := (room - gap) / 2
		padH := gtx.Dp(6)
		radius := gtx.Dp(2)

		for i, l := range lines {
			x := 0
			if i == 1 {
				x = room - boxW
			}
			box := image.Rect(x, 0, x+boxW, boxH)
			// The rim is the box's shape with the panel's black cut back
			// inside it, so the edge is exactly rim wide on every side —
			// which a stroke centred on a whole-pixel path is not.
			paint.FillShape(gtx.Ops, p.DisplayRim, clip.UniformRRect(box, radius).Op(gtx.Ops))
			paint.FillShape(gtx.Ops, p.DisplayPanel,
				clip.UniformRRect(box.Inset(rim), max(0, radius-rim)).Op(gtx.Ops))

			in := image.Rect(box.Min.X+rim+padH, box.Min.Y+rim, box.Max.X-rim-padH, box.Max.Y-rim)
			textdraw.FillText(gtx, typ.Shaper, typ.Set,
				image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+h), 0.5, 0.5, p.DisplayCaption, l.Caption)
			half := in.Dx() / 2
			textdraw.FillText(gtx, typ.Shaper, typ.Set,
				image.Rect(in.Min.X, in.Min.Y+h, in.Min.X+half, in.Min.Y+2*h), 0.5, 0.5, p.DisplayVolt, l.Volts)
			textdraw.FillText(gtx, typ.Shaper, typ.Set,
				image.Rect(in.Max.X-half, in.Min.Y+h, in.Max.X, in.Min.Y+2*h), 0.5, 0.5, p.DisplayAmp, l.Amps)
		}
		return layout.Dimensions{Size: image.Pt(room, boxH)}
	}
}

// readoutPanelInset is the unlit margin the panel keeps on every side of its
// segments.
const readoutPanelInset unit.Dp = 12

// readoutPanel is the device's display: the readout lines lit on the
// panel's black. It is the same in both colour schemes because the meter has
// one panel — the window around it is the platform's.
func readoutPanel(t themed, rowGap unit.Dp, rows ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 2*len(rows))
		for i, w := range rows {
			if i > 0 {
				children = append(children, vgap(rowGap))
			}
			children = append(children, layout.Rigid(w))
		}
		// The fill is painted under the lines, so it is recorded first and
		// replayed after the panel's own size is known.
		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(readoutPanelInset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
		lines := macro.Stop()
		paint.FillShape(gtx.Ops, t.palette.DisplayPanel,
			clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(6)).Op(gtx.Ops))
		lines.Add(gtx.Ops)
		return dims
	}
}

// panelRow draws one line of the readout block: digits right-aligned
// against the unit column — the half-size capital unit letter sharing the
// digits' baseline, the optional mode badge left-aligned with it, its top
// on the digits' cap line.
//
// The row spans the width it is given (readoutWidth for the natural
// block).
func panelRow(t themed, foreground color.NRGBA, digits, unit, badgeTxt string, badgeFill color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		typ := t.typ
		h := textdraw.MeasureText(gtx, typ.Shaper, typ.Digits, "0").Y
		w := gtx.Constraints.Max.X
		rect := image.Rectangle{Max: image.Pt(w, h)}
		capTop := int(capTopFrac * float64(h))
		baseline := int(baseFrac * float64(h))

		// The unit letter shares the digits' baseline: its own line box is
		// placed so the two baselines coincide.
		unitSz := textdraw.MeasureText(gtx, typ.Shaper, typ.Unit, unit)
		unitX := w - unitSz.X
		unitTop := baseline - int(baseFrac*float64(unitSz.Y))
		textdraw.FillText(gtx, typ.Shaper, typ.Unit,
			image.Rect(unitX, unitTop, w, unitTop+unitSz.Y), 0.5, 0.5, foreground, unit)

		digitsRight := unitX - gtx.Dp(8)
		textdraw.FillText(gtx, typ.Shaper, typ.Digits,
			image.Rectangle{Max: image.Pt(digitsRight, h)}, 1.0, 0.5, foreground, digits)

		if badgeTxt != "" {
			sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Stack, badgeTxt)
			padX := gtx.Dp(6)
			// The same height as the ON/OFF boxes; the lowercase glyphs
			// just sit lighter inside it.
			bw, bh := sz.X+2*padX, sz.Y+gtx.Dp(1)
			badgeBox(gtx, t, badgeTxt, badgeFill, t.palette.DisplayPanel, true,
				image.Rect(unitX, capTop, unitX+bw, capTop+bh))
		}

		return layout.Dimensions{Size: rect.Max}
	}
}

// tripBadge is the header's protection flag: the latest trip, the badge
// labels' style on the danger colour. Empty while nothing is tripped.
func tripBadge(t themed, r Reading) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if r.Protect == 0 {
			return layout.Dimensions{}
		}
		typ := t.typ
		txt := "PROTECTION TRIPPED: " + ProtectName(r.Protect)
		sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Stack, txt)
		rect := image.Rect(0, 0, sz.X+2*gtx.Dp(8), sz.Y+2*gtx.Dp(2))
		badgeBox(gtx, t, txt, t.palette.Danger, t.palette.FilledText, true, rect)
		return layout.Dimensions{Size: rect.Max}
	}
}

// outputCluster is the amp row's bolt + ON/OFF block reused small: the
// header's output-state display beside the power toggle.
func outputCluster(t themed, r Reading) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		p, typ := t.palette, t.typ
		onSz := textdraw.MeasureText(gtx, typ.Shaper, typ.Stack, "ON")
		offSz := textdraw.MeasureText(gtx, typ.Shaper, typ.Stack, "OFF")
		padX := gtx.Dp(6)
		boxW := max(onSz.X, offSz.X) + 2*padX
		boxH := offSz.Y + gtx.Dp(1)
		gap := gtx.Dp(2)
		totalH := 2*boxH + gap

		icon := t.ic.BoltOff
		if r.On {
			icon = t.ic.BoltOn
		}
		igtx := gtx
		igtx.Constraints = layout.Exact(image.Pt(totalH, totalH))
		icon(igtx)

		stackX := totalH - totalH*7/24 + gtx.Dp(2)
		badgeBox(gtx, t, "ON", p.DisplayAmp, p.DisplayPanel, r.On,
			image.Rect(stackX, 0, stackX+boxW, boxH))
		badgeBox(gtx, t, "OFF", p.SecondaryLabel, p.FilledText, !r.On,
			image.Rect(stackX, boxH+gap, stackX+boxW, totalH))
		return layout.Dimensions{Size: image.Pt(stackX+boxW, totalH)}
	}
}

// badgeBox paints one badge: the label centered in the box — on the state
// colour in the foreground given when active, in the platform's disabled
// control text with no fill when idle. A badge lit in one of the display's
// colours reads its label in the panel's black, as the device's own cut-out
// segments do; a badge on a platform fill reads the platform's foreground.
func badgeBox(gtx layout.Context, t themed, txt string, fill, foreground color.NRGBA, active bool, r image.Rectangle) {
	badgeBoxStyled(gtx, t, t.typ.Stack, txt, fill, foreground, active, r)
}

// badgeBoxStyled is badgeBox with the label's text style chosen.
func badgeBoxStyled(gtx layout.Context, t themed, style textdraw.TextStyle, txt string, fill, foreground color.NRGBA, active bool, r image.Rectangle) {
	if active {
		paint.FillShape(gtx.Ops, fill, clip.UniformRRect(r, gtx.Dp(4)).Op(gtx.Ops))
		textdraw.FillText(gtx, t.typ.Shaper, style, r, 0.5, 0.5, foreground, txt)
		return
	}
	textdraw.FillText(gtx, t.typ.Shaper, style, r, 0.5, 0.5, t.palette.Dim, txt)
}
