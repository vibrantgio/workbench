package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/sidebar"
)

// railFocusPaths are the notes railFocusModel holds, in the order the rail
// draws them: all at the root, so every row is a note and a click lands on
// one with no folder in the way.
var railFocusPaths = []string{"a.md", "b.md", "c.md", "d.md"}

// railFocusModel is a vault over those notes with none of them open, which is
// the state a window opens in.
func railFocusModel() Model {
	m := Model{Screen: screenVault, Vault: "/v", CurAnchor: -1}
	m.Index = treeIndex(railFocusPaths...)
	for _, p := range railFocusPaths {
		m = cacheNote(m, noteFromSource(p, "# "+p+"\n\nprose\n"))
	}
	return m
}

// railFocusFrame lays the folder rail and the note column out side by side
// against one router, and keeps the model as the loop does: a message a
// column posts is reduced into the model the next frame lays out. Both
// columns are the live ones and the keyboard is the router's, so where the
// keys are after a frame is where the running window would have them.
//
// A focus command reaches the router at the end of the frame that executed
// it, so what a frame reads is where the keys were put by the frame before
// it: every assertion below is made one frame after the press it is about.
type railFocusFrame struct {
	t      *testing.T
	v      *treeView
	read   reader
	model  Model
	tok    themeTokens
	router *input.Router
	posted []mvu.Message
	docs   map[string]*markdown.Document
	size   image.Point

	onRail bool
	inNote bool
}

func newRailFocusFrame(t *testing.T) *railFocusFrame {
	t.Helper()
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return goldenLeading }}
	f := &railFocusFrame{
		t:      t,
		v:      v,
		model:  railFocusModel(),
		tok:    goldenTokens(),
		router: new(input.Router),
		docs:   map[string]*markdown.Document{},
		size:   image.Pt(treeWidthDp+400, 700),
	}
	f.read.rail = &railFocus{tag: v.list.Focus()}
	f.v.post = func(_ layout.Context, msg mvu.Message) { f.posted = append(f.posted, msg) }
	return f
}

// doc is the document the note column shows: one per path, so a landing on
// another note hands the column a different pointer, which is the arrival the
// reader answers.
func (f *railFocusFrame) doc() *markdown.Document {
	note := f.model.CurrentNote()
	if note == nil {
		return nil
	}
	d := f.docs[note.Path]
	if d == nil {
		d = markdown.NewDocument(note.Blocks)
		f.docs[note.Path] = d
	}
	return d
}

func (f *railFocusFrame) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(f.size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      f.router.Source(),
	}
	f.columns(gtx)
	f.onRail = gtx.Focused(f.v.list.Focus())
	f.inNote = gtx.Focused(&f.read.tag)
	f.router.Frame(ops)
	for _, msg := range f.posted {
		f.model, _ = Update(f.model, msg)
	}
	f.posted = nil
}

// columns lays the rail out at its own width and the note column beside it,
// which is the arrangement the window's frame makes of the two.
func (f *railFocusFrame) columns(gtx layout.Context) {
	rail := gtx
	rail.Constraints = layout.Exact(image.Pt(treeWidthDp, f.size.Y))
	f.v.layout(rail, f.model, f.tok, nil)
	off := op.Offset(image.Pt(treeWidthDp, 0)).Push(gtx.Ops)
	note := gtx
	note.Constraints = layout.Exact(image.Pt(f.size.X-treeWidthDp, f.size.Y))
	f.read.layout(note, f.doc(), func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	off.Pop()
}

// railRowHeight is the height of one rail row in this frame's pixels, which
// is what turns a row index into a place to press and a pill back into a row.
func railRowHeight() int {
	return unit.Metric{PxPerDp: 1, PxPerSp: 1}.Dp(sidebar.RowHeight)
}

func (f *railFocusFrame) click(at f32.Point) {
	f.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	f.frame()
}

// clickRailRow presses the middle of the rail row at i. The rows run from the
// band the last layout arranged, one row height apart.
func (f *railFocusFrame) clickRailRow(i int) {
	f.t.Helper()
	h := railRowHeight()
	f.click(f32.Pt(float32(treeWidthDp)/2, float32(f.v.geom.rows.Min.Y+i*h+h/2)))
}

// clickInNote presses the middle of the note column, where the document is.
func (f *railFocusFrame) clickInNote() {
	f.t.Helper()
	f.click(f32.Pt(float32(treeWidthDp+(f.size.X-treeWidthDp)/2), float32(f.size.Y)/2))
}

func (f *railFocusFrame) press(name key.Name) {
	f.t.Helper()
	f.router.Queue(key.Event{Name: name, State: key.Press})
	f.frame()
}

// navigate posts the landing a followed link posts and lays out the frame the
// new document arrives on, then one more so the frame's focus command can be
// read back.
func (f *railFocusFrame) navigate(path string) {
	f.t.Helper()
	f.model, _ = Update(f.model, Navigate{Path: path})
	f.frame()
	f.frame()
}

// emphasizedPillRow answers which rail row wears the emphasized pill, and -1
// when no row does. It reads the pill's fill near the rail's trailing edge,
// past where a row's label reaches, so only the pill itself is scanned.
func (f *railFocusFrame) emphasizedPillRow(t *testing.T) int {
	t.Helper()
	img := golden.Capture(t, image.Pt(treeWidthDp, f.size.Y), scene(func(gtx layout.Context) layout.Dimensions {
		gtx.Source = f.router.Source()
		return f.v.layout(gtx, f.model, f.tok, nil)
	}, themeCases[0].bg))
	want := sidebar.SelectionFill(f.tok.col, false)
	x := treeWidthDp - int(sidebar.SelectionInset) - 4
	lo, hi := -1, -1
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		c := img.RGBAAt(x, y)
		if c.R == want.R && c.G == want.G && c.B == want.B {
			if lo < 0 {
				lo = y
			}
			hi = y
		}
	}
	if lo < 0 {
		return -1
	}
	return ((lo+hi)/2 - f.v.geom.rows.Min.Y) / railRowHeight()
}

// TestARailClickLeavesTheKeyboardInTheRail drives a click on a note row
// through a real router and reads where the keys are on the frame the note
// the click opened arrives on.
//
// The click opens a note, so the note column is handed a document it was not
// showing — and that arrival is the moment the column would otherwise claim
// the keyboard, one frame after the click put it in the rail. The platform's
// sidebars keep it: the arrows go on walking the rows, and the row they land
// on wears the emphasized pill.
func TestARailClickLeavesTheKeyboardInTheRail(t *testing.T) {
	f := newRailFocusFrame(t)
	f.frame() // registers the tags and measures the rows

	f.clickRailRow(0)
	if f.model.Current != railFocusPaths[0] {
		t.Fatalf("the click opened %q, want %q: it did not land on the first row",
			f.model.Current, railFocusPaths[0])
	}
	f.frame() // the frame the opened note's document arrives on
	f.frame() // and the frame that reads back what that one did with the keys
	if !f.onRail {
		t.Fatal("the rail lost the keyboard on the frame the opened note arrived: the note column took it")
	}
	if f.inNote {
		t.Error("the note column holds the keyboard after a click in the rail")
	}

	f.press(key.NameDownArrow)
	if got := f.v.list.Selected(); got != 1 {
		t.Fatalf("Down left the rail's selection on row %d, want the second row: the arrows no longer move it", got)
	}
	if !f.onRail {
		t.Fatal("the rail does not hold the keyboard after Down")
	}
	if got := f.emphasizedPillRow(t); got != 1 {
		t.Errorf("the emphasized pill stands on row %d, want the row Down landed on", got)
	}
}

// TestAFollowedLinkKeepsTheKeyboardInTheNote reads the other half: a landing
// the rail did not cause leaves the keys in the document.
//
// The first note a window shows, back and forward all land with the rail
// empty-handed, and the column takes the keys so the note can be read at
// once. A link is followed by pressing it in the document, which is itself
// what hands the column the keys; the landing that follows keeps them there.
func TestAFollowedLinkKeepsTheKeyboardInTheNote(t *testing.T) {
	f := newRailFocusFrame(t)
	f.frame() // registers the tags

	f.navigate(railFocusPaths[0])
	if !f.inNote {
		t.Fatal("the note column did not take the keyboard on a landing the rail did not cause")
	}

	f.clickInNote()
	f.frame() // the press reaches the router at the end of the frame it landed in
	if !f.inNote {
		t.Fatal("a press in the document did not put the keyboard in the note")
	}
	f.navigate(railFocusPaths[1])
	if !f.inNote {
		t.Error("a followed link left the keyboard out of the note")
	}
	if f.onRail {
		t.Error("the rail took the keyboard on a followed link")
	}
}
