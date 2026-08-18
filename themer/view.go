package main

import (
	"fmt"
	"image"
	stdcolor "image/color"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Layout dimensions. None of them varies with the colour scheme.
const (
	Pad      unit.Dp = 20 // window margin
	Gap      unit.Dp = 14 // between the page's stacked parts
	Radius   unit.Dp = 12 // the picture mat and the candidate cards
	Hairline unit.Dp = 1  // the mat's resting outline
	Ring     unit.Dp = 2  // the chosen candidate's ring, and the drag highlight

	HeaderH   unit.Dp = 24 // file name and the replace hint, on one line
	RowLabelH unit.Dp = 20 // the label over the candidate row
)

// dropZone is the one zone the window registers: the whole of it. The
// application has no second drop target, so the index is a constant.
const dropZone = 0

// buildLayers returns the layer-builder the theme window renders.
func buildLayers(modelObs rx.Observable[Model], zones *desktop.ZoneGroup) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th, modelObs),
			ContentLayer(th, modelObs, zones),
		}
	}
}

// themed carries one emission's OS palette and typography. The palette the
// window actually draws in is resolved from this one and the model together,
// in SchemeFor, because the chosen candidate re-seeds it.
type themed struct {
	os  tokens.ColorTokens
	typ Type
}

// BackdropLayer fills the window. It follows the model as well as the OS,
// because the background is one of the surfaces a chosen seed changes.
func BackdropLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(rx.CombineLatest2(colors, modelObs),
		func(n rx.Tuple2[tokens.ColorTokens, Model]) layout.Widget {
			return backdrop.Widget(SchemeFor(n.First, n.Second).Background)
		})
}

// ContentLayer renders the page: the dropped picture over the candidate row,
// or the invitation to drop one.
//
// The click handlers live at subscription scope, OUTSIDE the per-emission
// Map (llms.txt rule 2): a gesture handler reconstructed every emission
// loses the press it is in the middle of, and every selection re-emits.
// There is one per candidate slot, not per candidate, so the handlers
// outlive a picture being replaced by another.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], zones *desktop.ZoneGroup) rx.Observable[layout.Widget] {
	clicks := make([]gesture.Click, imageseed.DefaultMax)
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography),
			func(n rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themed {
				return themed{os: n.First, typ: TypeFrom(n.Second)}
			})
	})
	return rx.Map(rx.CombineLatest2(themes, modelObs),
		func(n rx.Tuple2[themed, Model]) layout.Widget {
			return Page(n.First, n.Second, zones, clicks)
		})
}

// Page lays the window out and registers it, whole, as the drop zone.
func Page(t themed, m Model, zones *desktop.ZoneGroup, clicks []gesture.Click) layout.Widget {
	scheme := SchemeFor(t.os, m)
	p := PaletteFrom(scheme)
	// Each candidate's generated primary pair, on the side the window is
	// currently showing, so a swatch promises what choosing it delivers.
	pairs := make([]tokens.ColorTokens, len(m.Candidates))
	for i, c := range m.Candidates {
		light, dark := tokens.FromSeed(c.Color)
		pairs[i] = light
		if isDark(scheme) {
			pairs[i] = dark
		}
	}
	var picture paint.ImageOp
	if m.Preview != nil {
		picture = paint.NewImageOp(m.Preview)
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		// The whole window is the target, recorded once per frame before
		// anything is laid out inside it.
		zones.Update(gtx)
		zones.Record(dropZone, image.Rectangle{Max: size})

		inset := layout.UniformInset(Pad)
		inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{}
			if len(m.Candidates) > 0 {
				children = append(children,
					rigid(Header(p, t.typ, m)),
					spacer(Gap),
					layout.Flexed(1, Picture(p, m, picture)),
					spacer(Gap),
					rigid(CandidateRow(p, t.typ, m, pairs, clicks)),
				)
			} else {
				children = append(children, layout.Flexed(1, Invitation(p, t.typ, m)))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})

		// The drag highlight rings the window itself, not a panel inside
		// it, because the window itself is what accepts the drop.
		if m.DragOver {
			strokeRRect(gtx, image.Rectangle{Max: size}, gtx.Dp(Radius+Pad/2), gtx.Dp(Ring+1), p.Accent)
		}
		return layout.Dimensions{Size: size}
	}
}

// Header is the loaded picture's line: its name, and the standing offer to
// replace it.
func Header(p Palette, ty Type, m Model) layout.Widget {
	hint := "Drop another image to replace it"
	tone := p.Muted
	if m.Problem != "" {
		hint, tone = m.Problem, p.Problem
	}
	return func(gtx layout.Context) layout.Dimensions {
		r := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, gtx.Dp(HeaderH))}
		textdraw.FillText(gtx, ty.Shaper, ty.Body, r, 0, 0.5, p.Text, m.Name)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, r, 1, 0.5, tone, hint)
		return layout.Dimensions{Size: r.Max}
	}
}

// Picture draws the dropped image on a mat, scaled to fit and centred, so
// the candidates below it can be compared against what they came from.
func Picture(p Palette, m Model, src paint.ImageOp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		r := image.Rectangle{Max: size}
		fill, edge := p.Surface, p.Divider
		if m.DragOver {
			fill, edge = p.Selection, p.Accent
		}
		fillRRect(gtx, r, gtx.Dp(Radius), fill)
		strokeRRect(gtx, r, gtx.Dp(Radius), gtx.Dp(Hairline), edge)
		if m.Preview == nil {
			return layout.Dimensions{Size: size}
		}
		defer clip.UniformRRect(r, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
		return layout.UniformInset(Gap).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return widget.Image{
					Src:      src,
					Fit:      widget.Contain,
					Position: layout.Center,
					Scale:    1 / gtx.Metric.PxPerDp,
				}.Layout(gtx)
			})
		})
	}
}

// Invitation is the window before anything has been dropped on it: a well
// covering the page, saying what to drop and where. "Where" is the whole
// window, and saying so is the only way a target with no edges of its own
// can be discovered.
func Invitation(p Palette, ty Type, m Model) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		r := image.Rectangle{Max: size}
		fill, edge, width := p.Surface, p.Divider, gtx.Dp(Hairline)
		if m.DragOver {
			fill, edge, width = p.Selection, p.Accent, gtx.Dp(Ring)
		}
		fillRRect(gtx, r, gtx.Dp(Radius), fill)
		strokeRRect(gtx, r, gtx.Dp(Radius), width, edge)

		line := gtx.Dp(28)
		mid := size.Y / 2
		title := image.Rect(0, mid-line*3/2, size.X, mid-line/2)
		sub := image.Rect(0, mid-line/2, size.X, mid+line/2)
		note := image.Rect(0, mid+line/2, size.X, mid+line*3/2)
		textdraw.FillText(gtx, ty.Shaper, ty.Title, title, 0.5, 0.5, p.Text, "Drop an image here")
		textdraw.FillText(gtx, ty.Shaper, ty.Body, sub, 0.5, 0.5, p.Muted, "Anywhere on the window. PNG, JPEG or GIF.")
		if m.Problem != "" {
			textdraw.FillText(gtx, ty.Shaper, ty.Small, note, 0.5, 0.5, p.Problem, m.Problem)
		}
		return layout.Dimensions{Size: size}
	}
}

// rigid wraps a widget as a Flex child that takes the height it asks for.
func rigid(w layout.Widget) layout.FlexChild { return layout.Rigid(w) }

// spacer is a fixed vertical gap between two Flex children.
func spacer(h unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Height: h}.Layout)
}

// fillRRect paints a rounded rectangle.
func fillRRect(gtx layout.Context, r image.Rectangle, radius int, c stdcolor.NRGBA) {
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// strokeRRect outlines a rounded rectangle, inset by half the stroke width
// so the whole line lands inside the rectangle rather than half outside it.
func strokeRRect(gtx layout.Context, r image.Rectangle, radius, width int, c stdcolor.NRGBA) {
	if width <= 0 {
		return
	}
	half := float32(width) / 2
	inner := image.Rect(r.Min.X+width/2, r.Min.Y+width/2, r.Max.X-width/2, r.Max.Y-width/2)
	path := clip.UniformRRect(inner, max(0, radius-width/2)).Path(gtx.Ops)
	defer clip.Stroke{Path: path, Width: half * 2}.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// hexOf writes a colour the way a stylesheet would.
func hexOf(c stdcolor.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// at offsets the operations w records to origin, leaving the caller's
// coordinate system untouched.
func at(gtx layout.Context, origin image.Point, w func(gtx layout.Context)) {
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	w(gtx)
}
