package main

// The Presets and Device tabs while their content is arriving: the
// application's own sentence is drawn as an empty state is placed, centred
// both ways in the frame the list would occupy, and no row of the list is
// drawn; once the content has arrived the sentence is removed.

import (
	"image"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	vgcolor "github.com/vibrantgio/theme/color"
)

// arrivingTabs are the two list tabs, each with the model field that says
// its content has arrived, its sentence, and the room under its list: the
// Device tab's footer row keeps the tab's 4 dp and the page's bottom
// Padding under the list, the Presets list runs to the window's bottom edge.
type arrivingTab struct {
	screen   string
	arrive   func(m *Model)
	arriving func(Model) string
	line     string
	under    int
}

var arrivingTabs = []arrivingTab{
	{"presets", func(m *Model) { m.HavePresets = true }, presetsArriving, "Reading the memory slots…", 0},
	{"device", func(m *Model) { m.HaveD = true }, deviceArriving, "Reading the device settings…", 4 + int(Padding)},
}

// listFrame is the frame the tab's list occupies, read off the arrangement:
// from the row under the tab strip's trim to the list's foot, across the
// page's column, which runs to the window's edges, less the bar's lane when
// the bar stands.
func listFrame(t *testing.T, img *image.RGBA, tc int, bars desktop.ScrollBarSetting, under int) image.Rectangle {
	t.Helper()
	c := schemes[tc].c
	top := monitorFrame(t, img, c).Min.Y
	right := windowSize.X
	if bars == desktop.ScrollBarsAlways {
		right -= int(themed{platform: c, scrollBars: bars}.listBar().Width())
	}
	return image.Rect(0, top, right, windowSize.Y-under)
}

func renderTab(t *testing.T, tc int, m Model, bars desktop.ScrollBarSetting) *image.RGBA {
	t.Helper()
	w := windowFrameBars(t, schemes[tc].c, m, postMessage, bars)
	golden.Capture(t, windowSize, w)
	return golden.Capture(t, windowSize, w)
}

// TestThePresetsTabCentresTheArrivingSentence holds the Presets tab while
// the memory slots are arriving; see [checkArrivingSentence].
func TestThePresetsTabCentresTheArrivingSentence(t *testing.T) {
	checkArrivingSentence(t, arrivingTabs[0])
}

// TestTheDeviceTabCentresTheArrivingSentence holds the Device tab while the
// device settings are arriving; see [checkArrivingSentence].
func TestTheDeviceTabCentresTheArrivingSentence(t *testing.T) {
	checkArrivingSentence(t, arrivingTabs[1])
}

// checkArrivingSentence holds one tab while its content is arriving, in
// both schemes and at both bar settings: the frame
// the list would occupy holds one run of painted rows, the sentence's line,
// centred across and down in it, painted in the empty state's tertiary
// label over the page's fill and not in the secondary label a row of the
// list is set in. After the content arrives the sentence is removed: the
// tab's first painted run begins at the list's top, where the rows begin.
func checkArrivingSentence(t *testing.T, tab arrivingTab) {
	for i, sc := range schemes {
		{
			for _, bars := range []desktop.ScrollBarSetting{desktop.ScrollBarsWhenScrolling, desktop.ScrollBarsAlways} {
				name := sc.name + "-" + tab.screen + "-whenscrolling"
				if bars == desktop.ScrollBarsAlways {
					name = sc.name + "-" + tab.screen + "-always"
				}
				t.Run(name, func(t *testing.T) {
					m := goldenModel()
					m.Screen = tab.screen
					if got := tab.arriving(m); got != tab.line {
						t.Fatalf("while the content is arriving the sentence is %q, want %q", got, tab.line)
					}
					fill := pageFill(sc.c)
					img := renderTab(t, i, m, bars)
					frame := listFrame(t, img, i, bars, tab.under)
					runs := paintedRuns(img, frame, fill)
					if len(runs) != 1 {
						t.Fatalf("the list's frame %v holds %d runs of painted rows %v, want the sentence's line alone and no row", frame, len(runs), runs)
					}
					line := runs[0]
					if got, want := line.Min.X+line.Max.X, frame.Min.X+frame.Max.X; abs(got-want) > 2 {
						t.Errorf("the sentence centres at %.1f across, want the frame's %.1f", float64(got)/2, float64(want)/2)
					}
					if got, want := line.Min.Y+line.Max.Y, frame.Min.Y+frame.Max.Y; abs(got-want) > 6 {
						t.Errorf("the sentence centres at %.1f down, want the frame's %.1f", float64(got)/2, float64(want)/2)
					}
					t.Logf("frame %v centre (%.1f, %.1f); sentence's glyphs %v centre (%.1f, %.1f)", frame,
						float64(frame.Min.X+frame.Max.X)/2, float64(frame.Min.Y+frame.Max.Y)/2, line,
						float64(line.Min.X+line.Max.X)/2, float64(line.Min.Y+line.Max.Y)/2)
					want := vgcolor.Flatten(sc.c.TertiaryLabel, fill)
					found := false
					for y := line.Min.Y; y < line.Max.Y && !found; y++ {
						for x := line.Min.X; x < line.Max.X; x++ {
							if sameColour(at(img, x, y), want) {
								found = true
								break
							}
						}
					}
					if !found {
						t.Errorf("no glyph of the sentence is painted TertiaryLabel over the page's fill, #%02x%02x%02x", want.R, want.G, want.B)
					}

					tab.arrive(&m)
					if got := tab.arriving(m); got != "" {
						t.Fatalf("after the content arrives the sentence is %q, want none", got)
					}
					img = renderTab(t, i, m, bars)
					runs = paintedRuns(img, frame, fill)
					if len(runs) == 0 {
						t.Fatal("after the content arrives the list's frame is bare, want its rows")
					}
					// The list's first row is drawn under the tab's 4 dp and
					// the 10 dp above the first row.
					if got := runs[0].Min.Y - frame.Min.Y; got > 4+10+8 {
						t.Errorf("after the content arrives the first painted run begins %d px under the list's top, want the rows at its top and no centred sentence", got)
					}
				})
			}
		}
	}
}
