package main

import (
	"image/color"
	"math"
)

// rgbToHSL converts an NRGBA colour to HSL with every channel in [0, 1)/[0, 1].
// Used to steal the hue of the theme's Primary token and the lightness of its
// Background for the field palette (field.go); alpha is ignored.
func rgbToHSL(c color.NRGBA) (h, s, l float64) {
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2

	d := max - min
	if d == 0 {
		return 0, 0, l // achromatic
	}
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h /= 6
	return h, s, l
}
