package main

// What the settings sheet says when the provider catalogue is empty, and
// where it says it: the well's own empty state, in the frame its rows
// would have occupied.

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	marks "github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// wellThemed is the slice of settingsThemed the providers well is a
// function of: the palette and type it draws in, the bar its rows would
// scroll under, and the two marks its add/remove pair carries — built the
// way the dialog's own theme emission builds them.
func wellThemed(c tokens.PlatformColors) settingsThemed {
	typ := tokens.DefaultTypography
	return settingsThemed{
		palette: PaletteFrom(c),
		colors:  c,
		bar:     scrollbar.FromTokens(c, c.WindowBackground),
		typ:     typ,
		shaper:  typ.DeterministicShaper(),
		add:     marks.Mark(marks.Plus),
		remove:  marks.Mark(marks.Minus),
	}
}

// wellSize is the frame the providers well stands in inside the dialog
// body: the well's own width, the body's own height.
var wellSize = image.Pt(int(SettingsListWidth), int(SettingsBodyHeight))

// TestTheEmptyProviderWellCentresItsLineInTheRowsFrame holds the sheet's
// empty state to the platform's placement for a field: the line centres
// both ways in the frame the well's rows would have occupied — not the
// well's whole box, which carries the "Providers" caption above those rows
// and the add/remove pair below them — and it is set at the size those
// rows are set at, not at a content column's reading.
//
// The line is read off a render of the well itself inside that rectangle,
// and held against what patterns/shell draws alone in a frame of the same
// size: the two boxes are equal to the pixel, so the well hands the
// pattern the rows' whole frame and nothing less.
func TestTheEmptyProviderWellCentresItsLineInTheRowsFrame(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			th := wellThemed(tc.c)
			fill := th.palette.Pane
			img := golden.Capture(t, wellSize, wellScene(th, SettingsState{}, tc.c))
			region := wellRowsRegion(t, th)
			if region.Empty() {
				t.Fatal("the well laid out no row region")
			}

			got := wellGlyphBox(img, region, fill)
			if got.Empty() {
				t.Fatal("the well is bare, want the line the empty state draws")
			}

			ref := wellEmptyStateProbe(t, th, region.Size(), settingsNoProvidersLine)
			refBox := wellGlyphBox(ref, ref.Bounds(), fill)
			if want := refBox.Add(region.Min); got != want {
				t.Errorf("the well draws its line at %v, want %v — the frame the line centres in is the rows' own",
					got, want)
			}
			if c, want := refBox.Min.X+refBox.Max.X, region.Dx(); abs(c-want) > 2 {
				t.Errorf("the well centres its line at %.1f across, want the rows' %.1f",
					float64(c)/2, float64(want)/2)
			}
			if c, want := refBox.Min.Y+refBox.Max.Y, region.Dy(); abs(c-want) > 4 {
				t.Errorf("the well centres its line at %.1f down, want the rows' %.1f",
					float64(c)/2, float64(want)/2)
			}

			// The size is the rows' own and not the content column's
			// measured reading, which is twice it.
			rows := wellLineHeight(t, th, th.typ.BodyMedium)
			column := th.typ.HeadlineMedium
			column.LineHeight = 0
			if h := got.Dy(); h != rows {
				t.Errorf("the well sets its line %d pixels tall, want the rows' own %d", h, rows)
			} else if h == wellLineHeight(t, th, column) {
				t.Errorf("the well sets its line at the content column's reading, %v sp", column.Size)
			}

			// And it spends the platform's tertiary label over the fill
			// the well is painted in.
			want := vgcolor.Flatten(tc.c.TertiaryLabel, fill)
			if p := wellPlateau(img, fill, region); p.R != want.R || p.G != want.G || p.B != want.B {
				t.Errorf("the well's line plateaus at #%02x%02x%02x, want TertiaryLabel over the pane, #%02x%02x%02x",
					p.R, p.G, p.B, want.R, want.G, want.B)
			}
		})
	}
}

// TestTheEmptyProviderWellStandsOutsideAStandingBarsLane holds the rows'
// frame against the reader's setting: where the well's bar stands, the
// frame the line centres in is the rows' less the bar's lane on the
// trailing side, so the line's centre stands half a lane further in than
// where the bar fades; up and down it does not move.
func TestTheEmptyProviderWellStandsOutsideAStandingBarsLane(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			fading := wellThemed(tc.c)
			standing := fading
			standing.bar = scrollBarStyle(fading.bar, desktop.ScrollBarsAlways)
			fill := fading.palette.Pane
			region := wellRowsRegion(t, fading)
			box := func(th settingsThemed) image.Rectangle {
				img := golden.Capture(t, wellSize, wellScene(th, SettingsState{}, tc.c))
				b := wellGlyphBox(img, region, fill)
				if b.Empty() {
					t.Fatal("the well is bare, want the line the empty state draws")
				}
				return b
			}
			at, still := box(standing), box(fading)
			lane := int(standing.bar.Width())
			moved := (still.Min.X + still.Max.X) - (at.Min.X + at.Max.X)
			if abs(moved-lane) > 2 {
				t.Errorf("at Always the line's centre moves %.1f px in, want half the %d px lane, %.1f",
					float64(moved)/2, lane, float64(lane)/2)
			}
			if still.Min.Y != at.Min.Y || still.Max.Y != at.Max.Y {
				t.Errorf("at Always the line stands at y %d–%d, want %d–%d as where the bar fades",
					at.Min.Y, at.Max.Y, still.Min.Y, still.Max.Y)
			}
		})
	}
}

// TestTheEmptyProviderWellNamesNoControl holds the other half of the
// ruling: the line is the sheet's own sentence about what is missing, so
// the add segment it used to spell out is not in it. The control is under
// the well, where it can be pressed.
func TestTheEmptyProviderWellNamesNoControl(t *testing.T) {
	if strings.Contains(settingsNoProvidersLine, "+") {
		t.Errorf("the well says %q, which names the add control; the line names none",
			settingsNoProvidersLine)
	}
	for _, named := range []string{"add", "Add", "button", "press", "click"} {
		if strings.Contains(settingsNoProvidersLine, named) {
			t.Errorf("the well says %q, which names a control with %q", settingsNoProvidersLine, named)
		}
	}
}

// wellScene draws the providers well alone on the plane the dialog stands
// on, at the frame the body gives it.
func wellScene(th settingsThemed, s SettingsState, c tokens.PlatformColors) layout.Widget {
	clicks := map[int]*widget.Clickable{}
	var addClick, removeClick widget.Clickable
	rows := list.NewState()
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.WindowBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return providerColumn(gtx, th, s, clicks, &addClick, &removeClick, rows)
	}
}

// wellRowsRegion is the rectangle the well's rows stand in, in the well's
// own frame: the pane's inset on every side, the caption row off the top,
// and the add/remove pair with its own air off the foot. It is the flex's
// arithmetic restated, with the pair's height measured rather than
// assumed, since the segmented control reports its own.
func wellRowsRegion(t *testing.T, th settingsThemed) image.Rectangle {
	t.Helper()
	inset := int(SettingsPaneInset)
	pair := drawnSize(t, image.Pt(wellSize.X-2*inset, wellSize.Y), func(gtx layout.Context) layout.Dimensions {
		return addRemovePair(gtx, th, new(widget.Clickable), new(widget.Clickable))
	}).Y
	return image.Rect(
		inset, inset+int(SettingsCaptionRow),
		wellSize.X-inset, wellSize.Y-inset-inset-pair,
	)
}

// drawnSize lays one layout.Widget out at the given frame and reports
// what it measured itself at.
func drawnSize(t *testing.T, frame image.Point, w layout.Widget) image.Point {
	t.Helper()
	var dims layout.Dimensions
	golden.Capture(t, frame, func(gtx layout.Context) layout.Dimensions {
		dims = w(gtx)
		return dims
	})
	return dims.Size
}

// wellEmptyStateProbe draws the pattern alone in a frame of the given size
// on the well's own fill, at the size the well's rows are set at.
func wellEmptyStateProbe(t *testing.T, th settingsThemed, size image.Point, line string) *image.RGBA {
	t.Helper()
	fill := th.palette.Pane
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return shell.EmptyState(gtx, th.shaper, th.colors, th.typ, fill,
			shell.EmptyStateProps{Line: line, LineStyle: th.typ.BodyMedium})
	})
}

// wellLineHeight is how tall the painted glyphs of the line stand when it
// is set at a stated style in the colour the empty state spends, read off
// a drawing of nothing else.
func wellLineHeight(t *testing.T, th settingsThemed, style tokens.TextStyle) int {
	t.Helper()
	fill := th.palette.Pane
	size := image.Pt(600, 200)
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return typeset.Text(gtx, th.shaper, settingsNoProvidersLine, style,
			vgcolor.Flatten(th.colors.TertiaryLabel, fill), 1)
	})
	return wellGlyphBox(img, img.Bounds(), fill).Dy()
}

// wellGlyphBox is the box the line's glyphs occupy inside one region of a
// render: every pixel standing at least a fifth of the way from the well's
// own fill to the colour the glyphs plateau at. A share of the plateau
// rather than "not the fill", because the pane's rounded corners leave a
// ramp of anti-aliased pixels a cruder reading would take for a glyph.
func wellGlyphBox(img *image.RGBA, region image.Rectangle, fill color.NRGBA) image.Rectangle {
	least := wellFromFill(wellPlateau(img, fill, region), fill) / 5
	box := image.Rectangle{}
	first := true
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if c := img.RGBAAt(x, y); wellFromFill(c, fill) < least {
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

// wellFromFill is how far a painted pixel stands from the fill it was
// painted over, summed across the three channels.
func wellFromFill(c color.RGBA, fill color.NRGBA) int {
	return abs(int(c.R)-int(fill.R)) + abs(int(c.G)-int(fill.G)) + abs(int(c.B)-int(fill.B))
}

// wellPlateau is the painted colour furthest from the fill inside a
// region, which is the colour of the pixels the glyphs cover wholly.
func wellPlateau(img *image.RGBA, fill color.NRGBA, region image.Rectangle) color.RGBA {
	var found color.RGBA
	far := -1
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if d := wellFromFill(c, fill); d > far {
				far, found = d, c
			}
		}
	}
	return found
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
