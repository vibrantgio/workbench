// The preview: one Go source file, drawn as a fenced Go block is drawn in a
// document, in the chosen code face and the chosen syntax highlighter style.
//
// The file holds every kind of token a syntax highlighter style sets, and the
// characters a code face keeps apart. One line is wider than the box, so the
// code scrolls across as well as down.
package main

import (
	_ "embed"
	"image"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/paragraph"
	"github.com/vibrantgio/components/scrollarea"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// PreviewSource is the Go file the preview shows. The file's extension keeps
// the Go tool from compiling it.
//
//go:embed preview.go.txt
var PreviewSource string

// PreviewLabel heads the preview group and PreviewHint says what it shows.
const (
	PreviewLabel = "Preview"
	PreviewHint  = "A Go file in the chosen code face and syntax highlighter style."
)

// CodePad is the space between the code and the block's edges, the padding a
// fence has in a document.
var CodePad = unit.Dp(tokens.Spacing.S3)

// codeInf is the size the code is measured at on both axes: no line wraps and
// no line is cut.
const codeInf = 1e6

// codeView is the preview's scroll position. The scroll area moves the code
// across, and the view itself moves it down.
type codeView struct {
	across    scrollarea.State
	down      gesture.Scroll
	acrossBar scrollbar.State
	downBar   scrollbar.State
	// y is how far the viewport's top edge is into the code, in pixels,
	// always within [0, maxY].
	y int
	// content and viewport are the heights the most recent layout measured.
	content, viewport int
}

func (v *codeView) maxY() int { return max(v.content-v.viewport, 0) }

func (v *codeView) setY(y int) { v.y = min(max(y, 0), v.maxY()) }

// Preview draws the Go file in the whole box it is given. The box's fill is
// the style's own background and its rim the style's code border, as a
// fence's are; the text is the fence's spans in the code face.
func Preview(view *codeView, block *markdown.CodeBlock, shaper *text.Shaper, st markdown.Style) layout.Widget {
	spans := codeSpans(st, block)
	codeStyle := paragraph.Style{Color: st.CodeColor, Size: st.CodeSize}
	code := paragraph.Render(shaper, codeStyle, spans, paragraph.Idle())
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		box := image.Rectangle{Max: size}
		if size.X <= 0 || size.Y <= 0 {
			return layout.Dimensions{Size: size}
		}
		radius := gtx.Dp(BoxRadius)
		edge := 0
		if st.CodeBorder.A > 0 {
			edge = max(gtx.Dp(Hairline), 1)
			fillRRect(gtx, box, radius, st.CodeBorder)
		}
		inner := box.Inset(edge)
		innerR := max(radius-edge, 0)
		fillRRect(gtx, inner, innerR, st.CodeBackground)
		defer clip.UniformRRect(inner, innerR).Push(gtx.Ops).Pop()
		defer op.Offset(inner.Min).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(inner.Size())
		view.layout(gtx, gtx.Dp(CodePad), st.CodeScrollbar, code)
		return layout.Dimensions{Size: size}
	}
}

// layout shows code through a viewport of the size gtx gives, scrolled on
// both axes. A bar that stands has a lane of its own at its edge while its
// axis overflows; a bar that fades is drawn over the code.
func (v *codeView) layout(gtx layout.Context, pad int, bar scrollbar.Style, code layout.Widget) {
	size := gtx.Constraints.Max

	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(codeInf, codeInf)}
	macro := op.Record(gtx.Ops)
	dims := code(cgtx)
	call := macro.Stop()
	content := dims.Size.Add(image.Pt(2*pad, 2*pad))

	// Each lane makes the other axis shorter, so the second pass settles a
	// lane the first pass made necessary.
	lane := 0
	if list.AnchorFor(bar) == list.Occupy {
		lane = gtx.Dp(bar.Width())
	}
	acrossLane, downLane := 0, 0
	for range 2 {
		if content.X > size.X-downLane {
			acrossLane = lane
		}
		if content.Y > size.Y-acrossLane {
			downLane = lane
		}
	}
	port := image.Pt(size.X-downLane, size.Y-acrossLane)

	v.content, v.viewport = content.Y, port.Y
	v.setY(v.y)
	if v.maxY() > 0 {
		dist := v.down.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical,
			pointer.ScrollRange{}, pointer.ScrollRange{Min: -v.y, Max: v.maxY() - v.y})
		v.setY(v.y + dist)
	}
	// The scroll area is laid out inside the view's own area, so a vertical
	// gesture over the code passes through the scroll area to the view.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	if v.maxY() > 0 {
		v.down.Add(gtx.Ops)
	}

	agtx := gtx
	agtx.Constraints = layout.Exact(image.Pt(port.X, port.Y))
	page := func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: image.Pt(content.X, port.Y)}.Push(gtx.Ops).Pop()
		defer op.Offset(image.Pt(pad, pad-v.y)).Push(gtx.Ops).Pop()
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(content.X, port.Y)}
	}
	scrollarea.Style{}.Layout(agtx, &v.across, page)
	area.Pop()

	// Each bar runs along its edge of the viewport. A bar that fades is
	// drawn over the code, so while both axes overflow each bar stops short
	// of the corner by the other's width. A bar that stands has a lane, and
	// the two lanes leave the corner clear.
	w := gtx.Dp(bar.Width())
	overlay := list.AnchorFor(bar) == list.Overlay
	if v.across.Overflows() {
		long := port.X
		if overlay && v.maxY() > 0 {
			long -= w
		}
		start, end := v.across.Fractions()
		stack := op.Offset(image.Pt(0, size.Y-w)).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(image.Pt(max(long, 0), w))
		bar.Layout(bgtx, &v.acrossBar, layout.Horizontal, start, end)
		stack.Pop()
		if d := v.acrossBar.ScrollDistance(); d != 0 {
			v.across.ScrollBy(int(d * float32(v.across.Content())))
		}
	}
	if v.maxY() > 0 {
		long := port.Y
		if overlay && v.across.Overflows() {
			long -= w
		}
		start := float32(v.y) / float32(v.content)
		end := float32(v.y+v.viewport) / float32(v.content)
		stack := op.Offset(image.Pt(size.X-w, 0)).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(image.Pt(w, max(long, 0)))
		bar.Layout(bgtx, &v.downBar, layout.Vertical, start, end)
		stack.Pop()
		if d := v.downBar.ScrollDistance(); d != 0 {
			v.setY(v.y + int(d*float32(v.content)))
		}
	}
}

// codeSpans maps the block onto paragraph spans as the markdown package maps
// a fence: the highlighter's runs in the code face, with a newline that opens
// a run moved to the end of the run before it, or one plain run when the
// highlighter returns nothing.
func codeSpans(st markdown.Style, cb *markdown.CodeBlock) []paragraph.SpanStyle {
	if st.Highlight != nil {
		if hl := st.Highlight(cb.Language, cb.Code); len(hl) > 0 {
			out := make([]paragraph.SpanStyle, 0, len(hl))
			for _, h := range hl {
				content := h.Text
				if len(out) > 0 {
					i := 0
					for i < len(content) && content[i] == '\n' {
						i++
					}
					out[len(out)-1].Content += content[:i]
					content = content[i:]
				}
				if content == "" {
					continue
				}
				rs := paragraph.SpanStyle{Content: content, Color: h.Color, Typeface: st.Mono}
				if h.Bold {
					rs.Weight = font.Bold
				}
				if h.Italic {
					rs.Style = font.Italic
				}
				out = append(out, rs)
			}
			return out
		}
	}
	return []paragraph.SpanStyle{{Content: cb.Code, Typeface: st.Mono}}
}
