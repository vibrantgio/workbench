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
// of it. The button is the widest run of the palette's primary in the top
// bar — the scheme switch's selected segment is painted in the same colour
// and is a fraction of the width, which is what "widest" is for.
func keepBand(img *image.RGBA, primary stdcolor.NRGBA) (image.Rectangle, bool) {
	is := func(x, y int) bool {
		c := img.RGBAAt(x, y)
		return (stdcolor.NRGBA{c.R, c.G, c.B, 0xff}) == primary
	}
	top, bottom := int(Pad), int(Pad)+int(TopBarH)
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
