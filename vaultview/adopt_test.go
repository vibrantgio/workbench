package main

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	"github.com/vibrantgio/theme/tokens"
)

// harbourRed is the kept brand these tests adopt: nothing like the default
// seed, so a surface that failed to follow it is visible as itself.
var harbourRed = color.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}

// fixedAppearance is a desktop that always reports the same thing, so what
// these tests assert cannot depend on the machine they run on.
type fixedAppearance struct{ a specsystem.Appearance }

func (f fixedAppearance) Read() (specsystem.Appearance, error) { return f.a, nil }

// TestAKeptBrandDressesTheWholeWindow is the adoption proof. It builds the
// theme stream from exactly the expression the application builds its own
// from — a kept brand's options over the live bridge — renders the whole
// window in what that stream emits, and requires the default seed's accent
// to be gone from every pixel: a window that adopts a brand adopts it
// everywhere, or the adoption is a decoration on one panel.
//
// Both sides are checked because a kept brand pins a pair, not a colour,
// and the desktop still chooses between them.
func TestAKeptBrandDressesTheWholeWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := brand.SaveTo(path, brand.Brand{Seed: harbourRed, Source: "harbour.jpg"}); err != nil {
		t.Fatalf("keep: %v", err)
	}
	opts := brand.KeptFrom(path).Options()

	for _, tc := range []struct {
		name     string
		desktop  specsystem.Appearance
		fallback tokens.ColorTokens
	}{
		{"light", specsystem.Appearance{}, tokens.DefaultLight},
		{"dark", specsystem.Appearance{Dark: true}, tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th, err := specsystem.FromSourceTheme(fixedAppearance{tc.desktop}, time.Hour, opts...).First()
			if err != nil {
				t.Fatalf("theme: %v", err)
			}
			adopted, err := th.Color.First()
			if err != nil {
				t.Fatalf("colours: %v", err)
			}
			if adopted == tc.fallback {
				t.Fatal("the stream emitted the default palette with a brand kept")
			}

			before := window(t, tc.fallback)
			after := window(t, adopted)
			if pixels(before, tc.fallback.Primary) == 0 {
				t.Fatal("this window shows none of its accent, so it cannot show that it changed")
			}
			if n := pixels(after, tc.fallback.Primary); n != 0 {
				t.Errorf("%d pixels are still the default seed's accent while a brand is kept", n)
			}
			if pixels(after, adopted.Primary) == 0 {
				t.Error("the kept brand's accent is nowhere in the window")
			}
		})
	}
}

// TestTheKeptBasesColourTheCode is the adoption proof for the other half of a
// kept theme. The colour a person chose dresses the window; the syntax bases
// they chose beside it colour the code in it, one per appearance, and this
// asserts that what reaches a fence here is the same derivation the window that
// offered them was showing — ink for ink, not merely "some highlighting", and
// through the appearance's own member rather than through whichever one was
// named first.
//
// The comparison is against the derivation done directly from the kept names,
// because that is what "reproduces it" means: the file names a pair, and two
// applications deriving from that pair on one palette must land on the same
// colours or the names are a suggestion.
func TestTheKeptBasesColourTheCode(t *testing.T) {
	// Two styles that are nothing to do with each other, and nothing like the
	// default in either appearance.
	keptPair := brand.BasePair{Light: "solarized-light", Dark: "monokai"}
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := brand.SaveTo(path, brand.Brand{Seed: harbourRed, Base: keptPair, Source: "harbour.jpg"}); err != nil {
		t.Fatalf("keep: %v", err)
	}
	kept := brand.KeptFrom(path)
	if kept.Base != keptPair {
		t.Fatalf("the kept theme names %+v, want %+v", kept.Base, keptPair)
	}
	defer restoreCodeBases(noteCodeBases)
	noteCodeBases = adoptCodeBases(kept)
	want := highlight.BasePair{Light: keptPair.Light, Dark: keptPair.Dark}
	if noteCodeBases != want {
		t.Fatalf("this window adopted %+v, want the kept %+v", noteCodeBases, want)
	}

	const src = "// greet is a greeting.\nfunc greet(name string) string {\n\treturn fmt.Sprintf(\"hello, %s\", name)\n}\n"
	light, dark := kept.Colors()
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
		member string
		other  string
	}{
		{"light", light, want.Light, want.Dark},
		{"dark", dark, want.Dark, want.Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := noteStyle(tc.colors, tokens.DefaultTypography).Highlight("go", src)
			same := highlight.AdaptPair(want, tc.colors)("go", src)
			if len(got) == 0 {
				t.Fatal("the note's fence came back with no runs at all")
			}
			if len(got) != len(same) {
				t.Fatalf("the fence split into %d runs, the same derivation gives %d", len(got), len(same))
			}
			coloured := 0
			for i := range got {
				if got[i] != same[i] {
					t.Fatalf("run %d is %+v, the same derivation gives %+v", i, got[i], same[i])
				}
				if got[i].Color.A != 0 {
					coloured++
				}
			}
			if coloured == 0 {
				t.Fatal("no run carries a colour, so matching proves nothing")
			}
			// And it is this appearance's own member that got there: the same
			// runs, ink for ink, as that member derived alone — and not the
			// other member's, which is what a window colouring both appearances
			// through one name would have produced.
			member := highlight.Adapt(tc.member, tc.colors)("go", src)
			apart := 0
			for i := range got {
				if i >= len(member) || got[i].Color != member[i].Color {
					t.Fatalf("run %d is %v, %s alone gives %v", i, got[i].Color, tc.member, member[i].Color)
				}
			}
			other := highlight.Adapt(tc.other, tc.colors)("go", src)
			for i := range got {
				if i >= len(other) || got[i].Color != other[i].Color {
					apart++
				}
			}
			if apart == 0 {
				t.Fatalf("the fence is coloured exactly as %s would colour it — the pair is not being applied per appearance", tc.other)
			}
			t.Logf("%d runs, %d coloured, ink for ink %s's; %d runs unlike %s's", len(got), coloured, tc.member, apart, tc.other)
			// And it is the kept pair and not the default that got there.
			fallback := highlight.AdaptPair(highlight.DefaultBases(), tc.colors)("go", src)
			for i := range got {
				if i >= len(fallback) || got[i].Color != fallback[i].Color {
					return
				}
			}
			t.Error("the fence is coloured exactly as the default pair would colour it — the kept names reached nothing")
		})
	}

	// The API agreeing is one thing; the pixels are the other. The window is
	// rendered under the kept pair and under the default, and the two differ
	// — so the names reach the screen and not just a style value.
	under := func(p highlight.BasePair) *image.RGBA {
		noteCodeBases = p
		return window(t, light)
	}
	if golden.PixelDiff(under(want), under(highlight.DefaultBases())) == 0 {
		t.Error("the window drew the same pixels under two different syntax bases")
	}
}

// restoreCodeBases puts the process-wide pair back after a test has moved it.
func restoreCodeBases(p highlight.BasePair) { noteCodeBases = p }

// TestAnUnknownKeptBaseFallsBackToTheDefault: a theme naming a style this
// build cannot resolve — one whose file has left the styles folder — colours
// code exactly as it does for somebody who never chose one. The alternative
// is a window that refuses to draw a fence because a preference went stale.
//
// The last case is the theme kept before a theme carried a pair: one name, no
// appearance attached, arriving in both members. The appearance it was fitted
// to keeps it and the other falls back, so a note opens in the base that was
// chosen under the light the person chose it in.
func TestAnUnknownKeptBaseFallsBackToTheDefault(t *testing.T) {
	defer restoreCodeBases(noteCodeBases)
	d := highlight.DefaultBases()
	for _, tc := range []struct {
		name string
		kept brand.BasePair
		want highlight.BasePair
	}{
		{"nothing kept", brand.BasePair{}, d},
		{"names nothing resolves", brand.BasePair{Light: "a-style-nobody-wrote", Dark: "another"}, d},
		{"one dark base, no appearance attached", brand.BasePair{Light: "monokai", Dark: "monokai"},
			highlight.BasePair{Light: d.Light, Dark: "monokai"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := adoptCodeBases(brand.Brand{Seed: harbourRed, Base: tc.kept}); got != tc.want {
				t.Errorf("a kept %+v was adopted as %+v, want %+v", tc.kept, got, tc.want)
			}
		})
	}
}

// TestWithNothingKeptTheWindowIsTheOneItAlwaysWas: adoption is optional, and
// a machine that never chose a brand must render what it always rendered.
func TestWithNothingKeptTheWindowIsTheOneItAlwaysWas(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "theme.json")
	th, err := specsystem.FromSourceTheme(fixedAppearance{}, time.Hour, brand.KeptFrom(absent).Options()...).First()
	if err != nil {
		t.Fatalf("theme: %v", err)
	}
	colors, err := th.Color.First()
	if err != nil {
		t.Fatalf("colours: %v", err)
	}
	if colors != tokens.DefaultLight {
		t.Fatal("with nothing kept the stream did not emit the default palette")
	}
	if golden.PixelDiff(window(t, colors), window(t, tokens.DefaultLight)) != 0 {
		t.Error("with nothing kept the window is not the one it always was")
	}
}

// window renders the whole vault window in one palette, through the same
// composition the window goldens record. It captures rather than stores:
// what is asserted here is which colours reach which surfaces, and the
// stored goldens stay on the canonical palette because adoption happens at
// runtime and is not baked into the application.
func window(t *testing.T, colors tokens.ColorTokens) *image.RGBA {
	t.Helper()
	w, _ := renderWindow(tokens.DefaultTypography.DeterministicShaper(), goldenModel(), colors,
		tokens.Spacing, sharpRadius, tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
	return golden.Capture(t, windowCanvasSize, scene(w, colors.Background))
}

// pixels counts how many pixels of img are exactly c, alpha ignored: a
// colour that survives compositing unblended is a surface painted in it.
func pixels(img *image.RGBA, c color.NRGBA) int {
	if img == nil {
		return 0
	}
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px := img.RGBAAt(x, y)
			if px.R == c.R && px.G == c.G && px.B == c.B {
				n++
			}
		}
	}
	return n
}
