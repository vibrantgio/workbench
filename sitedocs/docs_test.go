package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

const (
	// docsCanvasW matches the runtime Main slot width budget: 1200 dp
	// window − 192 dp sidebar = 1008 dp, rounded down to 1000 for a
	// deterministic golden size.
	docsCanvasW = 1000
	// docsCanvasH is large enough to fit the breadcrumb + prose + two
	// or three cards comfortably without scroll, so the goldens capture
	// the full page composition rather than only the visible viewport.
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
			obs := docsPage(rx.Of(theme.Default()), shaper, tc.Content)
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

// TestDocsPageGolden records or diffs each docs page in light and dark
// themes. Following the G5.1b structural-golden pattern, text labels are
// replaced with single-space stand-ins so the diff captures structural
// regressions (breadcrumb chevron count, card outlines, paragraph stack
// heights) rather than font rasterisation noise. The runtime path in
// docsPage uses docs_content.go for real copy.
func TestDocsPageGolden(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	lightBG := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	darkBG := color.NRGBA{R: 20, G: 20, B: 20, A: 255}

	pageCases := []struct {
		id    string
		title string
		codes int
	}{
		{"getting-started", "getting-started", 2},
		{"phases-overview", "phases-overview", 2},
		{"component-reference", "component-reference", 3},
	}
	themeCases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light", tokens.DefaultLight, lightBG},
		{"dark", tokens.DefaultDark, darkBG},
	}
	for _, pc := range pageCases {
		for _, tc := range themeCases {
			name := tc.name + "-" + pc.id
			t.Run(name, func(t *testing.T) {
				content := structuralDocsContent(pc.title, 2, pc.codes)
				w := renderDocsPage(shaper, content, tc.colors, tokens.Spacing, sharpRadius, tokens.DefaultTypeScale)
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
	content := structuralDocsContent("getting-started", 2, 2)

	light := renderDocsPage(shaper, content, tokens.DefaultLight, tokens.Spacing, sharpRadius, tokens.DefaultTypeScale)
	dark := renderDocsPage(shaper, content, tokens.DefaultDark, tokens.Spacing, sharpRadius, tokens.DefaultTypeScale)
	a := capture(t, docsCanvasSize, scene(light, bg))
	b := capture(t, docsCanvasSize, scene(dark, bg))
	if a == nil || b == nil {
		return
	}
	if n := pixelDiff(a, b); n == 0 {
		t.Error("light and dark docs page render identically; expected colour differences across breadcrumb / prose / cards")
	}
}

// structuralDocsContent returns a docsPageContent populated with
// blank/single-space labels. The structural goldens depend on
// composition shape (breadcrumb segments, paragraph rows, card outlines)
// rather than font rasterisation. BreadcrumbLabels is set so the
// breadcrumb's Home/Docs/Title segments render as spaces too.
func structuralDocsContent(title string, paragraphs, codes int) docsPageContent {
	c := docsPageContent{
		Title:            title,
		BreadcrumbLabels: []string{" ", " ", " "},
	}
	for range paragraphs {
		c.Paragraphs = append(c.Paragraphs, " ")
	}
	for range codes {
		c.Codes = append(c.Codes, docsCodeSample{
			Caption: " ",
			Lines:   []string{" ", " "},
		})
	}
	return c
}
