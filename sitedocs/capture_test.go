package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// TestWriteTabReviewCaptures is not a test: it is the offscreen camera for
// review captures. Set SITEDOCS_CAPTURE_DIR to a directory and run this test
// to write window-sized captures of all five tabs — strip included, the Docs
// tab over the real checkout guide — in both schemes; unset (the normal test
// run) it skips, so no ordinary run depends on the checkout file. It renders
// offscreen and touches nothing on the owner's screen.
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
	for _, tc := range windowSchemes {
		for i, tabName := range tabPages {
			// windowFrame is the whole composition — the backdrop, the
			// title-bar band, and the shell with every cell's content in
			// the app's own contentSlot — so the camera photographs the
			// window rather than one layer of it.
			w := windowFrame(shaper, src, st, tc.c, typo, i, titleBandDp)
			img := golden.Capture(t, windowSize, w)
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
