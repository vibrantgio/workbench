package main

import (
	"image"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/theme/tokens"
)

// shellCanvasSize is the whole window at the app's default size. The seam
// these tests are about — the strip's underline against the first row of
// the content — exists only where strip and content meet, so it cannot be
// seen on the content-only canvas the per-tab goldens use.
var shellCanvasSize = image.Pt(windowW, windowH)

// TestStripUnderlineKeepsItsOwnLine is the AG1.1 guard: whatever a tab
// draws, the shell's content slot leaves a band of bare Surface between
// the strip's Primary underline and the content's first row, so the
// underline reads as a line rather than as the top edge of the content.
// The Gallery tab is the case that reported the defect — the inventory's
// first group banner is a full-width Primary fill, the underline's own
// colour — but the slot is shared, so all three tabs are checked.
//
// The expected colour is sampled from the strip itself rather than named
// from the token set: the capture round-trips through the GPU, and a
// sampled reference makes the assertion about "the same colour the strip
// is" instead of about colour-space arithmetic.
func TestStripUnderlineKeepsItsOwnLine(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	source := guideFixture(t)
	first := guideOutline(markdown.Parse(source))[0]
	st := outlineState{open: map[int]bool{0: true}, selected: first.Block}

	// PxPerDp is 1 in headless captures, so dp and px coincide here.
	// Named from the scale rather than read off contentGap: a test that
	// derives its expectation from the value under test would pass at a
	// gap of zero, which is the defect this guards.
	stripH := int(tokens.Comfortable.ControlHeight)
	gap := int(tokens.Spacing.S4)

	schemes := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	}
	for _, sc := range schemes {
		for i, tabName := range []string{"docs", "gallery", "theme"} {
			t.Run(sc.name+"/"+tabName, func(t *testing.T) {
				props := tabs.Props{Tabs: []tabs.Tab{
					{Label: "Docs", Content: contentSlot(renderDocsTab(shaper, source, st, sc.colors, typo))},
					{Label: "Gallery", Content: contentSlot(renderGalleryTab(shaper, sc.colors, typo))},
					{Label: "Theme", Content: contentSlot(renderThemeTab(shaper, sc.colors, typo))},
				}, Shaper: shaper}
				w := tabs.Render(shaper, props, i, sc.colors, tokens.Spacing, typo.LabelLarge, tokens.Comfortable)
				img := golden.Capture(t, shellCanvasSize, w)

				at := func(x, y int) [3]uint8 {
					off := img.PixOffset(x, y)
					return [3]uint8{img.Pix[off], img.Pix[off+1], img.Pix[off+2]}
				}
				// Right of the last tab cell the strip is bare Surface.
				surface := at(shellCanvasSize.X-1, stripH/2)

				// The underline must exist: the selected cell's bottom row
				// carries a colour the surface does not.
				underlined := false
				for x := 0; x < shellCanvasSize.X; x++ {
					if at(x, stripH-1) != surface {
						underlined = true
						break
					}
				}
				if !underlined {
					t.Fatalf("no underline on row %d — the seam test is not looking at the strip", stripH-1)
				}

				// And the gap band below it must be nothing but Surface.
				for y := stripH; y < stripH+gap; y++ {
					for x := 0; x < shellCanvasSize.X; x++ {
						if got := at(x, y); got != surface {
							t.Fatalf("content reaches into the strip gap at (%d,%d): got %v, want the strip's %v",
								x, y, got, surface)
						}
					}
				}
			})
		}
	}
}

// TestContentSlotPushesContentDown is the arithmetic half of the seam,
// free of the GPU: the slot reports the panel's full height and lays its
// child out one gap lower and shorter.
func TestContentSlotPushesContentDown(t *testing.T) {
	panel := image.Pt(400, 300)
	gap := int(contentGap)

	var gotConstraints layout.Constraints
	slot := contentSlot(func(gtx layout.Context) layout.Dimensions {
		gotConstraints = gtx.Constraints
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	dims := drawOnce(t, panel, slot)

	if dims.Size != panel {
		t.Errorf("slot reported %v, want the whole panel %v", dims.Size, panel)
	}
	if want := panel.Y - gap; gotConstraints.Max.Y != want {
		t.Errorf("child got %d px of height, want %d (panel minus the %d dp gap)",
			gotConstraints.Max.Y, want, gap)
	}
	if gotConstraints.Max.X != panel.X {
		t.Errorf("child got %d px of width, want the full %d — the gap is vertical only",
			gotConstraints.Max.X, panel.X)
	}
}
