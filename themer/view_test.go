package main

import (
	"image"
	stdcolor "image/color"
	"strings"
	"testing"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/input"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// The window renders are captured, never stored: what is asserted is that
// the swatches carry the candidates' own colours and that the chosen one is
// marked, and both are pixel facts a stored image would only illustrate.

// pinned builds the application's typography with the default faces and
// nothing else, system fonts off, so a render here cannot depend on the
// machine's font set.
func pinned() Type {
	ty := TypeFrom(tokens.DefaultTypography)
	ty.Shaper = tokens.DefaultTypography.DeterministicShaper()
	return ty
}

// page renders the whole window for a model and returns the capture.
func page(t *testing.T, m Model, os tokens.ColorTokens) *image.RGBA {
	t.Helper()
	return pageOn(t, newEmbed(), m, os)
}

// pageOn is page against an embedded inventory that outlives the render,
// which is how the window itself holds it: one inventory, many palettes. The
// base selector can be passed in for the same reason — a test that reads the
// column by position needs the column to be where it left it.
func pageOn(t *testing.T, e *embed, m Model, os tokens.ColorTokens, sel ...*baseSelector) *image.RGBA {
	t.Helper()
	return pageAt(t, e, m, os, image.Pt(windowW, windowH), sel...)
}

// pageAt is pageOn at a window size of the caller's choosing, which is how the
// top of the window is measured at more than the one width it was drawn at.
func pageAt(t *testing.T, e *embed, m Model, os tokens.ColorTokens, size image.Point, sel ...*baseSelector) *image.RGBA {
	t.Helper()
	bases := newBaseSelector()
	if len(sel) > 0 {
		bases = sel[0]
	}
	clicks := make([]gesture.Click, imageseed.DefaultMax)
	widget := Page(themed{os: os, typ: pinned()}, m, &desktop.ZoneGroup{}, clicks, new(topClicks), e, bases, newStyleGrid())
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		// The backdrop is its own layer at runtime; here it is one fill
		// under the page, resolved the same way that layer resolves it.
		fill(gtx, size, SchemeFor(os, m).Background)
		return widget(gtx)
	})
}

func fill(gtx layout.Context, size image.Point, c stdcolor.NRGBA) {
	fillRRect(gtx, image.Rectangle{Max: size}, 0, c)
}

// dropped is the model after a picture has landed: a real extraction of a
// painted scene, so the swatches under test are the ones the pipeline
// actually produces.
func dropped(t *testing.T) Model {
	t.Helper()
	img := scene(480, 360)
	candidates := imageseed.Extract(img)
	if len(candidates) < 3 {
		t.Fatalf("the fixture scene yielded %d candidates, want a row to test", len(candidates))
	}
	return Model{Preview: preview(img), Name: "scene.png", Candidates: candidates}
}

// The bands across the top of the window, from the same constants the page
// stacks them with: the title row both screens carry — inside the page's own
// margin, having no ground of its own to carry one on — and under it, on the
// screen that has a theme on it, the identity strip.
func titleTop() int    { return int(Pad) }
func titleBottom() int { return titleTop() + int(TitleH) }
func headTop() int     { return titleBottom() + int(Gap) }
func headBottom() int  { return headTop() + int(HeadH) }

// The columns the title row's three items stand in, measured the way the row
// measures them: the name takes its own width at the page's margin, the way
// back stands one row gap past it and is a mark, a small gap and a label, and
// the switch is the last thing before the trailing margin.
func titleNameW() int { return natural(measuring(), pinned().Shaper, pinned().Head, AppName) }

func backW() int {
	return int(BackMark) + int(BackGap) + natural(measuring(), pinned().Shaper, pinned().Body, BackLabel)
}

func backLeft() int { return int(Pad) + titleNameW() + int(Gap) }

// cardTop is the y of the candidate cards' top edge, cardW the width they
// share out between them, and cardX the left edge of card i — all computed
// from the same constants and the same arithmetic the row lays out with. The
// row sits under the identity strip, between the picture it came from and the
// page it themes.
func cardTop() int {
	return headBottom() + int(Gap) + int(RowLabelH) + int(RowTop)
}

// galleryTop is the y of the embedded page's panel, and galleryBottom the y
// past its last row: the band of the window the whole design system is drawn
// in.
func galleryTop() int {
	return cardTop() + int(CellH) + int(Gap) + int(RowLabelH) + int(RowTop)
}

func galleryBottom() int { return windowH - int(Pad) }

func cardW(n int) int { return cellWidth(windowW-2*int(Pad), int(CellGap), n) }

func cardX(n, i int) int { return int(Pad) + i*(cardW(n)+int(CellGap)) }

// cellCentre is the middle of candidate i's colour swatch, in a row of n.
func cellCentre(n, i int) image.Point {
	return image.Pt(cardX(n, i)+cardW(n)/2, cardTop()+int(CellPad)+int(SwatchH)/2)
}

// TestRowFillsItsWidth: a full row reaches the same right margin as the
// page above it, rather than stopping short of it with dead space wide
// enough to read as a card that failed to load.
func TestRowFillsItsWidth(t *testing.T) {
	n := len(dropped(t).Candidates)
	right := cardX(n, n-1) + cardW(n)
	if slack := (windowW - int(Pad)) - right; slack > int(CellGap) {
		t.Errorf("a row of %d cards ends %d dp short of the %d dp margin", n, slack, int(Pad))
	}
}

// TestSwatchesCarryTheCandidateColours: every candidate in the row is drawn
// as its own colour. A row of swatches that were not the extracted colours
// would be a convincing picture of nothing.
func TestSwatchesCarryTheCandidateColours(t *testing.T) {
	m := dropped(t)
	img := page(t, m, tokens.DefaultLight)
	n := len(m.Candidates)
	for i, c := range m.Candidates {
		p := cellCentre(n, i)
		if p.X+cardW(n)/2 > windowW-int(Pad) {
			break // a swatch the window is too narrow for is not drawn
		}
		got := img.RGBAAt(p.X, p.Y)
		if got.R != c.Color.R || got.G != c.Color.G || got.B != c.Color.B {
			t.Errorf("swatch %d at %v drew %v, want the candidate's %v", i, p, got, c.Color)
		}
	}
}

// TestChosenCandidateIsMarked: exactly one card carries the ring, and it is
// the chosen one. Asserted within each render rather than between two, so a
// window that re-themes wholesale on every choice cannot pass by accident.
func TestChosenCandidateIsMarked(t *testing.T) {
	m := dropped(t)
	n := len(m.Candidates)
	edge := func(img *image.RGBA, i int) stdcolor.RGBA {
		return img.RGBAAt(cardX(n, i)+cardW(n)/2, cardTop()+int(Ring)/2)
	}
	for _, chosen := range []int{0, 1} {
		img := page(t, ReduceModel(m, SelectCandidate{Index: chosen}), tokens.DefaultLight)
		other := 1 - chosen
		if edge(img, chosen) == edge(img, other) {
			t.Errorf("with candidate %d chosen, its card edge matches card %d's — nothing marks the choice", chosen, other)
		}
		if edge(img, other) != edge(img, 2) {
			t.Errorf("with candidate %d chosen, card %d's edge differs from card 2's — more than one card is marked", chosen, other)
		}
	}
}

// TestEmptyWindowInvites: with nothing dropped, the window is not blank —
// it says what to do, and the drop well is drawn.
func TestEmptyWindowInvites(t *testing.T) {
	empty := page(t, Model{}, tokens.DefaultLight)
	blank := golden.Capture(t, image.Pt(windowW, windowH), func(gtx layout.Context) layout.Dimensions {
		fill(gtx, image.Pt(windowW, windowH), tokens.DefaultLight.Background)
		return layout.Dimensions{Size: image.Pt(windowW, windowH)}
	})
	if golden.PixelDiff(empty, blank) == 0 {
		t.Error("the empty window drew nothing at all")
	}
}

// TestDragHighlightIsVisible: while a drag hovers, the window looks
// different — the affordance is what tells the user the drop will land.
func TestDragHighlightIsVisible(t *testing.T) {
	m := Model{}
	rest := page(t, m, tokens.DefaultLight)
	over := page(t, ReduceModel(m, desktop.FilesEntered{Zone: dropZone}), tokens.DefaultLight)
	if golden.PixelDiff(rest, over) == 0 {
		t.Error("a drag over the window changed no pixel")
	}
}

// TestBothSchemesRender: the page draws in the dark palette as well as the
// light one, and the two differ.
func TestBothSchemesRender(t *testing.T) {
	m := dropped(t)
	light := page(t, m, tokens.DefaultLight)
	dark := page(t, m, tokens.DefaultDark)
	if golden.PixelDiff(light, dark) == 0 {
		t.Error("the dark scheme rendered identically to the light one")
	}
}

// The two widths the top of the window is measured at: the one the owner's
// window was, and one narrow enough that the identity block has to give ground
// to the controls beside it. Alignment that only holds at the width a screen
// was captured at is not alignment, it is a coincidence.
const (
	wideW   = 1100
	narrowW = 700
)

// measuring is a layout context with nothing on it but the scale a render
// uses, for asking the shaper how wide a string is outside a frame.
func measuring() layout.Context {
	return layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
}

// probe is a control of a known size that paints itself solid in a colour
// nothing else uses, so where a row put it can be read back off a render
// rather than asserted about.
func probe(size image.Point, c stdcolor.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		w := min(size.X, gtx.Constraints.Max.X)
		if w <= 0 {
			return layout.Dimensions{}
		}
		box := image.Pt(w, size.Y)
		paint.FillShape(gtx.Ops, c, clip.Rect(image.Rectangle{Max: box}).Op())
		return layout.Dimensions{Size: box}
	}
}

// colourBand is the first and last row colour c was painted on, and how many
// rows that is.
func colourBand(img *image.RGBA, c stdcolor.NRGBA) (top, height int, found bool) {
	first, last := -1, -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R == c.R && p.G == c.G && p.B == c.B {
				if first < 0 {
					first = y
				}
				last = y
				break
			}
		}
	}
	if first < 0 {
		return 0, 0, false
	}
	return first, last - first + 1, true
}

// TestARowPutsEveryControlOnOneCentreLine holds the row that lays out both
// bands across the top of the window to its whole promise, at the two widths
// and with controls of the assorted heights the real ones have — a switch, a
// button, a two-line block, a line of hint text, a swatch the height of the
// row itself. Every one of them has to come out with its own middle on the
// row's middle, exactly, whether it is an odd number of points tall or an even
// one; "near enough" across five objects in the first thing anybody looks at
// is what a top that reads as sloppy is made of.
func TestARowPutsEveryControlOnOneCentreLine(t *testing.T) {
	heights := []int{36, 33, 40, 20, 56, 21}
	for _, h := range []int{int(TitleH), int(HeadH)} {
		for _, width := range []int{wideW, narrowW} {
			marks := make([]stdcolor.NRGBA, len(heights))
			slots := make([]slot, len(heights))
			for i, dy := range heights {
				marks[i] = stdcolor.NRGBA{R: uint8(40 + 30*i), G: 0x10, B: uint8(200 - 20*i), A: 0xff}
				end := leading
				if i%2 == 1 {
					end = trailing
				}
				slots[i] = slot{end, 0, probe(image.Pt(60, dy), marks[i])}
			}
			// The capture is taller than the row, with the row inset into it,
			// so a control that overflowed its row would still be measured
			// whole rather than cropped by the edge of the image.
			const margin = 32
			size := image.Pt(width, h+2*margin)
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				fill(gtx, size, tokens.DefaultLight.Background)
				at(gtx, image.Pt(0, margin), func(gtx layout.Context) {
					gtx.Constraints = layout.Exact(image.Pt(width, h))
					centreRow(gtx, h, int(Gap), slots...)
				})
				return layout.Dimensions{Size: size}
			})
			for i, dy := range heights {
				top, got, ok := colourBand(img, marks[i])
				if !ok {
					t.Errorf("h=%d w=%d: control %d (%d dp tall) was not drawn at all", h, width, i, dy)
					continue
				}
				if got != dy {
					t.Errorf("h=%d w=%d: control %d came out %d dp tall, want %d", h, width, i, got, dy)
				}
				if centre := top + got/2; centre != margin+h/2 {
					t.Errorf("h=%d w=%d: control %d (%d dp tall) is centred on y=%d, want the row's own centre y=%d",
						h, width, i, dy, centre-margin, h/2)
				}
			}
		}
	}
}

// inkBand is the first and last row between y0 and y1 on which something was
// drawn over the window's own ground, within the columns x0 up to x1 — the
// vertical extent of whatever control stands in that column.
func inkBand(img *image.RGBA, ground stdcolor.NRGBA, x0, x1, y0, y1 int) (top, height int, found bool) {
	first, last := -1, -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			p := img.RGBAAt(x, y)
			if p.R != ground.R || p.G != ground.G || p.B != ground.B {
				if first < 0 {
					first = y
				}
				last = y
				break
			}
		}
	}
	if first < 0 {
		return 0, 0, false
	}
	return first, last - first + 1, true
}

// textSlack is how far a run of text's inked rows may sit off the line its box
// is centred on. Text is centred by its line box and inked by its glyphs, and
// the two are not the same span: a line with descenders in it inks lower than
// one without, and a name in the body face inks taller than a caption in the
// small one. The slack is what that costs, and it is small enough that a
// control placed on the wrong line could not hide inside it.
const textSlack = 2

// TestTheTopOfTheWindowIsOnOneCentreLine is the alignment measured on the
// window itself rather than on the row in isolation: the title row's three
// items on one line — the window's name, the way back, the scheme switch — and
// the identity strip's four on another, in both schemes and at both widths.
//
// The solid controls are held to the exact centre, because their inked rows
// are their boxes. Everything drawn as glyphs or as a mark is held to it within
// the slack an outline's own extent costs, and to sitting wholly inside the
// line box it was laid out in.
func TestTheTopOfTheWindowIsOnOneCentreLine(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		on := ReduceModel(after, SetScheme{Dark: sc.dark})
		ground := stdcolor.NRGBA(SchemeFor(sc.os, on).Background)
		for _, width := range []int{wideW, narrowW} {
			img := pageAt(t, newEmbed(), on, sc.os, image.Pt(width, windowH))
			// The title row. The switch is solid and holds the exact
			// centre; the name and the way back are ink on the page, and are
			// held to it within the slack a glyph's own extent costs.
			titleMid := titleTop() + int(TitleH)/2
			for _, c := range []struct {
				what   string
				x0, x1 int
				solid  bool
			}{
				{"the window's name", int(Pad), int(Pad) + titleNameW(), false},
				{"the way back", backLeft(), backLeft() + backW(), false},
				{"the scheme switch", width - int(Pad) - int(inventory.SchemeSwitchW), width - int(Pad), true},
			} {
				top, h, ok := inkBand(img, ground, c.x0, c.x1, titleTop(), titleBottom())
				if !ok {
					t.Errorf("%s at %d dp: %s is not drawn in the title row", sc.name, width, c.what)
					continue
				}
				centre, slack := top+h/2, textSlack
				if c.solid {
					slack = 0
				}
				t.Logf("%s at %d dp: %s inks %d dp from y=%d, centre y=%d against the row's centre y=%d",
					sc.name, width, c.what, h, top, centre, titleMid)
				if centre < titleMid-slack || centre > titleMid+slack {
					t.Errorf("%s at %d dp: %s is %d dp tall and centred on y=%d, want within %d dp of the row's centre y=%d",
						sc.name, width, c.what, h, centre, slack, titleMid)
				}
			}
			// The identity strip. The swatch and the keep affordance are
			// solid; the name block and the standing offer are text.
			headMid := headTop() + int(HeadH)/2
			solid := []struct {
				what   string
				x0, x1 int
			}{
				{"the swatch", int(Pad), int(Pad) + int(ThumbW)},
				{"the keep affordance", width - int(Pad) - int(KeepW), width - int(Pad)},
			}
			for _, c := range solid {
				top, h, ok := inkBand(img, ground, c.x0, c.x1, headTop(), headBottom())
				if !ok {
					t.Errorf("%s at %d dp: %s is not drawn in the identity strip", sc.name, width, c.what)
					continue
				}
				if centre := top + h/2; centre != headMid {
					t.Errorf("%s at %d dp: %s is %d dp tall and centred on y=%d, want the strip's centre y=%d",
						sc.name, width, c.what, h, centre, headMid)
				}
			}
			// The name and its caption, as one block, and the standing offer
			// beside it. The offer's own columns are found by asking the
			// shaper how wide it is — the same measurement the row sizes it
			// with — rather than by guessing a gutter at it. It stands one
			// row gap and one extra gap clear of the keep affordance, which
			// is the room that stops it reading as part of the button.
			offerRight := width - int(Pad) - int(KeepW) - 2*int(Gap)
			offerLeft := offerRight - natural(measuring(), pinned().Shaper, pinned().Small, ReplaceHintFor(on))
			textAt := []struct {
				what   string
				x0, x1 int
				box    int // the block's own height, which the ink must sit inside
			}{
				{"the name and its caption", int(Pad) + int(ThumbW) + int(Gap), offerLeft - int(Gap), 2 * int(LineH)},
				{"the standing offer", offerLeft, offerRight, int(LineH)},
			}
			for _, c := range textAt {
				top, h, ok := inkBand(img, ground, c.x0, c.x1, headTop(), headBottom())
				if !ok {
					t.Errorf("%s at %d dp: %s is not drawn in the identity strip", sc.name, width, c.what)
					continue
				}
				centre := top + h/2
				t.Logf("%s at %d dp: %s inks %d dp from y=%d, centre y=%d against the strip's y=%d",
					sc.name, width, c.what, h, top, centre, headMid)
				if centre < headMid-textSlack || centre > headMid+textSlack {
					t.Errorf("%s at %d dp: %s inks around y=%d, want it within %d dp of the strip's centre y=%d",
						sc.name, width, c.what, centre, textSlack, headMid)
				}
				if top < headMid-c.box/2 || top+h > headMid+c.box/2 {
					t.Errorf("%s at %d dp: %s inks rows %d..%d, outside the %d dp block centred on y=%d it was laid out in",
						sc.name, width, c.what, top, top+h, c.box, headMid)
				}
			}
		}
	}
}

// TestTheNameCanNeverRunUnderTheSwatch is the width constraint's whole point,
// measured: a name and a caption long enough to cross the window twice change
// nothing in the swatch's own columns, and nothing in the keep affordance's,
// at either width and in either scheme. What holds them in is the block's
// width — it is only ever offered the room the swatch and the controls left —
// and the clip drawn round it, which makes that a property of the painting and
// not of the arithmetic.
func TestTheNameCanNeverRunUnderTheSwatch(t *testing.T) {
	m := withStyles()
	short := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	long := short
	long.Name = strings.Repeat("quayside-night-", 16)
	long.Problem = strings.Repeat("a style in the folder would not load and here is why ", 6)
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		for _, width := range []int{wideW, narrowW} {
			size := image.Pt(width, windowH)
			was := pageAt(t, newEmbed(), ReduceModel(short, SetScheme{Dark: sc.dark}), sc.os, size)
			got := pageAt(t, newEmbed(), ReduceModel(long, SetScheme{Dark: sc.dark}), sc.os, size)
			for _, c := range []struct {
				what   string
				x0, x1 int
			}{
				{"the swatch and the gap after it", 0, int(Pad) + int(ThumbW) + int(Gap)/2},
				{"the keep affordance", width - int(Pad) - int(KeepW), width},
			} {
				if n := changedIn(was, got, c.x0, c.x1, headTop(), headBottom()); n != 0 {
					t.Errorf("%s at %d dp: a name and a caption of their longest moved %d pixels of %s",
						sc.name, width, n, c.what)
				}
			}
			// And the long name is really on screen: a block that drew
			// nothing at all would pass everything above.
			mid := int(Pad) + int(ThumbW) + int(Gap)
			if n := changedIn(was, got, mid, mid+int(LineH), headTop(), headBottom()); n == 0 {
				t.Errorf("%s at %d dp: the long name changed nothing where the name is drawn", sc.name, width)
			}
		}
	}
}

// changedIn counts the pixels inside a rectangle that two renders disagree on,
// past the rasteriser's own jitter.
func changedIn(a, b *image.RGBA, x0, x1, y0, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := max(x0, 0); x < min(x1, a.Bounds().Max.X); x++ {
			p, q := a.RGBAAt(x, y), b.RGBAAt(x, y)
			if apart(p.R, q.R) > inkJitter || apart(p.G, q.G) > inkJitter ||
				apart(p.B, q.B) > inkJitter || apart(p.A, q.A) > inkJitter {
				n++
			}
		}
	}
	return n
}

// inkColumns is the first and last column between x0 and x1 on which something
// was drawn over the window's own ground, within the rows y0 up to y1.
func inkColumns(img *image.RGBA, ground stdcolor.NRGBA, x0, x1, y0, y1 int) (first, last int, found bool) {
	first, last = -1, -1
	for x := max(x0, 0); x < min(x1, img.Bounds().Max.X); x++ {
		for y := y0; y < y1; y++ {
			p := img.RGBAAt(x, y)
			if p.R != ground.R || p.G != ground.G || p.B != ground.B {
				if first < 0 {
					first = x
				}
				last = x
				break
			}
		}
	}
	if first < 0 {
		return 0, 0, false
	}
	return first, last, true
}

// TestNothingRulesOffTheTitleRow is the band and the rule measured gone.
//
// A bar is a strip of a different surface with a line closing it, and that is
// what the top of this window used to be. A title row is neither: it stands on
// the window's own page, and what marks it as the top of the window is that it
// is at the top of the window. Two scans say so. The first sweeps the row's own
// height in the columns between the way back and the scheme switch — the middle
// of the row, where a ground would show if there were one. The second sweeps
// every column of the band between the row's foot and whatever stands under it,
// which is where a rule would be. Both have to come back as the window's own
// ground, on both screens and in both schemes.
func TestNothingRulesOffTheTitleRow(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		for _, screen := range []struct {
			what string
			m    Model
		}{
			{"the start screen", ReduceModel(m, SetScheme{Dark: sc.dark})},
			{"the screen after a click", ReduceModel(after, SetScheme{Dark: sc.dark})},
		} {
			ground := stdcolor.NRGBA(SchemeFor(sc.os, screen.m).Background)
			for _, width := range []int{wideW, narrowW} {
				img := pageAt(t, newEmbed(), screen.m, sc.os, image.Pt(width, windowH))
				// The middle of the row, from its top edge to its foot: no
				// ground of its own anywhere the controls are not.
				mid0, mid1 := backLeft()+backW()+int(Gap), width-int(Pad)-int(inventory.SchemeSwitchW)-int(Gap)
				if _, _, inked := inkBand(img, ground, mid0, mid1, titleTop(), titleBottom()); inked {
					t.Errorf("%s, %s at %d dp: something is painted across the middle of the title row — it has a ground of its own",
						sc.name, screen.what, width)
				}
				// The whole width of the band under the row, where the rule
				// was: the row's foot down to the next thing on the page,
				// stopping a hairline short of it. The object below owns its
				// own edge, and a one-point outline drawn on that edge is
				// half a point of antialiasing in the row above it.
				if _, _, inked := inkBand(img, ground, 0, width, titleBottom(), titleBottom()+int(Gap)-int(Hairline)); inked {
					t.Errorf("%s, %s at %d dp: something is painted between the title row and what follows it — the row is still ruled off",
						sc.name, screen.what, width)
				}
			}
		}
	}
}

// TestTheStandingOfferIsNotCrowdedOntoTheKeepAffordance is the one thing in the
// identity strip a shared centre line does not by itself buy. A button carries
// its own inner padding, and text closer to the button than the button's label
// is to its own edge reads as the first half of that label rather than as a
// line standing on its own.
func TestTheStandingOfferIsNotCrowdedOntoTheKeepAffordance(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		on := ReduceModel(after, SetScheme{Dark: sc.dark})
		ground := stdcolor.NRGBA(SchemeFor(sc.os, on).Background)
		for _, width := range []int{wideW, narrowW} {
			img := pageAt(t, newEmbed(), on, sc.os, image.Pt(width, windowH))
			// The offer's trailing edge against the keep affordance's leading
			// one, both read off the render.
			offerW := natural(measuring(), pinned().Shaper, pinned().Small, ReplaceHintFor(on))
			_, offerEnds, ok := inkColumns(img, ground,
				width-int(Pad)-int(KeepW)-2*int(Gap)-offerW, width-int(Pad)-int(KeepW)-int(Gap), headTop(), headBottom())
			if !ok {
				t.Errorf("%s at %d dp: the standing offer is not drawn", sc.name, width)
				continue
			}
			keepStarts, _, ok := inkColumns(img, ground, width-int(Pad)-int(KeepW), width-int(Pad), headTop(), headBottom())
			if !ok {
				t.Errorf("%s at %d dp: the keep affordance is not drawn", sc.name, width)
				continue
			}
			apart, padding := keepStarts-offerEnds, int(tokens.Comfortable.PaddingX)
			t.Logf("%s at %d dp: %d dp between the offer and the keep affordance, whose own inner padding is %d dp",
				sc.name, width, apart, padding)
			if apart <= padding {
				t.Errorf("%s at %d dp: the offer ends %d dp from the keep affordance, which pads its own label by %d dp — the offer reads as part of the label",
					sc.name, width, apart, padding)
			}
		}
	}
}

// TestTheWayBackIsUndressedChromeUnderTheName is the dressing the title row
// asks of the way back, measured two ways.
//
// The first is the ink, measured off the tokens: the muted step has to clear
// the floor a line of text has to reach against the page it is on, and it has
// to stay plainly under the ink the window's own name is drawn in. That pair of
// facts is what makes the two items in the row a name and something quieter
// beside it rather than two things of equal weight — and it is what lets the
// control drop the ground and the boundary it used to need when it stood alone
// on a bar with nothing to be read against.
//
// The second is the dressing, measured off the render: inside the control's own
// line box, the colour most of it is has to be the window's own page. A tonal
// fill or an outline would take that majority, and a control drawn on the page
// cannot. The ratios are logged rather than read off pixels on purpose — a
// fourteen-point stem and a two-unit chevron are antialiased at this scale, so
// what a pixel reports is coverage and not the colour anybody chose.
func TestTheWayBackIsUndressedChromeUnderTheName(t *testing.T) {
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		on := ReduceModel(after, SetScheme{Dark: sc.dark})
		c := SchemeFor(sc.os, on)
		p := PaletteFrom(c)
		ground := stdcolor.NRGBA(c.Background)
		chrome := color.ContrastRatio(p.Muted, ground)
		name := color.ContrastRatio(p.Text, ground)
		t.Logf("%s: the way back's ink %v reaches %.2f:1 against the page %v, the window's name %v reaches %.2f:1",
			sc.name, p.Muted, chrome, ground, p.Text, name)
		if chrome < legibleFloor {
			t.Errorf("%s: the way back's ink measures %.2f:1 against the page, under the %.1f:1 a line of text has to reach — undressed, it cannot be read",
				sc.name, chrome, legibleFloor)
		}
		if chrome >= name {
			t.Errorf("%s: the way back's ink measures %.2f:1 against the page and the window's name %.2f:1 — the chrome does not read under the name",
				sc.name, chrome, name)
		}
		for _, width := range []int{wideW, narrowW} {
			img := pageAt(t, newEmbed(), on, sc.os, image.Pt(width, windowH))
			top, h, ok := inkBand(img, ground, backLeft(), backLeft()+backW(), titleTop(), titleBottom())
			if !ok {
				t.Errorf("%s at %d dp: the way back is not drawn", sc.name, width)
				continue
			}
			box := image.Rect(backLeft(), top, backLeft()+backW(), top+h)
			fill, _ := inkOn(img, box)
			if fill != ground {
				t.Errorf("%s at %d dp: most of the way back's own box is %v, want the window's page %v — the control is standing on a ground of its own",
					sc.name, width, fill, ground)
			}
		}
	}
}

// keptSeed is the brand the startup tests open a window on. It is nothing like
// the theme's own default seed — a different hue, and one nobody could mistake
// for it in a render — because the whole point of these two tests is telling
// the two apart on screen.
var keptSeed = stdcolor.NRGBA{R: 0xdf, G: 0x8e, B: 0x1d, A: 0xff}

// opened is the model a window starts from with a brand already kept: every
// base and every card on offer, nothing chosen yet, and the seed the window's
// own theme was built from recorded the way Init records it.
func opened(seed stdcolor.NRGBA) Model {
	m := withStyles().adoptKept(brand.Brand{Seed: seed})
	m.Opened = seed
	return m
}

// switchFill is the colour the filled half of the scheme switch is painted,
// read off a render at the title row's own centre line. The thumb is inset three
// points inside its half and the glyph on it is twenty points wide and
// centred, so the band between the two is the fill and nothing else.
func switchFill(img *image.RGBA, width int, dark bool) stdcolor.NRGBA {
	x := width - int(Pad) - int(inventory.SchemeSwitchW)
	if dark {
		x += int(inventory.SchemeSegmentW)
	}
	return opaque(img.RGBAAt(x+7, titleTop()+int(TitleH)/2))
}

// TestTheWindowIsTheSameScreenWhicheverWayItIsReached is the measurement the
// scheme switch's colour turns on: one side of a theme, arrived at two ways —
// opened on a desktop already set to it, and toggled to from a desktop set to
// the other — has to be the same window, pixel for pixel.
//
// The whole window and not the switch alone, on purpose. A control resolving
// its colour from somewhere other than the theme on screen is a kind of defect
// rather than one control's bug, and comparing every pixel is the only way to
// say that nothing else in the window does it either.
//
// Nothing is chosen here, which is the state the defect lived in: with a seed
// applied, both paths already derived from that seed, so a test that dropped a
// picture first would have passed against a window whose switch changed colour
// on the first press.
func TestTheWindowIsTheSameScreenWhicheverWayItIsReached(t *testing.T) {
	light, dark := tokens.FromSeed(keptSeed)
	m := opened(keptSeed)
	for _, sc := range []struct {
		name    string
		dark    bool
		on, off tokens.ColorTokens
	}{
		{"light", false, light, dark},
		{"dark", true, dark, light},
	} {
		t.Run(sc.name, func(t *testing.T) {
			// The desktop is already on this side and the window has not been
			// touched: the first frame.
			startup := page(t, m, sc.on)
			// The desktop is on the other side and the window's own switch is
			// asking for this one: the frame after a toggle.
			toggled := page(t, ReduceModel(m, SetScheme{Dark: sc.dark}), sc.off)
			if n := golden.PixelDiff(startup, toggled); n != 0 {
				t.Errorf("the %s scheme drew %d pixels differently opened into than toggled into — something in the window is themed from a palette other than the one on screen",
					sc.name, n)
			}
		})
	}
}

// TestTheSwitchOpensInTheThemeOnScreen reads the colour off the control
// itself: the filled half of the scheme switch, on the first frame, in both
// schemes and at both widths.
//
// It has to be the primary of the theme the window is wearing. The half of
// that worth stating is the negative — it must not be the primary of the
// theme's own default pair, which is what the control used to jump to on the
// first press of the switch, so that a window opened on a kept brand showed
// one colour until it was touched and another afterwards.
func TestTheSwitchOpensInTheThemeOnScreen(t *testing.T) {
	light, dark := tokens.FromSeed(keptSeed)
	m := opened(keptSeed)
	// Both wanted colours are read off the pair the kept seed derives, not off
	// what the window says it resolved: a window resolving the wrong palette
	// would agree with itself. on is the side the desktop is set to, which
	// before anything is chosen is the side of the kept theme on screen.
	for _, sc := range []struct {
		name             string
		dark             bool
		on, off, fallout tokens.ColorTokens
	}{
		{"light", false, light, dark, tokens.DefaultLight},
		{"dark", true, dark, light, tokens.DefaultDark},
	} {
		want := stdcolor.NRGBA(sc.on.Primary)
		for _, width := range []int{wideW, narrowW} {
			img := pageAt(t, newEmbed(), m, sc.on, image.Pt(width, windowH))
			got := switchFill(img, width, sc.dark)
			t.Logf("%s at %d dp: the switch opens filled %v", sc.name, width, got)
			if apart(got.R, want.R) > 1 || apart(got.G, want.G) > 1 || apart(got.B, want.B) > 1 {
				t.Errorf("%s at %d dp: the switch opens filled %v, want the kept theme's primary %v",
					sc.name, width, got, want)
			}
			if fallout := stdcolor.NRGBA(sc.fallout.Primary); got == fallout {
				t.Errorf("%s at %d dp: the switch opens in the default pair's primary %v rather than the kept theme's",
					sc.name, width, fallout)
			}
			// And the other half of the pair is reachable in that same theme:
			// one press moves the fill to the counterpart primary, not to a
			// colour from somewhere else.
			flipped := ReduceModel(m, SetScheme{Dark: !sc.dark})
			after := switchFill(pageAt(t, newEmbed(), flipped, sc.on, image.Pt(width, windowH)), width, !sc.dark)
			counterpart := stdcolor.NRGBA(sc.off.Primary)
			if apart(after.R, counterpart.R) > 1 || apart(after.G, counterpart.G) > 1 || apart(after.B, counterpart.B) > 1 {
				t.Errorf("%s at %d dp: one press filled the switch %v, want the other side of the same theme %v",
					sc.name, width, after, counterpart)
			}
		}
	}
}

// The window's own strip, and what stands in it.
//
// None of the four assertions below can be made from a render. The strip
// belongs to the native window — a headless capture has no window, no title bar
// and no control buttons — so what is asserted is the configuration the window
// is opened with and the insets the page lays the row out from, which is where
// the arrangement is actually decided. What a render can still say is said: the
// band the buttons stand in is measured off the page, and so is the run the row
// hands the window to be dragged by.

// TestTheWindowTakesTheNativeStrip is the treatment, read off the window
// configuration the options build.
//
// The claim is made against what the treatment does rather than against a
// window flag written out a second time. On macOS the treatment undecorates the
// window so the content extends behind a transparent title bar and the title
// row has the strip; everywhere else it contributes nothing and the window keeps
// the decorations the platform gives it. Comparing the two configurations says
// "this window opens the way the treatment opens one" on either platform, which
// is the whole claim.
func TestTheWindowTakesTheNativeStrip(t *testing.T) {
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	apply := func(opts []app.Option) app.Config {
		// Decorated is the field under test, and a window is decorated before
		// anybody asks otherwise, so that is what the base states.
		cnf := app.Config{Decorated: true}
		for _, o := range opts {
			o(metric, &cnf)
		}
		return cnf
	}
	treatment, got := apply(desktop.FullSizeContent()), apply(WindowOptions())
	t.Logf("the window opens decorated=%v; the treatment on its own gives decorated=%v",
		got.Decorated, treatment.Decorated)
	if got.Decorated != treatment.Decorated {
		t.Errorf("the window opens decorated=%v where the full-size-content treatment gives decorated=%v — the platform draws a strip above the page and the title row stands under it, which is two bands where there is one row",
			got.Decorated, treatment.Decorated)
	}
	// The name survives the treatment. It is hidden from the strip and read
	// everywhere else the platform names a window.
	if got.Title != AppName {
		t.Errorf("the window is titled %q, want %q", got.Title, AppName)
	}
	if want := image.Pt(windowW, windowH); got.Size != want {
		t.Errorf("the window opens at %v, want %v", got.Size, want)
	}
}

// TestTheWindowButtonsStandOnTheTitleRowsLine is the one centre line, asserted
// where it is decided.
//
// The row puts the window's name, the way back and the scheme switch on one
// line, and with the strip taken it has the window's three control buttons in
// it as well. They are placed on the row's line rather than left on the one the
// platform would default them to, and that line is the row's own middle — read
// here off the arithmetic every other measurement of this row is read off, so a
// row that changes height takes the buttons with it instead of leaving them
// behind on a number written down beside them.
func TestTheWindowButtonsStandOnTheTitleRowsLine(t *testing.T) {
	b := WindowButtons()
	mid := unit.Dp(titleTop()) + TitleH/2
	t.Logf("the title row runs %d to %d dp below the window's top edge; its middle is %v, and the buttons are placed on %v leading at %v",
		titleTop(), titleBottom(), mid, b.Center, b.Leading)
	if b.Center != mid {
		t.Errorf("the window buttons are placed on the line %v below the window's top edge and the title row's middle is %v — the row's fourth object is on a line of its own",
			b.Center, mid)
	}
	if b.Center != TitleCenter {
		t.Errorf("the placement asks for the line %v and the row is centred on %v — the two have come apart", b.Center, TitleCenter)
	}
	if b.Leading != Pad {
		t.Errorf("the window buttons lead at %v and the page's margin is %v — the buttons start on an edge of their own rather than the one everything else down the window starts on",
			b.Leading, Pad)
	}
}

// TestTheTitleRowLeadsPastTheWindowButtons is the strip claim's other half: the
// row starts where the window's own controls end, and the run before that is
// theirs.
func TestTheTitleRowLeadsPastTheWindowButtons(t *testing.T) {
	// A render has no window behind it, and neither has a platform that keeps
	// its decorations: with no buttons to clear, the row leads at the page's own
	// margin — which is where every render in this file measures it from.
	if got := TitleLead(); got != Pad {
		t.Fatalf("with no window buttons measured the title row leads at %v, want the page's margin %v", got, Pad)
	}

	// With them measured it starts one row gap past their trailing edge. The
	// edge stands in for the platform's own; what is asserted is that the row
	// is laid out from a measurement rather than from a constant.
	const end unit.Dp = 79
	defer func(was func() unit.Dp) { windowButtonsEnd = was }(windowButtonsEnd)
	windowButtonsEnd = func() unit.Dp { return end }
	if got, want := TitleLead(), end+Gap; got != want {
		t.Fatalf("with the buttons ending at %v the title row leads at %v, want %v", end, got, want)
	}

	// And the page lays the row out there. The band from the window's leading
	// edge to the row's start is the buttons' zone: nothing the application
	// draws may stand in it, and the row's first ink is at its far side.
	m := withStyles()
	after := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	for _, sc := range []struct {
		name string
		dark bool
		os   tokens.ColorTokens
	}{{"light", false, tokens.DefaultLight}, {"dark", true, tokens.DefaultDark}} {
		for _, screen := range []struct {
			what string
			m    Model
		}{
			{"the start screen", ReduceModel(m, SetScheme{Dark: sc.dark})},
			{"the screen after a click", ReduceModel(after, SetScheme{Dark: sc.dark})},
		} {
			ground := stdcolor.NRGBA(SchemeFor(sc.os, screen.m).Background)
			img := pageAt(t, newEmbed(), screen.m, sc.os, image.Pt(wideW, windowH))
			lead := int(TitleLead())
			if _, _, inked := inkColumns(img, ground, 0, lead, titleTop(), titleBottom()); inked {
				t.Errorf("%s, %s: something is drawn in the run the window's control buttons stand in", sc.name, screen.what)
			}
			first, _, ok := inkColumns(img, ground, lead, wideW, titleTop(), titleBottom())
			if !ok {
				t.Errorf("%s, %s: the title row draws nothing past the buttons", sc.name, screen.what)
				continue
			}
			if first-lead >= int(Gap) {
				t.Errorf("%s, %s: the title row's first ink is at x=%d, %d dp past where the row starts — the row is not laid out from the buttons' edge",
					sc.name, screen.what, first, first-lead)
			}
		}
	}
}

// TestTheTitleRowMovesTheWindow is the drag the strip came with.
//
// A window is dragged by the strip its title bar stands in, and with the content
// behind the title bar that press reaches this application instead of the
// platform. So the row hands a run of itself back as the handle — its empty
// middle, and only that, because a move action swallows the press before any
// control under it sees one. Without it the window could not be moved at all.
func TestTheTitleRowMovesTheWindow(t *testing.T) {
	m := withStyles()
	on := ReduceModel(m, AdoptStyle{Index: cardIndex(m, "dracula")})
	c := SchemeFor(tokens.DefaultLight, on)
	row := TitleRow(PaletteFrom(c), c, pinned(), on, false, new(topClicks))

	// The row as the page hands it to it: the window's width less its margins.
	width, h := wideW-2*int(Pad), int(TitleH)
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(width, h)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	row(gtx)
	var r input.Router
	r.Frame(&ops)
	moveAt := func(x int) bool {
		a, ok := r.ActionAt(f32.Pt(float32(x), float32(h/2)))
		return ok && a == system.ActionMove
	}

	// In the row's own coordinates the name leads, the way back stands one gap
	// past it, and the switch ends the row — so the middle runs from the way
	// back's trailing edge to the switch's leading one.
	backStarts := titleNameW() + int(Gap)
	free0, free1 := backStarts+backW(), width-int(inventory.SchemeSwitchW)
	t.Logf("the row is %d dp wide; its empty middle runs %d to %d", width, free0, free1)
	if free1-free0 <= 0 {
		t.Fatalf("the row has no empty middle between x=%d and x=%d — there is nothing to drag the window by", free0, free1)
	}
	for _, x := range []int{free0 + 1, (free0 + free1) / 2, free1 - 1} {
		if !moveAt(x) {
			t.Errorf("no window-move action at x=%d; the row holds no control there, so it must move the window", x)
		}
	}
	// And every control's own span belongs to the control.
	for _, c := range []struct {
		what string
		x    int
	}{
		{"the window's name", titleNameW() / 2},
		{"the way back", backStarts + backW()/2},
		{"the scheme switch", width - int(inventory.SchemeSwitchW)/2},
	} {
		if moveAt(c.x) {
			t.Errorf("a window-move action covers %s at x=%d; it would take the press meant for it", c.what, c.x)
		}
	}
}
