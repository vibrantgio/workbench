package main

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// editRig is a rig over the fixture board with the first card of column 0
// selected, and the file's path.
func editRig(t *testing.T) (*boardRig, string) {
	t.Helper()
	path, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(firstCard(0))
	return rig, path
}

// editing reports whether the rig's edit is open on card s and its field
// holds the keyboard.
func (rig *boardRig) editing(s Selection) bool {
	return rig.model.IsEditing && rig.model.Editing == s &&
		rig.edit.state != nil && rig.r.Source().Focused(rig.edit.state.tag)
}

// caret is the edit field's selection, in runes.
func (rig *boardRig) caret() (start, end int) {
	return rig.edit.state.tag.(*widget.Editor).Selection()
}

// insert gives text to the edit field at its caret, as the platform's text
// input gives it: in place of the selection that the field reports.
func (rig *boardRig) insert(text string) {
	start, end := rig.caret()
	rig.r.Queue(key.EditEvent{Range: key.Range{Start: start, End: end}, Text: text})
	rig.drive()
}

// cardPadding is a point on the first card of column col inside the card's
// leading inset, before the checkbox.
func cardPadding(col int) image.Point {
	return image.Pt(int(boardInset)+col*int(columnWidth+columnGap)+int(tokens.Spacing.S4)+4, firstCard(col).Y)
}

// boardSpace is a point on column 0 below its cards and its button.
func boardSpace() image.Point {
	return image.Pt(firstCard(0).X, int(winH)-60)
}

// Return on the selected card edits its title in place, with the caret after
// the title. Return keeps the new title, writes only the card's line, and
// gives the keyboard back to the column with the card still selected.
func TestReturnEditsTheSelectedCard(t *testing.T) {
	rig, path := editRig(t)
	rig.key(key.NameReturn)
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("Return opened no edit of the selected card, or its field does not hold the keyboard")
	}
	if start, end := rig.caret(); start != end || end != len("Draft the release notes") {
		t.Errorf("the caret selects %d to %d, want the end of the title", start, end)
	}
	rig.insert(" for June")
	rig.key(key.NameReturn)
	if rig.model.IsEditing {
		t.Fatal("Return left the edit open")
	}
	want := strings.Replace(fixtureBoard, "Draft the release notes\n", "Draft the release notes for June\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the file after the edit is\n%q\nwant\n%q", got, want)
	}
	if !rig.model.HasSelected || rig.model.Selected != (Selection{0, 0}) {
		t.Errorf("after the edit the model selects %+v (%v), want the edited card", rig.model.Selected, rig.model.HasSelected)
	}
	if !rig.r.Source().Focused(rig.state.column(0).Focus()) {
		t.Error("after Return the column does not hold the keyboard")
	}
}

// Escape keeps the old title and writes nothing.
func TestEscapeKeepsTheOldTitle(t *testing.T) {
	rig, path := editRig(t)
	rig.key(key.NameReturn)
	rig.insert(" for June")
	rig.key(key.NameEscape)
	if rig.model.IsEditing {
		t.Fatal("Escape left the edit open")
	}
	if got := readFile(t, path); got != fixtureBoard {
		t.Errorf("Escape wrote the file:\n%q", got)
	}
	if got := rig.model.Board.Columns[0].Cards()[0].Title; got != "Draft the release notes" {
		t.Errorf("after Escape the card's title is %q", got)
	}
	if !rig.r.Source().Focused(rig.state.column(0).Focus()) {
		t.Error("after Escape the column does not hold the keyboard")
	}
}

// A double-click on a card that is not selected selects it and edits its
// title. Two presses a second apart select the card and edit nothing.
func TestADoubleClickEditsTheCard(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(firstCard(2))
	rig.press(firstCard(2))
	if rig.model.IsEditing {
		t.Fatal("two presses a second apart opened an edit")
	}
	rig.doubleClick(firstCard(0))
	if !rig.editing(Selection{0, 0}) {
		t.Fatalf("the double-click opened no edit of card 0 of column 0: %v %+v", rig.model.IsEditing, rig.model.Editing)
	}
	if rig.model.Selected != (Selection{0, 0}) {
		t.Errorf("the double-click selects %+v, want the edited card", rig.model.Selected)
	}
}

// A press inside the field or on the edited card keeps the edit open. A
// press outside the card ends the edit and keeps the new title, as the
// platform's rename does.
func TestAPressOutsideTheCardKeepsTheNewTitle(t *testing.T) {
	rig, path := editRig(t)
	rig.key(key.NameReturn)
	rig.insert(" for June")
	rig.press(firstCard(0))
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("a press inside the field ended the edit")
	}
	rig.press(cardPadding(0))
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("a press on the edited card's inset ended the edit")
	}
	rig.press(boardSpace())
	if rig.model.IsEditing {
		t.Fatal("a press outside the card left the edit open")
	}
	want := strings.Replace(fixtureBoard, "Draft the release notes\n", "Draft the release notes for June\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the file after the press is\n%q\nwant\n%q", got, want)
	}
}

// A press on another card ends the edit, keeps the new title, and selects
// the other card.
func TestAPressOnAnotherCardKeepsTheNewTitle(t *testing.T) {
	rig, _ := editRig(t)
	rig.key(key.NameReturn)
	rig.insert(" for June")
	rig.press(firstCard(2))
	if rig.model.IsEditing {
		t.Fatal("a press on another card left the edit open")
	}
	if got := rig.model.Board.Columns[0].Cards()[0].Title; got != "Draft the release notes for June" {
		t.Errorf("the edited card's title is %q", got)
	}
	if rig.model.Selected != (Selection{2, 0}) {
		t.Errorf("the press selects %+v, want card 0 of column 2", rig.model.Selected)
	}
}

// An edit to an empty title keeps the old title and writes nothing.
func TestAnEmptyTitleKeepsTheOldTitle(t *testing.T) {
	rig, path := editRig(t)
	rig.key(key.NameReturn)
	n := len("Draft the release notes")
	rig.r.Queue(key.EditEvent{Range: key.Range{Start: 0, End: n}, Text: "  "})
	rig.drive()
	if got := rig.edit.state.text; got != "  " {
		t.Fatalf("the field holds %q, want two spaces", got)
	}
	rig.key(key.NameReturn)
	if rig.model.IsEditing {
		t.Fatal("Return left the edit open")
	}
	if got := readFile(t, path); got != fixtureBoard {
		t.Errorf("an empty title wrote the file:\n%q", got)
	}
}

// While the edit is open, Delete and Backspace change the field's text and
// delete no card, and Up and Down do not move the selection.
func TestTheFieldKeepsTheListsKeys(t *testing.T) {
	rig, path := editRig(t)
	rig.key(key.NameReturn)
	rig.key(key.NameDeleteBackward)
	rig.key(key.NameDownArrow)
	rig.key(key.NameUpArrow)
	rig.key(key.NameDownArrow)
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("the keys ended the edit")
	}
	if got := len(rig.model.Board.Columns[0].Cards()); got != 3 {
		t.Errorf("Backspace in the field deleted a card: %d cards", got)
	}
	if rig.model.Selected != (Selection{0, 0}) {
		t.Errorf("the keys moved the selection to %+v", rig.model.Selected)
	}
	if got := rig.edit.state.text; got != "Draft the release note" {
		t.Errorf("Backspace left %q in the field, want the last letter gone", got)
	}
	if got := readFile(t, path); got != fixtureBoard {
		t.Errorf("the keys wrote the file:\n%q", got)
	}
}

// A file change on disk keeps the text being typed. A change that removes
// the edited card ends the edit.
func TestAFileChangeKeepsTheTextBeingTyped(t *testing.T) {
	rig, _ := editRig(t)
	rig.key(key.NameReturn)
	rig.insert(" for June")
	changed := strings.Replace(fixtureBoard, "Water the plants", "Water the ferns", 1)
	rig.model, _ = rig.update(rig.model, FileChanged{Data: changed})
	rig.build()
	rig.drive()
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("the file change ended the edit")
	}
	if got := rig.edit.state.text; got != "Draft the release notes for June" {
		t.Errorf("after the file change the field holds %q", got)
	}
	gone := strings.Replace(changed, "- [ ] Draft the release notes\n", "", 1)
	gone = strings.Replace(gone, "- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n", "", 1)
	gone = strings.Replace(gone, "- [ ] Water the ferns\n", "", 1)
	rig.model, _ = rig.update(rig.model, FileChanged{Data: gone})
	rig.build()
	rig.drive()
	if rig.model.IsEditing {
		t.Error("the edit stays open on a card that the file no longer holds")
	}
}

// A late end of an edit that is closed changes nothing.
func TestALateEndOfAnEditChangesNothing(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m, _ = update(m, EditCard{Selection{0, 0}})
	first := m.Edits
	m, _ = update(m, CancelEdit{Edit: first})
	m, _ = update(m, EditCard{Selection{0, 1}})
	next, _ := update(m, KeepEdit{Edit: first, Title: "Late"})
	if !next.IsEditing || next.Seq != m.Seq {
		t.Errorf("the end of the first edit changed the second: editing %v, seq %d", next.IsEditing, next.Seq)
	}
	if got := next.Board.Columns[0].Cards()[0].Title; got != "Draft the release notes" {
		t.Errorf("the late end retitled the card to %q", got)
	}
}

// The title's text stands at the same pixels before the edit, while it is
// edited, and after Return. Inside the card the renders differ only in the
// caret. The cases are a one-line title, a title that wraps, and an edit
// that a double-click begins.
func TestTheTitleStandsStillInTheEdit(t *testing.T) {
	cases := []struct {
		name   string
		card   int
		lines  int
		double bool
	}{
		{"one line", 0, 1, false},
		{"wrapped", 1, 3, false},
		{"double-click", 0, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, f, m := boardFileWith(t, fixtureBoard)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			rig.press(firstCard(0))
			for range tc.card {
				rig.key(key.NameDownArrow)
			}
			before := golden.Capture(t, windowSize, rig.capture())
			if tc.double {
				// The press before is a second ago, so these two presses
				// are the double-click.
				p := firstCard(0)
				rig.press(p)
				rig.pressAfter(p, 100*time.Millisecond)
			} else {
				rig.key(key.NameReturn)
			}
			if !rig.editing(Selection{0, tc.card}) {
				t.Fatalf("no edit of card %d is open", tc.card)
			}
			during := golden.Capture(t, windowSize, rig.capture())
			rig.key(key.NameReturn)
			after := golden.Capture(t, windowSize, rig.capture())
			if before == nil || during == nil || after == nil {
				return
			}
			top, bottom, fill := selectedRows(after, rig.th, 0, tc.card)
			if lines := (bottom - top - 2*int(rig.th.spacing.S4)) / int(rig.th.typ.Body.LineHeight); lines != tc.lines {
				t.Fatalf("the edited card stands in rows %d to %d, %d lines, want %d", top, bottom, lines, tc.lines)
			}
			area := titleArea(after, rig.th, 0, top, bottom, fill)
			var caret image.Rectangle
			for y := area.Min.Y; y < area.Max.Y; y++ {
				for x := area.Min.X; x < area.Max.X; x++ {
					if during.RGBAAt(x, y) != after.RGBAAt(x, y) || during.RGBAAt(x, y) != before.RGBAAt(x, y) {
						caret = caret.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			if caret.Dx() > 3 {
				t.Errorf("the edit changed the pixels inside %v; only the caret may change", caret)
			}
			was, is, now := drawnIn(before, fill, area, caret), drawnIn(during, fill, area, caret), drawnIn(after, fill, area, caret)
			if was.Empty() {
				t.Fatal("the selected card draws no title; this measures nothing")
			}
			t.Logf("in %v the text stands at %v, and the caret at %v", area, is, caret)
			if was != is || is != now {
				t.Errorf("the text stands at %v before the edit, at %v in it and at %v after Return", was, is, now)
			}
		})
	}
}

// selectedRows is the rows of card n of column col in img, and the card's
// fill. It reads them down the line of pixels that cardRows reads, and
// counts each run that differs from the column's fill as a card, so it
// finds a selected card too.
func selectedRows(img *image.RGBA, th themed, col, n int) (top, bottom int, fill color.RGBA) {
	x := int(boardInset) + col*int(columnWidth+columnGap) + int(columnWidth) - 3*int(th.spacing.S4)/2
	surface := rgba(th.platform.ControlBackground)
	run, in := -1, false
	for y := firstCard(col).Y - 12; y < img.Bounds().Max.Y; y++ {
		on := img.RGBAAt(x, y) != surface
		switch {
		case on && !in:
			run++
			if run == n {
				top = y
			}
		case !on && in && run == n:
			return top, y, img.RGBAAt(x, (top+y)/2)
		}
		in = on
	}
	return top, top, fill
}

// TestEditCardGolden records or diffs the board with the first card's title
// in edit, in both schemes. With -board.dump it also writes the edit with
// text typed, and the board after Return.
func TestEditCardGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			page := func(radius tokens.RadiusScale) layout.Widget {
				_, f, m := boardFileWith(t, fixtureBoard)
				rig := newBoardRig(t, staticThemed(tc.c, radius), f, m)
				rig.press(firstCard(0))
				rig.key(key.NameReturn)
				return golden.Stack(rig.capture(), Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth }))
			}
			golden.Render(t, tc.name+"-edit-card", windowSize, page(sharpRadius))
			if *boardDump != "" {
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-edit-card.png"), golden.Capture(t, windowSize, page(tokens.Radius)))
				_, f, m := boardFileWith(t, fixtureBoard)
				rig := newBoardRig(t, staticThemed(tc.c, tokens.Radius), f, m)
				rig.press(firstCard(0))
				rig.key(key.NameReturn)
				rig.insert(" for June")
				// The platform moves the caret after the text it gives. The
				// rig does not, so the dump puts the caret at the text's end.
				ed := rig.edit.state.tag.(*widget.Editor)
				ed.SetCaret(ed.Len(), ed.Len())
				rig.drive()
				bar := Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth })
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-edit-card-typed.png"), golden.Capture(t, windowSize, golden.Stack(rig.capture(), bar)))
				rig.key(key.NameReturn)
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-edit-card-kept.png"), golden.Capture(t, windowSize, golden.Stack(rig.capture(), bar)))
			}
		})
	}
}
