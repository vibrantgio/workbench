package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/vibrantgio/components/composite"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// findQuery is the word the long guide holds six times: twice in its second
// section, which the first screen shows, and once each in four sections far
// below it.
const findQuery = "needle"

// findMatchCount is how many times the long guide holds findQuery.
const findMatchCount = 6

// longGuide is a guide much taller than the page, with findQuery in the
// sections findMatchCount counts.
func longGuide() []byte {
	var b strings.Builder
	b.WriteString("# Long guide\n\n")
	prose := "The reader moves through this section to find what the guide says about the subject at hand, and the text runs on so that it takes several lines in the column."
	for k := range 40 {
		fmt.Fprintf(&b, "## Section %d\n\n", k)
		switch k {
		case 1:
			b.WriteString("A needle is here and a second needle is here. " + prose + "\n\n")
		case 12, 22, 31, 39:
			b.WriteString("One needle is in this section. " + prose + "\n\n")
		default:
			b.WriteString(prose + "\n\n")
		}
		b.WriteString(prose + "\n\n")
	}
	return []byte(b.String())
}

// The guide column's frame in the live page: right of the rail and under the
// open bar, to the page's trailing edge.
var findColumn = image.Rect(docsOutlineWidthDp, findBarFillDp+1, pageFrameSize.X, pageFrameSize.Y)

// openFindPage is the live page over the long guide with the bar open and
// the query typed into its field.
func openFindPage(t *testing.T) *livePage {
	t.Helper()
	p := newLivePageFrom(t, longGuide())
	m, _ := Update(p.model, OpenFind{})
	p.set(m)
	if !p.inField() {
		t.Fatal("the keyboard is not in the field after OpenFind")
	}
	p.r.Queue(key.EditEvent{Text: findQuery})
	p.frames(4)
	return p
}

// frames lays out n frames. A match off screen takes two frames to scroll
// into view, and the test router asks for no frame by itself.
func (p *livePage) frames(n int) {
	for range n {
		p.frame()
	}
}

// focus gives the keyboard to tag on the next frame.
func (p *livePage) focus(tag any) {
	p.mu.Lock()
	page := p.latest
	p.mu.Unlock()
	driveFrame(func(gtx layout.Context) layout.Dimensions {
		dims := page(gtx)
		gtx.Execute(key.FocusCmd{Tag: tag})
		return dims
	}, p.ops, p.r, pageFrameSize)
	p.frames(2)
}

// press clicks the pointer at (x, y) in the page.
func (p *livePage) press(x, y int) {
	at := f32.Pt(float32(x), float32(y))
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse},
	)
	p.frames(4)
}

// key presses name in the field, with Shift when shift is true.
func (p *livePage) key(name key.Name, shift bool) {
	var mods key.Modifiers
	if shift {
		mods = key.ModShift
	}
	p.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	p.frames(4)
}

// capture lays one frame of the live page out on the page's fill and renders
// it offscreen.
func (p *livePage) capture() *image.RGBA {
	p.t.Helper()
	w, err := headless.NewWindow(pageFrameSize.X, pageFrameSize.Y)
	if err != nil {
		p.t.Skipf("headless rendering not supported: %v", err)
	}
	defer w.Release()
	p.mu.Lock()
	page := onPageFill(tokens.PlatformLight, p.latest)
	p.mu.Unlock()
	driveFrame(func(gtx layout.Context) layout.Dimensions {
		composite.Frame(gtx)
		return page(gtx)
	}, p.ops, p.r, pageFrameSize)
	if err := w.Frame(p.ops); err != nil {
		p.t.Fatalf("headless frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: pageFrameSize})
	if err := w.Screenshot(img); err != nil {
		p.t.Fatalf("headless screenshot: %v", err)
	}
	return img
}

// inView reports whether the current match lies wholly inside the guide's
// viewport, of height h.
func (p *livePage) inView(h int) (image.Rectangle, bool) {
	matches := p.doc.Matches()
	if p.bar.current < 0 || p.bar.current >= len(matches) {
		return image.Rectangle{}, false
	}
	r := matches[p.bar.current].Rect
	return r, !r.Empty() && r.Min.Y >= 0 && r.Max.Y <= h
}

// countIn counts the pixels of img inside r that are c to within one level.
func countIn(img *image.RGBA, r image.Rectangle, c color.NRGBA) int {
	n := 0
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if near(pixelAt(img, image.Pt(x, y)), c, 1) {
				n++
			}
		}
	}
	return n
}

// findFills are the four fills the find puts on the light page: the guide's
// marks and the scroll bar's marks, each plain and current.
type findFills struct {
	match, current, trackMatch, trackCurrent color.NRGBA
}

func lightFindFills() findFills {
	st := docsMarkdownStyle(tokens.PlatformLight, tokens.DefaultTypography)
	bar := columnBar(tokens.PlatformLight, desktop.ScrollBarsWhenScrolling)
	return findFills{st.MatchFill, st.CurrentMatchFill, bar.MatchFill, bar.CurrentMatchFill}
}

// trackStrip is the strip at the guide column's trailing edge that the
// scroll bar draws in. The guide's text stops a padding short of it.
func trackStrip() image.Rectangle {
	bar := columnBar(tokens.PlatformLight, desktop.ScrollBarsWhenScrolling)
	r := findColumn
	r.Min.X = r.Max.X - int(bar.Width())
	return r
}

// As the user types, every match in the guide is marked: the current one at
// the highlight's full coverage and the others at the match coverage. The
// scroll bar's track shows where the matches are, the current one stronger,
// and the field counts them all.
func TestTypingMarksEveryMatchInTheGuideAndTheTrack(t *testing.T) {
	p := openFindPage(t)
	matches := p.doc.Matches()
	if len(matches) != findMatchCount {
		t.Fatalf("the guide holds %d matches of %q, want %d", len(matches), findQuery, findMatchCount)
	}
	if got := p.bar.count(); got != fmt.Sprint(findMatchCount) {
		t.Errorf("the field's count is %q, want %q", got, fmt.Sprint(findMatchCount))
	}
	if p.bar.current != 0 {
		t.Errorf("the current match is %d after typing, want 0", p.bar.current)
	}
	img := p.capture()
	f := lightFindFills()
	off := findColumn.Min
	for i, want := range []color.NRGBA{f.current, f.match} {
		r := matches[i].Rect
		if r.Empty() {
			t.Fatalf("match %d has no place on the first screen", i)
		}
		if n := countIn(img, r.Add(off), want); n < r.Dx() {
			t.Errorf("match %d at %v holds %d pixels of %v, want its box filled", i, r.Add(off), n, want)
		}
	}
	strip := trackStrip()
	if n := countIn(img, strip, f.trackCurrent); n == 0 {
		t.Errorf("the track holds no pixel of the current match's fill %v", f.trackCurrent)
	}
	if n := countIn(img, strip, f.trackMatch); n == 0 {
		t.Errorf("the track holds no pixel of the match fill %v", f.trackMatch)
	}
}

// Enter goes to the next match and Shift+Enter to the previous one, from the
// last to the first and from the first to the last. The current match
// scrolls into view and is drawn in the current match's fill.
func TestEnterAndShiftEnterStepTheCurrentMatchIntoView(t *testing.T) {
	p := openFindPage(t)
	h := findColumn.Dy()
	for _, step := range []struct {
		shift bool
		want  int
	}{
		{false, 1}, {false, 2}, {true, 1}, {true, 0}, {true, 5}, {false, 0}, {true, 5}, {true, 4},
	} {
		p.key(key.NameReturn, step.shift)
		if p.bar.current != step.want {
			t.Fatalf("Return with Shift %v makes match %d current, want %d", step.shift, p.bar.current, step.want)
		}
		if r, ok := p.inView(h); !ok {
			t.Errorf("match %d is at %v, want it inside the guide's %d rows", step.want, r, h)
		}
	}
	r, _ := p.inView(h)
	img := p.capture()
	if n := countIn(img, r.Add(findColumn.Min), lightFindFills().current); n < r.Dx() {
		t.Errorf("the current match at %v holds %d pixels of the current fill", r.Add(findColumn.Min), n)
	}
	if !p.inField() {
		t.Error("the keyboard left the field while Enter stepped")
	}
}

// The previous and next control steps the current match as Shift+Enter and
// Enter do, and the keyboard stays in the field.
func TestThePreviousAndNextControlStepsTheCurrentMatch(t *testing.T) {
	p := openFindPage(t)
	h := findColumn.Dy()
	// The control is 33 wide: 9, Done's 48 and 8 columns stand after it at
	// the page's trailing edge. Its middle row is 16 rows down in the bar.
	end := pageFrameSize.X - 9 - 48 - 8
	prev, next, y := end-33+8, end-8, 16
	for _, step := range []struct {
		x    int
		want int
	}{
		{next, 1}, {prev, 0}, {prev, 5}, {next, 0},
	} {
		p.press(step.x, y)
		if p.bar.current != step.want {
			t.Fatalf("a press at x=%d makes match %d current, want %d", step.x, p.bar.current, step.want)
		}
		if r, ok := p.inView(h); !ok {
			t.Errorf("match %d is at %v, want it inside the guide's %d rows", step.want, r, h)
		}
		if !p.inField() {
			t.Errorf("after a press at x=%d the keyboard is not in the field", step.x)
		}
	}
}

// The marks and the count leave when the field gives up the keyboard, and
// come back with the same current match when the keyboard goes back into
// the field with the query still there.
func TestTheMarksLeaveWithTheKeyboard(t *testing.T) {
	p := openFindPage(t)
	p.key(key.NameReturn, false)
	f := lightFindFills()

	p.focus(p.doc.Focus())
	if p.inField() {
		t.Fatal("the keyboard is still in the field")
	}
	if n := len(p.doc.Matches()); n != 0 {
		t.Errorf("the guide holds %d matches with the keyboard in the guide, want 0", n)
	}
	if got := p.bar.count(); got != "" {
		t.Errorf("the field's count is %q with the keyboard in the guide, want none", got)
	}
	img := p.capture()
	for _, c := range []color.NRGBA{f.match, f.current} {
		if n := countIn(img, findColumn, c); n != 0 {
			t.Errorf("the guide column holds %d pixels of %v with the keyboard in the guide", n, c)
		}
	}
	for _, c := range []color.NRGBA{f.trackMatch, f.trackCurrent} {
		if n := countIn(img, trackStrip(), c); n != 0 {
			t.Errorf("the track holds %d pixels of %v with the keyboard in the guide", n, c)
		}
	}

	p.focus(p.bar.field)
	if n := len(p.doc.Matches()); n != findMatchCount {
		t.Errorf("the guide holds %d matches with the keyboard back in the field, want %d", n, findMatchCount)
	}
	if got := p.bar.count(); got != fmt.Sprint(findMatchCount) {
		t.Errorf("the field's count is %q with the keyboard back in the field, want %q", got, fmt.Sprint(findMatchCount))
	}
	if p.bar.current != 1 {
		t.Errorf("the current match is %d with the keyboard back in the field, want 1", p.bar.current)
	}
}

// The clear mark empties the field, and the marks and the count go with the
// query.
func TestTheClearMarkEmptiesTheField(t *testing.T) {
	p := openFindPage(t)
	// The field ends 7 columns before the previous and next control. Its
	// clear mark is an 11 px disc 6 columns in from the field's trailing
	// edge, on the field's middle row.
	fieldEnd := pageFrameSize.X - 9 - 48 - 8 - 33 - 7
	p.press(fieldEnd-6-5, 16)
	if p.bar.query != "" {
		t.Errorf("the field holds %q after the clear mark, want nothing", p.bar.query)
	}
	if n := len(p.doc.Matches()); n != 0 {
		t.Errorf("the guide holds %d matches after the clear mark, want 0", n)
	}
	if got := p.bar.count(); got != "" {
		t.Errorf("the field's count is %q after the clear mark, want none", got)
	}
}

// When the bar closes, the guide does not move: the current match stays
// in view.
func TestClosingTheBarKeepsTheCurrentMatchInView(t *testing.T) {
	p := openFindPage(t)
	p.key(key.NameReturn, false)
	p.key(key.NameReturn, false)
	r, ok := p.inView(findColumn.Dy())
	if !ok {
		t.Fatalf("match 2 is at %v before the bar closes, want it in view", r)
	}
	before := p.doc.Position()
	p.key(key.NameEscape, false)
	if len(p.posted) != 1 || p.posted[0] != (CloseFind{}) {
		t.Fatalf("Escape posted %#v, want CloseFind{}", p.posted)
	}
	p.apply()
	p.frames(2)
	if p.model.findOpen {
		t.Fatal("the bar is open after CloseFind")
	}
	after := p.doc.Position()
	if after.First != before.First || after.Offset != before.Offset {
		t.Errorf("closing the bar moved the guide from %+v to %+v", before, after)
	}
	// The guide's viewport is taller by the bar's rows now and starts where
	// the bar started, so the match keeps its rows in the viewport.
	if h := pageFrameSize.Y; r.Max.Y > h {
		t.Errorf("match 2 at %v is below the guide's %d rows", r, h)
	}
}
