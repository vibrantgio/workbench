// The code-face chooser: which monospace typeface the sample's fence wears.
// Two names, not a font picker — Roboto Mono and JetBrains Mono, exactly
// those, standing side by side in the group that is titled after them.
package main

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/tokens"
)

// FaceChipW is one name's width. It is a fixed width rather than a share of
// the row because the two names are a pair of alternatives and not a table:
// the longer name sets the size and the shorter one takes it, so neither reads
// as the more important of the two.
const FaceChipW unit.Dp = 150

// FaceLabel heads the group.
const FaceLabel = "Code face"

// codeFaces is the two names on offer, in the order they are drawn. Nothing
// else is choosable here.
var codeFaces = []string{tokens.CodeFaceRoboto, tokens.CodeFaceJetBrains}

// faceChooser is what the pair keeps across emissions: one click handler per
// name. Two names, allocated once.
type faceChooser struct {
	clicks [2]gesture.Click
}

func newFaceChooser() *faceChooser { return &faceChooser{} }

// FaceChoices draws the two names side by side in the group's box, the chosen
// one marked the way a base row is.
func FaceChoices(p Palette, ty Type, m Model, faces *faceChooser) layout.Widget {
	if faces == nil {
		faces = newFaceChooser()
	}
	chosen := m.AppliedMono()
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		w, gap := gtx.Dp(FaceChipW), gtx.Dp(CellGap)
		for i, face := range codeFaces {
			x := i * (w + gap)
			if x+w > size.X {
				break
			}
			at(gtx, image.Pt(x, 0), func(gtx layout.Context) {
				gtx.Constraints = layout.Exact(image.Pt(w, size.Y))
				FaceRow(gtx, p, ty, face, face == chosen, &faces.clicks[i])
			})
		}
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
