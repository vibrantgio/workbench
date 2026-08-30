package main

import (
	"flag"
	"image"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// dumpDir, when set, makes TestWindowDump write the window out as a PNG per
// scheme instead of skipping. It is a diagnostic and never a comparison.
//
//	go test . -themer.dump=/tmp/themer
var dumpDir = flag.String("themer.dump", "", "write the window to this directory, one PNG per scheme")

// TestWindowDump writes the window mid-flow — a picture loaded, a candidate
// applied, a syntax base chosen — in both schemes and on every tab, plus the
// Markdown tab scrolled to the specimen, where the code and the base selector
// sit side by side. It skips unless -themer.dump names a directory.
func TestWindowDump(t *testing.T) {
	if *dumpDir == "" {
		t.Skip("themer: pass -themer.dump=DIR to write the window out")
	}
	m := ReduceModel(withBases(), ImageLoaded{
		Path:       "harbour.png",
		Preview:    preview(scene(900, 600)),
		Candidates: imageseed.Extract(scene(900, 600)),
	})
	m = ReduceModel(m, SelectCandidate{Index: 1})
	// One pair, chosen once: a base off the sun's list and a base off the
	// moon's, each a recognisable name rather than whatever sorted first.
	// Nothing is picked again inside the loop
	// — what the two schemes show is the same theme, flipped.
	m = pick(pick(m, "github", false), "dracula", true)
	e, sel := newEmbed(), newBaseSelector()
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
	}{{"light", false}, {"dark", true}} {
		on := ReduceModel(m, SetScheme{Dark: sc.dark})
		for tab := range TabCount {
			e.state(tab).ScrollToStart()
			save(sc.name+"-"+strings.ToLower(TabLabels[tab]), pageOn(t, e, onTab(on, tab), tokens.DefaultLight, sel))
		}
		// And the specimen itself, which is several screens down the
		// Markdown tab and is the only place the base selector is on screen.
		md := onTab(on, TabMarkdown)
		if row := e.codeRow(SchemeFor(tokens.DefaultLight, md)); row > 0 {
			e.state(TabMarkdown).ScrollTo(row - 1) // the specimen's heading, then its body
		}
		save(sc.name+"-code", pageOn(t, e, md, tokens.DefaultLight, sel))
		save(sc.name+"-code-jetbrains", pageOn(t, e, pickMono(md, tokens.CodeFaceJetBrains), tokens.DefaultLight, sel))
	}
}

// The embedded page is asserted on the window's own pixels rather than on a
// stored image, for the reason the rest of this file's captures are: what
// matters is that a band of the window moved when the theme did, and a stored
// image would illustrate that without ever checking it.

// contrasting is a loaded picture whose candidate row holds colours nothing
// derived from one of them survives the other — the case a pick has to
// repaint the page for, rather than tint it.
func contrasting(t *testing.T) Model {
	t.Helper()
	m := dropped(t)
	m.Candidates = []imageseed.Candidate{
		candidate(fixtureRed, 0.4),
		candidate(fixtureBlue, 0.3),
		candidate(fixtureGrey, 0.3),
	}
	m.Selected = 0
	return m
}

// bandChange reports the share of pixels between rows y0 and y1 that differ
// between two captures, as a percentage.
func bandChange(a, b *image.RGBA, y0, y1 int) float64 {
	n, total := 0, 0
	for y := y0; y < y1; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			total++
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

// bands cuts the embedded page into horizontal strips, so a region of it that
// did not move can be found rather than averaged away.
func bands(n int) [][2]int {
	top, bottom := galleryTop(), galleryBottom()
	out := make([][2]int, n)
	for i := range out {
		out[i] = [2]int{top + i*(bottom-top)/n, top + (i+1)*(bottom-top)/n}
	}
	return out
}

// TestPickRepaintsTheEmbeddedPage: choosing another candidate redraws the
// page under the row, not just the swatch that was clicked. It is the whole
// judgment loop in one assertion — a pick that left the page alone would make
// the application a colour picker with a picture of a gallery under it.
func TestPickRepaintsTheEmbeddedPage(t *testing.T) {
	e := newEmbed()
	m := contrasting(t)
	first := pageOn(t, e, m, tokens.DefaultLight)
	second := pageOn(t, e, ReduceModel(m, SelectCandidate{Index: 1}), tokens.DefaultLight)
	pct := bandChange(first, second, galleryTop(), galleryBottom())
	t.Logf("a pick moved %.1f%% of the embedded page", pct)
	if pct < pickFloor {
		t.Errorf("choosing another candidate moved %.1f%% of the embedded page, want at least %.0f%% — the page is not following the chosen seed",
			pct, pickFloor)
	}
}

// pickFloor is how much of the page a pick has to move. It is well under the
// whole band: the neutral ramps a page mostly stands on carry no
// hue by derivation, so what a brand colour moves is the accented furniture,
// and that is a minority of any honest inventory. What it is not is nothing.
const pickFloor = 5.0

// TestTheSchemeSwitchRepaintsEveryBand: the switch is the other half of the
// judgment loop, and the sharper of the two. Light and dark invert the ground
// every section stands on, so no strip of the page can come out of the switch
// unchanged. One that does is a surface drawing itself from something other
// than the theme it was handed — the defect that hides on a page where
// everything around it changed correctly.
func TestTheSchemeSwitchRepaintsEveryBand(t *testing.T) {
	e := newEmbed()
	m := contrasting(t)
	light := pageOn(t, e, ReduceModel(m, SetScheme{Dark: false}), tokens.DefaultLight)
	dark := pageOn(t, e, ReduceModel(m, SetScheme{Dark: true}), tokens.DefaultLight)
	for i, band := range bands(8) {
		pct := bandChange(light, dark, band[0], band[1])
		t.Logf("band %d (y %d..%d): %.1f%% followed the switch", i, band[0], band[1], pct)
		if pct < schemeBandFloor {
			t.Errorf("band %d of the embedded page (y %d..%d) moved %.1f%% between light and dark, want at least %.0f%% — part of the page is pinned to a stale scheme",
				i, band[0], band[1], pct, schemeBandFloor)
		}
	}
}

// schemeBandFloor is how much of one strip has to move when the scheme flips.
// Every strip measures over 99% today, because the ground itself inverts; the
// floor is set just under that rather than at some cautious fraction, since a
// fraction is exactly what lets one stale panel hide inside a strip that
// changed everywhere else.
const schemeBandFloor = 90.0

// TestTheSchemeSwitchIsIndependentOfTheDesktop: the window's own switch has to
// be able to show the side the desktop is not set to, or half of every seed is
// unreachable without changing a system preference.
func TestTheSchemeSwitchIsIndependentOfTheDesktop(t *testing.T) {
	m := contrasting(t)
	for _, os := range []tokens.ColorTokens{tokens.DefaultLight, tokens.DefaultDark} {
		if got := SchemeFor(os, ReduceModel(m, SetScheme{Dark: true})); !isDark(got) {
			t.Error("with the switch on dark, the window resolved a light palette")
		}
		if got := SchemeFor(os, ReduceModel(m, SetScheme{Dark: false})); isDark(got) {
			t.Error("with the switch on light, the window resolved a dark palette")
		}
	}
}

// TestTheEmbeddedPageIsTheBiggestBand: the page is what the application is
// for, so it gets the room. The picture is a reference and takes a strip.
func TestTheEmbeddedPageIsTheBiggestBand(t *testing.T) {
	page := galleryBottom() - galleryTop()
	top := headBottom() // the window's top edge down to the foot of the identity strip
	if page <= top+int(CellH) {
		t.Errorf("the embedded page has %d dp against %d dp of picture and candidates — the thing being judged is not the biggest thing in the window",
			page, top+int(CellH))
	}
	if share := 100 * page / windowH; share < 50 {
		t.Errorf("the embedded page is %d%% of the window, want at least half of it", share)
	}
}

// TestAPickDoesNotRebuildTheInventory: what a pick costs must be a palette
// derivation and a frame. The reading sample is parsed once, and re-parsing
// it per pick — which is what rebuilding the inventory would mean — is the
// one thing on this page that could be felt.
func TestAPickDoesNotRebuildTheInventory(t *testing.T) {
	e := newEmbed()
	shaper := pinned().Shaper
	built := e.catalogue(shaper, tokens.DefaultTypography, highlight.DefaultBases())
	if built == nil {
		t.Fatal("the first render built no inventory")
	}
	e.catalogue(shaper, tokens.DefaultTypography, highlight.DefaultBases())
	e.catalogue(shaper, tokens.DefaultTypography, highlight.DefaultBases())
	if e.inv != built {
		t.Error("a change of palette rebuilt the inventory — the reading sample is being parsed again on every pick")
	}
}

// BenchmarkRetheme measures what a pick actually costs: the whole inventory
// rebuilt as section values in a new palette. It is not a gate, it is the
// number behind the claim that picking does not stall the window.
func BenchmarkRetheme(b *testing.B) {
	e := newEmbed()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	light, dark := tokens.FromSeed(fixtureBlue)
	inv := e.catalogue(shaper, tokens.DefaultTypography, highlight.DefaultBases())
	for i := 0; b.Loop(); i++ {
		c := light
		if i%2 == 1 {
			c = dark
		}
		inv.Items(c)
	}
}

// TestEveryGroupTabNamesALiveGroup: each of the three catalogue tabs names a
// group the published inventory actually carries, and the only group without
// a tab is Foundations — whose colour story the Theme tab tells in this
// window's own words, with this seed's provenance in it.
//
// Both halves matter. A group renamed upstream would leave its tab showing a
// blank surface, which is a defect somebody has to notice; a group added
// upstream would be a whole module of the design system the window silently
// stops showing. Named rather than counted, so the failure says which.
func TestEveryGroupTabNamesALiveGroup(t *testing.T) {
	e := newEmbed()
	inv := e.catalogue(pinned().Shaper, tokens.DefaultTypography, highlight.DefaultBases())
	c := tokens.DefaultLight
	shown := map[string]bool{"Foundations": true}
	for tab := TabComponents; tab < TabCount; tab++ {
		group := TabGroups[tab]
		shown[group] = true
		if rows := inv.TabItems(c, group); len(rows) == 0 {
			t.Errorf("the %s tab names group %q, which the inventory does not carry", TabLabels[tab], group)
		}
	}
	for _, grp := range inv.Groups(c) {
		if !shown[grp.Name] {
			t.Errorf("the inventory carries group %q and no tab shows it", grp.Name)
		}
	}
}

// TestTheWindowOpensOnTheTheme: the tab a window opens on is the one that
// answers the question it was opened with — what this colour makes.
func TestTheWindowOpensOnTheTheme(t *testing.T) {
	if got := (Model{}).Tab; got != TabTheme {
		t.Errorf("a fresh window opens on tab %d, want the Theme tab at %d", got, TabTheme)
	}
}

// TestTheTabSurvivesAPick: choosing another candidate re-derives every palette
// on screen, and a reader judging buttons is still judging buttons afterwards.
// A tab reset by a pick would make the row of swatches a control that undoes
// the reader's own navigation.
func TestTheTabSurvivesAPick(t *testing.T) {
	m := onTab(contrasting(t), TabComponents)
	for _, msg := range []any{
		SelectCandidate{Index: 1},
		SetScheme{Dark: true},
		SelectMono{Name: tokens.CodeFaceJetBrains},
	} {
		if got := ReduceModel(m, msg).Tab; got != TabComponents {
			t.Errorf("after %T the page is on tab %d, want the tab the reader was on (%d)", msg, got, TabComponents)
		}
	}
}

// TestEachTabKeepsItsOwnScroll: a tab scrolled into is a place the reader
// means to come back to. One shared scroll position would drop them at the
// other tab's offset every time they switched, which on surfaces of very
// different heights is a page that jumps under them.
func TestEachTabKeepsItsOwnScroll(t *testing.T) {
	e := newEmbed()
	m := judging()
	pageOn(t, e, onTab(m, TabComponents), tokens.DefaultLight)
	e.state(TabComponents).ScrollTo(4)
	pageOn(t, e, onTab(m, TabComponents), tokens.DefaultLight)
	was := e.state(TabComponents).Position()
	pageOn(t, e, onTab(m, TabPatterns), tokens.DefaultLight)
	if got := e.state(TabComponents).Position(); got.First != was.First {
		t.Errorf("reading another tab moved the Components tab from row %d to row %d", was.First, got.First)
	}
	if got := e.state(TabPatterns).Position(); got.First != 0 {
		t.Errorf("the Patterns tab opened at row %d, want the top of its own column", got.First)
	}
}
