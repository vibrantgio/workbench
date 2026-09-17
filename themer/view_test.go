package main

import (
	"image"
	stdcolor "image/color"
	"slices"
	"testing"

	"gioui.org/gesture"
	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/theme/tokens"
)

// Most of the renders here are captured and not stored: what they assert is
// that every fill is the platform's name for what it is, and that is a pixel
// fact a stored image would only illustrate. The exception is the whole
// window, which is stored in both appearances — the composition is the one
// thing about this page no pixel assertion states, and it is what a reviewer
// is given.

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
		code, func(gtx layout.Context) layout.Dimensions {
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
// pixels: the cards stand in the picture group's box, in the run the mat left,
// and the swatch itself under the card's own padding.
func cardsLead() int { return boxLead() + int(ThumbW) + int(BoxInset) }

func cellCentre(n, i int) image.Point {
	width := boxTrail() - cardsLead()
	w := cellWidth(width, int(CellGap), n)
	x := cardsLead() + i*(w+int(CellGap)) + w/2
	top := pictureBoxTop() + int(BoxPad)
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

// TestTheChosenSwatchWearsTheAccentRing: the colour the window is standing on
// is marked the way the platform marks a chosen thumbnail — a ring of the
// accent round the swatch, a point of the box's fill between them — and by
// nothing else. The swatch itself does not move and the caption does not
// change colour.
func TestTheChosenSwatchWearsTheAccentRing(t *testing.T) {
	plain := dropped(t)
	m := ReduceModel(plain, SelectCandidate{Index: 1})
	n := len(m.Candidates) + 1
	c := tokens.PlatformLight
	img := page(t, m, c)
	p := PaletteFrom(c)

	// The ring stands MarkGutter clear of the swatch's own top edge, so it is
	// read half a ring above that.
	centre := cellCentre(n, 2)
	top := centre.Y - int(SwatchH)/2
	ringY := top - int(MarkGutter) - int(MarkRing)
	if got := img.RGBAAt(centre.X, ringY); !is(got, p.Accent) {
		t.Errorf("the chosen swatch's ring at (%d,%d) drew %v, want the platform's accent %v",
			centre.X, ringY, got, p.Accent)
	}
	// And the ring stops short of the swatch: the row against the swatch's
	// own top edge is not the accent.
	if got := img.RGBAAt(centre.X, top-1); is(got, p.Accent) {
		t.Error("the ring runs up against the swatch — the gutter between them is not drawn")
	}
	// The swatch is where an unchosen one is, to the pixel.
	before := page(t, plain, c)
	if got, want := img.RGBAAt(centre.X, centre.Y), before.RGBAAt(centre.X, centre.Y); got != want {
		t.Errorf("choosing the colour moved the swatch under it: %v became %v", want, got)
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

			// The code face group: the applied name wears the platform's
			// selection and the other does not.
			chosen := slices.Index(codeFaces, m.AppliedMono())
			if chosen < 0 {
				t.Fatalf("the applied face %q is not one the group offers", m.AppliedMono())
			}
			at := image.Pt(faceFillX(chosen), faceRowY())
			if got := img.RGBAAt(at.X, at.Y); !is(got, p.Selection) {
				t.Errorf("the chosen face's row at %v drew %v, want the platform's selection %v", at, got, p.Selection)
			}
			if got := img.RGBAAt(faceFillX(1-chosen), faceRowY()); is(got, p.Selection) {
				t.Error("both faces are marked — the group is not saying which one is applied")
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

// columnRigid is every part of the page's column that states its own height:
// everything but the preview's box, which takes what is left over.
func columnRigid() int {
	group := func(box int) int {
		return int(GroupTitleH) + int(GroupBelow) + box + int(GroupAbove)
	}
	return int(TitleH) + int(Gap) +
		group(int(PictureBoxH)) + group(int(ColourBoxH)) +
		group(int(FaceBoxH)) + group(int(BaseBoxH)) +
		group(0) + int(FooterH)
}

// sampleOrigin is the top-leading corner of the nth sample window in the
// preview's box, worked out the way [Preview] centres the pair in whatever
// room the box has.
func sampleOrigin(i int) image.Point {
	roomX := boxTrail() - boxLead()
	roomY := previewBoxBottom() - previewBoxTop() - 2*int(BoxPad)
	both := 2*int(SampleW) + int(Gap)
	return image.Pt(
		boxLead()+max(0, (roomX-both)/2)+i*(int(SampleW)+int(Gap)),
		previewBoxTop()+int(BoxPad)+max(0, (roomY-int(SampleH))/2),
	)
}

// TestTheWholeColumnFitsTheWindow: every choice, the preview and the footer
// stand in the window the application opens, with no scroller anywhere in the
// page. The column is a stack of stated heights with one flexed box in it, so
// what has to be checked is that the flexed box is still big enough for the
// thing it holds: a box smaller than the sample would crop the preview rather
// than report anything.
func TestTheWholeColumnFitsTheWindow(t *testing.T) {
	left := windowH - 2*int(Pad) - columnRigid()
	if want := int(SampleH) + 2*int(BoxPad); left < want {
		t.Fatalf("the column's stated parts leave %d points for the preview's box and the sample wants %d — the page no longer closes in a %d by %d window",
			left, want, windowW, windowH)
	}
	if got := previewBoxBottom() - previewBoxTop(); got != left {
		t.Errorf("the preview's box measures %d points and the column leaves %d", got, left)
	}
	t.Logf("the preview's box is %d points tall and the sample is %d", left, int(SampleH))
}

// TestBothAppearancesStandSideBySide: the preview draws the sample twice, the
// light set leading and the dark one beside it, each at its own fixed size.
// One appearance at a time behind a switch makes judging a colour on both two
// looks and a memory; this makes it one look.
func TestBothAppearancesStandSideBySide(t *testing.T) {
	img := page(t, judging(), tokens.PlatformLight)
	for i, want := range []tokens.PlatformColors{tokens.PlatformLight, tokens.PlatformDark} {
		o := sampleOrigin(i)
		// A point in the sample's own title band, clear of the word centred
		// in it: the band is the chrome material of the set being previewed.
		got := img.RGBAAt(o.X+int(SampleW)/4, o.Y+int(SampleTitleH)/2)
		if !is(got, want.SidebarMaterial) {
			t.Errorf("sample %d drew %v in its title band, want the %s set's chrome material %v",
				i, got, [2]string{"light", "dark"}[i], want.SidebarMaterial)
		}
	}
}

// TestTheSamplesOneSelectionIsTheRailsPill: the picture of an application
// carries one selection and it is the platform's sidebar pill — the rail's
// open entry, inset from the rail's edges and rounded. The content list
// beside it carries none: a full-width bar in the theme colour read as a slab
// dropped on the page, and two marks together said the colour lands in more
// places than it does.
func TestTheSamplesOneSelectionIsTheRailsPill(t *testing.T) {
	const themeColor = "#3d5f57"
	m := ReduceModel(judging(), HexTyped{Text: themeColor})
	img := page(t, m, tokens.PlatformLight)
	set := PreviewSet(tokens.PlatformLight, m, false)
	pill := sidebar.SelectionFill(set, false)
	o := sampleOrigin(0)

	// The pill is inset from the rail's leading edge: the rail's own
	// chrome material shows beside it at the selected row's centre line.
	rowY := o.Y + int(SampleTitleH) + int(SampleRowH) + int(SampleRowH)/2
	if got := img.RGBAAt(o.X+int(sidebar.SelectionInset)/2, rowY); !is(got, set.SidebarMaterial) {
		t.Errorf("the rail drew %v inside the pill's own inset, want the chrome material %v — the mark is not inset",
			got, set.SidebarMaterial)
	}
	if got := img.RGBAAt(o.X+int(SampleRailW)/2, rowY); !is(got, pill) {
		t.Errorf("the rail's open entry drew %v, want the platform's sidebar pill %v", got, pill)
	}

	// And nothing in the sample wears the content list's emphasized
	// selection, which is the fill the theme colour used to land in as a
	// full-width box beside the pill.
	box := set.SelectedContentBackground
	for y := o.Y; y < o.Y+int(SampleH); y++ {
		for x := o.X + int(SampleRailW); x < o.X+int(SampleW); x++ {
			if is(img.RGBAAt(x, y), box) {
				t.Fatalf("the sample's content pane draws the emphasized selection %v at (%d,%d) — the green box is back", box, x, y)
			}
		}
	}
}

// goldenBases is the list of syntax palettes the stored window shows, written
// down here rather than read off the highlighting package's registry. Another
// test in this package loads a style out of a folder into that registry, and a
// stored image must not depend on which tests ran before it. Every name is one
// that ships embedded, so the fence they colour resolves the same way the
// window's own list resolves it.
var goldenBases = []string{"borland", "bw", "catppuccin-latte", "catppuccin-mocha", "colorful", "dracula", "emacs", "monokai"}

// goldenModel is the window as the stored image shows it: a picture dropped
// and its colours extracted, on the fixed base list above.
func goldenModel(t *testing.T, dark bool) Model {
	t.Helper()
	m := dropped(t)
	m.Bases = make([]BaseOption, len(goldenBases))
	for i, name := range goldenBases {
		m.Bases[i] = BaseOption{
			Name:  name,
			Light: highlight.BaseSuits(name, false),
			Dark:  highlight.BaseSuits(name, true),
		}
	}
	m.LightAt = baseIndex(m.Bases, highlight.DefaultBases().Light, false)
	m.DarkAt = baseIndex(m.Bases, highlight.DefaultBases().Dark, true)
	return ReduceModel(m, SetScheme{Dark: dark})
}

// TestTheWholeWindow stores the window as it opens, both appearances, at the
// size it opens at: the composition itself, which is the one thing about this
// window a pixel assertion cannot state. Every other test in the package
// reads a part of the page; this stores the whole of it, and a review of a
// window is a review of the window.
//
// Regenerate with:
//
//	go test ./ -run TestTheWholeWindow -golden.update
func TestTheWholeWindow(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			golden.Compare(t, "window-"+tc.name, pageWith(t, goldenModel(t, tc.dark), tc.set, newCodeState()))
		})
	}
}
