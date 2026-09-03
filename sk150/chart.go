package main

// The monitor's history strip: output voltage and output current over time, as two
// stacked panels sharing the time axis — deliberately NOT one dual-axis
// chart (two y-scales on one plot is the classic chart defect; two measures
// of different scale get two panels). One series per panel, so the panel
// title carries identity and no legend is needed; the series ink is the
// same theme-derived colour the monitor readout wears, axis text stays in
// text tokens, and the grid is a recessive hairline.

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/textdraw"
)

// hoverState is the crosshair's cross-frame state. It lives at subscription
// scope in ContentLayer and is read and written only during layout (the one
// frame goroutine), llms.txt rule 2. It doubles as the pointer event tag,
// shared by both panels so the crosshair tracks in sync.
type hoverState struct {
	active bool
	frac   float64 // cursor position as a fraction of the plot width
}

// chartPanels is the monitor's history strip under the readout block:
// the two stacked panels sharing the height they are given, or a waiting
// line until there is a history to draw.
func chartPanels(t themed, m Model, hov *hoverState) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if len(m.History) < 2 {
			return textLine(t.typ, t.typ.Body, t.palette.Label, "collecting samples…")(gtx)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Text, "Output voltage (V)")),
			vgap(6),
			layout.Flexed(1, chartPanel(t, m.History,
				func(s Sample) float64 { return s.V }, t.palette.Volt, "%.2f V", 0.05, hov)),
			vgap(12),
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Text, "Output current (A)")),
			vgap(6),
			layout.Flexed(1, chartPanel(t, m.History,
				func(s Sample) float64 { return s.I }, t.palette.Amp, "%.3f A", 0.02, hov)),
		)
	}
}

// chartPanel draws one series over the shared time window: a raised panel
// with a hairline edge, three recessive gridlines, the 2 dp series stroke,
// min/max labels in text ink, and the synced hover crosshair.
func chartPanel(t themed, samples []Sample, sel func(Sample) float64,
	ink color.NRGBA, valFmt string, minSpan float64, hov *hoverState) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		p, typ := t.palette, t.typ
		size := gtx.Constraints.Max
		pad := gtx.Dp(10)
		plot := image.Rect(pad, pad, size.X-pad, size.Y-pad)
		if plot.Dx() < 10 || plot.Dy() < 10 {
			return layout.Dimensions{Size: size}
		}

		// This frame's pointer events, before drawing, so the crosshair
		// tracks without a frame of lag.
		for {
			e, ok := gtx.Event(pointer.Filter{
				Target: hov,
				Kinds:  pointer.Move | pointer.Drag | pointer.Enter | pointer.Leave | pointer.Cancel,
			})
			if !ok {
				break
			}
			pe, isPtr := e.(pointer.Event)
			if !isPtr {
				continue
			}
			switch pe.Kind {
			case pointer.Leave, pointer.Cancel:
				hov.active = false
			default:
				hov.active = true
				hov.frac = clampf(float64(pe.Position.X-float32(plot.Min.X))/float64(plot.Dx()), 0, 1)
			}
		}

		// The panel surface and its edge.
		rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(6))
		paint.FillShape(gtx.Ops, p.Panel, rr.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, p.Hairline,
			clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())

		// The pointer area for the crosshair.
		area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
		event.Op(gtx.Ops, hov)
		area.Pop()

		// Recessive grid: three horizontal hairlines.
		for i := 1; i < 4; i++ {
			y := plot.Min.Y + i*plot.Dy()/4
			paint.FillShape(gtx.Ops, p.Hairline,
				clip.Rect(image.Rect(plot.Min.X, y, plot.Max.X, y+1)).Op())
		}

		// Scales. X is time from the oldest to the newest sample; Y is the
		// padded data range, floored at zero (both measures are physical
		// non-negatives) and held open to minSpan so a flat line sits
		// mid-panel instead of hugging an edge.
		lo, hi := rangeOf(samples, sel, minSpan)
		t0 := samples[0].At
		span := samples[len(samples)-1].At.Sub(t0)
		if span <= 0 {
			span = time.Millisecond
		}
		xAt := func(at time.Time) float32 {
			return float32(plot.Min.X) + float32(at.Sub(t0))/float32(span)*float32(plot.Dx())
		}
		yAt := func(v float64) float32 {
			return float32(plot.Max.Y) - float32((v-lo)/(hi-lo))*float32(plot.Dy())
		}

		// The series, strided down to at most one point per pixel column.
		step := 1
		if n := len(samples); n > plot.Dx() {
			step = (n + plot.Dx() - 1) / plot.Dx()
		}
		var path clip.Path
		path.Begin(gtx.Ops)
		started := false
		for i := 0; i < len(samples); i += step {
			s := samples[i]
			pt := f32.Pt(xAt(s.At), yAt(sel(s)))
			if !started {
				path.MoveTo(pt)
				started = true
			} else {
				path.LineTo(pt)
			}
		}
		last := samples[len(samples)-1]
		path.LineTo(f32.Pt(xAt(last.At), yAt(sel(last))))
		paint.FillShape(gtx.Ops, ink,
			clip.Stroke{Path: path.End(), Width: float32(gtx.Dp(2))}.Op())

		// Scale labels in text tokens, tucked inside the plot corners.
		inset := gtx.Dp(4)
		label := func(txt string, ax, ay float64, r image.Rectangle) {
			sz := textdraw.MeasureText(gtx, typ.Shaper, typ.Small, txt)
			x := r.Min.X + int(ax*float64(r.Dx()-sz.X))
			y := r.Min.Y + int(ay*float64(r.Dy()-sz.Y))
			rect := image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x+sz.X, y+sz.Y)}
			textdraw.FillText(gtx, typ.Shaper, typ.Small, rect, 0, 0.5, p.Label, txt)
		}
		in := plot.Inset(inset)
		label(fmt.Sprintf(valFmt, hi), 0, 0, in)
		label(fmt.Sprintf(valFmt, lo), 0, 1, in)
		label("-"+fmtAge(span), 0.35, 1, in)
		label("now", 1, 1, in)

		// The crosshair: a hairline at the cursor with the nearest
		// sample's value and age in the top-right corner.
		if hov.active {
			cx := plot.Min.X + int(hov.frac*float64(plot.Dx()))
			paint.FillShape(gtx.Ops, p.Label,
				clip.Rect(image.Rect(cx, plot.Min.Y, cx+1, plot.Max.Y)).Op())
			s := nearestSample(samples, t0.Add(time.Duration(hov.frac*float64(span))))
			txt := fmt.Sprintf(valFmt, sel(s)) + " · -" + fmtAge(last.At.Sub(s.At))
			label(txt, 1, 0, in)
		}

		return layout.Dimensions{Size: size}
	}
}

// rangeOf returns the padded [lo, hi] display range of one series: 10%
// headroom, at least minSpan wide, floored at zero.
func rangeOf(samples []Sample, sel func(Sample) float64, minSpan float64) (lo, hi float64) {
	lo, hi = sel(samples[0]), sel(samples[0])
	for _, s := range samples[1:] {
		v := sel(s)
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	pad := (hi - lo) * 0.1
	if pad < minSpan/2 {
		pad = minSpan / 2
	}
	lo -= pad
	hi += pad
	if lo < 0 {
		lo = 0
	}
	return lo, hi
}

// nearestSample returns the sample closest in time to at; samples are
// oldest-first.
func nearestSample(samples []Sample, at time.Time) Sample {
	best := samples[0]
	bestD := absDur(best.At.Sub(at))
	for _, s := range samples[1:] {
		if d := absDur(s.At.Sub(at)); d < bestD {
			best, bestD = s, d
		}
	}
	return best
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// fmtAge renders a duration the way an instrument panel would: 42s, 3m07s,
// 1h04m.
func fmtAge(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
