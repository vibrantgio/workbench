package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/theme/tokens"
)

// TestWriteTabReviewCaptures is not a test of anything: it is the
// offscreen camera for the fresh-eyes review the plan's standing rules
// require. Set SITEDOCS_CAPTURE_DIR to a directory and run this test to
// write window-sized captures of all five tabs — strip included, the
// Docs tab over the real checkout guide — in both schemes; unset (the
// normal test run) it skips, so no ordinary run depends on the checkout
// file. It renders offscreen and touches nothing on the owner's screen.
func TestWriteTabReviewCaptures(t *testing.T) {
	dir := os.Getenv("SITEDOCS_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set SITEDOCS_CAPTURE_DIR to write review captures")
	}
	src, err := os.ReadFile("../llms.txt")
	if err != nil {
		t.Fatalf("review captures need the checkout guide: %v", err)
	}
	shaper := tokens.DefaultTypography.DeterministicShaper()
	typo := tokens.DefaultTypography
	st := outlineState{open: map[int]bool{0: true}, selected: guideOutline(markdown.Parse(src))[0].Block}
	size := image.Pt(1200, 800)
	schemes := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range schemes {
		// staticTabs puts every cell's content in the shell's own
		// contentSlot: the camera has to photograph the same gap under the
		// strip that the app draws.
		props := tabs.Props{Tabs: staticTabs(shaper, src, st, tc.colors, typo), Shaper: shaper}
		for i, tabName := range tabPages {
			w := tabs.Render(shaper, props, i, tc.colors, tokens.Spacing, typo.LabelLarge, tokens.Comfortable)
			img := golden.Capture(t, size, scene(w, tc.bg))
			path := filepath.Join(dir, tabName+"-tab-review-"+tc.name+".png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", path)
		}
	}
}
