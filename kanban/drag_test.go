package main

import (
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// cardSpot is a point on card n of column col, in its upper or its lower
// half, read from the board as the rig draws it before a press.
type cardSpot struct {
	rig *boardRig
	img *image.RGBA
}

func spots(rig *boardRig) cardSpot {
	return cardSpot{rig: rig, img: golden.Capture(rig.t, windowSize, rig.capture())}
}

// upper is a point in the upper half of card n of column col, and lower one
// in its lower half.
func (c cardSpot) upper(col, n int) image.Point {
	top, bottom, _ := cardRows(c.img, c.rig.th, col, n)
	if bottom <= top {
		c.rig.t.Fatalf("column %d has no card %d", col, n)
	}
	return image.Pt(firstCard(col).X, top+(bottom-top)/4)
}

func (c cardSpot) lower(col, n int) image.Point {
	top, bottom, _ := cardRows(c.img, c.rig.th, col, n)
	if bottom <= top {
		c.rig.t.Fatalf("column %d has no card %d", col, n)
	}
	return image.Pt(firstCard(col).X, bottom-(bottom-top)/4)
}

// pointerAt queues one pointer event at p and draws a frame. A move with
// the button down is a drag.
func (rig *boardRig) pointerAt(kind pointer.Kind, p image.Point, down bool) {
	rig.now += 16 * time.Millisecond
	e := pointer.Event{Kind: kind, Source: pointer.Mouse, Position: f32.Pt(float32(p.X), float32(p.Y)), Time: rig.now}
	if down {
		e.Buttons = pointer.ButtonPrimary
	}
	rig.r.Queue(e)
	rig.drive()
}

// hold presses at from and moves the pointer in steps to to, with the
// button down, as the start of a drag does.
func (rig *boardRig) hold(from, to image.Point) {
	rig.now += time.Second
	rig.pointerAt(pointer.Move, from, false)
	rig.pointerAt(pointer.Press, from, true)
	const steps = 6
	for i := 1; i <= steps; i++ {
		p := from.Add(to.Sub(from).Mul(i).Div(steps))
		rig.pointerAt(pointer.Move, p, true)
	}
}

// drop releases the button at to, and draws the frame after it.
func (rig *boardRig) drop(to image.Point) {
	rig.pointerAt(pointer.Release, to, false)
	rig.drive()
}

// dragTo drags from from to to and releases there.
func (rig *boardRig) dragTo(from, to image.Point) {
	rig.hold(from, to)
	rig.drop(to)
}

// moveCase is one drag on a board and what the drag writes.
type moveCase struct {
	name     string
	board    string
	from, to func(cardSpot) image.Point
	// file is the board's file after the move, and selected the card the
	// model selects then.
	file     string
	selected Selection
}

var moveCases = []moveCase{
	{
		name:  "within its column",
		board: fixtureBoard,
		from:  func(c cardSpot) image.Point { return c.upper(0, 0) },
		to:    func(c cardSpot) image.Point { return c.lower(0, 2) },
		file: strings.Replace(fixtureBoard,
			"- [ ] Draft the release notes\n- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n- [ ] Water the plants\n",
			"- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n- [ ] Water the plants\n- [ ] Draft the release notes\n", 1),
		selected: Selection{0, 2},
	},
	{
		name:  "to another column",
		board: fixtureBoard,
		from:  func(c cardSpot) image.Point { return c.upper(0, 0) },
		to:    func(c cardSpot) image.Point { return c.upper(1, 0) },
		file: strings.Replace(strings.Replace(fixtureBoard, "- [ ] Draft the release notes\n", "", 1),
			"## In Progress\n\n", "## In Progress\n\n- [ ] Draft the release notes\n", 1),
		selected: Selection{1, 0},
	},
	{
		name:  "to an empty column",
		board: fixtureEmptyColumn,
		from:  func(c cardSpot) image.Point { return c.upper(0, 0) },
		to:    func(cardSpot) image.Point { return firstCard(1).Add(image.Pt(0, 40)) },
		file: strings.Replace(strings.Replace(fixtureEmptyColumn, "- [ ] Draft the release notes\n", "", 1),
			"## In Progress\n\n", "## In Progress\n\n- [ ] Draft the release notes\n", 1),
		selected: Selection{1, 0},
	},
	{
		name:  "from another column to the end of a column",
		board: fixtureBoard,
		from:  func(c cardSpot) image.Point { return c.upper(1, 0) },
		to:    func(c cardSpot) image.Point { return c.lower(2, 1) },
		file: strings.Replace(strings.Replace(fixtureBoard, "- [ ] Paint the hallway\n", "", 1),
			"- [x] Renew the passport\n", "- [x] Renew the passport\n- [ ] Paint the hallway\n", 1),
		selected: Selection{2, 2},
	},
}

// A card dragged to a place lands there, and the file is written when it
// lands: the card's line leaves its old place and stands at the new one,
// and every other line stays. The card is selected at its new place, and
// its column holds the keyboard. The move is one step of the undo stack,
// "Move Card"; Undo puts the card back with the selection on it, and Redo
// moves it again.
func TestACardIsDraggedToAPlace(t *testing.T) {
	for _, tc := range moveCases {
		t.Run(tc.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, tc.board)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			at := spots(rig)
			rig.dragTo(tc.from(at), tc.to(at))
			if got := readFile(t, path); got != tc.file {
				t.Fatalf("the file after the move is\n%q\nwant\n%q", got, tc.file)
			}
			if !rig.model.HasSelected || rig.model.Selected != tc.selected {
				t.Errorf("after the move the model selects %+v (%v), want %+v", rig.model.Selected, rig.model.HasSelected, tc.selected)
			}
			if !rig.r.Source().Focused(rig.state.column(tc.selected.Column).Focus()) {
				t.Error("the column the card landed in does not hold the keyboard")
			}
			if n := len(rig.model.Undo); n != 1 || rig.model.Undo[0].Name != stepMove {
				t.Fatalf("the undo stack is %d steps, want one %q step", n, stepMove)
			}
			from := rig.model.Board
			rig.model, _ = rig.update(rig.model, Undo{})
			if got := Format(rig.model.Board); got != tc.board {
				t.Errorf("after Undo the board is\n%q\nwant\n%q", got, tc.board)
			}
			if want, _ := followed(from, rig.model.Board, tc.selected); !rig.model.HasSelected || rig.model.Selected != want {
				t.Errorf("after Undo the model selects %+v (%v), want the moved card at %+v", rig.model.Selected, rig.model.HasSelected, want)
			}
			rig.model, _ = rig.update(rig.model, Redo{})
			if got := Format(rig.model.Board); got != tc.file {
				t.Errorf("after Redo the board is\n%q\nwant\n%q", got, tc.file)
			}
			if rig.model.Selected != tc.selected {
				t.Errorf("after Redo the model selects %+v, want %+v", rig.model.Selected, tc.selected)
			}
		})
	}
}

// Undo after a move to another column puts the selection on the card in
// the column it came from.
func TestUndoOfAMoveSelectsTheCardWhereItWas(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m, _ = update(m, MoveCard{From: Selection{0, 1}, To: Selection{2, 0}})
	if m.Selected != (Selection{2, 0}) {
		t.Fatalf("after the move the model selects %+v, want column 2, card 0", m.Selected)
	}
	m, _ = update(m, Undo{})
	if !m.HasSelected || m.Selected != (Selection{0, 1}) {
		t.Errorf("after Undo the model selects %+v (%v), want column 0, card 1", m.Selected, m.HasSelected)
	}
}

// A move keeps the card's notes with it, and the lines around its old
// place stay. A card from the last line of a file with no line end takes
// a line end at its new place, and the line before it keeps its own.
func TestAMoveTakesTheNotesAndTheLineEnds(t *testing.T) {
	cases := []struct{ name, board, file string }{
		{
			name:  "notes",
			board: "## A\n\n- [ ] One\n\tnote one\n- [ ] Two\n\n## B\n\n- [ ] Three\n",
			file:  "## A\n\n- [ ] Two\n\n## B\n\n- [ ] Three\n- [ ] One\n\tnote one\n",
		},
		{
			name:  "no line end",
			board: "## A\n\n- [ ] One\n\n## B\n\n- [ ] Two\n\tnote two",
			file:  "## A\n\n- [ ] Two\n\tnote two\n- [ ] One\n\n## B\n\n",
		},
		{
			name:  "CRLF",
			board: "## A\r\n\r\n- [ ] One\r\n- [ ] Two\r\n",
			file:  "## A\r\n\r\n- [ ] Two\r\n- [ ] One\r\n",
		},
	}
	moves := map[string]MoveCard{
		"notes":       {From: Selection{0, 0}, To: Selection{1, 1}},
		"no line end": {From: Selection{1, 0}, To: Selection{0, 0}},
		"CRLF":        {From: Selection{0, 1}, To: Selection{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, tc.board)
			m, cmd := newUpdate(f)(m, moves[tc.name])
			runCmd(t, cmd)
			if got := readFile(t, path); got != tc.file {
				t.Errorf("the file after the move is\n%q\nwant\n%q", got, tc.file)
			}
			if Format(m.Board) != tc.file {
				t.Errorf("the board after the move is\n%q\nwant\n%q", Format(m.Board), tc.file)
			}
		})
	}
}

// A drop at the card's own place, a release over no place and Escape
// during a drag change nothing: the file stays, and the undo stack holds
// no step.
func TestAMoveThatLandsNowhereChangesNothing(t *testing.T) {
	cases := []struct {
		name string
		drag func(rig *boardRig, at cardSpot)
	}{
		{"its own place", func(rig *boardRig, at cardSpot) {
			rig.dragTo(at.upper(0, 1), at.lower(0, 1))
		}},
		{"the gap before its own place", func(rig *boardRig, at cardSpot) {
			rig.dragTo(at.lower(0, 1), at.upper(0, 1))
		}},
		{"no place", func(rig *boardRig, at cardSpot) {
			rig.dragTo(at.upper(0, 1), image.Pt(firstCard(0).X, int(shell.ToolbarDepth)/2))
		}},
		{"Escape", func(rig *boardRig, at cardSpot) {
			rig.hold(at.upper(0, 1), at.upper(1, 0))
			rig.key(key.NameEscape)
			rig.drop(at.upper(1, 0))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, fixtureBoard)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			tc.drag(rig, spots(rig))
			if got := readFile(t, path); got != fixtureBoard {
				t.Errorf("the file is\n%q\nwant it as it was", got)
			}
			if len(rig.model.Undo) != 0 {
				t.Errorf("the undo stack holds %q, want no step", rig.model.Undo[0].Name)
			}
		})
	}
}

// A column scrolls while the dragged card is near its bottom edge.
func TestAColumnScrollsNearItsBottomEdge(t *testing.T) {
	var b strings.Builder
	b.WriteString("## Long\n\n")
	for range 30 {
		b.WriteString("- [ ] A task\n")
	}
	_, f, m := boardFileWith(t, b.String())
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	at := spots(rig)
	// The viewport's bottom is the lowest row of the column where a card
	// is drawn: the cards run past it.
	x := firstCard(0).X
	bottom := 0
	fill := at.img.RGBAAt(x, firstCard(0).Y)
	for y := firstCard(0).Y; y < windowSize.Y; y++ {
		if at.img.RGBAAt(x, y) == fill {
			bottom = y
		}
	}
	edge := image.Pt(x, bottom-4)
	rig.hold(at.upper(0, 0), edge)
	before := rig.state.column(0).Position()
	for range 20 {
		rig.pointerAt(pointer.Move, edge.Add(image.Pt(1, 0)), true)
		rig.pointerAt(pointer.Move, edge, true)
	}
	after := rig.state.column(0).Position()
	if after.First == before.First && after.Offset <= before.Offset {
		t.Errorf("the column did not scroll: its position stays %+v", after)
	}
	rig.key(key.NameEscape)
	rig.drop(edge)
}

// TestDragGolden records or diffs a card of the second column dragged
// over the first column, with the line between its second and its third
// card.
func TestDragGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			page := func(radius tokens.RadiusScale) layout.Widget {
				_, f, m := boardFileWith(t, fixtureBoard)
				rig := newBoardRig(t, staticThemed(tc.c, radius), f, m)
				at := spots(rig)
				rig.hold(at.upper(1, 0), at.upper(0, 2).Add(image.Pt(24, 2)))
				if _, ok := rig.state.drag.Target(); !ok {
					t.Fatal("the drag holds no place")
				}
				return golden.Stack(rig.capture(), Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth }))
			}
			golden.Render(t, tc.name+"-drag", windowSize, page(sharpRadius))
			if *boardDump != "" {
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-drag.png"), golden.Capture(t, windowSize, page(tokens.Radius)))
			}
		})
	}
}
