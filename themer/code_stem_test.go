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

// codeStemRow is the Code role in a text colour on a fill that draws code,
// with the platform's stem as stored in px.
type codeStemRow struct {
	name     string
	fg, fill color.NRGBA
	stem     float64
}

// codeStemRows are the Code role on the window, as the Typography tab draws
// it, and on the fill of a fenced code block and an inline code span. The
// platform's monospaced row is TextEdit's 11 pt plain-text body, 0.101 em
// in light and 0.122 em in dark, from the reference's section "What the
// stem measures for the semibold, bold and monospaced faces". No stored
// capture holds monospaced text at 13 pt, so the 13 pt stem is DERIVED from
// the 11 pt em: 1.31 px in light and 1.59 px in dark.
func codeStemRows(set tokens.PlatformColors, dark bool) []codeStemRow {
	stem := 1.31
	if dark {
		stem = 1.59
	}
	fence := vgcolor.Flatten(set.AlternatingContentBackground, set.TextBackground)
	return []codeStemRow{
		{"Code on the window", set.Label, set.WindowBackground, stem},
		{"Code on the code fill", set.Text, fence, stem},
	}
}

// TestTheCodeStemIsThePlatforms sets "Vibrant Gio" in the Code role at four
// horizontal offsets, in each text colour on its fill, and stores the
// lines. The text is told its fill. The mean stem of the four lines is
// within 0.1 px of the platform's monospaced stem, DERIVED at 13 pt from
// the platform's 11 pt em.
//
// Regenerate with:
//
//	go test ./ -run TestTheCodeStemIsThePlatforms -golden.update
func TestTheCodeStemIsThePlatforms(t *testing.T) {
	role := tokens.DefaultTypography.Code
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			sh := tokens.DefaultTypography.DeterministicShaper()
			rows := codeStemRows(tc.set, tc.dark)
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
						dims = typeset.Text(gtx, sh, stemWord, role, text, 1)
						return dims
					})
					y := stemLineH * (r*len(stemOffsets) + o)
					bases = append(bases, y+dims.Size.Y-dims.Baseline)
					draw.Draw(img, line.Bounds().Add(image.Pt(0, y)), line, image.Point{}, draw.Src)
				}
			}
			golden.Compare(t, "typography-code-stem-"+tc.name, img)

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
				f := typeset.FontOn(typeset.WithFill(layout.Context{}, fill), role, font.Normal, text)
				var stems []float64
				for o, off := range stemOffsets {
					st, _ := readStem(img, sh, f, role, bases[r*len(stemOffsets)+o], float64(stemLeft+off), fill.R, plateau)
					stems = append(stems, st)
				}
				mean := (stems[0] + stems[1] + stems[2] + stems[3]) / 4
				t.Logf("%s: stems %.2f %.2f %.2f %.2f, mean %.3f; font %q",
					row.name, stems[0], stems[1], stems[2], stems[3], mean, f.Typeface)
				if d := math.Abs(mean - row.stem); d > 0.1+1e-9 {
					t.Errorf("%s: the mean stem is %.3f px, %.3f px from the platform's %.2f", row.name, mean, d, row.stem)
				}
			}
		})
	}
}
