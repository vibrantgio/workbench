package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// formThemed is the slice of settingsThemed the form's two columns are a
// function of: the palette the labels are drawn in, and the type they are
// set in.
func formThemed(c tokens.PlatformColors) settingsThemed {
	typ := tokens.DefaultTypography
	return settingsThemed{
		palette: PaletteFrom(c),
		colors:  c,
		typ:     typ,
		shaper:  typ.DeterministicShaper(),
	}
}

// TestTheFormLabelsEndOnOneColumn reads the dialog's label column back off
// the drawing: every row label is right-aligned in it, so all four end
// together whatever their length.
//
// MEASURED, save-dialog-{light,dark}.png: "Save As:", "Tags:" and "Where:"
// lead at x 206, 225 and 214 and all three end at x=255.
func TestTheFormLabelsEndOnOneColumn(t *testing.T) {
	c := tokens.PlatformLight
	th := formThemed(c)
	size := image.Pt(400, 40)
	ends := make(map[string]int, len(settingsRowLabels))
	for _, label := range settingsRowLabels {
		var cols settingsColumns
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, c.WindowBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
			cols = formColumns(gtx, th)
			return formRow(th, cols, label, SettingsFieldHeight, blankRow)(gtx)
		})
		last := -1
		for x := 0; x < cols.label+cols.gap; x++ {
			for y := 0; y < size.Y; y++ {
				if img.RGBAAt(x, y) != color.RGBA(c.WindowBackground) {
					last = x
					break
				}
			}
		}
		ends[label] = last
	}
	var want int
	for _, label := range settingsRowLabels {
		if want == 0 {
			want = ends[label]
			continue
		}
		// One column of tolerance: the labels are right-aligned on their
		// advance and the last column a colon covers depends on the
		// sub-pixel phase that alignment leaves it at.
		if d := ends[label] - want; d < -1 || d > 1 {
			t.Errorf("%q ends at column %d where %q ends at %d; the column is right-aligned and every label ends on it",
				label, ends[label], settingsRowLabels[0], want)
		}
	}
	if want <= 0 {
		t.Fatal("no label drew anything")
	}
}

// TestTheFormContentStandsOnTheFieldColumn reads the other column back: a
// row's content begins the measured gap past the label column, whatever the
// row's own label is, and a row with no label begins there too.
//
// MEASURED, save-dialog-{light,dark}.png: the "Save As:" and "Tags:" field
// boxes and the "Where:" pop-up all begin at x=264 against a label column
// ending at x=255.
func TestTheFormContentStandsOnTheFieldColumn(t *testing.T) {
	c := tokens.PlatformLight
	th := formThemed(c)
	size := image.Pt(400, 40)
	mark := color.NRGBA{R: 0xff, B: 0xff, A: 0xff}
	content := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, mark, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	for _, label := range append([]string{""}, settingsRowLabels...) {
		var cols settingsColumns
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, c.WindowBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
			cols = formColumns(gtx, th)
			return formRow(th, cols, label, SettingsFieldHeight, content)(gtx)
		})
		first := -1
		for x := 0; x < size.X; x++ {
			if img.RGBAAt(x, size.Y/2) == color.RGBA(mark) {
				first = x
				break
			}
		}
		if want := cols.label + cols.gap; first != want {
			t.Errorf("the row labelled %q begins its content at column %d, want the field column %d", label, first, want)
		}
		if cols.gap != int(SettingsLabelGap) {
			t.Errorf("the gap to the field column measures %d, want the platform's %d", cols.gap, int(SettingsLabelGap))
		}
	}
}
