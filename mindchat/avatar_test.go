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

// TestTheAvatarIsDrawnInTheTranscriptsTextColour draws the assistant's avatar
// symbol on the transcript's fill in both schemes. Under the Icon entry a
// symbol on the content is in the text colour: the symbol's solid pixels are
// the transcript's text colour, and no pixel is near the accent.
func TestTheAvatarIsDrawnInTheTranscriptsTextColour(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			avatar, err := avatarMark(p)
			if err != nil {
				t.Fatalf("avatar raster: %v", err)
			}
			size := image.Pt(int(AvatarSize), int(AvatarSize))
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(size)
				paint.FillShape(gtx.Ops, p.Transcript, clip.Rect{Max: size}.Op())
				return avatar(gtx)
			})
			text, accent := symbolPixels(img, p.TurnText, tc.c)
			if text == 0 {
				t.Errorf("no pixel of the avatar is the transcript's text colour %v", p.TurnText)
			}
			if accent != 0 {
				t.Errorf("%d pixels of the avatar are near the accent %v", accent, tc.c.ControlAccent)
			}
		})
	}
}

// symbolPixels counts the pixels of img that are exactly fg, and the pixels
// within a small distance of the platform's accent.
func symbolPixels(img *image.RGBA, fg color.NRGBA, c tokens.PlatformColors) (exact, accent int) {
	near := func(a, b color.NRGBA) bool {
		d := func(x, y uint8) int {
			if x > y {
				return int(x - y)
			}
			return int(y - x)
		}
		return d(a.R, b.R)+d(a.G, b.G)+d(a.B, b.B) <= 24
	}
	r := img.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			px := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 0xff}
			if px == fg {
				exact++
			}
			if near(px, c.ControlAccent) {
				accent++
			}
		}
	}
	return exact, accent
}
