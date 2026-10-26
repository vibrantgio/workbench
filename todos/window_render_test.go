package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. Both layers the app stacks are plain layout.Widgets over
// pre-resolved tokens, so composing the same two paints into a headless image
// at the size the window opens at produces the frame the window would show.
//
// The frames are drawn under a stated band, because the app takes the
// full-size-content treatment: its fill runs to the top edge, its toolbar
// takes the band at the top, and its list starts below the band.
// desktop.TopInset reports 0 without a live macOS window behind it, so the
// depth is stated rather than measured.
//
// Run it with -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/todos
//
// Without the flag it still renders every scheme and route each run, which
// makes it a smoke test of the whole view: a panic in the backdrop, the list,
// the toolbar or the dialog fails it.

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
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Todos window opens at, and the only size these
// frames are drawn at. golden.Capture renders at one pixel per dp, so every dp
// figure below is also a pixel figure.
var windowSize = image.Pt(int(winW), int(winH))

// bandDp is the band View lays the toolbar out in on a full-size-content
// macOS window: the platform's toolbar depth, which is deeper than the native
// title-bar strip.
const bandDp unit.Dp = shell.ToolbarDepth

// The frames carry the theme's real radius scale rather than a pinned-sharp
// one: nothing here is stored, so no pixel-exact diff needs GPU-independent
// corners, and every assertion below counts pixels with room to spare for a few
// anti-aliased ones.

// staticThemed is one theme emission frozen into the snapshot the view
// consumes, with the pinned shaper — Roboto and nothing the machine happens to
// own — that a stored render has to shape with. It builds Type by hand rather
// than through TypeFrom, which would take the theme's own cached shaper and so
// whatever the host can find.
func staticThemed(p tokens.PlatformColors) themed {
	typo := tokens.DefaultTypography
	return themed{
		components: theme.Theme{
			Platform:   rx.Of(p),
			Typography: rx.Of(typo),
			Density:    rx.Of(tokens.Comfortable),
			Motion:     rx.Of(tokens.Motion),
			Spacing:    rx.Of(tokens.Spacing),
			Radius:     rx.Of(tokens.Radius),
		},
		palette:    PaletteFrom(p),
		platform:   p,
		typography: typo,
		density:    tokens.Comfortable,
		typ: Type{
			Shaper: typo.DeterministicShaper(),
			Field:  textStyle(typo.SystemFont),
			Row:    textStyle(typo.SystemFont),
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
func windowFrame(p tokens.PlatformColors, model Model, band unit.Dp) layout.Widget {
	th := staticThemed(p)
	paintBackdrop := backdrop.Widget(th.palette.Backdrop)
	page := view(th, model, complist.NewState(), func() unit.Dp { return band })
	return func(gtx layout.Context) layout.Dimensions {
		paintBackdrop(gtx)
		return page(gtx)
	}
}

func renderWindow(t *testing.T, p tokens.PlatformColors, route string, band unit.Dp) *image.RGBA {
	t.Helper()
	return golden.Capture(t, windowSize, windowFrame(p, fixtureModel(route), band))
}

// renderModel renders the window showing model under the toolbar's band.
func renderModel(t *testing.T, p tokens.PlatformColors, model Model) *image.RGBA {
	t.Helper()
	return golden.Capture(t, windowSize, windowFrame(p, model, bandDp))
}

// topmostDrawn reports the first row of the page carrying a pixel that is
// not the fill it is given, and how many rows were scanned when there is none.
// The toolbar's drawing in its band is not the page's and is skipped.
func topmostDrawn(img *image.RGBA, fill color.NRGBA) int {
	size := img.Bounds().Size()
	bar := toolbarBox(img, fill)
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if image.Pt(x, y).In(bar) {
				continue
			}
			if pixelAt(img, image.Pt(x, y)) != fill {
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
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
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
				img := renderWindow(t, tc.c, r.route, bandDp)
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

// TestTheListStandsOnTheWindowsOwnPlane reads the window's fills off the
// frame: the middle and the edge are the platform's window background, and
// that fill is most of what the window is. The list paints nothing of its
// own, so anything else in the frame is text, a checkbox or a symbol.
//
// It renders the route with no dialog on it, because the claim is about the
// window at rest: a dialog lays a scrim over everything.
func TestTheListStandsOnTheWindowsOwnPlane(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", bandDp)
			frame := image.Rectangle{Max: windowSize}
			plane := tc.c.WindowBackground

			// Both points are clear of paint — the rows stack from the
			// window's top and end well above its middle.
			for _, p := range []struct {
				name string
				at   image.Point
			}{
				{"window centre", image.Pt(windowSize.X/2, windowSize.Y/2)},
				{"window edge", image.Pt(2, windowSize.Y/2)},
			} {
				if got := pixelAt(img, p.at); got != plane {
					t.Errorf("%s at %v = %v, want the window's own plane %v", p.name, p.at, got, plane)
				}
			}

			// And it is that fill in bulk, not just at two points.
			total := windowSize.X * windowSize.Y
			if n := countFill(img, frame, plane); n*4 < total*3 {
				t.Errorf("the window's own plane %v covers %d of %d pixels; the thing this window exists to show is not what most of it is",
					plane, n, total)
			}
		})
	}
}

// dialogRect is where UpsertDialog puts its surface, derived from the same
// constants the layout uses rather than measured off the frame: the page's
// uniform Padding inset, then a DialogWidth by DialogHeight box constrained to
// that inset area and centred in it.
func dialogRect() image.Rectangle {
	pad := int(Padding)
	inner := image.Rect(pad, pad, windowSize.X-pad, windowSize.Y-pad)
	w, h := int(DialogWidth), int(DialogHeight)
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

// TestTheDialogAndItsFieldWearThePlatformsFills reads the dialog's two fills
// off the frame. A floating surface is the window's own plane on this
// platform, and the field inside it is the platform's text plane inside the
// hairline a field draws around itself — the two fills are the same value,
// which is what the platform's own save sheet draws, so the hairline is the
// whole of what says where the field is.
func TestTheDialogAndItsFieldWearThePlatformsFills(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			if p.Dialog != tc.c.WindowBackground {
				t.Errorf("dialog fill = %v, want the window's own plane %v", p.Dialog, tc.c.WindowBackground)
			}
			if p.Field != tc.c.TextBackground {
				t.Errorf("field fill = %v, want the platform's text plane %v", p.Field, tc.c.TextBackground)
			}
			if p.FieldEdge != tc.c.FieldEdge {
				t.Errorf("field hairline = %v, want the platform's field edge %v", p.FieldEdge, tc.c.FieldEdge)
			}

			img := renderWindow(t, tc.c, "add.todo", bandDp)
			rect := dialogRect()

			// The dialog's own fill, read 6 dp above its bottom edge and
			// centred: the button row is inset a full Padding from there, so
			// nothing is drawn over it.
			at := image.Pt(rect.Min.X+rect.Dx()/2, rect.Max.Y-6)
			if got := pixelAt(img, at); got != p.Dialog {
				t.Errorf("dialog fill at %v = %v, want %v", at, got, p.Dialog)
			}

			// The hairline is actually drawn: it rings a field the width of
			// the dialog's inset content, so a ring of it is thousands of
			// pixels and a threshold well under that catches a field that
			// stopped being painted.
			if n := countFill(img, rect, p.FieldEdge); n < 1000 {
				t.Errorf("the field hairline %v covers %d pixels of the dialog; the hairline is the whole of what says where the field is",
					p.FieldEdge, n)
			}

			// And the page behind it is dimmed, which is what tells the
			// dialog from the plane it repeats — dimmed to the byte the
			// platform's own blend puts there, which is the scrim composited
			// over the plane in encoded sRGB rather than blended with it.
			behind := image.Pt(rect.Min.X/2, rect.Max.Y-6)
			got := pixelAt(img, behind)
			if got == p.Dialog {
				t.Errorf("the page beside the dialog at %v reads %v, the dialog's own fill; the scrim is not dimming it", behind, got)
			}
			if want := vgcolor.Flatten(p.Cover, p.Backdrop); got != want {
				t.Errorf("the dimmed page at %v reads %v, want the platform's composite %v", behind, got, want)
			}
		})
	}
}

// TestTheBackdropReachesTheWindowsTopEdge pins that the band shows the
// full-bleed backdrop already painted under it rather than a second fill drawn
// over it, which is why the toolbar paints no fill. Leading of the toolbar's
// button the band must be the resting fill and nothing else: no page paint in
// it, and no unpainted glass.
func TestTheBackdropReachesTheWindowsTopEdge(t *testing.T) {
	band := int(bandDp)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", bandDp)
			content := tc.c.WindowBackground
			if got := PaletteFrom(tc.c).Backdrop; got != content {
				t.Fatalf("the backdrop paints %v and the window's own plane is %v", got, content)
			}
			bar := toolbarBox(img, content)
			for _, x := range []int{0, windowSize.X / 2, bar.Min.X - 1} {
				for _, y := range []int{0, band / 2, band - 1} {
					if got := pixelAt(img, image.Pt(x, y)); got != content {
						t.Errorf("band pixel at (%d,%d) = %v, want the window's own plane %v", x, y, got, content)
					}
				}
			}
		})
	}
}

// TestThePageStartsBelowTheStrip is the other half: the fill runs under the
// strip, the page does not. Read off the frame rather than off the inset, so
// that a page which grew a layer of its own outside the cap would fail here.
func TestThePageStartsBelowTheStrip(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", bandDp)
			if top := topmostDrawn(img, tc.c.WindowBackground); top < int(bandDp) {
				t.Errorf("the page paints row %d, inside the %d dp title-bar strip; only the window fill belongs there", top, int(bandDp))
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
	run := desktop.ButtonRunIn(bandDp)
	bottom := int(run.Leading + run.Diameter)
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c, "", bandDp)
			content := tc.c.WindowBackground
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

// TestTheDialogCoversTheStripToo is the exception the page makes for what is
// transient. The resting page starts below the strip; the scrim does not,
// because a cover with a strip-shaped hole in its top edge isolates nothing —
// the un-dimmed band would read as a seam across the window's top edge. The
// dialog and its cover are therefore laid out in the window's own
// coordinates.
func TestTheDialogCoversTheStripToo(t *testing.T) {
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			resting := renderWindow(t, tc.c, "", bandDp)
			covered := renderWindow(t, tc.c, "add.todo", bandDp)
			for _, x := range []int{0, windowSize.X / 2, windowSize.X - 1} {
				for _, y := range []int{0, int(bandDp) / 2, int(bandDp) - 1} {
					at := image.Pt(x, y)
					if pixelAt(covered, at) == pixelAt(resting, at) {
						t.Errorf("the strip at %v is unchanged with the dialog open; the scrim stops at the page's top edge", at)
					}
				}
			}
		})
	}
}

// TestTheStripMovesThePage guards the inset itself: a frame drawn under the
// strip is not the frame drawn without one. Without this the tests above would
// all still pass if the cap stopped insetting anything, since the first
// row's paint starts below the buttons' run on its own.
func TestTheStripMovesThePage(t *testing.T) {
	capped := renderWindow(t, tokens.PlatformLight, "", bandDp)
	bare := renderWindow(t, tokens.PlatformLight, "", 0)
	if n := golden.PixelDiff(capped, bare); n == 0 {
		t.Error("the window renders identically with and without a title-bar strip; the page is not being inset")
	}
}
