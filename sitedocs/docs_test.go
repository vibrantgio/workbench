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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

const (
	// docsCanvasW matches the runtime Main slot width budget: 1200 dp
	// window − 192 dp sidebar = 1008 dp, rounded down to 1000 for a
	// deterministic golden size.
	docsCanvasW = 1000
	// docsCanvasH is the golden viewport height. The markdown document
	// scrolls, so the goldens capture the top-of-page viewport — heading
	// scale, prose, list markers, and the first code block.
	docsCanvasH = 700
)

var docsCanvasSize = image.Pt(docsCanvasW, docsCanvasH)

// TestDocsPageConstructs is the smoke test: every docs page in the
// docsPages registry must build and emit a widget that lays out one
// frame without panicking, so the sidebar can never route to a page
// that fails to render.
func TestDocsPageConstructs(t *testing.T) {
	for _, def := range docsPages() {
		tc := def
		t.Run(tc.ID, func(t *testing.T) {
			obs := docsPage(rx.Of(theme.Default()), tc)
			w, err := collectOne(obs)
			if err != nil {
				t.Fatalf("docsPage subscribe: %v", err)
			}
			if w == nil {
				t.Fatal("docsPage produced no widget")
			}
			dims := drawOnce(t, docsCanvasSize, w)
			if dims.Size.X == 0 || dims.Size.Y == 0 {
				t.Errorf("docsPage produced zero dimensions: %v", dims)
			}
		})
	}
}

// TestDocsSourcesParse pins the embedded markdown sources' shape: every
// page parses to a document that opens with a level-1 heading (the page
// title) followed by more content, so a truncated or mis-named .md file
// fails loudly rather than rendering an empty page.
func TestDocsSourcesParse(t *testing.T) {
	for _, def := range docsPages() {
		tc := def
		t.Run(tc.ID, func(t *testing.T) {
			if len(tc.Source) == 0 {
				t.Fatal("embedded source is empty")
			}
			blocks := markdown.Parse(tc.Source)
			if len(blocks) < 2 {
				t.Fatalf("parsed %d blocks, want at least a heading and a body", len(blocks))
			}
			h, ok := blocks[0].(*markdown.Heading)
			if !ok {
				t.Fatalf("first block is %T, want *markdown.Heading", blocks[0])
			}
			if h.Level != 1 {
				t.Errorf("first heading level = %d, want 1", h.Level)
			}
		})
	}
}

// TestDocsSidebarConstructs verifies the accordion-grouped sidebar
// widget builds and lays out a frame without panicking. The accordion's
// initial open state is driven by an rx.Of observable seeded with
// section 0 open (matching the MVU initial model).
func TestDocsSidebarConstructs(t *testing.T) {
	openObs := rx.Of(map[int]bool{0: true})
	sbObs := docsSidebar(rx.Of(theme.Default()), openObs)
	w, err := collectOne(sbObs)
	if err != nil {
		t.Fatalf("docsSidebar subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("docsSidebar produced no widget")
	}
	dims := drawOnce(t, image.Pt(docsSidebarWidthDp, docsCanvasH), w)
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Errorf("docsSidebar produced zero dimensions: %v", dims)
	}
}

// TestDocsPageGolden records or diffs representative docs pages in light
// and dark themes, rendered from their embedded markdown sources exactly
// as the runtime path does (breadcrumb + markdown document with chroma
// highlighting). Three pages cover the block variety: getting-started
// (list + links + two code fences), cadence-shells (table), and mvu-loop
// (multi-line Go fences). Rendering uses the theme's shaper — Roboto and
// Roboto Mono, system fonts disabled — so the rasterisation is
// deterministic on a given text stack and the code blocks capture the
// mono face (F0.2).
func TestDocsPageGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	pageCases := []string{pagePrismGettingStarted, pageCadenceShells, pageMVULoop}
	themeCases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light", tokens.DefaultLight, lightBG},
		{"dark", tokens.DefaultDark, darkBG},
	}
	for _, id := range pageCases {
		def := docsPageByID(t, id)
		for _, tc := range themeCases {
			name := tc.name + "-" + id
			t.Run(name, func(t *testing.T) {
				w := renderDocsPage(shaper, def, tc.colors, tokens.Spacing, tokens.DefaultTypography)
				renderGolden(t, "docs-"+name, docsCanvasSize, scene(w, tc.bg))
			})
		}
	}
}

// TestDocsPageLightDarkDiffer confirms swapping the colour token set
// changes the rendered output of a docs page. The Getting-started page
// is used as the representative case.
func TestDocsPageLightDarkDiffer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	def := docsPageByID(t, pagePrismGettingStarted)

	light := renderDocsPage(shaper, def, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography)
	dark := renderDocsPage(shaper, def, tokens.DefaultDark, tokens.Spacing, tokens.DefaultTypography)
	a := capture(t, docsCanvasSize, scene(light, bg))
	b := capture(t, docsCanvasSize, scene(dark, bg))
	if a == nil || b == nil {
		return
	}
	if n := pixelDiff(a, b); n == 0 {
		t.Error("light and dark docs page render identically; expected colour differences across breadcrumb / prose / code")
	}
}

// TestDocsCodeShapesInMonoFace is the F1.4 headless confirmation that the
// docs pages — the first app surface after F0.2 wired markdown's code path
// to the theme's Code role — render code in the mono face. It exercises the
// exact pieces the runtime path (docsPage) composes for the default theme:
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
//  3. a real docs page with Go code fences (mvu-loop), drawn through the
//     app's own drawDocsPage with the app's style and shaper, renders
//     different pixels than the same page with Mono forced back to Roboto —
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

	// 3. The mono face changes the rendered pixels of a real docs page.
	def := docsPageByID(t, pageMVULoop) // multi-line Go fences
	propStyle := style
	propStyle.Mono = "Roboto"
	monoDoc := markdown.NewDocument(markdown.Parse(def.Source))
	propDoc := markdown.NewDocument(markdown.Parse(def.Source))
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	a := capture(t, docsCanvasSize, scene(func(gtx layout.Context) layout.Dimensions {
		return drawDocsPage(gtx, nil, monoDoc, shaper, style)
	}, bg))
	b := capture(t, docsCanvasSize, scene(func(gtx layout.Context) layout.Dimensions {
		return drawDocsPage(gtx, nil, propDoc, shaper, propStyle)
	}, bg))
	if a == nil || b == nil {
		return
	}
	if n := pixelDiff(a, b); n <= 0 {
		t.Errorf("docs page renders identically with Mono forced to Roboto (%d pixels differ); code is not shaping in the mono face", n)
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

// docsPageByID returns the registry entry for a route identifier.
func docsPageByID(t *testing.T, id string) docsPageDef {
	t.Helper()
	for _, def := range docsPages() {
		if def.ID == id {
			return def
		}
	}
	t.Fatalf("no docs page with ID %q", id)
	return docsPageDef{}
}
