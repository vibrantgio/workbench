package main

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"

	stdcolor "image/color"

	"github.com/vibrantgio/components/gallery/inventory"
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

// atTheCode renders the window with the embedded page scrolled so the code
// specimen's own row leads the viewport, which is the only place the selector
// is on screen at all. The first render is what builds the column and learns
// which row that is; the capture is the second.
func atTheCode(t *testing.T, e *embed, m Model, os tokens.ColorTokens, sel ...*baseSelector) *image.RGBA {
	t.Helper()
	pageOn(t, e, m, os, sel...)
	row := e.codeRow()
	if row < 0 {
		t.Fatal("the embedded page has no code specimen row to scroll to")
	}
	e.st.ScrollTo(row)
	return pageOn(t, e, m, os, sel...)
}

// settled is a selector that will not move itself: the column brings the
// applied base into view when the list it is in is new, and a test that reads
// rows by position needs the rows where it left them.
func settled(dark bool) *baseSelector {
	sel := newBaseSelector()
	sel.shown, sel.dark = true, dark
	return sel
}

// baseInkX is the x of the marker on a chosen row, and baseRowY the y of the
// centre of visible row i — both in a window scrolled to the code specimen,
// computed from the same constants the row and the column lay out with.
func baseInkX() int {
	return int(Pad) + int(inventory.SectionPadX) + int(BasePad) + int(BaseInk)/2
}

func baseRowY(i int) int {
	top := galleryTop() + int(inventory.SectionPadY) + int(BasePad) + int(BaseHead)
	return top + i*int(BaseRow) + int(BaseRow)/2
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

// judging is the model mid-flow: a picture dropped, its candidates in hand and
// every base on offer — the only state the embedded page and its selector are
// drawn in.
func judging() Model {
	return ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5), candidate(fixtureRed, 0.5)},
	})
}

// TestChoosingABaseRecoloursTheCode is the selector's whole claim: the names
// are not a list of styles, they are what the code on the page is coloured
// with. Both captures are taken with the specimen in view, so what moves
// between them is ink and not scroll.
func TestChoosingABaseRecoloursTheCode(t *testing.T) {
	// Two names off each scheme's own list, which is the only way the window
	// offers them.
	for _, tc := range []struct {
		scheme    string
		dark      bool
		one, then string
	}{
		{"under the sun", false, "github", "solarized-light"},
		{"under the moon", true, "dracula", "monokai"},
	} {
		t.Run(tc.scheme, func(t *testing.T) {
			e := newEmbed()
			m := ReduceModel(judging(), SetScheme{Dark: tc.dark})
			for _, name := range []string{tc.one, tc.then} {
				if !m.Bases[baseIndex(m.Bases, name)].Suits(tc.dark) {
					t.Fatalf("%q is not on the list this scheme shows", name)
				}
			}
			first := atTheCode(t, e, ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, tc.one)}), tokens.DefaultLight)
			second := atTheCode(t, e, ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, tc.then)}), tokens.DefaultLight)
			pct := bandChange(first, second, galleryTop(), galleryBottom())
			t.Logf("switching base moved %.2f%% of the band the page is in", pct)
			if pct == 0 {
				t.Error("switching the syntax base changed no pixel of the page — the code is not following the choice")
			}
		})
	}
}

// TestTheChosenBaseIsMarked: one row carries the choice, and it is the one
// that was chosen. Asserted inside a single render, so a window that repaints
// wholesale cannot pass by accident.
func TestTheChosenBaseIsMarked(t *testing.T) {
	m := judging()
	// Three names off the light half, so the list under test is the same one
	// in all three renders and a row keeps its place between them.
	visible := m.VisibleBases(false)
	if len(visible) < 3 {
		t.Fatalf("the light half holds %d bases — too few to tell a marked row from its neighbours", len(visible))
	}
	sel := settled(false)
	for _, row := range []int{0, 1, 2} {
		img := atTheCode(t, newEmbed(), ReduceModel(m, SelectBase{Index: visible[row]}), tokens.DefaultLight, sel)
		at := func(r int) stdcolor.RGBA { return img.RGBAAt(baseInkX(), baseRowY(r)) }
		for _, other := range []int{0, 1, 2} {
			if other == row {
				continue
			}
			if at(row) == at(other) {
				t.Errorf("with row %d chosen, it is drawn exactly like row %d — nothing marks the choice", row, other)
			}
		}
		if a, b := (row+1)%3, (row+2)%3; at(a) != at(b) {
			t.Errorf("with row %d chosen, rows %d and %d differ from each other — more than one row is marked", row, a, b)
		}
	}
}

// TestTheSelectorSitsBesideTheCodeAndNowhereElse: the standing column is gone.
// Before the page is scrolled to the specimen there is no selector on screen at
// all, and once it is there is one — beside the code, inside the row, not down
// the side of the window.
func TestTheSelectorSitsBesideTheCodeAndNowhereElse(t *testing.T) {
	m := judging()
	e := newEmbed()
	top := pageOn(t, e, m, tokens.DefaultLight, settled(false))
	beside := atTheCode(t, e, m, tokens.DefaultLight, settled(false))
	page := SchemeFor(tokens.DefaultLight, m)
	// The page reaches the window's own margin. Its first row is the group
	// banner, painted in the theme's primary across the whole panel; a column
	// standing beside the page would have that banner starting a hundred and
	// ninety points further in, with the column's own ground here instead. The
	// probe is inside the footprint the standing column used to occupy, past
	// the banner's own label and clear of the panel's rounded corner.
	same := func(got stdcolor.RGBA, want stdcolor.NRGBA) bool {
		return got.R == want.R && got.G == want.G && got.B == want.B
	}
	if edge := top.RGBAAt(int(Pad)+int(BaseW)-int(Gap), galleryTop()+int(RowLabelH)); !same(edge, page.Primary) {
		t.Errorf("the page's first row starts at %v rather than the theme's primary %v — something is standing between the window's margin and the page",
			edge, page.Primary)
	}
	// And the column is inside the specimen's row rather than beside the page:
	// the strip left of it is that row's own margin, which is the page's ground.
	if edge := beside.RGBAAt(int(Pad)+int(inventory.SectionPadX)/2, baseRowY(2)); !same(edge, page.Background) {
		t.Errorf("the strip left of the column drew %v, want the page's own ground %v — the column is not seated in the page", edge, page.Background)
	}
}

// TestTheColumnFollowsTheSchemeControl: the sun's list is the light bases and
// the moon's the dark ones, and it is the one control at the top that says
// which. A second control could be set to disagree with the appearance on
// screen; this is the assertion that there is no second control.
func TestTheColumnFollowsTheSchemeControl(t *testing.T) {
	m := judging()
	light := ReduceModel(m, SetScheme{Dark: false}).VisibleBases(false)
	dark := ReduceModel(m, SetScheme{Dark: true}).VisibleBases(true)
	t.Logf("the sun lists %d bases and the moon %d, out of %d", len(light), len(dark), len(m.Bases))
	if len(light) == 0 || len(dark) == 0 {
		t.Fatalf("one half of the list came out empty: %d light, %d dark", len(light), len(dark))
	}
	if len(light) == len(m.Bases) || len(dark) == len(m.Bases) {
		t.Error("a half of the list is the whole list — nothing is being filtered")
	}
	for _, i := range light {
		if b := m.Bases[i]; !b.Light && i != m.BaseAt {
			t.Errorf("the sun lists %q, which was fitted to a dark ground", b.Name)
		}
	}
	for _, i := range dark {
		if b := m.Bases[i]; !b.Dark && i != m.BaseAt {
			t.Errorf("the moon lists %q, which was fitted to a light ground", b.Name)
		}
	}
	// And the two lists are drawn, not just computed.
	e := newEmbed()
	sun := atTheCode(t, e, ReduceModel(m, SetScheme{Dark: false}), tokens.DefaultLight)
	moon := atTheCode(t, e, ReduceModel(m, SetScheme{Dark: true}), tokens.DefaultLight)
	if bandChange(sun, moon, baseRowY(0), baseRowY(8)) == 0 {
		t.Error("the column drew the same pixels under both schemes — the list is not following the control")
	}
}

// TestTheAppliedBaseIsNeverTakenOffTheList: a base chosen under one scheme is
// still the base when the other is showing, and the row saying so stays on the
// list. Dropping it would leave the page coloured by something the column no
// longer admits to, and no way back to it.
func TestTheAppliedBaseIsNeverTakenOffTheList(t *testing.T) {
	m := ReduceModel(judging(), SelectBase{Index: baseIndex(withBases().Bases, "dracula")})
	if m.Base() != "dracula" {
		t.Fatalf("the model applied %q, want dracula", m.Base())
	}
	if !slices.Contains(m.VisibleBases(true), m.BaseAt) {
		t.Error("a dark base is missing from the moon's own list")
	}
	if !slices.Contains(m.VisibleBases(false), m.BaseAt) {
		t.Error("the applied base was dropped from the list when the scheme it was fitted to stopped showing")
	}
	// Flipping the scheme changes what is offered, never what was chosen.
	if flipped := ReduceModel(m, SetScheme{Dark: false}); flipped.Base() != "dracula" {
		t.Errorf("flipping the scheme changed the applied base to %q", flipped.Base())
	}
	// And the row is drawn, marked, on the list it does not match. A fresh
	// selector brings the applied base into view — which is what makes a row
	// this far down a list of thirty-six findable at all — and leaves baseLead
	// names above it, so that is the row under test.
	img := atTheCode(t, newEmbed(), m, tokens.DefaultLight, newBaseSelector())
	mark := img.RGBAAt(baseInkX(), baseRowY(baseLead))
	for _, other := range []int{baseLead - 1, baseLead + 1} {
		if mark == img.RGBAAt(baseInkX(), baseRowY(other)) {
			t.Errorf("the applied base's row is drawn exactly like row %d — a base on the list for the other scheme is not marked as the applied one", other)
		}
	}
}

// TestABasePickedUnderEitherSchemeKeeps: the two halves are one choice and one
// file. A name picked under the moon comes back out of the kept theme exactly
// as a name picked under the sun does — the file records what was chosen, and
// the appearance it was chosen under is not part of it.
func TestABasePickedUnderEitherSchemeKeeps(t *testing.T) {
	for _, tc := range []struct {
		scheme string
		dark   bool
		base   string
	}{
		{"under the sun", false, "solarized-light"},
		{"under the moon", true, "solarized-dark"},
	} {
		t.Run(tc.scheme, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "theme.json")
			m := ReduceModel(judging(), SetScheme{Dark: tc.dark})
			m.KeepPath = path
			// Picked off the list the scheme is showing, which is the only way
			// the window offers it.
			at := -1
			for _, i := range m.VisibleBases(tc.dark) {
				if m.Bases[i].Name == tc.base {
					at = i
				}
			}
			if at < 0 {
				t.Fatalf("%q is not on the list this scheme shows", tc.base)
			}
			m = ReduceModel(m, SelectBase{Index: at})
			_, cmd := Update(m, KeepSeed{})
			msg, err := cmd.First()
			if err != nil {
				t.Fatalf("the keep command failed: %v", err)
			}
			m = ReduceModel(m, msg)
			if kept := brand.KeptFrom(path); kept.Base != tc.base {
				t.Errorf("the file holds base %q, want %q", kept.Base, tc.base)
			}
			if !m.SeedIsKept() {
				t.Error("the window does not report the kept choice as kept")
			}
			// And it comes back: a window opening on that file lands on the
			// same name, whichever scheme it opens in.
			if back := withBases().adoptKept(brand.KeptFrom(path)); back.Base() != tc.base {
				t.Errorf("a window opening on the kept file landed on %q, want %q", back.Base(), tc.base)
			}
		})
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
