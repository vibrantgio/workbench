package main

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
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
	return themed{platform: p, typ: typ, shaper: typ.DeterministicShaper(),
		spacing: tokens.Spacing, radius: radius}
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
		view(th, m, state, func() unit.Dp { return shell.ToolbarDepth }),
	))
}

var schemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// TestBoardGolden records or diffs the board in both schemes.
func TestBoardGolden(t *testing.T) {
	m := newModel("Kanban.md", fixtureBoard)
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			golden.Render(t, tc.name+"-board", windowSize, frame(staticThemed(tc.c, sharpRadius), m, newScrollState()))
			if *boardDump == "" {
				return
			}
			img := golden.Capture(t, windowSize, frame(staticThemed(tc.c, tokens.Radius), m, newScrollState()))
			path := filepath.Join(*boardDump, "kanban-"+tc.name+".png")
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

// TestTheColumnsPaintNoFill reads the frame under the last card of a column
// and between two columns: both are the content fill, because a column is
// not a box.
func TestTheColumnsPaintNoFill(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := golden.Capture(t, windowSize, frame(staticThemed(tc.c, sharpRadius), newModel("Kanban.md", fixtureBoard), newScrollState()))
			fill := tc.c.ControlBackground
			inset, width, gap := int(boardInset), int(columnWidth), int(columnGap)
			below := image.Pt(inset+width/2, windowSize.Y-2*inset)
			between := image.Pt(inset+width+gap/2, windowSize.Y/2)
			for _, p := range []image.Point{below, between} {
				if got := img.RGBAAt(p.X, p.Y); got.R != fill.R || got.G != fill.G || got.B != fill.B {
					t.Errorf("pixel %v = %v, want the content fill %v", p, got, fill)
				}
			}
			card := image.Pt(inset+width/2, int(shell.ToolbarDepth)+int(tokens.ContentListRowHeight)+4)
			want := tc.c.CardFill
			if got := img.RGBAAt(card.X, card.Y); got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("pixel %v = %v, want the card fill %v", card, got, want)
			}
		})
	}
}

// TestTheBoardScrollsSideways lays the board out in the window, where its
// three columns are wider than the window.
func TestTheBoardScrollsSideways(t *testing.T) {
	state := newScrollState()
	golden.Capture(t, windowSize, frame(staticThemed(tokens.PlatformLight, sharpRadius), newModel("Kanban.md", fixtureBoard), state))
	if !state.board.Overflows() {
		t.Fatal("three columns fit a window of 650 dp; the board does not scroll")
	}
	want := 2*int(boardInset) + 3*int(columnWidth) + 2*int(columnGap)
	if got := state.board.Content(); got != want {
		t.Errorf("the board is %d px wide, want %d", got, want)
	}
}
