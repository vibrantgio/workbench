package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// findPad is a note pad with the find field open over the page and the
// keyboard in it: the state every key below is pressed in. The field itself
// draws nothing — what these tests are about is the keys, the query and the
// marks, and the rival tag holds the keyboard exactly as the field's editor
// does in the running window.
func findPad(t *testing.T, m Model) *notePad {
	t.Helper()
	p := newNotePad(t, m)
	p.find.tag = &p.rival
	p.frame()
	p.press(findKey, key.ModShortcut)
	p.takeFocus = true
	p.frame()
	return p
}

// typeQuery is what the field reports as the reader types, which is the one
// way a query reaches the page.
func typeQuery(p *notePad, query string) {
	p.find.typed(query)
	p.frame()
	p.frame()
}

// TestTheFindShortcutOpensTheFieldOverThePage is the invocation: the
// platform's find shortcut opens the field, and nothing was open before it
// was pressed.
func TestTheFindShortcutOpensTheFieldOverThePage(t *testing.T) {
	p := newNotePad(t, findModel(-1))
	p.frame()
	if p.find.open {
		t.Fatal("the find field was open before anybody asked for it")
	}
	p.press(findKey, key.ModShortcut)
	if !p.find.open {
		t.Error("the platform's find shortcut did not open the find field")
	}
}

// TestTypingMarksTheMatchesAsTheyCome is the marking: what the field reports
// reaches the document, which marks every match of it and says how many there
// are.
func TestTypingMarksTheMatchesAsTheyCome(t *testing.T) {
	p := findPad(t, findModel(-1))
	typeQuery(p, findQuery)

	if n := len(p.doc.Matches()); n != findMatches {
		t.Errorf("the query found %d matches in the note, want %d", n, findMatches)
	}
	if p.find.count != findMatches {
		t.Errorf("the field says %d matches and the note holds %d", p.find.count, findMatches)
	}
	style := noteStyle(p.tok.col, p.tok.typ)
	img := p.shot(t)
	if n := markPixels(img, style.MatchFill) + markPixels(img, style.CurrentMatchFill); n == 0 {
		t.Error("nothing on the page wears a match fill; the matches are not marked")
	}
}

// TestEnterStepsThroughTheMatchesAndBringsThemIntoView is the stepping: Enter
// takes the reader to the next match and Shift+Enter back to the one before,
// each wrapping at the note's ends, and the page moves so that the match the
// reader is on is on the page.
func TestEnterStepsThroughTheMatchesAndBringsThemIntoView(t *testing.T) {
	p := findPad(t, findModel(-1))
	typeQuery(p, findQuery)

	for want := 1; want < findMatches; want++ {
		p.press(key.NameReturn, 0)
		p.frame()
		if p.find.current != want {
			t.Fatalf("Enter stepped to match %d, want %d", p.find.current+1, want+1)
		}
		if r := p.doc.Matches()[want].Rect; r.Empty() || r.Min.Y < 0 || r.Max.Y > p.size.Y {
			t.Errorf("match %d sits at %v after being stepped to, outside the %d-high page", want+1, r, p.size.Y)
		}
	}
	p.press(key.NameReturn, 0)
	p.frame()
	if p.find.current != 0 {
		t.Errorf("Enter on the last match stepped to match %d, want the first", p.find.current+1)
	}
	p.press(key.NameReturn, key.ModShift)
	p.frame()
	if p.find.current != findMatches-1 {
		t.Errorf("Shift+Enter on the first match stepped to match %d, want the last", p.find.current+1)
	}
}

// TestEscapeClearsTheFieldAndTheMarksAndGivesThePageBackTheKeyboard is the
// dismissal: the query goes, every mark with it, and the reading keys answer
// again — which they only do while the document holds the keyboard.
func TestEscapeClearsTheFieldAndTheMarksAndGivesThePageBackTheKeyboard(t *testing.T) {
	p := findPad(t, findModel(-1))
	typeQuery(p, findQuery)
	if len(p.doc.Matches()) == 0 {
		t.Fatal("the query marked nothing; there is nothing for Escape to clear")
	}

	p.press(key.NameEscape, 0)
	p.frame()
	if p.find.open || p.find.query != "" {
		t.Errorf("Escape left the field %#v; it closes the field and takes the query with it", p.find)
	}
	if n := len(p.doc.Matches()); n != 0 {
		t.Errorf("Escape left %d matches marked; the marks die with the query", n)
	}

	before := p.pos()
	p.press(key.NamePageDown, 0)
	if p.pos() == before {
		t.Error("the reading keys move nothing after Escape; the keyboard did not come back to the page")
	}
}

// TestTheFieldSaysHowManyMatchesAndWhichIsCurrent pins what the field says
// beside the query, which is what the Search field entry names and nothing
// else: which match out of how many, that there are none, and — with nothing
// typed — nothing at all.
func TestTheFieldSaysHowManyMatchesAndWhichIsCurrent(t *testing.T) {
	for _, tc := range []struct {
		find pageFind
		want string
	}{
		{pageFind{}, ""},
		{pageFind{query: "margin", count: 0}, "No matches"},
		{pageFind{query: "margin", count: 7, current: 2}, "3 of 7"},
		{pageFind{query: "margin", count: 1}, "1 of 1"},
	} {
		if got := tc.find.label(); got != tc.want {
			t.Errorf("a field holding %q on match %d of %d says %q, want %q",
				tc.find.query, tc.find.current+1, tc.find.count, got, tc.want)
		}
	}
}

// TestTheMatchesReachTheScrollbar is the other half of the marking: the places
// the bar paints are the query's, one for every match, and they go when the
// query does.
func TestTheMatchesReachTheScrollbar(t *testing.T) {
	p := findPad(t, findModel(-1))
	typeQuery(p, findQuery)

	places := p.doc.MatchPlaces()
	if len(places) != findMatches {
		t.Fatalf("the bar is handed %d places for %d matches", len(places), findMatches)
	}
	p.press(key.NameEscape, 0)
	p.frame()
	if n := len(p.doc.MatchPlaces()); n != 0 {
		t.Errorf("the bar is still handed %d places after the query was dismissed", n)
	}
}

// markPixels counts the pixels of img wearing c exactly, ignoring alpha: a
// glyph drawn over a field is antialiased against it and does not answer, so
// the count measures the marking and not the text on it.
func markPixels(img *image.RGBA, c color.NRGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R == c.R && p.G == c.G && p.B == c.B {
				n++
			}
		}
	}
	return n
}

// The note the find tests and the find goldens read: one word, three times,
// in three sections next to each other, in a note far taller than the window
// it is read in.
const (
	findQuery   = "margin"
	findMatches = 3
)

func findNoteSource() string {
	var b strings.Builder
	b.WriteString("# Reading room\n\n")
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n", i)
		switch i {
		case 8, 9, 10:
			b.WriteString("A paragraph that keeps the margin the reader came looking for.\n\n")
		default:
			b.WriteString("A paragraph of a note that runs well past the bottom of any window it is read in.\n\n")
		}
	}
	return b.String()
}

// findModel is the golden model with the find note current, seated at the
// given block so a still image lands where the matches are.
//
// The note is in the vault's own index, so the rail shows it and marks it
// open: a window whose tree does not hold the note on screen is a window
// nobody is looking at.
func findModel(first int) Model {
	m := goldenModel()
	m.Index = treeIndex(
		"Sources.md",
		"Design/Principles.md",
		"Design/notes/Colour.md",
		"guide/Reading list.md",
		"guide/Reading room.md",
	)
	m = cacheNote(m, noteFromSource("guide/Reading room.md", findNoteSource()))
	m.Current = "guide/Reading room.md"
	m.CurAnchor = first
	m.PropsOpen = false
	return m
}

// findState is the find the still images are taken in: the field open, the
// query typed, and the reader on the middle of its three matches.
func findState() pageFind {
	return pageFind{open: true, query: findQuery, current: 1}
}

// TestTheRailsFindAnswersItsOwnShortcut is the other half of the pair: the
// rail's find-a-note takes the platform's find shortcut with Shift, and the
// page's find takes it plain, so neither answers the other's.
func TestTheRailsFindAnswersItsOwnShortcut(t *testing.T) {
	var (
		r     input.Router
		ops   op.Ops
		tag   = new(int)
		field = new(int)
	)
	// focused reports where the keyboard is after the frame, read off a
	// target that does nothing else.
	frame := func() bool {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(240, 400)),
			Ops:         &ops,
			Source:      r.Source(),
		}
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: field}); !ok {
				break
			}
		}
		area := clip.Rect{Max: image.Pt(240, 400)}.Push(gtx.Ops)
		event.Op(gtx.Ops, field)
		event.Op(gtx.Ops, tag)
		area.Pop()
		focusFindField(gtx, field)
		holds := gtx.Focused(field)
		r.Frame(&ops)
		return holds
	}
	frame()

	r.Queue(key.Event{Name: findKey, Modifiers: key.ModShortcut, State: key.Press})
	frame()
	if frame() {
		t.Error("the page's find shortcut put the keyboard in the rail's find field")
	}

	r.Queue(key.Event{Name: findKey, Modifiers: key.ModShortcut | key.ModShift, State: key.Press})
	frame()
	if !frame() {
		t.Error("the rail's own shortcut did not put the keyboard in its find field")
	}
}
