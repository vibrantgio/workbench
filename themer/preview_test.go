package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// previewTokens lexes the preview's code with the lexer the markdown
// package's highlighter uses for a Go fence, whitespace left out.
func previewTokens(t *testing.T) []chroma.Token {
	t.Helper()
	block := previewBlock()
	if block.Language != "go" {
		t.Fatalf("the preview's block is in language %q, want go", block.Language)
	}
	lexer := lexers.Get(block.Language)
	if lexer == nil {
		t.Fatal("chroma has no lexer for go")
	}
	it, err := lexer.Tokenise(nil, block.Code)
	if err != nil {
		t.Fatal(err)
	}
	var out []chroma.Token
	for _, tok := range it.Tokens() {
		if tok.Type != chroma.TextWhitespace && strings.TrimSpace(tok.Value) != "" {
			out = append(out, tok)
		}
	}
	return out
}

// TestThePreviewHoldsEveryTokenKind lexes the preview's Go file and finds
// each kind of token a syntax highlighter style sets. The lexer splits a raw
// string into its two backquotes and its body. Chroma gives doc
// comments, struct tags, type parameters and escapes no token types of their
// own, so these four are found from where their tokens are: a comment on the
// line above a declaration, a raw string inside a struct, a bracket after a
// name that opens a name and a constraint, and a backslash in a string.
func TestThePreviewHoldsEveryTokenKind(t *testing.T) {
	toks := previewTokens(t)
	has := func(pred func(i int, tok chroma.Token) bool) bool {
		for i, tok := range toks {
			if pred(i, tok) {
				return true
			}
		}
		return false
	}
	of := func(tt chroma.TokenType) func(int, chroma.Token) bool {
		return func(_ int, tok chroma.Token) bool { return tok.Type == tt }
	}
	isName := func(tok chroma.Token) bool { return tok.Type.InCategory(chroma.Name) }
	isKeyword := func(tok chroma.Token) bool { return tok.Type.InCategory(chroma.Keyword) }

	depth, inStruct := 0, -1
	tags := 0
	for i, tok := range toks {
		switch {
		case tok.Value == "struct" && isKeyword(tok):
			inStruct = depth
		case tok.Type == chroma.Punctuation && tok.Value == "{":
			depth++
		case tok.Type == chroma.Punctuation && tok.Value == "}":
			depth--
			if depth == inStruct {
				inStruct = -1
			}
		case inStruct >= 0 && depth > inStruct && tok.Type == chroma.LiteralString &&
			i > 0 && toks[i-1].Value == "`" && strings.Contains(tok.Value, `:"`):
			tags++
		}
	}

	for _, kind := range []struct {
		name  string
		found bool
	}{
		{"keywords", has(of(chroma.Keyword))},
		{"types", has(of(chroma.KeywordType))},
		{"built-in names", has(of(chroma.NameBuiltin))},
		{"function names", has(of(chroma.NameFunction))},
		{"strings with escapes", has(func(_ int, tok chroma.Token) bool {
			return tok.Type == chroma.LiteralString && strings.HasPrefix(tok.Value, `"`) && strings.Contains(tok.Value, `\`)
		})},
		{"raw strings", has(func(_ int, tok chroma.Token) bool {
			return tok.Type == chroma.LiteralString && strings.HasPrefix(tok.Value, "`")
		})},
		{"integers", has(of(chroma.LiteralNumberInteger))},
		{"floats", has(of(chroma.LiteralNumberFloat))},
		{"hexadecimal numbers", has(of(chroma.LiteralNumberHex))},
		{"line comments", has(of(chroma.CommentSingle))},
		{"doc comments", has(func(i int, tok chroma.Token) bool {
			return tok.Type == chroma.CommentSingle && i+1 < len(toks) && isKeyword(toks[i+1]) &&
				strings.Contains("func type var const package", toks[i+1].Value)
		})},
		{"operators", has(of(chroma.Operator))},
		{"punctuation", has(of(chroma.Punctuation))},
		{"struct tags", tags > 0},
		{"type parameters", has(func(i int, tok chroma.Token) bool {
			return tok.Type == chroma.Punctuation && tok.Value == "[" && i > 0 && i+2 < len(toks) &&
				isName(toks[i-1]) && isName(toks[i+1]) && isKeyword(toks[i+2])
		})},
	} {
		if !kind.found {
			t.Errorf("the preview's Go file holds no %s", kind.name)
		}
	}

	// The characters a code face keeps apart.
	for _, chars := range []string{"0 O o", "1 l I", "()", "[]", "{}", "=>", "!=", "<="} {
		if !strings.Contains(PreviewSource, chars) {
			t.Errorf("the preview's Go file does not hold %q", chars)
		}
	}
	if n := strings.Count(PreviewSource, "\n"); n < 60 || n > 90 {
		t.Errorf("the preview's Go file has %d lines, want 60 to 90", n)
	}
}

// TestTheLongLineScrollsAcross finds the code wider than the preview at the
// window's opening size, and then scrolls it: a horizontal scroll over the
// preview moves the code across, and a vertical one moves it down.
func TestTheLongLineScrollsAcross(t *testing.T) {
	m := onCode(judging())
	c := tokens.PlatformLight
	code := settled(false)
	pageWith(t, m, c, code)
	if got, port := code.view.across.Content(), code.view.across.Viewport(); got <= port {
		t.Fatalf("the code is %d points wide in a preview %d points wide: no line is wider than the preview", got, port)
	}
	if code.view.maxY() <= 0 {
		t.Fatalf("the code is %d points tall in a preview %d points tall: it does not scroll down", code.view.content, code.view.viewport)
	}

	box := previewRect()
	size := box.Size()
	typo := tokens.CodeFace(m.AppliedMono())
	shaper := typo.DeterministicShaper()
	st := CodeStyle(PreviewSet(c, m, false), typo, PaletteFrom(WindowSet(c, m)).Backdrop, m.AppliedStyles())
	view := new(codeView)
	w := Preview(view, previewBlock(), shaper, st)
	before := golden.Capture(t, size, w)

	r := new(gioinput.Router)
	var ops op.Ops
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Now:         time.Time{},
			Source:      r.Source(),
		}
		w(gtx)
		r.Frame(&ops)
	}
	frame()
	r.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(float32(size.X)/2, float32(size.Y)/2),
		Scroll:   f32.Pt(80, 0),
		Source:   pointer.Mouse,
	})
	frame()
	if view.across.Offset() == 0 {
		t.Fatal("a horizontal scroll over the preview left the code at its leading edge")
	}
	r.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(float32(size.X)/2, float32(size.Y)/2),
		Scroll:   f32.Pt(0, 40),
		Source:   pointer.Mouse,
	})
	frame()
	if view.y == 0 {
		t.Error("a vertical scroll over the preview left the code at its top")
	}
	after := golden.Capture(t, size, w)
	if n := movedIn(before, after, image.Rectangle{Max: size}); n == 0 {
		t.Error("the scrolls moved no pixel of the preview")
	}
	t.Logf("scrolled to %d across and %d down", view.across.Offset(), view.y)
}
