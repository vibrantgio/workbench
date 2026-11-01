package main

import (
	"image"
	stdcolor "image/color"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// TestTheSchemeSwitchTellsTheTextItsFill flips the scheme switch on a light
// desktop and on a dark one. A layer laid out the way the window lays out
// its layers is told the window background that the switch shows, not the
// desktop's. Text picks its face against that fill: the label and a grey
// half way between the two window backgrounds each pick the face of their
// size and colour on the window background the switch shows.
func TestTheSchemeSwitchTellsTheTextItsFill(t *testing.T) {
	role := tokens.DefaultTypography.SystemFont
	for _, desktop := range []struct {
		name string
		live tokens.PlatformColors
	}{{"light desktop", tokens.PlatformLight}, {"dark desktop", tokens.PlatformDark}} {
		m := judging()
		for _, dark := range []bool{false, true} {
			m = ReduceModel(m, SetScheme{Dark: dark})
			shown := WindowSet(desktop.live, m)
			mid := midGrey(tokens.PlatformLight.WindowBackground, tokens.PlatformDark.WindowBackground)

			var fill stdcolor.NRGBA
			var told bool
			var label, grey font.Typeface
			probe := rx.Of[layout.Widget](func(gtx layout.Context) layout.Dimensions {
				fill, told = typeset.Fill(gtx)
				label = typeset.FontOn(gtx, role, font.Normal, shown.Label).Typeface
				grey = typeset.FontOn(gtx, role, font.Normal, mid).Typeface
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			golden.Capture(t, image.Pt(8, 8), golden.ThemeLayers(t, golden.StaticTheme(desktop.live),
				func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
					return onSchemeBackground(th, rx.Of(m), probe)
				}))

			side := "light switch"
			if dark {
				side = "dark switch"
			}
			if !told {
				t.Fatalf("%s, %s: the layer is told no fill", desktop.name, side)
			}
			if want := opaque(shown.WindowBackground); fill != want {
				t.Errorf("%s, %s: the layer is told %v, want the window background the switch shows %v", desktop.name, side, fill, want)
			}
			bg := opaque(shown.WindowBackground)
			base := typeset.Font(role, font.Normal)
			if want := tokens.StemFont(base, role.Size, shown.Label, bg).Typeface; label != want {
				t.Errorf("%s, %s: the label picks %q, want %q, its face on the window background the switch shows", desktop.name, side, label, want)
			}
			if want := tokens.StemFont(base, role.Size, mid, bg).Typeface; grey != want {
				t.Errorf("%s, %s: the grey picks %q, want %q, its face on the window background the switch shows", desktop.name, side, grey, want)
			}
		}
	}
}

// midGrey is the colour half way between a and b on each channel.
func midGrey(a, b stdcolor.NRGBA) stdcolor.NRGBA {
	return stdcolor.NRGBA{
		R: uint8((int(a.R) + int(b.R)) / 2),
		G: uint8((int(a.G) + int(b.G)) / 2),
		B: uint8((int(a.B) + int(b.B)) / 2),
		A: 0xff,
	}
}

// opaque is c as [typeset.WithFill] stores an opaque fill.
func opaque(c stdcolor.NRGBA) stdcolor.NRGBA {
	c.A = 0xff
	return c
}
