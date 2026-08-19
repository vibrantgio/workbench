package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	stdcolor "image/color"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// withBases is the model as the window starts it, as far as the syntax bases
// go: every base on offer, sitting on the default.
func withBases() Model {
	bases := baseOptions()
	return Model{Bases: bases, BaseAt: baseIndex(bases, highlight.DefaultBase)}
}

// baseRowY is the y of row i of the selector, in a window whose selector has
// not been scrolled. It is computed from the same constants the column lays
// out with.
func baseRowY(i int) int {
	return galleryTop() + int(BasePad) + i*int(BaseRow) + int(BaseRow)/2
}

// TestEveryBaseIsOnOffer: the column is built from the highlighter's own list,
// so a base that exists is a base that can be chosen. A selector showing a
// subset would be a window that cannot reach half of what it claims to.
func TestEveryBaseIsOnOffer(t *testing.T) {
	got := baseOptions()
	names := make([]string, len(got))
	for i, b := range got {
		names[i] = b.Name
	}
	want := highlight.Bases()
	if !slices.Equal(names, want) {
		t.Errorf("the selector offers %d names, the highlighter has %d", len(names), len(want))
	}
	if len(names) < 70 {
		t.Errorf("only %d bases on offer — the embedded set alone is larger than that", len(names))
	}
	if !slices.Contains(names, highlight.DefaultBase) {
		t.Errorf("the default base %q is not on offer", highlight.DefaultBase)
	}
}

// TestAStyleFromTheFolderJoinsTheColumn: a style somebody wrote themselves is
// choosable beside the ones that ship, and says which it is. The mark is the
// only difference — a loaded base is derived from exactly like an embedded
// one — and it is there because "did my file load" is the first question
// anybody who dropped one in has.
func TestAStyleFromTheFolderJoinsTheColumn(t *testing.T) {
	const style = `<style name="quayside-day">
  <entry type="Background" style="bg:#fdf6e3 #586e75"/>
  <entry type="Keyword" style="bold #d33682"/>
  <entry type="LiteralString" style="#2aa198"/>
</style>
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "quayside.xml"), []byte(style), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if names, skipped := highlight.LoadDir(dir); len(names) != 1 || len(skipped) != 0 {
		t.Fatalf("loaded %v and skipped %v, want the one style", names, skipped)
	}
	var found *BaseOption
	for _, b := range baseOptions() {
		if b.Name == "quayside-day" {
			b := b
			found = &b
		}
	}
	if found == nil {
		t.Fatal("a style read from the folder is not on offer in the column")
	}
	if !found.Added {
		t.Error("a style read from the folder is not marked as one — it reads as something that ships")
	}
	for _, b := range baseOptions() {
		if b.Name == highlight.DefaultBase && b.Added {
			t.Error("an embedded style is marked as added")
		}
	}
}

// TestTheWindowOpensOnTheKeptBase, and on the default when what was kept is a
// name this build cannot resolve — a style whose file has left the folder, or
// one written by a build that had it. Neither is a reason to open on whatever
// sorted first.
func TestTheWindowOpensOnTheKeptBase(t *testing.T) {
	m := withBases()
	for _, tc := range []struct {
		name string
		kept string
		want string
	}{
		{"a base that resolves", "dracula", "dracula"},
		{"a base that does not", "a-style-nobody-wrote", highlight.DefaultBase},
		{"nothing kept", "", highlight.DefaultBase},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := m.adoptKept(brand.Brand{Seed: fixtureRed, Base: tc.kept})
			if got.Base() != tc.want {
				t.Errorf("opened on %q, want %q", got.Base(), tc.want)
			}
			if got.KeptBase != tc.want {
				t.Errorf("the kept base reads %q, want %q", got.KeptBase, tc.want)
			}
		})
	}
}

// TestChoosingABaseRecoloursTheCode is the selector's whole claim: the names
// are not a list of styles, they are what the code on the page is coloured
// with. Both captures are taken with the specimen already in view, so what
// moves between them is ink and not scroll.
func TestChoosingABaseRecoloursTheCode(t *testing.T) {
	e := newEmbed()
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5), candidate(fixtureRed, 0.5)},
	})
	// The first pick brings the specimen into view; the two after it are the
	// comparison.
	pageOn(t, e, ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, "dracula")}), tokens.DefaultLight)
	first := pageOn(t, e, ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, "dracula")}), tokens.DefaultLight)
	second := pageOn(t, e, ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, "monokai")}), tokens.DefaultLight)
	pct := bandChange(first, second, galleryTop(), galleryBottom())
	t.Logf("switching base moved %.2f%% of the band the page is in", pct)
	if pct == 0 {
		t.Error("switching the syntax base changed no pixel of the page — the code is not following the choice")
	}
}

// TestTheChosenBaseIsMarked: one row carries the choice, and it is the one
// that was chosen. Asserted inside a single render, so a window that repaints
// wholesale cannot pass by accident.
func TestTheChosenBaseIsMarked(t *testing.T) {
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5), candidate(fixtureRed, 0.5)},
	})
	// A column already scrolled where the test wants it: the selector brings
	// the kept base into view once, at the start, and a fresh one would put
	// whichever row is chosen at the top.
	sel := newBaseSelector()
	sel.shown = true
	for _, chosen := range []int{0, 1, 2} {
		img := pageOn(t, newEmbed(), ReduceModel(m, SelectBase{Index: chosen}), tokens.DefaultLight, sel)
		at := func(row int) stdcolor.RGBA {
			return img.RGBAAt(int(Pad)+int(BasePad)+int(BaseInk)/2, baseRowY(row))
		}
		for _, other := range []int{0, 1, 2} {
			if other == chosen {
				continue
			}
			if at(chosen) == at(other) {
				t.Errorf("with base %d chosen, its row is drawn exactly like row %d — nothing marks the choice", chosen, other)
			}
		}
		if a, b := (chosen+1)%3, (chosen+2)%3; at(a) != at(b) {
			t.Errorf("with base %d chosen, rows %d and %d differ from each other — more than one row is marked", chosen, a, b)
		}
	}
}

// TestKeepingWritesTheBaseBesideTheSeed: the two choices are one theme, so
// they go into the file together and come back together.
func TestKeepingWritesTheBaseBesideTheSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5)},
	})
	m.KeepPath = path
	m = ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, "monokai")})
	_, cmd := Update(m, KeepSeed{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	m = ReduceModel(m, msg)
	kept := brand.KeptFrom(path)
	if kept.Base != "monokai" {
		t.Errorf("the file holds base %q, want monokai", kept.Base)
	}
	if seed, _ := m.Seed(); kept.Seed != seed {
		t.Errorf("the file holds seed %v, want %v", kept.Seed, seed)
	}
	if !m.SeedIsKept() {
		t.Error("the window does not report the kept choice as kept")
	}
	// Changing the base alone is a change to what is kept, and the affordance
	// has to go back to offering rather than confirming.
	if ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, "dracula")}).SeedIsKept() {
		t.Error("choosing another base left the window claiming the theme on screen was kept")
	}
}

// TestAMalformedStyleIsNamedAndNotThrown: the styles folder is a place people
// edit by hand. A file that will not parse costs that file and says so where
// the window says everything else that went wrong; it does not cost the
// folder, and it certainly does not cost the window.
func TestAMalformedStyleIsNamedAndNotThrown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "half-typed.xml"), []byte("<style name=\"x\"><entry"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	names, skipped := highlight.LoadDir(dir)
	if len(names) != 0 || len(skipped) != 1 {
		t.Fatalf("loaded %v and skipped %v, want the one bad file skipped", names, skipped)
	}
	sentence := skippedSentence(skipped)
	if sentence == "" {
		t.Fatal("a skipped style produced nothing to show")
	}
	t.Logf("the caption reads: %s", sentence)
	// And it reaches the window: the caption is where a problem is said, and
	// a sentence nobody draws is a sentence nobody reads.
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5)},
	})
	quiet := page(t, m, tokens.DefaultLight)
	m.Problem = sentence
	noisy := page(t, m, tokens.DefaultLight)
	if golden.PixelDiff(quiet, noisy) == 0 {
		t.Error("the window drew the same pixels with and without a style it could not load")
	}
}
