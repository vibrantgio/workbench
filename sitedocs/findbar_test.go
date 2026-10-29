package main

import (
	"image"
	"image/color"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// The menu bar holds Edit ▸ Find… on Command-F, posting OpenFind: the
// platform's title, with its ellipsis, and the platform's chord.
func TestTheMenuBarHoldsFind(t *testing.T) {
	items := MenuItems()
	if len(items) != 1 {
		t.Fatalf("the menu declares %d items, want 1: %+v", len(items), items)
	}
	item := items[0]
	if item.Menu != "Edit" || item.Title != "Find…" || item.Key != "f" || item.Msg != (OpenFind{}) {
		t.Errorf("the item is %+v, want Edit Find… on f posting OpenFind{}", item)
	}
}

// pageFrameSize is the size the page tests below lay the live page out at,
// at one pixel per dp.
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

// OpenFind opens the bar and counts a keyboard move each time. CloseFind
// closes it and counts one move; with the bar closed it does nothing.
func TestUpdateOpensAndClosesTheFindBar(t *testing.T) {
	m := initialModel()
	if m.findOpen || m.findMoves != 0 {
		t.Fatalf("the window opens with the find bar open=%v moves=%d, want closed and 0", m.findOpen, m.findMoves)
	}
	m, _ = Update(m, CloseFind{})
	if m.findOpen || m.findMoves != 0 {
		t.Errorf("CloseFind on a closed bar gives open=%v moves=%d, want no change", m.findOpen, m.findMoves)
	}
	m, _ = Update(m, OpenFind{})
	if !m.findOpen || m.findMoves != 1 {
		t.Errorf("OpenFind gives open=%v moves=%d, want open and 1", m.findOpen, m.findMoves)
	}
	m, _ = Update(m, OpenFind{})
	if !m.findOpen || m.findMoves != 2 {
		t.Errorf("a second OpenFind gives open=%v moves=%d, want open and 2", m.findOpen, m.findMoves)
	}
	m, _ = Update(m, CloseFind{})
	if m.findOpen || m.findMoves != 3 {
		t.Errorf("CloseFind gives open=%v moves=%d, want closed and 3", m.findOpen, m.findMoves)
	}
}

// livePage is the live page under test: its model input, the latest layout
// it emitted, and what its chords, Escape and Done posted.
type livePage struct {
	t      *testing.T
	send   rx.Observer[Model]
	model  Model
	doc    *markdown.Document
	bar    *findBar
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
	page, doc, bar := docsPage(th, modelObs, source, desktop.ScrollBarsWhenScrolling, post)
	p.doc, p.bar = doc, bar
	sub := page.Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
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

// set hands the page model, waits for the page to emit for it, and lays out
// two frames.
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
			p.t.Fatal("the page did not emit for the model")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// The page emits once per input; let the emissions for this model settle.
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

// apply reduces every posted message into the model and hands the page the
// result.
func (p *livePage) apply() {
	p.t.Helper()
	m := p.model
	for _, msg := range p.posted {
		m, _ = Update(m, msg)
	}
	p.posted = nil
	p.set(m)
}

func (p *livePage) inField() bool { return p.bar.field != nil && p.r.Source().Focused(p.bar.field) }
func (p *livePage) inGuide() bool { return p.r.Source().Focused(p.doc.Focus()) }

// ⌘F opens the bar with the keyboard in its field. Escape closes it at once
// with the keyboard in the guide, text typed or none, and so does Done.
func TestFindOpensWithTheKeyboardInTheFieldAndClosesBackToTheGuide(t *testing.T) {
	p := newLivePage(t)

	open := func(what string) {
		t.Helper()
		p.r.Queue(key.Event{Name: "F", Modifiers: key.ModShortcut, State: key.Press})
		p.frame()
		if len(p.posted) != 1 || p.posted[0] != (OpenFind{}) {
			t.Fatalf("%s: ⌘F posted %#v, want OpenFind{}", what, p.posted)
		}
		p.apply()
		if !p.model.findOpen {
			t.Fatalf("%s: the bar is closed after OpenFind", what)
		}
		if !p.inField() {
			t.Errorf("%s: after ⌘F the keyboard is not in the find bar's field", what)
		}
	}
	closed := func(what string) {
		t.Helper()
		if len(p.posted) != 1 || p.posted[0] != (CloseFind{}) {
			t.Fatalf("%s posted %#v, want CloseFind{}", what, p.posted)
		}
		p.apply()
		if p.model.findOpen {
			t.Fatalf("%s: the bar is open after CloseFind", what)
		}
		if !p.inGuide() {
			t.Errorf("after %s the keyboard is not in the guide", what)
		}
	}

	open("first open")
	p.r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	p.frame()
	closed("Escape in an empty field")

	open("second open")
	p.r.Queue(key.EditEvent{Text: "Gio"})
	p.frame()
	p.frame()
	p.r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	p.frame()
	closed("Escape with text typed")

	open("third open")
	// Done is the bar's trailing part: 48 by 20, 9 px in from the window's
	// trailing edge and 6 rows down in the bar.
	at := f32.Pt(float32(pageFrameSize.X-9-24), 6+10)
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse},
	)
	p.frame()
	closed("Done")
}

// findBarFrame is the window at its own size with the first section open,
// under the toolbar, with the find bar in the state find. With seat true
// the first section is selected and the guide is seated at its heading;
// otherwise the guide is at its top.
//
// The toolbar's search field casts its measured shadow below the strip's
// foot, onto the bar's top rows. The TextEdit captures that these tests
// read the bar against have no toolbar, so these frames put an empty box of
// the field's width in the field's place.
func findBarFrame(t *testing.T, c tokens.PlatformColors, find *input.RenderState, seat bool) *image.RGBA {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	guide := guideFixture(t)
	st := outlineState{open: map[int]bool{0: true}, selected: -1}
	if seat {
		st.selected = guideOutline(markdown.Parse(guide))[0].Block
	}
	page := onPageFill(c, renderDocsPage(shaper, guide, st, c, tokens.DefaultTypography, find))
	background := backdrop.Widget(c.WindowBackground)
	strip := withToolbar(toolbarDepth, c, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}, page)
	return golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
		background(gtx)
		return strip(gtx)
	})
}

// runs reads the row y of img from x0 to x1 inclusive as alternating runs of
// columns on fill and off it, starting with the first column's kind. A
// column is off fill when any channel differs by more than 2 levels.
func runs(img *image.RGBA, fill color.NRGBA, y, x0, x1 int) []int {
	var out []int
	prev, n := false, 0
	for x := x0; x <= x1; x++ {
		off := !near(pixelAt(img, image.Pt(x, y)), fill, 2)
		if x > x0 && off != prev {
			out = append(out, n)
			n = 0
		}
		prev = off
		n++
	}
	return append(out, n)
}

// offRows reads the column x of img from y0 to y1 inclusive and returns the
// first and last row off fill.
func offRows(img *image.RGBA, fill color.NRGBA, x, y0, y1 int) (first, last int) {
	first, last = -1, -1
	for y := y0; y <= y1; y++ {
		if !near(pixelAt(img, image.Pt(x, y)), fill, 2) {
			if first < 0 {
				first = y
			}
			last = y
		}
	}
	return first, last
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

// The bar stands over the guide column and over no other, in the guide
// column's fill with the seam under it. Its height and its parts are the
// reference's rows, read at the bar's middle row as in
// textedit-find-unfocused-{light,dark}.png: 31 rows of fill and a 1 px seam;
// 7 clear columns, the field, 7, the previous and next control 33 wide, 8,
// Done 48 wide, 9. The field is 22 rows with 5 above it, the buttons 20 rows
// with 6 above them.
func TestTheFindBarStandsOverTheGuideColumnOnly(t *testing.T) {
	rail := docsOutlineWidthDp
	top := toolbarDp
	w := windowSize.X
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			// The field holds a query without the keyboard: the reference
			// reads the field's edge off the unfocused captures, where no
			// ring covers it.
			closedImg := findBarFrame(t, tc.c, nil, true)
			img := findBarFrame(t, tc.c, &input.RenderState{Text: "Gio"}, true)
			fill := pageFill(tc.c)

			// The rail keeps its fill and its height: every pixel left of
			// the guide column is the closed window's.
			for y := 0; y < windowSize.Y; y++ {
				for x := 0; x < rail; x++ {
					if a, b := pixelAt(img, image.Pt(x, y)), pixelAt(closedImg, image.Pt(x, y)); a != b {
						t.Fatalf("the rail at (%d,%d) is %v with the bar open, %v closed", x, y, a, b)
					}
				}
			}
			// The bar's fill at both ends of every row, and the seam under it.
			for y := top; y < top+findBarFillDp; y++ {
				for _, x := range []int{rail, rail + findBarLeadDp - 1, w - findBarTrailDp, w - 1} {
					if got := pixelAt(img, image.Pt(x, y)); got != fill {
						t.Errorf("the bar at (%d,%d) is %v, want the guide column's fill %v", x, y, got, fill)
					}
				}
			}
			seam := findBarSeam(tc.c)
			for x := rail; x < w; x++ {
				if got := pixelAt(img, image.Pt(x, top+findBarFillDp)); got != seam {
					t.Fatalf("the seam at x=%d is %v, want the separator over the fill %v", x, got, seam)
				}
			}

			// The parts, leading to trailing, at the bar's middle row.
			mid := top + 15
			got := runs(img, fill, mid, rail, w-1)
			if len(got) < 2 || got[0] != 7 {
				t.Fatalf("the bar's middle row reads %v, want 7 clear columns before the field", got)
			}
			tail := got[len(got)-6:]
			if want := []int{7, 33, 8, 48, 9}; tail[1] != want[0] || tail[2] != want[1] || tail[3] != want[2] || tail[4] != want[3] || tail[5] != want[4] {
				t.Errorf("the bar's trailing runs read %v, want the field's edge, then %v", tail, want)
			}
			fieldStart := rail + 7
			fieldEnd := w - 1 - 9 - 48 - 8 - 33 - 7
			if f, l := offRows(img, fill, fieldStart+60, top, top+findBarFillDp-1); f != top+5 || l != top+26 {
				t.Errorf("the field covers rows %d–%d of the bar, want 5–26", f-top, l-top)
			}
			doneX := w - 1 - 9 - 24
			if f, l := offRows(img, fill, doneX-20, top, top+findBarFillDp-1); f != top+6 || l != top+25 {
				t.Errorf("Done covers rows %d–%d of the bar, want 6–25", f-top, l-top)
			}
			stepsX := fieldEnd + 1 + 7 + 3
			if f, l := offRows(img, fill, stepsX, top, top+findBarFillDp-1); f != top+6 || l != top+25 {
				t.Errorf("the previous and next control covers rows %d–%d of the bar, want 6–25", f-top, l-top)
			}
		})
	}
}

// The bar covers no part of the guide: the guide moves down by the bar's
// height and draws what it drew before, row for row, to 8 levels: the
// renderer moves a few dozen antialiased text pixels by up to 4 levels when
// the text stands at another row, where a guide off by one row moves whole
// glyph edges. The guide is at its top
// in both frames: a guide seated at a heading near its end can scroll further
// in the shorter column.
func TestTheFindBarHidesNoPartOfTheGuide(t *testing.T) {
	rail := docsOutlineWidthDp
	top := toolbarDp
	h := findBarFillDp + 1
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			closedImg := findBarFrame(t, tc.c, nil, false)
			img := findBarFrame(t, tc.c, &input.RenderState{}, false)
			for y := top; y+h < windowSize.Y; y++ {
				for x := rail; x < windowSize.X; x++ {
					if a, b := pixelAt(closedImg, image.Pt(x, y)), pixelAt(img, image.Pt(x, y+h)); !near(a, b, 8) {
						t.Fatalf("the guide at (%d,%d) closed is %v, at (%d,%d) open %v", x, y, a, x, y+h, b)
					}
				}
			}
		})
	}
}
