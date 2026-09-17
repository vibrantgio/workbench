package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/tokens"
)

// pickMono applies the named code face the way a click on that row does.
func pickMono(m Model, name string) Model {
	return ReduceModel(m, SelectMono{Name: name})
}

// TestTheGroupOffersExactlyTwoFaces: Roboto Mono and JetBrains Mono, nothing
// else. A font picker this is not.
func TestTheGroupOffersExactlyTwoFaces(t *testing.T) {
	if len(codeFaces) != 2 {
		t.Fatalf("the group offers %d names, want exactly two", len(codeFaces))
	}
	if codeFaces[0] != tokens.CodeFaceRoboto || codeFaces[1] != tokens.CodeFaceJetBrains {
		t.Errorf("the group offers %v, want Roboto Mono then JetBrains Mono", codeFaces)
	}
}

// TestChoosingACodeFaceRestylesTheSample: a press restyles the fence on the
// next frame. What is counted is the sample's own plate, so the marked row
// moving on the plate cannot pass for a restyle.
func TestChoosingACodeFaceRestylesTheSample(t *testing.T) {
	m := judging()
	first := pageWith(t, m, tokens.PlatformLight, settled(false))
	second := pageWith(t, pickMono(m, tokens.CodeFaceJetBrains), tokens.PlatformLight, settled(false))
	if n := movedIn(first, second, codePlate()); n == 0 {
		t.Error("switching the code face changed no pixel of the sample — the fence is not following the choice")
	} else {
		t.Logf("switching the face moved %d pixels of the sample", n)
	}
}

// TestTheTwoFacesMarkTheChosenName: one of the two carries the choice, and it
// is the one that was chosen.
func TestTheTwoFacesMarkTheChosenName(t *testing.T) {
	for i, name := range codeFaces {
		img := pageWith(t, pickMono(judging(), name), tokens.PlatformLight, settled(false))
		mine := img.RGBAAt(faceMarkX(i), faceRowY())
		other := img.RGBAAt(faceMarkX(1-i), faceRowY())
		if mine == other {
			t.Errorf("with %q chosen its row is drawn exactly like the other — nothing marks the choice", name)
		}
		if !is(mine, PaletteFrom(tokens.PlatformLight).AccentForeground) {
			t.Errorf("the marker on %q drew %v, want what reads on the platform's selection", name, mine)
		}
	}
}

// TestTheCodeFaceMovesTheShaperAndNotTheSample: a face change needs the
// matching font collection, so the shaper is replaced; the parsed document
// stays and is only restyled.
func TestTheCodeFaceMovesTheShaperAndNotTheSample(t *testing.T) {
	th := themed{col: tokens.PlatformLight, typ: pinned(), pinned: true}
	roboto, one := th.codeType(pickMono(judging(), tokens.CodeFaceRoboto))
	jetbrains, two := th.codeType(pickMono(judging(), tokens.CodeFaceJetBrains))
	if roboto.Code.Typeface != tokens.CodeFaceRoboto || jetbrains.Code.Typeface != tokens.CodeFaceJetBrains {
		t.Fatalf("the two faces resolved to %q and %q", roboto.Code.Typeface, jetbrains.Code.Typeface)
	}
	if one == two {
		t.Error("both faces shape through one shaper — the JetBrains collection is not being reached")
	}
	code := newCodeState()
	doc := code.doc
	pageWith(t, pickMono(judging(), tokens.CodeFaceJetBrains), tokens.PlatformLight, code)
	if code.doc != doc {
		t.Error("choosing a code face rebuilt the sample — the document was re-parsed")
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
			got := m.adoptKept(brand.Brand{ThemeColor: sceneAccent, Mono: tc.kept})
			if got.AppliedMono() != tc.want {
				t.Errorf("opened on %q, want %q", got.AppliedMono(), tc.want)
			}
		})
	}
}

// TestKeepingWritesTheMonoBesideTheColour: keeping writes Brand.Mono —
// "JetBrains Mono", or empty for Roboto Mono, which is how the file spells
// the default — and a window opening on that file comes back on the same
// face.
func TestKeepingWritesTheMonoBesideTheColour(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	m.KeepPath = path
	m = pickMono(m, tokens.CodeFaceJetBrains)
	m = keep(t, m)
	kept := brand.KeptFrom(path)
	if kept.Mono != tokens.CodeFaceJetBrains {
		t.Errorf("the file holds mono %q, want JetBrains Mono", kept.Mono)
	}
	if !m.IsKept() {
		t.Error("the window does not report the kept choice as kept")
	}
	if back := withBases().adoptKept(kept); back.AppliedMono() != tokens.CodeFaceJetBrains {
		t.Errorf("a window opening on the kept file landed on %q", back.AppliedMono())
	}

	// Roboto Mono writes empty, so the key is omitted, and a file without the
	// key opens on Roboto Mono.
	m = pickMono(m, tokens.CodeFaceRoboto)
	if m.IsKept() {
		t.Error("choosing Roboto Mono left the window claiming the theme on screen was kept")
	}
	m = keep(t, m)
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

// TestAnUnknownMonoInTheFileIsRobotoMono: a junk name survives the file and
// opens the window on the default.
func TestAnUnknownMonoInTheFileIsRobotoMono(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := brand.SaveTo(path, brand.Brand{ThemeColor: sceneAccent, Mono: "Comic Sans"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := withBases().adoptKept(brand.KeptFrom(path)); got.AppliedMono() != tokens.CodeFaceRoboto {
		t.Errorf("opened on %q, want Roboto Mono", got.AppliedMono())
	}
}

// keep runs the keep command the affordance sends and folds its answer back
// into the model, which is the whole path from a press to a file.
func keep(t *testing.T, m Model) Model {
	t.Helper()
	_, cmd := Update(m, KeepColor{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	return ReduceModel(m, msg)
}
