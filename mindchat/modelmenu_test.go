package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
)

// The header anchor names the model in effect and nothing else: the default-ness
// of that model is the menu's story, not the anchor's.
func TestAnchorLabelNamesTheModelOnly(t *testing.T) {
	base := Model{
		Providers: []Provider{
			{Name: "OpenAI", Models: []string{"gpt-5.5", "gpt-5.5-codex"}},
			{Name: "xAI", Models: []string{"grok-4"}},
		},
		DefaultProvider: "OpenAI",
		DefaultModel:    "gpt-5.5",
	}

	fromDefault := anchorKeyOf(base)
	if fromDefault.title != "OpenAI · gpt-5.5" {
		t.Fatalf("default title = %q, want %q", fromDefault.title, "OpenAI · gpt-5.5")
	}

	override := base
	override.CurrentChat.Provider, override.CurrentChat.Model = "xAI", "grok-4"
	if got := anchorKeyOf(override).title; got != "xAI · grok-4" {
		t.Fatalf("override title = %q, want %q", got, "xAI · grok-4")
	}

	// Picking the model that already was the default says the same thing, so
	// it is the same key and the anchor keeps its subscription.
	sameAsDefault := base
	sameAsDefault.CurrentChat.Provider, sameAsDefault.CurrentChat.Model = "OpenAI", "gpt-5.5"
	if anchorKeyOf(sameAsDefault) != fromDefault {
		t.Fatalf("explicit pick of the default = %+v, want %+v", anchorKeyOf(sameAsDefault), fromDefault)
	}

	// Opening the menu does not part two keys. The anchor's mark is the
	// component's pop-up pair and it does not move — a pair of chevrons
	// pointing opposite ways says the control holds one of several values and
	// cannot say "this is open" — so an open menu draws the identical anchor
	// and rebuilding its subscription for one would be rebuilding for a frame
	// that looks the same.
	open := base
	open.ModelMenu = true
	if anchorKeyOf(open) != fromDefault {
		t.Fatalf("open menu key = %+v, want the closed menu's %+v: the anchor's mark does not move when the menu stands",
			anchorKeyOf(open), fromDefault)
	}
	if got := anchorKeyOf(Model{}).title; got != "No model configured" {
		t.Fatalf("empty title = %q, want %q", got, "No model configured")
	}
}

// TestTheModelMenuOpenAtTheTrailingEdgeLiesInsideTheWindow lays the window out
// at its own size on a wider capture painted a colour the window never uses,
// with the model menu open. The menu's anchor stands at the toolbar's
// trailing edge, so a surface centred on it would run past the window's
// trailing edge; the window reports its available room, and the surface is
// shifted back inside it. No pixel of the surface's fill may land on the
// capture beyond the window.
func TestTheModelMenuOpenAtTheTrailingEdgeLiesInsideTheWindow(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	beyond := color.NRGBA{R: 0xff, G: 0x00, B: 0xff, A: 0xff}
	wide := windowSize.Add(image.Pt(300, 0))
	onWide := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, beyond, clip.Rect{Max: wide}.Op())
			wgtx := gtx
			wgtx.Constraints = layout.Exact(windowSize)
			w(wgtx)
			return layout.Dimensions{Size: wide}
		}
	}

	open := demoModel()
	open.ModelMenu = true
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			closedImg := golden.Capture(t, wide, onWide(frame(t, tc.c, demoModel())))
			openImg := golden.Capture(t, wide, onWide(frame(t, tc.c, open)))
			if golden.PixelDiff(closedImg, openImg) == 0 {
				t.Fatal("opening the model menu changed nothing: the menu did not open")
			}
			fill := tc.c.WindowBackground
			for y := 0; y < wide.Y; y++ {
				for x := windowSize.X; x < wide.X; x++ {
					r, g, b, _ := openImg.At(x, y).RGBA()
					if uint8(r>>8) == fill.R && uint8(g>>8) == fill.G && uint8(b>>8) == fill.B {
						t.Fatalf("the menu's surface reaches x=%d on row %d, past the %d-wide window", x, y, windowSize.X)
					}
				}
			}
		})
	}
}
