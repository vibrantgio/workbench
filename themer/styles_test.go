package main

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// withStyles is the model as the window starts it: every base on offer, every
// style that has a colour on a card, and the default pair applied.
func withStyles() Model {
	m := withBases()
	m.Styles = styleCards()
	return m
}

// cardIndex finds a style's card by name.
func cardIndex(m Model, name string) int {
	for i, s := range m.Styles {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// The grid's geometry, computed from the same constants the first screen lays
// out with: the bar across the top, the drop well, the grid's label, and then
// the cards.
func wellTop() int  { return navBottom() + int(Gap) }
func gridTop() int  { return wellTop() + int(DropH) + int(Gap) }
func cardsTop() int { return gridTop() + int(StyleHead) + int(RowTop) }

// gridW is the width the cards are laid out in: the page's width less the
// gutter the scrollbar occupies.
func gridW() int {
	return windowW - 2*int(Pad) - int(scrollbar.FromTokens(tokens.DefaultLight).Width())
}

func gridCols() int { return styleColumns(gridW(), int(StyleGap), int(StyleMinW)) }

func styleCellW() int {
	cols := gridCols()
	return (gridW() - (cols-1)*int(StyleGap)) / cols
}

// leadSwatch is a point well inside the leading ink's band on the card at
// position n of the visible grid — the band a click takes its seed from. An
// eighth of the strip's width in: the leading band is between two sevenths and
// two thirds of the strip depending on how many colours the style has, so an
// eighth is inside it either way and clear of the outline round the strip.
func leadSwatch(n int) image.Point {
	cols := gridCols()
	left := int(Pad) + (n%cols)*(styleCellW()+int(StyleGap)) + int(StylePad)
	strip := styleCellW() - 2*int(StylePad)
	top := cardsTop() + (n/cols)*(int(StyleH)+int(StyleGap)) + int(StylePad)
	bottom := cardsTop() + (n/cols)*(int(StyleH)+int(StyleGap)) + int(StyleH) - int(StylePad) - int(StyleChipH) - int(StyleFoot)
	return image.Pt(left+strip/8, (top+bottom)/2)
}

// TestEveryColouredStyleHasACard: the grid is built from the highlighter's own
// list, so a style with colour in it is a style that can be started from. The
// one exception is the style with no colour in it at all, which could only
// offer a click that did nothing.
func TestEveryColouredStyleHasACard(t *testing.T) {
	cards := styleCards()
	byName := map[string]StyleCard{}
	for _, c := range cards {
		byName[c.Name] = c
	}
	for _, n := range highlight.Bases() {
		_, carded := byName[n]
		coloured := len(highlight.BasePalette(n)) > 0
		if carded != coloured {
			t.Errorf("%q has palette=%v but card=%v", n, coloured, carded)
		}
	}
	if len(cards) < 70 {
		t.Errorf("only %d cards — the embedded set alone is larger than that", len(cards))
	}
	if _, carded := byName["bw"]; carded {
		t.Error("bw has a card, and it has no colour to put on one")
	}
}

// TestTheGridIsVividFirst: the leading card is the one whose leading ink has
// the most colour in it, and the near-grey styles honestly come last. A grid
// ordered by anything else would bury the styles somebody would actually want
// to wear under thirty they would not.
func TestTheGridIsVividFirst(t *testing.T) {
	cards := styleCards()
	for i := 1; i < len(cards); i++ {
		if cards[i].Chroma() > cards[i-1].Chroma() {
			t.Fatalf("card %d (%s, chroma %.4f) is more vivid than card %d (%s, chroma %.4f)",
				i, cards[i].Name, cards[i].Chroma(), i-1, cards[i-1].Name, cards[i-1].Chroma())
		}
	}
	t.Logf("leads with %s (chroma %.4f), ends with %s (chroma %.4f)",
		cards[0].Name, cards[0].Chroma(), cards[len(cards)-1].Name, cards[len(cards)-1].Chroma())
}

// TestTheGridIsTheSameGridEveryRun: two builds of the list agree, name for
// name. Ties on vividness are common — four styles lead with pure blue — and a
// grid that reshuffled them between runs would make every screenshot of it a
// different screenshot.
func TestTheGridIsTheSameGridEveryRun(t *testing.T) {
	first, second := styleCards(), styleCards()
	if len(first) != len(second) {
		t.Fatalf("two builds gave %d and %d cards", len(first), len(second))
	}
	for i := range first {
		if first[i].Name != second[i].Name || first[i].Seed() != second[i].Seed() {
			t.Fatalf("card %d is %q/%v then %q/%v", i, first[i].Name, first[i].Seed(), second[i].Name, second[i].Seed())
		}
	}
}

// TestTheGridFollowsTheSchemeControl: the sun's cards are the styles fitted to
// a light ground and the moon's those fitted to a dark one, exactly as the base
// list is filtered. A style that named no ground is on both.
func TestTheGridFollowsTheSchemeControl(t *testing.T) {
	m := withStyles()
	light, dark := m.VisibleStyles(false), m.VisibleStyles(true)
	if len(light) == 0 || len(dark) == 0 {
		t.Fatalf("the grid offers %d light and %d dark cards", len(light), len(dark))
	}
	for _, i := range light {
		if !m.Styles[i].Light {
			t.Errorf("%q is on the sun's grid and is not fitted to a light ground", m.Styles[i].Name)
		}
	}
	for _, i := range dark {
		if !m.Styles[i].Dark {
			t.Errorf("%q is on the moon's grid and is not fitted to a dark ground", m.Styles[i].Name)
		}
	}
	both := 0
	for _, s := range m.Styles {
		if s.Light && s.Dark {
			both++
		}
	}
	if len(light)+len(dark)-both != len(m.Styles) {
		t.Errorf("%d light + %d dark - %d on both is not the %d cards there are",
			len(light), len(dark), both, len(m.Styles))
	}
	t.Logf("%d light cards, %d dark cards, %d on both", len(light), len(dark), both)
}

// TestAStyleFromTheFolderGetsACard: a style somebody wrote themselves is one
// the grid can be started from, and it says where it came from in the same
// word the base list uses.
func TestAStyleFromTheFolderGetsACard(t *testing.T) {
	const style = `<style name="quayside-night">
  <entry type="Background" style="bg:#12171c #cfd8dd"/>
  <entry type="Keyword" style="bold #ff5c8a"/>
  <entry type="LiteralString" style="#3fd0c9"/>
  <entry type="Comment" style="italic #5a6b76"/>
</style>
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "quayside-night.xml"), []byte(style), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if names, skipped := highlight.LoadDir(dir); len(names) != 1 || len(skipped) != 0 {
		t.Fatalf("loaded %v and skipped %v, want the one style", names, skipped)
	}
	m := withStyles()
	i := cardIndex(m, "quayside-night")
	if i < 0 {
		t.Fatal("a style read from the folder has no card in the grid")
	}
	card := m.Styles[i]
	if !card.Added {
		t.Error("a folder style's card does not say it came from the folder")
	}
	if originTag(card.Added, card.Light, card.Dark) != BaseAdded {
		t.Errorf("the card is tagged %q, want the same word the base list uses", originTag(card.Added, card.Light, card.Dark))
	}
	if !card.Dark || card.Light {
		t.Errorf("the card is offered light=%v dark=%v, want the dark ground its own background names", card.Light, card.Dark)
	}
}

// TestTheGridSitsUnderTheDropWell: the well is the first thing and the biggest
// single object on the first screen, and the cards are under it — on screen,
// not below a fold nobody scrolls to.
func TestTheGridSitsUnderTheDropWell(t *testing.T) {
	if cardsTop() >= windowH-int(Pad) {
		t.Fatalf("the first row of cards starts at y=%d, past the bottom of a %d-tall window", cardsTop(), windowH)
	}
	rows := (windowH - int(Pad) - cardsTop()) / (int(StyleH) + int(StyleGap))
	if rows < 3 {
		t.Errorf("only %d rows of cards are on screen under the well", rows)
	}
	if int(DropH) <= int(StyleH) {
		t.Errorf("the drop well is %d dp against a %d dp card — it is not the primary invitation", int(DropH), int(StyleH))
	}
	t.Logf("well %d dp at y=%d, %d rows of cards from y=%d, %d columns of %d dp",
		int(DropH), wellTop(), rows, cardsTop(), gridCols(), styleCellW())
}

// TestTheCardsCarryTheirStylesLeadingInk: every card on screen draws the colour
// a click on it applies. A grid of swatches that were not the seeds would be a
// convincing picture of nothing.
func TestTheCardsCarryTheirStylesLeadingInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		m := withStyles()
		m.Scheme = ShowLight
		os := tokens.DefaultLight
		if dark {
			m.Scheme, os = ShowDark, tokens.DefaultDark
		}
		img := page(t, m, os)
		visible := m.VisibleStyles(dark)
		checked := 0
		for n, i := range visible {
			p := leadSwatch(n)
			if p.Y > windowH-int(Pad) {
				break // a row the window is too short for is not drawn
			}
			want := m.Styles[i].Seed()
			got := img.RGBAAt(p.X, p.Y)
			if got.R != want.R || got.G != want.G || got.B != want.B {
				t.Errorf("card %d (%s) at %v drew %v, want its leading ink %v", n, m.Styles[i].Name, p, got, want)
			}
			checked++
		}
		if checked < 12 {
			t.Errorf("only %d cards were on screen to check", checked)
		}
	}
}

// TestTheFirstScreenIsAnInvitationAndNotABlank: the window before anything has
// been chosen carries the well, the label over the grid and the cards — most
// of the page is drawn on, and the grid is the larger part of what is drawn.
func TestTheFirstScreenIsAnInvitationAndNotABlank(t *testing.T) {
	full := page(t, withStyles(), tokens.DefaultLight)
	bare := page(t, Model{}, tokens.DefaultLight)
	if n := movedInk(full, bare, cardsTop(), windowH-int(Pad)); n == 0 {
		t.Fatal("the first screen draws the same thing with styles as without — the grid is not there")
	}
	blank := golden.Capture(t, image.Pt(windowW, windowH), func(gtx layout.Context) layout.Dimensions {
		fill(gtx, image.Pt(windowW, windowH), tokens.DefaultLight.Background)
		return layout.Dimensions{Size: image.Pt(windowW, windowH)}
	})
	painted := 100 * float64(golden.PixelDiff(full, blank)) / float64(windowW*windowH)
	t.Logf("the first screen paints %.0f%% of the window", painted)
	if painted < 40 {
		t.Errorf("the first screen paints %.0f%% of the window — it reads as a blank", painted)
	}
}

// TestOneClickAdoptsTheSeedAndBothMembers: the whole of what a card promises,
// in the reducer. The seed is the card's leading ink; the pair is the card's
// own style on the side its author fitted it to and the completed member on the
// other; and the row beside it holds the rest of the style's colours, so what a
// style hands the window is what a picture hands it.
func TestOneClickAdoptsTheSeedAndBothMembers(t *testing.T) {
	m := withStyles()
	for _, name := range []string{"github", "dracula", "solarized-light", "monokai"} {
		i := cardIndex(m, name)
		if i < 0 {
			t.Fatalf("no card for %q", name)
		}
		card := m.Styles[i]
		after := ReduceModel(m, AdoptStyle{Index: i})
		seed, ok := after.Seed()
		if !ok || seed != card.Seed() {
			t.Errorf("%s: the window wears %v, want the card's leading ink %v", name, seed, card.Seed())
		}
		if after.Selected != 0 {
			t.Errorf("%s: the row leads with candidate %d, want the leading one", name, after.Selected)
		}
		if len(after.Candidates) != len(card.Candidates) {
			t.Errorf("%s: the row holds %d candidates, want the style's %d", name, len(after.Candidates), len(card.Candidates))
		}
		want := highlight.CompletePair(name)
		if got := after.AppliedBases(); got != want {
			t.Errorf("%s: the pair is %+v, want %+v", name, got, want)
		}
		side := after.Base(card.Dark && !card.Light)
		if card.Light != card.Dark && side != name {
			t.Errorf("%s: the style is not on its own side of the pair — got %q", name, side)
		}
		if after.Style != name || after.Name != name {
			t.Errorf("%s: the window credits %q/%q", name, after.Style, after.Name)
		}
	}
}

// TestAdoptingACardIsBoundsChecked: a message naming a card that is not there
// changes nothing, the way every other index-carrying message here behaves.
func TestAdoptingACardIsBoundsChecked(t *testing.T) {
	m := withStyles()
	for _, i := range []int{-1, len(m.Styles), len(m.Styles) + 100} {
		if after := ReduceModel(m, AdoptStyle{Index: i}); after.Style != "" || len(after.Candidates) != 0 {
			t.Errorf("adopting card %d dressed the window in something", i)
		}
	}
}

// TestBothSchemesWearAnAdoptedCard is the exit measurement: a dark card and a
// light card, each clicked and then read back under the sun and under the moon
// — the palette the window resolves, and both members of the pair it is
// drawing code through.
func TestBothSchemesWearAnAdoptedCard(t *testing.T) {
	m := withStyles()
	for _, tc := range []struct {
		name string
		dark bool
	}{
		{"dracula", true},
		{"github", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := cardIndex(m, tc.name)
			if i < 0 || m.Styles[i].Suits(tc.dark) == m.Styles[i].Suits(!tc.dark) {
				t.Fatalf("%q is not a card of the expected one-sided kind", tc.name)
			}
			card := m.Styles[i]
			after := ReduceModel(m, AdoptStyle{Index: i})
			wantLight, wantDark := tokens.FromSeed(card.Seed())
			pair := highlight.CompletePair(tc.name)
			for _, scheme := range []struct {
				dark bool
				os   tokens.ColorTokens
				want tokens.ColorTokens
			}{
				{false, tokens.DefaultLight, wantLight},
				{true, tokens.DefaultDark, wantDark},
			} {
				on := ReduceModel(after, SetScheme{Dark: scheme.dark})
				if got := SchemeFor(scheme.os, on); got != scheme.want {
					t.Errorf("under %s the window resolved a palette that is not the one %v derives",
						sideName(scheme.dark), card.Seed())
				}
				if got := on.Base(scheme.dark); got != pair.Base(scheme.dark) {
					t.Errorf("under %s the code is coloured from %q, want %q", sideName(scheme.dark), got, pair.Base(scheme.dark))
				}
				// And the window really draws it: the whole page moves off the
				// unchosen first screen.
				before := page(t, ReduceModel(m, SetScheme{Dark: scheme.dark}), scheme.os)
				got := page(t, on, scheme.os)
				if golden.PixelDiff(before, got) == 0 {
					t.Errorf("under %s the click changed no pixel", sideName(scheme.dark))
				}
				t.Logf("%s under %s: seed %s, syntax base %s + %s, background %s",
					tc.name, sideName(scheme.dark), hexOf(card.Seed()), pair.Light, pair.Dark, hexOf(scheme.want.Background))
			}
		})
	}
}

func sideName(dark bool) string {
	if dark {
		return "the moon"
	}
	return "the sun"
}

// TestTheCaptionNamesBothMembers: a click that set two names must not read as a
// click that set one, so the line under the source names the pair whichever
// half is being drawn.
func TestTheCaptionNamesBothMembers(t *testing.T) {
	m := withStyles()
	i := cardIndex(m, "dracula")
	after := ReduceModel(m, AdoptStyle{Index: i})
	pair := after.AppliedBases()
	hint := CaptionHintFor(after)
	for _, name := range []string{pair.Light, pair.Dark} {
		if !strings.Contains(hint, name) {
			t.Errorf("the caption %q does not name %q", hint, name)
		}
	}
	if pair.Light == pair.Dark {
		t.Fatal("the fixture style is its own counterpart, so the test cannot tell one name from two")
	}
	// And it says so under either appearance: the pair does not depend on
	// which half the window happens to be showing.
	if CaptionHintFor(ReduceModel(after, SetScheme{Dark: true})) != hint {
		t.Error("the caption names a different pair under the moon than under the sun")
	}
}

// TestTheCaptionSaysWhichMemberIsWhich: the pair is two names for two
// appearances, and a reader looking at one of them has to be able to tell which
// name is the one in front of them — the more so because one member is usually
// completed by measurement and is not the name that was clicked.
func TestTheCaptionSaysWhichMemberIsWhich(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	pair := after.AppliedBases()
	hint := CaptionHintFor(after)
	if pair.Light == pair.Dark {
		t.Fatal("the fixture style is its own counterpart, so this test cannot tell one name from two")
	}
	if !strings.Contains(hint, pair.Light+" by day") || !strings.Contains(hint, pair.Dark+" by night") {
		t.Errorf("the caption reads %q, want it to say which member each appearance wears", hint)
	}
	// A style fitted to no ground is its own counterpart, and writing such a
	// name out twice reads as a mistake rather than as the fact it is.
	both := -1
	for i, s := range m.Styles {
		if s.Light && s.Dark {
			both = i
			break
		}
	}
	if both < 0 {
		t.Fatal("no style in the set is fitted to both appearances")
	}
	name := m.Styles[both].Name
	one := CaptionHintFor(ReduceModel(m, AdoptStyle{Index: both}))
	if strings.Count(one, name) != 1 {
		t.Errorf("a style that is its own counterpart reads %q, want its name once", one)
	}
	t.Logf("two members: %q; one member: %q", hint, one)
}

// TestTheRowSaysWhatTheShareIsAShareOf: the number under a swatch is a fraction
// of whatever the colours were counted out of. After a card is clicked that is
// a palette, and a line explaining a percentage of a picture — on a window
// where no picture was ever dropped — leaves the number standing for nothing.
func TestTheRowSaysWhatTheShareIsAShareOf(t *testing.T) {
	m := withStyles()
	fromStyle := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	if got := RowHintFor(fromStyle); strings.Contains(got, "picture") {
		t.Errorf("after a card was clicked the row says %q — there is no picture", got)
	}
	if got := RowHintFor(fromStyle); !strings.Contains(got, "palette") {
		t.Errorf("after a card was clicked the row says %q, want it to name the palette", got)
	}
	img := scene(240, 180)
	fromImage := ReduceModel(m, ImageLoaded{Path: "/tmp/harbour.png", Preview: preview(img), Candidates: imageseed.Extract(img)})
	if got := RowHintFor(fromImage); !strings.Contains(got, "picture") {
		t.Errorf("after a picture was dropped the row says %q, want it to name the picture", got)
	}
}

// TestThereIsAWayBackToTheGrid: a click on a card must not be a one-way door.
// The window offers to keep what is on screen, which is an offer that means
// nothing where there is no way not to — and the other forty cards are only
// reachable from the first screen.
func TestThereIsAWayBackToTheGrid(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	if _, ok := after.Seed(); !ok {
		t.Fatal("the fixture click did not dress the window")
	}
	back := ReduceModel(after, ShowStyles{})
	if _, ok := back.Seed(); ok {
		t.Error("going back left the window still wearing the style's seed")
	}
	if back.Style != "" || back.Name != "" || back.Preview != nil {
		t.Errorf("going back left the window crediting %q/%q", back.Style, back.Name)
	}
	// The pair was a separate choice and survives: coming back to look at more
	// styles is not a reason to un-choose how code is coloured.
	if back.AppliedBases() != after.AppliedBases() {
		t.Errorf("going back moved the syntax pair to %+v, want %+v", back.AppliedBases(), after.AppliedBases())
	}
	// And the first screen is really back: the grid is drawn where it was.
	img := page(t, back, tokens.DefaultLight)
	first := m.VisibleStyles(false)[0]
	p := leadSwatch(0)
	want := m.Styles[first].Seed()
	if got := img.RGBAAt(p.X, p.Y); got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("after going back the grid's first card at %v drew %v, want %v", p, got, want)
	}
}

// TestTheBaseListStillOverridesAnAdoptedPair: adopting is a starting point and
// not a lock. Picking a name off the list afterwards moves that appearance's
// member and nothing else — not the other member, and not the seed.
func TestTheBaseListStillOverridesAnAdoptedPair(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	seed, _ := after.Seed()
	adopted := after.AppliedBases()

	underSun := pick(after, "solarized-light", false)
	if underSun.Base(false) != "solarized-light" || underSun.Base(true) != adopted.Dark {
		t.Errorf("a press under the sun left the pair %+v, want the light member alone moved from %+v", underSun.AppliedBases(), adopted)
	}
	underMoon := pick(after, "monokai", true)
	if underMoon.Base(true) != "monokai" || underMoon.Base(false) != adopted.Light {
		t.Errorf("a press under the moon left the pair %+v, want the dark member alone moved from %+v", underMoon.AppliedBases(), adopted)
	}
	for _, m := range []Model{underSun, underMoon} {
		if got, _ := m.Seed(); got != seed {
			t.Errorf("overriding a base moved the seed to %v, want %v left alone", got, seed)
		}
	}
}

// TestAPictureReplacesAnAdoptedStyle: the two doors lead to one room, and what
// arrives last is what the window credits.
func TestAPictureReplacesAnAdoptedStyle(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	img := scene(240, 180)
	dropped := ReduceModel(after, ImageLoaded{Path: "/tmp/harbour.png", Preview: preview(img), Candidates: imageseed.Extract(img)})
	if dropped.Style != "" {
		t.Errorf("a picture landed and the window still credits the style %q", dropped.Style)
	}
	if dropped.Name != "harbour.png" {
		t.Errorf("the window credits %q, want the picture", dropped.Name)
	}
	// The bases stay: they were chosen, and a picture is not a choice about
	// how code is coloured.
	if dropped.AppliedBases() != after.AppliedBases() {
		t.Errorf("dropping a picture moved the syntax pair to %+v", dropped.AppliedBases())
	}
}

// TestKeepingAnAdoptedStyleReproducesIt: the exit's other half. What a click
// put on screen is written to the kept-theme file, and the next window to open
// on that file — this one, or any application that adopts a brand — comes back
// wearing the same colour and the same pair.
func TestKeepingAnAdoptedStyleReproducesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	m := withStyles()
	m.KeepPath = path
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	seed, _ := after.Seed()

	next, cmd := Update(after, KeepSeed{})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("the keep command failed: %v", err)
	}
	next = ReduceModel(next, msg)
	if !next.SeedIsKept() {
		t.Fatal("after keeping, the window does not know what is on screen is what is on disk")
	}

	onDisk := brand.KeptFrom(path)
	if onDisk.Seed != seed {
		t.Errorf("the file holds %v, want the card's leading ink %v", onDisk.Seed, seed)
	}
	if onDisk.Source != "dracula" {
		t.Errorf("the file credits %q, want the style the colour came out of", onDisk.Source)
	}
	if got := highlight.BasesOrDefault(onDisk.Base.Names()); got != after.AppliedBases() {
		t.Errorf("the file holds the pair %+v, want %+v", got, after.AppliedBases())
	}

	// What an adopting application does with that file: derive both schemes
	// from the seed it names, and resolve the pair it names.
	wantLight, wantDark := tokens.FromSeed(seed)
	gotLight, gotDark := onDisk.Colors()
	if gotLight != wantLight || gotDark != wantDark {
		t.Error("the theme that comes back is not the one that was kept")
	}
	// And this window reopening on it: the same pair applied, per appearance.
	reopened := withStyles().adoptKept(onDisk)
	if reopened.AppliedBases() != after.AppliedBases() {
		t.Errorf("a window reopening on the file applies %+v, want %+v", reopened.AppliedBases(), after.AppliedBases())
	}
	t.Logf("kept %s from %s with syntax base %s + %s, and it came back",
		hexOf(seed), onDisk.Source, onDisk.Base.Light, onDisk.Base.Dark)
}

// TestStyleGridDump writes the first screen and a window a card was clicked
// on, each in both schemes, for the same reason TestWindowDump writes its
// four: looking at a window is a review step and not a test. It skips unless
// -themer.dump names a directory.
func TestStyleGridDump(t *testing.T) {
	if *dumpDir == "" {
		t.Skip("themer: pass -themer.dump=DIR to write the window out")
	}
	m := withStyles()
	save := func(name string, img *image.RGBA) {
		t.Helper()
		path := filepath.Join(*dumpDir, "themer-"+name+".png")
		if err := golden.Save(path, img); err != nil {
			t.Fatalf("themer: save %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
	}
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		save("start-"+sc.name, page(t, ReduceModel(m, SetScheme{Dark: sc.dark}), sc.os))
	}
	// One card off each list, each shown under both appearances: what a click
	// did has to be legible from the side it was made on and from the other.
	for _, name := range []string{"github", "dracula"} {
		after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, name)})
		for _, sc := range []struct {
			label string
			dark  bool
			os    tokens.ColorTokens
		}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
			on := ReduceModel(after, SetScheme{Dark: sc.dark})
			save("clicked-"+name+"-"+sc.label, page(t, on, sc.os))
		}
	}
}

// TestAnAdoptedStyleStandsWhereThePictureWould: a window whose colours came out
// of a palette has no photograph to show, and an empty mat beside a name reads
// as a picture that failed to load. The style's own inks go there instead.
func TestAnAdoptedStyleStandsWhereThePictureWould(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	img := page(t, after, tokens.DefaultDark)
	mat := image.Pt(int(Pad)+int(ThumbPad)+6, headTop()+int(HeadH)/2)
	want := after.Candidates[0].Color
	got := img.RGBAAt(mat.X, mat.Y)
	if got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("the mat at %v drew %v, want the style's leading ink %v", mat, got, want)
	}
}

// BenchmarkStyleCards is what the first screen costs before it is drawn: every
// style's palette extracted, its pair completed and its chips derived, once.
func BenchmarkStyleCards(b *testing.B) {
	for b.Loop() {
		styleCards()
	}
}
