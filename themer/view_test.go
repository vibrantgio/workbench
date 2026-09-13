package main

import (
	"image"
	stdcolor "image/color"
	"slices"
	"testing"

	"gioui.org/gesture"
	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// The window renders are captured, never stored: what is asserted is that
// every fill is the platform's name for what it is, and that is a pixel fact
// a stored image would only illustrate.

// pinned builds the application's typography with the default faces and
// nothing else, system fonts off, so a render here cannot depend on the
// machine's font set.
func pinned() Type {
	ty := TypeFrom(tokens.DefaultTypography)
	ty.Shaper = tokens.DefaultTypography.DeterministicShaper()
	return ty
}

// page renders the whole window for a model on one platform set and returns
// the capture. The colour field is a published component built from a theme
// stream; here it stands as the empty box it draws before a key is pressed,
// which is what the row has to make room for.
func page(t *testing.T, m Model, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	return pageAt(t, m, c, image.Pt(windowW, windowH))
}

// pageAt is page at a window size of the caller's choosing.
func pageAt(t *testing.T, m Model, c tokens.PlatformColors, size image.Point) *image.RGBA {
	t.Helper()
	return pageState(t, m, c, size, newCodeState())
}

// pageState is pageAt with the code section's own state in the caller's
// hands, so a test can read the column where it left it.
func pageState(t *testing.T, m Model, c tokens.PlatformColors, size image.Point, code *codeState) *image.RGBA {
	t.Helper()
	clicks := make([]gesture.Click, rowSlots)
	w := Page(themed{col: c, typ: pinned(), pinned: true}, m, &desktop.ZoneGroup{}, clicks, new(topClicks),
		list.NewState(), code, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(SampleFieldH))}
		})
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		// The backdrop is its own layer at runtime; here it is one fill under
		// the page, resolved the same way that layer resolves it.
		fillRect(gtx, image.Rectangle{Max: size}, c.WindowBackground)
		return w(gtx)
	})
}

// is reports whether a captured pixel is the colour named, which is an exact
// comparison: every colour this window hands the rasterizer is opaque.
func is(got stdcolor.RGBA, want stdcolor.NRGBA) bool {
	return got.R == want.R && got.G == want.G && got.B == want.B
}

// cellCentre is the middle of the nth swatch in a row of n cells, in window
// pixels: the row stands under the title row and the source row, and the
// swatch itself under the card's own padding.
func cellCentre(n, i int) image.Point {
	width := windowW - 2*int(Pad)
	w := cellWidth(width, int(CellGap), n)
	x := int(Pad) + i*(w+int(CellGap)) + w/2
	top := int(Pad) + int(TitleH) + int(Gap) + int(HeadH) + int(Gap) + int(RowLabelH) + int(RowTop)
	return image.Pt(x, top+int(CellPad)+int(SwatchH)/2)
}

// TestTheWindowStandsOnThePlatformsPlane: the window's own fill is
// windowBackground in both appearances, because this window follows the
// desktop's setting like every other application.
func TestTheWindowStandsOnThePlatformsPlane(t *testing.T) {
	for _, c := range []tokens.PlatformColors{tokens.PlatformLight, tokens.PlatformDark} {
		img := page(t, judging(), c)
		// A point in the margin, where nothing else is drawn.
		got := img.RGBAAt(windowW/2, int(Pad)/2)
		if !is(got, c.WindowBackground) {
			t.Errorf("the window's plane drew %v, want the platform's %v", got, c.WindowBackground)
		}
	}
}

// TestThePlatformsColourLeadsTheRow: the leading swatch is the accent colour
// the platform reports, drawn as its own colour, so what is offered is the
// setting in force and not a description of it.
func TestThePlatformsColourLeadsTheRow(t *testing.T) {
	m := dropped(t)
	n := len(m.Candidates) + 1
	img := page(t, m, tokens.PlatformLight)
	p := cellCentre(n, systemSlot)
	if got := img.RGBAAt(p.X, p.Y); !is(got, fixturePlatform) {
		t.Errorf("the leading swatch at %v drew %v, want the platform's %v", p, got, fixturePlatform)
	}
}

// TestNoCellWhereThePlatformReportsNoColour: a desktop that publishes no
// colour of its own is offered no choice of it, and the row is the picture's
// colours alone.
func TestNoCellWhereThePlatformReportsNoColour(t *testing.T) {
	m := dropped(t)
	m.Platform = stdcolor.NRGBA{}
	img := page(t, m, tokens.PlatformLight)
	p := cellCentre(len(m.Candidates), 0)
	if got := img.RGBAAt(p.X, p.Y); !is(got, m.Candidates[0].Color) {
		t.Errorf("the leading swatch at %v drew %v, want the picture's own %v", p, got, m.Candidates[0].Color)
	}
}

// TestTheChosenSwatchWearsThePlatformsSelection: a card the window is
// standing on is marked the way the platform marks a chosen row — the
// emphasized selection fill — rather than by a rim of its own.
func TestTheChosenSwatchWearsThePlatformsSelection(t *testing.T) {
	m := ReduceModel(dropped(t), SelectCandidate{Index: 1})
	n := len(m.Candidates) + 1
	c := tokens.PlatformLight
	img := page(t, m, c)
	// The card's fill shows between its own edge and the swatch inside it.
	p := cellCentre(n, 2)
	got := img.RGBAAt(p.X, p.Y-int(SwatchH)/2-int(CellPad)/2)
	if !is(got, c.SelectedContentBackground) {
		t.Errorf("the chosen card drew %v at %v, want the platform's selection %v", got, p, c.SelectedContentBackground)
	}
}

// TestThePreviewCarriesTheThemeColour: the preview's default button is the
// one control the platform fills with its accent, so the colour chosen is
// what is standing there.
func TestThePreviewCarriesTheThemeColour(t *testing.T) {
	want := stdcolor.NRGBA{R: 0xe8, G: 0x11, B: 0x2d, A: 0xff}
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	img := page(t, m, tokens.PlatformLight)
	if !found(img, want) {
		t.Errorf("the theme colour %v is nowhere in the window — the preview is not wearing it", want)
	}
}

// TestThePreviewSwitchesSides: the dark preview draws the dark set's plane
// while the window around it stays light, which is what a window that follows
// the desktop's setting and previews the other side looks like.
func TestThePreviewSwitchesSides(t *testing.T) {
	m := ReduceModel(judging(), SetScheme{Dark: true})
	img := page(t, m, tokens.PlatformLight)
	if !found(img, tokens.PlatformDark.WindowBackground) {
		t.Error("the dark preview did not draw the dark set's plane")
	}
	if got := img.RGBAAt(windowW/2, int(Pad)/2); !is(got, tokens.PlatformLight.WindowBackground) {
		t.Errorf("the window around the preview drew %v, want the light set's plane", got)
	}
}

// found reports whether a colour is drawn anywhere in the capture.
func found(img *image.RGBA, want stdcolor.NRGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if is(img.RGBAAt(x, y), want) {
				return true
			}
		}
	}
	return false
}

// TestTheTwoCodeChoicesAreOnScreen: the window carries both choices it makes
// about code, under either appearance — the face plate with the chosen name
// marked, the base column with the applied base marked, and beside them the
// fence wearing that appearance's own member of the pair.
//
// It is the assertion that the section is on screen at all. Every other test
// in the package reads a part of it; this one is the composition, and it is
// run twice because a theme's code has two appearances and the window shows
// one of them at a time.
func TestTheTwoCodeChoicesAreOnScreen(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
		set  tokens.PlatformColors
	}{{"under the sun", false, tokens.PlatformLight}, {"under the moon", true, tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			m := ReduceModel(judging(), SetScheme{Dark: tc.dark})
			p := PaletteFrom(tc.set)
			code := newCodeState()
			img := pageWith(t, m, tc.set, code)

			// The face plate: the applied name wears the platform's selection.
			chosen := slices.Index(codeFaces, m.AppliedMono())
			if chosen < 0 {
				t.Fatalf("the applied face %q is not one the plate offers", m.AppliedMono())
			}
			at := image.Pt(rowFillX(), faceRowY(chosen))
			if got := img.RGBAAt(at.X, at.Y); !is(got, p.Selection) {
				t.Errorf("the chosen face's row at %v drew %v, want the platform's selection %v", at, got, p.Selection)
			}
			if got := img.RGBAAt(at.X, faceRowY(1-chosen)); is(got, p.Selection) {
				t.Error("both faces are marked — the plate is not saying which one is applied")
			}

			// The base column: the applied base is brought into view and
			// marked, which is where the chooser's own reveal puts it.
			visible := m.VisibleBases(tc.dark)
			row := slices.Index(visible, m.BaseAt(tc.dark))
			if row < 0 {
				t.Fatalf("the applied base is not on the list this appearance shows")
			}
			on := row - max(0, row-baseLead)
			if got := img.RGBAAt(rowFillX(), baseRowY(on)); !is(got, p.Selection) {
				t.Errorf("the applied base's row drew %v, want the platform's selection %v", got, p.Selection)
			}

			// And the fence beside them wears that appearance's own member.
			fence := CodeStyle(PreviewSet(tc.set, m, tc.dark), tokens.DefaultTypography, m.AppliedBases()).CodeBackground
			if !found(img, fence) {
				t.Errorf("the fence's own background %v (from %q) is nowhere in the window", fence, m.Base(tc.dark))
			}
		})
	}
}
