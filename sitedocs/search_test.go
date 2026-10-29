package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/composite"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// The menu bar holds Edit ▸ Find… on Command-F, posting FocusSearch: the
// platform's title, with its ellipsis, and the platform's chord.
func TestTheMenuBarHoldsFind(t *testing.T) {
	items := MenuItems()
	if len(items) != 1 {
		t.Fatalf("the menu declares %d items, want 1: %+v", len(items), items)
	}
	item := items[0]
	if item.Menu != "Edit" || item.Title != "Find…" || item.Key != "f" || item.Msg != (FocusSearch{}) {
		t.Errorf("the item is %+v, want Edit Find… on f posting FocusSearch{}", item)
	}
}

// pageFrameSize is the size the tests below lay the live window out at, at
// one pixel per dp.
var pageFrameSize = image.Pt(1000, 600)

// driveFrame lays one frame of w out through r.
func driveFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) {
	ops.Reset()
	w(layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      r.Source(),
	})
	r.Frame(ops)
}

// Each menu item's key equivalent posts the item's message from the window's
// own key area too. The bare key posts nothing.
func TestTheChordPostsTheItemsMessage(t *testing.T) {
	for _, item := range MenuItems() {
		name := key.Name(strings.ToUpper(item.Key))
		for _, tc := range []struct {
			mods key.Modifiers
			want int
		}{{key.ModShortcut, 1}, {0, 0}} {
			var posted []mvu.Message
			areas := ChordAreas(func(_ layout.Context, msg mvu.Message) { posted = append(posted, msg) })
			frame := func(gtx layout.Context) layout.Dimensions {
				for _, a := range areas {
					a(gtx)
				}
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			r := new(gioinput.Router)
			ops := new(op.Ops)
			driveFrame(frame, ops, r, pageFrameSize)
			r.Queue(key.Event{Name: name, Modifiers: tc.mods, State: key.Press})
			driveFrame(frame, ops, r, pageFrameSize)
			if len(posted) != tc.want || (tc.want == 1 && posted[0] != item.Msg) {
				t.Errorf("%q with modifiers %v posts %#v, want %d of %#v", name, tc.mods, posted, tc.want, item.Msg)
			}
		}
	}
}

// FocusSearch counts one request each time.
func TestUpdateCountsSearchRequests(t *testing.T) {
	m := initialModel()
	if m.searchRequests != 0 {
		t.Fatalf("the window opens with %d search requests, want 0", m.searchRequests)
	}
	for want := 1; want <= 2; want++ {
		m, _ = Update(m, FocusSearch{})
		if m.searchRequests != want {
			t.Errorf("FocusSearch gives %d requests, want %d", m.searchRequests, want)
		}
	}
}

// livePage is the live window under test, the toolbar included: its model
// input, the latest layout it emitted, and what its chords posted.
type livePage struct {
	t      *testing.T
	send   rx.Observer[Model]
	model  Model
	doc    *markdown.Document
	find   *finder
	r      *gioinput.Router
	ops    *op.Ops
	mu     sync.Mutex
	latest layout.Widget
	gen    int
	posted []mvu.Message
}

func newLivePage(t *testing.T) *livePage {
	t.Helper()
	return newLivePageFrom(t, guideFixture(t))
}

// newLivePageFrom is newLivePage over the guide source.
func newLivePageFrom(t *testing.T, source []byte) *livePage {
	t.Helper()
	p := &livePage{t: t, r: new(gioinput.Router), ops: new(op.Ops)}
	send, modelObs := rx.Subject[Model](0, 1)
	p.send = send
	th := rx.Of(golden.StaticTheme(tokens.PlatformLight))
	post := func(_ layout.Context, msg mvu.Message) { p.posted = append(p.posted, msg) }
	content, doc, f := docsWindow(th, modelObs, source, nil, desktop.ScrollBarsWhenScrolling, post)
	p.doc, p.find = doc, f
	sub := content.Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
		if !done && err == nil && w != nil {
			p.mu.Lock()
			p.latest = w
			p.gen++
			p.mu.Unlock()
		}
	})
	t.Cleanup(sub.Unsubscribe)
	p.set(initialModel())
	return p
}

// set hands the window model, waits for the window to emit for it, and lays
// out two frames.
func (p *livePage) set(m Model) {
	p.t.Helper()
	p.mu.Lock()
	before := p.gen
	p.mu.Unlock()
	p.model = m
	p.send.Next(m)
	deadline := time.Now().Add(2 * time.Second)
	for {
		p.mu.Lock()
		gen := p.gen
		p.mu.Unlock()
		if gen > before {
			break
		}
		if time.Now().After(deadline) {
			p.t.Fatal("the window did not emit for the model")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// The window emits once per input; let the emissions for this model
	// settle.
	time.Sleep(20 * time.Millisecond)
	p.frame()
	p.frame()
}

func (p *livePage) frame() {
	p.mu.Lock()
	w := p.latest
	p.mu.Unlock()
	driveFrame(w, p.ops, p.r, pageFrameSize)
}

// frames lays out n frames. A match off screen takes two frames to scroll
// into view, and the test router asks for no frame by itself.
func (p *livePage) frames(n int) {
	for range n {
		p.frame()
	}
}

// apply reduces every posted message into the model and hands the window
// the result.
func (p *livePage) apply() {
	p.t.Helper()
	m := p.model
	for _, msg := range p.posted {
		m, _ = Update(m, msg)
	}
	p.posted = nil
	p.set(m)
}

func (p *livePage) inField() bool { return p.find.field != nil && p.r.Source().Focused(p.find.field) }
func (p *livePage) inGuide() bool { return p.r.Source().Focused(p.doc.Focus()) }

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

// press clicks the pointer at (x, y) in the window.
func (p *livePage) press(x, y int) {
	at := f32.Pt(float32(x), float32(y))
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse},
	)
	p.frames(4)
}

// key presses name, with Shift when shift is true.
func (p *livePage) key(name key.Name, shift bool) {
	var mods key.Modifiers
	if shift {
		mods = key.ModShift
	}
	p.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	p.frames(4)
}

// findCommand presses ⌘F and reduces what it posts.
func (p *livePage) findCommand() {
	p.t.Helper()
	p.r.Queue(key.Event{Name: "F", Modifiers: key.ModShortcut, State: key.Press})
	p.frame()
	if len(p.posted) != 1 || p.posted[0] != (FocusSearch{}) {
		p.t.Fatalf("⌘F posted %#v, want FocusSearch{}", p.posted)
	}
	p.apply()
}

// ⌘F gives the keyboard to the toolbar's search field, from the guide too.
// Escape in the empty field posts nothing and leaves the keyboard in it.
func TestFindGivesTheKeyboardToTheToolbarsField(t *testing.T) {
	p := newLivePage(t)
	p.findCommand()
	if !p.inField() {
		t.Fatal("after ⌘F the keyboard is not in the toolbar's search field")
	}
	p.key(key.NameEscape, false)
	if len(p.posted) != 0 {
		t.Errorf("Escape in the empty field posted %#v, want nothing", p.posted)
	}
	if !p.inField() {
		t.Error("after Escape in the empty field the keyboard is not in the field")
	}

	p.focus(p.doc.Focus())
	if !p.inGuide() {
		t.Fatal("the keyboard is not in the guide")
	}
	p.findCommand()
	if !p.inField() {
		t.Error("after ⌘F from the guide the keyboard is not in the field")
	}
}

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

// findColumn is the guide column's frame in the live window: right of the
// rail and under the toolbar, to the window's trailing edge.
var findColumn = image.Rect(docsOutlineWidthDp, toolbarDp, pageFrameSize.X, pageFrameSize.Y)

// queryPage is the live window over the long guide with the query typed
// into the toolbar's search field.
func queryPage(t *testing.T) *livePage {
	t.Helper()
	p := newLivePageFrom(t, longGuide())
	p.findCommand()
	if !p.inField() {
		t.Fatal("the keyboard is not in the field after ⌘F")
	}
	p.r.Queue(key.EditEvent{Text: findQuery})
	p.frames(4)
	return p
}

// capture lays one frame of the live window out and renders it offscreen.
func (p *livePage) capture() *image.RGBA {
	p.t.Helper()
	w, err := headless.NewWindow(pageFrameSize.X, pageFrameSize.Y)
	if err != nil {
		p.t.Skipf("headless rendering not supported: %v", err)
	}
	defer w.Release()
	p.mu.Lock()
	page := p.latest
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
	if p.find.current < 0 || p.find.current >= len(matches) {
		return image.Rectangle{}, false
	}
	r := matches[p.find.current].Rect
	return r, !r.Empty() && r.Min.Y >= 0 && r.Max.Y <= h
}

func near(a, b color.NRGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
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

// marked checks that the guide and the track show the find: the first two
// matches on the first screen marked, the first one current when
// firstCurrent is true, and both track fills present.
func (p *livePage) marked(what string, firstCurrent bool) {
	p.t.Helper()
	matches := p.doc.Matches()
	if len(matches) != findMatchCount {
		p.t.Fatalf("%s: the guide holds %d matches of %q, want %d", what, len(matches), findQuery, findMatchCount)
	}
	img := p.capture()
	f := lightFindFills()
	wants := []color.NRGBA{f.match, f.match}
	if firstCurrent {
		wants[0] = f.current
	}
	for i, want := range wants {
		r := matches[i].Rect
		if r.Empty() {
			p.t.Fatalf("%s: match %d has no place on the screen", what, i)
		}
		if n := countIn(img, r.Add(findColumn.Min), want); n < r.Dx() {
			p.t.Errorf("%s: match %d at %v holds %d pixels of %v, want its box filled", what, i, r.Add(findColumn.Min), n, want)
		}
	}
	strip := trackStrip()
	if n := countIn(img, strip, f.trackCurrent); n == 0 {
		p.t.Errorf("%s: the track holds no pixel of the current match's fill %v", what, f.trackCurrent)
	}
	if n := countIn(img, strip, f.trackMatch); n == 0 {
		p.t.Errorf("%s: the track holds no pixel of the match fill %v", what, f.trackMatch)
	}
}

// unmarked checks that the find is gone: no query, no matches, no count, and
// no mark in the guide column or the track.
func (p *livePage) unmarked(what string) {
	p.t.Helper()
	if p.find.query != "" {
		p.t.Errorf("%s: the field holds %q, want nothing", what, p.find.query)
	}
	if n := len(p.doc.Matches()); n != 0 {
		p.t.Errorf("%s: the guide holds %d matches, want 0", what, n)
	}
	if got := p.find.count(); got != "" {
		p.t.Errorf("%s: the field's count is %q, want none", what, got)
	}
	img := p.capture()
	f := lightFindFills()
	for _, c := range []color.NRGBA{f.match, f.current} {
		if n := countIn(img, findColumn, c); n != 0 {
			p.t.Errorf("%s: the guide column holds %d pixels of %v", what, n, c)
		}
	}
	for _, c := range []color.NRGBA{f.trackMatch, f.trackCurrent} {
		if n := countIn(img, trackStrip(), c); n != 0 {
			p.t.Errorf("%s: the track holds %d pixels of %v", what, n, c)
		}
	}
}

// As the user types in the toolbar's field, every match in the guide is
// marked in the match fill and the current one in the current match's fill.
// The scroll bar's track shows where the matches are in the same two fills,
// and the field counts them all.
func TestTypingMarksEveryMatchInTheGuideAndTheTrack(t *testing.T) {
	p := queryPage(t)
	if got := p.find.count(); got != fmt.Sprint(findMatchCount) {
		t.Errorf("the field's count is %q, want %q", got, fmt.Sprint(findMatchCount))
	}
	if p.find.current != 0 {
		t.Errorf("the current match is %d after typing, want 0", p.find.current)
	}
	p.marked("after typing", true)
}

// The field draws its count inside it: the field's box with the query and
// its count differs from the same box with the count taken away.
func TestTheFieldDrawsTheCount(t *testing.T) {
	p := queryPage(t)
	box := image.Rect(pageFrameSize.X-8-toolbarSearchWDp, 0, pageFrameSize.X-8, toolbarDp)
	with := p.capture()
	q := p.find.query
	p.find.query = ""
	without := p.capture()
	p.find.query = q
	differ := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if pixelAt(with, image.Pt(x, y)) != pixelAt(without, image.Pt(x, y)) {
				differ++
			}
		}
	}
	if differ == 0 {
		t.Error("the field draws nothing for its count")
	}
}

// Enter goes to the next match and Shift+Enter to the previous one, from the
// last to the first and from the first to the last. The current match
// scrolls into view and is drawn in the current match's fill. The keyboard
// stays in the field, and the editor does not take the keys.
func TestEnterAndShiftEnterStepTheCurrentMatchIntoView(t *testing.T) {
	p := queryPage(t)
	h := findColumn.Dy()
	for _, step := range []struct {
		shift bool
		want  int
	}{
		{false, 1}, {false, 2}, {true, 1}, {true, 0}, {true, 5}, {false, 0}, {true, 5}, {true, 4},
	} {
		p.key(key.NameReturn, step.shift)
		if p.find.current != step.want {
			t.Fatalf("Return with Shift %v makes match %d current, want %d", step.shift, p.find.current, step.want)
		}
		if r, ok := p.inView(h); !ok {
			t.Errorf("match %d is at %v, want it inside the guide's %d rows", step.want, r, h)
		}
	}
	p.key(key.NameEnter, false)
	if p.find.current != 5 {
		t.Errorf("Enter makes match %d current, want 5", p.find.current)
	}
	r, _ := p.inView(h)
	img := p.capture()
	if n := countIn(img, r.Add(findColumn.Min), lightFindFills().current); n < r.Dx() {
		t.Errorf("the current match at %v holds %d pixels of the current fill", r.Add(findColumn.Min), n)
	}
	if !p.inField() {
		t.Error("the keyboard left the field while Enter stepped")
	}
	if p.find.query != findQuery {
		t.Errorf("the field holds %q after stepping, want %q", p.find.query, findQuery)
	}
}

// The marks and the count stay while the query is in the field, wherever
// the keyboard is: the user clicks and scrolls the guide with the marks
// shown.
func TestTheMarksStayWhenTheKeyboardMovesToTheGuide(t *testing.T) {
	p := queryPage(t)
	p.focus(p.doc.Focus())
	if p.inField() {
		t.Fatal("the keyboard is still in the field")
	}
	if got := p.find.count(); got != fmt.Sprint(findMatchCount) {
		t.Errorf("the field's count is %q with the keyboard in the guide, want %q", got, fmt.Sprint(findMatchCount))
	}
	p.marked("with the keyboard in the guide", true)

	// A click in the guide's text keeps the marks too.
	p.press(findColumn.Min.X+200, findColumn.Min.Y+300)
	if p.inField() {
		t.Error("the keyboard is in the field after a click in the guide")
	}
	if got := p.find.count(); got != fmt.Sprint(findMatchCount) {
		t.Errorf("the field's count is %q after a click in the guide, want %q", got, fmt.Sprint(findMatchCount))
	}
	p.marked("after a click in the guide", true)
}

// Escape in the field with text empties it and keeps the keyboard there,
// and the marks and the count leave with the query.
func TestEscapeEmptiesTheFieldAndKeepsTheKeyboard(t *testing.T) {
	p := queryPage(t)
	p.key(key.NameEscape, false)
	if len(p.posted) != 0 {
		t.Errorf("Escape posted %#v, want nothing", p.posted)
	}
	if !p.inField() {
		t.Error("after Escape the keyboard is not in the field")
	}
	p.unmarked("after Escape")
}

// The clear mark empties the field, and the marks and the count go with the
// query. The field ends 8 columns before the window's trailing edge; its
// clear mark is a 13 px disc that ends 15 columns inside the field's edge,
// on the toolbar's middle row.
func TestTheClearMarkEmptiesTheField(t *testing.T) {
	p := queryPage(t)
	p.focus(p.doc.Focus())
	p.press(pageFrameSize.X-8-15-7, toolbarDp/2)
	p.unmarked("after the clear mark")
}

// Deleting the query's text takes the marks and the count off: ⌘A selects
// the text in the field and Delete removes it.
func TestDeletingTheTextRemovesTheMarks(t *testing.T) {
	p := queryPage(t)
	p.r.Queue(key.Event{Name: "A", Modifiers: key.ModShortcut, State: key.Press})
	p.frames(2)
	p.key(key.NameDeleteBackward, false)
	p.unmarked("after the text is deleted")
}
