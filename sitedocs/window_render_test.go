package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. The app is a native window binary with no offscreen mode of
// its own, but every layer it stacks is a plain layout.Widget over
// pre-resolved tokens, so composing the same three paints into a headless
// image at the size the window opens at produces the frame the window would
// show.
//
// Run it with -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/sitedocs
//
// Without the flag it still renders both schemes every run, which makes it a
// smoke test of the whole stack: a panic anywhere in the backdrop, the title
// band, the outline rail or the document fails it.
//
// The assertions sample the rendered frame rather than a struct of colours,
// because this app holds none: each region paints its own fill at the point
// it draws, and the frame is the only place the question "what is this region
// drawn in" has an answer that sees what was painted rather than what was
// meant.

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/markdown"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Site Docs window opens at (main.go), and the
// only size these frames are drawn at: a composition is worth looking at
// where somebody actually looks at it.
var windowSize = image.Pt(windowW, windowH)

// titleBandDp is the strip desktop.TopInset reports on a full-size-content
// macOS window, stated here because a go test binary has no live window to
// measure. The number is the stored reference's plain title bar band, 32 px
// with the fill running y0–30 — not a guess and not a live measurement.
const titleBandDp = 32

// windowFrame composes the window for one scheme exactly as buildLayers
// stacks it: the backdrop first, then the page on its fill, inset under a
// title-bar strip painted in the fills of the columns it caps. The page is
// the static twin the goldens draw (renderDocsPage), so the frame is the
// app's composition minus the streams.
//
// band is the strip height to render under; 0 draws the window as every
// platform but macOS shows it, with the page at the window's own top edge.
// find is the find bar's field state; nil draws the window with the bar
// closed.
func windowFrame(
	shaper *text.Shaper,
	guide []byte,
	st outlineState,
	c tokens.PlatformColors,
	typo tokens.Typography,
	band unit.Dp,
	find *input.RenderState,
) layout.Widget {
	page := onPageFill(c, renderDocsPage(shaper, guide, st, c, typo, find))
	background := backdrop.Widget(c.WindowBackground)
	capped := bandedCap(func() unit.Dp { return band }, c, page)
	return func(gtx layout.Context) layout.Dimensions {
		background(gtx)
		return capped(gtx)
	}
}

// renderWindow draws the settled window — its first section disclosed and
// selected, which is the state a reader leaves this window in — at the app's
// own size.
func renderWindow(t *testing.T, c tokens.PlatformColors, band unit.Dp) *image.RGBA {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	guide := guideFixture(t)
	first := guideOutline(markdown.Parse(guide))[0]
	st := outlineState{open: map[int]bool{0: true}, selected: first.Block}
	w := windowFrame(shaper, guide, st, c, typo, band, nil)
	return golden.Capture(t, windowSize, w)
}

// at reads one pixel as an opaque NRGBA, which is what every token in the
// set is.
func pixelAt(img *image.RGBA, p image.Point) color.NRGBA {
	r, g, b, _ := img.At(p.X, p.Y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
}

// windowSchemes is the pair every rule below is stated once and checked
// twice against.
var windowSchemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// Sample points in the rendered window, in the pixels the frame is drawn at
// (PxPerDp is 1, so a dp is a pixel). Each names a resting expanse and is
// chosen clear of paint: the title band over the rail and over the document,
// the rail under its last row, the document plane out past the reading
// measure the guide is capped to, and the first row under the band on both
// sides of the rail's seam.
var (
	atBandOverRail     = image.Pt(150, titleBandDp/2)
	atBandOverDocument = image.Pt(1100, titleBandDp/2)
	atRail             = image.Pt(150, 700)
	atRailTop          = image.Pt(150, titleBandDp)
	atDocument         = image.Pt(1100, 700)
	atDocumentTop      = image.Pt(1100, titleBandDp)
)

// TestWholeWindowRender draws the composed window in both schemes, and
// writes the frames out when -window.dump names a directory. The dumped
// frames carry the title band; the assertions below read the same
// composition without it, which is what a headless capture and every
// non-macOS platform actually show.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			if img.Bounds().Size() != windowSize {
				t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowSize)
			}
			if *windowDump == "" {
				return
			}
			if err := os.MkdirAll(*windowDump, 0o755); err != nil {
				t.Fatalf("dump dir: %v", err)
			}
			path := filepath.Join(*windowDump, "sitedocs-"+tc.name+".png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("create %s: %v", path, err)
			}
			defer f.Close()
			if err := png.Encode(f, img); err != nil {
				t.Fatalf("encode %s: %v", path, err)
			}
			t.Logf("wrote %s", path)
		})
	}
}

// TestWindowRegionsAreDrawnInThePlatformsFills reads the assignment off the frame:
// the guide document — the thing this window exists to show — on the
// platform's content plane, the outline rail indexing it in the fill a
// sidebar is drawn in, and the title band over each of them in that
// region's own fill.
//
// It is read off the rendered frame rather than off the set, because those
// regions are painted by different pieces of code (this app and backdrop)
// and the only place they can be seen agreeing is a frame that has all of
// them in it.
func TestWindowRegionsAreDrawnInThePlatformsFills(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"document plane", atDocument, tc.c.ControlBackground},
				{"outline rail", atRail, tc.c.SidebarMaterial},
				{"title band over the rail", atBandOverRail, tc.c.SidebarMaterial},
				{"title band over the document", atBandOverDocument, tc.c.ControlBackground},
			} {
				if got := pixelAt(img, r.at); got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
			}
		})
	}
}

// TestTheBandAgreesWithTheColumnsItCaps states the agreement directly: the
// band's fill over each column is read off one frame and compared to the
// column's first row under the band, so the rule holds wherever the fills
// under it move to. The rail's seam runs through the band as well.
func TestTheBandAgreesWithTheColumnsItCaps(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			for _, p := range []struct {
				name        string
				band, below image.Point
			}{
				{"rail", atBandOverRail, atRailTop},
				{"document", atBandOverDocument, atDocumentTop},
				{"rail seam", image.Pt(docsOutlineWidthDp-1, titleBandDp/2), image.Pt(docsOutlineWidthDp-1, titleBandDp)},
			} {
				if band, below := pixelAt(img, p.band), pixelAt(img, p.below); band != below {
					t.Errorf("title band over the %s is %v and the %s under it is %v; a painted window may not step at its own top edge",
						p.name, band, p.name, below)
				}
			}
		})
	}
}

// TestTheWindowShowsTheGuideAndNoTabStrip renders the window as every
// platform but macOS shows it, with the document at its top, and finds the
// guide's first heading at the window's top edge. The heading's rows are
// compared with a render of a document that holds that heading alone, in
// the same column, within the rasterizer's own spread: a strip or any other
// band above the page moves the heading down and the rows no longer agree. The rail's seam reaching the
// window's top row says the same for the leading column.
func TestTheWindowShowsTheGuideAndNoTabStrip(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	guide := guideFixture(t)
	blocks := markdown.Parse(guide)
	h, ok := blocks[0].(*markdown.Heading)
	if !ok {
		t.Fatalf("the fixture guide opens on %T, want its first heading", blocks[0])
	}
	headingOnly := []byte(strings.Repeat("#", h.Level) + " " + headingText(h) + "\n")
	st := outlineState{open: map[int]bool{0: true}, selected: -1}

	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			full := golden.Capture(t, windowSize, windowFrame(shaper, guide, st, tc.c, typo, 0, nil))
			alone := golden.Capture(t, windowSize, windowFrame(shaper, headingOnly, st, tc.c, typo, 0, nil))

			seam := vgcolor.Flatten(tc.c.Separator, tc.c.SidebarMaterial)
			if got := pixelAt(full, image.Pt(docsOutlineWidthDp-1, 0)); got != seam {
				t.Errorf("the window's top row at the rail's seam is %v, want the seam %v", got, seam)
			}

			fill := pageFill(tc.c)
			drawn := 0
			for y := 0; y < windowSize.Y; y++ {
				rowDrawn := false
				for x := docsOutlineWidthDp; x < windowSize.X; x++ {
					if pixelAt(alone, image.Pt(x, y)) != fill {
						rowDrawn = true
						break
					}
				}
				if !rowDrawn {
					continue
				}
				drawn++
				for x := docsOutlineWidthDp; x < windowSize.X; x++ {
					p := image.Pt(x, y)
					if got, want := pixelAt(full, p), pixelAt(alone, p); !nearColor(got, want) {
						t.Fatalf("the guide's first heading %q is not at the window's top: at %v the window draws %v, the heading alone %v",
							headingText(h), p, got, want)
					}
				}
			}
			if drawn == 0 {
				t.Fatal("the heading alone drew nothing in the document column; the comparison has no rows")
			}
		})
	}
}

// nearColor reports whether two pixels agree within the rasterizer's own
// spread: one glyph's edge, drawn twice, may land a level or two apart. A
// heading moved by even one row differs by far more at its edges.
func nearColor(a, b color.NRGBA) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= 4 && d(a.G, b.G) <= 4 && d(a.B, b.B) <= 4
}

// TestAFencedBlockHasAStepToStandOn checks the fence has a fill to stand on.
// The markdown style gives a fenced block the one step a document takes off
// its page, which says nothing at all if the page it lies on is that step
// itself. The frame is asked for the pixels rather than the intention.
func TestAFencedBlockHasAStepToStandOn(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			page := pixelAt(img, atDocument)
			style := docsMarkdownStyle(tc.c, tokens.DefaultTypography)

			fence := color.NRGBA{R: style.CodeBackground.R, G: style.CodeBackground.G, B: style.CodeBackground.B, A: 0xff}
			if fence == page {
				t.Fatalf("a fenced block fills %v on a page of %v; a code fence with no step has nothing to stand on", fence, page)
			}
			// And it is actually drawn: the document column between the rail
			// and the reading measure carries fence pixels.
			found := 0
			for y := titleBandDp; y < windowSize.Y; y++ {
				for x := docsOutlineWidthDp; x < windowSize.X; x++ {
					if pixelAt(img, image.Pt(x, y)) == fence {
						found++
					}
				}
			}
			if found == 0 {
				t.Errorf("no pixel of the fence fill %v anywhere in the document column; the page shows no fenced block to judge", fence)
			}

			// The quote block is marked rather than filled, and the mark
			// must not be the page either.
			bar := color.NRGBA{R: style.QuoteBar.R, G: style.QuoteBar.G, B: style.QuoteBar.B, A: 0xff}
			if bar == page {
				t.Errorf("a quote bar paints %v on a page of %v; the mark is invisible", bar, page)
			}
		})
	}
}
