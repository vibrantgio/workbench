package main

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// thumbIn counts the pixels inside r drawn exactly in the bar's thumb
// colour.
func thumbIn(img *image.RGBA, r image.Rectangle, bar scrollbar.Style) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R == bar.ThumbColor.R && c.G == bar.ThumbColor.G && c.B == bar.ThumbColor.B {
				n++
			}
		}
	}
	return n
}

// TestThePickersRowsDrawTheWindowsBar lays the folder browser's rows out in
// a frame shorter than they are, once per setting, on a frame nothing has
// scrolled, with the first row selected so that its fill shows where the row
// ends.
//
// At Always the bar is drawn at rest in a lane at the list's trailing edge,
// the thumb inside the lane's padding, and the selected row's fill stops
// short of the lane. Where the bar fades a resting frame draws no thumb and
// the row's fill runs the list's full width.
func TestThePickersRowsDrawTheWindowsBar(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	size := image.Pt(360, 160)
	var entries []DirEntry
	for i := range 20 {
		entries = append(entries, DirEntry{Idx: i, Name: fmt.Sprintf("Folder %d", i), Path: fmt.Sprintf("/notes/Folder %d", i)})
	}
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			fill := chromeSurface(tc.colors)
			render := func(bars desktop.ScrollBarSetting) *image.RGBA {
				tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
					sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper, scrollBars: bars}
				v := &pickerView{list: list.NewState()}
				v.list.Select(0)
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					return v.rows(gtx, tok, entries, fill, pickerRowInsetDp)
				})
			}
			bar := scrollBarStyle(scrollbar.FromTokens(tc.colors, fill), desktop.ScrollBarsAlways)
			lane, track := int(bar.Width()), int(bar.TrackPadding)
			laneMin := size.X - lane
			thumb := image.Rect(laneMin+track, 0, size.X-track, size.Y)
			selected := tc.colors.SelectedContentBackground

			standing := render(desktop.ScrollBarsAlways)
			if thumbIn(standing, thumb, bar) == 0 {
				t.Errorf("at Always a resting frame drew no thumb in the trailing lane x %d–%d", thumb.Min.X, thumb.Max.X)
			}
			if row := colorBox(standing, selected); row.Max.X != laneMin {
				t.Errorf("at Always the selected row's fill ends at x %d, want it to stop short of the %d px lane at x %d", row.Max.X, lane, laneMin)
			}

			fading := render(desktop.ScrollBarsWhenScrolling)
			if n := thumbIn(fading, thumb, bar); n != 0 {
				t.Errorf("at WhenScrolling a resting frame drew %d thumb pixels, want none", n)
			}
			if row := colorBox(fading, selected); row.Max.X != size.X {
				t.Errorf("at WhenScrolling the selected row's fill ends at x %d, want the list's full width %d", row.Max.X, size.X)
			}
		})
	}
}

// TestTheChoosersListDrawsTheWindowsBar raises the chooser over more
// candidates than the dialog has room for, once per setting.
//
// The list is no taller than the room the dialog gives it, so it scrolls.
// At Always the bar is drawn at rest in a lane at the list's trailing edge
// and the selected row's fill stops short of the lane. Where the bar fades
// a resting frame draws no thumb and the row's fill runs to the list's own
// trailing edge, under the ring's inner half.
func TestTheChoosersListDrawsTheWindowsBar(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	var candidates []string
	for i := range 40 {
		candidates = append(candidates, fmt.Sprintf("Design/%02d/Colour.md", i))
	}
	m, _ := Update(goldenModel(), OpenChooser{Body: "Colour", Candidates: candidates})
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			selected := tc.colors.SelectedContentBackground
			bar := scrollBarStyle(scrollbar.FromTokens(tc.colors, tc.colors.WindowBackground), desktop.ScrollBarsAlways)
			lane, track := int(bar.Width()), int(bar.TrackPadding)

			fading := settledDialog(t, liveDialogBars(t, chooserLayer, m, tc.colors, shaper, desktop.ScrollBarsWhenScrolling), tc.colors)
			standing := settledDialog(t, liveDialogBars(t, chooserLayer, m, tc.colors, shaper, desktop.ScrollBarsAlways), tc.colors)
			ring := colorBox(fading, ringOnPlane(tc.colors))
			full := colorBox(fading, selected)
			if ring.Empty() || full.Empty() {
				t.Fatal("the chooser opened with no ring or no selected row")
			}
			if ring.Max.Y > windowH {
				t.Fatalf("the list's ring ends at y %d, past the window's %d: the list is taller than the room the dialog gives it", ring.Max.Y, windowH)
			}
			// The ring's band lies half over the list's box, so a row's
			// fill shows up to ringOutside short of the list's edge.
			listRight := full.Max.X + ringOutside
			laneMin := listRight - lane
			thumb := image.Rect(laneMin+track, full.Min.Y, listRight-track, ring.Max.Y-2*ringOutside)

			if thumbIn(standing, thumb, bar) == 0 {
				t.Errorf("at Always a resting frame drew no thumb in the lane x %d–%d at the list's trailing edge", thumb.Min.X, thumb.Max.X)
			}
			if row := colorBox(standing, selected); row.Max.X != laneMin {
				t.Errorf("at Always the selected row's fill ends at x %d, want it to stop short of the %d px lane at x %d (the list's edge x %d)", row.Max.X, lane, laneMin, listRight)
			}
			if n := thumbIn(fading, thumb.Intersect(image.Rect(0, full.Max.Y, windowW, windowH)), bar); n != 0 {
				t.Errorf("at WhenScrolling a resting frame drew %d thumb pixels below the selected row, want none", n)
			}
		})
	}
}
