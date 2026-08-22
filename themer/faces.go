// The code-face selector: which monospace typeface the specimen's fence
// wears. Two names, not a font picker — Roboto Mono and JetBrains Mono,
// exactly those, sitting beside the specimen the way the base selector
// already does.
package main

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// The face selector's dimensions. The column is the base selector's
// width, so the two plates read as one stack beside the specimen.
const (
	// FaceHead is one line. A two-name plate does not need the invite
	// the seventy-odd-base list carries; repeating "Click one to apply"
	// a thumb above that list reads as leftover chrome.
	FaceHead unit.Dp = 20
	// FacePanelH is the whole plate: heading, two names, padding.
	FacePanelH         = FaceHead + 2*BaseRow + 2*BasePad
	FaceGap    unit.Dp = 8 // between this plate and the base selector
)

// What heads the plate.
const FaceLabel = "Code face"

// codeFaces is the two names the plate offers, in the order they are
// drawn. Nothing else is choosable here.
var codeFaces = []string{tokens.CodeFaceRoboto, tokens.CodeFaceJetBrains}

// faceSelector is what the plate keeps across emissions: one click
// handler per name. Two names, allocated once.
type faceSelector struct {
	clicks [2]gesture.Click
}

func newFaceSelector() *faceSelector { return &faceSelector{} }

// FacePanel draws the two-name plate: Roboto Mono and JetBrains Mono,
// the chosen one marked the way a base row is.
func FacePanel(p Palette, ty Type, m Model, faces *faceSelector) layout.Widget {
	if faces == nil {
		faces = newFaceSelector()
	}
	chosen := m.AppliedMono()
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		panel := image.Rectangle{Max: size}
		fillRRect(gtx, panel, gtx.Dp(Radius), p.Surface)
		defer clip.UniformRRect(panel, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(size)
		layout.UniformInset(BasePad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			w, headH := gtx.Constraints.Max.X, gtx.Dp(FaceHead)
			textdraw.FillText(gtx, ty.Shaper, ty.Label, image.Rect(0, 0, w, headH), 0, 0.5, p.Text, FaceLabel)

			rowH := gtx.Dp(BaseRow)
			for i, face := range codeFaces {
				i, face := i, face
				at(gtx, image.Pt(0, headH+i*rowH), func(gtx layout.Context) {
					gtx.Constraints = layout.Exact(image.Pt(w, rowH))
					FaceRowWidget(gtx, p, ty, face, face == chosen, &faces.clicks[i])
				})
			}
			return layout.Dimensions{Size: size}
		})
		strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline), p.Divider)
		return layout.Dimensions{Size: size}
	}
}

// FaceRowWidget draws one of the two names and makes it clickable.
func FaceRowWidget(gtx layout.Context, p Palette, ty Type, name string, chosen bool, click *gesture.Click) layout.Dimensions {
	h := gtx.Dp(BaseRow)
	size := image.Pt(gtx.Constraints.Max.X, h)
	r := image.Rectangle{Max: size}
	switch {
	case chosen:
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Selection)
	case click.Hovered():
		fillRRect(gtx, r, gtx.Dp(InnerR), p.Divider)
	}
	pad := gtx.Dp(BasePad)
	if chosen {
		mark := image.Rect(0, h/8, gtx.Dp(BaseInk), h-h/8)
		fillRRect(gtx, mark, gtx.Dp(BaseInk)/2, p.Accent)
	}
	tone := p.Muted
	if chosen {
		tone = p.Text
	}
	text := image.Rect(pad+gtx.Dp(BaseInk), 0, size.X-pad, h)
	textdraw.FillText(gtx, ty.Shaper, ty.Small, text, 0, 0.5, tone, name)

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: SelectMono{Name: name}}.Add(gtx.Ops)
		}
	}
	return layout.Dimensions{Size: size}
}
