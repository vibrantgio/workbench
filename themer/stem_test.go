package main

import (
	"image"
	"image/draw"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	fx "golang.org/x/image/math/fixed"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// The platform's rows, as stored, from the reference's section "What the
// same text measures in a platform window and a library window". Each row
// is the lowercase stem of "Vibrant Gio" in px, the other stem where the
// platform gives two, and the darkest pixel's share of the way from the
// fill to the text colour.
type stemRow struct {
	role       string
	size       int
	dark       bool
	stem, also float64
	darkest    float64
}

var platformStems = []stemRow{
	{"System font", 13, false, 1.26, 1.26, 1.00},
	{"System font", 13, true, 1.51, 1.57, 1.00},
	{"Caption 1", 10, false, 0.97, 0.97, 0.90},
	{"Caption 1", 10, true, 1.17, 1.17, 1.00},
}

// stemOffsets are the four horizontal offsets the word is set at, in px.
var stemOffsets = []float32{0, 0.25, 0.5, 0.75}

const (
	stemWord  = "Vibrant Gio"
	stemLineH = 24
	stemLeft  = 20
	stemW     = 120
)

// TestTheStemIsThePlatforms sets "Vibrant Gio" in the System font and in
// Caption 1 at four horizontal offsets, in the label colour on
// controlBackground, and stores the eight lines. The text is told that fill,
// so it is darker text in light and lighter text in dark. The mean stem of
// the four lines is within 0.06 px of the platform's, and the darkest of the
// four is within 5 % of the platform's share or above it.
//
// Regenerate with:
//
//	go test ./ -run TestTheStemIsThePlatforms -golden.update
func TestTheStemIsThePlatforms(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			typ := tokens.DefaultTypography
			sh := typ.DeterministicShaper()
			roles := []tokens.TextStyle{typ.SystemFont, typ.Caption1}
			// Each line is its own capture. One frame draws a glyph once and
			// places that drawing at each offset, so four lines in one frame
			// show one offset four times.
			size := image.Pt(stemW, stemLineH*len(roles)*len(stemOffsets))
			img := image.NewRGBA(image.Rectangle{Max: size})
			text := color.Flatten(tc.set.Label, tc.set.ControlBackground)
			// bases holds each line's baseline: the first row under it.
			var bases []int
			for r, role := range roles {
				for o, off := range stemOffsets {
					var dims layout.Dimensions
					line := golden.Capture(t, image.Pt(stemW, stemLineH), func(gtx layout.Context) layout.Dimensions {
						fillRect(gtx, image.Rectangle{Max: image.Pt(stemW, stemLineH)}, tc.set.ControlBackground)
						gtx = typeset.WithFill(gtx, tc.set.ControlBackground)
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
			golden.Compare(t, "typography-stem-"+tc.name, img)

			fill := tc.set.ControlBackground
			// The text colour is the 13 pt line's plateau: the pixel
			// farthest from the fill.
			plateau := fill.R
			for y := 0; y < stemLineH; y++ {
				for x := 0; x < stemW; x++ {
					c := img.RGBAAt(x, y).R
					if absDiff(c, fill.R) > absDiff(plateau, fill.R) {
						plateau = c
					}
				}
			}
			for r, role := range roles {
				want := platformStemRow(int(role.Size), tc.dark)
				var stems []float64
				darkest := 0.0
				for o, off := range stemOffsets {
					base := bases[r*len(stemOffsets)+o]
					f := typeset.FontOn(typeset.WithFill(layout.Context{}, fill), role, font.Normal, text)
					st, dk := readStem(img, sh, f, role, base, float64(stemLeft+off), fill.R, plateau)
					stems = append(stems, st)
					darkest = max(darkest, dk)
				}
				mean := (stems[0] + stems[1] + stems[2] + stems[3]) / 4
				t.Logf("%s %v pt: stems %.2f %.2f %.2f %.2f, mean %.3f; darkest %.0f %%; text colour %#02x",
					want.role, role.Size, stems[0], stems[1], stems[2], stems[3], mean, 100*darkest, plateau)
				if d := min(math.Abs(mean-want.stem), math.Abs(mean-want.also)); d > 0.06+1e-9 {
					t.Errorf("%s: the mean stem is %.3f px, %.3f px from the platform's %.2f", want.role, mean, d, want.stem)
				}
				if darkest < want.darkest-0.05 {
					t.Errorf("%s: the darkest pixel reaches %.0f %%, under the platform's %.0f %% less 5 %%", want.role, 100*darkest, 100*want.darkest)
				}
			}
		})
	}
}

func platformStemRow(size int, dark bool) stemRow {
	for _, r := range platformStems {
		if r.size == size && r.dark == dark {
			return r
		}
	}
	panic("no platform row")
}

func absDiff(a, b uint8) int {
	return int(math.Abs(float64(a) - float64(b)))
}

// readStem reads one line the reference's way. The stem is the left stem of
// the n in "Vibrant" at 13 pt and its right stem at 10 pt, as in the
// platform's readings. The row is four rows over the baseline at 13 pt and
// three at 10 pt. The baseline is the line's own, from its layout. The stem is the coverage summed over the run of covered
// columns, as stored. The darkest is the largest share over the stem's
// columns and the rows from the reading row to the baseline. f is the font
// the line is drawn in, so the reader finds the glyphs the line draws.
func readStem(img *image.RGBA, sh *text.Shaper, f font.Font, role tokens.TextStyle, base int, left float64, fill, plateau uint8) (stem, darkest float64) {
	sh.LayoutString(text.Parameters{
		Font:     f,
		PxPerEm:  fx.Int26_6(role.Size * 64),
		MaxLines: 1, MaxWidth: 10000,
	}, stemWord)
	var nx, nadv float64
	for i := 0; ; i++ {
		g, ok := sh.NextGlyph()
		if !ok {
			break
		}
		if i == 5 {
			nx = float64(g.X)/64 + left
			nadv = float64(g.Advance) / 64
		}
	}
	fv, tv := float64(fill), float64(plateau)
	cov := func(x, y int) float64 {
		c := img.RGBAAt(x, y)
		return ((fv-float64(c.R))/(fv-tv) + (fv-float64(c.G))/(fv-tv) + (fv-float64(c.B))/(fv-tv)) / 3
	}
	row, ylo := base-4, base-6
	xa, xb := int(math.Floor(nx+0.56)), int(math.Floor(nx+nadv/2))
	if role.Size == 10 {
		row, ylo = base-3, base-4
		xa, xb = int(math.Floor(nx+3.57)), int(math.Ceil(nx+nadv))
	}
	for xa <= xb && cov(xa, row) < 0.005 {
		xa++
	}
	e := xa
	peak := false
	for e+1 <= xa+3 {
		c, n := cov(e, row), cov(e+1, row)
		if n < 0.01 {
			break
		}
		if n < c {
			peak = true
		} else if peak {
			break
		}
		e++
	}
	for x := xa; x <= e; x++ {
		stem += cov(x, row)
	}
	for y := ylo; y < base; y++ {
		for x := xa; x <= e; x++ {
			darkest = max(darkest, (fv-float64(img.RGBAAt(x, y).R))/(fv-tv))
		}
	}
	return stem, darkest
}
