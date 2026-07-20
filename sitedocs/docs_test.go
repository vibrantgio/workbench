package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
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
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	for _, def := range docsPages() {
		tc := def
		t.Run(tc.ID, func(t *testing.T) {
			obs := docsPage(rx.Of(theme.Default()), shaper, tc)
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
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	openObs := rx.Of(map[int]bool{0: true})
	sbObs := docsSidebar(rx.Of(theme.Default()), shaper, openObs)
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
// (multi-line Go fences). Rendering uses gofont with system fonts
// disabled, so the rasterisation is deterministic on a given text stack.
func TestDocsPageGolden(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
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
				w := renderDocsPage(shaper, def, tc.colors, tokens.Spacing, tokens.DefaultTypeScale)
				renderGolden(t, "docs-"+name, docsCanvasSize, scene(w, tc.bg))
			})
		}
	}
}

// TestDocsPageLightDarkDiffer confirms swapping the colour token set
// changes the rendered output of a docs page. The Getting-started page
// is used as the representative case.
func TestDocsPageLightDarkDiffer(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	def := docsPageByID(t, pagePrismGettingStarted)

	light := renderDocsPage(shaper, def, tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypeScale)
	dark := renderDocsPage(shaper, def, tokens.DefaultDark, tokens.Spacing, tokens.DefaultTypeScale)
	a := capture(t, docsCanvasSize, scene(light, bg))
	b := capture(t, docsCanvasSize, scene(dark, bg))
	if a == nil || b == nil {
		return
	}
	if n := pixelDiff(a, b); n == 0 {
		t.Error("light and dark docs page render identically; expected colour differences across breadcrumb / prose / code")
	}
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
