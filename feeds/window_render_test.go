package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. The app has no offscreen mode of its own — it is a native
// window binary — but the layers the window renders are plain observables of
// layout.Widget, so composing them over a frozen theme and drawing them into
// a headless canvas produces the same frame the window would show, at the
// size the window opens at.
//
// It is here because a composition can only be judged as a composition. A
// render of one column in isolation cannot see that a window's furniture
// stands level with the content it is meant to frame; this can. Run it with
// -window.dump=<dir> to write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/feeds
//
// Without the flag it still renders both schemes every run, which makes it a
// smoke test of the whole layer stack: a panic anywhere in the sidebar, the
// navbar, the articles table or the detail pane fails it.
//
// The grammar tests below sample the rendered frame rather than a palette
// struct, because this app holds no palette: each region paints its own fill
// at the point it draws. Sampling the frame is therefore the only place the
// question "what rung is this region wearing" has an answer, and it is the
// honest one — it sees what the window paints, not what it meant to.

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size the Feeds window opens at (main.go), and the only
// size these frames are drawn at: a composition is worth looking at where
// somebody actually looks at it.
var windowSize = image.Pt(1200, 800)

// schemes is the pair every rule below is stated once and checked twice
// against.
var schemes = []struct {
	name string
	c    tokens.ColorTokens
}{
	{"light", tokens.DefaultLight},
	{"dark", tokens.DefaultDark},
}

// densities is the pair the title band's depth is checked against. The live
// theme emits Comfortable and nothing else, so Compact appears here rather
// than in the window: the band's whole point is that it holds whatever depth
// patterns/shell pins the navbar to, and a band that only ever meets one
// density has not been asked the question.
var densities = []struct {
	name string
	d    tokens.Density
}{
	{"comfortable", tokens.Comfortable},
	{"compact", tokens.Compact},
}

// staticTheme freezes one colour scheme into a Theme whose every field emits
// once — the shape theme/window feeds the layers, minus the live OS poll.
func staticTheme(c tokens.ColorTokens, d tokens.Density) theme.Theme {
	return theme.Theme{
		Color:      rx.Of(c),
		Typography: rx.Of(tokens.DefaultTypography),
		Density:    rx.Of(d),
		Motion:     rx.Of(tokens.Motion),
		Spacing:    rx.Of(tokens.Spacing),
		Radius:     rx.Of(tokens.Radius),
		Elevation:  rx.Of(tokens.Elevation),
	}
}

// settledModel is the window as a reader actually leaves it: a feed chosen in
// the sidebar and one of its articles open in the detail pane. Both matter to
// the grammar — the chosen-item fill has nothing to paint until something is
// chosen.
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
// widget: the backdrop first, the shell over it, exactly as theme/window
// stacks them.
func windowFrame(t *testing.T, c tokens.ColorTokens, d tokens.Density, model Model) layout.Widget {
	t.Helper()
	layers := buildLayers(rx.Of(model))(rx.Of(staticTheme(c, d)))

	widgets := make([]layout.Widget, len(layers))
	for i, layer := range layers {
		w, err := collectOne(layer)
		if err != nil {
			t.Fatalf("layer %d never emitted a widget: %v", i, err)
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
// widget tree needs before any of them can report state, which is the same
// warm-up drawShellOnce does for the shell tests.
func renderWindow(t *testing.T, c tokens.ColorTokens) *image.RGBA {
	t.Helper()
	return renderWindowAt(t, c, tokens.Comfortable)
}

// renderWindowAt is renderWindow at a stated density — the input the title
// band's depth follows, and the one the live theme never varies.
func renderWindowAt(t *testing.T, c tokens.ColorTokens, d tokens.Density) *image.RGBA {
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
// chosen well clear of ink: the sidebar below its last feed, the navbar
// between the brand and the actions, the articles pane under the last row,
// a body row's empty trailing column, the reading pane's lower half. They
// are coordinates because this app holds no palette to interrogate — every
// region paints its own fill where it draws, so the frame is the only place
// the question has an answer.
var (
	atSidebar     = image.Pt(96, 400)            // sidebar, below the open section's feeds
	atNavbar      = image.Pt(600, 12)            // navbar, between the brand and the actions
	atListPane    = image.Pt(494, 640)           // articles pane, under the last row
	atListRow     = image.Pt(760, 209)           // second body row, past the Unread glyph
	atReadingPane = image.Pt(1000, 600)          // reading pane, below the article body
	atPaneHead    = image.Pt(900, 60)            // reading pane, beside the article title
	atTabStrip    = image.Pt(1100, 144)          // the tab strip band, past the last label
	atOpenFeed    = image.Pt(100, windowBand+61) // the open feed's pill, under the band
	atRestingFeed = image.Pt(100, windowBand+89) // the feed under it, unchosen and unhovered
	atOpenRow     = image.Pt(760, 173)           // the open article's row, past the glyph
)

// TestWindowRegionsWearTheirRungs reads ADR-021's assignment off the frame:
// content at level 0, furniture exactly one rung up, nothing resting at
// level 2. Before this task the window had no level-0 surface at all — the
// backdrop filled it with Surface and every region drawn over it was Surface
// too, so the sidebar and navbar stood zero rungs off the content they frame.
func TestWindowRegionsWearTheirRungs(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			ground := tc.c.SurfaceAt(tokens.Level0)
			furniture := tc.c.SurfaceAt(tokens.Level1)
			transient := tc.c.SurfaceAt(tokens.Level2)

			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"articles pane", atListPane, ground},
				{"article row", atListRow, ground},
				{"reading pane", atReadingPane, ground},
				{"reading pane header", atPaneHead, ground},
				{"sidebar", atSidebar, furniture},
				{"navbar", atNavbar, furniture},
				{"tab strip", atTabStrip, furniture},
			} {
				got := at(img, r.at.X, r.at.Y)
				if got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
				if got == transient {
					t.Errorf("%s at %v rests at level 2 (%v), the rung the ladder keeps for what appears and leaves", r.name, r.at, transient)
				}
			}
		})
	}
}

// TestRungsNeverDecreaseWalkingOut is the grammar's own check, applied to
// this window's resting fills: content, then the furniture around it, then
// the surface a dialog would arrive on. Each step out is a step further from
// the ground, which the paired ramps render as darker in the light scheme and
// lighter in the dark one — one rule, both schemes.
func TestRungsNeverDecreaseWalkingOut(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			out := []struct {
				name string
				fill color.NRGBA
			}{
				{"reading pane", at(img, atReadingPane.X, atReadingPane.Y)},
				{"sidebar", at(img, atSidebar.X, atSidebar.Y)},
				{"dialog surface", tc.c.SurfaceAt(tokens.Level2)},
			}
			dark := luma(tc.c.Background) < luma(tc.c.Ramps.Neutral.Step(500))
			for i := 1; i < len(out); i++ {
				in, next := out[i-1], out[i]
				step := luma(next.fill) - luma(in.fill)
				if dark && step <= 0 {
					t.Errorf("%s (%v) is not lighter than %s (%v); on slate every rung out lightens",
						next.name, next.fill, in.name, in.fill)
				}
				if !dark && step >= 0 {
					t.Errorf("%s (%v) is not darker than %s (%v); on paper every rung out darkens",
						next.name, next.fill, in.name, in.fill)
				}
			}
		})
	}
}

// TestChosenItemsCarryThePrimaryTint is R5 read off the frame: the feed the
// table is listing and the article the pane is showing both fill from the
// Primary ramp's tinted end, and a feed nobody chose keeps its region's own
// ground. The window had neither mark before this task — the sidebar drew
// every row in one ink and was never handed the selection, and the table
// painted no row fill at all, so the article on screen was unmarked in the
// list it came from.
func TestChosenItemsCarryThePrimaryTint(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			tint := tc.c.Ramps.Primary.Step(300)
			for _, r := range []struct {
				name string
				at   image.Point
			}{
				{"open feed", atOpenFeed},
				{"open article row", atOpenRow},
			} {
				if got := at(img, r.at.X, r.at.Y); got != tint {
					t.Errorf("%s at %v = %v, want the Primary tint %v", r.name, r.at, got, tint)
				}
			}
			// A neutral step standing in for the current item is what ADR-021
			// forbids, so the resting neighbour must NOT be tinted — and must
			// be its own region's ground rather than a walk of it.
			rest := at(img, atRestingFeed.X, atRestingFeed.Y)
			if rest == tint {
				t.Errorf("an unchosen feed at %v is tinted %v; the mark says nothing if every row wears it", atRestingFeed, rest)
			}
			if want := tc.c.SurfaceAt(tokens.Level1); rest != want {
				t.Errorf("resting feed at %v = %v, want the sidebar's own ground %v", atRestingFeed, rest, want)
			}
		})
	}
}

// TestFeedRowStatesKeepTheirInksApart is the other half of R5, which one
// rendered frame cannot show: a list has to say "the pointer is here" and
// "this is the one you are reading" at the same time, so the two fills may
// never be the same ink and the tint may never lose to the walk. The pill is
// drawn over a sentinel, so a state that painted nothing is caught too.
func TestFeedRowStatesKeepTheirInksApart(t *testing.T) {
	sentinel := color.NRGBA{R: 255, G: 0, B: 255, A: 255}
	size := image.Pt(160, 28)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			tok := themeTokens{
				col:    tc.c,
				typ:    tokens.DefaultTypography,
				shaper: tokens.DefaultTypography.DeterministicShaper(),
			}
			fill := func(selected, hovered bool) color.NRGBA {
				img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					paint.FillShape(gtx.Ops, sentinel, clip.Rect{Max: gtx.Constraints.Max}.Op())
					drawFeedEntryPill(gtx, tok, gtx.Constraints.Max, selected, hovered)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
				return at(img, 40, size.Y/2)
			}

			tint := tc.c.Ramps.Primary.Step(300)
			walk := tc.c.Ramps.Neutral.Step(300)
			ground := tc.c.SurfaceAt(tokens.Level1)

			if got := fill(false, false); got != sentinel {
				t.Errorf("a resting row painted %v; it must leave its region's own ground showing", got)
			}
			if got := fill(false, true); got != walk {
				t.Errorf("hovered row = %v, want the neutral walk %v off the sidebar's own ground %v", got, walk, ground)
			}
			if got := fill(true, false); got != tint {
				t.Errorf("open row = %v, want the Primary tint %v", got, tint)
			}
			if got := fill(true, true); got != tint {
				t.Errorf("hovered open row = %v, want the tint %v to hold; the pointer must not take the answer away", got, tint)
			}
			if tint == walk {
				t.Errorf("the chosen ink and the hover walk are the same colour %v; a row cannot say both things at once", tint)
			}
		})
	}
}

// topmostInkIn is the first row of the given box that holds a pixel other than
// ground, or -1 for a box that is nothing but ground. It is how a region's
// first drawn thing is found without knowing what that thing is — the question
// the band's assertions ask of the sidebar, whose whole column is one fill
// until something is drawn on it.
func topmostInkIn(img *image.RGBA, ground color.NRGBA, x0, x1, y0, y1 int) int {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if at(img, x, y) != ground {
				return y
			}
		}
	}
	return -1
}

// TestTheSidebarClearsTheWindowButtons is R6's first consequence read off this
// window: with the native strip gone, the platform's three control buttons
// float over the top-leading corner of whatever the application drew there,
// and in this layout that corner belongs to the sidebar.
//
// The collision it guards was real rather than hypothetical, and the audit
// found it. Measured off this window's frames before the band existed: the
// first accordion section's header inked from row 17 and the sidebar's own
// content ran the band's whole depth, while the buttons in a 52 dp band run
// rows 19 to 33 and reach 79 dp along (desktop.ButtonRunIn(52): leading 19,
// centre 26, trailing 79) — 184 pixels of the header's caret and name stood
// inside the run. The band is the whole of the clearance the corner now has;
// nothing in the sidebar is centred out of the buttons' way.
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
			surface := tc.c.SurfaceAt(tokens.Level1)
			for y := 0; y <= bottom; y++ {
				for x := 0; x <= int(run.Trailing); x++ {
					if got := at(img, x, y); got != surface {
						t.Fatalf("sidebar ink %v at (%d,%d), inside the window buttons' run (leading %v, trailing %v, centre %v)",
							got, x, y, run.Leading, run.Trailing, run.Center)
					}
				}
			}
			top := topmostInkIn(img, surface, 0, feedsSidebarWidthDp, 0, windowSize.Y)
			if top < 0 {
				t.Fatalf("the sidebar draws nothing at all; the clearance below the buttons cannot be judged")
			}
			if top <= bottom {
				t.Errorf("the sidebar's topmost ink is row %d and the buttons end at row %d; the sidebar has no clearance under them", top, bottom)
			}
			t.Logf("sidebar's topmost ink is row %d; the buttons run rows %v to %d", top, run.Leading, bottom)
		})
	}
}

// TestTheWindowsTopStripIsOneBand is R6's later half, which is the half this
// window had to answer for: its top edge is crossed by two regions, not one.
// The sidebar caps the leading side and the navbar caps the content region
// beside it, and the rule is that the two wear their own fills but hold one
// depth between them — a strip deeper on one side of the seam than the other
// is a step in the window's top edge rather than a band with a seam in it.
//
// Each half is measured off the frame, and neither is asked to agree with a
// number written down in this test:
//
//   - The trailing half declares its own depth, because the navbar's Surface
//     ends where the content region's ground begins. That edge must land
//     exactly on the band, which is what proves this app's restatement of
//     patterns/shell's navbar pin has not drifted from the pin itself.
//   - The leading half declares nothing, because the sidebar's fill runs the
//     whole column and the band is the same Surface as everything under it.
//     What can be seen there is where the sidebar starts drawing, which must
//     be at or below the band's foot — the band is held open, and wears the
//     sidebar's own ground while it is.
//
// Both densities are checked because the depth is the density's, not this
// app's: a band that only ever met Comfortable would pass while hard-coding
// 52. Checking two also turns the leading half's loose bound into an exact
// one. The accordion's own lead — the padding above its first section's
// caret — is whatever it is, but it is the same at both densities, so the gap
// between the band's foot and the sidebar's first ink has to be the same at
// both as well. A sidebar reserving anything other than the band would open a
// different gap at 52 than at 40 and be caught here, without this test ever
// having to know what the accordion's lead is.
func TestTheWindowsTopStripIsOneBand(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			leads := make([]int, len(densities))
			for i, dc := range densities {
				img := renderWindowAt(t, tc.c, dc.d)
				band := int(windowBandDp(dc.d))
				surface := tc.c.SurfaceAt(tokens.Level1)

				depth := -1
				for y := 0; y < windowSize.Y; y++ {
					if at(img, atNavbar.X, y) != surface {
						depth = y
						break
					}
				}
				if depth != band {
					t.Errorf("%s: the navbar's half of the strip is %d dp deep at x=%d, want the band's %d dp; the two halves of the window's top edge stand at different depths",
						dc.name, depth, atNavbar.X, band)
				}

				top := topmostInkIn(img, surface, 0, feedsSidebarWidthDp, 0, windowSize.Y)
				if top < 0 {
					t.Fatalf("%s: the sidebar draws nothing at all; its half of the strip cannot be judged", dc.name)
				}
				if top < band {
					t.Errorf("%s: the sidebar inks row %d, inside a band %d dp deep; its half of the strip is shallower than the navbar's beside it",
						dc.name, top, band)
				}
				leads[i] = top - band
				t.Logf("%s: band %d dp, navbar's fill ends at row %d, sidebar's first ink is row %d (%d dp below the band)",
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
