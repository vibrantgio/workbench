package main

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/brand"
	themecolor "github.com/vibrantgio/theme/color"
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
			if n := pixelsOf(after, primaryRoleAnswers(adopted)); n == 0 {
				t.Error("none of the kept brand's own answers for the primary role are anywhere in the window")
			}
		})
	}
}

// primaryRoleAnswers is the palette's own set of legitimate pixels for the
// primary role, in this window: the pin, on its own, wherever a surface
// fills with it outright (the tree's active row and the outline's
// current-section pill both paint [tokens.RampSet.Primary]'s step 300
// directly — see tree.go and aside.go); and [tokens.ColorTokens.InkOn]'s
// answer for the two floors this window gates the role's ink at when it is
// drawn ON a page rather than filling one — [tokens.TextFloor] for the
// wikilinks a note's prose carries, [tokens.GraphicFloor] for a graphic
// mark such as a blockquote's bar. InkOn already returns the bare pin where
// it clears a floor and a walked ramp step where it does not, so this one
// list covers both without needing to know which side of the floor c falls
// on.
//
// "The window adopted the brand" no longer means the bare pin reaches every
// surface — AV1's gate means it may legitimately not — so this asks the
// palette what its own answers are instead of naming one byte and hoping
// every seed agrees with it.
func primaryRoleAnswers(c tokens.ColorTokens) []color.NRGBA {
	ground := c.SurfaceAt(tokens.Level0)
	return []color.NRGBA{
		c.Primary,
		c.InkOn(tokens.RolePrimary, ground, tokens.TextFloor),
		c.InkOn(tokens.RolePrimary, ground, tokens.GraphicFloor),
		c.Ramps.Primary.Step(300),
	}
}

// pixelsOf sums [pixels] over every colour in cs: how many pixels of img
// match ANY of a role's acceptable answers, rather than one named byte.
func pixelsOf(img *image.RGBA, cs []color.NRGBA) int {
	n := 0
	for _, c := range cs {
		n += pixels(img, c)
	}
	return n
}

// TestAPinThatClearsDressesTheWindowWithItself is the adoption proof's other
// direction. TestAKeptBrandDressesTheWholeWindow's harbourRed cannot exercise
// it: its light pin measures 4.27:1, under the text floor by design, so
// InkOn always walks off it there. A seed whose pin clears needs no walk,
// and this asserts the bare pin itself — not merely one of
// [primaryRoleAnswers] — reaches the window, because InkOn is required to
// hand it back unmodified once it reads on its own page.
func TestAPinThatClearsDressesTheWindowWithItself(t *testing.T) {
	// The default brand's own seed: its light pin measures 5.94:1 against
	// its own paper, clear of the 4.5:1 text floor, and its dark pin is
	// realized at a fixed depth that always clears — the shape this test is
	// written for on both sides of the appearance switch.
	seed := tokens.DefaultSeed
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := brand.SaveTo(path, brand.Brand{Seed: seed, Source: "clears.jpg"}); err != nil {
		t.Fatalf("keep: %v", err)
	}
	opts := brand.KeptFrom(path).Options()

	for _, tc := range []struct {
		name    string
		desktop specsystem.Appearance
	}{
		{"light", specsystem.Appearance{}},
		{"dark", specsystem.Appearance{Dark: true}},
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

			ground := adopted.SurfaceAt(tokens.Level0)
			if got := themecolor.ContrastRatio(adopted.Primary, ground); got < tokens.TextFloor {
				t.Fatalf("this seed's pin now measures %.2f:1 against its own page, under the %.1f:1 text floor — the test no longer reads the shape it was written for", got, tokens.TextFloor)
			}

			after := window(t, adopted)
			if pixels(after, adopted.Primary) == 0 {
				t.Error("the pin clears its own floor, and its colour is nowhere in the window")
			}
		})
	}
}

// worn is a fresh note style dressed in one pair under these tokens: what a
// second application holding the same pair puts under a fence, to compare a
// note's own fence against.
func worn(p highlight.BasePair, c tokens.ColorTokens) markdown.Style {
	st := markdown.FromTokens(c, tokens.DefaultTypography)
	highlight.WearPair(&st, p, c)
	return st
}

// wornAlone is [worn] for one name under both appearances: the plate somebody
// who chose that base and nothing else would be looking at.
func wornAlone(name string, c tokens.ColorTokens) markdown.Style {
	return worn(highlight.BasePair{Light: name, Dark: name}, c)
}

// plate is the three things a base puts on a fence and a comparison has to
// cover: the ground under the block, the ink plain code falls back to, and the
// colours the highlighter hands out run by run.
type plate struct {
	ground, body color.NRGBA
	runs         []markdown.CodeSpan
}

func plateOf(st markdown.Style, src string) plate {
	return plate{ground: st.CodeBackground, body: st.CodeColor, runs: st.Highlight("go", src)}
}

// unlike reports how far two plates are apart: whether the grounds differ, and
// how many runs are inked differently. Either alone is a visible difference, so
// a base that reaches the screen shows up in one or the other.
func (p plate) unlike(q plate) (grounds bool, runs int) {
	grounds = p.ground != q.ground || p.body != q.body
	for i := range p.runs {
		if i >= len(q.runs) || p.runs[i].Color != q.runs[i].Color {
			runs++
		}
	}
	return grounds, runs
}

// TestTheKeptBasesColourTheCode is the adoption proof for the other half of a
// kept theme. The colour a person chose dresses the window; the syntax bases
// they chose beside it draw the code in it, one per appearance, and this
// asserts that a fence here wears the same plate the window that offered them
// was showing — the base's own ground under the base's own inks, ink for ink
// and not merely "some highlighting", and through the appearance's own member
// rather than through whichever one was named first.
//
// The comparison is against the pair worn directly from the kept names,
// because that is what "reproduces it" means: the file names a pair, and two
// applications wearing that pair on one palette must land on the same plate or
// the names are a suggestion.
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
			got := plateOf(noteStyle(tc.colors, tokens.DefaultTypography), src)
			same := plateOf(worn(want, tc.colors), src)
			if len(got.runs) == 0 {
				t.Fatal("the note's fence came back with no runs at all")
			}
			if len(got.runs) != len(same.runs) {
				t.Fatalf("the fence split into %d runs, the same pair gives %d", len(got.runs), len(same.runs))
			}
			coloured := 0
			for i := range got.runs {
				if got.runs[i] != same.runs[i] {
					t.Fatalf("run %d is %+v, the same pair gives %+v", i, got.runs[i], same.runs[i])
				}
				if got.runs[i].Color.A != 0 {
					coloured++
				}
			}
			if coloured == 0 {
				t.Fatal("no run carries a colour, so matching proves nothing")
			}
			if got.ground != same.ground || got.body != same.body {
				t.Fatalf("the fence sits on %v under %v ink, the same pair gives %v under %v",
					got.ground, got.body, same.ground, same.body)
			}
			// And it is this appearance's own member that got there: the same
			// ground, the same body ink and the same runs as that member worn
			// alone — and not the other member's, which is what a window
			// drawing both appearances through one name would have produced.
			member := plateOf(wornAlone(tc.member, tc.colors), src)
			if grounds, apart := got.unlike(member); grounds || apart != 0 {
				t.Fatalf("the fence is not %s's plate: grounds differ=%v, %d runs inked differently", tc.member, grounds, apart)
			}
			otherGrounds, otherRuns := got.unlike(plateOf(wornAlone(tc.other, tc.colors), src))
			if !otherGrounds && otherRuns == 0 {
				t.Fatalf("the fence is drawn exactly as %s would draw it — the pair is not being applied per appearance", tc.other)
			}
			t.Logf("%d runs, %d coloured, %s's plate on %v; unlike %s's by ground=%v and %d runs",
				len(got.runs), coloured, tc.member, got.ground, tc.other, otherGrounds, otherRuns)
			// And it is the kept pair and not the default that got there.
			if grounds, apart := got.unlike(plateOf(worn(highlight.DefaultBases(), tc.colors), src)); !grounds && apart == 0 {
				t.Error("the fence is drawn exactly as the default pair would draw it — the kept names reached nothing")
			}
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
		tokens.Spacing, goldenRadius, tokens.DefaultTypography, tokens.Comfortable, unit.Dp(goldenLeading))
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
