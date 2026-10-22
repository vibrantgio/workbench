package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
)

// TestAppIconsAreDrawnInTheTextColour renders every application's icon on the
// window's own plane in both schemes. Every pixel the icon draws is a mix of
// the plane and the text colour, the text colour itself is among them, and no
// pixel is the accent.
func TestAppIconsAreDrawnInTheTextColour(t *testing.T) {
	for _, sc := range windowSchemes {
		t.Run(sc.name, func(t *testing.T) {
			tok := staticThemed(sc.colors)
			plane, text, accent := windowPlane(sc.colors), textColour(sc.colors), sc.colors.ControlAccent
			size := image.Pt(int(IconSize), int(IconSize))
			for _, app := range Apps {
				img := golden.Capture(t, size, golden.Stack(backdrop.Widget(plane), appIcon(tok, app)))
				drawn, full, accents, unmixed := 0, 0, 0, 0
				for y := 0; y < size.Y; y++ {
					for x := 0; x < size.X; x++ {
						c := pixelAt(img, image.Pt(x, y))
						if c == plane {
							continue
						}
						drawn++
						if c == text {
							full++
						}
						if channelGap(c, accent) == 0 {
							accents++
						}
						if !between(c, plane, text) {
							unmixed++
						}
					}
				}
				if drawn == 0 || full == 0 || accents != 0 || unmixed != 0 {
					t.Errorf("%s: %d pixels drawn, %d the text colour %v, %d the accent %v, %d no mix of the plane %v and the text colour; want the first two above zero and the last two zero",
						app.Name, drawn, full, text, accents, accent, unmixed, plane)
				}
			}
		})
	}
}

// between reports whether every channel of c lies within one 255th of the
// span from a to b, which is where every mix of the two lands.
func between(c, a, b color.NRGBA) bool {
	in := func(v, lo, hi uint8) bool {
		if lo > hi {
			lo, hi = hi, lo
		}
		return int(v) >= int(lo)-1 && int(v) <= int(hi)+1
	}
	return in(c.R, a.R, b.R) && in(c.G, a.G, b.G) && in(c.B, a.B, b.B)
}

// TestLaunchButtonIsTheDefaultButton renders the launch button at rest in
// both schemes: its fill is read from the platform's default button fill, and
// its title is drawn in the text colour the token is paired with.
func TestLaunchButtonIsTheDefaultButton(t *testing.T) {
	for _, sc := range windowSchemes {
		t.Run(sc.name, func(t *testing.T) {
			tok := staticThemed(sc.colors)
			plane := windowPlane(sc.colors)
			var click widget.Clickable
			btn := launchButton(rx.Of(tok.components), tok.shaper, Apps[0], &click, Status{}, plane)
			size := image.Pt(int(ButtonW)+8, 40)
			img := golden.Capture(t, size, golden.Stack(backdrop.Widget(plane), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return btn(gtx)
			}))
			fill, title, accent := 0, 0, 0
			for y := 0; y < size.Y; y++ {
				for x := 0; x < size.X; x++ {
					switch pixelAt(img, image.Pt(x, y)) {
					case sc.colors.DefaultButtonFill:
						fill++
					case sc.colors.AlternateSelectedControlText:
						title++
					case sc.colors.ControlAccent:
						accent++
					}
				}
			}
			if fill == 0 || title == 0 || accent != 0 {
				t.Errorf("default button fill %d px, its title colour %d px, accent %d px; want fill and title above zero and no accent",
					fill, title, accent)
			}
		})
	}
}
