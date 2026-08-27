package main

// A whole-window render, headless, of the launcher as its layers actually
// stack: the Background ground, and the page held down past the title-bar
// strip a full-size-content window opens the top of itself into. The window is
// a native binary with no offscreen mode of its own, but every layer it stacks
// is a plain widget over pre-resolved tokens, so composing the same paints
// into a headless canvas at the size the window opens at produces the frame
// the window would show.
//
// It is here because the goldens beside this file render the page on its
// ground at the window's own top edge — the frame every platform but macOS
// shows — and so cannot see the one thing this task changed, which is where
// the page starts. Run it with -window.dump=<dir> to write the frames out for
// a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/workbench
//
// The frames come in two makes, and the difference is the animated seen
// triangle field the running window floats the page on. The assertions read a
// frame drawn without it, for the reason the goldens give: the field is driven
// by the clock, so it has no one frame to store — and with it gone, every
// pixel that is not the Background pin is ink the page put there, which is
// what makes a claim about the strip readable at all. The dumped frames carry
// it, because a composition handed to a pair of eyes with its most prominent
// layer missing is not the window.

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
// measure — the same substitution mvu/desktop's own drag tests make. The
// number is the stored reference's plain title bar band (ADR-019: 32 px), not
// a guess and not a live measurement.
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
// scale put back. The goldens pin radius to zero because anti-aliased corners
// vary between GPU contexts and a pixel-exact diff cannot survive that; these
// frames store nothing and are drawn to be looked at, where the opposite need
// applies. A square-cornered card is the first thing a reviewer on this
// platform complains about, and on a frame drawn with RadiusScale{} they would
// be complaining about the harness rather than the window.
func roundedThemed(colors tokens.ColorTokens) themed {
	tok := staticThemed(colors)
	tok.components.Radius = rx.Of(tokens.Radius)
	return tok
}

// livingWindowFrame is windowFrame with the triangle field in the middle of
// the stack where the running window has it — the whole composition, and the
// only make of this frame worth showing anybody. It is not what the assertions
// read: the field's vertices are displaced from a clock, so two captures of it
// are two pictures.
//
// The field invalidates a window to drive its own frames, and here there is no
// window to drive: an app.Window that is never run absorbs those calls, which
// is all this needs, since a single capture takes the scene as the constructor
// left it rather than animating it. For the same reason the palette is applied
// here rather than left to the tick that would normally do it — waiting on a
// clock for a colour is how a capture ends up photographing the placeholder.
func livingWindowFrame(tok themed, model Model, band unit.Dp) layout.Widget {
	field := NewField(new(app.Window), winW, winH)
	field.SetColors(tok.color)
	field.applyPending()
	return stack(backdrop.Widget(tok.color.Background), field.Widget(), cappedPage(tok, model, band))
}

// windowSchemes is the pair every rule below is stated once and checked twice
// against.
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
// token that was painted into it and still count as that rung. Gio blends in
// linear space, so a flat fill does not always survive the round trip to 8-bit
// sRGB exactly: in the dark scheme, at the bottom of the curve where the
// quantisation is coarsest, a level-1 card comes back speckled a value or two
// above its own token. The ladder's rungs are ten steps apart at their closest,
// so this leaves no room for a pixel to be claimed by the wrong one.
const rungTolerance = 4

// nearestRung reports the elevation rung a rendered pixel sits on — the level
// whose surface fill it is within rungTolerance of — and whether it is a
// surface fill at all rather than ink drawn on one.
func nearestRung(c color.NRGBA, colors tokens.ColorTokens) (tokens.ElevationLevel, bool) {
	for _, level := range []tokens.ElevationLevel{tokens.Level0, tokens.Level1, tokens.Level2, tokens.Level3} {
		if withinRung(c, colors.SurfaceAt(level)) {
			return level, true
		}
	}
	return 0, false
}

func withinRung(a, b color.NRGBA) bool {
	return channelDiff(a.R, b.R) <= rungTolerance &&
		channelDiff(a.G, b.G) <= rungTolerance &&
		channelDiff(a.B, b.B) <= rungTolerance
}

func channelDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// cardRects measures the launcher's resting app cards off the frame instead of
// recomputing the layout's arithmetic. The grid is the widest thing the page
// draws — cardRow lays every row out at GridW whatever it holds, and the hero
// above them is a fraction of that — so the frame rows carrying ink that wide
// are card rows and nothing else. Each contiguous stretch of them is one row of
// cards: it is shortened at top and bottom by the same corner radius, so the
// stretch's midpoint is the cards' midpoint, and it runs a pixel wide at both
// ends because the outlined card's 1 dp stroke straddles the edge it draws,
// which is why the samples below are taken half an S4 inside the measured
// leading edge rather than on it.
func cardRects(t *testing.T, img *image.RGBA, ground color.NRGBA) []image.Rectangle {
	t.Helper()
	size := img.Bounds().Size()
	var rects []image.Rectangle
	for y := 0; y < size.Y; y++ {
		first, last := inkSpan(img, y, ground)
		if last-first+1 < int(GridW) {
			continue
		}
		top := y
		for y+1 < size.Y {
			f, l := inkSpan(img, y+1, ground)
			if l-f+1 < int(GridW) {
				break
			}
			y++
		}
		mid := (top + y) / 2
		for c := 0; c < perRow && len(rects) < len(Apps); c++ {
			x := first + c*(int(CardW)+int(RowGap))
			rects = append(rects, image.Rect(x, mid-int(CardH)/2, x+int(CardW), mid+int(CardH)/2))
		}
	}
	return rects
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
// as well, so what a reviewer is handed is the window rather than a diagram
// of it.
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

// TestTheGroundReachesTheWindowsTopEdge is R6 for a window with no band of its
// own to paint. The region the strip caps here is the window's own ground, and
// the ground layer is full-bleed, so the rule is met by the strip showing what
// was already painted under it rather than by a second fill drawn over it —
// which is why nothing in this app paints a band. What has to be true for that
// to be R6 satisfied rather than R6 skipped is that the strip is the ground and
// nothing else: no page ink in it, and no unpainted glass either.
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

// TestThePageStartsBelowTheStrip is the other half of the same arrangement:
// the ground runs under the strip, the page does not. Read off the frame
// rather than off the inset, so that a page which grew a layer of its own
// outside the cap would fail here.
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

// TestThePageClearsTheWindowButtons is R6's first consequence: with the native
// strip gone the platform's three control buttons float over the top-leading
// corner of whatever this window drew there, so the region reaching that
// corner owes them their run. Here that region is the ground, which owes them
// nothing but its own colour — but the page must not reach into the run, and
// the run is taken from desktop's derivation of the platform's rule rather
// than from a guess at where the circles are.
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

// TestTheCardsRestOneRungOverThePage is R4 in the small, read off the frame:
// the launcher's eight app cards lie on the window's own ground, and a thing
// lying on a plane fills one rung over it. The rung is walked from that ground
// — tokens.Level0.Raised() — rather than named as a step, so what is pinned
// here is the grammar rather than a colour, and it is checked in both schemes
// because one rung up darkens in the one and lightens in the other.
//
// The arrangement this replaces is the audit's finding: the cards were built
// Elevated, and that variant fills at level 2, the rung the ladder keeps for
// surfaces that leave the plane. Eight of those at rest is why the grid read
// heavier than the window around it. Only a frame can say which rung was
// painted — the goldens beside this file would have stored either one without
// complaint, which is how the wrong one survived an audit that called these
// cards level 1.
func TestTheCardsRestOneRungOverThePage(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.colors, titleBandDp)
			page := tokens.Level0
			resting := page.Raised()
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
					t.Errorf("card %d fills %v at %v, which is no rung of the ladder at all; a card resting on the page fills at %v",
						i, got, at, tc.colors.SurfaceAt(resting))
				case level != resting:
					t.Errorf("card %d fills %v at %v — level %d; it rests on the level-%d page, so it fills one rung over it, at level %d (%v)",
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
// strip is not the frame drawn without one. Without this the three tests above
// would all still pass if the cap stopped insetting anything, since a
// centred page clears a 32 dp strip on its own.
func TestTheStripMovesThePage(t *testing.T) {
	capped := renderWindow(t, tokens.DefaultLight, titleBandDp)
	bare := renderWindow(t, tokens.DefaultLight, 0)
	if n := golden.PixelDiff(capped, bare); n == 0 {
		t.Error("the window renders identically with and without a title-bar strip; the page is not being inset")
	}
}
