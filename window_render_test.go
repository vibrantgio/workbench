package main

// A whole-window render, headless, of the launcher as its layers actually
// stack: the Background ground, and the page held down past the title-bar
// strip a full-size-content window opens the top of itself into. Every layer
// the window stacks is a plain widget over pre-resolved tokens, so composing
// the same paints into a headless canvas at the size the window opens at
// produces the frame the window would show.
//
// Run it with -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/workbench
//
// The assertions read a frame drawn without the animated seen triangle field:
// the field is driven by the clock, so it has no one frame to store, and with
// it gone every pixel that is not the Background pin is ink the page put
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
// it, minus the field: the ground full-bleed to the window's top edge, then
// the page under the strip that desktop.CapTop holds open and claims.
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

// stack draws the given layers back to front into one widget and reports the
// frontmost one's dimensions, which is what theme/window's own layer stack
// does with the observables buildLayers hands it.
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
	return golden.Capture(t, windowCanvasSize, windowFrame(roundedThemed(colors), Model{}, band))
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

// inkSpan reports the first and last column of row y carrying a pixel that is
// not the ground, or (-1, -1) for a row that is all ground.
func inkSpan(img *image.RGBA, y int, ground color.NRGBA) (int, int) {
	first, last := -1, -1
	size := img.Bounds().Size()
	for x := 0; x < size.X; x++ {
		if pixelAt(img, image.Pt(x, y)) != ground {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return first, last
}

// rungTolerance is how far a pixel read back out of the frame may sit from the
// token painted into it and still count as a level at all. Gio blends in
// linear space, so a flat fill does not always survive the round trip to 8-bit
// sRGB exactly: in the dark scheme, at the bottom of the curve where the
// quantisation is coarsest, a level-1 card comes back speckled a value or two
// above its own token.
//
// It is a membership test only, never a discriminator: the light scheme fills
// its raised and both its floating levels white, so a first-match walk would
// hand every card to whichever of them it met first. [nearestRung] takes the
// closest level instead, which stays decidable wherever the fills differ at
// all.
const rungTolerance = 4

// nearestRung reports the elevation level a rendered pixel sits on — the one
// whose surface fill it is closest to, if that fill is within rungTolerance —
// and whether it is a surface fill at all rather than ink drawn on one.
//
// The walk includes the chrome level: a window's furniture stands there, so
// a classifier covering only the four levels from the content up would
// report a sidebar as no level at all. The backdrop is left out — nothing
// is drawn at it, so no rendered pixel belongs to it.
func nearestRung(c color.NRGBA, colors tokens.ColorTokens) (tokens.ElevationLevel, bool) {
	best, dist := tokens.Level0, rungTolerance+1
	for _, level := range []tokens.ElevationLevel{tokens.LevelChrome, tokens.Level0, tokens.Level1, tokens.Level2, tokens.Level3} {
		if d := rungDistance(c, colors.SurfaceAt(level)); d < dist {
			best, dist = level, d
		}
	}
	return best, dist <= rungTolerance
}

// rungDistance is the largest per-channel gap between a rendered pixel and a
// level's token — the distance rungTolerance is stated in.
func rungDistance(a, b color.NRGBA) int {
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

// cardRects measures the app cards off the frame instead of recomputing the
// layout's arithmetic. A card row is a contiguous stretch of frame rows whose
// ink is exactly as wide as some number of cards with their gaps — one to
// perRow of them — and at least half a card tall; the hero's lines are the
// only other ink on the page and match neither. The last row holds the
// roster's remainder, so it may be narrower than GridW and is read for how
// many cards it actually holds. Each stretch is shortened top and bottom by
// the same corner radius, so its midpoint is the cards' midpoint. The
// outlined card's 1 dp stroke straddles the edge it draws, which is why the
// samples below are taken half an S4 inside the measured leading edge rather
// than on it.
func cardRects(t *testing.T, img *image.RGBA, ground color.NRGBA) []image.Rectangle {
	t.Helper()
	size := img.Bounds().Size()
	var rects []image.Rectangle
	for y := 0; y < size.Y; y++ {
		first, last := inkSpan(img, y, ground)
		n := cardsAcross(last - first + 1)
		if n == 0 {
			continue
		}
		top := y
		for y+1 < size.Y {
			f, l := inkSpan(img, y+1, ground)
			if f != first || cardsAcross(l-f+1) != n {
				break
			}
			y++
		}
		if y-top+1 < int(CardH)/2 {
			continue
		}
		mid := (top + y) / 2
		for c := 0; c < n && len(rects) < len(Apps); c++ {
			x := first + c*(int(CardW)+int(RowGap))
			rects = append(rects, image.Rect(x, mid-int(CardH)/2, x+int(CardW), mid+int(CardH)/2))
		}
	}
	return rects
}

// cardsAcross reports how many cards an ink span that wide holds, and zero
// when it is no row of cards. The outlined card's stroke straddles its edge,
// so a row of n cards inks up to two pixels wider than the n cards' own
// width; a pixel under it is antialiasing.
func cardsAcross(width int) int {
	for n := 1; n <= perRow; n++ {
		w := n*int(CardW) + (n-1)*int(RowGap)
		if width >= w-1 && width <= w+2 {
			return n
		}
	}
	return 0
}

// topmostInk reports the first row of the frame carrying a pixel that is not
// the ground, and how many rows were scanned when there is none.
func topmostInk(img *image.RGBA, ground color.NRGBA) int {
	size := img.Bounds().Size()
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if pixelAt(img, image.Pt(x, y)) != ground {
				return y
			}
		}
	}
	return size.Y
}

// TestWholeWindowRender draws the composed window in both schemes and writes
// the frames out when -window.dump names a directory. Without the flag it is
// still a smoke test of the whole stack: a panic anywhere in the ground, the
// strip, the hero or the card grid fails it. The dumped frames carry the field
// as well.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			if img.Bounds().Size() != windowCanvasSize {
				t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowCanvasSize)
			}
			if *windowDump == "" {
				return
			}
			img = golden.Capture(t, windowCanvasSize, livingWindowFrame(roundedThemed(tc.colors), Model{}, titleBandDp))
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

// TestTheGroundReachesTheWindowsTopEdge pins that the strip shows the
// full-bleed ground already painted under it rather than a second fill drawn
// over it, which is why nothing in this app paints a band. The strip must be
// the ground and nothing else: no page ink in it, and no unpainted glass.
func TestTheGroundReachesTheWindowsTopEdge(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			ground := tc.colors.SurfaceAt(tokens.Level0)
			if ground != tc.colors.Background {
				t.Fatalf("level 0 resolves to %v and the ground layer paints %v", ground, tc.colors.Background)
			}
			for _, x := range []int{0, windowCanvasSize.X / 2, windowCanvasSize.X - 1} {
				for _, y := range []int{0, titleBandDp / 2, titleBandDp - 1} {
					if got := pixelAt(img, image.Pt(x, y)); got != ground {
						t.Errorf("strip pixel at (%d,%d) = %v, want the window ground %v", x, y, got, ground)
					}
				}
			}
		})
	}
}

// TestThePageStartsBelowTheStrip is the other half: the ground runs under the
// strip, the page does not. Read off the frame rather than off the inset, so
// that a page which grew a layer of its own outside the cap would fail here.
func TestThePageStartsBelowTheStrip(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			if top := topmostInk(img, tc.colors.SurfaceAt(tokens.Level0)); top < titleBandDp {
				t.Errorf("the page inks row %d, inside the %d dp title-bar strip; only the ground belongs there", top, titleBandDp)
			}
		})
	}
}

// TestThePageClearsTheWindowButtons: with the native strip gone the platform's
// three control buttons float over the top-leading corner of whatever this
// window drew there. The ground owes them nothing but its own colour, but the
// page must not reach into their run, which is taken from desktop's derivation
// of the platform's rule rather than from a guess at where the circles are.
//
// The margin is generous on purpose: the page is centred in the window, so the
// distance between its topmost ink and the buttons is not a tuned number and a
// regression that ate the inset would close it entirely.
func TestThePageClearsTheWindowButtons(t *testing.T) {
	run := desktop.ButtonRunIn(titleBandDp)
	bottom := int(run.Leading + run.Diameter)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			ground := tc.colors.SurfaceAt(tokens.Level0)
			for y := 0; y <= bottom; y++ {
				for x := 0; x <= int(run.Trailing); x++ {
					if got := pixelAt(img, image.Pt(x, y)); got != ground {
						t.Fatalf("page ink %v at (%d,%d), inside the window buttons' run (leading %v, trailing %v, centre %v)",
							got, x, y, run.Leading, run.Trailing, run.Center)
					}
				}
			}
			if top := topmostInk(img, ground); top <= bottom {
				t.Errorf("the page's topmost ink is row %d and the buttons end at row %d; the page has no clearance under them", top, bottom)
			}
		})
	}
}

// TestTheCardsRestOneStepOverThePage reads off the frame that the app cards
// lie exactly one step over the window's own page. The fill is the raise
// walked from that page ([tokens.ColorTokens.RaisedOn]) rather than a named
// step, so what is pinned is the grammar rather than a colour, and it is
// checked in both schemes because one step up means lighter in both.
//
// On paper the cards are #FFFFFF on a #F1F1F1 page and on slate #222222 on
// #181818 — one band step in either, and the fill carries it. Neither is a
// colour this test asserts: it asks elevation which level it painted, and
// level 1 is the name the walk's answer off the content goes by.
func TestTheCardsRestOneStepOverThePage(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			page := tokens.Level0
			resting := tokens.Level1
			ground := tc.colors.SurfaceAt(page)
			cards := cardRects(t, img, ground)
			if len(cards) != len(Apps) {
				t.Fatalf("measured %d cards in the frame, want one per app in the roster (%d)", len(cards), len(Apps))
			}
			for i, r := range cards {
				at := image.Pt(r.Min.X+int(tokens.Spacing.S4)/2, (r.Min.Y+r.Max.Y)/2)
				got := pixelAt(img, at)
				switch level, ok := nearestRung(got, tc.colors); {
				case !ok:
					t.Errorf("card %d fills %v at %v, which is no elevation level at all; a card resting on the page fills at %v",
						i, got, at, tc.colors.SurfaceAt(resting))
				case level != resting:
					t.Errorf("card %d fills %v at %v — level %d; it rests on the level-%d page, so it fills one step over it, at level %d (%v)",
						i, got, at, level, page, resting, tc.colors.SurfaceAt(resting))
				}

				// One pixel is a spot check; the fill is the claim. Count the
				// whole card, so a level-1 gutter around a level-2 body would
				// fail here even though the sample above passed.
				covered := map[tokens.ElevationLevel]int{}
				for y := r.Min.Y; y < r.Max.Y; y++ {
					for x := r.Min.X; x < r.Max.X; x++ {
						if level, ok := nearestRung(pixelAt(img, image.Pt(x, y)), tc.colors); ok {
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
				if area := r.Dx() * r.Dy(); widest != resting || n*2 < area {
					t.Errorf("card %d is covered by level %d over %d%% of itself; a card resting on the level-%d page is a level-%d fill",
						i, widest, n*100/area, page, resting)
				}

				// The plane has to be the level-0 ground, or "one rung over it"
				// is a claim about nothing. Read it back off the page beside the
				// card rather than trusting the ground layer's own token.
				beside := image.Pt(r.Max.X+int(RowGap)/2, (r.Min.Y+r.Max.Y)/2)
				if level, ok := nearestRung(pixelAt(img, beside), tc.colors); !ok || level != page {
					t.Errorf("the page beside card %d reads %v at %v, not the level-%d ground %v",
						i, pixelAt(img, beside), beside, page, ground)
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
