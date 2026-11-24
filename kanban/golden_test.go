package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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

var boardDump = flag.String("board.dump", "", "directory to write renders with the theme's own corner radius into")

// windowSize is the size the window opens at; golden.Capture draws one
// pixel per dp.
var windowSize = image.Pt(int(winW), int(winH))

// sharpRadius keeps the stored renders the same between machines: the
// anti-aliased edge of a rounded corner differs between GPU contexts.
var sharpRadius = tokens.RadiusScale{}

// staticThemed is one theme emission frozen, with the pinned shaper that a
// stored render shapes with and the overlay scroll bars.
func staticThemed(p tokens.PlatformColors, radius tokens.RadiusScale) themed {
	typ := tokens.DefaultTypography
	components := theme.Theme{
		Platform:   rx.Of(p),
		Typography: rx.Of(typ),
		Density:    rx.Of(tokens.Comfortable),
		Motion:     rx.Of(tokens.Motion),
		Spacing:    rx.Of(tokens.Spacing),
		Radius:     rx.Of(radius),
	}
	return themed{components: components, platform: p, typ: typ, shaper: typ.DeterministicShaper(),
		spacing: tokens.Spacing, radius: radius, density: tokens.Comfortable}
}

// fixtureBoard is the new board's three columns with a few tasks in them.
const fixtureBoard = "---\n\nkanban-plugin: board\n\n---\n\n" +
	"## To Do\n\n" +
	"- [ ] Draft the release notes\n" +
	"- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n" +
	"- [ ] Water the plants\n\n\n" +
	"## In Progress\n\n" +
	"- [ ] Paint the hallway\n\n\n" +
	"## Done\n\n" +
	"- [x] Buy oat milk\n" +
	"- [x] Renew the passport\n"

// frame is the window as buildLayers stacks it: the content fill, then the
// page under the toolbar's band.
func frame(th themed, m Model, state *scrollState) layout.Widget {
	return golden.On(th.platform.ControlBackground, golden.Stack(
		backdrop.Widget(th.platform.ControlBackground),
		view(th, m, addField{}, state, func() unit.Dp { return shell.ToolbarDepth }, postMessage),
	))
}

var schemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// fixtureEmptyColumn is the fixture board with its middle column empty.
var fixtureEmptyColumn = strings.Replace(fixtureBoard, "- [ ] Paint the hallway\n", "", 1)

// TestBoardGolden records or diffs the board in both schemes, with cards in
// every column and with an empty column.
func TestBoardGolden(t *testing.T) {
	boards := []struct{ name, file string }{
		{"board", fixtureBoard},
		{"empty-column", fixtureEmptyColumn},
	}
	for _, b := range boards {
		m := newModel("Kanban.md", b.file)
		for _, tc := range schemes {
			t.Run(tc.name+"-"+b.name, func(t *testing.T) {
				golden.Render(t, tc.name+"-"+b.name, windowSize, frame(staticThemed(tc.c, sharpRadius), m, newScrollState()))
				if *boardDump == "" {
					return
				}
				img := golden.Capture(t, windowSize, frame(staticThemed(tc.c, tokens.Radius), m, newScrollState()))
				path := filepath.Join(*boardDump, "kanban-"+tc.name+"-"+b.name+".png")
				out, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				defer out.Close()
				if err := png.Encode(out, img); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// sameRGB reports whether the pixel at p is want, alpha aside.
func sameRGB(t *testing.T, img *image.RGBA, p image.Point, want color.NRGBA, what string) {
	t.Helper()
	if got := img.RGBAAt(p.X, p.Y); got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("pixel %v = %v, want %s %v", p, got, what, want)
	}
}

// TestTheColumnsPaintNoFill reads the frame under the last card of a column
// and between two columns: both are the content fill, because a column is
// a group and not a box.
func TestTheColumnsPaintNoFill(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := golden.Capture(t, windowSize, frame(staticThemed(tc.c, sharpRadius), newModel("Kanban.md", fixtureBoard), newScrollState()))
			fill := tc.c.ControlBackground
			inset, width, gap := int(boardInset), int(columnWidth), int(columnGap)
			sameRGB(t, img, image.Pt(inset+width/2, windowSize.Y-2*inset), fill, "the content fill")
			sameRGB(t, img, image.Pt(inset+width+gap/2, windowSize.Y/2), fill, "the content fill")
			// The first card starts under the room the focus ring keeps, the
			// group's inset, its title and the gap under the title.
			cardTop := int(shell.ToolbarDepth) + int(complist.RingWidth) + int(tokens.Spacing.S4) +
				int(tokens.DefaultTypography.SectionHeading.LineHeight) + int(tokens.Spacing.S3)
			sameRGB(t, img, image.Pt(inset+width/2, cardTop+4), tc.c.CardFill, "the card fill")
		})
	}
}

// TestAnEmptyColumnShowsItsExtent reads the hairline of an empty column's
// group at its leading edge and at its bottom edge: the group runs the
// column's full height also when the column holds no card.
func TestAnEmptyColumnShowsItsExtent(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := golden.Capture(t, windowSize, frame(staticThemed(tc.c, sharpRadius), newModel("Kanban.md", fixtureEmptyColumn), newScrollState()))
			seam := vgcolor.Flatten(tc.c.Separator, tc.c.ControlBackground)
			inset, width, gap := int(boardInset), int(columnWidth), int(columnGap)
			leading := inset + width + gap
			bottom := windowSize.Y - inset - 1
			sameRGB(t, img, image.Pt(leading, windowSize.Y/2), seam, "the separator")
			sameRGB(t, img, image.Pt(leading, bottom-8), seam, "the separator")
			sameRGB(t, img, image.Pt(leading+width/2, bottom), seam, "the separator")
			sameRGB(t, img, image.Pt(leading+width/2, windowSize.Y/2), tc.c.ControlBackground, "the content fill")
		})
	}
}

// TestTheWindowShowsThreeColumns lays the board out in the window: its
// three columns fit, and a fourth column makes the board scroll sideways.
func TestTheWindowShowsThreeColumns(t *testing.T) {
	want := 2*int(boardInset) + 3*int(columnWidth) + 2*int(columnGap)
	if windowSize.X != want {
		t.Errorf("the window is %d px wide, want %d", windowSize.X, want)
	}
	state := newScrollState()
	golden.Capture(t, windowSize, frame(staticThemed(tokens.PlatformLight, sharpRadius), newModel("Kanban.md", fixtureBoard), state))
	if state.board.Overflows() {
		t.Error("three columns do not fit the window; the board scrolls")
	}
	state = newScrollState()
	golden.Capture(t, windowSize, frame(staticThemed(tokens.PlatformLight, sharpRadius), newModel("Kanban.md", fixtureBoard+"\n## Archive\n\n"), state))
	if !state.board.Overflows() {
		t.Error("four columns fit the window; the board does not scroll")
	}
}

// TestColumnBarStandsAtTheGroupEdge renders a column with more cards than
// fit, with the bar standing, and reads the pixel column down the middle of
// the bar's lane at the group's trailing hairline: the bar draws there
// beside the cards, down to the "Add a card" button's row, and draws
// nothing beside the title or beside the button's row.
func TestColumnBarStandsAtTheGroupEdge(t *testing.T) {
	var b strings.Builder
	b.WriteString("## To Do\n\n")
	for range 30 {
		b.WriteString("- [ ] Water the plants\n")
	}
	th := staticThemed(tokens.PlatformLight, sharpRadius)
	th.scrollBars = desktop.ScrollBarsAlways
	img := golden.Capture(t, windowSize, frame(th, newModel("Kanban.md", b.String()), newScrollState()))

	bg := color.RGBA(th.platform.ControlBackground)
	band := int(shell.ToolbarDepth)
	inset := int(boardInset)
	// The middle of the bar's lane, which ends at the hairline.
	x := inset + int(columnWidth) - 1 - int(th.bar().Width())/2
	top, bottom := band+int(complist.RingWidth), windowSize.Y-inset-1
	titleFoot := top + int(tokens.Spacing.S4) + int(th.typ.SectionHeading.LineHeight)
	for y := top + 1; y < titleFoot; y++ {
		if img.RGBAAt(x, y) != bg {
			t.Fatalf("pixel at x %d, y %d beside the title is %v; expected the fill %v", x, y, img.RGBAAt(x, y), bg)
		}
	}
	drawn := 0
	for y := titleFoot; y < bottom; y++ {
		if img.RGBAAt(x, y) != bg {
			drawn++
		}
	}
	if drawn < (bottom-titleFoot)/2 {
		t.Errorf("the bar covers %d of the %d rows beside the cards; expected it to run beside them", drawn, bottom-titleFoot)
	}
	// The button's row is the density's control height, with the group's
	// padding under it. The bar's track ends a few pixels short of its
	// lane's end, which is the row's top.
	rowTop := bottom - int(tokens.Spacing.S4) - int(tokens.ComfortableControlHeight)
	end := -1
	for y := titleFoot; y < bottom; y++ {
		if img.RGBAAt(x, y) != bg {
			end = y
		}
	}
	if end < rowTop-6 || end >= rowTop {
		t.Errorf("the bar ends at y %d; expected it to run to the button's row at y %d", end, rowTop)
	}
}
