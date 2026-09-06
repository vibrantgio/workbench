package main

// A whole-window render, headless, of the launcher as its layers actually
// stack: the Background fill, and the page held down past the title-bar
// strip a full-size-content window opens the top of itself into. Every layer
// the window stacks is a plain layout.Widget over pre-resolved tokens, so
// composing the same paints into a headless image at the size the window
// opens at produces the frame the window would show.
//
// Run it with -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/workbench
//
// The assertions read a frame drawn without the animated seen triangle field:
// the field is driven by the clock, so it has no one frame to store, and with
// it gone every pixel that is not the Background pin is paint the page put
// there, which is what makes a claim about the strip readable. The dumped
// frames carry the field, since a composition shown to a reviewer without its
// most prominent layer is not the window.

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// titleBandDp is the strip desktop.TopInset reports on a full-size-content
// macOS window, stated here because a go test binary has no live window to
// measure. The number is the stored reference's plain title bar band, 32 px.
const titleBandDp = 32

// windowFrame composes the window for one scheme exactly as buildLayers stacks
// it, minus the field: the background fill full-bleed to the window's top
// edge, then the page under the strip that desktop.CapTop holds open and
// claims.
//
// band is the strip height to render under; 0 draws the window as every
// platform but macOS shows it, with the page at the window's own top edge.
func windowFrame(tok themed, model Model, band unit.Dp) layout.Widget {
	return stack(backdrop.Widget(tok.color.Background), cappedPage(tok, model, band))
}

// cappedPage is the window's content layer as buildLayers wraps it: the page
// held down past a strip of the given height, with that same strip claimed for
// the window's drag.
func cappedPage(tok themed, model Model, band unit.Dp) layout.Widget {
	return desktop.CapTop(func() unit.Dp { return band }, pageContent(tok, model))
}

// stack draws the given layers back to front into one layout.Widget and
// reports the frontmost one's dimensions, which is what theme/window's own
// layer stack does with the observables buildLayers hands it.
func stack(layers ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var dims layout.Dimensions
		for _, w := range layers {
			dims = w(gtx)
		}
		return dims
	}
}

func renderWindow(t *testing.T, colors tokens.ColorTokens, band unit.Dp) *image.RGBA {
	t.Helper()
	return golden.Capture(t, windowFrameSize, windowFrame(roundedThemed(colors), Model{}, band))
}

// roundedThemed is the goldens' frozen snapshot with the theme's real radius
// scale put back. The goldens pin radius to zero so a pixel-exact diff can
// survive GPU-dependent corner anti-aliasing; these frames store nothing and
// are drawn to be looked at, so they keep the real corners.
func roundedThemed(colors tokens.ColorTokens) themed {
	tok := staticThemed(colors)
	tok.components.Radius = rx.Of(tokens.Radius)
	return tok
}

// livingWindowFrame is windowFrame with the triangle field in the middle of
// the stack where the running window has it. The assertions do not read it:
// the field's vertices are displaced from a clock, so two captures are two
// pictures.
//
// An app.Window that is never run absorbs the invalidations the field uses to
// drive its frames, which is all a single capture needs — it takes the scene
// as the constructor left it. The palette is applied here rather than left to
// the animation tick, or the capture would photograph the pre-theme
// placeholder.
func livingWindowFrame(tok themed, model Model, band unit.Dp) layout.Widget {
	field := NewField(new(app.Window), winW, winH)
	field.SetColors(tok.color)
	field.applyPending()
	return stack(backdrop.Widget(tok.color.Background), field.Widget(), cappedPage(tok, model, band))
}

// windowSchemes is the pair every rule below is checked against.
var windowSchemes = []struct {
	name   string
	colors tokens.ColorTokens
}{
	{"light", tokens.DefaultLight},
	{"dark", tokens.DefaultDark},
}

// pixelAt reads one pixel as an opaque NRGBA, which is what every token in the
// set is.
func pixelAt(img *image.RGBA, p image.Point) color.NRGBA {
	r, g, b, _ := img.At(p.X, p.Y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
}

// drawnSpan reports the first and last column of row y carrying a pixel that
// is not the fill it is given, or (-1, -1) for a row that is all that fill.
func drawnSpan(img *image.RGBA, y int, fill color.NRGBA) (int, int) {
	first, last := -1, -1
	size := img.Bounds().Size()
	for x := 0; x < size.X; x++ {
		if pixelAt(img, image.Pt(x, y)) != fill {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return first, last
}

// levelTolerance is how far a pixel read back out of the frame may sit from
// the token painted into it and still count as a level at all. Gio blends in
// linear space, so a flat fill does not always survive the round trip to 8-bit
// sRGB exactly: in the dark scheme, at the bottom of the curve where the
// quantisation is coarsest, a level-1 fill comes back speckled a value or two
// above its own token.
//
// It is a membership test only, never a discriminator: the light scheme fills
// its raised and both its floating levels white, so a first-match walk would
// hand every fill to whichever of them it met first. [nearestLevel] takes the
// closest level instead, which stays decidable wherever the fills differ at
// all.
const levelTolerance = 4

// nearestLevel reports the elevation level a rendered pixel sits on — the one
// whose surface fill it is closest to, if that fill is within levelTolerance —
// and whether it is a surface fill at all rather than foreground drawn on it.
//
// The walk includes the chrome level: a window's chrome stands there, so
// a classifier covering only the four levels from the content up would
// report a sidebar as no level at all. The backdrop is left out — nothing
// is drawn at it, so no rendered pixel belongs to it.
func nearestLevel(c color.NRGBA, colors tokens.ColorTokens) (tokens.ElevationLevel, bool) {
	best, dist := tokens.Level0, levelTolerance+1
	for _, level := range []tokens.ElevationLevel{tokens.LevelChrome, tokens.Level0, tokens.Level1, tokens.Level2, tokens.Level3} {
		if d := levelDistance(c, colors.SurfaceAt(level)); d < dist {
			best, dist = level, d
		}
	}
	return best, dist <= levelTolerance
}

// levelDistance is the largest per-channel gap between a rendered pixel and a
// level's token — the distance levelTolerance is stated in.
func levelDistance(a, b color.NRGBA) int {
	d := channelDiff(a.R, b.R)
	if g := channelDiff(a.G, b.G); g > d {
		d = g
	}
	if bl := channelDiff(a.B, b.B); bl > d {
		d = bl
	}
	return d
}

func channelDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// cellRects measures the app groups off the frame instead of recomputing the
// layout's arithmetic. A grid row is a contiguous stretch of frame rows whose
// paint is exactly as wide as some number of cells with their gaps — one to
// perRow of them — and at least half a cell tall; the hero's lines are the
// only other paint on the page and match neither. The last row holds the
// roster's remainder, so it may be narrower than GridW and is read for how
// many cells it actually holds. Each stretch is shortened top and bottom by
// the same corner radius, so its midpoint is the cells' midpoint.
func cellRects(t *testing.T, img *image.RGBA, fill color.NRGBA) []image.Rectangle {
	t.Helper()
	size := img.Bounds().Size()
	var rects []image.Rectangle
	for y := 0; y < size.Y; y++ {
		first, last := drawnSpan(img, y, fill)
		n := cellsAcross(last - first + 1)
		if n == 0 {
			continue
		}
		top := y
		for y+1 < size.Y {
			f, l := drawnSpan(img, y+1, fill)
			if f != first || cellsAcross(l-f+1) != n {
				break
			}
			y++
		}
		if y-top+1 < int(CellH)/2 {
			continue
		}
		mid := (top + y) / 2
		for c := 0; c < n && len(rects) < len(Apps); c++ {
			x := first + c*(int(CellW)+int(RowGap))
			rects = append(rects, image.Rect(x, mid-int(CellH)/2, x+int(CellW), mid+int(CellH)/2))
		}
	}
	return rects
}

// cellsAcross reports how many cells a painted span that wide holds, and zero
// when it is no row of cells. A group's hairline lies wholly inside the
// bounds it draws, so a row of n cells paints exactly the n cells' own width;
// the couple of pixels of slack absorb the corner radius's antialiasing.
func cellsAcross(width int) int {
	for n := 1; n <= perRow; n++ {
		w := n*int(CellW) + (n-1)*int(RowGap)
		if width >= w-1 && width <= w+2 {
			return n
		}
	}
	return 0
}

// topmostDrawn reports the first row of the frame carrying a pixel that is
// not the fill it is given, and how many rows were scanned when there is none.
func topmostDrawn(img *image.RGBA, fill color.NRGBA) int {
	size := img.Bounds().Size()
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if pixelAt(img, image.Pt(x, y)) != fill {
				return y
			}
		}
	}
	return size.Y
}

// TestWholeWindowRender draws the composed window in both schemes and writes
// the frames out when -window.dump names a directory. Without the flag it is
// still a smoke test of the whole stack: a panic anywhere in the background
// fill, the strip, the hero or the app grid fails it. The dumped frames carry
// the field as well.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			if img.Bounds().Size() != windowFrameSize {
				t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowFrameSize)
			}
			if *windowDump == "" {
				return
			}
			img = golden.Capture(t, windowFrameSize, livingWindowFrame(roundedThemed(tc.colors), Model{}, titleBandDp))
			if err := os.MkdirAll(*windowDump, 0o755); err != nil {
				t.Fatalf("dump dir: %v", err)
			}
			path := filepath.Join(*windowDump, "workbench-"+tc.name+".png")
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

// TestTheBackdropReachesTheWindowsTopEdge pins that the strip shows the
// full-bleed backdrop already painted under it rather than a second fill
// drawn over it, which is why nothing in this app paints a band. The strip
// must be that fill and nothing else: no page paint in it, and no unpainted
// glass.
func TestTheBackdropReachesTheWindowsTopEdge(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			content := tc.colors.SurfaceAt(tokens.Level0)
			if content != tc.colors.Background {
				t.Fatalf("level 0 resolves to %v and the Background layer paints %v", content, tc.colors.Background)
			}
			for _, x := range []int{0, windowFrameSize.X / 2, windowFrameSize.X - 1} {
				for _, y := range []int{0, titleBandDp / 2, titleBandDp - 1} {
					if got := pixelAt(img, image.Pt(x, y)); got != content {
						t.Errorf("strip pixel at (%d,%d) = %v, want the window's background fill %v", x, y, got, content)
					}
				}
			}
		})
	}
}

// TestThePageStartsBelowTheStrip is the other half: the background fill runs
// under the strip, the page does not. Read off the frame rather than off the
// inset, so that a page which grew a layer of its own outside the cap would
// fail here.
func TestThePageStartsBelowTheStrip(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			if top := topmostDrawn(img, tc.colors.SurfaceAt(tokens.Level0)); top < titleBandDp {
				t.Errorf("the page paints row %d, inside the %d dp title-bar strip; only the background fill belongs there", top, titleBandDp)
			}
		})
	}
}

// TestThePageClearsTheWindowButtons: with the native strip gone the platform's
// three control buttons float over the top-leading corner of whatever this
// window drew there. The background fill owes them nothing but its own
// colour, but the page must not reach into their run, which is taken from
// desktop's derivation of the platform's rule rather than from a guess at
// where the circles are.
//
// The margin is generous on purpose: the page is centred in the window, so the
// distance between its topmost paint and the buttons is not a tuned number
// and a regression that ate the inset would close it entirely.
func TestThePageClearsTheWindowButtons(t *testing.T) {
	run := desktop.ButtonRunIn(titleBandDp)
	bottom := int(run.Leading + run.Diameter)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			content := tc.colors.SurfaceAt(tokens.Level0)
			for y := 0; y <= bottom; y++ {
				for x := 0; x <= int(run.Trailing); x++ {
					if got := pixelAt(img, image.Pt(x, y)); got != content {
						t.Fatalf("page paint %v at (%d,%d), inside the window buttons' run (leading %v, trailing %v, centre %v)",
							got, x, y, run.Leading, run.Trailing, run.Center)
					}
				}
			}
			if top := topmostDrawn(img, content); top <= bottom {
				t.Errorf("the page's topmost paint is row %d and the buttons end at row %d; the page has no clearance under them", top, bottom)
			}
		})
	}
}

// TestTheAppGroupsTakeThePagesOwnFill reads off the frame that the launcher's
// roster divides the page rather than standing over it: every cell is a
// group, so its inside is the window's own page, byte for byte, and the
// hairline at its edge is the whole of what says where one app ends and the
// next begins. Checked in both schemes, because a group is derived against
// whatever surface it is in and neither scheme is the reference one.
//
// The claim is the grammar, not a colour: nothing here names a step. It asks
// elevation which level it painted and requires the page's own, and it asks
// the palette for the seam of two regions sharing that fill
// ([tokens.ColorTokens.SeamOn]) and requires the edge to be it.
func TestTheAppGroupsTakeThePagesOwnFill(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			page := tokens.Level0
			content := tc.colors.SurfaceAt(page)
			cells := cellRects(t, img, content)
			if len(cells) != len(Apps) {
				t.Fatalf("measured %d cells in the frame, want one per app in the roster (%d)", len(cells), len(Apps))
			}
			for i, r := range cells {
				at := image.Pt(r.Min.X+int(tokens.Spacing.S4)/2, (r.Min.Y+r.Max.Y)/2)
				if got := pixelAt(img, at); got != content {
					t.Errorf("group %d fills %v at %v; a group takes the fill of the surface it is in, which is the level-%d page %v",
						i, got, at, page, content)
				}

				// One pixel is a spot check; the fill is the claim. Count the
				// whole cell, so a raised body inside a hairline would fail
				// here even though the sample above passed.
				covered := map[tokens.ElevationLevel]int{}
				for y := r.Min.Y; y < r.Max.Y; y++ {
					for x := r.Min.X; x < r.Max.X; x++ {
						if level, ok := nearestLevel(pixelAt(img, image.Pt(x, y)), tc.colors); ok {
							covered[level]++
						}
					}
				}
				widest, n := tokens.Level0, -1
				for level, count := range covered {
					if count > n || (count == n && level < widest) {
						widest, n = level, count
					}
				}
				if area := r.Dx() * r.Dy(); widest != page || n*2 < area {
					t.Errorf("group %d is covered by level %d over %d%% of itself; a group raises nothing, so it is the level-%d fill throughout",
						i, widest, n*100/area, page)
				}

				// The hairline is the whole of what says the group is there,
				// so it is read off the frame too: the leading edge at mid
				// height, where the corner radius is behind and the edge is
				// one straight column.
				edge := image.Pt(r.Min.X, (r.Min.Y+r.Max.Y)/2)
				if got, want := pixelAt(img, edge), tc.colors.SeamOn(content); got != want {
					t.Errorf("group %d draws %v at its leading edge %v, not the seam %v the two regions sharing the page's fill are parted by",
						i, got, edge, want)
				}

				// The plane has to be the level-0 fill, or "takes the page's
				// own fill" is a claim about nothing. Read it back off the page
				// beside the cell rather than trusting the Background layer's token.
				beside := image.Pt(r.Max.X+int(RowGap)/2, (r.Min.Y+r.Max.Y)/2)
				if level, ok := nearestLevel(pixelAt(img, beside), tc.colors); !ok || level != page {
					t.Errorf("the page beside group %d reads %v at %v, not the level-%d fill %v",
						i, pixelAt(img, beside), beside, page, content)
				}
			}
		})
	}
}

// TestTheStripMovesThePage guards the inset itself: a frame drawn under the
// strip is not the frame drawn without one. Without this the tests above would
// all still pass if the cap stopped insetting anything, since a centred page
// clears a 32 dp strip on its own.
func TestTheStripMovesThePage(t *testing.T) {
	capped := renderWindow(t, tokens.DefaultLight, titleBandDp)
	bare := renderWindow(t, tokens.DefaultLight, 0)
	if n := golden.PixelDiff(capped, bare); n == 0 {
		t.Error("the window renders identically with and without a title-bar strip; the page is not being inset")
	}
}
