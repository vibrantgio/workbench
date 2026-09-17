// The syntax base group's box: the column of palette names and, beside it,
// the fence they colour.
//
// The names sit beside the thing they change, because a name chosen from
// behind the thing it changes is chosen blind. The sample is drawn in the
// appearance the scheme switch on the group's title row is showing, through
// that appearance's own member of the pair, so flipping the switch swaps the
// list, the marked row and the colours under the code together.
package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/tokens"
)

// CodeSource is the sample both choices are judged on: a line of prose with a
// word of code quoted into it, and under it a Go block carrying a comment, a
// keyword run, a string and a number — a syntax palette that cannot be told
// from another on those four cannot be told from it at all.
//
// The prose is there because a fence is judged against the page it is an
// island in: a block of colour on a plate of its own says nothing about how
// it reads under a paragraph. Two lines of code and not eight because the
// whole sample has to stand inside the group's box at the height the column
// of names wants, and a block laid out taller than its plate is a block with
// its last line cut through.
const CodeSource = "A code block wears the syntax base, and `Greet` is inline code in a line of prose.\n\n" +
	"```go\n" +
	"// Greet names whoever is asking.\n" +
	"func Greet(name string) string { return fmt.Sprintf(\"Hello, %s! (%d)\", name, 42) }\n" +
	"```\n"

// codeState is what the group keeps across emissions: the two choosers'
// handlers and scroll position, and the parsed sample.
//
// The sample is parsed once and restyled per emission. Colour is no part of
// the parse — the document takes its style at layout time — so choosing
// another base re-dresses a document already read rather than reading one
// again.
type codeState struct {
	bases *baseChooser
	faces *faceChooser
	doc   *markdown.Document
}

func newCodeState() *codeState {
	return &codeState{
		bases: newBaseChooser(),
		faces: newFaceChooser(),
		doc:   markdown.NewDocument(markdown.Parse([]byte(CodeSource))),
	}
}

// BaseChoices lays the syntax base group's box out: the column of names at
// the leading edge and the sample beside it.
//
// c is the set being previewed under the appearance on screen — the
// platform's with the theme colour standing in for the accent — because the
// sample is a preview like the ones under it. The column is the window's own
// chrome and takes the live set's names through p and live.
func BaseChoices(p Palette, live, c tokens.PlatformColors, ty Type, typ tokens.Typography, shaper *text.Shaper, m Model, dark bool, code *codeState) layout.Widget {
	if code == nil {
		code = newCodeState()
	}
	panel := BasePanel(p, live, ty, m, dark, code.bases)
	st := CodeStyle(c, typ, m.AppliedBases())
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		col := min(gtx.Dp(BaseW), size.X)
		at(gtx, image.Point{}, func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(image.Pt(col, size.Y))
			panel(gtx)
		})

		plate := image.Rect(col+gtx.Dp(Gap), 0, size.X, size.Y)
		if plate.Dx() <= 0 {
			return layout.Dimensions{Size: size}
		}
		fillRRect(gtx, plate, gtx.Dp(InnerR), c.WindowBackground)
		// A line round the plate, because the plate is a second surface
		// inside the group's box and the two met with nothing between them.
		strokeRRect(gtx, plate, gtx.Dp(InnerR), gtx.Dp(Hairline), p.Edge)
		inner := plate.Inset(gtx.Dp(BasePad))
		if inner.Dx() <= 0 || inner.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}
		// Clipped to the plate: the sample is a fixed document laid out in
		// whatever height the box has, and a fence running past the plate's
		// edge would read as a plate that failed to draw.
		defer clip.Rect(inner).Push(gtx.Ops).Pop()
		// Recorded, measured and then placed, so the air the document does
		// not use is shared between its two ends rather than all left under
		// it. A plate with its content against the top and a hand's width of
		// nothing below reads as a plate that stopped drawing.
		macro := op.Record(gtx.Ops)
		doc := gtx
		doc.Constraints = layout.Constraints{Min: image.Pt(inner.Dx(), 0), Max: inner.Size()}
		dims := code.doc.LayoutColumn(doc, shaper, st)
		call := macro.Stop()
		origin := image.Pt(inner.Min.X, inner.Min.Y+max(0, (inner.Dy()-dims.Size.Y)/2))
		stack := op.Offset(origin).Push(gtx.Ops)
		call.Add(gtx.Ops)
		stack.Pop()
		return layout.Dimensions{Size: size}
	}
}

// CodeStyle is the document style the sample is drawn through: the previewed
// set's own reading colours, the chosen code face, and a fence wearing the
// chosen base.
//
// The base is worn rather than re-fitted. A fence takes the background the
// style was written against and its colours as they were written, so the
// block is the palette itself and not a rendering of it; everything round it
// stays the platform's. Which member of the pair reaches the fence follows
// the set, so a change of appearance is a change of plate.
func CodeStyle(c tokens.PlatformColors, typ tokens.Typography, bases highlight.BasePair) markdown.Style {
	st := markdown.FromTokens(c, typ, c.WindowBackground)
	highlight.WearPair(&st, bases, c)
	return st
}
