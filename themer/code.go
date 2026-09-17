// What the window keeps about code: the document the preview's fence is laid
// out from, and the style that document is drawn through.
//
// The document is parsed once and restyled per emission. Colour is no part of
// the parse — the document takes its style at layout time — so choosing
// another highlighter style re-dresses a document already read rather than
// reading one again.
package main

import (
	stdcolor "image/color"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/tokens"
)

// CodeSource is what the preview's content pane holds: a line of prose with a
// word of code quoted into it, a Go block carrying a comment, a keyword run,
// a string and a number, and a line of prose under it — a highlighter style
// that cannot be told from another on those four runs cannot be told from it
// at all.
//
// The prose is there because a fence is judged against the page it is an
// island in: a block of colour on a plate of its own says nothing about how it
// reads under a paragraph. The lines are short because the block stands in the
// content pane of a picture of a window, and a line wider than that pane
// scrolls under a bar instead of being read.
const CodeSource = "The greeting is one call, and `Greet` is the one that makes it.\n\n" +
	"```go\n" +
	"// Greet names whoever is asking.\n" +
	"func Greet(name string) string {\n" +
	"\treturn fmt.Sprintf(\"Hello, %s! (%d)\", name, 42)\n" +
	"}\n" +
	"```\n\n" +
	"Anything that needs a name of its own gets one; the rest takes the\n" +
	"caller's.\n"

// codeState is what the window keeps across emissions: the two choosers'
// handlers and scroll position, and the parsed sample.
type codeState struct {
	styles *styleChooser
	faces  *faceChooser
	doc    *markdown.Document
}

func newCodeState() *codeState {
	return &codeState{
		styles: newStyleChooser(),
		faces:  newFaceChooser(),
		doc:    markdown.NewDocument(markdown.Parse([]byte(CodeSource))),
	}
}

// CodeStyle is the document style the preview's fence is drawn through: the
// previewed set's own reading colours, the chosen code face, and a fence
// wearing the chosen highlighter style.
//
// The style is worn rather than re-fitted. A fence takes the background its
// author drew their colours on and those colours as they were written, so the
// block is the style itself and not a rendering of it; everything round it
// stays the platform's. Which member of the pair reaches the fence follows the
// set, so a change of appearance is a change of plate.
//
// standsOn is the fill the document is laid on — the content pane of the
// picture of a window — so every alpha-carrying name resolves against what it
// actually lands on.
func CodeStyle(c tokens.PlatformColors, typ tokens.Typography, standsOn stdcolor.NRGBA, styles highlight.StylePair) markdown.Style {
	st := markdown.FromTokens(c, typ, standsOn)
	highlight.WearPair(&st, styles, c)
	return st
}
