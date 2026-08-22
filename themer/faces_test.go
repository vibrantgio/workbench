package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// pickMono applies the named code face the way a click on that row does.
func pickMono(m Model, name string) Model {
	return ReduceModel(m, SelectMono{Name: name})
}

// TestThePlateOffersExactlyTwoFaces: Roboto Mono and JetBrains Mono,
// nothing else. A font picker this is not.
func TestThePlateOffersExactlyTwoFaces(t *testing.T) {
	if len(codeFaces) != 2 {
		t.Fatalf("the plate offers %d names, want exactly two", len(codeFaces))
	}
	if codeFaces[0] != tokens.CodeFaceRoboto || codeFaces[1] != tokens.CodeFaceJetBrains {
		t.Errorf("the plate offers %v, want Roboto Mono then JetBrains Mono", codeFaces)
	}
}

// TestChoosingACodeFaceRestylesTheSpecimen: a press restyles the fence on
// the next frame. Both captures are taken with the specimen in view, so
// what moves is the face, not scroll.
func TestChoosingACodeFaceRestylesTheSpecimen(t *testing.T) {
	e := newEmbed()
	m := judging()
	first := atTheCode(t, e, m, tokens.DefaultLight)
	second := atTheCode(t, e, pickMono(m, tokens.CodeFaceJetBrains), tokens.DefaultLight)
	if golden.PixelDiff(first, second) == 0 {
		t.Error("switching the code face changed no pixel of the page — the specimen is not following the choice")
	}
}

// TestChoosingACodeFaceDoesNotRebuildTheInventory: the parsed document
// stays. Only Code and the extra faces change.
func TestChoosingACodeFaceDoesNotRebuildTheInventory(t *testing.T) {
	e := newEmbed()
	m := judging()
	pageOn(t, e, m, tokens.DefaultLight)
	built := e.inv
	if built == nil {
		t.Fatal("the first render built no inventory")
	}
	pageOn(t, e, pickMono(m, tokens.CodeFaceJetBrains), tokens.DefaultLight)
	if e.inv != built {
		t.Error("choosing a code face rebuilt the inventory — the specimen was re-parsed")
	}
}

// TestTheWindowOpensOnTheKeptMono, and on Roboto Mono when the file says
// nothing, says an empty name, or names a face this build does not ship.
func TestTheWindowOpensOnTheKeptMono(t *testing.T) {
	m := withBases()
	for _, tc := range []struct {
		name string
		kept string
		want string
	}{
		{"JetBrains Mono", tokens.CodeFaceJetBrains, tokens.CodeFaceJetBrains},
		{"Roboto Mono written out", tokens.CodeFaceRoboto, tokens.CodeFaceRoboto},
		{"nothing kept", "", tokens.CodeFaceRoboto},
		{"a name nobody ships", "Comic Sans", tokens.CodeFaceRoboto},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := m.adoptKept(brand.Brand{Seed: fixtureRed, Mono: tc.kept})
			if got.AppliedMono() != tc.want {
				t.Errorf("opened on %q, want %q", got.AppliedMono(), tc.want)
			}
		})
	}
}

// TestKeepingWritesTheMonoBesideTheSeed: Keep writes Brand.Mono
// ("JetBrains Mono" or empty for Roboto Mono) and does not drop the key
// when JetBrains is selected. A kept file restores the same selection.
func TestKeepingWritesTheMonoBesideTheSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5)},
	})
	m.KeepPath = path
	m = pickMono(m, tokens.CodeFaceJetBrains)
	_, cmd := Update(m, KeepSeed{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	m = ReduceModel(m, msg)
	kept := brand.KeptFrom(path)
	if kept.Mono != tokens.CodeFaceJetBrains {
		t.Errorf("the file holds mono %q, want JetBrains Mono", kept.Mono)
	}
	if !m.SeedIsKept() {
		t.Error("the window does not report the kept choice as kept")
	}
	back := withBases().adoptKept(kept)
	if back.AppliedMono() != tokens.CodeFaceJetBrains {
		t.Errorf("a window opening on the kept file landed on %q", back.AppliedMono())
	}

	// Roboto Mono writes empty, so the key is omitted, and a file without
	// the key opens on Roboto Mono.
	m = pickMono(m, tokens.CodeFaceRoboto)
	if m.SeedIsKept() {
		t.Error("choosing Roboto Mono left the window claiming the theme on screen was kept")
	}
	_, cmd = Update(m, KeepSeed{})
	msg, err = cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	m = ReduceModel(m, msg)
	kept = brand.KeptFrom(path)
	if kept.Mono != "" {
		t.Errorf("Roboto Mono wrote mono %q, want empty", kept.Mono)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), `"mono"`) {
		t.Errorf("Roboto Mono left a mono key in the file:\n%s", raw)
	}
	if withBases().adoptKept(kept).AppliedMono() != tokens.CodeFaceRoboto {
		t.Error("a file without the key did not open on Roboto Mono")
	}
}

// TestAnUnknownMonoInTheFileIsRobotoMono: a junk name survives the file
// and opens the window on the default.
func TestAnUnknownMonoInTheFileIsRobotoMono(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := brand.SaveTo(path, brand.Brand{Seed: fixtureRed, Mono: "Comic Sans"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	kept := brand.KeptFrom(path)
	got := withBases().adoptKept(kept)
	if got.AppliedMono() != tokens.CodeFaceRoboto {
		t.Errorf("opened on %q, want Roboto Mono", got.AppliedMono())
	}
}
