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
// smoke test of the whole stack: a panic anywhere in the backdrop, the
// toolbar, the outline rail or the document fails it.
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
	"strconv"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Site Docs window opens at (main.go), and the
// only size these frames are drawn at: a composition is worth looking at
// where somebody actually looks at it.
var windowSize = image.Pt(windowW, windowH)

// toolbarDp is the depth of the toolbar's strip in pixels at 1x: the
// platform's toolbar. A go test binary has no live window, so
// desktop.TopInset reports 0 and the strip is the toolbar's own depth.
const toolbarDp = int(shell.ToolbarDepth)

// windowFrame composes the window for one scheme exactly as docsWindow
// stacks it: the backdrop first, then the toolbar's strip with the page on
// its fill below it. The page is the static twin the goldens draw
// (renderDocsPage), so the frame is the app's composition minus the streams.
//
// field is the search field's state; nil draws the field empty and without
// the keyboard. A field with text marks the guide and counts the matches,
// as the live window does.
func windowFrame(
	shaper *text.Shaper,
	guide []byte,
	st outlineState,
	c tokens.PlatformColors,
	typo tokens.Typography,
	field *input.RenderState,
) layout.Widget {
	var state input.RenderState
	if field != nil {
		state = *field
	}
	if state.Text != "" {
		state.Count = strconv.Itoa(guideMatchCount(guide, state.Text))
	}
	page := onPageFill(c, renderDocsPage(shaper, guide, st, c, typo, state.Text))
	background := backdrop.Widget(c.WindowBackground)
	strip := withToolbar(toolbarDepth, c, renderToolbarField(shaper, c, typo, state), page)
	return func(gtx layout.Context) layout.Dimensions {
		background(gtx)
		return strip(gtx)
	}
}

// guideMatchCount is the number of matches of query in the guide source.
func guideMatchCount(source []byte, query string) int {
	doc := markdown.NewDocument(markdown.Parse(source))
	doc.Find(query, 0)
	return len(doc.Matches())
}

// renderToolbarField is the static twin of the toolbar's search field: the
// chrome variant in a toolbar, on the page's fill, in the state given.
func renderToolbarField(shaper *text.Shaper, c tokens.PlatformColors, typo tokens.Typography, state input.RenderState) layout.Widget {
	state.Variant = input.Chrome
	state.Region = input.Toolbar
	state.Surface = pageFill(c)
	return input.RenderSearch(shaper, toolbarSearchPrompt, c, tokens.Spacing, tokens.Radius, typo.SystemFont, tokens.Comfortable, state)
}

// renderWindow draws the settled window — its first section disclosed and
// selected, which is the state a reader leaves this window in — at the app's
// own size.
func renderWindow(t *testing.T, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	guide := guideFixture(t)
	first := guideOutline(markdown.Parse(guide))[0]
	st := outlineState{open: map[int]bool{0: true}, selected: first.Block}
	w := windowFrame(shaper, guide, st, c, typo, nil)
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
// chosen clear of paint: the toolbar's strip over the rail and over the
// guide column before the search field, the rail under its last row, the
// document plane out past the reading measure the guide is capped to, and
// the first row under the strip on both sides of the rail's seam.
var (
	atBandOverRail     = image.Pt(150, toolbarDp/2)
	atBandOverDocument = image.Pt(600, toolbarDp/2)
	atRail             = image.Pt(150, 700)
	atRailTop          = image.Pt(150, toolbarDp)
	atDocument         = image.Pt(1100, 700)
	atDocumentTop      = image.Pt(600, toolbarDp)
)

// TestWholeWindowRender draws the composed window in both schemes, and
// writes the frames out when -window.dump names a directory.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
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
// sidebar is drawn in, and the toolbar's strip over each of them in that
// region's own fill.
//
// It is read off the rendered frame rather than off the set, because those
// regions are painted by different pieces of code (this app and backdrop)
// and the only place they can be seen agreeing is a frame that has all of
// them in it.
func TestWindowRegionsAreDrawnInThePlatformsFills(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"document plane", atDocument, tc.c.ControlBackground},
				{"outline rail", atRail, tc.c.SidebarMaterial},
				{"toolbar over the rail", atBandOverRail, tc.c.SidebarMaterial},
				{"toolbar over the guide column", atBandOverDocument, tc.c.ControlBackground},
			} {
				if got := pixelAt(img, r.at); got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
			}
		})
	}
}

// TestTheBandAgreesWithTheColumnsItCaps states the agreement directly: the
// strip's fill over each column is read off one frame and compared to the
// column's first row under the strip, so the rule holds wherever the fills
// under it move to. The rail's seam runs through the strip as well.
func TestTheBandAgreesWithTheColumnsItCaps(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			for _, p := range []struct {
				name        string
				band, below image.Point
			}{
				{"rail", atBandOverRail, atRailTop},
				{"document", atBandOverDocument, atDocumentTop},
				{"rail seam", image.Pt(docsOutlineWidthDp-1, toolbarDp/2), image.Pt(docsOutlineWidthDp-1, toolbarDp)},
			} {
				if band, below := pixelAt(img, p.band), pixelAt(img, p.below); band != below {
					t.Errorf("the toolbar over the %s is %v and the %s under it is %v; a painted window may not step at its own top edge",
						p.name, band, p.name, below)
				}
			}
		})
	}
}

// TestTheGuideStartsBelowTheToolbar finds the guide's first heading directly
// below the toolbar. The heading's rows are compared with a render of a
// document that holds that heading alone, in the same column, within the
// rasterizer's own spread: a tab strip or any other band between the toolbar
// and the page moves the heading down and the rows no longer agree. The
// rail's seam reaching the window's top row says that the rail's fill
// reaches the window's top edge.
func TestTheGuideStartsBelowTheToolbar(t *testing.T) {
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
			full := golden.Capture(t, windowSize, windowFrame(shaper, guide, st, tc.c, typo, nil))
			alone := golden.Capture(t, windowSize, windowFrame(shaper, headingOnly, st, tc.c, typo, nil))

			seam := vgcolor.Flatten(tc.c.Separator, tc.c.SidebarMaterial)
			if got := pixelAt(full, image.Pt(docsOutlineWidthDp-1, 0)); got != seam {
				t.Errorf("the window's top row at the rail's seam is %v, want the seam %v", got, seam)
			}

			fill := pageFill(tc.c)
			drawn := 0
			for y := toolbarDp; y < windowSize.Y; y++ {
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
						t.Fatalf("the guide's first heading %q is not directly below the toolbar: at %v the window draws %v, the heading alone %v",
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
			img := renderWindow(t, tc.c)
			page := pixelAt(img, atDocument)
			style := docsMarkdownStyle(tc.c, tokens.DefaultTypography)

			fence := color.NRGBA{R: style.CodeBackground.R, G: style.CodeBackground.G, B: style.CodeBackground.B, A: 0xff}
			if fence == page {
				t.Fatalf("a fenced block fills %v on a page of %v; a code fence with no step has nothing to stand on", fence, page)
			}
			// And it is actually drawn: the document column between the rail
			// and the reading measure carries fence pixels.
			found := 0
			for y := toolbarDp; y < windowSize.Y; y++ {
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

// TestTheToolbarHoldsTheSearchFieldAtItsTrailingEnd reads the toolbar off the
// window in both schemes against the reference rows. MEASURED at 1x: the
// toolbar's strip is 52 deep, 8 rows above a 36-row control and 8 below it.
// mail-window.png, 1200 wide as this window is, has its search recess at
// x 867–1191, 325 wide and 8 from the window's trailing edge.
// voicememos-window.png measures the recess's fill and rim: #e8e8e8 with no
// rim in light, #363636 with a #4d4d4d rim in dark. The outline rail's fill
// reaches the window's top edge, as the stored Mail capture shows.
func TestTheToolbarHoldsTheSearchFieldAtItsTrailingEnd(t *testing.T) {
	const (
		depth    = 52
		above    = 8
		height   = 36
		fieldX0  = 867
		fieldX1  = 1191
		trailing = 8
		// shadowLevels is how deep the field's shadow darkens the strip's
		// fill beside the field, in levels of 255.
		shadowLevels = 12
	)
	if toolbarDp != depth {
		t.Fatalf("the toolbar is %d deep, want the measured %d", toolbarDp, depth)
	}
	for _, tc := range []struct {
		name       string
		c          tokens.PlatformColors
		fill, edge color.NRGBA
	}{
		{"light", tokens.PlatformLight, color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}, color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}},
		{"dark", tokens.PlatformDark, color.NRGBA{R: 0x36, G: 0x36, B: 0x36, A: 0xff}, color.NRGBA{R: 0x4d, G: 0x4d, B: 0x4d, A: 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			page := pageFill(tc.c)

			// A column through the field clear of its looking glass and its
			// prompt: the strip's fill, the field's edge, its fill, its edge,
			// and the strip's fill again to the strip's foot. Outside the
			// field the strip's fill carries the field's measured shadow, a
			// few levels deep, so those rows are read to a tolerance.
			x := 1100
			for y := 0; y < depth; y++ {
				got := pixelAt(img, image.Pt(x, y))
				switch {
				case y == above || y == above+height-1:
					if got != tc.edge {
						t.Errorf("the field's edge at (%d,%d) is %v, want %v", x, y, got, tc.edge)
					}
				case y > above && y < above+height-1:
					if got != tc.fill {
						t.Errorf("the field's fill at (%d,%d) is %v, want %v", x, y, got, tc.fill)
					}
				default:
					if !near(got, page, shadowLevels) {
						t.Errorf("the strip at (%d,%d) is %v, want the page's fill %v", x, y, got, page)
					}
				}
			}

			// The field's middle row: where the field begins and ends.
			y := above + height/2
			first, last := -1, -1
			for x := docsOutlineWidthDp; x < windowSize.X; x++ {
				if !near(pixelAt(img, image.Pt(x, y)), page, shadowLevels) {
					if first < 0 {
						first = x
					}
					last = x
				}
			}
			if first != fieldX0 || last != fieldX1 {
				t.Errorf("the field's middle row runs x %d–%d (%d wide), want x %d–%d (%d wide)",
					first, last, last-first+1, fieldX0, fieldX1, fieldX1-fieldX0+1)
			}
			if got := windowSize.X - 1 - last; got != trailing {
				t.Errorf("the field ends %d from the window's trailing edge, want %d", got, trailing)
			}

			if got := pixelAt(img, image.Pt(docsOutlineWidthDp/2, 0)); got != tc.c.SidebarMaterial {
				t.Errorf("the rail's top row is %v, want its fill %v", got, tc.c.SidebarMaterial)
			}
		})
	}
}
