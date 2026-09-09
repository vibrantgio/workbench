package main

import (
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// fixturePlatform is a colour a platform might report — systemBlue, which is
// what macOS paints an application that has chosen none.
var fixturePlatform = stdcolor.NRGBA{R: 0x00, G: 0x7a, B: 0xff, A: 0xff}

// following is the loaded window on a machine whose platform reports a
// colour of its own, which is the state the last cell of the row exists in.
func following() Model {
	m := loaded()
	m.Platform = fixturePlatform
	return m
}

// TestThePlatformsColourStandsBesideTheCandidates: the row's last cell is
// the colour the platform reports, drawn as its own colour like every other
// swatch, so what is offered is the setting in force and not a description
// of it.
func TestThePlatformsColourStandsBesideTheCandidates(t *testing.T) {
	m := following()
	n := len(m.Candidates) + 1
	img := page(t, m, tokens.DefaultLight)
	p := cellCentre(n, n-1)
	got := img.RGBAAt(p.X, p.Y)
	if got.R != fixturePlatform.R || got.G != fixturePlatform.G || got.B != fixturePlatform.B {
		t.Errorf("the last swatch at %v drew %v, want the platform's %v", p, got, fixturePlatform)
	}
}

// TestNoCellWhereThePlatformReportsNoColour: a desktop that publishes no
// colour of its own is offered no choice of it, and the row is the
// candidates alone.
func TestNoCellWhereThePlatformReportsNoColour(t *testing.T) {
	m := loaded()
	if m.Platform.A != 0 {
		t.Fatal("the fixture already carries a platform colour")
	}
	n := len(m.Candidates)
	img := page(t, m, tokens.DefaultLight)
	// The cell that would be there in a row of one more is not drawn, so
	// what stands where its swatch would is the window's own page.
	p := cellCentre(n+1, n)
	got := img.RGBAAt(p.X, p.Y)
	background := tokens.DefaultLight.Background
	if got.R != background.R || got.G != background.G || got.B != background.B {
		t.Errorf("a cell was drawn at %v (%v) on a platform reporting no colour", p, got)
	}
	if golden.PixelDiff(img, page(t, following(), tokens.DefaultLight)) == 0 {
		t.Error("the row looks the same whether or not the platform reports a colour")
	}
}

// TestChoosingThePlatformsColourThemesTheWindowInIt: the cell is a choice
// like any other in the row — the whole window re-derives from it, and the
// ring moves onto it.
func TestChoosingThePlatformsColourThemesTheWindowInIt(t *testing.T) {
	m := ReduceModel(following(), FollowSystem{})
	if !m.Follows {
		t.Fatal("the window did not take the platform's colour")
	}
	seed, ok := m.Seed()
	if !ok || seed != fixturePlatform {
		t.Fatalf("the seed on screen is (%v, %v), want the platform's %v", seed, ok, fixturePlatform)
	}
	wantLight, wantDark := tokens.FromSeed(fixturePlatform)
	gotLight, gotDark := m.Pair(tokens.DefaultLight)
	if gotLight != wantLight || gotDark != wantDark {
		t.Error("the window is not drawn in the pair the platform's colour derives")
	}
	if GalleryHintFor(m, false) != "rendered from "+hexOf(fixturePlatform)+" · syntax base "+m.Base(false) {
		t.Errorf("the page says it is rendered from %q", GalleryHintFor(m, false))
	}

	n := len(m.Candidates) + 1
	edge := func(img *image.RGBA, i int) stdcolor.RGBA {
		return img.RGBAAt(cardX(n, i)+cardW(n)/2, cardTop()+int(Ring)/2)
	}
	img := page(t, m, tokens.DefaultLight)
	if edge(img, n-1) == edge(img, 0) {
		t.Error("the last cell's edge matches a candidate's — nothing marks the choice")
	}
	if edge(img, 0) != edge(img, 1) {
		t.Error("a candidate is marked while the window follows the system")
	}
}

// TestChoosingACandidateStopsFollowing: the two are one choice, so taking a
// colour out of the picture is leaving the platform's behind.
func TestChoosingACandidateStopsFollowing(t *testing.T) {
	m := ReduceModel(following(), FollowSystem{})
	m = ReduceModel(m, SelectCandidate{Index: 1})
	if m.Follows {
		t.Error("a candidate was chosen and the window is still following the system")
	}
	if seed, _ := m.Seed(); seed != fixtureBlue {
		t.Errorf("the seed on screen is %v, want the candidate just clicked %v", seed, fixtureBlue)
	}
}

// TestFollowingMovesWithTheSettingItFollows: the colour is not copied when
// it is chosen — a window that is following is showing whatever the platform
// is set to now, which is what makes it worth keeping.
func TestFollowingMovesWithTheSettingItFollows(t *testing.T) {
	m := ReduceModel(following(), FollowSystem{})
	pink := stdcolor.NRGBA{R: 0xff, G: 0x2d, B: 0x55, A: 0xff}
	m = ReduceModel(m, PlatformColorChanged{Color: pink})
	if seed, _ := m.Seed(); seed != pink {
		t.Errorf("after the setting changed the window shows %v, want %v", seed, pink)
	}
}

// TestKeepingWhileFollowingWritesNoColour: what is written is the standing
// instruction and not a colour, so every application that adopts the brand
// goes on deriving from the platform rather than from the colour it happened
// to be on when the button was pressed.
func TestKeepingWhileFollowingWritesNoColour(t *testing.T) {
	m, path := keeping(t)
	m.Platform = fixturePlatform
	m = ReduceModel(m, FollowSystem{})

	next, cmd := Update(m, KeepSeed{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	kept, ok := msg.(SeedKept)
	if !ok {
		t.Fatalf("keeping reported %T, want SeedKept", msg)
	}
	if !kept.Follows || kept.Seed.A != 0 {
		t.Errorf("keeping reported %+v, want a kept theme that follows the system and names no colour", kept)
	}

	onDisk, found, err := brand.LoadFrom(path)
	if err != nil || !found {
		t.Fatalf("the kept theme read back as (%v, %v), want a brand and no error", found, err)
	}
	if !onDisk.FollowSystem {
		t.Error("the file pins a colour, want one that follows the system")
	}
	if onDisk.Seed.A != 0 {
		t.Errorf("the file holds the colour %v, want none", onDisk.Seed)
	}
	if onDisk.Source != "" {
		t.Errorf("the file credits %q for a colour no picture supplied", onDisk.Source)
	}
	if onDisk.Base.Light != m.Base(false) || onDisk.Base.Dark != m.Base(true) {
		t.Errorf("the syntax bases came back as %+v, want the pair on screen", onDisk.Base)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the file is not there: %v", err)
	}

	next = ReduceModel(next, kept)
	if !next.SeedIsKept() {
		t.Error("after keeping, the window does not know what is on screen is what is on disk")
	}
	// And a candidate chosen after that is a change again, since the file
	// holds no colour to match it.
	if ReduceModel(next, SelectCandidate{Index: 1}).SeedIsKept() {
		t.Error("a candidate chosen after a following keep is reported as already kept")
	}
}

// TestAWindowOpensOnAFollowingBrandFollowing: the file is read at startup
// like any other, and what it says the window says back.
func TestAWindowOpensOnAFollowingBrandFollowing(t *testing.T) {
	m := loaded().adoptKept(brand.Brand{FollowSystem: true, Mono: tokens.CodeFaceJetBrains})
	if !m.KeptFollows {
		t.Fatal("the window did not read the file's instruction to follow the system")
	}
	if m.Kept.A != 0 {
		t.Errorf("the window read a kept colour %v out of a file that holds none", m.Kept)
	}
	m.Platform = fixturePlatform
	if m.SeedIsKept() {
		t.Error("a candidate on screen is reported as kept by a file that pins nothing")
	}
	if !ReduceModel(m, FollowSystem{}).SeedIsKept() {
		t.Error("the window following the system is not reported as the theme on disk")
	}
}

// TestFollowingLeavesThePictureWhereItIs: choosing the platform's colour is
// choosing a colour, not opening a different document — the picture the
// candidates came out of is still named and still on its mat, exactly as it
// is when a candidate beside it is chosen.
func TestFollowingLeavesThePictureWhereItIs(t *testing.T) {
	m := ReduceModel(following(), FollowSystem{})
	if m.Name != loaded().Name {
		t.Errorf("the window names %q after following the system, want the picture %q", m.Name, loaded().Name)
	}
	if m.Preview == nil {
		t.Error("following the system took the picture down with it")
	}
	if golden.PixelDiff(page(t, following(), tokens.DefaultLight), page(t, m, tokens.DefaultLight)) == 0 {
		t.Error("following the system draws the same window as not following it")
	}
}

// TestFollowingDump writes the window with the last cell of the row on
// screen and chosen, in both schemes, for the same reason TestWindowDump
// writes its tabs: looking at a window is a review step and not a test. It
// skips unless -themer.dump names a directory.
func TestFollowingDump(t *testing.T) {
	if *dumpDir == "" {
		t.Skip("themer: pass -themer.dump=DIR to write the window out")
	}
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "harbour.png",
		Preview:    preview(scene(900, 600)),
		Candidates: imageseed.Extract(scene(900, 600)),
	})
	m.Platform = fixturePlatform
	m = pick(pick(m, "github", false), "dracula", true)
	e, sel := newEmbed(), newBaseSelector()
	for _, sc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		on := ReduceModel(m, SetScheme{Dark: sc.dark})
		for _, st := range []struct {
			name string
			m    Model
		}{
			{"offered", on},
			{"following", ReduceModel(on, FollowSystem{})},
		} {
			e.state(st.m.Tab).ScrollToStart()
			path := filepath.Join(*dumpDir, "themer-system-"+st.name+"-"+sc.name+".png")
			if err := golden.Save(path, pageOn(t, e, st.m, tokens.DefaultLight, sel)); err != nil {
				t.Fatalf("themer: save %s: %v", path, err)
			}
			t.Logf("wrote %s", path)
		}
	}
}
