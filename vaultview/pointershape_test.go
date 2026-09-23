package main

import (
	"context"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// pointerNoteSource is the note these readings are taken over: one line of
// prose carrying one link, and no frontmatter, so the link stands on the
// column's first body line and a reading of the prose either side of it is a
// reading of prose and not of the pinned head above it.
const pointerNoteSource = "The shelf I keep coming back to. See [[Sources]] for where.\n"

// pointerWindow is one live vaultview window driven through a router: the
// rail's own find field over the rail's list, and the note column beside
// them, so a shape read here is the shape the running window declares.
type pointerWindow struct {
	t    *testing.T
	f    *frameState
	r    input.Router
	ops  op.Ops
	m    Model
	tok  themeTokens
	sb   layout.Widget
	main layout.Widget
	as   layout.Widget
}

func newPointerWindow(t *testing.T, col tokens.PlatformColors) *pointerWindow {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := goldenModel()
	m.PropsOpen = false
	m = cacheNote(m, noteFromSource("guide/Reading list.md", pointerNoteSource))
	tok := themeTokens{col: col, typ: tokens.DefaultTypography, sp: tokens.Spacing,
		den: tokens.Comfortable, shaper: shaper}
	w := &pointerWindow{t: t, f: newFrameState(defaultWidths()), m: m, tok: tok}
	w.f.leading = func() unit.Dp { return goldenLeading }
	cur := &docCursor{}
	av := newAsideView(cur)
	// The rail is the live one, built from its own stream: the find field it
	// carries is the live text field, which is the region that was leaving
	// its shape on the list under it.
	w.sb = materializeWidget(t, treeSidebar(rx.Of(theme.Default()),
		func() Model { return w.m }, func() themeTokens { return w.tok }))
	w.main = renderNotePageInto(cur, shaper, m, col, tokens.Spacing,
		tokens.DefaultTypography, tokens.Comfortable, &pageFind{})
	w.as = func(gtx layout.Context) layout.Dimensions { return av.layout(gtx, w.m, w.tok) }
	w.frame()
	w.frame()
	return w
}

func materializeWidget(t *testing.T, obs rx.Observable[layout.Widget]) layout.Widget {
	t.Helper()
	var w layout.Widget
	if err := obs.Subscribe(context.Background(), func(next layout.Widget, _ error, done bool) {
		if !done && next != nil {
			w = next
		}
	}).Wait(); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("the rail emitted no layout.Widget")
	}
	return w
}

func (w *pointerWindow) frame() {
	w.ops.Reset()
	gtx := layout.Context{
		Constraints: layout.Exact(windowFrameSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      w.r.Source(),
		Ops:         &w.ops,
	}
	w.f.layout(gtx, w.m, w.tok, w.sb, w.as, w.main)
	w.r.Frame(&w.ops)
}

// at moves the pointer to (x, y) and reports the shape the frame after the
// move declares there.
func (w *pointerWindow) at(x, y int) pointer.Cursor {
	w.r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x), float32(y)), Source: pointer.Mouse})
	w.frame()
	return w.r.Cursor()
}

// TestTheRailsListKeepsTheArrowWhenTheFindFieldIsLeft is the owner's report
// read back: moving the pointer out of the rail's find field and onto the list
// under it, the list declares the arrow. The field's I-beam belongs to the
// field, which is the only region in that column that has one.
func TestTheRailsListKeepsTheArrowWhenTheFindFieldIsLeft(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := newPointerWindow(t, tc.colors)
			pane := w.f.geom.pane
			mid := (pane.Min.X + pane.Max.X) / 2
			// The find field, found by reading down the rail rather than by
			// pinning a height the density decides.
			fieldY := -1
			for y := pane.Min.Y; y < pane.Max.Y && fieldY < 0; y++ {
				if w.at(mid, y) == pointer.CursorText {
					fieldY = y
				}
			}
			if fieldY < 0 {
				t.Fatal("no row of the rail declares the I-beam: the find field shows no shape")
			}
			if got, want := w.at(mid, fieldY), pointer.CursorText; got != want {
				t.Fatalf("pointer in the find field = %v; want %v", got, want)
			}
			// Out of the field and onto the list, near the rail's bottom,
			// where the rows have run out and the list itself stands.
			listY := pane.Max.Y - listReadInsetDp
			if got, want := w.at(mid, listY), pointer.CursorDefault; got != want {
				t.Errorf("pointer on the list under the find field = %v; want %v", got, want)
			}
			// And back, so the reading is of what is under the pointer now
			// and not of an order the two were visited in.
			if got, want := w.at(mid, fieldY), pointer.CursorText; got != want {
				t.Errorf("pointer back in the find field = %v; want %v", got, want)
			}
		})
	}
}

// listReadInsetDp is how far above the rail's bottom edge the list is read:
// clear of the pane's own rounded corner, and well past the rows the model
// carries, so the reading is of the list and not of a row standing in it.
const listReadInsetDp = 40

// TestTheProseKeepsTheArrowWhenALinkIsLeft reads across the note's first body
// line: the hand over the link's own run of words, the arrow over the prose
// either side of it. A link's shape is the shape of the text it marks.
func TestTheProseKeepsTheArrowWhenALinkIsLeft(t *testing.T) {
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			w := newPointerWindow(t, tc.colors)
			from, to, y, ok := w.findLinkRun()
			if !ok {
				t.Fatal("no run of the note's prose declares the hand: a link shows no shape")
			}
			if got, want := w.at((from+to)/2, y), pointer.CursorPointer; got != want {
				t.Fatalf("pointer on the link = %v; want %v", got, want)
			}
			if got, want := w.at(from-linkProseGapDp, y), pointer.CursorDefault; got != want {
				t.Errorf("pointer on the prose before the link = %v; want %v", got, want)
			}
			if got, want := w.at(to+linkProseGapDp, y), pointer.CursorDefault; got != want {
				t.Errorf("pointer on the prose after the link = %v; want %v", got, want)
			}
			if got, want := w.at((from+to)/2, y), pointer.CursorPointer; got != want {
				t.Errorf("pointer back on the link = %v; want %v", got, want)
			}
		})
	}
}

// linkProseGapDp is how far past a link's own run the prose is read: far
// enough clear of the run's edge that the reading cannot be of the link's last
// column, near enough that it is still the same line of prose.
const linkProseGapDp = 6

// findLinkRun reads down the note column for the first line carrying an inline
// run that declares the hand with prose either side of it. A control in the
// column's pinned head runs its whole width, so requiring prose on both sides
// is what tells a link in a line from a row standing across one.
func (w *pointerWindow) findLinkRun() (from, to, y int, ok bool) {
	colFrom := w.f.geom.contentX + noteColumnInsetPx
	colTo := w.f.geom.pane.Max.X + w.noteColumnWidth() - noteColumnInsetPx
	for y := w.f.geom.rowTop; y < w.f.geom.rowTop+w.f.geom.rowH; y += 3 {
		first, last := -1, -1
		for x := colFrom; x < colTo; x += 4 {
			if w.at(x, y) == pointer.CursorPointer {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first > colFrom && last < colTo-4 && last > first {
			return first, last, y, true
		}
	}
	return 0, 0, 0, false
}

// noteColumnWidth is how wide the note column lays out in this window: the
// content area less the aside beside it.
func (w *pointerWindow) noteColumnWidth() int {
	return windowFrameSize.X - w.f.geom.contentX
}

// noteColumnInsetPx keeps the reading off the column's own edges, where the
// splitters beside it declare their resize shapes.
const noteColumnInsetPx = 24
