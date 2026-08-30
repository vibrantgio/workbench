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
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/todos
//
// Without the flag it still renders every scheme and route each run, which
// makes it a smoke test of the whole view: a panic in the backdrop, the list,
// the fab or the dialog fails it.

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
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Todos window opens at, and the only size these
// frames are drawn at. golden.Capture renders at one pixel per dp, so every dp
// figure below is also a pixel figure.
var windowSize = image.Pt(int(winW), int(winH))

// titleBandDp is the strip desktop.TopInset reports on a full-size-content
// macOS window. The number is the stored reference's plain title bar band,
// 32 px.
const titleBandDp unit.Dp = 32

// The frames carry the theme's real radius scale rather than a pinned-sharp
// one: nothing here is stored, so no pixel-exact diff needs GPU-independent
// corners, and every assertion below counts pixels with room to spare for a few
// anti-aliased ones.

// staticThemed is one theme emission frozen into the snapshot the view
// consumes, with the pinned shaper — Roboto and nothing the machine happens to
// own — that a stored render has to shape with. It builds Type by hand rather
// than through TypeFrom, which would take the theme's own cached shaper and so
// whatever the host can find.
func staticThemed(c tokens.ColorTokens) themed {
	typo := tokens.DefaultTypography
	return themed{
		components: theme.Theme{
			Color:      rx.Of(c),
			Typography: rx.Of(typo),
			Density:    rx.Of(tokens.Comfortable),
			Motion:     rx.Of(tokens.Motion),
			Spacing:    rx.Of(tokens.Spacing),
			Radius:     rx.Of(tokens.Radius),
			Elevation:  rx.Of(tokens.Elevation),
		},
		palette: PaletteFrom(c),
		typ: Type{
			Shaper:   typo.DeterministicShaper(),
			Headline: textStyle(typo.HeadlineSmall),
			Title:    textStyle(typo.TitleLarge),
		},
	}
}

// fixtureModel is a list somebody would actually keep, on the route named. The
// wording is deliberately ordinary so a reader judges the frame on how it
// looks rather than reading the copy.
func fixtureModel(route string) Model {
	return Model{
		Route: route,
		List: TodoList{
			{Id: 0, Text: "Buy oat milk", Completed: true},
			{Id: 1, Text: "Draft the release notes", Completed: false},
			{Id: 2, Text: "Book the dentist", Completed: false},
			{Id: 3, Text: "Water the plants", Completed: false},
		},
	}
}

// windowFrame composes the window exactly as buildLayers stacks it: the
// backdrop first, full-bleed to the window's top edge, then the page over it,
// held down past the strip of the given height. Both come from the same colour
// tokens the running app resolves them from, so the frame is the app's
// composition minus the streams.
//
// band is the strip height to render under; 0 draws the window as every
// platform but macOS shows it, with the page at the window's own top edge.
func windowFrame(c tokens.ColorTokens, model Model, band unit.Dp) layout.Widget {
	th := staticThemed(c)
	ground := backdrop.Widget(th.palette.Backdrop)
	page := view(th, model, func() unit.Dp { return band })
	return func(gtx layout.Context) layout.Dimensions {
		ground(gtx)
		return page(gtx)
	}
}

func renderWindow(t *testing.T, c tokens.ColorTokens, route string, band unit.Dp) *image.RGBA {
	t.Helper()
	return golden.Capture(t, windowSize, windowFrame(c, fixtureModel(route), band))
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

var windowRoutes = []struct {
	name  string
	route string
}{
	{"list", ""},
	{"add", "add.todo"},
}

// TestWholeWindowRender draws the composed window in both schemes on both
// routes, and writes the frames out when -window.dump names a directory.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range windowSchemes {
		for _, r := range windowRoutes {
			t.Run(tc.name+"-"+r.name, func(t *testing.T) {
				img := renderWindow(t, tc.c, r.route, titleBandDp)
				if img.Bounds().Size() != windowSize {
					t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowSize)
				}
				if *windowDump == "" {
					return
				}
				if err := os.MkdirAll(*windowDump, 0o755); err != nil {
					t.Fatalf("dump dir: %v", err)
				}
				path := filepath.Join(*windowDump, "todos-"+tc.name+"-"+r.name+".png")
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

// TestTheListRestsOnTheWindowGround reads the surface walk off the frame: the
// middle and the edge are the same ground, and the only level-1 pixels are the
// raised controls standing on it.
//
// It renders the route with no dialog on it, because the walk is over the
// window at rest: a modal stands in the middle at level 2 by design, so a walk
// taken with one open would report a violation the grammar granted.
func TestTheListRestsOnTheWindowGround(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", titleBandDp)
			frame := image.Rectangle{Max: windowSize}
			ground := tc.c.SurfaceAt(tokens.Level0)
			furniture := tc.c.SurfaceAt(tokens.Level1)
			transient := tc.c.SurfaceAt(tokens.Level2)

			// The walk out from the middle: centre and edge wear one rung,
			// and it is the ground. Both points are clear of ink — the rows
			// stack from the window's top and end well above its middle.
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
			// resting window is the pin, with ink and controls on it.
			total := windowSize.X * windowSize.Y
			if n := countFill(img, frame, ground); n*4 < total*3 {
				t.Errorf("the ground %v covers %d of %d pixels; the thing this window exists to show is not what most of it is",
					ground, n, total)
			}
			if n := countFill(img, frame, furniture); n*100 > total {
				t.Errorf("level 1 (%v) covers %d of %d pixels; a control on the ground may wear it, a resting expanse may not",
					furniture, n, total)
			}
			// Level 2 is not asked for zero: a ramp step is a colour, and an
			// anti-aliased edge between two other colours can land on it by
			// arithmetic — a couple of dozen pixels along the rounded checkbox
			// borders do, in both schemes. What the rung may not be is an
			// expanse.
			if n := countFill(img, frame, transient); n*1000 > total {
				t.Errorf("level 2 (%v) covers %d of %d pixels of the resting window; that rung is for what appears and leaves",
					transient, n, total)
			}
		})
	}
}

// dialogRect is where UpsertDialog puts its surface, derived from the same
// constants the layout uses rather than measured off the frame: the page's
// uniform Padding inset, then a ModalWidth by ModalHeight box constrained to
// that inset area and centred in it.
func dialogRect() image.Rectangle {
	pad := int(Padding)
	inner := image.Rect(pad, pad, windowSize.X-pad, windowSize.Y-pad)
	w, h := int(ModalWidth), int(ModalHeight)
	if w > inner.Dx() {
		w = inner.Dx()
	}
	if h > inner.Dy() {
		h = inner.Dy()
	}
	x := inner.Min.X + (inner.Dx()-w)/2
	y := inner.Min.Y + (inner.Dy()-h)/2
	return image.Rect(x, y, x+w, y+h)
}

// TestTheDialogAndItsFieldHoldTheirRungs reads the modal's two rungs off the
// frame. The dialog is at level 2, and the field inside it walks one rung on
// from the dialog rather than from the window: a raised inset steps up from
// the surface it lies on.
func TestTheDialogAndItsFieldHoldTheirRungs(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			if want := tc.c.SurfaceAt(tokens.Level2); p.Dialog != want {
				t.Errorf("dialog surface = %v, want level 2 %v", p.Dialog, want)
			}
			if want := tc.c.SurfaceAt(tokens.Level3); p.Edit != want {
				t.Errorf("field fill = %v, want level 3 %v", p.Edit, want)
			}

			img := renderWindow(t, tc.c, "add.todo", titleBandDp)
			rect := dialogRect()

			// The dialog's own fill, read 6 dp above its bottom edge and
			// centred: the button row is inset a full Padding from there, so
			// nothing is drawn over it.
			at := image.Pt(rect.Min.X+rect.Dx()/2, rect.Max.Y-6)
			if got := pixelAt(img, at); got != p.Dialog {
				t.Errorf("dialog surface at %v = %v, want %v", at, got, p.Dialog)
			}

			// The field is actually painted, and the dialog it lies in is
			// still the larger of the two.
			field := countFill(img, rect, p.Edit)
			dialog := countFill(img, rect, p.Dialog)
			if field < 2000 {
				t.Errorf("the field fill %v covers %d pixels of the dialog; a rung nothing is painted in is not a rung",
					p.Edit, field)
			}
			if dialog <= field {
				t.Errorf("the dialog covers %d pixels and the field on it %d; the inset has swallowed the surface it rests on",
					dialog, field)
			}

			// And the page behind is not showing through at either rung.
			if got := pixelAt(img, at); got == tc.c.SurfaceAt(tokens.Level0) {
				t.Errorf("the dialog reads as the window ground at %v", at)
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
// application drew there, and this page's first list row starts at the very
// corner they stand in. Nothing here is centred out of their way — the rows
// stack from the top edge — so the clearance is the inset's alone, and it is
// asserted off the frame rather than trusted to the arithmetic. The run comes
// from desktop's derivation of the platform's rule rather than a guess at
// where the circles are.
func TestThePageClearsTheWindowButtons(t *testing.T) {
	run := desktop.ButtonRunIn(titleBandDp)
	bottom := int(run.Leading + run.Diameter)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", titleBandDp)
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

// TestTheModalCoversTheStripToo is the exception the page makes for what is
// transient. The resting page starts below the strip; the scrim does not,
// because a cover with a strip-shaped hole in its top edge isolates nothing —
// the un-dimmed band would read as a seam across the window's top edge. The
// dialog and its cover are therefore laid out in the window's own
// coordinates.
func TestTheModalCoversTheStripToo(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			resting := renderWindow(t, tc.c, "", titleBandDp)
			covered := renderWindow(t, tc.c, "add.todo", titleBandDp)
			for _, x := range []int{0, windowSize.X / 2, windowSize.X - 1} {
				for _, y := range []int{0, int(titleBandDp) / 2, int(titleBandDp) - 1} {
					at := image.Pt(x, y)
					if pixelAt(covered, at) == pixelAt(resting, at) {
						t.Errorf("the strip at %v is unchanged with the modal open; the scrim stops at the page's top edge", at)
					}
				}
			}
		})
	}
}

// TestTheStripMovesThePage guards the inset itself: a frame drawn under the
// strip is not the frame drawn without one. Without this the tests above would
// all still pass if the cap stopped insetting anything, since the first
// row's ink starts below the buttons' run on its own.
func TestTheStripMovesThePage(t *testing.T) {
	capped := renderWindow(t, tokens.DefaultLight, "", titleBandDp)
	bare := renderWindow(t, tokens.DefaultLight, "", 0)
	if n := golden.PixelDiff(capped, bare); n == 0 {
		t.Error("the window renders identically with and without a title-bar strip; the page is not being inset")
	}
}
