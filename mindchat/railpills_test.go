package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// railThemed is the token snapshot a rail needs: the palette's own colours
// plus every mark a row and the footer draw, rastered the way the live
// pipeline rasters them.
func railThemed(t *testing.T, c tokens.PlatformColors) themed {
	t.Helper()
	th := schemeThemed(t, c)
	p := th.palette
	th.bar = scrollbar.FromTokens(c, c.ControlBackground)
	mark := func(m []byte, size unit.Dp, col color.NRGBA) layout.Widget {
		w, err := raster.Widget(m, size, size, raster.WithColors(col))
		if err != nil {
			t.Fatalf("raster: %v", err)
		}
		return w
	}
	th.remove = mark(icons.ContentClear, DeleteIconSize, p.Row)
	th.edit = mark(icons.EditorModeEdit, DeleteIconSize, p.Row)
	th.removeOn = mark(icons.ContentClear, DeleteIconSize, p.RowActive)
	th.editOn = mark(icons.EditorModeEdit, DeleteIconSize, p.RowActive)
	th.add = mark(icons.ContentAdd, AddIconSize, p.Heading)
	th.gear = mark(icons.ActionSettings, SettingsIconSize, p.Heading)
	return th
}

// railPillSize is the rail the reading below is taken from: the pane's own
// width at the window's opening size, tall enough for its strip, a few rows
// and the footer.
var railPillSize = image.Pt(240, 320)

// TestTheRailDrawsThePlatformsTwoPills reads both pill states off a rendered
// rail in both appearances: the accent pill under a white label while the
// rail holds the keyboard, and the grey pill under the label in the accent
// colour while it does not.
//
// The rail's rows are its focusables, so what hands it the keyboard is the
// keyboard landing on one of them — which is what the frame below does
// through a real router, so gtx.Focused answers as the window would.
func TestTheRailDrawsThePlatformsTwoPills(t *testing.T) {
	const open = "Reading list.md"
	for _, sc := range schemes {
		t.Run(sc.name, func(t *testing.T) {
			th := railThemed(t, sc.c)
			chats := ChatList{open, "Sources.md"}
			rowClicks := map[string]*widget.Clickable{}
			delClicks := map[string]*widget.Clickable{}
			renClicks := map[string]*widget.Clickable{}
			rows := list.NewState()
			var newChat, toggle, settings widget.Clickable
			pane := SidebarPane(th, chats, open, nil, rows, rowClicks, delClicks, renClicks, &newChat, &toggle, &settings)

			r := new(input.Router)
			ops := new(op.Ops)
			var take bool
			w := func(gtx layout.Context) layout.Dimensions {
				dims := pane(gtx)
				if take {
					if c, ok := rowClicks[open]; ok {
						gtx.Execute(key.FocusCmd{Tag: c})
						take = false
					}
				}
				return dims
			}
			drive := func() {
				ops.Reset()
				w(layout.Context{
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(railPillSize),
					Ops:         ops,
					Source:      r.Source(),
				})
				r.Frame(ops)
			}
			capture := func() *image.RGBA {
				drive()
				return golden.Capture(t, railPillSize, func(gtx layout.Context) layout.Dimensions {
					gtx.Source = r.Source()
					return w(gtx)
				})
			}
			drive()
			bare := capture()
			if bare == nil {
				return // headless unavailable; Capture called t.Skip
			}
			take = true
			drive()
			held := capture()

			y := railPillRowMidY(bare, sc.c)
			if y < 0 {
				t.Fatal("the rail drew no filled row to read")
			}
			x := railPillSize.X - int(sidebar.SelectionInset) - 4
			for _, c := range []struct {
				what string
				img  *image.RGBA
				want color.NRGBA
			}{
				{"while the rail does not hold the keyboard", bare, sidebar.SelectionFill(sc.c, true)},
				{"while it does", held, sidebar.SelectionFill(sc.c, false)},
			} {
				got := c.img.RGBAAt(x, y)
				if got.R != c.want.R || got.G != c.want.G || got.B != c.want.B {
					t.Errorf("the pill %s reads %v at (%d, %d), want the measured %v", c.what, got, x, y, c.want)
				}
			}
		})
	}
}

// railPillRowMidY finds the middle row of the grey pill in a rendered rail.
func railPillRowMidY(img *image.RGBA, c tokens.PlatformColors) int {
	want := sidebar.SelectionFill(c, true)
	lo, hi := -1, -1
	x := railPillSize.X / 2
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		p := img.RGBAAt(x, y)
		if p.R == want.R && p.G == want.G && p.B == want.B {
			if lo < 0 {
				lo = y
			}
			hi = y
		}
	}
	if lo < 0 {
		return -1
	}
	return (lo + hi) / 2
}
