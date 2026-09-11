package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. The app is a native window binary with no offscreen mode of
// its own, but every layer it stacks is a plain layout.Widget over
// pre-resolved tokens, so composing the same three paints into a headless
// image at the size the window opens at produces the frame the window would
// show.
//
// The per-tab goldens beside this file render one tab's content onto a frame
// of their own and cannot see that the document is being read on the same
// level as the rail indexing it; this can. Run it with -window.dump=<dir> to
// write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/sitedocs
//
// Without the flag it still renders both schemes every run, which makes it a
// smoke test of the whole stack: a panic anywhere in the backdrop, the title
// band, the tab strip, the outline rail or the document fails it.
//
// The assertions sample the rendered frame rather than a struct of colours,
// because this app holds none: each region paints its own fill at the point
// it draws, and the frame is the only place the question "what is this region
// wearing" has an answer that sees what was painted rather than what was
// meant.

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/tabs"
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
// stacks it: the backdrop first, then the tab shell inset under a title-bar
// strip painted in the fill of the region it caps. The shell is the static
// twin the goldens already draw (staticTabs over tabs.Render), so the frame
// is the app's composition minus the streams.
//
// band is the strip height to render under; 0 draws the window as every
// platform but macOS shows it, with the shell at the window's own top edge.
func windowFrame(
	shaper *text.Shaper,
	guide []byte,
	st outlineState,
	c tokens.PlatformColors,
	typo tokens.Typography,
	selected int,
	band unit.Dp,
) layout.Widget {
	props := tabs.Props{Tabs: staticTabs(shaper, guide, st, c, typo), Shaper: shaper}
	shell := tabs.Render(shaper, props, selected, c, tokens.Spacing, typo.LabelLarge, tokens.Comfortable)
	background := backdrop.Widget(c.WindowBackground)
	capped := bandedCap(func() unit.Dp { return band }, titleBandFill(c), shell)
	return func(gtx layout.Context) layout.Dimensions {
		background(gtx)
		return capped(gtx)
	}
}

// renderWindow draws the settled window — the Docs tab open, its first
// section disclosed and selected, which is the state a reader leaves this
// window in — at the app's own size.
func renderWindow(t *testing.T, c tokens.PlatformColors, band unit.Dp) *image.RGBA {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	guide := guideFixture(t)
	first := guideOutline(markdown.Parse(guide))[0]
	st := outlineState{open: map[int]bool{0: true}, selected: first.Block}
	w := windowFrame(shaper, guide, st, c, typo, tabIndex(pageDocs), band)
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
// chosen clear of paint: the title band right of the window title, the tab
// strip right of the last cell, and the document plane out past the reading
// measure the guide is capped to.
var (
	atTitleBand = image.Pt(1100, titleBandDp/2)
	atTabStrip  = image.Pt(1100, titleBandDp+18)
	atRail      = image.Pt(150, 700)
	atDocument  = image.Pt(1100, 700)
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

// TestWindowRegionsWearThePlatformsFills reads the assignment off the frame:
// the guide document — the thing this window exists to show — on the
// platform's content plane, the outline rail indexing it in the chrome
// material a sidebar wears, and the tab strip capping the panel in that same
// material.
//
// It is read off the rendered frame rather than off the set, because those
// three are painted by three different pieces of code (this app, backdrop,
// and patterns/tabs) and the only place they can be seen agreeing is a frame
// that has all three in it.
func TestWindowRegionsWearThePlatformsFills(t *testing.T) {
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
				{"tab strip", atTabStrip, tc.c.SidebarMaterial},
				// The band wears the fill of the region under it, which here
				// is the tab strip rather than the document.
				{"title band", atTitleBand, tc.c.SidebarMaterial},
			} {
				if got := pixelAt(img, r.at); got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
			}
		})
	}
}

// TestTheBandAgreesWithTheStripItCaps states the agreement directly: the two
// fills are read off one frame and compared to each other, so the rule holds
// wherever the fill under both of them moves to.
//
// Only the agreement is asserted, and not that the band differs from the
// document: the chrome material the strip carries is the content's own fill
// exactly in the light appearance, so on this platform those two are one
// colour there and a test demanding a step would be demanding a colour the
// platform does not draw.
func TestTheBandAgreesWithTheStripItCaps(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			band, strip := pixelAt(img, atTitleBand), pixelAt(img, atTabStrip)
			if band != strip {
				t.Errorf("title band is %v and the tab strip it caps is %v; a painted window may not step at its own top edge",
					band, strip)
			}
		})
	}
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
