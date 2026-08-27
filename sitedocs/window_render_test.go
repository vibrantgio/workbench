package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. The app is a native window binary with no offscreen mode of
// its own, but every layer it stacks is a plain widget over pre-resolved
// tokens, so composing the same three paints into a headless canvas at the
// size the window opens at produces the frame the window would show.
//
// It is here because a composition can only be judged as a composition. The
// per-tab goldens beside this file render one tab's content onto a canvas of
// their own and cannot see that the document is being read on the same rung
// as the rail indexing it; this can. Run it with -window.dump=<dir> to write
// the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/sitedocs
//
// Without the flag it still renders both schemes every run, which makes it a
// smoke test of the whole stack: a panic anywhere in the backdrop, the title
// band, the tab strip, the outline rail or the document fails it.
//
// The assertions sample the rendered frame rather than a palette struct,
// because this app holds no palette: each region paints its own fill at the
// point it draws, and the frame is the only place the question "what rung is
// this region wearing" has an answer that sees what was painted rather than
// what was meant.

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
// measure — the same substitution drag_test.go makes. The number is the
// stored reference's plain title bar band (ADR-019: 32 px, fill y0–30), not
// a guess and not a live measurement.
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
	c tokens.ColorTokens,
	typo tokens.Typography,
	selected int,
	band unit.Dp,
) layout.Widget {
	props := tabs.Props{Tabs: staticTabs(shaper, guide, st, c, typo), Shaper: shaper}
	shell := tabs.Render(shaper, props, selected, c, tokens.Spacing, typo.LabelLarge, tokens.Comfortable)
	ground := backdrop.Widget(c.Background)
	capped := bandedCap(func() unit.Dp { return band }, titleBandFill(c), shell)
	return func(gtx layout.Context) layout.Dimensions {
		ground(gtx)
		return capped(gtx)
	}
}

// renderWindow draws the settled window — the Docs tab open, its first
// section disclosed and selected, which is the state a reader leaves this
// window in — at the app's own size.
func renderWindow(t *testing.T, c tokens.ColorTokens, band unit.Dp) *image.RGBA {
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
	c    tokens.ColorTokens
}{
	{"light", tokens.DefaultLight},
	{"dark", tokens.DefaultDark},
}

// Sample points in the rendered window, in the pixels the frame is drawn at
// (PxPerDp is 1, so a dp is a pixel). Each names a resting expanse and is
// chosen clear of ink: the title band right of the window title, the tab
// strip right of the last cell, the gap the shell keeps under the strip, the
// outline rail below its last row, and the document plane out past the
// reading measure the guide is capped to.
var (
	atTitleBand = image.Pt(1100, titleBandDp/2)
	atTabStrip  = image.Pt(1100, titleBandDp+18)
	atStripGap  = image.Pt(1100, titleBandDp+36+8)
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

// TestWindowRegionsWearTheirRungs reads the surface grammar's assignment off
// the frame: the guide document — the thing this window exists to show — on
// the paper at level 0, the outline rail indexing it on the FLOOR under that
// paper, the tab strip raised over the panel it caps, and nothing resting at
// level 2.
//
// Before AK6.4 the window had no level-0 surface at all: the backdrop filled
// it with Surface and patterns/tabs painted strip and panel alike in Surface
// over that, so the document was read on furniture and the outline rail
// beside it stood level with what it indexes. ADR-022 then moved the rail the
// other way. It had been called furniture and given the storey ABOVE the
// paper, and furniture is the desk the document lies on: on paper the fill
// does not move (the floor is the same neutral 200 the rail already wore) and
// on slate it drops from #222222 to #0C0C0C.
//
// The strip does not follow it, and the difference is the whole of what this
// test is now worth reading for. A rail is chrome standing beside the
// document; a tab strip is the panel's own control band, drawn one storey
// over the panel it belongs to (patterns/tabs walks it from Props.Ground).
// So this one window carries a region below the paper and a region above it,
// and the two are named apart here rather than lumped as "furniture".
func TestWindowRegionsWearTheirRungs(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			floor := tc.c.SurfaceAt(tokens.LevelFloor)
			ground := tc.c.SurfaceAt(tokens.Level0)
			raised := tc.c.SurfaceAt(tokens.Level1)
			transient := tc.c.SurfaceAt(tokens.Level2)

			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"document plane", atDocument, ground},
				{"strip gap", atStripGap, ground},
				{"outline rail", atRail, floor},
				{"tab strip", atTabStrip, raised},
				// R6: the band wears the fill of the region under it, which
				// here is the tab strip rather than the document.
				{"title band", atTitleBand, raised},
			} {
				got := pixelAt(img, r.at)
				if got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
				if got == transient {
					t.Errorf("%s at %v rests at level 2 (%v), the rung the ladder keeps for what appears and leaves",
						r.name, r.at, transient)
				}
			}
		})
	}
}

// TestLightnessClimbsTowardTheViewer is ADR-022's own check taken along this
// window's depth axis rather than across its plane: the outline rail is the
// desk, the guide is the paper laid on it, the tab strip is the panel's band
// raised over that paper, and a dialog would arrive over the lot. Walking
// that order toward the reader, lightness may only increase — in the light
// scheme AND in the dark one, which is why this needs no per-scheme clause.
//
// It is read off the rendered frame for the rail, the page and the strip,
// because those three are painted by three different pieces of code (this
// app, backdrop, and patterns/tabs) and the only place they can be seen
// agreeing is a frame that has all three in it.
func TestLightnessClimbsTowardTheViewer(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			toward := []struct {
				name string
				fill color.NRGBA
			}{
				{"the outline rail's floor", pixelAt(img, atRail)},
				{"the guide's paper", pixelAt(img, atDocument)},
				{"the tab strip's band", pixelAt(img, atTabStrip)},
				{"a dialog's surface", tc.c.SurfaceAt(tokens.Level2)},
			}
			for i := 1; i < len(toward); i++ {
				below, above := toward[i-1], toward[i]
				if luma(above.fill) <= luma(below.fill) {
					t.Errorf("%s (%v) is not lighter than %s (%v); walking toward the viewer never gets darker",
						above.name, above.fill, below.name, below.fill)
				}
			}
			// The composition corollary, stated as the picture it is: the
			// furniture is this window's darkest region.
			for _, other := range toward[1:] {
				if luma(toward[0].fill) >= luma(other.fill) {
					t.Errorf("the outline rail (%v) is not darker than %s (%v); a window's furniture is its darkest region",
						toward[0].fill, other.name, other.fill)
				}
			}
		})
	}
}

// luma is the Rec. 601 brightness of a fill, the axis "lighter" and "darker"
// are measured on above.
func luma(c color.NRGBA) float32 {
	return 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
}

// TestTheBandAgreesWithTheStripItCaps is R6 stated as the agreement it
// actually is: the two fills are read off one frame and compared to each
// other, so the rule holds even if the rung under both of them moves.
func TestTheBandAgreesWithTheStripItCaps(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, titleBandDp)
			band, strip := pixelAt(img, atTitleBand), pixelAt(img, atTabStrip)
			if band != strip {
				t.Errorf("title band is %v and the tab strip it caps is %v; a painted window may not step at its own top edge",
					band, strip)
			}
			if doc := pixelAt(img, atDocument); band == doc {
				t.Errorf("title band and document plane are both %v; the band wears the fill of the region under IT, not of the window's content",
					band)
			}
		})
	}
}

// TestARaisedInsetHasAStepToStandOn is the tell the audit read the whole
// inversion off. The markdown style gives a fenced block neutral 200 — "the
// step off the page" — which said nothing at all while the page it lay on
// was neutral 200 itself. On the window ground the fence is a step, and the
// frame is asked for the pixels rather than the intention.
func TestARaisedInsetHasAStepToStandOn(t *testing.T) {
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

			// The quote block is marked rather than filled: its bar is the
			// Primary ink, which must not be the page either.
			bar := color.NRGBA{R: style.QuoteBar.R, G: style.QuoteBar.G, B: style.QuoteBar.B, A: 0xff}
			if bar == page {
				t.Errorf("a quote bar inks %v on a page of %v; the mark is invisible", bar, page)
			}
		})
	}
}
