package main

// A whole-window render, headless, plus the surface-grammar assertions that
// read off it. The app has no offscreen mode of its own — it is a native
// window binary — but the layers the window renders are plain observables of
// layout.Widget, so composing them over a frozen theme and drawing them into
// a headless canvas produces the same frame the window would show, at the
// size the window opens at.
//
// A render of one column in isolation cannot see that a window's furniture
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
// question "what rung is this region wearing" has an answer.

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
// size these frames are drawn at.
var windowSize = image.Pt(1200, 800)

// schemes is the pair every rule below is checked against.
var schemes = []struct {
	name string
	c    tokens.ColorTokens
}{
	{"light", tokens.DefaultLight},
	{"dark", tokens.DefaultDark},
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

	// The pager, under the table: a leading chevron and then one
	// ControlHeight square per page. The current page's square spans
	// x 252–287, the page beside it 296–331; both are sampled at mid-height
	// and five pixels in from their leading edge, which clears the rounded
	// corners and the centred digit alike.
	atCurrentPage = image.Pt(257, 766) // the page the table is showing
	atRestingPage = image.Pt(301, 766) // the page beside it, unchosen
)

// TestWindowRegionsWearTheirRungs reads the surface grammar's assignment off
// the frame: content at level 0, the window's furniture at the CHROME level
// under it, the reading pane's own tab strip raised over the panel it caps,
// nothing resting at level 2.
//
// A sidebar and a navbar are the furniture this window's articles stand
// beside, so they are its darkest regions in both schemes: neutral 200 in the
// light scheme, #151515 in the dark one. The tab strip does not follow
// them — a sidebar is chrome standing beside the document, while a tab strip
// is the reading pane's own
// control band, drawn one level over the panel it belongs to (patterns/tabs
// walks it from Props.Ground). This window therefore carries regions on three
// levels at rest.
func TestWindowRegionsWearTheirRungs(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			chrome := tc.c.SurfaceAt(tokens.LevelChrome)
			ground := tc.c.SurfaceAt(tokens.Level0)
			raised := tc.c.SurfaceAt(tokens.Level1)
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
				{"sidebar", atSidebar, chrome},
				{"navbar", atNavbar, chrome},
				{"tab strip", atTabStrip, raised},
			} {
				got := at(img, r.at.X, r.at.Y)
				if got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
				// A resting region must not be painted at the floating
				// level. The check only bites where the two are different
				// fills: the light scheme has one band step above its
				// content and spends it on the first raise, so its raised
				// and floating levels are one colour and no pixel can tell
				// them apart.
				if r.want != transient && got == transient {
					t.Errorf("%s at %v rests at level 2 (%v), the level elevation keeps for what appears and leaves", r.name, r.at, transient)
				}
			}
		})
	}
}

// TestLightnessNeverFallsTowardTheViewer walks this window's depth axis rather
// than its plane: the sidebar is the window's furniture, the reading pane is
// the content beside it, the tab strip is the pane's own band raised over
// that content, and a dialog arrives over the lot. Walking that order toward
// the reader, lightness may never fall — in the light scheme AND in the dark
// one. Never fall rather than always rise: the light scheme has one band step
// above its content, so its raise and the dialog over it are both white, and
// what tells them apart is the dialog's shadow and scrim.
//
// Three of the four fills are read off the frame rather than off tokens,
// because they are painted by three different pieces of code — patterns/
// sidebar, this app's backdrop, and patterns/tabs — and a frame with all
// three in it is the only place they can be seen agreeing.
func TestLightnessNeverFallsTowardTheViewer(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			toward := []struct {
				name string
				fill color.NRGBA
			}{
				{"the sidebar's chrome", at(img, atSidebar.X, atSidebar.Y)},
				{"the reading pane's content", at(img, atReadingPane.X, atReadingPane.Y)},
				{"the tab strip's band", at(img, atTabStrip.X, atTabStrip.Y)},
				{"a dialog's surface", tc.c.SurfaceAt(tokens.Level2)},
			}
			for i := 1; i < len(toward); i++ {
				below, above := toward[i-1], toward[i]
				if luma(above.fill) < luma(below.fill) {
					t.Errorf("%s (%v) is under %s (%v); walking toward the viewer never gets darker",
						above.name, above.fill, below.name, below.fill)
				}
			}
			// The furniture is this window's darkest region. The navbar is in
			// it because this window's furniture is two regions on one level,
			// and a window that painted only one of them the floor would read
			// as a step across its own top edge.
			for _, furniture := range []struct {
				name string
				fill color.NRGBA
			}{
				{"sidebar", at(img, atSidebar.X, atSidebar.Y)},
				{"navbar", at(img, atNavbar.X, atNavbar.Y)},
			} {
				for _, other := range toward[1:] {
					if luma(furniture.fill) >= luma(other.fill) {
						t.Errorf("the %s (%v) is not darker than %s (%v); a window's furniture is its darkest region",
							furniture.name, furniture.fill, other.name, other.fill)
					}
				}
			}
		})
	}
}

// TestChosenItemsCarryThePrimaryTint reads off the frame that every mark in
// this window meaning "this is the one you are on" fills from the Primary
// ramp's tinted end, and that a neighbour nobody chose keeps its own region's
// ground. The window has three such marks — the feed the table is listing, the
// article the pane is showing, and the page the pager is on — asserted
// together, in both schemes, because a window may not answer one question in
// two tones.
//
// The mark must be the Primary STEP, never the Primary pin: the pin runs
// saturated in light (#723AD4) and pale in dark (#D0C4FF) while the step runs
// the other way (#D8CEFF / #3F0085), so a window mixing them swaps which mark
// is the pale one every time the scheme changes. Sampling both schemes is what
// sees that; one frame cannot.
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
				{"current page", atCurrentPage},
			} {
				got := at(img, r.at.X, r.at.Y)
				if got != tint {
					t.Errorf("%s at %v = %v, want the Primary tint %v", r.name, r.at, got, tint)
				}
				if got == tc.c.Primary {
					t.Errorf("%s at %v wears the Primary pin %v; the pin and the step invert against each other between schemes", r.name, r.at, got)
				}
			}
			// A neutral step may not stand in for the current item, so the
			// resting neighbour must NOT be tinted — and must be its own
			// region's ground rather than a walk of it.
			rest := at(img, atRestingFeed.X, atRestingFeed.Y)
			if rest == tint {
				t.Errorf("an unchosen feed at %v is tinted %v; the mark says nothing if every row wears it", atRestingFeed, rest)
			}
			if want := tc.c.SurfaceAt(tokens.LevelChrome); rest != want {
				t.Errorf("resting feed at %v = %v, want the sidebar's own ground %v", atRestingFeed, rest, want)
			}
			// The pager's resting cell says the same thing about the pager:
			// its own neutral fill, not the tint.
			restPage := at(img, atRestingPage.X, atRestingPage.Y)
			if restPage == tint {
				t.Errorf("an unchosen page at %v is tinted %v; only the page the table is showing may wear the mark", atRestingPage, restPage)
			}
			if want := tc.c.Ramps.Neutral.Step(300); restPage != want {
				t.Errorf("resting page at %v = %v, want the pager's neutral fill %v", atRestingPage, restPage, want)
			}
		})
	}
}

// TestFeedRowStatesKeepTheirInksApart covers what one rendered frame cannot
// show: a list has to say "the pointer is here" and "this is the one you are
// reading" at the same time, so the two fills may never be the same ink and
// the tint may never lose to the walk. The pill is drawn over a sentinel, so a
// state that painted nothing is caught too.
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
			// The walk is taken from the level the rows stand on — the
			// sidebar's chrome level — rather than named as a ramp index. In
			// the light scheme the two spell the same #D4D4D4; in the dark
			// one an index is a step off the wrong level entirely.
			walk := tc.c.StateAt(tokens.LevelChrome, tokens.StateHover)
			ground := tc.c.SurfaceAt(tokens.LevelChrome)

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

// TestTheSidebarClearsTheWindowButtons: with the native strip gone, the
// platform's three control buttons float over the top-leading corner of
// whatever the application drew there, and in this layout that corner belongs
// to the sidebar.
//
// Measured off this window's frames without the band: the first accordion
// section's header inks from row 17 and the sidebar's own content runs the
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
			surface := tc.c.SurfaceAt(tokens.LevelChrome)
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
//     ends where the content region begins. That edge must land exactly on
//     the band, which is what proves this app's restatement of
//     patterns/shell's navbar pin has not drifted from the pin itself.
//   - The leading half declares nothing, because the sidebar's fill runs the
//     whole column and the band is the same chrome as everything under it.
//     What can be seen there is where the sidebar starts drawing, which must
//     be at or below the band's foot — the band is held open, and wears the
//     sidebar's own ground while it is.
//
// Both halves are read off ONE level: a column painted at the Surface ALIAS
// while patterns/accordion sits on the floor stands a whole level over the
// sidebar beneath it on slate, and a scan looking for the alias cannot see
// that.
//
// Both densities are checked because the depth is the density's, not this
// app's: a band that only ever met Comfortable would pass while hard-coding
// 52. Checking two also turns the leading half's loose bound into an exact
// one. The accordion's own lead — the padding above its first section's caret
// — is the same at both densities, so the gap between the band's foot and the
// sidebar's first ink has to be the same at both as well. A sidebar reserving
// anything other than the band would open a different gap at 52 than at 40 and
// be caught here, without this test ever having to know what the accordion's
// lead is.
func TestTheWindowsTopStripIsOneBand(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			leads := make([]int, len(densities))
			for i, dc := range densities {
				img := renderWindowAt(t, tc.c, dc.d)
				band := int(windowBandDp(dc.d))
				surface := tc.c.SurfaceAt(tokens.LevelChrome)

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
