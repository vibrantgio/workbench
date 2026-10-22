package main

import (
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

// TestAFolderWithNoFoldersDrawsTheBrowsersEmptyState lays the region under
// the folder browser's trail out for a folder whose listing is read and
// holds no folder, once per setting, in both schemes.
//
// The line is drawn centred both ways in the frame the rows would occupy:
// where the bar fades that is the region's full frame, and where it stands
// it is the frame less the bar's lane on the trailing side. The glyph box is
// the painted glyphs' own and the line box is what is centred, so the two
// centres differ by a pixel or two.
func TestAFolderWithNoFoldersDrawsTheBrowsersEmptyState(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	size := image.Pt(360, 240)
	m := Model{PickerOpen: true, PickerDir: "/notes/Empty", PickerListed: true}
	for _, tc := range themeCases {
		for _, bars := range []desktop.ScrollBarSetting{desktop.ScrollBarsWhenScrolling, desktop.ScrollBarsAlways} {
			t.Run(tc.name+"/"+bars.String(), func(t *testing.T) {
				fill := tc.colors.WindowBackground
				tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
					sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper, scrollBars: bars}
				v := &pickerView{list: list.NewState()}
				img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Exact(size)
					paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
					return v.listRegion(gtx, m, tok, fill, 0)
				})
				box := noteGlyphBox(img, fill)
				if box.Empty() {
					t.Fatal("the region is bare, want the line the empty state draws")
				}
				frameW := size.X
				if bars == desktop.ScrollBarsAlways {
					frameW -= int(scrollBarStyle(scrollbar.FromTokens(tc.colors, fill), bars).Width())
				}
				if got := box.Min.X + box.Max.X; abs(got-frameW) > 2 {
					t.Errorf("the line's glyphs centre at %.1f across, want the frame's %.1f",
						float64(got)/2, float64(frameW)/2)
				}
				if got := box.Min.Y + box.Max.Y; abs(got-size.Y) > 4 {
					t.Errorf("the line's glyphs centre at %.1f down, want the frame's %.1f",
						float64(got)/2, float64(size.Y)/2)
				}
			})
		}
	}
}

// TestAListingStillBeingReadDrawsNoEmptyState holds the other half: until
// the listing for the folder is read, the region has no rows and says
// nothing, because it does not yet know that the folder holds no folders.
func TestAListingStillBeingReadDrawsNoEmptyState(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	size := image.Pt(360, 240)
	browsed, _ := Update(Model{PickerOpen: true, PickerDir: "/notes", PickerListed: true}, BrowseTo{Dir: "/notes/Empty"})
	if browsed.PickerListed {
		t.Fatal("a move to another folder kept the listing read, want it unread until the listing for the new folder is read")
	}
	fill := tokens.PlatformLight.WindowBackground
	tok := themeTokens{col: tokens.PlatformLight, typ: tokens.DefaultTypography,
		sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}
	v := &pickerView{list: list.NewState()}
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(size)
		paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
		return v.listRegion(gtx, browsed, tok, fill, 0)
	})
	if box := noteGlyphBox(img, fill); !box.Empty() {
		t.Errorf("the region draws at %v while the listing is being read, want nothing", box)
	}
	listed, _ := Update(browsed, pickerListed{dir: "/notes/Empty"})
	if !listed.PickerListed || len(listed.PickerEntries) != 0 {
		t.Errorf("the listing for the folder is read as %v with %d rows, want read with none",
			listed.PickerListed, len(listed.PickerEntries))
	}
}
