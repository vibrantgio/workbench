package main

import (
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/tokens"
)

// pick applies the named base under one appearance, the way a click on that
// appearance's own list does.
func pick(m Model, name string, dark bool) Model {
	return ReduceModel(m, SelectBase{Index: baseIndex(m.Bases, name, dark), Dark: dark})
}

// wearPair is a fresh document style dressed in one pair under one set: the
// plate the sample is drawn with, to measure the pixels against.
func wearPair(p highlight.BasePair, c tokens.PlatformColors) markdown.Style {
	return CodeStyle(c, tokens.DefaultTypography, p)
}

// wearAlone is [wearPair] for one name under both appearances: what somebody
// who chose that base and nothing else would be looking at.
func wearAlone(name string, c tokens.PlatformColors) markdown.Style {
	return wearPair(highlight.BasePair{Light: name, Dark: name}, c)
}

// The column's geometry in window pixels, worked out from the same constants
// the page lays out with: a title row, then five groups, each a title row and
// a box under it, then the footer.
func pictureTitleTop() int { return int(Pad) + int(TitleH) + int(Gap) }

func boxTopAfter(titleTop int) int { return titleTop + int(GroupTitleH) + int(GroupBelow) }

func pictureBoxTop() int { return boxTopAfter(pictureTitleTop()) }

func colourTitleTop() int { return pictureBoxTop() + int(PictureBoxH) + int(GroupAbove) }

func colourBoxTop() int { return boxTopAfter(colourTitleTop()) }

func faceTitleTop() int { return colourBoxTop() + int(ColourBoxH) + int(GroupAbove) }

func faceBoxTop() int { return boxTopAfter(faceTitleTop()) }

func baseTitleTop() int { return faceBoxTop() + int(FaceBoxH) + int(GroupAbove) }

func baseBoxTop() int { return boxTopAfter(baseTitleTop()) }

func previewTitleTop() int { return baseBoxTop() + int(BaseBoxH) + int(GroupAbove) }

func previewBoxTop() int { return boxTopAfter(previewTitleTop()) }

// footerTop is where the footer's row begins, and previewBoxBottom the edge
// the preview's box is cut at: the footer is pinned to the page's bottom
// margin and the preview takes what is left.
func footerTop() int { return windowH - int(Pad) - int(FooterH) }

func previewBoxBottom() int { return footerTop() - int(GroupAbove) }

// boxLead is the leading edge of what stands in a box, and boxTrail the
// trailing one.
func boxLead() int { return int(Pad) + int(BoxInset) }

func boxTrail() int { return windowW - int(Pad) - int(BoxInset) }

// faceChipX is the leading edge of the nth code face's row, faceMarkX the x
// of the marker a chosen one carries, and faceFillX a point clear of both the
// marker and the name, where the row's own fill can be read.
func faceChipX(i int) int { return boxLead() + i*(int(FaceChipW)+int(CellGap)) }

func faceMarkX(i int) int { return faceChipX(i) + int(BaseMark)/2 }

func faceFillX(i int) int { return faceChipX(i) + int(BaseMark) + 4 }

// faceRowY is the centre line the two code faces stand on: the box's content
// is one row tall, so both are on it.
func faceRowY() int { return faceBoxTop() + int(BoxPad) + int(BaseRow)/2 }

// baseListTop is the leading edge of the first name in the column: the box's
// own air above its content.
func baseListTop() int { return baseBoxTop() + int(BoxPad) }

// baseListBottom is where the column runs out: the box's own air below its
// content.
func baseListBottom() int { return baseBoxTop() + int(BaseBoxH) - int(BoxPad) }

// baseRowY is the centre of visible row i, and baseMarkX the x of the marker
// a chosen row carries.
func baseRowY(i int) int { return baseListTop() + i*int(BaseRow) + int(BaseRow)/2 }

func baseMarkX() int { return boxLead() + int(BaseMark)/2 }

// rowFillX is a point in a chooser row clear of both its marker and the name
// it carries, which is where a row's own fill can be read.
func rowFillX() int { return boxLead() + int(BaseMark) + 4 }

// codePlate is the region the sample is drawn in: what is left of the syntax
// base group's box after the column of names.
func codePlate() image.Rectangle {
	return image.Rect(boxLead()+int(BaseW)+int(Gap), baseListTop(),
		boxTrail(), baseListBottom())
}

// pixelJitter is how far apart two channel values may be and still count as
// the same pixel. The window shapes its text through one shaper with a cache
// inside it, so two frames whose captions differ by a word come out a level or
// two of antialiasing apart in glyphs neither frame changed. The tolerance
// sits above that and far below a recolour, which is what the assertions
// using it are actually about.
const pixelJitter = 8

// movedIn counts the pixels of r that changed between two captures by more
// than the rasteriser's own jitter.
func movedIn(a, b *image.RGBA, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p, q := a.RGBAAt(x, y), b.RGBAAt(x, y)
			if apart(p.R, q.R) > pixelJitter || apart(p.G, q.G) > pixelJitter ||
				apart(p.B, q.B) > pixelJitter || apart(p.A, q.A) > pixelJitter {
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

// settled is a chooser that will not move itself: the column brings the
// applied base into view when the list it is in is new, and a test that reads
// rows by position needs the rows where it left them.
func settled(dark bool) *codeState {
	cs := newCodeState()
	cs.bases.shown, cs.bases.dark = true, dark
	return cs
}

// pageWith renders the whole window for a model on one platform set, with the
// code section's state in the caller's hands.
func pageWith(t *testing.T, m Model, c tokens.PlatformColors, cs *codeState) *image.RGBA {
	t.Helper()
	return pageState(t, m, c, image.Pt(windowW, windowH), cs)
}

// TestEveryBaseIsOnOffer: the column is built from the highlighting package's
// own list, so a base that exists is a base that can be chosen. A chooser
// showing a subset would be a window that cannot reach half of what it claims
// to.
//
// The two are compared as sets. Which order the column lists them in is the
// window's own decision and is asserted where that decision is made; what is
// asserted here is that nothing went missing on the way.
func TestEveryBaseIsOnOffer(t *testing.T) {
	got := baseOptions()
	names := make([]string, len(got))
	for i, b := range got {
		names[i] = b.Name
	}
	want := highlight.Bases()
	if offered := slices.Sorted(slices.Values(names)); !slices.Equal(offered, want) {
		t.Errorf("the chooser offers %d names, the highlighting package has %d", len(offered), len(want))
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
// only difference — a loaded base is worn exactly like an embedded one — and
// it is there because "did my file load" is the first question anybody who
// dropped one in has.
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
// The last cases are the file that predates the pair. It names one base with
// no appearance attached, and it arrives with that name in both members: the
// window keeps it for the appearance it was measured to be fitted to, and
// opens the other on the default rather than putting a palette balanced for a
// light page on a near-black slab.
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
			got := m.adoptKept(brand.Brand{ThemeColor: sceneAccent, Base: tc.kept})
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

// TestChoosingABaseRecoloursTheCode is the chooser's whole claim: the names
// are not a list of styles, they are what the code beside them is coloured
// with. What is counted is the sample's own plate alone, so a marked row
// moving in the column cannot pass for a recolour.
func TestChoosingABaseRecoloursTheCode(t *testing.T) {
	for _, tc := range []struct {
		scheme    string
		dark      bool
		set       tokens.PlatformColors
		one, then string
	}{
		{"under the sun", false, tokens.PlatformLight, "github", "solarized-light"},
		{"under the moon", true, tokens.PlatformDark, "dracula", "monokai"},
	} {
		t.Run(tc.scheme, func(t *testing.T) {
			m := ReduceModel(judging(), SetScheme{Dark: tc.dark})
			for _, name := range []string{tc.one, tc.then} {
				if !m.Bases[baseIndex(m.Bases, name, tc.dark)].Suits(tc.dark) {
					t.Fatalf("%q is not on the list this scheme shows", name)
				}
			}
			first := pageWith(t, pick(m, tc.one, tc.dark), tc.set, settled(tc.dark))
			second := pageWith(t, pick(m, tc.then, tc.dark), tc.set, settled(tc.dark))
			if n := movedIn(first, second, codePlate()); n == 0 {
				t.Error("switching the syntax base changed no pixel of the sample — the code is not following the choice")
			} else {
				t.Logf("switching the base moved %d pixels of the sample", n)
			}
		})
	}
}

// TestTheChosenBaseIsMarked: one row carries the choice, and it is the one
// that was chosen. Asserted inside a single render, so a window that repaints
// wholesale cannot pass by accident.
func TestTheChosenBaseIsMarked(t *testing.T) {
	m := judging()
	visible := m.VisibleBases(false)
	if len(visible) < 3 {
		t.Fatalf("the light half holds %d bases — too few to tell a marked row from its neighbours", len(visible))
	}
	for _, row := range []int{0, 1, 2} {
		img := pageWith(t, ReduceModel(m, SelectBase{Index: visible[row], Dark: false}),
			tokens.PlatformLight, settled(false))
		at := func(r int) stdcolor.RGBA { return img.RGBAAt(baseMarkX(), baseRowY(r)) }
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

// TestTheChooserStandsBesideTheSample: the names and the fence they colour
// are on screen at the same time, in one row — the column at the group box's
// leading edge, the sample's plate beside it. A name chosen from behind the
// thing it changes is chosen blind.
func TestTheChooserStandsBesideTheSample(t *testing.T) {
	c := tokens.PlatformLight
	img := pageWith(t, judging(), c, settled(false))
	p := PaletteFrom(c)
	// The run between the column and the plate, where the group's box shows
	// through: the box is the platform's box on the window's plane, and the
	// column draws no plate of its own inside it.
	if got := img.RGBAAt(boxLead()+int(BaseW)+int(Gap)/2, baseRowY(0)); !is(got, p.Surface) {
		t.Errorf("the run between the column and the sample drew %v, want the group's box %v", got, p.Surface)
	}
	// And the sample's plate is where the row leaves it, on the previewed
	// set's own plane.
	plate := codePlate()
	if got := img.RGBAAt(plate.Min.X+int(BasePad)/2, plate.Min.Y+plate.Dy()/2); !is(got, c.WindowBackground) {
		t.Errorf("the sample's plate drew %v, want the previewed set's plane %v", got, c.WindowBackground)
	}
}

// TestTheColumnScrollsToEveryNameOnIt: the column's end and the window's end
// are the same end. The list lays out inside the plate it is drawn in, and
// with the last name in the half applied and the column scrolled to its end,
// the marker on that name is read off the pixels.
func TestTheColumnScrollsToEveryNameOnIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
		set  tokens.PlatformColors
	}{{"sun", false, tokens.PlatformLight}, {"moon", true, tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			m := ReduceModel(judging(), SetScheme{Dark: tc.dark})
			visible := m.VisibleBases(tc.dark)
			n := len(visible)
			if n < 3 {
				t.Fatalf("the %s lists %d bases — too few to scroll", tc.name, n)
			}
			shows := baseListBottom() - baseListTop()
			cs := settled(tc.dark)
			pageWith(t, m, tc.set, cs)
			if got := cs.bases.st.Viewport(); got > shows {
				t.Errorf("the column lays out in %d points where the window shows %d of it — %d names sit under the fold with no scroll that reaches them",
					got, shows, (got-shows)/int(BaseRow))
			}
			t.Logf("the %s lists %d names and the window shows %d points of the column", tc.name, n, cs.bases.st.Viewport())

			// And the last name on the list is one the column can be scrolled
			// to. It is the applied one, so the marker is what says it arrived.
			on := ReduceModel(m, SelectBase{Index: visible[n-1], Dark: tc.dark})
			end := settled(tc.dark)
			pageWith(t, on, tc.set, end)
			end.bases.st.ScrollToEnd(n)
			img := pageWith(t, on, tc.set, end)
			q := end.bases.st.Position()
			if last := q.First + q.Count; last < n {
				t.Fatalf("scrolled to its end the column shows names %d to %d of %d — the last %d cannot be reached",
					q.First+1, last, n, n-last)
			}
			y := baseRowY(n-1-q.First) - q.Offset
			if y >= baseListBottom() {
				t.Fatalf("the last name is laid out at y=%d, past the box's bottom edge at y=%d", y, baseListBottom())
			}
			if mark, plain := img.RGBAAt(baseMarkX(), y), img.RGBAAt(baseMarkX(), baseRowY(0)-q.Offset); mark == plain {
				t.Errorf("the last name is applied and its row at y=%d is drawn %v, exactly like an unmarked row — nothing on screen says the column reached it",
					y, mark)
			}
		})
	}
}

// TestTheColumnFollowsTheSchemeSwitch: the sun's list is the light bases and
// the moon's the dark ones, and it is the one switch at the top of the window
// that says which. A second control could be set to disagree with the
// appearance on screen; this is the assertion that there is no second control.
func TestTheColumnFollowsTheSchemeSwitch(t *testing.T) {
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
			t.Errorf("the sun lists %q, which was fitted to a dark background", b.Name)
		}
	}
	for _, i := range dark {
		if b := m.Bases[i]; !b.Dark {
			t.Errorf("the moon lists %q, which was fitted to a light background", b.Name)
		}
	}
	// And the two lists are drawn, not just computed.
	sun := pageWith(t, ReduceModel(m, SetScheme{Dark: false}), tokens.PlatformLight, settled(false))
	moon := pageWith(t, ReduceModel(m, SetScheme{Dark: true}), tokens.PlatformLight, settled(true))
	column := image.Rect(int(Pad), baseListTop(), int(Pad)+int(BaseW), baseRowY(4))
	if movedIn(sun, moon, column) == 0 {
		t.Error("the column drew the same pixels under both schemes — the list is not following the switch")
	}
}

// The fixture pair: two styles that are nothing to do with each other, so
// neither could be reached from the other by any counterpart rule.
const (
	pairLight = "github"
	pairDark  = "dracula"
)

// paired is the model with a distinct base under each appearance: what the
// window holds once somebody has chosen twice.
func paired(t *testing.T) Model {
	t.Helper()
	m := pick(pick(judging(), pairLight, false), pairDark, true)
	if m.Base(false) != pairLight || m.Base(true) != pairDark {
		t.Fatalf("the model applied %q and %q, want %q and %q", m.Base(false), m.Base(true), pairLight, pairDark)
	}
	return m
}

// TestFlippingTheSchemeSwitchesTheAppliedBase is the pair's whole point. The
// window holds a base per appearance, and the scheme switch moves between
// them: the code takes the other member's plate and the column marks that
// member's row, in the frame the switch is pressed, without either choice
// being edited.
func TestFlippingTheSchemeSwitchesTheAppliedBase(t *testing.T) {
	m := paired(t)
	sun := ReduceModel(m, SetScheme{Dark: false})
	moon := ReduceModel(m, SetScheme{Dark: true})
	if sun.AppliedBases() != moon.AppliedBases() {
		t.Errorf("flipping the scheme edited the pair: %+v became %+v", sun.AppliedBases(), moon.AppliedBases())
	}
	if got := BaseHintFor(sun, false, len(sun.VisibleBases(false))); !strings.HasPrefix(got, pairLight+", ") {
		t.Errorf("under the sun the group says %q, want it naming %q", got, pairLight)
	}
	if got := BaseHintFor(moon, true, len(moon.VisibleBases(true))); !strings.HasPrefix(got, pairDark+", ") {
		t.Errorf("under the moon the group says %q, want it naming %q", got, pairDark)
	}

	// The plate the sample is drawn on, background and foreground: under each
	// appearance it is that appearance's own member, worn alone. This is the
	// measurement behind the pixels — a window that had gone on drawing
	// through the other member would match the other plate here.
	const src = "// greet.\nfunc greet(name string) string { return name }\n"
	for _, tc := range []struct {
		name string
		dark bool
		set  tokens.PlatformColors
	}{
		{"under the sun", false, tokens.PlatformLight},
		{"under the moon", true, tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			applied := m.Base(tc.dark)
			got := wearPair(m.AppliedBases(), tc.set)
			want := wearAlone(applied, tc.set)
			other := wearAlone(m.Base(!tc.dark), tc.set)
			gotRuns, wantRuns := got.Highlight("go", src), want.Highlight("go", src)
			otherRuns := other.Highlight("go", src)
			if len(gotRuns) == 0 || len(gotRuns) != len(wantRuns) {
				t.Fatalf("the sample split into %d runs, %s alone gives %d", len(gotRuns), applied, len(wantRuns))
			}
			if got.CodeBackground != want.CodeBackground || got.CodeColor != want.CodeColor {
				t.Fatalf("the sample sits on %v under %v foreground, %s alone gives %v under %v",
					got.CodeBackground, got.CodeColor, applied, want.CodeBackground, want.CodeColor)
			}
			coloured, differ := 0, 0
			for i := range gotRuns {
				if gotRuns[i].Color != wantRuns[i].Color {
					t.Fatalf("run %d is %v, %s alone gives %v", i, gotRuns[i].Color, applied, wantRuns[i].Color)
				}
				if gotRuns[i].Color.A != 0 {
					coloured++
				}
				if i < len(otherRuns) && gotRuns[i].Color != otherRuns[i].Color {
					differ++
				}
			}
			backgroundsDiffer := got.CodeBackground != other.CodeBackground
			if coloured == 0 || (differ == 0 && !backgroundsDiffer) {
				t.Fatalf("%d runs carry a colour, %d differ from the other member's and the backgrounds differ=%v — this pair cannot show which member is applied",
					coloured, differ, backgroundsDiffer)
			}
			t.Logf("%s: %d runs, %d coloured, %s's plate on %v", tc.name, len(gotRuns), coloured, applied, got.CodeBackground)
		})
	}
}

// TestEachSchemeRendersThroughItsOwnMember: changing the member an appearance
// is not showing repaints no part of the section that appearance is drawn
// through, and changing the one it is showing does. Same colour, same scheme,
// same scroll — the only thing that moves between the two renders is one
// member of the pair.
func TestEachSchemeRendersThroughItsOwnMember(t *testing.T) {
	base := paired(t)
	for _, tc := range []struct {
		name    string
		dark    bool
		set     tokens.PlatformColors
		other   string // the member for the appearance NOT on screen
		instead string // and the member for the one that is
	}{
		{"under the sun", false, tokens.PlatformLight, "monokai", "solarized-light"},
		{"under the moon", true, tokens.PlatformDark, "solarized-light", "monokai"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			on := ReduceModel(base, SetScheme{Dark: tc.dark})
			was := pageWith(t, on, tc.set, settled(tc.dark))
			hidden := pick(on, tc.other, !tc.dark)
			got := pageWith(t, hidden, tc.set, settled(tc.dark))
			if n := golden.PixelDiff(was, got); n != 0 {
				t.Errorf("choosing %q for the appearance that is not showing repainted %d pixels", tc.other, n)
			}
			shown := pick(on, tc.instead, tc.dark)
			if img := pageWith(t, shown, tc.set, settled(tc.dark)); golden.PixelDiff(was, img) == 0 {
				t.Errorf("choosing %q for the appearance on screen changed no pixel", tc.instead)
			}
		})
	}
}

// TestThreeFlavoursOfOneFamilyAreThreeDifferentSamples is the case that is
// hardest for a sample to pass and the plainest thing it is for. One family's
// three dark flavours are the same colours at the same strengths on three
// different backgrounds: tell a reader they are the same picture and the
// reader is right. The sample shows each base's own background, so the
// difference between them is on screen and choosing between them is a choice
// somebody can see they made.
func TestThreeFlavoursOfOneFamilyAreThreeDifferentSamples(t *testing.T) {
	moon := ReduceModel(judging(), SetScheme{Dark: true})
	c := tokens.PlatformDark
	flavours := []string{"catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha"}
	backgrounds := make([]stdcolor.NRGBA, len(flavours))
	for i, name := range flavours {
		backgrounds[i] = wearAlone(name, c).CodeBackground
		t.Logf("%s draws its fence on %v", name, backgrounds[i])
	}
	for i, a := range flavours {
		for j := i + 1; j < len(flavours); j++ {
			if backgrounds[i] == backgrounds[j] {
				t.Errorf("%s and %s put the same background %v under a fence — these two cannot be told apart", a, flavours[j], backgrounds[i])
			}
		}
	}
	for i, name := range flavours {
		img := pageWith(t, pick(moon, name, true), c, settled(true))
		own := exactly(img, backgrounds[i])
		if own == 0 {
			t.Errorf("%s is chosen and not one pixel of the window is its background %v", name, backgrounds[i])
		}
		for j, other := range backgrounds {
			if j == i {
				continue
			}
			if n := exactly(img, other); n >= own {
				t.Errorf("with %s chosen, %d pixels are its background and %d are %s's — the sample is not showing what was picked",
					name, own, n, flavours[j])
			}
		}
		t.Logf("%s chosen: %d pixels of its own background on screen", name, own)
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
// affordance was pressed — and a window opening on that file comes back on
// the same pair.
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
			m = ReduceModel(m, HexTyped{Text: "#e8112d"})
			_, cmd := Update(m, KeepColor{})
			msg, err := cmd.First()
			if err != nil {
				t.Fatalf("the keep command failed: %v", err)
			}
			m = ReduceModel(m, msg)
			kept := brand.KeptFrom(path)
			if kept.Base.Light != pairLight || kept.Base.Dark != pairDark {
				t.Errorf("the file holds %+v, want %q under the sun and %q under the moon", kept.Base, pairLight, pairDark)
			}
			if !m.IsKept() {
				t.Error("the window does not report the kept choice as kept")
			}
			back := withBases().adoptKept(kept)
			if back.Base(false) != pairLight || back.Base(true) != pairDark {
				t.Errorf("a window opening on the kept file landed on %q and %q", back.Base(false), back.Base(true))
			}
		})
	}
}

// TestKeepingWritesTheBasesBesideTheColour: the choices are one theme, so
// they go into the file together and come back together — including a change
// to the member the window is not showing, which is still a change to what is
// kept.
func TestKeepingWritesTheBasesBesideTheColour(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	m.KeepPath = path
	m = pick(m, "monokai", true)
	_, cmd := Update(m, KeepColor{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	m = ReduceModel(m, msg)
	kept := brand.KeptFrom(path)
	if kept.Base.Dark != "monokai" || kept.Base.Light != highlight.DefaultBase {
		t.Errorf("the file holds %+v, want monokai under the moon and the default under the sun", kept.Base)
	}
	if col, _ := m.Color(); kept.ThemeColor != col {
		t.Errorf("the file holds the colour %v, want %v", kept.ThemeColor, col)
	}
	if !m.IsKept() {
		t.Error("the window does not report the kept choice as kept")
	}
	// Changing either member is a change to what is kept, and the affordance
	// has to go back to offering rather than confirming — including the member
	// of the pair that is not on screen.
	if pick(m, "dracula", true).IsKept() {
		t.Error("choosing another base left the window claiming the theme on screen was kept")
	}
	if pick(m, "solarized-light", false).IsKept() {
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
	m := judging()
	without := page(t, m, tokens.PlatformLight)
	m.Problem = sentence
	with := page(t, m, tokens.PlatformLight)
	if golden.PixelDiff(without, with) == 0 {
		t.Error("the window drew the same pixels with and without a style it could not load")
	}
}
