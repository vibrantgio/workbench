package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/hero"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// windowCanvasSize is the size the window opens at, and the only size these
// renders are recorded at: a composition is worth a picture at the size
// somebody actually looks at it.
var windowCanvasSize = image.Pt(int(winW), int(winH))

// sharpRadius keeps the renders comparable between machines: anti-aliased
// rounded corners vary slightly between GPU contexts, which is enough to fail
// a pixel-exact diff on a page of cards and buttons. The pattern renders
// upstream pin it the same way.
var sharpRadius = tokens.RadiusScale{}

// staticThemed is one theme emission frozen into the snapshot the view
// consumes, with the pinned shaper — Roboto and nothing the machine happens to
// own — that a stored render has to shape with.
func staticThemed(colors tokens.ColorTokens) themed {
	typ := tokens.DefaultTypography
	return themed{
		components: theme.Theme{
			Color:      rx.Of(colors),
			Typography: rx.Of(typ),
			Density:    rx.Of(tokens.Comfortable),
			Motion:     rx.Of(tokens.Motion),
			Spacing:    rx.Of(tokens.Spacing),
			Radius:     rx.Of(sharpRadius),
			Elevation:  rx.Of(tokens.Elevation),
		},
		color:   colors,
		spacing: tokens.Spacing,
		typ:     typ,
		shaper:  typ.DeterministicShaper(),
	}
}

// page is the window as a single widget: the theme's background fill with the
// hero and the card grid on it.
//
// The animated 3D field the running window floats these on sits between the
// two and is not here. It is driven by the clock and by the window it
// invalidates, so it has no one frame to store; the background it is keyed to
// stands in for it.
func page(tok themed, model Model) layout.Widget {
	props := HeroProps
	props.Shaper = tok.shaper
	// tok.components is static (rx.Of throughout), so First() resolves
	// synchronously.
	heroW, _ := hero.Hero(rx.Of(tok.components), props).First()
	clicks := make([]widget.Clickable, len(Apps))
	back := backdrop.Widget(tok.color.Background)
	content := View(tok, heroW, clicks, model)
	return func(gtx layout.Context) layout.Dimensions {
		back(gtx)
		return content(gtx)
	}
}

// TestWindowGolden records or diffs the window in both colour schemes, with
// every app in the roster and none of them running.
func TestWindowGolden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light-window", tokens.DefaultLight},
		{"dark-window", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden.Render(t, tc.name, windowCanvasSize, page(staticThemed(tc.colors), Model{}))
		})
	}
}

// TestSchemesDiffer confirms the two schemes are two renders rather than one
// drawn twice — the failure a pair of goldens recorded from the same tokens
// would otherwise hide.
func TestSchemesDiffer(t *testing.T) {
	light := golden.Capture(t, windowCanvasSize, page(staticThemed(tokens.DefaultLight), Model{}))
	dark := golden.Capture(t, windowCanvasSize, page(staticThemed(tokens.DefaultDark), Model{}))
	if n := golden.PixelDiff(light, dark); n == 0 {
		t.Error("the light and dark windows render identically")
	}
}
