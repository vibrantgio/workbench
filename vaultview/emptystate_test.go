package main

import (
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// scanningModel is the window on a vault it is still reading, with nothing
// loaded to show yet.
func scanningModel() Model {
	return Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1, Scanning: true}
}

// emptyVaultModel is the window on a vault that holds no notes at all: the
// reading is over and there is nothing to open.
func emptyVaultModel() Model {
	return Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1, Index: treeIndex()}
}

// TestEmptyStateStandsInTheColumnsFullFrame holds the two states a vault
// column stands empty in against the platform's placement: the line is
// drawn by the shell pattern in the column's FULL frame — the main slot
// grown upward by the toolbar's share — so its centre stands on that
// frame's centre and not on the centre of the row below the toolbar.
//
// The glyph box is the painted glyphs' own and the line box is what the
// pattern centres, so the two centres differ by what the descenders reach
// below the box's middle: a pixel across and two down at this size. A line placed any other
// way in this column is out by tens of pixels at least.
func TestEmptyStateStandsInTheColumnsFullFrame(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	states := []struct {
		name  string
		model Model
	}{
		{"scanning", scanningModel()},
		{"empty", emptyVaultModel()},
	}
	for _, st := range states {
		for _, tc := range themeCases {
			t.Run(st.name+"-"+tc.name, func(t *testing.T) {
				fill := tc.colors.TextBackground
				img := golden.Capture(t, noteFrameSize,
					renderNotePage(shaper, st.model, tc.colors, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable))
				box := noteGlyphBox(img, fill)
				if box.Empty() {
					t.Fatal("the column is bare, want the line the empty state draws")
				}
				// The frame reaches a toolbar's share above the slot, so its
				// centre stands that much above the slot's own.
				bar := int(shell.ToolbarDepth)
				wantX, wantY := noteFrameW, noteFrameH-bar
				if got := box.Min.X + box.Max.X; abs(got-wantX) > 2 {
					t.Errorf("the line's glyphs centre at %.1f across, want the column's %.1f",
						float64(got)/2, float64(wantX)/2)
				}
				if got := box.Min.Y + box.Max.Y; abs(got-wantY) > 4 {
					t.Errorf("the line's glyphs centre at %.1f down, want the full frame's %.1f",
						float64(got)/2, float64(wantY)/2)
				}
			})
		}
	}
}

// TestEmptyStateStandsOnNoMeasure holds the other half of the ruling: the
// line is not content, so it does not begin where the column's text begins.
// Content stands on the measure's lead with everything else in the column;
// this stands in the middle of the column, which is a long way past it.
func TestEmptyStateStandsOnNoMeasure(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	tok := goldenTokens()
	tok.shaper = shaper
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(noteFrameSize),
		Ops:         new(op.Ops),
	}
	lead, _ := noteTextBox(gtx, tok, noteFrameW)

	img := golden.Capture(t, noteFrameSize,
		renderNotePage(shaper, emptyVaultModel(), tokens.PlatformLight, tokens.Spacing,
			tokens.DefaultTypography, tokens.Comfortable))
	box := noteGlyphBox(img, tokens.PlatformLight.TextBackground)
	if box.Empty() {
		t.Fatal("the column is bare, want the line the empty state draws")
	}
	if box.Min.X <= lead {
		t.Errorf("the line begins at x=%d, at or before the measure's lead %d; the empty state stands on no measure",
			box.Min.X, lead)
	}
}

// noteGlyphBox is the box the painted glyphs occupy in a rendering of the note
// column: every pixel the column's own fill does not account for.
func noteGlyphBox(img *image.RGBA, fill color.NRGBA) image.Rectangle {
	b := img.Bounds()
	box := image.Rectangle{}
	first := true
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R == fill.R && c.G == fill.G && c.B == fill.B {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// TestAVaultThatCannotBeReadSaysSoInItsOwnWords drives a real scan at a
// path no vault stands at and holds both halves of the state: the
// column's line is the application's own sentence, read off
// emptyStateLine rather than off the picture, since no picture can be
// asked what its glyphs spell; and the system's own text, which the
// line does not carry, reaches the log with the vault's path.
func TestAVaultThatCannotBeReadSaysSoInItsOwnWords(t *testing.T) {
	vault := filepath.Join(t.TempDir(), "no-vault-here")

	logged := captureStderr(t)
	model := Model{Screen: screenVault, Vault: vault, Scanning: true, CurAnchor: -1}
	gen, stop := model.nextScan()
	model = foldScan(t, model, scanCmd(vault, "", gen, stop, func(err string) mvu.Message {
		return vaultScanned{vault: vault, gen: gen, err: err}
	}))
	written := logged()

	if model.ScanErr == "" {
		t.Fatal("scanning a path with no vault at it reported no error")
	}
	const want = "This vault can't be read."
	got := emptyStateLine(model)
	if got != want {
		t.Errorf("the column says %q, want %q", got, want)
	}
	if strings.Contains(got, model.ScanErr) {
		t.Errorf("the column says %q, which stands the system's text %q in the empty state", got, model.ScanErr)
	}
	if !strings.Contains(written, model.ScanErr) {
		t.Errorf("the log says %q, want the system's text %q in it", written, model.ScanErr)
	}
	if !strings.Contains(written, vault) {
		t.Errorf("the log says %q, want the vault's path %q in it", written, vault)
	}
}

// captureStderr diverts the stream the application writes its diagnostics
// to and answers the reader that restores it and returns what was written.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	read := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		read <- b.String()
	}()
	return func() string {
		os.Stderr = saved
		w.Close()
		s := <-read
		r.Close()
		return s
	}
}
