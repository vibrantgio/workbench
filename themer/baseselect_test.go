package main

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	stdcolor "image/color"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// withBases is the model as the window starts it, as far as the syntax bases
// go: every base on offer, sitting on the default pair.
func withBases() Model {
	bases := baseOptions()
	d := highlight.DefaultBases()
	return Model{
		Bases:   bases,
		LightAt: baseIndex(bases, d.Light, false),
		DarkAt:  baseIndex(bases, d.Dark, true),
	}
}

// pick applies the named base under one appearance, the way a click on that
// appearance's own list does.
func pick(m Model, name string, dark bool) Model {
	return ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, name, dark), Dark: dark})
}

// wearPair is a fresh document style dressed in one pair under these tokens:
// the plate the specimen on the embedded page is drawn with, to measure the
// pixels against.
func wearPair(p highlight.BasePair, c tokens.ColorTokens) markdown.Style {
	st := markdown.FromTokens(c, tokens.DefaultTypography)
	highlight.WearPair(&st, p, c)
	return st
}

// wearAlone is [wearPair] for one name under both appearances: what somebody
// who chose that base and nothing else would be looking at.
func wearAlone(name string, c tokens.ColorTokens) markdown.Style {
	return wearPair(highlight.BasePair{Light: name, Dark: name}, c)
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

// inkJitter is how far apart two channel values may be and still count as the
// same pixel. The window shapes its text through one process-wide shaper with
// a cache inside it, so two frames whose captions differ by a word come out a
// level or two of antialiasing apart in glyphs neither frame changed —
// measured at three levels out of 255 over 293 pixels of a half-million. The
// tolerance sits above that and far below a recolour, which is what the
// assertions using it are actually about.
const inkJitter = 8

// movedInk counts the pixels between rows y0 and y1 that changed by more than
// the rasteriser's own jitter.
func movedInk(a, b *image.RGBA, y0, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			p, q := a.RGBAAt(x, y), b.RGBAAt(x, y)
			if apart(p.R, q.R) > inkJitter || apart(p.G, q.G) > inkJitter ||
				apart(p.B, q.B) > inkJitter || apart(p.A, q.A) > inkJitter {
				n++
			}
		}
	}
	return n
}

func apart(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
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
//
// The two are compared as sets. Which order the column lists them in is the
// window's own decision and is asserted where that decision is made; what is
// asserted here is that nothing the highlighter has went missing on the way.
func TestEveryBaseIsOnOffer(t *testing.T) {
	got := baseOptions()
	names := make([]string, len(got))
	for i, b := range got {
		names[i] = b.Name
	}
	want := highlight.Bases()
	if offered := slices.Sorted(slices.Values(names)); !slices.Equal(offered, want) {
		t.Errorf("the selector offers %d names, the highlighter has %d", len(offered), len(want))
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
// only difference — a loaded base is worn exactly like an embedded
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

// TestTheWindowOpensOnTheKeptBases, one per appearance, and on that
// appearance's default when what was kept is a name this build cannot resolve
// — a style whose file has left the folder, or one written by a build that had
// it. Neither is a reason to open on whatever sorted first.
//
// The last case is the file that predates the pair. It names one base with no
// appearance attached, and it arrives with that name in both members: the
// window keeps it for the appearance it was measured to be fitted to, and
// opens the other on the default rather than putting a palette balanced for
// paper on a near-black slab.
func TestTheWindowOpensOnTheKeptBases(t *testing.T) {
	m := withBases()
	d := highlight.DefaultBases()
	for _, tc := range []struct {
		name        string
		kept        brand.BasePair
		light, dark string
	}{
		{"a pair that resolves", brand.BasePair{Light: "github", Dark: "dracula"}, "github", "dracula"},
		{"names that do not", brand.BasePair{Light: "a-style-nobody-wrote", Dark: "another"}, d.Light, d.Dark},
		{"nothing kept", brand.BasePair{}, d.Light, d.Dark},
		{"one dark base from a file that predates the pair", brand.BasePair{Light: "dracula", Dark: "dracula"}, d.Light, "dracula"},
		{"one light base from a file that predates the pair", brand.BasePair{Light: "github", Dark: "github"}, "github", d.Dark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := m.adoptKept(brand.Brand{Seed: fixtureRed, Base: tc.kept})
			if got.Base(false) != tc.light || got.Base(true) != tc.dark {
				t.Errorf("opened on %q under the sun and %q under the moon, want %q and %q",
					got.Base(false), got.Base(true), tc.light, tc.dark)
			}
			if want := (highlight.BasePair{Light: tc.light, Dark: tc.dark}); got.KeptBases != want {
				t.Errorf("the kept bases read %+v, want %+v", got.KeptBases, want)
			}
			// And each member is on the list of the appearance it applies to,
			// so the window opens with the applied row marked on both halves.
			if !slices.Contains(got.VisibleBases(false), got.LightAt) ||
				!slices.Contains(got.VisibleBases(true), got.DarkAt) {
				t.Error("an applied base is missing from the list of the appearance it colours")
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
				if !m.Bases[baseIndex(m.Bases, name, tc.dark)].Suits(tc.dark) {
					t.Fatalf("%q is not on the list this scheme shows", name)
				}
			}
			first := atTheCode(t, e, pick(m, tc.one, tc.dark), tokens.DefaultLight)
			second := atTheCode(t, e, pick(m, tc.then, tc.dark), tokens.DefaultLight)
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
		img := atTheCode(t, newEmbed(), ReduceModel(m, SelectBase{Index: visible[row], Dark: false}), tokens.DefaultLight, sel)
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
		if b := m.Bases[i]; !b.Light {
			t.Errorf("the sun lists %q, which was fitted to a dark ground", b.Name)
		}
	}
	for _, i := range dark {
		if b := m.Bases[i]; !b.Dark {
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

// paired is the model mid-flow with a distinct base under each appearance:
// what the window looks like once somebody has chosen twice.
func paired(t *testing.T) Model {
	t.Helper()
	m := pick(pick(judging(), pairLight, false), pairDark, true)
	if m.Base(false) != pairLight || m.Base(true) != pairDark {
		t.Fatalf("the model applied %q and %q, want %q and %q", m.Base(false), m.Base(true), pairLight, pairDark)
	}
	return m
}

// The fixture pair: two styles that are nothing to do with each other, so
// neither could be reached from the other by any counterpart rule.
const (
	pairLight = "github"
	pairDark  = "dracula"
)

// TestFlippingTheSchemeSwitchesTheAppliedBase is the pair's whole point. The
// window holds a base per appearance, and the scheme control moves between
// them: the code takes the other member's plate, the column marks that
// member's row, and the line over the page names it — all in the frame the
// switch is pressed, and without either choice being edited.
func TestFlippingTheSchemeSwitchesTheAppliedBase(t *testing.T) {
	m := paired(t)
	sun := ReduceModel(m, SetScheme{Dark: false})
	moon := ReduceModel(m, SetScheme{Dark: true})
	// Neither choice moved: the flip picks which one applies.
	if sun.AppliedBases() != moon.AppliedBases() {
		t.Errorf("flipping the scheme edited the pair: %+v became %+v", sun.AppliedBases(), moon.AppliedBases())
	}
	if got := GalleryHintFor(sun, false); !strings.Contains(got, pairLight) {
		t.Errorf("under the sun the page says %q, want it naming %q", got, pairLight)
	}
	if got := GalleryHintFor(moon, true); !strings.Contains(got, pairDark) {
		t.Errorf("under the moon the page says %q, want it naming %q", got, pairDark)
	}

	// The plate the specimen is drawn on, ground and ink: under each
	// appearance it is that appearance's own member, worn alone. This is the
	// measurement behind the pixels — a page that had gone on drawing through
	// the other member would match the other plate here.
	const src = "// greet.\nfunc greet(name string) string { return name }\n"
	for _, tc := range []struct {
		name   string
		dark   bool
		colors tokens.ColorTokens
	}{
		{"under the sun", false, SchemeFor(tokens.DefaultLight, sun)},
		{"under the moon", true, SchemeFor(tokens.DefaultLight, moon)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			applied := m.Base(tc.dark)
			got := wearPair(m.AppliedBases(), tc.colors)
			want := wearAlone(applied, tc.colors)
			other := wearAlone(m.Base(!tc.dark), tc.colors)
			gotRuns, wantRuns := got.Highlight("go", src), want.Highlight("go", src)
			otherRuns := other.Highlight("go", src)
			if len(gotRuns) == 0 || len(gotRuns) != len(wantRuns) {
				t.Fatalf("the specimen split into %d runs, %s alone gives %d", len(gotRuns), applied, len(wantRuns))
			}
			if got.CodeBackground != want.CodeBackground || got.CodeColor != want.CodeColor {
				t.Fatalf("the specimen sits on %v under %v ink, %s alone gives %v under %v",
					got.CodeBackground, got.CodeColor, applied, want.CodeBackground, want.CodeColor)
			}
			coloured, apart := 0, 0
			for i := range gotRuns {
				if gotRuns[i].Color != wantRuns[i].Color {
					t.Fatalf("run %d is %v, %s alone gives %v", i, gotRuns[i].Color, applied, wantRuns[i].Color)
				}
				if gotRuns[i].Color.A != 0 {
					coloured++
				}
				if i < len(otherRuns) && gotRuns[i].Color != otherRuns[i].Color {
					apart++
				}
			}
			ground := got.CodeBackground != other.CodeBackground
			if coloured == 0 || (apart == 0 && !ground) {
				t.Fatalf("%d runs carry a colour, %d differ from the other member's inks and the grounds differ=%v — this pair cannot show which member is applied", coloured, apart, ground)
			}
			t.Logf("%s: %d runs, %d coloured, %s's plate on %v; unlike %s's by ground=%v and %d runs",
				tc.name, len(gotRuns), coloured, applied, got.CodeBackground, m.Base(!tc.dark), ground, apart)
		})
	}

	// And it is drawn. Each half of the column marks its own appearance's
	// choice, at its own place in a list the other half does not hold.
	for _, tc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"sun", false, tokens.DefaultLight}, {"moon", true, tokens.DefaultDark}} {
		on := ReduceModel(m, SetScheme{Dark: tc.dark})
		visible := on.VisibleBases(tc.dark)
		row := slices.Index(visible, on.BaseAt(tc.dark))
		if row < 0 {
			t.Fatalf("the %s's list does not hold the base it applies", tc.name)
		}
		sel := settled(tc.dark)
		sel.st.ScrollTo(max(0, row-baseLead))
		img := atTheCode(t, newEmbed(), on, tc.os, sel)
		mark := img.RGBAAt(baseInkX(), baseRowY(baseLead))
		for _, other := range []int{baseLead - 1, baseLead + 1} {
			if mark == img.RGBAAt(baseInkX(), baseRowY(other)) {
				t.Errorf("under the %s the applied base's row is drawn exactly like row %d — nothing marks which base is applied", tc.name, other)
			}
		}
	}
}

// TestThreeFlavoursOfOneFamilyAreThreeDifferentSpecimens is the case that is
// hardest for a specimen to pass and the plainest thing it is for. One
// family's three dark flavours are the same inks at the same volumes on three
// different grounds: tell a reader they are the same picture and the reader is
// right. The specimen shows each base's own ground, so the difference between
// them is on screen and choosing between them is a choice somebody can see
// they made.
//
// Both halves are asserted. The grounds are read off the plate, which is the
// claim; the pixels are counted in the window, which is whether the claim
// reached anybody. A ground that only the plate knows about is a value in a
// struct.
func TestThreeFlavoursOfOneFamilyAreThreeDifferentSpecimens(t *testing.T) {
	moon := ReduceModel(judging(), SetScheme{Dark: true})
	c := SchemeFor(tokens.DefaultDark, moon)
	flavours := []string{"catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha"}
	grounds := make([]stdcolor.NRGBA, len(flavours))
	for i, name := range flavours {
		grounds[i] = wearAlone(name, c).CodeBackground
		t.Logf("%s draws its fence on %v", name, grounds[i])
	}
	for i, a := range flavours {
		for j := i + 1; j < len(flavours); j++ {
			if grounds[i] == grounds[j] {
				t.Errorf("%s and %s put the same ground %v under a fence — these two cannot be told apart", a, flavours[j], grounds[i])
			}
		}
	}

	for i, name := range flavours {
		img := atTheCode(t, newEmbed(), pick(moon, name, true), tokens.DefaultDark, settled(true))
		own := exactly(img, grounds[i])
		if own == 0 {
			t.Errorf("%s is chosen and not one pixel of the window is its ground %v", name, grounds[i])
		}
		for j, other := range grounds {
			if j == i {
				continue
			}
			if n := exactly(img, other); n >= own {
				t.Errorf("with %s chosen, %d pixels are its ground and %d are %s's — the specimen is not showing what was picked",
					name, own, n, flavours[j])
			}
		}
		t.Logf("%s chosen: %d pixels of its own ground on screen", name, own)
	}
}

// exactly counts the pixels of img that are precisely c, alpha ignored: a
// surface filled in a colour and composited over nothing keeps it, so this is
// how much of the window a fill actually reached.
func exactly(img *image.RGBA, c stdcolor.NRGBA) int {
	n, b := 0, img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if p := img.RGBAAt(x, y); p.R == c.R && p.G == c.G && p.B == c.B {
				n++
			}
		}
	}
	return n
}

// TestEachSchemeRendersThroughItsOwnMember: changing the member an appearance
// is not showing repaints no part of the page that appearance is drawn
// through, and changing the one it is showing does. Same seed, same scheme,
// same scroll — the only thing that moves between the two renders is one
// member of the pair.
//
// The identity strip is excluded and then asserted on separately: the line under the
// source's name reads both members out, so a change to the hidden one has to
// show there. That is the point of naming the pair rather than the half in
// force — a choice that reaches two things must not look like one that reached
// one — and it is the one place on the page where the hidden member is
// legitimately visible.
func TestEachSchemeRendersThroughItsOwnMember(t *testing.T) {
	base := paired(t)
	for _, tc := range []struct {
		name    string
		dark    bool
		os      tokens.ColorTokens
		other   string // the member for the appearance NOT on screen
		instead string // and the member for the one that is
	}{
		{"under the sun", false, tokens.DefaultLight, "monokai", "solarized-light"},
		{"under the moon", true, tokens.DefaultDark, "solarized-light", "monokai"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			on := ReduceModel(base, SetScheme{Dark: tc.dark})
			was := atTheCode(t, newEmbed(), on, tc.os, settled(tc.dark))
			hidden := pick(on, tc.other, !tc.dark)
			got := atTheCode(t, newEmbed(), hidden, tc.os, settled(tc.dark))
			if n := movedInk(was, got, galleryTop(), galleryBottom()); n != 0 {
				t.Errorf("choosing %q for the appearance that is not showing repainted %d pixels of the page", tc.other, n)
			}
			if pct := bandChange(was, got, headTop(), headBottom()); pct == 0 {
				t.Errorf("choosing %q for the appearance that is not showing left the caption naming the old pair", tc.other)
			}
			shown := pick(on, tc.instead, tc.dark)
			if got := atTheCode(t, newEmbed(), shown, tc.os, settled(tc.dark)); golden.PixelDiff(was, got) == 0 {
				t.Errorf("choosing %q for the appearance on screen changed no pixel", tc.instead)
			}
		})
	}
}

// TestAPressIsForTheAppearanceItWasMadeUnder: the sun's list sets the light
// base and the moon's the dark one, and neither reaches across.
func TestAPressIsForTheAppearanceItWasMadeUnder(t *testing.T) {
	m := withBases()
	d := highlight.DefaultBases()
	if under := pick(m, "solarized-light", false); under.Base(false) != "solarized-light" || under.Base(true) != d.Dark {
		t.Errorf("a press under the sun left the pair %+v, want the light member alone moved", under.AppliedBases())
	}
	if under := pick(m, "monokai", true); under.Base(true) != "monokai" || under.Base(false) != d.Light {
		t.Errorf("a press under the moon left the pair %+v, want the dark member alone moved", under.AppliedBases())
	}
}

// TestBothMembersOfThePairAreKept: the pair is the choice, so the file holds
// both names whichever appearance the window happened to be showing when the
// affordance was pressed — and a window opening on that file comes back on the
// same pair.
func TestBothMembersOfThePairAreKept(t *testing.T) {
	for _, tc := range []struct {
		scheme string
		dark   bool
	}{
		{"pressed under the sun", false},
		{"pressed under the moon", true},
	} {
		t.Run(tc.scheme, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "theme.json")
			m := ReduceModel(paired(t), SetScheme{Dark: tc.dark})
			m.KeepPath = path
			_, cmd := Update(m, KeepSeed{})
			msg, err := cmd.First()
			if err != nil {
				t.Fatalf("the keep command failed: %v", err)
			}
			m = ReduceModel(m, msg)
			kept := brand.KeptFrom(path)
			if kept.Base.Light != pairLight || kept.Base.Dark != pairDark {
				t.Errorf("the file holds %+v, want %q under the sun and %q under the moon", kept.Base, pairLight, pairDark)
			}
			if !m.SeedIsKept() {
				t.Error("the window does not report the kept choice as kept")
			}
			back := withBases().adoptKept(kept)
			if back.Base(false) != pairLight || back.Base(true) != pairDark {
				t.Errorf("a window opening on the kept file landed on %q and %q", back.Base(false), back.Base(true))
			}
		})
	}
}

// TestKeepingWritesTheBasesBesideTheSeed: the choices are one theme, so they go
// into the file together and come back together — including a change to the
// member the window is not showing, which is still a change to what is kept.
func TestKeepingWritesTheBasesBesideTheSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "scene.png",
		Preview:    preview(scene(480, 360)),
		Candidates: []imageseed.Candidate{candidate(fixtureBlue, 0.5)},
	})
	m.KeepPath = path
	m = pick(m, "monokai", true)
	_, cmd := Update(m, KeepSeed{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	m = ReduceModel(m, msg)
	kept := brand.KeptFrom(path)
	if kept.Base.Dark != "monokai" || kept.Base.Light != highlight.DefaultBase {
		t.Errorf("the file holds %+v, want monokai under the moon and the default under the sun", kept.Base)
	}
	if seed, _ := m.Seed(); kept.Seed != seed {
		t.Errorf("the file holds seed %v, want %v", kept.Seed, seed)
	}
	if !m.SeedIsKept() {
		t.Error("the window does not report the kept choice as kept")
	}
	// Changing either member is a change to what is kept, and the affordance
	// has to go back to offering rather than confirming — including the member
	// of the pair that is not on screen.
	if pick(m, "dracula", true).SeedIsKept() {
		t.Error("choosing another base left the window claiming the theme on screen was kept")
	}
	if pick(m, "solarized-light", false).SeedIsKept() {
		t.Error("choosing a base for the other appearance left the window claiming the theme on screen was kept")
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
