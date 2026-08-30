// status.go is the bar across the foot of the vault window's content
// area: a quiet band under the document saying how long the note on
// screen is.
//
// # What a line is
//
// The count is the note file's own source lines: one for each line the
// file holds, where a line ends at a newline and a last line written
// without one is a line all the same. An empty file has no lines to count.
//
// A file ending with a final newline reports the lines it holds rather
// than an extra empty one after them: a viewer is not an editor and has no
// insertion point to seat after that newline, so it counts the file.
//
// The whole file counts, the frontmatter and its fences included — a count
// that left them out would disagree with every other tool that reads the
// same note.
//
// It is taken once, off the bytes the read returned, and kept on the note
// beside the stamp saying when that read happened. Nothing the window does
// can move it: not the width the prose wraps at, not the pane coming and
// going, not the properties panel folding away.
//
// # Where the bar stands
//
// It takes the content area's foot: it begins where the sidebar pane ends
// and runs to the window's trailing edge — the chrome row's span, at the
// other end of the same column — and everything above it stops at its top
// edge, so the document and the trailing panel end on one line. The
// sidebar pane is not under it and does not shorten: the pane floats one
// margin inside the window's own edges and keeps its own foot, which is
// the vault's actions.
//
// It stands on the window's ground, which is the note's paper, and paints
// nothing: no fill and no rule — the treatment the chrome row at the head
// of the same column has. A surface step under the document would make the
// paper a shape cut out of furniture on three sides.
//
// The count stands on the note column's own reading margin, under the
// breadcrumb and under the vault's name above that: one column of ink down
// the leading edge of the content area, top to bottom. The trailing end,
// where editors on this platform put their counts, is the panel of
// citations here rather than the document.
//
// The bar does not share a line with the vault's actions at the foot of
// the sidebar: those stand inside a card that floats one margin above the
// window's bottom edge and spends a rule and its own padding above them,
// so a band on the window's own foot would have to be twice as deep as the
// line it carries to put its baseline on theirs — depth that comes out of
// the document. What the two share is measured, on the window as it opens:
// the count's baseline rests level with the card's own bottom edge, a row
// under it.
//
// # What it says
//
// One fact, in one line: how long the note is. The bar is as deep as that
// line and no deeper, and it is the quiet ink this window gives what
// annotates rather than states — the same step the properties panel's keys
// and the standing messages take.
//
// The line is set a step under the window's other labels, the size the
// platform's own status lines take. That is also what keeps a grey label at
// the foot of a window from being read as a control somebody disabled.

package main

import (
	"bytes"
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// statusInkStep is the neutral step the bar's text takes: the quiet ink
// this window already spends on what annotates the document rather than
// being it — the properties panel's keys and the messages that stand in
// place of a document are the same step on the same paper.
//
// Measured against that paper, the pairing reads 6.19:1 in the light
// appearance and 11.06:1 in the dark one; against the trailing panel's
// surface, which the band runs over without putting ink on it, 5.46:1 and
// 9.91:1. Both are clear of the floor for text this size. The spread
// between them is the neutral ramp's own — its dark half runs at about
// twice the light half's contrast everywhere in this window.
const statusInkStep = 700

// statusBarHeight is the band's depth: one LabelMedium line box with the
// smallest spacing step above and below. It is the chrome row's own
// construction in the smaller role a status line takes, and like that row
// it spends no control padding — every dp the foot takes is a dp of
// document.
func statusBarHeight(tok themeTokens) unit.Dp {
	return unit.Dp(tok.typ.LabelMedium.LineHeight + 2*tok.sp.S1)
}

// sourceLines counts a note file's own lines: one per newline, plus a last
// line that ends without one, and none at all in an empty file. Bytes
// rather than text — a note whose lines end with a carriage return before
// the newline counts the same, since it is the newline that ends the line
// either way.
func sourceLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	n := bytes.Count(src, []byte{'\n'})
	if src[len(src)-1] != '\n' {
		n++
	}
	return n
}

// statusLine is what the bar says about the model: the open note's length,
// and nothing at all before a note is open — a bar reporting zero lines
// while the vault is still being scanned would state a fact it does not
// have.
func statusLine(m Model) string {
	note := m.CurrentNote()
	if note == nil {
		return ""
	}
	return lineCountLabel(note.Lines)
}

// lineCountLabel words a line count, the number leading.
func lineCountLabel(n int) string {
	if n == 1 {
		return "1 line"
	}
	return strconv.Itoa(n) + " lines"
}

// layoutStatusBar draws the bar into the band the frame reserved for it:
// the count on the note column's reading margin, centred in the band by
// the spacing step the band's own depth was built from.
//
// The band is claimed whether or not there is a count to put in it, so the
// document's height is the same before a note has loaded as after: a
// column that grew by a bar's depth as the first note arrived would reflow
// the whole page at that moment.
func layoutStatusBar(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	size := gtx.Constraints.Max
	line := statusLine(m)
	if line == "" || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	inset := gtx.Dp(unit.Dp(noteInsetDp))
	lgtx := gtx
	lgtx.Constraints.Min = image.Point{}
	lgtx.Constraints.Max.X = max(size.X-2*inset, 0)
	defer op.Offset(image.Pt(inset, gtx.Dp(unit.Dp(tok.sp.S1)))).Push(gtx.Ops).Pop()
	drawLabel(lgtx, tok.shaper, line, tok.typ.LabelMedium, tok.col.Ramps.Neutral.Step(statusInkStep))
	return layout.Dimensions{Size: size}
}
