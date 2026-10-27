// What the window keeps about code: the Go file the preview shows, parsed
// once as a fenced Go block, and the style that block is drawn through.
//
// Colour is no part of the parse: the block takes its style at layout time.
// So a change of highlighter style restyles the block already read and does
// not read it again.
package main

import (
	stdcolor "image/color"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/tokens"
)

// codeState is what the window keeps across emissions: the two choosers'
// handlers and scroll positions, the preview's scroll position, and the
// parsed file.
type codeState struct {
	styles *styleChooser
	faces  *faceChooser
	view   *codeView
	block  *markdown.CodeBlock
}

func newCodeState() *codeState {
	return &codeState{
		styles: newStyleChooser(),
		faces:  newFaceChooser(),
		view:   new(codeView),
		block:  previewBlock(),
	}
}

// previewBlock parses [PreviewSource] as the body of a Go fence, so the
// preview's code is the code a Go fence in a document holds, tabs expanded.
func previewBlock() *markdown.CodeBlock {
	for _, b := range markdown.Parse([]byte("```go\n" + PreviewSource + "```\n")) {
		if cb, ok := b.(*markdown.CodeBlock); ok {
			return cb
		}
	}
	return &markdown.CodeBlock{Language: "go", Code: PreviewSource}
}

// CodeStyle is the document style the preview's fence is drawn through: the
// previewed set's own reading colours, the chosen code face, and a fence
// with the chosen highlighter style applied.
//
// The style is applied rather than re-fitted. A fence takes the background its
// author drew their colours on and those colours as they were written, so the
// block is the style itself and not a rendering of it; everything round it
// stays the platform's. Which member of the pair reaches the fence follows the
// set, so a change of appearance is a change of plate.
//
// standsOn is the fill the preview's box is drawn on, so every name that
// carries alpha resolves against the fill under it.
func CodeStyle(c tokens.PlatformColors, typ tokens.Typography, standsOn stdcolor.NRGBA, styles highlight.StylePair) markdown.Style {
	st := markdown.FromTokens(c, typ, standsOn)
	highlight.ApplyPair(&st, styles, c)
	return st
}
