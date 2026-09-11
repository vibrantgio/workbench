package main

// A whole-window render, headless, plus the assertions that read the
// platform's names off it. The app has no offscreen mode of its own — it is a native
// window binary — but the layers the window renders are plain observables of
// layout.Widget, so composing them over a frozen theme and drawing them into
// a headless image produces the same frame the window would show, at the
// size the window opens at.
//
// A render of one column in isolation cannot see that a window's chrome and
// the content it frames have come out the same fill; this can. Run it with
// -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/feeds
//
// Without the flag it still renders both schemes every run, which makes it a
// smoke test of the whole layer stack: a panic anywhere in the sidebar, the
// navbar, the articles table or the detail pane fails it.
//
// The tests below sample the rendered frame rather than the token set,
// because this app holds no palette: each region paints its own fill at the
// point it draws. Sampling the frame is therefore the only place the question
// "which platform name is this region wearing" has an answer.

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

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Feeds window opens at (main.go), and the only
// size these frames are drawn at.
var windowSize = image.Pt(1200, 800)

// schemes is the pair every rule below is checked against.
var schemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// densities is the pair the title band's depth is checked against. The live
// theme emits Comfortable and nothing else, so Compact appears here rather
// than in the window: the band has to hold whatever depth patterns/shell pins
// the navbar to, which a single density cannot show.
var densities = []struct {
	name string
	d    tokens.Density
}{
	{"comfortable", tokens.Comfortable},
	{"compact", tokens.Compact},
}

// staticTheme freezes one colour scheme into a Theme whose every field emits
// once — the shape theme/window feeds the layers, minus the live OS poll.
func staticTheme(c tokens.PlatformColors, d tokens.Density) theme.Theme {
	return theme.Theme{
		Platform:   rx.Of(c),
		Typography: rx.Of(tokens.DefaultTypography),
		Density:    rx.Of(d),
		Motion:     rx.Of(tokens.Motion),
		Spacing:    rx.Of(tokens.Spacing),
		Radius:     rx.Of(tokens.Radius),
		Elevation:  rx.Of(tokens.Elevation),
	}
}

// settledModel is the window as a reader leaves it: a feed chosen in the
// sidebar and one of its articles open in the detail pane. The chosen-item
// fill has nothing to paint until something is chosen.
func settledModel() Model {
	m := initialModel()
	m, _ = Update(m, SelectArticle{Article: settledArticle})
	return m
}

// settledArticle is the first row of the default feed's first page under the
// model's seed sort (Published, descending), so the article the detail pane
// shows is the top row of the table beside it.
var settledArticle = func() ArticleID {
	seed := initialModel()
	rows := filterAndSortArticles(hardCodedArticles(), seed.selectedFeed, "", seed.sort)
	if len(rows) == 0 {
		return ""
	}
	return rows[0].ID
}()

// windowFrame composes the window's layers for one scheme into a single
// layout.Widget: the backdrop first, the shell over it, exactly as
// theme/window stacks them.
func windowFrame(t *testing.T, c tokens.PlatformColors, d tokens.Density, model Model) layout.Widget {
	t.Helper()
	layers := buildLayers(rx.Of(model))(rx.Of(staticTheme(c, d)))

	widgets := make([]layout.Widget, len(layers))
	for i, layer := range layers {
		w, err := collectOne(layer)
		if err != nil {
			t.Fatalf("layer %d never emitted a layout.Widget: %v", i, err)
		}
		widgets[i] = w
	}

	return func(gtx layout.Context) layout.Dimensions {
		for _, w := range widgets {
			w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// renderWindow draws the settled window in one scheme. Two frames are drawn
// and the second is kept: the first registers the pointer and click tags the
// layout tree needs before any of them can report state, which is the same
// warm-up drawShellOnce does for the shell tests.
func renderWindow(t *testing.T, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	return renderWindowAt(t, c, tokens.Comfortable)
}

// renderWindowAt is renderWindow at a stated density — the input the title
// band's depth follows, and the one the live theme never varies.
func renderWindowAt(t *testing.T, c tokens.PlatformColors, d tokens.Density) *image.RGBA {
	t.Helper()
	w := windowFrame(t, c, d, settledModel())
	golden.Capture(t, windowSize, w)
	return golden.Capture(t, windowSize, w)
}

// at reads one pixel as an opaque NRGBA, which is what every token in the set
// is.
func at(img *image.RGBA, x, y int) color.NRGBA {
	r, g, b, _ := img.At(x, y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
}

// luma is the Rec. 601 brightness of a fill, the axis "lighter" and "darker"
// are measured on below.
func luma(c color.NRGBA) float32 {
	return 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
}

// TestWholeWindowRender draws the composed window in both schemes, and writes
// the frames out when -window.dump names a directory.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range schemes {
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
			path := filepath.Join(*windowDump, "feeds-"+tc.name+".png")
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

// windowBand is the depth of the title band these frames are drawn with: the
// strip the sidebar and the navbar hold open across the window's top edge, at
// the density staticTheme is given below. The sidebar's sample points are
// stated from it rather than from the window's top edge, because everything
// the sidebar draws begins under the band.
var windowBand = int(windowBandDp(tokens.Comfortable))

// Sample points in the rendered window, in the pixels the frame is drawn at
// (PxPerDp is 1, so a dp is a pixel). Each names a resting expanse and is
// chosen well clear of paint: the sidebar below its last feed, the navbar
// between the brand and the actions, the articles pane under the last row,
// a body row's empty trailing column, the reading pane's lower half. They
// are coordinates because this app holds no palette to interrogate — every
// region paints its own fill where it draws, so the frame is the only place
// the question has an answer.
var (
	atSidebar     = image.Pt(96, 10)             // the sidebar's own band, above the accordion
	atNavbar      = image.Pt(600, 12)            // navbar, between the brand and the actions
	atListPane    = image.Pt(494, 640)           // articles pane, under the last row
	atListRow     = image.Pt(760, 153)           // a body row the stripe skips, past the glyph
	atReadingPane = image.Pt(1000, 600)          // reading pane, below the article body
	atPaneHead    = image.Pt(900, 60)            // reading pane, beside the article title
	atTabStrip    = image.Pt(1100, 114)          // the tab strip band, past the last label
	atOpenFeed    = image.Pt(100, windowBand+63) // the open feed's pill, under the band
	atRestingFeed = image.Pt(100, windowBand+95) // the feed under it, unchosen
	atOpenRow     = image.Pt(760, 113)           // the open article's row, past the glyph

	// The pager, under the table: a leading chevron and then one square per
	// page. Only the page the table is showing is filled; the others carry
	// the pane's own fill, so the resting sample is taken in the gap between
	// two squares.
	atCurrentPage = image.Pt(247, 762) // the page the table is showing
	atRestingPage = image.Pt(280, 762) // the pager beside it, unfilled
)

// TestWindowRegionsWearThePlatformNames reads off the frame that every
// resting expanse of this window wears the platform's name for what it is:
// the content regions ControlBackground, the chrome regions — the sidebar,
// the navbar and the reading pane's tab strip — the chrome material.
//
// It is read off the frame rather than off the token set because the three
// chrome regions are painted by three different pieces of code (this app's
// sidebar, patterns/shell's navbar band, patterns/tabs' strip) and a frame
// with all of them in it is the only place they can be seen agreeing.
func TestWindowRegionsWearThePlatformNames(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"articles pane", atListPane, tc.c.ControlBackground},
				{"article row", atListRow, tc.c.ControlBackground},
				{"reading pane", atReadingPane, tc.c.ControlBackground},
				{"reading pane header", atPaneHead, tc.c.ControlBackground},
				{"sidebar", atSidebar, tc.c.SidebarMaterial},
				{"navbar", atNavbar, tc.c.SidebarMaterial},
				{"tab strip", atTabStrip, tc.c.SidebarMaterial},
			} {
				if got := at(img, r.at.X, r.at.Y); got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
			}
		})
	}
}

// TestChosenItemsWearThePlatformsSelection reads off the frame that every
// mark in this window meaning "this is the one you are on" is the platform's
// own answer for that kind of row, and that a neighbour nobody chose keeps
// its own region's fill.
//
// The three marks are three different names on this platform, which is the
// point of asserting them together: a sidebar row's pill follows the accent,
// a content list's current row wears the emphasized selection, and the pager
// fills the page it is on with the accent under the foreground the platform
// pairs with it. Both schemes are sampled, because a window may not answer
// one question in two ways.
func TestChosenItemsWearThePlatformsSelection(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"open feed", atOpenFeed, patsidebar.SelectionFill(tc.c, false)},
				{"open article row", atOpenRow, tc.c.SelectedContentBackground},
				{"current page", atCurrentPage, tc.c.ControlAccent},
			} {
				if got := at(img, r.at.X, r.at.Y); got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
			}
			// A resting row takes no fill of its own: the platform tints
			// neither a sidebar row nor a list row, so each shows whatever
			// its region already painted. patterns/accordion paints the
			// rail's body, which is why the resting feed reads the content's
			// fill rather than the chrome the sidebar's own band wears.
			if got, want := at(img, atRestingFeed.X, atRestingFeed.Y), tc.c.ControlBackground; got != want {
				t.Errorf("resting feed at %v = %v, want the fill under it %v", atRestingFeed, got, want)
			}
			if got, want := at(img, atRestingPage.X, atRestingPage.Y), tc.c.ControlBackground; got != want {
				t.Errorf("the pager beside the current page at %v = %v, want the pane's own fill %v", atRestingPage, got, want)
			}
		})
	}
}

// topmostDrawnIn is the first row of the given box that holds a pixel other
// than the region's fill, or -1 for a box that is nothing but that fill. It
// is how a region's first drawn thing is found without knowing what that
// thing is — the question the band's assertions ask of the sidebar, whose
// whole column is one fill until something is drawn on it.
func topmostDrawnIn(img *image.RGBA, surface color.NRGBA, x0, x1, y0, y1 int) int {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if at(img, x, y) != surface {
				return y
			}
		}
	}
	return -1
}

// TestTheSidebarClearsTheWindowButtons: with the native strip gone, the
// platform's three control buttons float over the top-leading corner of
// whatever the application drew there, and in this layout that corner belongs
// to the sidebar.
//
// Measured off this window's frames without the band: the first accordion
// section's header paints from row 17 and the sidebar's own content runs the
// band's whole depth, while the buttons in a 52 dp band run rows 19 to 33 and
// reach 79 dp along (desktop.ButtonRunIn(52): leading 19, centre 26, trailing
// 79) — 184 pixels of the header's caret and name inside the run. The band is
// the whole of the clearance the corner has; nothing in the sidebar is centred
// out of the buttons' way.
//
// The run is desktop's derivation of the platform's rule rather than a guess
// at where the circles are, and the clearance is asserted off the frame rather
// than trusted to the arithmetic that produced it.
func TestTheSidebarClearsTheWindowButtons(t *testing.T) {
	run := desktop.ButtonRunIn(windowBandDp(tokens.Comfortable))
	bottom := int(run.Leading + run.Diameter)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			surface := tc.c.SidebarMaterial
			for y := 0; y <= bottom; y++ {
				for x := 0; x <= int(run.Trailing); x++ {
					if got := at(img, x, y); got != surface {
						t.Fatalf("sidebar paint %v at (%d,%d), inside the window buttons' run (leading %v, trailing %v, centre %v)",
							got, x, y, run.Leading, run.Trailing, run.Center)
					}
				}
			}
			top := topmostDrawnIn(img, surface, 0, feedsSidebarWidthDp, 0, windowSize.Y)
			if top < 0 {
				t.Fatalf("the sidebar draws nothing at all; the clearance below the buttons cannot be judged")
			}
			if top <= bottom {
				t.Errorf("the sidebar's topmost paint is row %d and the buttons end at row %d; the sidebar has no clearance under them", top, bottom)
			}
			t.Logf("sidebar's topmost paint is row %d; the buttons run rows %v to %d", top, run.Leading, bottom)
		})
	}
}

// TestTheWindowsTopStripIsOneBand covers a top edge crossed by two regions,
// not one. The sidebar caps the leading side and the navbar caps the content
// region beside it, and the two wear their own fills but hold one depth
// between them — a strip deeper on one side of the seam than the other is a
// step in the window's top edge rather than a band with a seam in it.
//
// Each half is measured off the frame, and neither is asked to agree with a
// number written down in this test:
//
//   - The trailing half declares its own depth, because the navbar's chrome
//     ends where the content region begins. patterns/navbar closes its band
//     with a seam along its own foot, so the last row of flat chrome is the
//     one above that line and the edge lands at band-1. That is what proves
//     this app's restatement of patterns/shell's navbar pin has not drifted
//     from the pin itself.
//   - The leading half declares nothing, because the sidebar's own fill runs
//     the whole column under the accordion. What can be seen there is where
//     the sidebar starts drawing, which must be at or below the band's foot —
//     the band is held open, and wears the sidebar's own fill while it is.
//
// Both densities are checked because the depth is the density's, not this
// app's: a band that only ever met Comfortable would pass while hard-coding
// its depth. Checking two also turns the leading half's loose bound into an exact
// one. The accordion's own lead — the padding above its first section's caret
// — is the same at both densities, so the gap between the band's foot and the
// sidebar's first paint has to be the same at both as well. A sidebar that
// reserved anything other than the band would open a different gap at 52 than
// at 40 and be caught here, without this test ever having to know what the
// accordion's lead is.
func TestTheWindowsTopStripIsOneBand(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			leads := make([]int, len(densities))
			for i, dc := range densities {
				img := renderWindowAt(t, tc.c, dc.d)
				band := int(windowBandDp(dc.d))
				surface := tc.c.SidebarMaterial

				depth := -1
				for y := 0; y < windowSize.Y; y++ {
					if at(img, atNavbar.X, y) != surface {
						depth = y
						break
					}
				}
				if depth != band-1 {
					t.Errorf("%s: the navbar's flat chrome ends at row %d at x=%d, want the row above the seam that closes a %d dp band; the two halves of the window's top edge stand at different depths",
						dc.name, depth, atNavbar.X, band)
				}

				top := topmostDrawnIn(img, surface, 0, feedsSidebarWidthDp, 0, windowSize.Y)
				if top < 0 {
					t.Fatalf("%s: the sidebar draws nothing at all; its half of the strip cannot be judged", dc.name)
				}
				if top < band {
					t.Errorf("%s: the sidebar paints row %d, inside a band %d dp deep; its half of the strip is shallower than the navbar's beside it",
						dc.name, top, band)
				}
				leads[i] = top - band
				t.Logf("%s: band %d dp, navbar's fill ends at row %d, sidebar's first paint is row %d (%d dp below the band)",
					dc.name, band, depth, top, leads[i])
			}
			for i := 1; i < len(leads); i++ {
				if leads[i] != leads[0] {
					t.Errorf("the sidebar starts drawing %d dp below a %s band and %d dp below a %s one; the depth it holds open is not the band's",
						leads[0], densities[0].name, leads[i], densities[i].name)
				}
			}
		})
	}
}

// TestTheBandIsTheDensitysBarHeight states the arithmetic the frames above
// measure, so a failure says which of the two is wrong. The band is the
// density's bar height — ControlHeight + 2·PaddingY — which is what
// patterns/shell pins its navbar slot to, and the window buttons' whole
// geometry falls out of that one number through the platform's centring rule.
func TestTheBandIsTheDensitysBarHeight(t *testing.T) {
	for _, dc := range densities {
		want := unit.Dp(dc.d.ControlHeight + 2*dc.d.PaddingY)
		if got := windowBandDp(dc.d); got != want {
			t.Errorf("%s band = %v, want the density's bar height %v", dc.name, got, want)
		}
	}
	run := desktop.ButtonRunIn(windowBandDp(tokens.Comfortable))
	if windowButtonRun != run {
		t.Errorf("the window buttons are placed at %+v, want the run derived from the band %+v", windowButtonRun, run)
	}
	if windowButtonRun.Center != windowBandDp(tokens.Comfortable)/2 {
		t.Errorf("the buttons' centre line is %v in a band %v deep; they are not centred in the band they stand in",
			windowButtonRun.Center, windowBandDp(tokens.Comfortable))
	}
}
