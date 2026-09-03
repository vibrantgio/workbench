package main

// The readout block, styled after the SK150's own display. Each line is
// big mono digits with a half-size capital unit letter baseline-aligned in
// a right column; while the output is regulating, the line of the quantity
// in control wears the mode badge above its unit letter — CV on the volt
// line in the volt ink, CC on the amp line in the amp ink. Output state
// lives in the header's bolt + ON/OFF cluster beside the power toggle, not
// on the readout block.
//
// Everything aligns with the digits' INK, not their line box: the box
// carries the font's leading, and furniture hung off its edges floats
// visibly too far from the glyphs.

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// boltDp is the construction size of the output bolt; the header's output
// cluster hands the widget exact constraints at draw time, so this is only
// its default.
const boltDp unit.Dp = 26

// Roboto Mono's vertical metrics as fractions of the line box: where the
// digit glyphs' top (ascent minus cap height) and the baseline sit. The
// digits and unit letters are caps and numerals, so ink runs from capTop to
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
	return panelRow(t, t.palette.Volt, fmt.Sprintf("%05.2f", r.VOut), "V", badgeTxt, t.palette.Volt)
}

// presetBadge is the active-preset pill ("M2"): the unit letters' style
// knocked out of the volt ink, on the line above the readouts.
func presetBadge(t themed, active int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !isGroup(active) {
			return layout.Dimensions{}
		}
		typ := t.typ
		txt := fmt.Sprintf("M%d", active)
		sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Unit, txt)
		r := image.Rect(0, 0, sz.X+2*gtx.Dp(8), sz.Y+2*gtx.Dp(2))
		badgeBoxStyled(gtx, t, typ.Unit, txt, t.palette.Volt, true, r)
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
	return panelRow(t, t.palette.Amp, fmt.Sprintf("%5.3f", r.IOut), "A", badgeTxt, t.palette.Amp)
}

// wattRow is the power line, three decimals like the device.
func wattRow(t themed, r Reading) layout.Widget {
	return panelRow(t, t.palette.Watt, fmt.Sprintf("%.3f", r.Power), "W", "", color.NRGBA{})
}

// panelRow draws one line of the readout block: digits right-aligned
// against the unit column — the half-size capital unit letter sharing the
// digits' baseline, the optional mode badge left-aligned with it, its top
// on the digits' cap line.
//
// The row spans the width it is given (readoutWidth for the natural
// block).
func panelRow(t themed, ink color.NRGBA, digits, unit, badgeTxt string, badgeFill color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		typ := t.typ
		h := textdraw.MeasureText(gtx, typ.Shaper, typ.Digits, "0").Y
		w := gtx.Constraints.Max.X
		rect := image.Rectangle{Max: image.Pt(w, h)}
		inkTop := int(capTopFrac * float64(h))
		baseline := int(baseFrac * float64(h))

		// The unit letter shares the digits' baseline: its own line box is
		// placed so the two baselines coincide.
		unitSz := textdraw.MeasureText(gtx, typ.Shaper, typ.Unit, unit)
		unitX := w - unitSz.X
		unitTop := baseline - int(baseFrac*float64(unitSz.Y))
		textdraw.FillText(gtx, typ.Shaper, typ.Unit,
			image.Rect(unitX, unitTop, w, unitTop+unitSz.Y), 0.5, 0.5, ink, unit)

		digitsRight := unitX - gtx.Dp(8)
		textdraw.FillText(gtx, typ.Shaper, typ.Digits,
			image.Rectangle{Max: image.Pt(digitsRight, h)}, 1.0, 0.5, ink, digits)

		if badgeTxt != "" {
			sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Stack, badgeTxt)
			padX := gtx.Dp(6)
			// The same height as the ON/OFF boxes; the lowercase glyphs
			// just sit lighter inside it.
			bw, bh := sz.X+2*padX, sz.Y+gtx.Dp(1)
			badgeBox(gtx, t, badgeTxt, badgeFill, true,
				image.Rect(unitX, inkTop, unitX+bw, inkTop+bh))
		}

		return layout.Dimensions{Size: rect.Max}
	}
}

// tripBadge is the header's protection flag: the latest trip, the badge
// labels' style knocked out of the danger ink — the backdrop-coloured text
// reads white-on-red in a light scheme. Empty while nothing is tripped.
func tripBadge(t themed, r Reading) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if r.Protect == 0 {
			return layout.Dimensions{}
		}
		typ := t.typ
		txt := "PROTECTION TRIPPED: " + ProtectName(r.Protect)
		sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Stack, txt)
		rect := image.Rect(0, 0, sz.X+2*gtx.Dp(8), sz.Y+2*gtx.Dp(2))
		badgeBox(gtx, t, txt, t.palette.Danger, true, rect)
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
		badgeBox(gtx, t, "ON", p.Volt, r.On,
			image.Rect(stackX, 0, stackX+boxW, boxH))
		badgeBox(gtx, t, "OFF", p.Label, !r.On,
			image.Rect(stackX, boxH+gap, stackX+boxW, totalH))
		return layout.Dimensions{Size: image.Pt(stackX+boxW, totalH)}
	}
}

// badgeBox paints one badge: the label centered in the box — knocked out of
// the state ink when active, set in disabled ink with no fill when idle.
func badgeBox(gtx layout.Context, t themed, txt string, fill color.NRGBA, active bool, r image.Rectangle) {
	badgeBoxStyled(gtx, t, t.typ.Stack, txt, fill, active, r)
}

// badgeBoxStyled is badgeBox with the label's text style chosen.
func badgeBoxStyled(gtx layout.Context, t themed, style textdraw.TextStyle, txt string, fill color.NRGBA, active bool, r image.Rectangle) {
	if active {
		paint.FillShape(gtx.Ops, fill, clip.UniformRRect(r, gtx.Dp(4)).Op(gtx.Ops))
		textdraw.FillText(gtx, t.typ.Shaper, style, r, 0.5, 0.5, t.palette.Backdrop, txt)
		return
	}
	textdraw.FillText(gtx, t.typ.Shaper, style, r, 0.5, 0.5, tokens.Disabled(t.palette.Label), txt)
}
