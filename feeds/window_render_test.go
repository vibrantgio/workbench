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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
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

// staticTheme freezes one colour scheme into a Theme whose every field emits
// once — the shape theme/window feeds the layers, minus the live OS poll.
func staticTheme(c tokens.ColorTokens) theme.Theme {
	return theme.Theme{
		Color:      rx.Of(c),
		Typography: rx.Of(tokens.DefaultTypography),
		Density:    rx.Of(tokens.Comfortable),
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
func windowFrame(t *testing.T, c tokens.ColorTokens, model Model) layout.Widget {
	t.Helper()
	layers := buildLayers(rx.Of(model))(rx.Of(staticTheme(c)))

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
	w := windowFrame(t, c, settledModel())
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

// Sample points in the rendered window, in the pixels the frame is drawn at
// (PxPerDp is 1, so a dp is a pixel). Each names a resting expanse and is
// chosen well clear of ink: the sidebar below its last feed, the navbar
// between the brand and the actions, the articles pane under the last row,
// a body row's empty trailing column, the reading pane's lower half. They
// are coordinates because this app holds no palette to interrogate — every
// region paints its own fill where it draws, so the frame is the only place
// the question has an answer.
var (
	atSidebar     = image.Pt(96, 400)   // sidebar, below the open section's feeds
	atNavbar      = image.Pt(600, 12)   // navbar, between the brand and the actions
	atListPane    = image.Pt(494, 640)  // articles pane, under the last row
	atListRow     = image.Pt(760, 209)  // second body row, past the Unread glyph
	atReadingPane = image.Pt(1000, 600) // reading pane, below the article body
	atPaneHead    = image.Pt(900, 60)   // reading pane, beside the article title
	atTabStrip    = image.Pt(1100, 144) // the tab strip band, past the last label
	atOpenFeed    = image.Pt(100, 61)   // the open feed's pill
	atRestingFeed = image.Pt(100, 89)   // the feed under it, unchosen and unhovered
	atOpenRow     = image.Pt(760, 173)  // the open article's row, past the glyph
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
