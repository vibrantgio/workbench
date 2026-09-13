// The code section: the two choices this window makes about code, and the
// fence they land on.
//
// The choices sit beside the thing they change, because a name chosen from
// behind the thing it changes is chosen blind: the face plate and the base
// chooser stand in one column at the leading edge, and the sample takes the
// rest of the row. The sample is drawn in the appearance the scheme switch is
// showing, through that appearance's own member of the pair, so flipping the
// switch swaps the list, the marked row and the colours under the code
// together.
package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// CodeH is the whole section: its label row and the plates under it. The
// column's own two plates are cut from what is left, the face plate keeping
// its fixed height and the base chooser taking the rest, which is what makes
// the list as long as the window can show.
const CodeH unit.Dp = 320

// CodeLabel heads the section.
const CodeLabel = "Code"

// CodeSource is the sample both choices are judged on: a line of prose with a
// word of code quoted into it, and under it one short Go block carrying a
// comment, a keyword run, a string and a number — a syntax palette that
// cannot be told from another on those four cannot be told from it at all.
//
// The prose is there because a fence is judged against the page it is an
// island in: a block of colour on a plate of its own says nothing about how
// it reads under a paragraph.
const CodeSource = "Fenced code wears the syntax base; `Greet` is a word of it quoted into a line.\n\n" +
	"```go\n" +
	"// Greet names whoever is asking.\n" +
	"func Greet(name string) string {\n" +
	"\tif name == \"\" {\n" +
	"\t\tname = \"world\"\n" +
	"\t}\n" +
	"\treturn fmt.Sprintf(\"Hello, %s! (%d)\", name, 42)\n" +
	"}\n" +
	"```\n"

// codeState is what the section keeps across emissions: the two choosers'
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

// CodeHintFor is what the section's label row says at its trailing end: the
// base the code on screen is coloured from, and the face it is set in. Both
// are the applied values rather than the kept ones, because what is on screen
// is what is being judged.
func CodeHintFor(m Model, dark bool) string {
	return m.Base(dark) + " · " + m.AppliedMono()
}

// CodeSection lays the section out: the label row, the column of two
// choosers, and the sample beside them.
//
// c is the set being previewed — the platform's with the theme colour
// standing in for the accent — because the sample is a preview like the one
// under it. The choosers are the window's own chrome and take the live set's
// names through p.
func CodeSection(p Palette, live, c tokens.PlatformColors, ty Type, typ tokens.Typography, shaper *text.Shaper, m Model, dark bool, code *codeState) layout.Widget {
	if code == nil {
		code = newCodeState()
	}
	face := FacePlate(p, ty, m, code.faces)
	panel := BasePanel(p, live, ty, m, dark, code.bases)
	st := CodeStyle(c, typ, m.AppliedBases())
	hint := CodeHintFor(m, dark)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, size.X, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, CodeLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, hint)

		body := image.Rect(0, labelH, size.X, size.Y)
		if body.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}
		col := min(gtx.Dp(BaseW), body.Dx())
		faceH := min(gtx.Dp(FacePanelH), body.Dy())
		at(gtx, body.Min, func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(image.Pt(col, faceH))
			face(gtx)
		})
		if rest := body.Dy() - faceH - gtx.Dp(FaceGap); rest > 0 {
			at(gtx, image.Pt(body.Min.X, body.Min.Y+faceH+gtx.Dp(FaceGap)), func(gtx layout.Context) {
				gtx.Constraints = layout.Exact(image.Pt(col, rest))
				panel(gtx)
			})
		}

		plate := image.Rect(body.Min.X+col+gtx.Dp(Gap), body.Min.Y, body.Max.X, body.Max.Y)
		if plate.Dx() <= 0 {
			return layout.Dimensions{Size: size}
		}
		fillRRect(gtx, plate, gtx.Dp(Radius), c.WindowBackground)
		strokeRRect(gtx, plate, gtx.Dp(Radius), gtx.Dp(Hairline),
			vgcolor.Flatten(c.Separator, p.Backdrop))
		inner := plate.Inset(gtx.Dp(SampleInset))
		if inner.Dx() <= 0 || inner.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}
		// Clipped to the plate: the sample is a fixed document laid out in
		// whatever height the window has left, and a fence running past the
		// plate's edge would read as a plate that failed to draw.
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
