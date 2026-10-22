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
// toolbar, the articles table or the article's own column fails it.
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
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
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

// densities is the pair the toolbar's depth is checked against. The live
// theme emits Comfortable and nothing else, so Compact appears here rather
// than in the window: the strip has to hold whatever depth patterns/shell
// pins the toolbar to, which a single density cannot show.
var densities = []struct {
	name string
	d    tokens.Density
}{
	{"comfortable", tokens.Comfortable},
	{"compact", tokens.Compact},
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

// frame composes the window's layers for one scheme into a single
// layout.Widget, through the library's whole-window renderer: the layer stack
// feeds builds for its window, over a frozen theme, laid out as the window
// stacks it.
//
// The arrangement memory is nil, which is a working one: a frame laid out
// for a picture rather than for a reader opens on this window's defaults and
// writes over nothing a real window kept.
func frame(t *testing.T, c tokens.PlatformColors, d tokens.Density, model Model) layout.Widget {
	t.Helper()
	th := rx.Of(golden.StaticTheme(c, golden.WithDensity(d)))
	return golden.Layers(t, buildLayers(rx.Of(model), nil, desktop.ScrollBarsWhenScrolling)(th)...)
}

// The macOS window controls, as this window will actually carry them: the
// leading inset, diameter and pitch the stored platform reference measured
// (19 in, 14 across, 23 apart, so the third circle's trailing edge lands at
// 79). A headless frame has no window behind it, so it is told none of this
// unless it says so — LeadingInset reports 0 with no window — and every
// frame in this package is drawn with the run the platform's buttons will
// really claim. The circles themselves are the window's and are not drawn:
// what the pane reserves for them is.
const (
	buttonLeadDp     = 19
	buttonDiameterDp = 14
	buttonPitchDp    = 23
	buttonsEndDp     = buttonLeadDp + buttonDiameterDp + 2*buttonPitchDp
)

func TestMain(m *testing.M) {
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }
	os.Exit(m.Run())
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
	w := frame(t, c, d, settledModel())
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
// chosen well clear of paint: the pane's own strip, each share of the
// toolbar between what stands at its two ends, the list column under its
// last row, a body row the stripe skips, the article's own column. They are
// coordinates because this app holds no palette to interrogate — every
// region paints its own fill where it draws, so the frame is the only place
// the question has an answer.
var (
	atSidebar     = image.Pt(96, 20)    // the pane's own strip, above its first section
	atListToolbar = image.Pt(400, 12)   // the toolbar's share over the list, past the feed's name
	atToolbar     = image.Pt(900, 12)   // the toolbar's share over the article, before its acts
	atListColumn  = image.Pt(400, 640)  // the list column, clear of the toolbar's own content
	atListPane    = image.Pt(500, 640)  // the list column, under the last row
	atListRow     = image.Pt(460, 121)  // a body row the stripe skips
	atReadingPane = image.Pt(1000, 600) // the article's column, below its body
	atPaneHead    = image.Pt(1100, 100) // the article's column, beside its title
	atTabStrip    = image.Pt(1150, 132) // the tab strip band, past the last title
	// Both feed samples are taken in the air between the row's symbol box
	// and the column its name starts in, which every row keeps clear
	// whatever it is called. The rows begin under the pane's own strip and
	// the first section's heading block.
	atOpenFeed    = image.Pt(52, 100) // the open feed's pill
	atRestingFeed = image.Pt(52, 132) // the feed under it, unchosen
	atOpenRow     = image.Pt(460, 81) // the open article's row

	// The pager, the list column's own footer: a leading chevron and then
	// one square per page. Only the page the table is showing is filled;
	// the others carry the column's own fill, so the resting sample is
	// taken in the gap between two squares.
	atCurrentPage = image.Pt(278, 780) // the page the table is showing
	atRestingPage = image.Pt(302, 780) // the pager beside it, unfilled
)

// TestWindowRegionsWearThePlatformNames reads off the frame that every
// resting expanse of this window wears the platform's name for what it is:
// the document regions ControlBackground — the toolbar's two shares
// included, since a toolbar carries no fill and what stands in it stands on
// the column underneath — and the chrome regions, the pane and the
// article's tab strip, the chrome material.
//
// It is read off the frame rather than off the token set because the
// regions are painted by different pieces of code (this app's rail,
// patterns/shell's arrangement, patterns/table, patterns/tabs' strip) and a
// frame with all of them in it is the only place they can be seen agreeing.
func TestWindowRegionsWearThePlatformNames(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			for _, r := range []struct {
				name string
				at   image.Point
				want color.NRGBA
			}{
				{"list column", atListPane, tc.c.ControlBackground},
				{"article row", atListRow, tc.c.ControlBackground},
				{"the toolbar's share over the list", atListToolbar, tc.c.ControlBackground},
				{"the toolbar's share over the article", atToolbar, tc.c.ControlBackground},
				{"reading pane", atReadingPane, tc.c.ControlBackground},
				{"reading pane header", atPaneHead, tc.c.ControlBackground},
				{"sidebar", atSidebar, tc.c.SidebarMaterial},
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
// point of asserting them together: a sidebar row's pill is the rail's own,
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
				// The rail holds no keyboard in a still render, so the open
				// feed wears the platform's other pill: the grey one.
				{"open feed", atOpenFeed, patsidebar.SelectionFill(tc.c, true)},
				{"open article row", atOpenRow, tc.c.SelectedContentBackground},
				{"current page", atCurrentPage, tc.c.ControlAccent},
			} {
				if got := at(img, r.at.X, r.at.Y); got != r.want {
					t.Errorf("%s at %v = %v, want %v", r.name, r.at, got, r.want)
				}
			}
			// A resting row takes no fill of its own: the platform tints
			// neither a sidebar row nor a list row, so each shows whatever
			// its region already painted. The rail's rows stand on the
			// pane's own chrome material and on nothing else — no section's
			// body paints the content's fill over them.
			if got, want := at(img, atRestingFeed.X, atRestingFeed.Y), tc.c.SidebarMaterial; got != want {
				t.Errorf("resting feed at %v = %v, want the pane's own fill under it %v", atRestingFeed, got, want)
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

// TestTheRailsStripClearsTheWindowButtons: with the native strip gone, the
// platform's three control buttons float over the top-leading corner of
// whatever the application drew there, and in this layout that corner is
// INSIDE the rail's pane. The pane's own top strip is cut to hold them
// where the window puts them, and nothing of this window's stands in the
// run they claim.
//
// The clearance is read inside the pane rather than over the whole window,
// because the pane's rim and the surface around it are drawing the buttons
// stand over by design: the buttons are the window's and the pane slid in
// under them. What must be clear is the pane's INTERIOR over the buttons'
// own run, across the strip's whole depth. The run is where the buttons
// end plus the gap the strip leaves after them; what stands past it is the
// Toggle Sidebar button at the strip's trailing corner, which is the one
// thing the pane does draw up there.
//
// The run is patterns/pane's statement of the platform's measured inset
// rather than a guess at where the circles are, and the clearance is asserted
// off the frame rather than trusted to the arithmetic that produced it.
func TestTheRailsStripClearsTheWindowButtons(t *testing.T) {
	margin := int(pane.MarginDp)
	rim := int(pane.RimDp)
	strip := margin + int(pane.StripDp)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			surface := tc.c.SidebarMaterial
			// The straight run of the pane's leading edge, as far along as
			// the buttons and the gap after them reach.
			x0 := margin + int(pane.RadiusDp)
			x1 := int(buttonsEndDp) + int(pane.ButtonGapDp)
			for y := margin + rim; y < strip; y++ {
				for x := x0; x < x1; x++ {
					if got := at(img, x, y); got != surface {
						t.Fatalf("the pane paints %v at (%d,%d), inside the run the window's three buttons claim, which ends at %d and runs to row %d",
							got, x, y, x1, strip)
					}
				}
			}
			top := topmostDrawnIn(img, surface, x0, x1, margin+rim, windowSize.Y)
			if top < 0 {
				t.Fatalf("the pane draws nothing at all; the clearance below the buttons cannot be judged")
			}
			if top < strip {
				t.Errorf("the pane's topmost paint under the buttons is row %d and its strip runs to row %d; the strip is not clear", top, strip)
			}
			t.Logf("the pane's topmost paint under the buttons is row %d; its strip runs rows %d to %d", top, margin+rim, strip)
		})
	}
}

// TestTheWindowsTopStripIsOneBand covers a top edge crossed by three
// regions, not one. The rail's pane caps the leading side, and the
// toolbar's two shares cap the list column and the article beside it. The
// pane and the toolbar hold one LINE between them rather than one depth:
// the pane stands one margin inside the window's top edge, so its own strip
// is the band less a margin at each end and the buttons' centre falls on
// the middle of both.
//
// Each half is measured off the frame, and neither is asked to agree with a
// number written down in this test:
//
//   - The trailing half declares its depth by where the list column's own
//     content begins: a toolbar carries no fill, so the column's fill runs
//     up behind it and what can be seen is where the table starts, which
//     must be at or below the toolbar's foot.
//   - The leading half declares nothing, because the pane's own fill runs
//     the whole column under the strip. What can be seen there is where the
//     pane starts drawing, which must be at or below the strip's foot.
//
// Both densities are checked because the band is NOT the density's: this
// window's strip is the platform's measured one, so a density change must
// move neither half.
func TestTheWindowsTopStripIsOneBand(t *testing.T) {
	band := int(shell.ToolbarDepth)
	strip := int(pane.MarginDp) + int(pane.StripDp)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			for _, dc := range densities {
				img := renderWindowAt(t, tc.c, dc.d)

				// Down a column of the list clear of the toolbar's own
				// content — past the filter field's trailing edge and short
				// of the seam — the first thing drawn is the table's header
				// row, which begins at the toolbar's foot.
				depth := -1
				for y := 0; y < windowSize.Y; y++ {
					if at(img, atListColumn.X, y) != tc.c.ControlBackground {
						depth = y
						break
					}
				}
				if depth < band {
					t.Errorf("%s: the list column draws at row %d at x=%d, inside a toolbar %d dp deep",
						dc.name, depth, atListColumn.X, band)
				}

				top := topmostDrawnIn(img, tc.c.SidebarMaterial,
					int(pane.MarginDp)+int(pane.RadiusDp), int(buttonsEndDp)+int(pane.ButtonGapDp),
					int(pane.MarginDp)+int(pane.RimDp), windowSize.Y)
				if top < 0 {
					t.Fatalf("%s: the pane draws nothing at all; its half of the strip cannot be judged", dc.name)
				}
				if top < strip {
					t.Errorf("%s: the pane paints row %d, inside a strip %d dp deep; its half of the window's top edge is shallower than the toolbar's",
						dc.name, top, strip)
				}
				t.Logf("%s: band %d, the list column's first paint is row %d, the pane's strip ends at row %d and its first paint is row %d",
					dc.name, band, depth, strip, top)
			}
		})
	}
}

// TestTheBandIsThePlatformsAndNotADensitys states the arithmetic the frames
// above measure, so a failure says which of the two is wrong.
//
// The band is 52, and no density settles it: the
// three window control buttons stand a MEASURED nineteen dp in from the
// window's own glass on both axes, so the band that holds them centred is
// nineteen above a fourteen dp circle and nineteen below it. patterns/pane
// states that once and patterns/shell's toolbar reads it. The rail's half of
// the strip is the pane's own, which the pane's margin cuts from the same
// inset, so the two halves hold one LINE between them — the buttons' centre
// — rather than one depth.
func TestTheBandIsThePlatformsAndNotADensitys(t *testing.T) {
	band := unit.Dp(shell.ToolbarDepth)
	if got, want := band, unit.Dp(pane.BandDp); got != want {
		t.Errorf("band = %v, want the platform's measured band %v", got, want)
	}
	if pane.Buttons.Center != band/2 {
		t.Errorf("the buttons' centre line is %v in a band %v deep; they are not centred in the band they stand in",
			pane.Buttons.Center, band)
	}
	if strip := unit.Dp(pane.MarginDp) + unit.Dp(pane.StripDp)/2; strip != pane.Buttons.Center {
		t.Errorf("the pane's strip centres on %v and the band on %v; the two halves of the window's top edge stand on different lines",
			strip, pane.Buttons.Center)
	}
}

// TestFeedsWindowGolden stores the composed window in both appearances: the
// rail's pane set into the window's surface, one section open and the two
// below it collapsed, the article list beside the pane at the platform's
// own width, the article in the column that takes the rest, and one toolbar
// across the two of them with a share each.
//
// A picture of a slot cannot show a defect that lives in the composition —
// a nick of plane behind a rounded corner, a shadow painted before the column
// it falls on, a band standing at two depths across one top edge — so the
// composition gets a picture of its own. The pane's corners are anti-aliased
// and the golden carries those pixels: the sharp-radius trick the slot
// goldens use covers a component's own radius, not the rounded box the frame
// draws.
//
// It is taken through the LIVE layers, where every other golden in this
// package is taken through a static render path with the pinned shaper. This
// window has no static path — its layers are the only composition of it that
// exists — so the frames are shaped with the theme's own shaper. Everything
// drawn here is Latin and the embedded Roboto leads the collection, so no run
// in this window ever reaches the platform's fonts; a window that grew a rune
// Roboto does not carry would need the static path before it could be stored.
func TestFeedsWindowGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			golden.Compare(t, "window-"+tc.name, renderWindow(t, tc.c))
		})
	}
}

// TestTheRailIsAPaneAndDrawsNoSeam reads the rail's two boundaries off the
// frame. An inset object needs no seam — the rim and the plane around it do
// that work — so what must stand down the pane's trailing edge is the
// platform's rim and not a separator, and what must stand on the leading side
// is the window's own plane.
//
// It also reads that no line runs across the rail at all. The rail's groups
// used to be headed by a row with a full-width hairline under it; the
// platform parts a section from what stands above it by air alone, and a row
// of one colour spanning the pane's interior is what that defect looks like
// in a frame.
func TestTheRailIsAPaneAndDrawsNoSeam(t *testing.T) {
	margin, rim := int(pane.MarginDp), int(pane.RimDp)
	radius := int(pane.RadiusDp)
	edge := int(feedsPaneColumnDp) - rim
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			seam := vgcolor.Flatten(tc.c.Separator, tc.c.SidebarMaterial)
			for _, y := range []int{margin + radius, windowSize.Y / 2, windowSize.Y - margin - radius - 1} {
				if got := at(img, edge, y); got != tc.c.PaneRim {
					t.Errorf("the pane's trailing edge at (%d,%d) = %v, want the platform's rim %v", edge, y, got, tc.c.PaneRim)
				}
				if got := at(img, edge, y); got == seam {
					t.Errorf("the pane's trailing edge at (%d,%d) wears the separator; an inset object parts from nothing with a line", edge, y)
				}
				if got := at(img, 0, y); got == tc.c.SidebarMaterial {
					t.Errorf("the window's leading margin at (0,%d) wears the pane's own fill; the surface does not show around it", y)
				}
			}
			// No line across the rail: inside the pane, below its strip, no
			// row is one colour other than the pane's own fill. The scan
			// runs the pane's WHOLE interior, so the selection pill — inset
			// ten from each of the pane's edges — never reads as one.
			x0, x1 := margin+rim, edge
			for y := margin + int(pane.StripDp); y < windowSize.Y-margin-radius; y++ {
				first := at(img, x0, y)
				if first == tc.c.SidebarMaterial {
					continue
				}
				flat := true
				for x := x0; x < x1; x++ {
					if at(img, x, y) != first {
						flat = false
						break
					}
				}
				if flat {
					t.Fatalf("row %d runs %v across the whole rail; nothing in a sidebar is parted by a line", y, first)
				}
			}
		})
	}
}

// TestAHeadingCollapsesTheRowsBeneathIt reads the section's own behaviour off
// two frames: with the group open its feeds stand under the heading, and with
// it collapsed the pane's own fill stands there instead. The heading itself
// does not move — it is a heading and not a row, so collapsing a section
// takes its rows away and leaves its name where it was.
func TestAHeadingCollapsesTheRowsBeneathIt(t *testing.T) {
	// Where the first group's rows stand: under the pane's strip and the
	// first heading's block.
	rowsTop := int(pane.MarginDp) + int(pane.StripDp) + int(patsidebar.SectionHeight)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			open := settledModel()
			shut, _ := Update(open, ToggleSection{Idx: 0})
			if shut.openSections[0] {
				t.Fatal("ToggleSection left the first section open; the frames below would be the same")
			}
			a := golden.Capture(t, windowSize, frame(t, tc.c, tokens.Comfortable, open))
			b := golden.Capture(t, windowSize, frame(t, tc.c, tokens.Comfortable, shut))
			if a == nil || b == nil {
				t.Skip("no capture backend")
			}
			if got := at(a, atOpenFeed.X, atOpenFeed.Y); got == tc.c.SidebarMaterial {
				t.Fatalf("with the group open its first row reads the pane's own fill at %v; nothing was drawn there", atOpenFeed)
			}
			if got := at(b, atOpenFeed.X, atOpenFeed.Y); got != tc.c.SidebarMaterial {
				t.Errorf("with the group collapsed %v still reads %v; the rows beneath the heading did not go", atOpenFeed, got)
			}
			// The heading's own name is untouched: the two frames agree
			// across the block's leading side, where the label stands. Its
			// trailing side is the control's, which turns a quarter — that is
			// the whole of what collapsing a section changes about a heading.
			nameEnd := int(feedsPaneColumnDp) - int(patsidebar.DisclosureInset) - int(patsidebar.DisclosureBox)
			for y := int(pane.MarginDp) + int(pane.StripDp); y < rowsTop; y++ {
				for x := int(pane.MarginDp) + int(pane.RadiusDp); x < nameEnd; x++ {
					if at(a, x, y) != at(b, x, y) {
						t.Fatalf("the heading's block differs at (%d,%d) between the open and collapsed frames; a heading is not a row and does not move with its rows", x, y)
					}
				}
			}
		})
	}
}
