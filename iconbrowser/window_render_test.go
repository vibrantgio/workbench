package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. Both layers the app stacks are plain widgets over pre-resolved
// tokens, so composing the same two paints into a headless canvas at the size
// the window opens at produces the frame the window would show.
//
// The frames are drawn under a stated title-bar strip, because the app takes
// the full-size-content treatment: its ground runs to the top edge and its page
// starts below a strip the platform no longer draws. desktop.TopInset reports 0
// without a live macOS window behind it, so the height is stated rather than
// measured.
//
// Run it with -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/iconbrowser
//
// Without the flag it still renders every scheme and every query state each
// run, which makes it a smoke test of the whole view: a panic in the backdrop,
// the search field, either grid or the no-match notice fails it.

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Icon browser window opens at, and the only size
// these frames are drawn at. golden.Capture renders at one pixel per dp, so
// every dp figure below is also a pixel figure.
var windowSize = image.Pt(int(winW), int(winH))

// titleBandDp is the strip desktop.TopInset reports on a full-size-content
// macOS window. The number is the stored reference's plain title bar band,
// 32 px.
const titleBandDp unit.Dp = 32

// The frames carry the theme's real radius scale rather than a pinned-sharp
// one: nothing here is stored, so nothing needs corners that survive a
// pixel-exact diff between GPU contexts, and every assertion below counts
// pixels with room to spare for a few anti-aliased ones.

// staticTheme is one theme emission as a snapshot: every stream is rx.Of, so
// the components built from it — the search field is the only one here —
// resolve synchronously through First().
func staticTheme(c tokens.ColorTokens, typo tokens.Typography) theme.Theme {
	return theme.Theme{
		Color:      rx.Of(c),
		Typography: rx.Of(typo),
		Density:    rx.Of(tokens.Comfortable),
		Motion:     rx.Of(tokens.Motion),
		Spacing:    rx.Of(tokens.Spacing),
		Radius:     rx.Of(tokens.Radius),
		Elevation:  rx.Of(tokens.Elevation),
	}
}

// staticTypo builds Type by hand rather than through TypeFrom, which would
// take the theme's own cached shaper — whatever typeface the host happens to
// own. A render made outside the window has to name the one it shapes with.
func staticTypo(typo tokens.Typography) Type {
	return Type{
		Shaper:  typo.DeterministicShaper(),
		Caption: textStyle(typo.BodySmall),
		Section: textStyle(typo.TitleSmall),
		Notice:  textStyle(typo.TitleLarge),
	}
}

// staticThemed is the snapshot ContentLayer's own themes stream builds per
// emission, built once per scheme here instead: the palette, the pinned
// typography, and the whole icon table prebuilt in the palette's glyph colour.
// The prebuild is the app's own — raster.Widget decodes a viewBox up front and
// rasterises lazily — and it is cached across frames so that four renders of a
// 961-glyph catalogue cost one pass over it.
func staticThemed(t *testing.T, c tokens.ColorTokens) themed {
	t.Helper()
	p := PaletteFrom(c)
	if cached, ok := prebuilt[p.Icon]; ok {
		return themed{palette: p, typ: staticTypo(tokens.DefaultTypography), icons: cached}
	}
	widgets := make([]layout.Widget, len(IconTable))
	for i, icon := range IconTable {
		w, err := raster.Widget(icon.Data, IconSize, IconSize, raster.WithColors(p.Icon))
		if err != nil {
			t.Fatalf("icon %s: %v", icon.Name, err)
		}
		widgets[i] = w
	}
	prebuilt[p.Icon] = widgets
	return themed{palette: p, typ: staticTypo(tokens.DefaultTypography), icons: widgets}
}

// prebuilt caches the icon widgets per glyph colour, which is the only thing
// about a scheme they depend on.
var prebuilt = map[color.NRGBA][]layout.Widget{}

// staticSearch is the page's search field, resolved off a static theme
// snapshot: the same components TextField ContentLayer subscribes to, with the
// pinned shaper a render made outside the window has to name. It is the
// window's topmost ink, so it is the thing the clearance assertions are about.
func staticSearch(t *testing.T, c tokens.ColorTokens) layout.Widget {
	t.Helper()
	typo := tokens.DefaultTypography
	w, err := input.TextField(rx.Of(staticTheme(c, typo)), input.TextFieldProps{
		Placeholder: "Search icons…",
		Description: "search icons by name",
		Shaper:      typo.DeterministicShaper(),
	}).First()
	if err != nil {
		t.Fatalf("search field: %v", err)
	}
	return w
}

// windowFrame composes the window exactly as buildLayers stacks it: the
// backdrop first, full-bleed to the window's top edge, then the page over it,
// held down past the strip of the given height and with that strip claimed for
// the window's drag.
//
// band is the strip height to render under; 0 draws the window as every
// platform but macOS shows it, with the page at the window's own top edge.
func windowFrame(t *testing.T, c tokens.ColorTokens, model Model, band unit.Dp) layout.Widget {
	t.Helper()
	tok := staticThemed(t, c)
	ground := backdrop.Widget(tok.palette.Backdrop)
	page := desktop.CapTop(
		func() unit.Dp { return band },
		Page(tok, staticSearch(t, c), model, &layout.List{Axis: layout.Vertical}),
	)
	return func(gtx layout.Context) layout.Dimensions {
		ground(gtx)
		return page(gtx)
	}
}

func renderWindow(t *testing.T, c tokens.ColorTokens, query string, band unit.Dp) *image.RGBA {
	t.Helper()
	return golden.Capture(t, windowSize, windowFrame(t, c, Model{Query: query}, band))
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

// pixelAt reads one pixel as an opaque NRGBA, which is what every token in the
// set is.
func pixelAt(img *image.RGBA, p image.Point) color.NRGBA {
	r, g, b, _ := img.At(p.X, p.Y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
}

// countFill counts the pixels inside r that are exactly want.
func countFill(img *image.RGBA, r image.Rectangle, want color.NRGBA) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if pixelAt(img, image.Pt(x, y)) == want {
				n++
			}
		}
	}
	return n
}

// windowSchemes is the pair every rule below is checked against.
var windowSchemes = []struct {
	name string
	c    tokens.ColorTokens
}{
	{"light", tokens.DefaultLight},
	{"dark", tokens.DefaultDark},
}

// windowQueries are the three shapes the page takes: the whole catalogue under
// both section labels, a query that keeps some of each set, and a query that
// matches nothing — which drops the marks section whole and puts the notice in
// the middle of the grid.
var windowQueries = []struct {
	name  string
	query string
}{
	{"all", ""},
	{"filtered", "history"},
	{"nomatch", "zzzz"},
}

// TestWholeWindowRender draws the composed window in both schemes on every
// query state, and writes the frames out when -window.dump names a directory.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range windowSchemes {
		for _, q := range windowQueries {
			t.Run(tc.name+"-"+q.name, func(t *testing.T) {
				img := renderWindow(t, tc.c, q.query, titleBandDp)
				if img.Bounds().Size() != windowSize {
					t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowSize)
				}
				if *windowDump == "" {
					return
				}
				if err := os.MkdirAll(*windowDump, 0o755); err != nil {
					t.Fatalf("dump dir: %v", err)
				}
				path := filepath.Join(*windowDump, "iconbrowser-"+tc.name+"-"+q.name+".png")
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
}

// TestTheGridRestsOnTheWindowGround reads the surface walk off the frame: the
// grid draws straight onto the Background pin, with no furniture to raise and
// no selection to tint, so the only thing off the pin is ink and the one
// control standing on it.
func TestTheGridRestsOnTheWindowGround(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", titleBandDp)
			frame := image.Rectangle{Max: windowSize}
			ground := tc.c.SurfaceAt(tokens.Level0)
			furniture := tc.c.SurfaceAt(tokens.Level1)
			transient := tc.c.SurfaceAt(tokens.Level2)

			// The walk out from the middle: the window's centre and its edge
			// wear one rung, and it is the ground. Both points land on the
			// paper — the catalogue's ink is a glyph and a caption centred in
			// each cell, never a fill of one.
			for _, p := range []struct {
				name string
				at   image.Point
			}{
				{"window centre", image.Pt(windowSize.X/2, windowSize.Y/2)},
				{"window edge", image.Pt(2, windowSize.Y/2)},
			} {
				if got := pixelAt(img, p.at); got != ground {
					t.Errorf("%s at %v = %v, want the ground %v", p.name, p.at, got, ground)
				}
			}

			// And it is the ground in bulk, not just at two points: the
			// catalogue is glyphs and captions on the paper, not tiles.
			total := windowSize.X * windowSize.Y
			if n := countFill(img, frame, ground); n*4 < total*3 {
				t.Errorf("the ground %v covers %d of %d pixels; the thing this window exists to show is not what most of it is",
					ground, n, total)
			}
			// Level 1 is bounded by the one control that may wear it rather
			// than by a round fraction of the window, because that control is
			// full-width: the search field is a Density.ControlHeight box
			// spanning the page less its Padding gutters. That box is what the
			// rung is allowed, with a few rows of slack for the field's own
			// text metrics; anything past it is an expanse, not a control.
			field := (windowSize.X - 2*int(Padding)) * (int(tokens.Comfortable.ControlHeight) + 8)
			if n := countFill(img, frame, furniture); n > field {
				t.Errorf("level 1 (%v) covers %d of %d pixels, past the %d the search field's own box accounts for; a control on the ground may wear it, a resting expanse may not",
					furniture, n, total, field)
			}
			// Level 2 is not asked for zero, because a ramp step is a colour
			// and an anti-aliased edge between two others can land on it by
			// arithmetic. What the rung may not be is an expanse.
			if n := countFill(img, frame, transient); n*1000 > total {
				t.Errorf("level 2 (%v) covers %d of %d pixels of the resting window; that rung is for what appears and leaves",
					transient, n, total)
			}
		})
	}
}

// TestTheGroundReachesTheWindowsTopEdge pins that the strip shows the
// full-bleed backdrop already painted under it rather than a second fill drawn
// over it, which is why this window paints no band. The strip must be the
// ground and nothing else: no page ink in it, and no unpainted glass.
func TestTheGroundReachesTheWindowsTopEdge(t *testing.T) {
	band := int(titleBandDp)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", titleBandDp)
			ground := tc.c.SurfaceAt(tokens.Level0)
			if got := PaletteFrom(tc.c).Backdrop; got != ground {
				t.Fatalf("the backdrop paints %v and level 0 resolves to %v", got, ground)
			}
			for _, x := range []int{0, windowSize.X / 2, windowSize.X - 1} {
				for _, y := range []int{0, band / 2, band - 1} {
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
			img := renderWindow(t, tc.c, "", titleBandDp)
			if top := topmostInk(img, tc.c.SurfaceAt(tokens.Level0)); top < int(titleBandDp) {
				t.Errorf("the page inks row %d, inside the %d dp title-bar strip; only the ground belongs there", top, int(titleBandDp))
			}
		})
	}
}

// TestThePageClearsTheWindowButtons: with the native strip gone the platform's
// three control buttons float over the top-leading corner of whatever the
// application drew there, and this page's first row is the search field, which
// starts at the very corner they stand in. Nothing here is centred out of their
// way, so the clearance is the inset's alone and it is asserted off the frame
// rather than trusted to the arithmetic. The run comes from desktop's
// derivation of the platform's rule rather than a guess at where the circles
// are.
//
// Measured off these frames: with the strip the page's topmost ink is row 44 of
// 700, and without it row 12 — while the buttons in a 32 dp band run rows 9 to
// 23 and reach 69 dp along (desktop.ButtonRunIn(32): leading 9, centre 16,
// trailing 69). The field's own Padding would put it eleven rows inside the
// run, so the inset is the whole of the 21 dp of clearance it has.
//
// It is checked on every query state, because the field is the first row of all
// three and a page that reflowed under a filter would still have to keep it
// clear.
func TestThePageClearsTheWindowButtons(t *testing.T) {
	run := desktop.ButtonRunIn(titleBandDp)
	bottom := int(run.Leading + run.Diameter)
	for _, tc := range windowSchemes {
		for _, q := range windowQueries {
			t.Run(tc.name+"-"+q.name, func(t *testing.T) {
				img := renderWindow(t, tc.c, q.query, titleBandDp)
				ground := tc.c.SurfaceAt(tokens.Level0)
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
}

// TestTheInsetIsWhatBuysTheClearance guards the inset itself: laid out at the
// window's own top edge, the search field inks inside the run the platform's
// three control buttons stand in. The inset is what takes it out of there, not
// the field's own Padding.
func TestTheInsetIsWhatBuysTheClearance(t *testing.T) {
	run := desktop.ButtonRunIn(titleBandDp)
	bottom := int(run.Leading + run.Diameter)
	ground := tokens.DefaultLight.SurfaceAt(tokens.Level0)

	capped := renderWindow(t, tokens.DefaultLight, "", titleBandDp)
	bare := renderWindow(t, tokens.DefaultLight, "", 0)

	if top := topmostInk(bare, ground); top > bottom {
		t.Errorf("the uninset page's topmost ink is row %d and the buttons end at row %d; it clears them without the strip, so this window's inset is not what it is documented to be",
			top, bottom)
	}
	if top := topmostInk(capped, ground); top <= bottom {
		t.Errorf("the inset page's topmost ink is row %d and the buttons end at row %d; the inset has stopped buying the clearance", top, bottom)
	}
	if n := golden.PixelDiff(capped, bare); n == 0 {
		t.Error("the window renders identically with and without a title-bar strip; the page is not being inset")
	}
}
