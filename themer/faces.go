// The code-face chooser: which monospace typeface the sample's fence wears.
// Two names, not a font picker — Roboto Mono and JetBrains Mono, exactly
// those, sitting above the syntax bases beside the sample they restyle.
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

// The face plate's dimensions. The plate is the base chooser's width, so the
// two read as one stack beside the sample.
const (
	// FaceHead is one line. A two-name plate does not need the invite the
	// seventy-odd-base list carries; repeating "Click one to apply" a thumb
	// above that list reads as leftover chrome.
	FaceHead unit.Dp = 20
	// FacePanelH is the whole plate: heading, two names, padding.
	FacePanelH = FaceHead + 2*BaseRow + 2*BasePad
	// FaceGap is the air between this plate and the base chooser under it.
	FaceGap unit.Dp = 8
)

// FaceLabel heads the plate.
const FaceLabel = "Code face"

// codeFaces is the two names the plate offers, in the order they are drawn.
// Nothing else is choosable here.
var codeFaces = []string{tokens.CodeFaceRoboto, tokens.CodeFaceJetBrains}

// faceChooser is what the plate keeps across emissions: one click handler per
// name. Two names, allocated once.
type faceChooser struct {
	clicks [2]gesture.Click
}

func newFaceChooser() *faceChooser { return &faceChooser{} }

// FacePlate draws the two-name plate: Roboto Mono and JetBrains Mono, the
// chosen one marked the way a base row is.
func FacePlate(p Palette, ty Type, m Model, faces *faceChooser) layout.Widget {
	if faces == nil {
		faces = newFaceChooser()
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
			textdraw.FillText(gtx, ty.Shaper, ty.Label, image.Rect(0, 0, w, headH), 0, 0.5, p.CardText, FaceLabel)

			rowH := gtx.Dp(BaseRow)
			for i, face := range codeFaces {
				at(gtx, image.Pt(0, headH+i*rowH), func(gtx layout.Context) {
					gtx.Constraints = layout.Exact(image.Pt(w, rowH))
					FaceRow(gtx, p, ty, face, face == chosen, &faces.clicks[i])
				})
			}
			return layout.Dimensions{Size: size}
		})
		strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline), p.Edge)
		return layout.Dimensions{Size: size}
	}
}

// FaceRow draws one of the two names and makes it clickable.
func FaceRow(gtx layout.Context, p Palette, ty Type, name string, chosen bool, click *gesture.Click) layout.Dimensions {
	dims := ChoiceRow(gtx, p, ty, name, "", chosen, click)
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: SelectMono{Name: name}}.Add(gtx.Ops)
		}
	}
	return dims
}
