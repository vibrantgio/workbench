package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestEveryTabNamesALiveGroup guards the wiring against a group renamed
// upstream: TabItems looks its group up by name and returns nothing when
// no group answers, which on screen is a blank tab and in a golden a flat
// rectangle nobody reads twice.
func TestEveryTabNamesALiveGroup(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight
	for tab, group := range tabGroups {
		rows := inv.TabItems(c, group)
		if len(rows) == 0 {
			t.Fatalf("the %s tab asks the inventory for a group named %q and gets nothing", tabTitles[tab], group)
		}
	}
}

// TestGroupTabDropsTheBanner asserts the whole reason these tabs build
// their own rows: a tab showing one group carries no banner repeating the
// name on the strip cell above it. GroupItems leads with that banner, so
// the tab's rows are its rows less one, plus the closing line.
func TestGroupTabDropsTheBanner(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight
	for _, grp := range inv.Groups(c) {
		if grp.Name == groupFoundations {
			continue
		}
		want := 2 * len(grp.Sections) // heading + body per section
		got := len(inv.TabItems(c, grp.Name)) - 1
		if got != want {
			t.Errorf("the %s tab lays out %d rows before its closing line, want %d — the group banner is still in",
				grp.Name, got, want)
		}
	}
}

// TestNoInventorySectionIsLost checks the arithmetic: every section the
// published inventory builds is on exactly one tab, save the colour section
// the Colours tab tells with the shared board instead. A section added
// upstream lands on a tab or fails here; it does not quietly vanish because
// the window picks its groups by name.
func TestNoInventorySectionIsLost(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight

	// dropped names the sections the window does not show: the inventory's own
	// board of the platform's set, which the Colours tab draws from the
	// shared section instead, so a reader meets those names once.
	dropped := map[string]bool{"foundations-platform": true}

	// shown counts the sections each surface accounts for: the three group
	// tabs by their groups, and the Typography tab by the one section it is.
	shown := map[string]int{typeSection: 1}
	for tab, group := range tabGroups {
		found := false
		for _, grp := range inv.Groups(c) {
			if grp.Name != group {
				continue
			}
			found = true
			for _, s := range grp.Sections {
				shown[s.Name]++
			}
		}
		if !found {
			t.Fatalf("no group named %q for the %s tab", group, tabTitles[tab])
		}
	}

	for _, grp := range inv.Groups(c) {
		for _, s := range grp.Sections {
			switch {
			case dropped[s.Name]:
				if shown[s.Name] != 0 {
					t.Errorf("section %q is dropped but shown %d times", s.Name, shown[s.Name])
				}
			case shown[s.Name] != 1:
				t.Errorf("section %q is on %d tabs, want exactly 1", s.Name, shown[s.Name])
			}
		}
	}
}

// TestTheTabsRowsStopShortOfAStandingBarsLane lays a column of rows out in a
// frame shorter than they are, once per setting, with each row painting the
// whole width it is given. At Always the rows end before the bar's lane
// and the thumb is drawn in it; where the bar fades the rows run the
// frame's full width under where the bar would be.
func TestTheTabsRowsStopShortOfAStandingBarsLane(t *testing.T) {
	c := tokens.PlatformLight
	bg := inventory.SectionSurface(c)
	// An opaque fill no part of the bar draws, so a row pixel reads as itself.
	rowFill := color.NRGBA{R: 0x20, G: 0x80, B: 0x40, A: 0xff}
	size := image.Pt(400, 300)
	render := func(bars desktop.ScrollBarSetting) *image.RGBA {
		items := make([]layout.Widget, 40)
		for i := range items {
			items[i] = func(gtx layout.Context) layout.Dimensions {
				// A row paints its top half, so the row's own width is in
				// the pixels and the bar's thumb is not covered by it.
				w := gtx.Constraints.Max.X
				paint.FillShape(gtx.Ops, rowFill, clip.Rect{Max: image.Pt(w, 10)}.Op())
				return layout.Dimensions{Size: image.Pt(w, 20)}
			}
		}
		column := scrollingColumn(list.NewState(), c, bars, items)
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return column(gtx)
		})
	}
	rowsEnd := func(img *image.RGBA) int {
		end := 0
		for y := range size.Y {
			for x := range size.X {
				p := img.RGBAAt(x, y)
				if p.R == rowFill.R && p.G == rowFill.G && p.B == rowFill.B {
					end = max(end, x+1)
				}
			}
		}
		return end
	}
	laneW := int(columnBar(c, desktop.ScrollBarsAlways).Width())
	laneMin := size.X - laneW
	if got := rowsEnd(render(desktop.ScrollBarsAlways)); got != laneMin {
		t.Errorf("at Always the rows end at x %d, want them to stop short of the %d px lane at x %d", got, laneW, laneMin)
	}
	if got := rowsEnd(render(desktop.ScrollBarsWhenScrolling)); got != size.X {
		t.Errorf("at WhenScrolling the rows end at x %d, want the frame's full width %d", got, size.X)
	}
}
