package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/tokens"
)

// keeping returns a loaded model whose kept theme goes to a file inside a
// temporary directory, and that file's path. Nothing in this package ever
// writes to the real one.
func keeping(t *testing.T) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme.json")
	m := loaded()
	m.KeepPath = path
	return m, path
}

// TestKeepingWritesTheChosenSeed drives the press the way the window does —
// the message into Update, the command it returns run to its message, and
// that message back through the reducer — and then reads the file.
func TestKeepingWritesTheChosenSeed(t *testing.T) {
	m, path := keeping(t)
	m.Selected = 1 // the blue one, so a passing test cannot be the default

	next, cmd := Update(m, KeepSeed{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	kept, ok := msg.(SeedKept)
	if !ok {
		t.Fatalf("keeping reported %T, want SeedKept", msg)
	}
	if kept.Seed != fixtureBlue {
		t.Errorf("kept %v, want the chosen candidate %v", kept.Seed, fixtureBlue)
	}

	onDisk, found, err := brand.LoadFrom(path)
	if err != nil || !found {
		t.Fatalf("the kept theme read back as (%v, %v), want a brand and no error", found, err)
	}
	if onDisk.Seed != fixtureBlue {
		t.Errorf("the file holds %v, want %v", onDisk.Seed, fixtureBlue)
	}
	if onDisk.Source != "scene.png" {
		t.Errorf("the file credits %q, want the picture the colour came out of", onDisk.Source)
	}

	next = ReduceModel(next, kept)
	if !next.SeedIsKept() {
		t.Error("after keeping, the window does not know the choice on screen is the one on disk")
	}
}

// TestAKeptThemeIsTheSameSchemeNextTime: what comes back off disk derives
// the schemes the window was showing when it was kept, exactly.
func TestAKeptThemeIsTheSameSchemeNextTime(t *testing.T) {
	m, path := keeping(t)
	seed, _ := m.Seed()
	_, cmd := Update(m, KeepSeed{})
	if _, err := cmd.First(); err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	wantLight, wantDark := tokens.FromSeed(seed)
	gotLight, gotDark := brand.KeptFrom(path).Colors()
	if gotLight != wantLight || gotDark != wantDark {
		t.Error("the theme that comes back is not the one that was kept")
	}
}

// TestKeepingWithNothingChosenWritesNothing: the affordance is not on screen
// before a picture has been dropped, and a message that arrives anyway is
// not a file.
func TestKeepingWithNothingChosenWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	_, cmd := Update(Model{KeepPath: path}, KeepSeed{})
	if msg, err := cmd.First(); msg != nil || err != nil {
		t.Errorf("keeping nothing reported (%v, %v), want no message at all", msg, err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("keeping nothing wrote a file")
	}
}

// TestAKeepThatCannotBeWrittenSaysSoAndStaysOpen: a full disk, a read-only
// directory or a machine with nowhere to put the file is a sentence in the
// window, not the end of the session.
func TestAKeepThatCannotBeWrittenSaysSoAndStaysOpen(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "in-the-way")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, tc := range []struct{ name, path string }{
		{"nowhere to write it", ""},
		{"a path that will not take a file", filepath.Join(blocked, "theme.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := loaded()
			m.KeepPath = tc.path
			_, cmd := Update(m, KeepSeed{})
			msg, err := cmd.First()
			if err != nil {
				t.Fatalf("the keep command failed instead of reporting: %v", err)
			}
			failed, ok := msg.(KeepFailed)
			if !ok {
				t.Fatalf("an impossible keep reported %T, want KeepFailed", msg)
			}
			after := ReduceModel(m, failed)
			if after.Problem == "" {
				t.Error("the window says nothing about a keep that did not happen")
			}
			if after.SeedIsKept() {
				t.Error("a failed keep left the window claiming the choice was kept")
			}
			if len(after.Candidates) != len(m.Candidates) {
				t.Error("a failed keep took the picture down with it")
			}
		})
	}
}

// TestKeepingAnotherCandidateReplacesTheKeptOne: the file holds one brand,
// and it is the last one chosen.
func TestKeepingAnotherCandidateReplacesTheKeptOne(t *testing.T) {
	m, path := keeping(t)
	for _, i := range []int{0, 2} {
		m.Selected = i
		_, cmd := Update(m, KeepSeed{})
		msg, err := cmd.First()
		if err != nil {
			t.Fatalf("the keep command failed: %v", err)
		}
		m = ReduceModel(m, msg)
	}
	want, _ := m.Seed()
	if got := brand.KeptFrom(path).Seed; got != want {
		t.Errorf("the file holds %v, want the last colour chosen %v", got, want)
	}
	if !m.SeedIsKept() {
		t.Error("the window does not report the last-kept choice as kept")
	}
}

// TestTheKeepButtonSaysWhetherItIsKept: the two states are drawn
// differently, which is the whole point of a control that reports as well
// as offers.
func TestTheKeepButtonSaysWhetherItIsKept(t *testing.T) {
	m := loaded()
	offering := page(t, m, tokens.DefaultLight)
	m.Kept, _ = m.Seed()
	m.KeptBases = m.AppliedBases()
	if !m.SeedIsKept() {
		t.Fatal("the fixture does not have its own choice kept")
	}
	confirming := page(t, m, tokens.DefaultLight)
	if golden.PixelDiff(offering, confirming) == 0 {
		t.Error("the keep affordance looks the same whether or not the choice is kept")
	}
}
