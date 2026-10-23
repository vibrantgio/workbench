package main

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
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

// TestAFolderThatFailsToReadDrawsItsOwnEmptyState lists two real folders
// and draws the region for each. The folder whose read fails draws the
// failed-read line. The folder that holds no folders draws the
// holds-no-folders line. Each region is compared pixel for pixel with the
// empty state drawn with the line the test expects.
func TestAFolderThatFailsToReadDrawsItsOwnEmptyState(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "Empty")
	unreadable := filepath.Join(root, "Unreadable")
	for _, d := range []string{empty, filepath.Join(unreadable, "Inside")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })

	shaper := tokens.DefaultTypography.DeterministicShaper()
	size := image.Pt(360, 240)
	fill := tokens.PlatformLight.WindowBackground
	tok := themeTokens{col: tokens.PlatformLight, typ: tokens.DefaultTypography,
		sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}
	draw := func(region func(gtx layout.Context) layout.Dimensions) *image.RGBA {
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(size)
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
			return region(gtx)
		})
	}
	for _, tc := range []struct {
		name, dir, line string
		root            bool
	}{
		{name: "unreadable", dir: unreadable, line: pickerReadFailedLine, root: true},
		{name: "empty", dir: empty, line: pickerNoFoldersLine},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.root && os.Geteuid() == 0 {
				t.Skip("root reads a folder whatever its permissions")
			}
			entries, err := ListDir(tc.dir)
			if tc.root != (err != nil) {
				t.Fatalf("ListDir(%s) returned the error %v; an error is expected: %v", tc.dir, err, tc.root)
			}
			if len(entries) != 0 {
				t.Fatalf("ListDir(%s) returned %d rows, want none", tc.dir, len(entries))
			}
			m := Model{PickerOpen: true, PickerDir: tc.dir}
			m, _ = Update(m, pickerListed{dir: tc.dir, entries: entries, err: err})
			if !m.PickerListed {
				t.Fatal("the listing is not read after its message, want it read")
			}
			v := &pickerView{list: list.NewState()}
			got := draw(func(gtx layout.Context) layout.Dimensions {
				return v.listRegion(gtx, m, tok, fill, 0)
			})
			want := draw(func(gtx layout.Context) layout.Dimensions {
				v := &pickerView{list: list.NewState()}
				dims := v.rows(gtx, tok, nil, fill, 0)
				bar := scrollBarStyle(scrollbar.FromTokens(tok.col, fill), tok.scrollBars)
				egtx := gtx
				egtx.Constraints = layout.Exact(shell.EmptyStateFrame(gtx, size, bar))
				shell.EmptyState(egtx, tok.shaper, tok.col, tok.typ, fill,
					shell.EmptyStateProps{Line: tc.line, LineStyle: tok.typ.BodyLarge})
				return dims
			})
			if box := noteGlyphBox(got, fill); box.Empty() {
				t.Fatal("the region is bare, want the line the empty state draws")
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Errorf("the region does not draw the line %q", tc.line)
			}
		})
	}
}
