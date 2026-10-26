package main

// What the Docs tab says when the guide cannot be loaded, and where the
// system's own text goes instead.

import (
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/vibrantgio/components/golden"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// docsColumnSize is the frame the Docs tab's one column stands in at the
// size the window opens at, less the tab strip above it.
var docsColumnSize = image.Pt(1180, 700)

// TestAGuideThatCannotBeLoadedSaysSoInItsOwnWords drives a real load at a
// path no guide stands at, with the network leg skipped, and holds both
// halves of the state: the column draws this app's own sentence and the
// location it looked at, and the system's own text — which the column
// does not carry — reaches the log with that same location.
func TestAGuideThatCannotBeLoadedSaysSoInItsOwnWords(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "llms.txt")

	logged := captureStderr(t)
	src, fail := loadGuideFrom([]string{missing}, nil)
	written := logged()

	if fail == nil {
		t.Fatal("a path with no guide at it reported no failure")
	}
	if src != nil {
		t.Errorf("loadGuideFrom returned %d bytes of source beside the failure", len(src))
	}
	if fail.Err == "" {
		t.Fatal("the failure carries no cause; there is nothing to log")
	}
	if fail.Path != missing {
		t.Errorf("the failure names %q, want the path it looked at, %q", fail.Path, missing)
	}
	if strings.Contains(guideUnavailableLine, fail.Err) {
		t.Errorf("the column says %q, which stands the system's text %q in the empty state",
			guideUnavailableLine, fail.Err)
	}
	if strings.Contains(fail.Path, fail.Err) {
		t.Errorf("the description %q carries the system's text %q", fail.Path, fail.Err)
	}
	if !strings.Contains(written, fail.Err) {
		t.Errorf("the log says %q, want the system's text %q in it", written, fail.Err)
	}
	if !strings.Contains(written, missing) {
		t.Errorf("the log says %q, want the guide's path %q in it", written, missing)
	}
}

// TestTheDocsColumnDrawsTheLineAndThePath holds what the column puts in
// place of the guide: the pattern's group — the one sentence with the
// location under it — centred in the column's own frame, and nothing
// else. The drawing is held against a probe built from the two strings
// alone, so a column that also painted the system's text, or that left
// the location out, draws different pixels.
func TestTheDocsColumnDrawsTheLineAndThePath(t *testing.T) {
	fail := &guideFailure{Path: "/vaults/workbench/llms.txt", Err: "open ../llms.txt: no such file or directory"}
	typ := tokens.DefaultTypography
	shaper := typ.DeterministicShaper()

	for _, tc := range emptyStateSchemes {
		t.Run(tc.name, func(t *testing.T) {
			fill := shellFill(tc.colors)
			img := golden.Capture(t, docsColumnSize, scene(func(gtx layout.Context) layout.Dimensions {
				return drawGuideUnavailable(gtx, shaper, tc.colors, typ, columnBar(tc.colors, desktop.ScrollBarsWhenScrolling), fail)
			}, fill))

			got := emptyStateBox(img, img.Bounds(), fill)
			if got.Empty() {
				t.Fatal("the column is bare, want the line the empty state draws")
			}

			ref := emptyStateProbe(t, shaper, tc.colors, typ, fill, shell.EmptyStateProps{
				Line:        "Guide Could Not Be Loaded",
				Description: guideUnavailableDescription(fail.Path),
			})
			if want := emptyStateBox(ref, ref.Bounds(), fill); got != want {
				t.Errorf("the column draws %v, want %v — the line and the path, and nothing else", got, want)
			}

			// A probe of the line alone proves the path is really drawn:
			// the group is taller with it than without.
			line := emptyStateProbe(t, shaper, tc.colors, typ, fill,
				shell.EmptyStateProps{Line: "Guide Could Not Be Loaded"})
			lineBox := emptyStateBox(line, line.Bounds(), fill)
			if got.Dy() <= lineBox.Dy() {
				t.Errorf("the column's group is %d pixels tall, the line alone %d; the path stands under the line",
					got.Dy(), lineBox.Dy())
			}

			// The line spends the platform's tertiary label over the
			// column it stands in, and the path under it the secondary
			// label: two strengths, read off the two bands of the group.
			bands := []struct {
				what   string
				region image.Rectangle
				want   color.NRGBA
			}{
				{"line", image.Rect(got.Min.X, got.Min.Y, got.Max.X, got.Min.Y+lineBox.Dy()),
					vgcolor.Flatten(tc.colors.TertiaryLabel, fill)},
				{"path", image.Rect(got.Min.X, got.Min.Y+lineBox.Dy(), got.Max.X, got.Max.Y),
					vgcolor.Flatten(tc.colors.SecondaryLabel, fill)},
			}
			for _, b := range bands {
				pl := plateau(img, fill, b.region)
				if pl.R != b.want.R || pl.G != b.want.G || pl.B != b.want.B {
					t.Errorf("the column's %s plateaus at #%02x%02x%02x, want #%02x%02x%02x",
						b.what, pl.R, pl.G, pl.B, b.want.R, b.want.G, b.want.B)
				}
			}

			// And the group centres in the column's own frame.
			if c, want := got.Min.X+got.Max.X, docsColumnSize.X; absInt(c-want) > 4 {
				t.Errorf("the column centres its group at %.1f across, want the frame's %.1f",
					float64(c)/2, float64(want)/2)
			}
			if c, want := got.Min.Y+got.Max.Y, docsColumnSize.Y; absInt(c-want) > 4 {
				t.Errorf("the column centres its group at %.1f down, want the frame's %.1f",
					float64(c)/2, float64(want)/2)
			}
		})
	}
}

// TestTheDocsColumnsEmptyStateTakesNoLane holds the frame the column's
// group centres in against the reader's setting: a column with no document
// draws no bar, so the group stands where it stands when the bar fades, even
// where the column's bar would stand.
func TestTheDocsColumnsEmptyStateTakesNoLane(t *testing.T) {
	fail := &guideFailure{Path: "/vaults/workbench/llms.txt", Err: "open ../llms.txt: no such file or directory"}
	typ := tokens.DefaultTypography
	shaper := typ.DeterministicShaper()

	for _, tc := range emptyStateSchemes {
		t.Run(tc.name, func(t *testing.T) {
			fill := shellFill(tc.colors)
			box := func(bars desktop.ScrollBarSetting) image.Rectangle {
				img := golden.Capture(t, docsColumnSize, scene(func(gtx layout.Context) layout.Dimensions {
					return drawGuideUnavailable(gtx, shaper, tc.colors, typ, columnBar(tc.colors, bars), fail)
				}, fill))
				b := emptyStateBox(img, img.Bounds(), fill)
				if b.Empty() {
					t.Fatalf("at %v the column is bare, want the group the empty state draws", bars)
				}
				return b
			}
			if fading, standing := box(desktop.ScrollBarsWhenScrolling), box(desktop.ScrollBarsAlways); standing != fading {
				t.Errorf("at Always the group stands at %v, want %v as where the bar fades", standing, fading)
			}
		})
	}
}

// TestAFailedLoadKeepsTheOutlineColumnWithItsOwnLine holds the Docs page
// when the guide cannot be loaded: the outline column is kept at its
// width with its fill, and draws "No Headings" centred in its own frame
// at the size of its top-level rows in the tertiary label, while the
// content column beside it draws the guide's line and location centred in
// the rest. Each column's drawing is held against that column's empty
// state drawn alone in that column's frame.
func TestAFailedLoadKeepsTheOutlineColumnWithItsOwnLine(t *testing.T) {
	fail := &guideFailure{Path: "/vaults/workbench/llms.txt", Err: "open ../llms.txt: no such file or directory"}
	typ := tokens.DefaultTypography
	shaper := typ.DeterministicShaper()
	outlineW := docsOutlineWidthDp

	for _, tc := range emptyStateSchemes {
		t.Run(tc.name, func(t *testing.T) {
			fill := shellFill(tc.colors)
			rail := tc.colors.SidebarMaterial
			tok := themeTokens{col: tc.colors, typ: typ, shaper: shaper}
			bar := columnBar(tc.colors, desktop.ScrollBarsWhenScrolling)
			img := golden.Capture(t, docsColumnSize, scene(func(gtx layout.Context) layout.Dimensions {
				return drawDocsUnavailable(gtx, tok, bar, fail)
			}, fill))

			// The column is kept at its width: its fill reaches the last
			// pixel before the seam, and the seam is the column's last.
			for _, y := range []int{0, docsColumnSize.Y / 2, docsColumnSize.Y - 1} {
				if c := img.RGBAAt(outlineW-2, y); c.R != rail.R || c.G != rail.G || c.B != rail.B {
					t.Errorf("at x %d, y %d the page is #%02x%02x%02x, want the outline column's fill #%02x%02x%02x",
						outlineW-2, y, c.R, c.G, c.B, rail.R, rail.G, rail.B)
				}
			}
			seam := vgcolor.Flatten(tc.colors.Separator, rail)
			if c := img.RGBAAt(outlineW-1, docsColumnSize.Y/2); c.R != seam.R || c.G != seam.G || c.B != seam.B {
				t.Errorf("at x %d the page is #%02x%02x%02x, want the column's seam #%02x%02x%02x",
					outlineW-1, c.R, c.G, c.B, seam.R, seam.G, seam.B)
			}

			// The outline column's line, against the pattern drawn alone in
			// the column's frame at the rows' style.
			outlineFrame := image.Pt(outlineW, docsColumnSize.Y)
			region := image.Rect(0, 0, outlineW-1, docsColumnSize.Y)
			got := emptyStateBox(img, region, rail)
			if got.Empty() {
				t.Fatal("the outline column is bare, want its line")
			}
			alone := func(style tokens.TextStyle) image.Rectangle {
				ref := golden.Capture(t, outlineFrame, scene(func(gtx layout.Context) layout.Dimensions {
					return shell.EmptyState(gtx, shaper, tc.colors, typ, rail,
						shell.EmptyStateProps{Line: "No Headings", LineStyle: style})
				}, rail))
				return emptyStateBox(ref, ref.Bounds(), rail)
			}
			if want := alone(typ.SystemFont); got != want {
				t.Errorf("the outline column draws %v, want %v — the line at the rows' size", got, want)
			}
			if column := alone(tokens.TextStyle{}); got.Dy() >= column.Dy() {
				t.Errorf("the outline column's line is %d pixels tall, a content column's %d; want the rows' smaller size",
					got.Dy(), column.Dy())
			}
			if c, want := got.Min.X+got.Max.X, outlineW; absInt(c-want) > 4 {
				t.Errorf("the outline column centres its line at %.1f across, want its frame's %.1f",
					float64(c)/2, float64(want)/2)
			}
			if c, want := got.Min.Y+got.Max.Y, docsColumnSize.Y; absInt(c-want) > 4 {
				t.Errorf("the outline column centres its line at %.1f down, want its frame's %.1f",
					float64(c)/2, float64(want)/2)
			}
			if pl, want := plateau(img, rail, got), vgcolor.Flatten(tc.colors.TertiaryLabel, rail); pl.R != want.R || pl.G != want.G || pl.B != want.B {
				t.Errorf("the outline column's line plateaus at #%02x%02x%02x, want #%02x%02x%02x",
					pl.R, pl.G, pl.B, want.R, want.G, want.B)
			}

			// The content column's line and location beside it, against
			// that column drawn alone in the rest of the page.
			content := image.Rect(outlineW, 0, docsColumnSize.X, docsColumnSize.Y)
			gotContent := emptyStateBox(img, content, fill)
			ref := golden.Capture(t, content.Size(), scene(func(gtx layout.Context) layout.Dimensions {
				return drawGuideUnavailable(gtx, shaper, tc.colors, typ, bar, fail)
			}, fill))
			if want := emptyStateBox(ref, ref.Bounds(), fill).Add(image.Pt(outlineW, 0)); gotContent != want {
				t.Errorf("the content column draws %v, want %v — its own group centred in its own frame",
					gotContent, want)
			}
		})
	}
}

// emptyStateSchemes is the appearance pair every reading above is taken
// twice against.
var emptyStateSchemes = []struct {
	name   string
	colors tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// emptyStateProbe draws the pattern alone in the column's frame on the
// column's own fill, from the props it is handed.
func emptyStateProbe(t *testing.T, shaper *text.Shaper, c tokens.PlatformColors,
	typ tokens.Typography, fill color.NRGBA, p shell.EmptyStateProps) *image.RGBA {
	t.Helper()
	return golden.Capture(t, docsColumnSize, scene(func(gtx layout.Context) layout.Dimensions {
		return complayout.Inset(docsOuterInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return shell.EmptyState(gtx, shaper, c, typ, fill, p)
		})
	}, fill))
}

// emptyStateBox is the box the painted group occupies: every pixel
// standing at least a fifth of the way from the column's fill to the
// colour the line's glyphs plateau at.
func emptyStateBox(img *image.RGBA, region image.Rectangle, fill color.NRGBA) image.Rectangle {
	least := fromFill(plateau(img, fill, region), fill) / 5
	box := image.Rectangle{}
	first := true
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if c := img.RGBAAt(x, y); fromFill(c, fill) < least {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// fromFill is how far a painted pixel stands from the fill it was painted
// over, summed across the three channels.
func fromFill(c color.RGBA, fill color.NRGBA) int {
	return absInt(int(c.R)-int(fill.R)) + absInt(int(c.G)-int(fill.G)) + absInt(int(c.B)-int(fill.B))
}

// plateau is the painted colour furthest from the fill inside a region,
// which is the colour of the pixels the glyphs cover wholly.
func plateau(img *image.RGBA, fill color.NRGBA, region image.Rectangle) color.RGBA {
	var found color.RGBA
	far := -1
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if d := fromFill(c, fill); d > far {
				far, found = d, c
			}
		}
	}
	return found
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// captureStderr diverts the stream the application writes its diagnostics
// to and answers the reader that restores it and returns what was written.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	read := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		read <- b.String()
	}()
	return func() string {
		os.Stderr = saved
		w.Close()
		s := <-read
		r.Close()
		return s
	}
}
