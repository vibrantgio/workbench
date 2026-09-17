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

// readoutPanelInset is the unlit margin the panel keeps on every side of its
// segments.
const readoutPanelInset unit.Dp = 12

// readoutPanel is the device's display: the three readout lines lit on the
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
