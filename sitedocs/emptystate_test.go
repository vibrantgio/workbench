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
				return drawGuideUnavailable(gtx, shaper, tc.colors, typ, fail)
			}, fill))

			got := emptyStateBox(img, img.Bounds(), fill)
			if got.Empty() {
				t.Fatal("the column is bare, want the line the empty state draws")
			}

			ref := emptyStateProbe(t, shaper, tc.colors, typ, fill, shell.EmptyStateProps{
				Line:        "This guide can't be loaded.",
				Description: fail.Path,
			})
			if want := emptyStateBox(ref, ref.Bounds(), fill); got != want {
				t.Errorf("the column draws %v, want %v — the line and the path, and nothing else", got, want)
			}

			// A probe of the line alone proves the path is really drawn:
			// the group is taller with it than without.
			line := emptyStateProbe(t, shaper, tc.colors, typ, fill,
				shell.EmptyStateProps{Line: "This guide can't be loaded."})
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
