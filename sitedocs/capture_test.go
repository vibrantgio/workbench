package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// TestWriteReviewCaptures is not a test: it is the offscreen camera for
// review captures. Set SITEDOCS_CAPTURE_DIR to a directory and run this test
// to write window-sized captures of the window — toolbar included, the
// page over the real checkout guide — in both schemes; unset (the normal test
// run) it skips, so no ordinary run depends on the checkout file. It renders
// offscreen and touches nothing on the owner's screen.
func TestWriteReviewCaptures(t *testing.T) {
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
		// windowFrame is the whole composition — the backdrop, the
		// toolbar, and the page on its fill — so the camera
		// photographs the window rather than one layer of it.
		// The second capture is the window with the find bar open and the
		// keyboard in its field, as Edit ▸ Find… leaves it.
		for _, shot := range []struct {
			name string
			find *input.RenderState
		}{
			{"window-review-" + tc.name, nil},
			{"window-review-find-" + tc.name, &input.RenderState{Focused: true}},
			// The third is the window with a query typed: the guide marks
			// every match, the first one current, and the field counts them.
			{"window-review-find-query-" + tc.name, &input.RenderState{Focused: true, Text: "Roboto"}},
		} {
			w := windowFrame(shaper, src, st, tc.c, typo, shot.find)
			img := golden.Capture(t, windowSize, w)
			path := filepath.Join(dir, shot.name+".png")
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
