package main

import (
	"encoding/json"
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// fixturePlatform is a colour a platform might report — systemBlue, which is
// what macOS paints an application that has chosen none.
var fixturePlatform = stdcolor.NRGBA{R: 0x00, G: 0x7a, B: 0xff, A: 0xff}

// sceneAccent is the vivid patch scene paints, and the colour the extraction
// must lead with: a small share of the frame, and the only vivid thing in it.
var sceneAccent = stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}

// scene paints a picture with no flat regions: a graded sky over graded
// terrain, with one vivid patch. It stands in for a photograph without one
// being stored — a stored photograph is a set of colours nobody can check.
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

// dropped is the model after a picture has landed: a real extraction of a
// painted scene, so the swatches under test are the ones the pipeline
// actually produces.
func dropped(t *testing.T) Model {
	t.Helper()
	img := scene(480, 360)
	candidates := imageseed.Extract(img)
	if len(candidates) < 3 {
		t.Fatalf("the fixture scene yielded %d candidates, want a row to test", len(candidates))
	}
	return Model{
		Preview: preview(img), Name: "scene.png",
		Candidates: candidates, From: FromImage,
		Platform: fixturePlatform,
	}
}

// judging is the model a window is in while somebody is looking at a colour:
// a picture dropped, a colour chosen out of it, and a platform reporting one
// of its own beside it.
func judging() Model { return Model{Platform: fixturePlatform} }

// TestThePlatformsColourIsTheDefault: a window that has been handed nothing
// is on the platform's own accent colour, which is what an application
// wearing no brand paints itself with.
func TestThePlatformsColourIsTheDefault(t *testing.T) {
	m := ReduceModel(Model{}, PlatformColorChanged{Color: fixturePlatform})
	if !m.Follows() {
		t.Fatal("a window with nothing chosen is not following the platform")
	}
	col, ok := m.Color()
	if !ok || col != fixturePlatform {
		t.Errorf("the theme colour is %v (%t), want the platform's %v", col, ok, fixturePlatform)
	}
}

// TestADesktopReportingNoColourOffersNone: a platform that publishes no
// accent has no colour to follow, so nothing is offered and the window
// previews the platform's set as it stands.
func TestADesktopReportingNoColourOffersNone(t *testing.T) {
	m := ReduceModel(Model{}, FollowSystem{})
	if _, ok := m.Color(); ok {
		t.Error("a colour was offered on a desktop reporting none")
	}
	if got := len(rowCells(m)); got != 0 {
		t.Errorf("the row drew %d cells with no colour to draw, want none", got)
	}
}

// TestAPictureOffersItsColours: a drop replaces the row with the colours the
// picture is made of, and lands on the leading one.
func TestAPictureOffersItsColours(t *testing.T) {
	m := dropped(t)
	if m.From != FromImage || m.Selected != 0 {
		t.Fatalf("a drop left the window on %v at %d, want the picture's leading colour", m.From, m.Selected)
	}
	col, ok := m.Color()
	if !ok || col != m.Candidates[0].Color {
		t.Errorf("the theme colour is %v, want the leading candidate %v", col, m.Candidates[0].Color)
	}
	if got := len(rowCells(m)); got != len(m.Candidates)+1 {
		t.Errorf("the row drew %d cells for %d colours and a platform colour", got, len(m.Candidates))
	}
	if cells := rowCells(m); cells[systemSlot].col != fixturePlatform {
		t.Errorf("the leading cell is %v, want the platform's own %v", cells[systemSlot].col, fixturePlatform)
	}
}

// TestASwatchChoosesItsColour: clicking one of a picture's swatches makes
// that colour the theme colour and stops the window following the platform.
func TestASwatchChoosesItsColour(t *testing.T) {
	m := ReduceModel(dropped(t), SelectCandidate{Index: 2})
	if m.Follows() {
		t.Error("choosing a colour out of a picture left the window following the platform")
	}
	col, _ := m.Color()
	if want := m.Candidates[2].Color; col != want {
		t.Errorf("the theme colour is %v, want the chosen swatch's %v", col, want)
	}
}

// TestAnIndexOutsideTheRowChangesNothing: the row is drawn from the same list
// the reducer clamps against, so an index outside it is a message nothing on
// screen could have sent.
func TestAnIndexOutsideTheRowChangesNothing(t *testing.T) {
	m := ReduceModel(dropped(t), SelectCandidate{Index: 1})
	for _, idx := range []int{-1, len(m.Candidates), len(m.Candidates) + 7} {
		if got := ReduceModel(m, SelectCandidate{Index: idx}).Selected; got != 1 {
			t.Errorf("index %d moved the choice to %d, want it left on 1", idx, got)
		}
	}
}

// TestAColourWrittenInIsTaken: six hex digits, with or without the mark, in
// either case, become the theme colour.
func TestAColourWrittenInIsTaken(t *testing.T) {
	want := stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}
	for _, typed := range []string{"#e8112d", "E8112D", " #E8112d "} {
		m := ReduceModel(judging(), HexTyped{Text: typed})
		col, ok := m.Color()
		if !ok || col != want {
			t.Errorf("%q became %v (%t), want %v", typed, col, ok, want)
		}
		if m.From != FromHex {
			t.Errorf("%q left the window on %v, want the written colour", typed, m.From)
		}
	}
}

// TestAFieldHalfTypedLeavesTheColourAlone: a field is empty, then one
// character long, then five, on the way to being six, and re-theming the
// preview at each step would show colours nobody asked for.
func TestAFieldHalfTypedLeavesTheColourAlone(t *testing.T) {
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	for _, typed := range []string{"", "#", "#e81", "#e8112", "#gggggg"} {
		got := ReduceModel(m, HexTyped{Text: typed})
		col, _ := got.Color()
		if want, _ := m.Color(); col != want {
			t.Errorf("%q moved the theme colour to %v, want it left on %v", typed, col, want)
		}
		if got.Text != typed {
			t.Errorf("the field holds %q, want what was written, %q", got.Text, typed)
		}
	}
}

// TestADropReplacesAWrittenColour: the three ways of choosing are one choice,
// so the last act wins and the window says where the colour came from.
func TestADropReplacesAWrittenColour(t *testing.T) {
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	m = ReduceModel(m, ImageLoaded{Path: "/tmp/scene.png", Candidates: dropped(t).Candidates})
	if m.From != FromImage {
		t.Fatalf("a drop over a written colour left the window on %v", m.From)
	}
	if got := IdentityHint(m); got != "from scene.png" {
		t.Errorf("the caption says %q, want the picture it came out of", got)
	}
}

// TestAFailedDropKeepsWhatIsOnScreen: a stray file dragged onto the window
// takes nothing away from the colours already there.
func TestAFailedDropKeepsWhatIsOnScreen(t *testing.T) {
	m := dropped(t)
	got := ReduceModel(m, ImageRejected{Path: "/tmp/notes.txt", Reason: "notes.txt is not an image this reads"})
	if len(got.Candidates) != len(m.Candidates) {
		t.Errorf("a failed drop left %d colours, want the %d already there", len(got.Candidates), len(m.Candidates))
	}
	if got.Problem == "" {
		t.Error("a failed drop said nothing about why")
	}
}

// TestTheSchemeSwitchMovesThePreviewAlone: the window wears the theme the
// desktop is set to, like every other application; what the switch moves is
// which side of the platform's pair is previewed.
func TestTheSchemeSwitchMovesThePreviewAlone(t *testing.T) {
	m := ReduceModel(judging(), SetScheme{Dark: true})
	if !m.Dark(tokens.PlatformLight) {
		t.Error("the switch was pressed for dark and the preview stayed light")
	}
	if m.Dark(tokens.PlatformDark) != true {
		t.Error("the preview is not dark on a dark desktop after the switch")
	}
	// The window's own colours are the live set's, whatever the switch says.
	p := PaletteFrom(tokens.PlatformLight)
	if p.Backdrop != tokens.PlatformLight.WindowBackground {
		t.Errorf("the window's plane is %v, want the platform's %v", p.Backdrop, tokens.PlatformLight.WindowBackground)
	}
}

// TestThePreviewStandsTheColourInForTheAccent: what is previewed is the
// platform's set with the theme colour where the platform uses its accent,
// and nothing else moved.
func TestThePreviewStandsTheColourInForTheAccent(t *testing.T) {
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	got := PreviewSet(tokens.PlatformLight, m, false)
	if want := (stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}); got.ControlAccent != want {
		t.Errorf("the previewed accent is %v, want the theme colour %v", got.ControlAccent, want)
	}
	base := tokens.PlatformLight
	for _, row := range []struct {
		name      string
		got, want stdcolor.NRGBA
	}{
		{"the window's plane", got.WindowBackground, base.WindowBackground},
		{"the content's fill", got.ControlBackground, base.ControlBackground},
		{"the chrome material", got.SidebarMaterial, base.SidebarMaterial},
		{"a seam", got.Separator, base.Separator},
		{"the label", got.Label, base.Label},
		{"the error colour", got.SystemRed, base.SystemRed},
		{"a link", got.Link, base.Link},
	} {
		if row.got != row.want {
			t.Errorf("%s moved to %v when the theme colour was chosen, want %v", row.name, row.got, row.want)
		}
	}
	if got.SelectedContentBackground == base.SelectedContentBackground {
		t.Error("the selection did not follow the theme colour")
	}
}

// TestThePreviewedSideIsTheRecordedOneWhenItIsNotTheDesktops: the live reader
// answers for the appearance the desktop is on and no other, so the other
// side of the preview is the recorded set.
func TestThePreviewedSideIsTheRecordedOneWhenItIsNotTheDesktops(t *testing.T) {
	m := Model{}
	if got := PreviewSet(tokens.PlatformLight, m, true); got.WindowBackground != tokens.PlatformDark.WindowBackground {
		t.Errorf("the dark preview on a light desktop is %v, want the recorded dark set's %v",
			got.WindowBackground, tokens.PlatformDark.WindowBackground)
	}
	if got := PreviewSet(tokens.PlatformDark, m, false); got.WindowBackground != tokens.PlatformLight.WindowBackground {
		t.Errorf("the light preview on a dark desktop is %v, want the recorded light set's %v",
			got.WindowBackground, tokens.PlatformLight.WindowBackground)
	}
}

// TestTheWindowsColoursAreThePlatformsNames: every colour this window draws
// with is the platform's name for what it draws, flattened onto the fill it
// lands on. Nothing is derived and nothing is a literal.
func TestTheWindowsColoursAreThePlatformsNames(t *testing.T) {
	for _, c := range []tokens.PlatformColors{tokens.PlatformLight, tokens.PlatformDark} {
		p := PaletteFrom(c)
		for _, row := range []struct {
			name      string
			got, want stdcolor.NRGBA
		}{
			{"the plane", p.Backdrop, c.WindowBackground},
			{"a card", p.Surface, c.CardFill},
			{"the accent", p.Accent, c.ControlAccent},
			{"the selection", p.Selection, c.SelectedContentBackground},
			{"what reads on it", p.OnAccent, c.AlternateSelectedControlText},
			{"a problem", p.Problem, c.SystemRed},
		} {
			if row.got != row.want {
				t.Errorf("%s is %v, want %v", row.name, row.got, row.want)
			}
		}
		for _, row := range []struct {
			name string
			got  stdcolor.NRGBA
		}{
			{"a seam", p.Seam}, {"a card's edge", p.Edge},
			{"the text", p.Text}, {"the muted step", p.Muted},
			{"the text on a card", p.CardText}, {"the muted step on a card", p.CardMuted},
			{"a card under the pointer", p.Hover},
		} {
			if row.got.A != 0xff {
				t.Errorf("%s carries a coverage (%v) — every alpha name is flattened before it reaches the rasterizer", row.name, row.got)
			}
		}
	}
}

// TestKeepWritesTheColour: the file holds the colour that was chosen, spelled
// the way theme/export spells it, and comes back as that colour.
func TestKeepWritesTheColour(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	want := stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}
	msg := keepTheme(path, want, false, "scene.png")
	kept, ok := msg.(SeedKept)
	if !ok {
		t.Fatalf("keeping answered %#v, want the colour kept", msg)
	}
	if kept.Seed != want || kept.Follows {
		t.Errorf("keeping answered %v (follows %t), want %v", kept.Seed, kept.Follows, want)
	}
	back := brand.KeptFrom(path)
	if back.Seed != want {
		t.Errorf("the file came back as %v, want %v", back.Seed, want)
	}
	if back.Source != "scene.png" {
		t.Errorf("the file credits %q, want the picture it came out of", back.Source)
	}
}

// TestKeepingTheSystemsColourWritesNoColour: a theme that follows the system
// records the instruction and no colour, because the colour is the platform's
// and changes after the file is written.
func TestKeepingTheSystemsColourWritesNoColour(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if _, ok := keepTheme(path, stdcolor.NRGBA{}, true, "").(SeedKept); !ok {
		t.Fatal("keeping the system's colour failed")
	}
	back := brand.KeptFrom(path)
	if !back.FollowSystem || back.Chosen() {
		t.Errorf("the file holds follows=%t seed=%v, want the instruction and no colour", back.FollowSystem, back.Seed)
	}
	var raw map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["seed"]; ok {
		t.Error("the file carries a seed key, which a reader written before the flag would pin")
	}
}

// TestKeepingTheColourLeavesEverythingElseInTheFile: the kept file is shared
// by every application that adopts a brand and holds choices this window does
// not make. Choosing a theme colour here must not take one away.
func TestKeepingTheColourLeavesEverythingElseInTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	before := brand.Brand{
		Seed: stdcolor.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff},
		Base: brand.BasePair{Light: "catppuccin-latte", Dark: "catppuccin-mocha"},
		Mono: tokens.CodeFaceJetBrains,
	}
	if err := brand.SaveTo(path, before); err != nil {
		t.Fatal(err)
	}
	want := stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}
	if _, ok := keepTheme(path, want, false, "").(SeedKept); !ok {
		t.Fatal("keeping the colour failed")
	}
	back := brand.KeptFrom(path)
	if back.Seed != want {
		t.Errorf("the colour is %v, want %v", back.Seed, want)
	}
	if back.Base != before.Base {
		t.Errorf("the syntax bases came back as %v, want the %v that were there", back.Base, before.Base)
	}
	if back.Mono != before.Mono {
		t.Errorf("the code face came back as %q, want the %q that was there", back.Mono, before.Mono)
	}
}

// TestKeepingSaysWhereItCouldNot: a machine with no config directory is not a
// reason to refuse to start, so the one thing the window cannot do says so
// when it is asked.
func TestKeepingSaysWhereItCouldNot(t *testing.T) {
	msg := keepTheme("", fixturePlatform, false, "")
	failed, ok := msg.(KeepFailed)
	if !ok {
		t.Fatalf("keeping with nowhere to write answered %#v, want a refusal", msg)
	}
	if failed.Reason == "" {
		t.Error("the refusal gave no reason")
	}
}

// TestTheAffordanceConfirmsOnlyWhatIsOnDisk: an offer while the colour on
// screen is not the one in the file, a confirmation the moment it is.
func TestTheAffordanceConfirmsOnlyWhatIsOnDisk(t *testing.T) {
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	if m.IsKept() {
		t.Error("a colour nothing has written confirms as kept")
	}
	col, _ := m.Color()
	m = ReduceModel(m, SeedKept{Seed: col})
	if !m.IsKept() {
		t.Error("the colour just written does not confirm as kept")
	}
	m = ReduceModel(m, FollowSystem{})
	if m.IsKept() {
		t.Error("following the system confirms against a file holding a colour")
	}
	m = ReduceModel(m, SeedKept{Follows: true})
	if !m.IsKept() {
		t.Error("following the system does not confirm against a file that follows too")
	}
}

// TestDragOverIsTheDropZonesState: the ring round the window is drawn on
// this, and a drop that lands must take it away.
func TestDragOverIsTheDropZonesState(t *testing.T) {
	m := ReduceModel(Model{}, desktop.FilesEntered{})
	if !m.DragOver {
		t.Fatal("a drag over the window did not raise the highlight")
	}
	if ReduceModel(m, desktop.FilesExited{}).DragOver {
		t.Error("a drag leaving the window left the highlight up")
	}
	after, _ := Update(m, desktop.FilesDropped{Paths: []string{"/tmp/scene.png"}})
	if after.DragOver {
		t.Error("a drop left the highlight up")
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
