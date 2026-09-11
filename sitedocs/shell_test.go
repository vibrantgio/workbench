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

// shellFrameSize is the whole window at the app's default size. The seam
// these tests are about — the strip's underline against the first row of
// the content — exists only where strip and content meet, so it cannot be
// seen on the content-only frame the per-tab goldens use.
var shellFrameSize = image.Pt(windowW, windowH)

// TestStripUnderlineKeepsItsOwnLine guards the seam: whatever a tab draws,
// the shell's content slot leaves a band of bare panel fill between the
// strip's underline and the content's first row, so the underline reads as a
// line rather than as the top edge of the content. The slot is shared, so all
// five tabs are checked.
//
// Both reference colours are sampled rather than named from the set: the
// capture round-trips through the GPU, and a sampled reference makes the
// assertions about "the colour the strip's foot is" and "the colour the panel
// is" instead of about colour-space arithmetic. The panel's reference comes
// from a frame rendered with no tab selected at all, so it is bare pattern
// fill and nothing else — a reference taken from the gap band itself would be
// satisfied by any content that filled the band uniformly, which is exactly
// the full-width band this test exists to catch.
//
// The strip's own fill is NOT a reference here. On this platform the chrome
// material the strip carries is the content's own fill exactly in the light
// appearance, so the two regions are one colour there and what parts them is
// the seam along the strip's foot — which is where the underline is read
// against.
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

	sample := func(img *image.RGBA, x, y int) [3]uint8 {
		off := img.PixOffset(x, y)
		return [3]uint8{img.Pix[off], img.Pix[off+1], img.Pix[off+2]}
	}

	schemes := []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	}
	for _, sc := range schemes {
		render := func(selected int) *image.RGBA {
			props := tabs.Props{Tabs: staticTabs(shaper, source, st, sc.colors, typo), Shaper: shaper}
			w := tabs.Render(shaper, props, selected, sc.colors, tokens.Spacing, typo.LabelLarge, tokens.Comfortable)
			return golden.Capture(t, shellFrameSize, w)
		}

		// An out-of-range selection draws no content, so the whole panel is
		// the pattern's own fill and the strip carries no underline: its foot
		// is the seam and nothing else.
		bare := render(-1)
		seam := sample(bare, shellFrameSize.X-1, stripH-1)
		panelFill := sample(bare, shellFrameSize.X-1, stripH+gap/2)

		for i, tabName := range tabPages {
			t.Run(sc.name+"/"+tabName, func(t *testing.T) {
				img := render(i)
				at := func(x, y int) [3]uint8 { return sample(img, x, y) }

				// The underline must exist: somewhere along the selected
				// cell, the strip's bottom row carries a colour the bare
				// seam does not.
				underlined := false
				for x := 0; x < shellFrameSize.X; x++ {
					if at(x, stripH-1) != seam {
						underlined = true
						break
					}
				}
				if !underlined {
					t.Fatalf("no underline on row %d — the seam test is not looking at the strip", stripH-1)
				}

				// And the gap band below it must be nothing but the page.
				for y := stripH; y < stripH+gap; y++ {
					for x := 0; x < shellFrameSize.X; x++ {
						if got := at(x, y); got != panelFill {
							t.Fatalf("content reaches into the strip gap at (%d,%d): got %v, want the panel's %v",
								x, y, got, panelFill)
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
