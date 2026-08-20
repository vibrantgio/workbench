package main

import (
	"image"
	stdcolor "image/color"
	"math"
	"testing"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// legibleFloor is WCAG AA for body text, the ratio a label has to reach over
// the colour it is drawn on. It is the floor the derivation itself now
// chooses on-colours against, and this file is where the window is held to
// it: a palette gate proves the tokens, and these prove that what the window
// paints out of them can be read.
const legibleFloor = 4.5

// TestCandidateChipsAreLegible measures every candidate chip in a rendered
// window: each card shows a colour extracted from the picture and, under it,
// the primary pair a palette derivation makes of that colour — the chip's
// whole job being to say whether the pair works. A chip whose own "Aa" cannot
// be read is the loudest possible answer to a question nobody asked.
//
// The picture is the fixture scene, whose extraction is where this defect was
// found: its sky and its foliage are light enough that the light scheme's
// primary used to come back under white text at 2.26–2.96:1, against a floor
// of 4.5.
func TestCandidateChipsAreLegible(t *testing.T) {
	m := dropped(t)
	img := page(t, m, tokens.DefaultLight)
	n := len(m.Candidates)
	for i, cand := range m.Candidates {
		at := chipBand(n, i)
		if at.Max.X > windowW-int(Pad) {
			break // a card the window is too narrow for is not drawn
		}
		pair, _ := tokens.FromSeed(cand.Color)
		fill, ink := inkOn(img, at)
		ratio := color.ContrastRatio(ink, fill)
		t.Logf("candidate %d %s: chip %v, label reaches %v, %.2f:1 (tokens: %v on %v, %.2f:1)",
			i, hexOf(cand.Color), fill, ink, ratio, pair.OnPrimary, pair.Primary,
			color.ContrastRatio(pair.OnPrimary, pair.Primary))
		if fill != stdcolor.NRGBA(pair.Primary) {
			t.Errorf("candidate %d: the chip is filled %v, want the derived primary %v", i, fill, pair.Primary)
		}
		if ratio < legibleFloor {
			t.Errorf("candidate %d (%s): its chip's label measures %.2f:1, under the %.1f:1 floor — the pair the card is showing off is unreadable",
				i, hexOf(cand.Color), ratio, legibleFloor)
		}
	}
}

// TestStyleChipsAreLegible measures the same thing on the grid, and measures
// it on every card rather than on the six a picture happens to produce: each
// card carries the primary pair its style's leading ink derives, under both
// appearances, and it carries it as a promise about what a click delivers. The
// sweep is over the model rather than over pixels because the chips are
// resolved once and drawn from — what a card shows and what a click applies
// come out of the same two colours.
func TestStyleChipsAreLegible(t *testing.T) {
	cards := styleCards()
	if len(cards) < 70 {
		t.Fatalf("only %d cards to measure", len(cards))
	}
	worst, worstAt := math.Inf(1), ""
	for _, s := range cards {
		for _, dark := range []bool{false, true} {
			chip := s.Chip(dark)
			ratio := color.ContrastRatio(chip.Ink, chip.Fill)
			if ratio < worst {
				worst, worstAt = ratio, s.Name+" under "+sideName(dark)
			}
			if ratio < legibleFloor {
				t.Errorf("%s (%s) under %s: its chip's label measures %.2f:1, under the %.1f:1 floor",
					s.Name, hexOf(s.Seed()), sideName(dark), ratio, legibleFloor)
			}
		}
	}
	t.Logf("the tightest chip on the grid is %s at %.2f:1", worstAt, worst)
}

// TestKeepButtonIsLegible measures the keep affordance in a rendered window,
// once per candidate. It is the published filled button under the chosen
// seed's own primary pair, so it fails exactly when a chip does — and it is
// the one control in the window that has to be read rather than looked at.
func TestKeepButtonIsLegible(t *testing.T) {
	m := dropped(t)
	for i, cand := range m.Candidates {
		chosen := ReduceModel(m, SelectCandidate{Index: i})
		c := SchemeFor(tokens.DefaultLight, chosen)
		img := page(t, chosen, tokens.DefaultLight)
		at, ok := keepBand(img, c.Primary)
		if !ok {
			t.Fatalf("candidate %d: no filled button found in the top bar", i)
		}
		fill, ink := inkOn(img, at)
		ratio := color.ContrastRatio(ink, fill)
		t.Logf("keep button on candidate %d %s: fill %v, label reaches %v, %.2f:1",
			i, hexOf(cand.Color), fill, ink, ratio)
		if fill != stdcolor.NRGBA(c.Primary) {
			t.Errorf("candidate %d: the keep button is filled %v, want the chosen seed's primary %v", i, fill, c.Primary)
		}
		if ratio < legibleFloor {
			t.Errorf("candidate %d (%s): the keep button's label measures %.2f:1, under the %.1f:1 floor",
				i, hexOf(cand.Color), ratio, legibleFloor)
		}
	}
}

// chipBand is the middle of candidate i's primary-pair chip, in a row of n:
// the band the label runs through, inset far enough from the chip's rounded
// corners that nothing behind it is in the sample. The arithmetic is the
// row's own, from the same constants Cell lays out with.
func chipBand(n, i int) image.Rectangle {
	x0 := cardX(n, i) + int(CellPad)
	x1 := cardX(n, i) + cardW(n) - int(CellPad)
	top := cardTop() + int(CellPad) + int(SwatchH) + 8
	mid := top + int(ChipH)/2
	w := (x1 - x0) / 4
	return image.Rect(x0+w, mid-6, x1-w, mid+6)
}

// keepBand finds the keep button in a rendered window and returns the middle
// of it. The button is the widest run of the palette's primary in the identity
// strip — nothing else there is painted in that colour at that width, and
// "widest" keeps the answer right even when something is.
func keepBand(img *image.RGBA, primary stdcolor.NRGBA) (image.Rectangle, bool) {
	is := func(x, y int) bool {
		c := img.RGBAAt(x, y)
		return (stdcolor.NRGBA{c.R, c.G, c.B, 0xff}) == primary
	}
	top, bottom := headTop(), headBottom()
	// The widest unbroken run of the fill anywhere in the bar is the
	// button's own width, found on one of the rows the label does not cross.
	best, at := 0, 0
	for y := top; y < bottom; y++ {
		for x, run := 0, 0; x < windowW; x++ {
			if !is(x, y) {
				run = 0
				continue
			}
			run++
			if run > best {
				best, at = run, x-run+1
			}
		}
	}
	if best < int(KeepW)/2 {
		return image.Rectangle{}, false
	}
	// How tall it is, counted rather than run: the rows the label crosses
	// are still mostly fill, they are just no longer one stretch of it.
	first, last := -1, -1
	for y := top; y < bottom; y++ {
		n := 0
		for x := at; x < at+best; x++ {
			if is(x, y) {
				n++
			}
		}
		if n < best/2 {
			continue
		}
		if first < 0 {
			first = y
		}
		last = y
	}
	w, mid := best/4, (first+last)/2
	return image.Rect(at+w, mid-6, at+best-w, mid+6), true
}

// inkOn reads a band of a render: the colour most of it is — the fill — and
// the pixel of the label furthest from that fill in relative luminance, which
// is the pixel the label's ink covers most. A band is used rather than a
// whole control so that a rounded corner never puts the surface behind it in
// the sample, where it would be mistaken for ink.
func inkOn(img *image.RGBA, at image.Rectangle) (fill, ink stdcolor.NRGBA) {
	counts := map[stdcolor.NRGBA]int{}
	for y := at.Min.Y; y < at.Max.Y; y++ {
		for x := at.Min.X; x < at.Max.X; x++ {
			c := img.RGBAAt(x, y)
			counts[stdcolor.NRGBA{c.R, c.G, c.B, 0xff}]++
		}
	}
	best := 0
	for c, n := range counts {
		if n > best {
			fill, best = c, n
		}
	}
	ink = fill
	fl := color.RelativeLuminance(fill)
	for c := range counts {
		if math.Abs(color.RelativeLuminance(c)-fl) > math.Abs(color.RelativeLuminance(ink)-fl) {
			ink = c
		}
	}
	return fill, ink
}

// boundaryFloor is how far apart a swatch's frame has to be from the colour on
// either side of it. It is nowhere near a text floor and is not meant to be:
// what it guards is that there is a line there at all, drawn at its own
// strength rather than smeared across two rows of antialiasing.
const boundaryFloor = 2.0

// TestANearWhiteSwatchKeepsItsBoundary: a style's palest ink, drawn at the end
// of a strip on a card that is itself pale, has to stay a colour somebody chose
// rather than becoming the place the strip appears to stop. The frame round the
// strip is what does that, and this measures it on the styles that actually
// carry a near-white ink — in both schemes, since the pale card and the pale
// ink swap which of them is the surprise.
func TestANearWhiteSwatchKeepsItsBoundary(t *testing.T) {
	m := withStyles()
	for _, tc := range []struct {
		scheme string
		dark   bool
		os     tokens.ColorTokens
		styles []string
	}{
		{"light", false, tokens.DefaultLight, []string{"tango", "rainbow_dash", "github"}},
		{"dark", true, tokens.DefaultDark, []string{"swapoff", "hr_high_contrast", "hrdark"}},
	} {
		on := ReduceModel(m, SetScheme{Dark: tc.dark})
		img := page(t, on, tc.os)
		visible := on.VisibleStyles(tc.dark)
		for _, name := range tc.styles {
			at := -1
			for n, i := range visible {
				if on.Styles[i].Name == name {
					at = n
				}
			}
			if at < 0 {
				t.Errorf("%s is not on the %s grid", name, tc.scheme)
				continue
			}
			cols := gridCols()
			right := int(Pad) + (at%cols)*(styleCellW()+int(StyleGap)) + styleCellW()
			y := leadSwatch(at).Y
			if y > windowH-int(Pad) {
				continue // a row the window is too short for is not drawn
			}
			band := opaque(img.RGBAAt(right-int(StylePad)-4, y))
			frame := opaque(img.RGBAAt(right-int(StylePad)-1, y))
			card := opaque(img.RGBAAt(right-int(StylePad)+2, y))
			inner := color.ContrastRatio(band, frame)
			outer := color.ContrastRatio(frame, card)
			t.Logf("%s %s: band %v | frame %v | card %v — %.2f:1 inside, %.2f:1 outside",
				tc.scheme, name, band, frame, card, inner, outer)
			if inner < boundaryFloor || outer < boundaryFloor {
				t.Errorf("%s %s: the strip's trailing edge measures %.2f:1 against the ink and %.2f:1 against the card, want %.1f:1 either side — the band has no boundary and the strip reads as one that stopped short",
					tc.scheme, name, inner, outer, boundaryFloor)
			}
		}
	}
}

// opaque reads a captured pixel back as the colour it was painted from.
func opaque(c stdcolor.RGBA) stdcolor.NRGBA {
	return stdcolor.NRGBA{R: c.R, G: c.G, B: c.B, A: 0xff}
}
