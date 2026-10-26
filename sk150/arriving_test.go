package main

// The Presets and Device tabs while their content is arriving: the
// platform's indeterminate indicator stands above the application's own
// sentence, the two centred as one group in the frame the list would occupy,
// and no row of the list is drawn; once the content has arrived both are
// removed.

import (
	"image"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/progress"
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
// page's column, which runs to the window's edges at either setting: a list
// with no rows draws no bar, so it takes no lane.
func listFrame(t *testing.T, img *image.RGBA, tc int, under int) image.Rectangle {
	t.Helper()
	top := monitorFrame(t, img, schemes[tc].c).Min.Y
	return image.Rect(0, top, windowSize.X, windowSize.Y-under)
}

func renderTab(t *testing.T, tc int, m Model, bars desktop.ScrollBarSetting) *image.RGBA {
	t.Helper()
	w := windowFrameBars(t, schemes[tc].c, m, postMessage, bars)
	golden.Capture(t, windowSize, w)
	return golden.Capture(t, windowSize, w)
}

// TestThePresetsTabDrawsTheIndicatorAboveTheArrivingSentence holds the Presets tab while
// the memory slots are arriving; see [checkArrivingSentence].
func TestThePresetsTabDrawsTheIndicatorAboveTheArrivingSentence(t *testing.T) {
	checkArrivingSentence(t, arrivingTabs[0])
}

// TestTheDeviceTabDrawsTheIndicatorAboveTheArrivingSentence holds the Device tab while the
// device settings are arriving; see [checkArrivingSentence].
func TestTheDeviceTabDrawsTheIndicatorAboveTheArrivingSentence(t *testing.T) {
	checkArrivingSentence(t, arrivingTabs[1])
}

// checkArrivingSentence holds one tab while its content is arriving, in
// both schemes and at both bar settings: the frame the list would occupy
// holds two runs of painted rows and no row of the list. The first run is
// the indicator, 16 px tall and centred across the frame. The second is the
// sentence's line, its glyphs beginning under the indicator at the DERIVED
// gap, painted in the label colour over the page's fill. The two centre down
// the frame as one group. After the content arrives both are removed: the
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
					frame := listFrame(t, img, i, tab.under)
					runs := paintedRuns(img, frame, fill)
					if len(runs) != 2 {
						t.Fatalf("the list's frame %v holds %d runs of painted rows %v, want the indicator and the sentence's line and no row", frame, len(runs), runs)
					}
					ind, line := runs[0], runs[1]
					if ind.Dx() != 16 || ind.Dy() != 16 {
						t.Errorf("the indicator paints %v, want the measured 16 × 16", ind)
					}
					if got, want := ind.Min.X+ind.Max.X, frame.Min.X+frame.Max.X; abs(got-want) > 1 {
						t.Errorf("the indicator centres at %.1f across, want the frame's %.1f", float64(got)/2, float64(want)/2)
					}
					// The line box holds a few rows of room above the
					// glyphs, so the glyphs begin at most that much past
					// the gap.
					gap, g := line.Min.Y-ind.Max.Y, int(progress.Gap)
					if gap < g || gap > g+6 {
						t.Errorf("the sentence's glyphs begin %d px under the indicator, want the gap %d and at most the line's own 6 px above its glyphs", gap, g)
					}
					if got, want := line.Min.X+line.Max.X, frame.Min.X+frame.Max.X; abs(got-want) > 2 {
						t.Errorf("the sentence centres at %.1f across, want the frame's %.1f", float64(got)/2, float64(want)/2)
					}
					if got, want := ind.Min.Y+line.Max.Y, frame.Min.Y+frame.Max.Y; abs(got-want) > 6 {
						t.Errorf("the group centres at %.1f down, want the frame's %.1f", float64(got)/2, float64(want)/2)
					}
					t.Logf("frame %v centre (%.1f, %.1f); indicator %v; sentence's glyphs %v; gap %d", frame,
						float64(frame.Min.X+frame.Max.X)/2, float64(frame.Min.Y+frame.Max.Y)/2, ind, line, gap)
					// A 13 px glyph's stems seldom cover a whole pixel, so
					// the line is held by the pixel nearest the label
					// colour: within 16 levels of it, where the secondary
					// label lies about 80 levels off.
					want := vgcolor.Flatten(sc.c.Label, fill)
					nearest := 255
					for y := line.Min.Y; y < line.Max.Y; y++ {
						for x := line.Min.X; x < line.Max.X; x++ {
							px := at(img, x, y)
							d := max(abs(int(px.R)-int(want.R)), abs(int(px.G)-int(want.G)), abs(int(px.B)-int(want.B)))
							nearest = min(nearest, d)
						}
					}
					if nearest > 16 {
						t.Errorf("the sentence's nearest glyph pixel lies %d levels off Label over the page's fill, #%02x%02x%02x, want within 16", nearest, want.R, want.G, want.B)
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
						t.Errorf("after the content arrives the first painted run begins %d px under the list's top, want the rows at its top and no indicator or sentence", got)
					}
				})
			}
		}
	}
}

// TestTheMonitorTabDrawsTheIndicatorAboveCollectingSamples holds the Monitor
// tab's history strip while the samples are arriving, in both schemes: the
// last two runs of painted rows in the tab are the indicator, 16 px square
// and centred across the window, and the phrase under it at the DERIVED gap.
// Once the history arrives neither is drawn: no run is the 16 px indicator
// standing over a run of text.
func TestTheMonitorTabDrawsTheIndicatorAboveCollectingSamples(t *testing.T) {
	for i, sc := range schemes {
		t.Run(sc.name, func(t *testing.T) {
			m := goldenModel()
			m.History = nil
			fill := pageFill(sc.c)
			img := renderTab(t, i, m, desktop.ScrollBarsWhenScrolling)
			frame := monitorFrame(t, img, sc.c)
			runs := paintedRuns(img, frame, fill)
			if len(runs) < 2 {
				t.Fatalf("the tab holds %d runs of painted rows, want the indicator and the phrase under the readout", len(runs))
			}
			ind, line := runs[len(runs)-2], runs[len(runs)-1]
			if ind.Dx() != 16 || ind.Dy() != 16 {
				t.Errorf("the indicator paints %v, want the measured 16 × 16", ind)
			}
			if got, want := ind.Min.X+ind.Max.X, frame.Min.X+frame.Max.X; abs(got-want) > 1 {
				t.Errorf("the indicator centres at %.1f across, want the frame's %.1f", float64(got)/2, float64(want)/2)
			}
			if got, want := line.Min.X+line.Max.X, frame.Min.X+frame.Max.X; abs(got-want) > 2 {
				t.Errorf("the phrase centres at %.1f across, want the frame's %.1f", float64(got)/2, float64(want)/2)
			}
			gap, g := line.Min.Y-ind.Max.Y, int(progress.Gap)
			if gap < g || gap > g+6 {
				t.Errorf("the phrase's glyphs begin %d px under the indicator, want the gap %d and at most the line's own 6 px above its glyphs", gap, g)
			}
			t.Logf("frame %v; indicator %v; phrase %v; gap %d", frame, ind, line, gap)

			m.History = goldenHistory()
			img = renderTab(t, i, m, desktop.ScrollBarsWhenScrolling)
			for _, r := range paintedRuns(img, frame, fill) {
				if r.Dx() == 16 && r.Dy() == 16 {
					t.Errorf("after the history arrives a 16 × 16 run %v is drawn, want the charts and no indicator", r)
				}
			}
		})
	}
}
