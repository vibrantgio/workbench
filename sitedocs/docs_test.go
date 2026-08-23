package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

const (
	// docsCanvasW/H is the deterministic canvas the docs-layer unit tests
	// draw one frame at.
	docsCanvasW = 1000
	docsCanvasH = 700
)

var docsCanvasSize = image.Pt(docsCanvasW, docsCanvasH)

// monoProofSource is a minimal guide-shaped document whose Go fence gives
// the mono-face proof below real code pixels to move.
const monoProofSource = "# Mono proof\n\nProse before the fence.\n\n```go\nfunc main() {\n\twiiim := \"{mono[0] != prose}\"\n\t_ = wiiim\n}\n```\n"

// TestDocsCodeShapesInMonoFace is the F1.4 headless confirmation that the
// Docs tab renders code in the mono face. It exercises the exact pieces
// the runtime path (guideDocObservable) composes for the default theme:
// docsMarkdownStyle over the theme-emitted tokens, and the theme's cached
// Typography shaper that doc.Layout shapes with.
//
// Three layers of proof, strongest headless verification available:
//  1. the style the app builds names the theme Code role's "Roboto Mono"
//     typeface at the Code role's size;
//  2. that typeface resolves in the app's shaper to a real distinct face —
//     F0.2's glyph-face/advance technique: the shaper holds no system
//     fonts, so glyphs coming back proves resolution; the advance differs
//     from proportional Roboto ('w', 'i', 'm', '.' collapse to one width
//     only under a mono face); and the glyph IDs differ (a Gio GlyphID
//     packs the face index, so this is face identity, not just metrics);
//  3. a document with a Go code fence, drawn through the app's own
//     drawGuideDoc with the app's style and shaper, renders different
//     pixels than the same document with Mono forced back to Roboto —
//     the mono face visibly reaches the composed page.
func TestDocsCodeShapesInMonoFace(t *testing.T) {
	typ := tokens.DefaultTypography
	style := docsMarkdownStyle(tokens.DefaultLight, typ)
	shaper := typ.DeterministicShaper()

	// 1. The style resolves the theme's Code role.
	if got, want := string(style.Mono), typ.Code.Typeface; got != want {
		t.Fatalf("Style.Mono = %q, want the theme Code role's %q", got, want)
	}
	if got, want := style.CodeSize, unit.Sp(typ.Code.Size); got != want {
		t.Errorf("Style.CodeSize = %v, want the Code role's %v", got, want)
	}

	// 2. The mono face resolves and is a distinct face from Roboto.
	shapeRun := func(f font.Font) (fixed.Int26_6, []text.GlyphID) {
		shaper.LayoutString(text.Parameters{
			Font:     f,
			PxPerEm:  fixed.I(16),
			MaxWidth: 100000,
		}, "wiiim... {mono[0] != prose}")
		var advance fixed.Int26_6
		var ids []text.GlyphID
		for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
			advance += g.Advance
			ids = append(ids, g.ID)
		}
		return advance, ids
	}
	monoAdvance, monoIDs := shapeRun(font.Font{Typeface: style.Mono})
	if len(monoIDs) == 0 {
		t.Fatalf("Mono typeface %q shaped no glyphs; the face did not resolve in the theme shaper", style.Mono)
	}
	robotoAdvance, robotoIDs := shapeRun(font.Font{Typeface: "Roboto"})
	if monoAdvance == robotoAdvance {
		t.Errorf("mono advance %v equals proportional Roboto's; %q likely fell back to Roboto", monoAdvance, style.Mono)
	}
	if glyphIDsEqual(monoIDs, robotoIDs) {
		t.Errorf("mono and Roboto shaped to identical glyph IDs; the two requests collapsed onto one face")
	}

	// 3. The mono face changes the rendered pixels of the composed page.
	propStyle := style
	propStyle.Mono = "Roboto"
	monoDoc := markdown.NewDocument(markdown.Parse([]byte(monoProofSource)))
	propDoc := markdown.NewDocument(markdown.Parse([]byte(monoProofSource)))
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	a := golden.Capture(t, docsCanvasSize, scene(func(gtx layout.Context) layout.Dimensions {
		return drawGuideDoc(gtx, monoDoc, shaper, style)
	}, bg))
	b := golden.Capture(t, docsCanvasSize, scene(func(gtx layout.Context) layout.Dimensions {
		return drawGuideDoc(gtx, propDoc, shaper, propStyle)
	}, bg))
	if n := golden.PixelDiff(a, b); n <= 0 {
		t.Errorf("guide document renders identically with Mono forced to Roboto (%d pixels differ); code is not shaping in the mono face", n)
	}
}

func glyphIDsEqual(a, b []text.GlyphID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
