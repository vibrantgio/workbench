package main

import (
	"flag"
	"image"
	"path/filepath"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// dumpDir, when set, makes TestWindowDump write the window out as a PNG per
// scheme instead of skipping. It is a diagnostic and never a comparison: what
// it is for is looking at the window with fresh eyes, which is a review step
// and not a test.
//
//	go test . -themer.dump=/tmp/themer
var dumpDir = flag.String("themer.dump", "", "write the window to this directory, one PNG per scheme")

// TestWindowDump writes the window mid-flow — a picture loaded, a candidate
// applied, the page drawn in it — in both schemes. It skips unless
// -themer.dump names a directory.
func TestWindowDump(t *testing.T) {
	if *dumpDir == "" {
		t.Skip("themer: pass -themer.dump=DIR to write the window out")
	}
	m := ReduceModel(Model{}, ImageLoaded{
		Path:       "harbour.png",
		Preview:    preview(scene(900, 600)),
		Candidates: imageseed.Extract(scene(900, 600)),
	})
	m = ReduceModel(m, SelectCandidate{Index: 1})
	e := newEmbed()
	for _, sc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		img := pageOn(t, e, ReduceModel(m, SetScheme{Dark: sc.dark}), tokens.DefaultLight)
		path := filepath.Join(*dumpDir, "themer-"+sc.name+".png")
		if err := golden.Save(path, img); err != nil {
			t.Fatalf("themer: save %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
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
// whole band on purpose: the neutral ramps a page mostly stands on carry no
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
	if page <= int(TopBarH)+int(CellH) {
		t.Errorf("the embedded page has %d dp against %d dp of picture and candidates — the thing being judged is not the biggest thing in the window",
			page, int(TopBarH)+int(CellH))
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
	e.items(shaper, tokens.DefaultLight)
	built := e.inv
	if built == nil {
		t.Fatal("the first render built no inventory")
	}
	light, dark := tokens.FromSeed(fixtureBlue)
	e.items(shaper, light)
	e.items(shaper, dark)
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
	e.items(shaper, light)
	for i := 0; b.Loop(); i++ {
		c := light
		if i%2 == 1 {
			c = dark
		}
		e.items(shaper, c)
	}
}
