package main

import (
	"image"
	stdcolor "image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestATabIsSelectedByItsIndex: the strip's selection moves the window to
// that tab, and an index outside the strip leaves the tab alone.
func TestATabIsSelectedByItsIndex(t *testing.T) {
	m := ReduceModel(Model{}, SelectTab{Index: TabCode})
	if m.Tab != TabCode {
		t.Fatalf("selecting the Code tab shows tab %d", m.Tab)
	}
	for _, idx := range []int{-1, tabCount} {
		if got := ReduceModel(m, SelectTab{Index: idx}).Tab; got != TabCode {
			t.Errorf("selecting tab %d moved the window to tab %d, want it left on the Code tab", idx, got)
		}
	}
}

// TestTheTabsAreInTheirOrder reads the strip's order: Colours, Typography,
// Components, Patterns, Code and Markdown.
func TestTheTabsAreInTheirOrder(t *testing.T) {
	want := [tabCount]string{"Colours", "Typography", "Components", "Patterns", "Code", "Markdown"}
	if tabTitles != want {
		t.Errorf("the strip's titles are %q, want %q", tabTitles, want)
	}
	for tab, title := range map[int]string{
		TabColours: "Colours", TabTypography: "Typography", TabComponents: "Components",
		TabPatterns: "Patterns", TabCode: "Code", TabMarkdown: "Markdown",
	} {
		if tabTitles[tab] != title {
			t.Errorf("tab %d is titled %q, want %q", tab, tabTitles[tab], title)
		}
	}
}

// controlsOn finds, on one tab, the rectangles the pointer hovers the scheme
// control in and the Keep button in. A rectangle is the run across a line
// through the control and the run down a line through the middle of that run.
func controlsOn(t *testing.T, tab int) (scheme, keep image.Rectangle) {
	t.Helper()
	m := goldenModel(t, false)
	m.Tab = tab
	bar := new(topClicks)
	w := staticPage(m, tokens.PlatformLight, newCodeState(), desktop.ScrollBarsWhenScrolling, bar)
	r := new(input.Router)
	ops := new(op.Ops)
	size := image.Pt(windowW, windowH)
	frame := func() {
		ops.Reset()
		w(layout.Context{Ops: ops, Source: r.Source(), Constraints: layout.Exact(size)})
		r.Frame(ops)
	}
	frame()
	frame()
	probe := func(hovered func() bool) func(image.Point) bool {
		return func(pt image.Point) bool {
			r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(pt.X)+0.5, float32(pt.Y)+0.5), Source: pointer.Mouse})
			frame()
			return hovered()
		}
	}
	onScheme := probe(func() bool { return bar.scheme[0].Hovered() || bar.scheme[1].Hovered() })
	onKeep := probe(func() bool { return bar.keep.Hovered() })
	trail := windowW - int(Pad)
	scheme = hoverRect(onScheme, int(TitleCenter), trail-200, windowW, 0, int(Pad)+int(TitleH)+int(Gap))
	keep = hoverRect(onKeep, footerTop()+int(FooterH)/2, trail-int(KeepW)-40, windowW, codeBoxBottom(), windowH)
	return scheme, keep
}

// hoverRect scans the row y from x0 to x1 for the points hit reports, and then
// the column through the middle of that run from y0 to y1.
func hoverRect(hit func(image.Point) bool, y, x0, x1, y0, y1 int) image.Rectangle {
	var r image.Rectangle
	found := false
	for x := x0; x < x1; x++ {
		if hit(image.Pt(x, y)) {
			if !found {
				r.Min.X, found = x, true
			}
			r.Max.X = x + 1
		}
	}
	if !found {
		return image.Rectangle{}
	}
	cx := (r.Min.X + r.Max.X) / 2
	found = false
	for yy := y0; yy < y1; yy++ {
		if hit(image.Pt(cx, yy)) {
			if !found {
				r.Min.Y, found = yy, true
			}
			r.Max.Y = yy + 1
		}
	}
	return r
}

// TestTheSchemeControlAndKeepAreAtTheSamePlaceOnEveryTab: the title row and
// the footer are outside the tabs. The scheme control is at the top right of
// the window and the Keep button is under the tabs, at the same place on
// every tab.
func TestTheSchemeControlAndKeepAreAtTheSamePlaceOnEveryTab(t *testing.T) {
	var schemes, keeps [tabCount]image.Rectangle
	for tab := range tabCount {
		schemes[tab], keeps[tab] = controlsOn(t, tab)
		s, k := schemes[tab], keeps[tab]
		if s.Empty() || k.Empty() {
			t.Fatalf("on the %s tab the scheme control is at %v and the Keep button at %v, want both found", tabTitles[tab], s, k)
		}
		trail := windowW - int(Pad)
		if s.Max.X != trail || s.Min.Y < 0 || s.Max.Y > int(Pad)+int(TitleH) {
			t.Errorf("on the %s tab the scheme control is at %v, want it to end at x %d inside the title row (y 0 to %d)",
				tabTitles[tab], s, trail, int(Pad)+int(TitleH))
		}
		if k.Max.X != trail || k.Min.Y < codeBoxBottom() {
			t.Errorf("on the %s tab the Keep button is at %v, want it to end at x %d under the tabs (y %d down)",
				tabTitles[tab], k, trail, codeBoxBottom())
		}
	}
	for tab := 1; tab < tabCount; tab++ {
		if schemes[tab] != schemes[0] {
			t.Errorf("the scheme control is at %v on the %s tab and at %v on the %s tab", schemes[tab], tabTitles[tab], schemes[0], tabTitles[0])
		}
		if keeps[tab] != keeps[0] {
			t.Errorf("the Keep button is at %v on the %s tab and at %v on the %s tab", keeps[tab], tabTitles[tab], keeps[0], tabTitles[0])
		}
	}
	t.Logf("scheme control %v, Keep button %v", schemes[0], keeps[0])
}

// TestThePreviewIsToTheRightOfTheCodeChoices: on the Code tab the face group
// and the style group are one column at the leading edge, and the preview's
// box starts to the right of both. The preview's box is as tall as the
// column: it starts level with the face group's box and ends level with the
// style group's box.
func TestThePreviewIsToTheRightOfTheCodeChoices(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m := onCode(goldenModel(t, tc.dark))
			img := pageWith(t, m, tc.set, settled(tc.dark))
			p := PaletteFrom(WindowSet(tc.set, m))
			column := int(Pad) + int(StyleW)
			// The preview's leading edge: the first box pixel right of the
			// column, read along a line through the middle of the box.
			y := (previewBoxTop() + codeBoxBottom()) / 2
			lead := -1
			for x := column; x < windowW; x++ {
				if is(img.RGBAAt(x, y), p.Surface) {
					lead = x
					break
				}
			}
			if lead < 0 {
				t.Fatalf("no preview box right of the column at y %d", y)
			}
			if faces := faceChipX(len(codeFaces)-1) + int(FaceChipW); lead <= faces {
				t.Errorf("the preview's box starts at x %d, left of the face choices' trailing edge at x %d", lead, faces)
			}
			// The style group's box: its trailing edge is the last box pixel
			// before the run of the window's plane.
			sy := styleBoxTop() + int(BoxPad)/2
			styleTrail := -1
			for x := int(Pad); x < lead; x++ {
				if is(img.RGBAAt(x, sy), p.Surface) {
					styleTrail = x + 1
				}
			}
			if styleTrail < 0 || lead <= styleTrail {
				t.Errorf("the preview's box starts at x %d and the style list's box ends at x %d", lead, styleTrail)
			}
			if is(img.RGBAAt(lead-1, y), p.Surface) || !is(img.RGBAAt(lead-1, y), p.Backdrop) {
				t.Errorf("left of the preview's box at x %d the window drew %v, want the window's plane", lead-1, img.RGBAAt(lead-1, y))
			}
			mid := lead + int(BoxRadius) + 2
			if top := faceBoxTop(); !is(img.RGBAAt(mid, top), p.Surface) || is(img.RGBAAt(mid, top-1), p.Surface) {
				t.Errorf("the preview's box does not start at y %d, level with the face group's box", top)
			}
			if bottom := codeBoxBottom(); !is(img.RGBAAt(mid, bottom-1), p.Surface) || is(img.RGBAAt(mid, bottom), p.Surface) {
				t.Errorf("the preview's box does not end at y %d, level with the style group's box", bottom)
			}
		})
	}
}

// movedTabs are the tabs that show the theme's colour set, type scale and
// inventory groups.
var movedTabs = []int{TabColours, TabTypography, TabComponents, TabPatterns, TabMarkdown}

// TestEachMovedTabIsDrawnInTheChosenScheme sets the scheme to dark on a
// desktop that is light, and finds each moved tab drawn in the dark set: the
// content's fill of the dark set covers most of the tab, and the content's
// fill of the light set covers almost none of it. The board on the Colours
// tab draws the light set's swatches, so the second count is not zero.
func TestEachMovedTabIsDrawnInTheChosenScheme(t *testing.T) {
	area := image.Rect(int(Pad), contentTop(), windowW-int(Pad), codeBoxBottom())
	count := func(img *image.RGBA, want stdcolor.NRGBA) int {
		n := 0
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if is(img.RGBAAt(x, y), want) {
					n++
				}
			}
		}
		return n
	}
	total := area.Dx() * area.Dy()
	for _, tab := range movedTabs {
		t.Run(tabTitles[tab], func(t *testing.T) {
			m := goldenModel(t, false)
			m.Tab = tab
			dark := page(t, ReduceModel(m, SetScheme{Dark: true}), tokens.PlatformLight)
			light := page(t, m, tokens.PlatformLight)
			if golden.PixelDiff(light, dark) == 0 {
				t.Fatalf("the %s tab is the same in the two schemes", tabTitles[tab])
			}
			darkFill := count(dark, inventory.SectionSurface(tokens.PlatformDark))
			lightFill := count(dark, inventory.SectionSurface(tokens.PlatformLight))
			if darkFill < total/2 {
				t.Errorf("in the dark scheme the dark content's fill covers %d of %d points of the %s tab, want at least half",
					darkFill, total, tabTitles[tab])
			}
			if lightFill > total/50 {
				t.Errorf("in the dark scheme the light content's fill covers %d of %d points of the %s tab, want at most 2%%",
					lightFill, total, tabTitles[tab])
			}
		})
	}
}

// TestACodeFaceChosenOnTheTypographyTabIsTheCodeTabsChoice: the Typography
// tab has its own handlers for the two code faces, and the pointer finds them
// on that tab. The choice it posts sets the code face the Code tab marks, and
// the type scale's Code line changes with it and no other line does.
func TestACodeFaceChosenOnTheTypographyTabIsTheCodeTabsChoice(t *testing.T) {
	m := goldenModel(t, false)
	m.Tab = TabTypography
	pages := newThemePages()
	w := staticPages(m, tokens.PlatformLight, newCodeState(), pages, desktop.ScrollBarsWhenScrolling, new(topClicks))
	r := new(input.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		w(layout.Context{Ops: ops, Source: r.Source(), Constraints: layout.Exact(image.Pt(windowW, windowH))})
		r.Frame(ops)
	}
	frame()
	r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(faceFillX(1)), float32(faceRowY())), Source: pointer.Mouse})
	frame()
	if !pages.faces.clicks[1].Hovered() {
		t.Fatal("the pointer over JetBrains Mono on the Typography tab does not reach that tab's handler")
	}

	chosen := ReduceModel(m, SelectMono{Name: tokens.CodeFaceJetBrains})
	onCodeTab := chosen
	onCodeTab.Tab = TabCode
	img := page(t, onCodeTab, tokens.PlatformLight)
	if got, want := img.RGBAAt(faceMarkX(1), faceRowY()), PaletteFrom(tokens.PlatformLight).Selection; !is(got, want) {
		t.Errorf("the Code tab draws %v under JetBrains Mono, want the selection %v", got, want)
	}

	scale := image.Rect(int(Pad), faceBoxTop()+int(FaceBoxH)+int(GroupAbove), windowW-int(Pad), codeBoxBottom())
	before, after := page(t, m, tokens.PlatformLight), page(t, chosen, tokens.PlatformLight)
	rows := make(map[int]int)
	bottom := -1
	for y := scale.Min.Y; y < scale.Max.Y; y++ {
		for x := scale.Min.X; x < scale.Max.X; x++ {
			if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				rows[y]++
				bottom = y
			}
		}
	}
	if bottom < 0 {
		t.Fatal("choosing JetBrains Mono changed no line of the type scale")
	}
	// The Code role is the scale's last line. The change is in that line: a
	// band at the foot of the change, no taller than two of the Code role's
	// lines. The other lines are set in the same faces through another
	// shaper, which moves a few edge pixels and no more.
	top := bottom - 2*int(tokens.DefaultTypography.Code.LineHeight)
	in, out := 0, 0
	for y, n := range rows {
		if y > top {
			in += n
		} else {
			out += n
		}
	}
	if in <= 3*out {
		t.Errorf("choosing JetBrains Mono changed %d pixels in the Code line (y %d to %d) and %d pixels in the other lines, want the Code line's change to lead",
			in, top+1, bottom, out)
	}
	t.Logf("the Code line changed %d pixels from y %d to y %d, the other lines %d", in, top+1, bottom, out)
}
