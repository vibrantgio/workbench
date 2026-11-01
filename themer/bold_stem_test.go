package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/vibrantgio/components/golden"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// boldStemRow is one role drawn in Roboto Bold, in its text colour on its
// fill, with the platform's stem as stored in px. The rows are from the
// reference's section "What the stem measures for the semibold, bold and
// monospaced faces". A row with no stem has no platform row to compare.
type boldStemRow struct {
	name     string
	role     tokens.TextStyle
	fg, fill color.NRGBA
	stem     float64
	hasStem  bool
}

func boldStemRows(set tokens.PlatformColors, dark bool) []boldStemRow {
	typ := tokens.DefaultTypography
	section := boldStemRow{"Section heading", typ.SectionHeading, set.Label, set.WindowBackground, 1.73, true}
	headline := boldStemRow{"Headline", typ.Headline, set.Label, set.WindowBackground, 2.02, true}
	small := boldStemRow{"Small section heading", typ.SmallSectionHeading, set.SecondaryLabel, set.SidebarMaterial, 1.52, true}
	if dark {
		section.stem = 1.99
		// No stored capture holds bold text lighter than its fill whose
		// weight is read. TextEdit's dark window title reads 2.08 px, and
		// its weight is not read. So the dark Headline has no platform
		// row, and it draws as shipped.
		headline.stem, headline.hasStem = 0, false
	}
	return []boldStemRow{section, headline, small}
}

// TestTheBoldStemIsThePlatforms sets "Vibrant Gio" in each role that draws
// Roboto Bold at four horizontal offsets, in its text colour on its fill,
// and stores the lines. The text is told its fill. The mean stem of the
// four lines is within 0.06 px of the platform's stem for the role's size
// and weight. The Section heading and the Headline draw at weight 600 and
// weight 700 in one face, and they meet two rows.
//
// Regenerate with:
//
//	go test ./ -run TestTheBoldStemIsThePlatforms -golden.update
func TestTheBoldStemIsThePlatforms(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			sh := tokens.DefaultTypography.DeterministicShaper()
			rows := boldStemRows(tc.set, tc.dark)
			size := image.Pt(stemW, stemLineH*len(rows)*len(stemOffsets))
			img := image.NewRGBA(image.Rectangle{Max: size})
			var bases []int
			for r, row := range rows {
				text := vgcolor.Flatten(row.fg, row.fill)
				for o, off := range stemOffsets {
					var dims layout.Dimensions
					line := golden.Capture(t, image.Pt(stemW, stemLineH), func(gtx layout.Context) layout.Dimensions {
						fillRect(gtx, image.Rectangle{Max: image.Pt(stemW, stemLineH)}, row.fill)
						gtx = typeset.WithFill(gtx, row.fill)
						tr := op.Affine(f32.Affine2D{}.Offset(f32.Pt(stemLeft+off, 0))).Push(gtx.Ops)
						defer tr.Pop()
						dims = typeset.Text(gtx, sh, stemWord, row.role, text, 1)
						return dims
					})
					y := stemLineH * (r*len(stemOffsets) + o)
					bases = append(bases, y+dims.Size.Y-dims.Baseline)
					draw.Draw(img, line.Bounds().Add(image.Pt(0, y)), line, image.Point{}, draw.Src)
				}
			}
			golden.Compare(t, "typography-bold-stem-"+tc.name, img)

			for r, row := range rows {
				fill := row.fill
				top := stemLineH * r * len(stemOffsets)
				// The text colour is the role's plateau: the pixel farthest
				// from the fill in the role's first line.
				plateau := fill.R
				for y := top; y < top+stemLineH; y++ {
					for x := 0; x < stemW; x++ {
						if c := img.RGBAAt(x, y).R; absDiff(c, fill.R) > absDiff(plateau, fill.R) {
							plateau = c
						}
					}
				}
				text := vgcolor.Flatten(row.fg, row.fill)
				f := typeset.FontOn(typeset.WithFill(layout.Context{}, fill), row.role, font.Normal, text)
				var stems []float64
				for o, off := range stemOffsets {
					st, _ := readStem(img, sh, f, row.role, bases[r*len(stemOffsets)+o], float64(stemLeft+off), fill.R, plateau)
					stems = append(stems, st)
				}
				mean := (stems[0] + stems[1] + stems[2] + stems[3]) / 4
				t.Logf("%s %v pt, weight %d: stems %.2f %.2f %.2f %.2f, mean %.3f; font %q",
					row.name, row.role.Size, row.role.Weight, stems[0], stems[1], stems[2], stems[3], mean, f.Typeface)
				if !row.hasStem {
					continue
				}
				if d := math.Abs(mean - row.stem); d > 0.06+1e-9 {
					t.Errorf("%s: the mean stem is %.3f px, %.3f px from the platform's %.2f", row.name, mean, d, row.stem)
				}
			}
		})
	}
}
