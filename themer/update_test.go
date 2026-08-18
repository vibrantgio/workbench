package main

import (
	"image"
	stdcolor "image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

func candidate(c stdcolor.NRGBA, share float64) imageseed.Candidate {
	return imageseed.Candidate{Color: c, Share: share}
}

var (
	fixtureRed  = stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}
	fixtureBlue = stdcolor.NRGBA{R: 0x00, G: 0x50, B: 0xd0, A: 0xff}
	fixtureGrey = stdcolor.NRGBA{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff}
)

func loaded() Model {
	return Model{
		Preview:    image.NewNRGBA(image.Rect(0, 0, 4, 3)),
		Name:       "scene.png",
		Candidates: []imageseed.Candidate{candidate(fixtureRed, 0.2), candidate(fixtureBlue, 0.1), candidate(fixtureGrey, 0.6)},
	}
}

// TestDragHighlightFollowsTheHover: the highlight is model state, so the
// window can draw it without asking the drop machinery anything.
func TestDragHighlightFollowsTheHover(t *testing.T) {
	m := ReduceModel(Model{}, desktop.FilesEntered{Zone: dropZone})
	if !m.DragOver {
		t.Error("a drag entering the window did not raise the highlight")
	}
	if ReduceModel(m, desktop.FilesExited{Zone: dropZone}).DragOver {
		t.Error("a drag leaving the window did not clear the highlight")
	}
	dropped, _ := Update(m, desktop.FilesDropped{Zone: dropZone, Paths: []string{"/tmp/x.png"}})
	if dropped.DragOver {
		t.Error("the drop itself did not clear the highlight")
	}
}

// TestLoadedImageLeadsWithItsFirstCandidate: a new picture replaces the row
// and resets the choice, so what the window shows itself in is the leading
// candidate of the picture just dropped rather than a stale index.
func TestLoadedImageLeadsWithItsFirstCandidate(t *testing.T) {
	prev := loaded()
	prev.Selected = 2
	next := ReduceModel(prev, ImageLoaded{
		Path:       "/pictures/beach.jpg",
		Preview:    image.NewNRGBA(image.Rect(0, 0, 2, 2)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5)},
	})
	if next.Selected != 0 {
		t.Errorf("Selected = %d after a new picture, want 0", next.Selected)
	}
	if next.Name != "beach.jpg" {
		t.Errorf("Name = %q, want the file's own name", next.Name)
	}
	if len(next.Candidates) != 1 || next.Candidates[0].Color != fixtureBlue {
		t.Errorf("candidates = %v, want the new picture's", next.Candidates)
	}
	if next.Problem != "" {
		t.Errorf("Problem = %q after a successful load, want empty", next.Problem)
	}
}

// TestRejectedDropKeepsWhatIsOnScreen: a file that is not a picture reports
// itself and changes nothing else. Clearing the row would punish the user
// for a slip of the wrist.
func TestRejectedDropKeepsWhatIsOnScreen(t *testing.T) {
	prev := loaded()
	prev.Selected = 1
	next := ReduceModel(prev, ImageRejected{Path: "/tmp/notes.txt", Reason: "notes.txt is not an image"})
	if next.Name != prev.Name || len(next.Candidates) != len(prev.Candidates) || next.Selected != 1 {
		t.Errorf("a rejected drop disturbed the loaded picture: %+v", next)
	}
	if next.Problem == "" {
		t.Error("a rejected drop reported nothing")
	}
}

// TestEmptyExtractionIsReportedNotShown: a picture that decodes but yields
// no colour is a rejection, not an empty row.
func TestEmptyExtractionIsReportedNotShown(t *testing.T) {
	prev := loaded()
	next := ReduceModel(prev, ImageLoaded{Path: "/tmp/clear.png"})
	if len(next.Candidates) != len(prev.Candidates) {
		t.Errorf("an empty extraction replaced the candidate row with %v", next.Candidates)
	}
	if next.Problem == "" {
		t.Error("an empty extraction reported nothing")
	}
}

// TestSelectionIsBoundsChecked: an index outside the row leaves the choice
// alone rather than pointing the theme at nothing.
func TestSelectionIsBoundsChecked(t *testing.T) {
	m := loaded()
	if got := ReduceModel(m, SelectCandidate{Index: 2}).Selected; got != 2 {
		t.Errorf("Selected = %d, want 2", got)
	}
	for _, bad := range []int{-1, 3, 99} {
		if got := ReduceModel(m, SelectCandidate{Index: bad}).Selected; got != m.Selected {
			t.Errorf("SelectCandidate{%d} moved the selection to %d", bad, got)
		}
	}
}

// TestSelectionFeedsTheTheme is the wiring the row exists for: the palette
// the window draws in is the one the chosen candidate generates, on the side
// the OS is showing, and it changes when the choice does.
func TestSelectionFeedsTheTheme(t *testing.T) {
	m := loaded()
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		first := SchemeFor(os, m)
		wantLight, wantDark := tokens.FromSeed(fixtureRed)
		want := wantLight
		if isDark(os) {
			want = wantDark
		}
		if first.Primary != want.Primary {
			t.Errorf("primary = %v, want the leading candidate's %v", first.Primary, want.Primary)
		}
		if isDark(first) != isDark(os) {
			t.Error("the generated scheme changed side from the OS's")
		}
		second := SchemeFor(os, ReduceModel(m, SelectCandidate{Index: 1}))
		if second.Primary == first.Primary {
			t.Error("choosing another candidate did not move the primary")
		}
	}
}

// TestNoCandidatesKeepsTheOSPalette: before anything is dropped the window
// is whatever the OS says it is, untouched.
func TestNoCandidatesKeepsTheOSPalette(t *testing.T) {
	if got := SchemeFor(tokens.DefaultLight, Model{}); got.Primary != tokens.DefaultLight.Primary {
		t.Errorf("primary = %v with no candidates, want the OS palette's %v", got.Primary, tokens.DefaultLight.Primary)
	}
}

// TestLoadImageFromDisk walks the whole drop path on a file: decode,
// preview, extract. The fixture is painted here, so the colour the leading
// candidate must come back as is one the test named.
func TestLoadImageFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.png")
	writePNG(t, path, scene(240, 180))

	msg := loadImage(path)
	got, ok := msg.(ImageLoaded)
	if !ok {
		t.Fatalf("loading a PNG gave %#v, want ImageLoaded", msg)
	}
	if got.Preview == nil {
		t.Error("no preview came back with the candidates")
	}
	if len(got.Candidates) == 0 {
		t.Fatal("no candidates came back")
	}
	if got.Candidates[0].Chroma < 0.05 {
		t.Errorf("the row leads with %v at chroma %.3f — a picture full of colour led with a near-neutral",
			got.Candidates[0].Color, got.Candidates[0].Chroma)
	}
	found := false
	for _, c := range got.Candidates {
		if c.Color == sceneAccent {
			found = true
		}
	}
	if !found {
		// The patch covers under two percent of the frame, so it is not
		// expected to lead the graded sky above it — but a row that loses
		// the one vivid thing in the picture is a row nobody can pick from.
		t.Errorf("the picture's vivid patch %v is missing from the row %v", sceneAccent, got.Candidates)
	}
}

// TestLoadImageRefusesWhatItCannotRead: neither a missing file nor a file
// that is not a picture may fail the command — both come back as messages.
func TestLoadImageRefusesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	text := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(text, []byte("not a picture"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "absent.png"), text} {
		if _, ok := loadImage(path).(ImageRejected); !ok {
			t.Errorf("loading %q did not come back as ImageRejected", filepath.Base(path))
		}
	}
}

// TestPreviewIsBounded: a picture larger than the display budget comes back
// shrunk, keeping its shape; a small one is left alone.
func TestPreviewIsBounded(t *testing.T) {
	big := preview(image.NewNRGBA(image.Rect(0, 0, 4000, 2000)))
	if got := big.Bounds().Size(); got.X != previewMax || got.Y != previewMax/2 {
		t.Errorf("preview of a 4000x2000 picture is %v, want %d wide at half the height", got, previewMax)
	}
	small := preview(image.NewNRGBA(image.Rect(0, 0, 40, 30)))
	if got := small.Bounds().Size(); got != image.Pt(40, 30) {
		t.Errorf("preview of a small picture is %v, want it left alone", got)
	}
}

// sceneAccent is the vivid patch scene paints, and the colour the extraction
// must lead with: a small share of the frame, and the only vivid thing in it.
var sceneAccent = stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}

// scene paints a picture with no flat regions: a graded sky over a graded
// ground, with one vivid patch. It stands in for a photograph without one
// being stored — a stored photograph is a palette nobody can check.
func scene(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		v := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			u := float64(x) / float64(w)
			var c stdcolor.NRGBA
			if v < 0.58 {
				c = stdcolor.NRGBA{R: uint8(90 + 70*v + 25*u), G: uint8(150 + 60*v), B: uint8(210 - 35*v), A: 0xff}
			} else {
				c = stdcolor.NRGBA{R: uint8(150 - 25*v), G: uint8(120 + 45*u), B: uint8(75 + 25*v), A: 0xff}
			}
			if x > w*66/100 && x < w*78/100 && y > h*28/100 && y < h*44/100 {
				c = sceneAccent
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
