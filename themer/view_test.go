package main

import (
	"image"
	stdcolor "image/color"
	"slices"
	"testing"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/tabs"
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
	// The scroll bar setting is stated, not read off the platform: a still
	// picture holds the overlay bar every desktop platform draws, whatever
	// the machine running the test asked for.
	return pageBars(t, m, c, size, code, desktop.ScrollBarsWhenScrolling)
}

// pageBars renders the whole window at one scroll bar setting.
func pageBars(t *testing.T, m Model, c tokens.PlatformColors, size image.Point, code *codeState, bars desktop.ScrollBarSetting) *image.RGBA {
	t.Helper()
	w := staticPage(m, c, code, bars, new(topClicks))
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		// The backdrop is its own layer at runtime; here it is one fill under
		// the page, resolved the same way that layer resolves it — through
		// the appearance the switch is on, not the desktop's.
		fillRect(gtx, image.Rectangle{Max: size}, WindowSet(c, m).WindowBackground)
		return w(gtx)
	})
}

// staticPage is the page for a model on one platform set, with the tab strip
// drawn by the pattern's still render at the model's tab. The colour field is
// the empty box it draws before a key is pressed.
func staticPage(m Model, c tokens.PlatformColors, code *codeState, bars desktop.ScrollBarSetting, bar *topClicks) layout.Widget {
	return staticPages(m, c, code, newThemePages(), bars, bar)
}

// staticPages is staticPage with the moved tabs' state in the caller's hands.
func staticPages(m Model, c tokens.PlatformColors, code *codeState, pages *themePages, bars desktop.ScrollBarSetting, bar *topClicks) layout.Widget {
	clicks := make([]gesture.Click, rowSlots)
	cells := new(tabCells)
	ty := pinned()
	strip := tabs.Render(ty.Shaper, tabs.Props{Tabs: cells.tabs(), Fill: tabFill}, m.Tab,
		WindowSet(c, m), tokens.Spacing, tokens.DefaultTypography.SystemFont, tokens.Comfortable)
	return Page(themed{col: c, typ: ty, pinned: true, scrollBars: bars}, m, &desktop.ZoneGroup{}, clicks, bar,
		code, pages, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(tokens.ComfortableTextFieldHeight)))}
		}, cells, strip)
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
// windowBackground in both appearances, because the plane a window stands on
// is the platform's name for it whichever appearance it is on.
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

// TestTheChosenSwatchHasTheMeasuredRing: the colour the window is standing
// on is marked the way System Settings marks its chosen accent colour swatch,
// and by nothing else. The reference measures that ring in
// `system-settings-grouped-box-{light,dark}.png`: 3 px of `#157efb`, opaque,
// with a 1 px gap of the box's fill between the ring and the swatch, in both
// schemes. The test reads the column through the swatch's middle, upwards
// from the swatch's top edge. The swatch itself does not move.
func TestTheChosenSwatchHasTheMeasuredRing(t *testing.T) {
	const ringW, gapW = 3, 1
	ring := stdcolor.NRGBA{R: 0x15, G: 0x7e, B: 0xfb, A: 0xff}
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
	}{{"light", tokens.PlatformLight}, {"dark", tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			plain := ReduceModel(dropped(t), SetScheme{Dark: tc.name == "dark"})
			m := ReduceModel(plain, SelectCandidate{Index: 1})
			n := len(m.Candidates) + 1
			img := page(t, m, tc.set)
			box := PaletteFrom(WindowSet(tc.set, m)).Surface

			centre := cellCentre(n, 2)
			top := centre.Y - int(SwatchH)/2
			y := top - 1
			gap := 0
			for ; is(img.RGBAAt(centre.X, y), box); y-- {
				gap++
			}
			width := 0
			for ; is(img.RGBAAt(centre.X, y), ring); y-- {
				width++
			}
			if gap != gapW {
				t.Errorf("the gap between the ring and the swatch is %d px of the box's fill, want %d", gap, gapW)
			}
			if width != ringW {
				t.Errorf("the ring is %d px of %v, want %d", width, ring, ringW)
			}
			if got := img.RGBAAt(centre.X, y); !is(got, box) {
				t.Errorf("past the ring at (%d,%d) the box drew %v, want its fill %v", centre.X, y, got, box)
			}
			// The swatch is where an unchosen one is, to the pixel.
			before := page(t, plain, tc.set)
			if got, want := img.RGBAAt(centre.X, centre.Y), before.RGBAAt(centre.X, centre.Y); got != want {
				t.Errorf("choosing the colour moved the swatch under it: %v became %v", want, got)
			}
		})
	}
}

// TestTheComponentsTabCarriesTheThemeColour: the Components tab draws the
// controls the platform fills with its accent, so the colour chosen is
// there. A filled control draws a highlight over the accent, so the tab is
// read for a red pixel and not for the exact colour.
func TestTheComponentsTabCarriesTheThemeColour(t *testing.T) {
	m := ReduceModel(judging(), HexTyped{Text: "#e8112d"})
	m.Tab = TabComponents
	img := page(t, m, tokens.PlatformLight)
	band := image.Rect(int(Pad), contentTop(), windowW-int(Pad), footerTop())
	red := 0
	for y := band.Min.Y; y < band.Max.Y; y++ {
		for x := band.Min.X; x < band.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R > 200 && c.G < 80 && c.B < 90 {
				red++
			}
		}
	}
	if red == 0 {
		t.Errorf("no red pixel on the Components tab in %v: no control is drawn in the theme colour #e8112d", band)
	}
}

// TestTheWindowSwitchesSides: pressing the switch for the appearance the
// desktop is not on moves the whole window to it: the window's margin, and
// the preview, which is drawn in the dark member of the style pair.
func TestTheWindowSwitchesSides(t *testing.T) {
	m := ReduceModel(onCode(judging()), SetScheme{Dark: true})
	img := page(t, m, tokens.PlatformLight)
	if got := img.RGBAAt(windowW/2, int(Pad)/2); !is(got, tokens.PlatformDark.WindowBackground) {
		t.Errorf("the window's own margin drew %v on a light desktop switched to dark, want the dark set's plane %v",
			got, tokens.PlatformDark.WindowBackground)
	}
	box := previewRect()
	if got, want := img.RGBAAt(box.Min.X+int(BoxRadius), box.Min.Y+int(BoxRadius)), previewFill(m, tokens.PlatformLight); !is(got, want) {
		t.Errorf("the preview drew %v, want the dark style's background %v", got, want)
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
// marked, the style column with the applied style marked, and beside them the
// fence with that appearance's own member applied of the pair.
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
			m := ReduceModel(onCode(judging()), SetScheme{Dark: tc.dark})
			p := PaletteFrom(tc.set)
			code := newCodeState()
			img := pageWith(t, m, tc.set, code)

			// The code face group: the applied name is drawn in the platform's
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

			// The style column: the applied style is brought into view and
			// marked, which is where the chooser's own reveal puts it.
			visible := m.VisibleStyles(tc.dark)
			row := slices.Index(visible, m.StyleAt(tc.dark))
			if row < 0 {
				t.Fatalf("the applied style is not on the list this appearance shows")
			}
			if got := img.RGBAAt(rowFillX(), styleNameY(row, code.styles.st.Position())); !is(got, p.Selection) {
				t.Errorf("the applied style's row drew %v, want the platform's selection %v", got, p.Selection)
			}

			// And the fence beside them has that appearance's own member applied.
			shown := PreviewSet(tc.set, m, tc.dark)
			fence := CodeStyle(shown, tokens.DefaultTypography, shown.ControlBackground, m.AppliedStyles()).CodeBackground
			if !found(img, fence) {
				t.Errorf("the fence's own background %v (from %q) is nowhere in the window", fence, m.Style(tc.dark))
			}
		})
	}
}

// codeRigid is every part of the Code tab's column, and of the page around
// the tab, that states its own height: everything but the style group's box,
// which takes what is left over.
func codeRigid() int {
	group := func(box int) int {
		return int(GroupTitleH) + int(GroupBelow) + box + int(GroupAbove)
	}
	return int(TitleH) + int(Gap) + stripH() + int(TabGap) +
		group(int(FaceBoxH)) + group(0) + int(FooterH)
}

// TestEveryTabFitsTheWindow: every choice, the style list, the preview and
// the footer are in the window the application opens, with no scroll in the
// page. The Code tab is a stack of stated heights with one flexed box in its
// column, and the preview beside the column is as tall as the column. So the
// check is that the preview's box still shows enough lines of code to judge
// a style by. The Colours tab states every height, so its check is that its
// last box ends above the footer.
func TestEveryTabFitsTheWindow(t *testing.T) {
	left := windowH - 2*int(Pad) - codeRigid()
	if got := codeBoxBottom() - styleBoxTop(); got != left {
		t.Errorf("the style group's box measures %d points and the column leaves %d", got, left)
	}
	// The preview has to show enough of the Go file to judge a style by:
	// twenty lines of code inside the block's padding.
	want := 20*int(tokens.DefaultTypography.Code.LineHeight) + 2*int(CodePad)
	if got := codeBoxBottom() - previewBoxTop(); got < want {
		t.Fatalf("the preview's box is %d points tall and twenty lines of code want %d: the Code tab no longer fits a %d by %d window",
			got, want, windowW, windowH)
	}
	rows := (styleListBottom() - styleListTop()) / int(StyleRowH)
	if rows < 12 {
		t.Errorf("the style list shows %d names at the window's opening size: a list that short is a keyhole", rows)
	}
	if end := colourBoxTop() + int(ColourBoxH); end > codeBoxBottom() {
		t.Errorf("the Colours tab's last box ends at y %d, under the tabs' foot at y %d", end, codeBoxBottom())
	}
	t.Logf("the style group's box is %d points tall; the style list shows %d names and the preview is %v",
		left, rows, previewRect().Size())
}

// TestThePreviewFillsItsBoxInTheSchemeOnScreen: the code block is the whole
// preview box, in the style for the scheme the switch at the top of the
// window is on. Each corner of the box, inside its rounding, is the style's
// background.
func TestThePreviewFillsItsBoxInTheSchemeOnScreen(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
		set  tokens.PlatformColors
	}{{"light", false, tokens.PlatformLight}, {"dark", true, tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			m := ReduceModel(onCode(judging()), SetScheme{Dark: tc.dark})
			img := page(t, m, tc.set)
			want := previewFill(m, tc.set)
			box := previewRect().Inset(int(BoxRadius))
			for _, at := range []image.Point{box.Min, {box.Max.X - 1, box.Min.Y}, {box.Min.X, box.Max.Y - 1}, box.Max.Sub(image.Pt(1, 1))} {
				if got := img.RGBAAt(at.X, at.Y); !is(got, want) {
					t.Errorf("the preview drew %v at %v, want the style's background %v: the code block does not fill the box", got, at, want)
				}
			}
		})
	}
}

// goldenModel is the window as the stored image shows it: a picture dropped
// and its colours extracted, on the embedded style set alone.
//
// Embedded alone, and not whatever the chooser happens to be offering:
// another test in this package loads a style out of a folder into the
// highlighting package's registry, and a stored image must not depend on
// which tests ran before it. Every style that ships is on the list either way,
// so this is the list the application shows on a machine nobody has added a
// style to.
func goldenModel(t *testing.T, dark bool) Model {
	t.Helper()
	m := dropped(t)
	m.Styles = nil
	for _, o := range styleOptions() {
		if o.Added {
			continue
		}
		m.Styles = append(m.Styles, o)
	}
	d := highlight.DefaultStyles()
	m.LightAt = styleIndex(m.Styles, d.Light, false)
	m.DarkAt = styleIndex(m.Styles, d.Dark, true)
	return ReduceModel(m, SetScheme{Dark: dark})
}

// TestTheWholeWindow stores the window on each tab, both appearances, at the
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
			for tab, name := range [tabCount]string{
				TabColours: "colours", TabTypography: "typography", TabComponents: "components",
				TabPatterns: "patterns", TabCode: "code", TabMarkdown: "markdown",
			} {
				m := goldenModel(t, tc.dark)
				m.Tab = tab
				golden.Compare(t, name+"-"+tc.name, pageWith(t, m, tc.set, newCodeState()))
			}
		})
	}
}
