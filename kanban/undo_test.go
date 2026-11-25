package main

import (
	"slices"
	"strings"
	"testing"

	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/tokens"
)

// apply gives each message to update in order, runs each command, and
// gives the last model.
func apply(t *testing.T, update func(Model, mvu.Message) (Model, mvu.Command), m Model, msgs ...mvu.Message) Model {
	t.Helper()
	for _, msg := range msgs {
		var cmd mvu.Command
		m, cmd = update(m, msg)
		runCmd(t, cmd)
	}
	return m
}

// Each kind of change is one step. Undo writes the file as it was before
// the change, and redo writes the file as it was after it. Each names the
// change.
func TestEachChangeIsUndoneAndRedone(t *testing.T) {
	changes := []struct {
		name string
		msgs []mvu.Message
		step string
	}{
		{"add", []mvu.Message{OpenAdd{Column: 1}, AddCard{Title: "Fix the gate"}}, stepAdd},
		{"delete", []mvu.Message{DeleteCard{Selection{Column: 0, Card: 1}}}, stepDelete},
		{"complete", []mvu.Message{ToggleCard{Selection{Column: 0, Card: 2}}}, stepComplete},
		{"reopen", []mvu.Message{ToggleCard{Selection{Column: 2, Card: 0}}}, stepReopen},
		{"edit", []mvu.Message{EditCard{Selection{Column: 0, Card: 2}}, KeepEdit{Edit: 1, Title: "Water the ferns"}}, stepEdit},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, fixtureBoard)
			update := newUpdate(f)
			m = apply(t, update, m, c.msgs...)
			after := readFile(t, path)
			if after == fixtureBoard {
				t.Fatal("the change wrote nothing")
			}
			if len(m.Undo) != 1 || m.Undo[0].Name != c.step {
				t.Fatalf("the undo stack is %v, want one %q step", stepNames(m.Undo), c.step)
			}
			m = apply(t, update, m, Undo{})
			if got := readFile(t, path); got != fixtureBoard {
				t.Fatalf("the file after the undo is\n%q\nwant\n%q", got, fixtureBoard)
			}
			if m.Text != fixtureBoard || len(m.Undo) != 0 || len(m.Redo) != 1 || m.Redo[0].Name != c.step {
				t.Fatalf("after the undo the stacks are %v and %v, want none and one %q", stepNames(m.Undo), stepNames(m.Redo), c.step)
			}
			m = apply(t, update, m, Redo{})
			if got := readFile(t, path); got != after {
				t.Fatalf("the file after the redo is\n%q\nwant\n%q", got, after)
			}
			if m.Text != after || len(m.Undo) != 1 || len(m.Redo) != 0 || m.Undo[0].Name != c.step {
				t.Fatalf("after the redo the stacks are %v and %v, want one %q and none", stepNames(m.Undo), stepNames(m.Redo), c.step)
			}
		})
	}
}

func stepNames(stack []Step) []string {
	names := make([]string, len(stack))
	for i, s := range stack {
		names[i] = s.Name
	}
	return names
}

// Undo goes back one step at a time, and each write reaches the disk: each
// undo and redo counts as a newer write than the change before it.
func TestUndoGoesBackOneStepAtATime(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m = apply(t, update, m, DeleteCard{Selection{0, 0}})
	one := readFile(t, path)
	m = apply(t, update, m, ToggleCard{Selection{0, 0}})
	two := readFile(t, path)
	m = apply(t, update, m, Undo{})
	if got := readFile(t, path); got != one {
		t.Fatalf("the first undo wrote\n%q\nwant\n%q", got, one)
	}
	m = apply(t, update, m, Undo{})
	if got := readFile(t, path); got != fixtureBoard {
		t.Fatalf("the second undo wrote\n%q\nwant the fixture", got)
	}
	m = apply(t, update, m, Redo{}, Redo{})
	if got := readFile(t, path); got != two {
		t.Fatalf("two redos wrote\n%q\nwant\n%q", got, two)
	}
	if !slices.Equal(stepNames(m.Undo), []string{stepDelete, stepComplete}) {
		t.Errorf("the undo stack is %v", stepNames(m.Undo))
	}
}

// Undo and redo do nothing when their stack is empty.
func TestAnEmptyStackDoesNothing(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	next, cmd := update(m, Undo{})
	if msgs := runCmd(t, cmd); len(msgs) != 0 || next.Seq != m.Seq {
		t.Error("an undo with no change counted as a change")
	}
	next, cmd = update(m, Redo{})
	if msgs := runCmd(t, cmd); len(msgs) != 0 || next.Seq != m.Seq {
		t.Error("a redo with no undone change counted as a change")
	}
	if got := readFile(t, path); got != fixtureBoard {
		t.Error("an empty stack wrote the file")
	}
}

// A new change clears the redo stack. A change that leaves the file's text
// as it was is no step.
func TestANewChangeClearsTheRedoStack(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m = apply(t, update, m, DeleteCard{Selection{0, 0}}, Undo{})
	if len(m.Redo) != 1 {
		t.Fatalf("the redo stack is %v after the undo", stepNames(m.Redo))
	}
	same := apply(t, update, m, EditCard{Selection{0, 0}}, KeepEdit{Edit: 1, Title: "Draft the release notes"})
	if len(same.Redo) != 1 || len(same.Undo) != 0 {
		t.Errorf("an edit that kept the title changed the stacks to %v and %v", stepNames(same.Undo), stepNames(same.Redo))
	}
	m = apply(t, update, m, ToggleCard{Selection{1, 0}})
	if len(m.Redo) != 0 || !slices.Equal(stepNames(m.Undo), []string{stepComplete}) {
		t.Errorf("after a new change the stacks are %v and %v, want one %q and none", stepNames(m.Undo), stepNames(m.Redo), stepComplete)
	}
}

// A change on disk clears both stacks, because they hold boards of the
// older file. A read of the text that the app holds changes neither.
func TestAChangeOnDiskClearsBothStacks(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m = apply(t, update, m, DeleteCard{Selection{0, 0}}, DeleteCard{Selection{0, 0}}, Undo{})
	if len(m.Undo) != 1 || len(m.Redo) != 1 {
		t.Fatalf("the stacks are %v and %v, want one step each", stepNames(m.Undo), stepNames(m.Redo))
	}
	same := apply(t, update, m, FileChanged{Data: m.Text})
	if len(same.Undo) != 1 || len(same.Redo) != 1 {
		t.Errorf("a read of the same text changed the stacks to %v and %v", stepNames(same.Undo), stepNames(same.Redo))
	}
	other := apply(t, update, m, FileChanged{Data: fixtureEmptyColumn})
	if len(other.Undo) != 0 || len(other.Redo) != 0 {
		t.Errorf("a change on disk left the stacks %v and %v", stepNames(other.Undo), stepNames(other.Redo))
	}
}

// An undone change leaves the earlier board as it was: a board that a model
// held formats to the same bytes after later changes, undos and redos. The
// steps of an earlier model's stacks stay as they were too.
func TestAnUndoneChangeLeavesTheEarlierBoardAsItWas(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	first := m.Board
	m = apply(t, update, m, OpenAdd{Column: 0}, AddCard{Title: "Call the bank"}, CloseAdd{})
	added, addedText := m.Board, m.Text
	m = apply(t, update, m, ToggleCard{Selection{0, 3}})
	held := m
	heldUndo := Format(held.Undo[1].Board)
	m = apply(t, update, m,
		EditCard{Selection{0, 3}}, KeepEdit{Edit: 1, Title: "Call the bank today"},
		DeleteCard{Selection{0, 0}},
		Undo{}, Undo{}, Undo{},
		ToggleCard{Selection{0, 1}},
		Undo{}, Redo{},
		OpenAdd{Column: 0}, AddCard{Title: "Pay the rent"}, CloseAdd{},
	)
	if got := Format(first); got != fixtureBoard {
		t.Errorf("the first board formats to\n%q\nwant\n%q", got, fixtureBoard)
	}
	if got := Format(added); got != addedText {
		t.Errorf("the board after the add formats to\n%q\nwant\n%q", got, addedText)
	}
	if got := Format(held.Undo[1].Board); got != heldUndo || held.Undo[1].Name != stepComplete {
		t.Errorf("the held model's last undo step changed to %q:\n%q", held.Undo[1].Name, got)
	}
}

// One function clears a selection, a new card and an edit that name a card
// or a column that an undo or a redo took away.
func TestAnUndoClearsASelectionOfACardThatIsGone(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m = apply(t, update, m, OpenAdd{Column: 0}, AddCard{Title: "Call the bank"}, CloseAdd{}, SelectCard{Selection{0, 3}})
	m = apply(t, update, m, Undo{})
	if m.HasSelected {
		t.Errorf("the undo of the add kept the selection of %+v", m.Selected)
	}
	m = apply(t, update, m, Redo{}, EditCard{Selection{0, 3}}, Undo{})
	if m.IsEditing || m.HasSelected {
		t.Errorf("the undo of the add kept the edit (%v) or the selection (%v) of the card that is gone", m.IsEditing, m.HasSelected)
	}
	kept := corrected(Model{Board: m.Board, Selected: Selection{0, 0}, HasSelected: true, Adding: 1, IsAdding: true})
	if !kept.HasSelected || !kept.IsAdding {
		t.Error("corrected cleared a selection or a new card that the board holds")
	}
}

// The Edit menu's Undo and Redo name the change that each undoes or
// redoes. Each is disabled, with no change in its title, when its stack
// is empty or the field that holds the keyboard holds typing for it. A
// field with no typing leaves the items to the stacks.
func TestTheEditMenuFollowsTheStacks(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	want := func(m Model, undo, redo menuItem) {
		t.Helper()
		got := editMenuState(m)
		if got[undoItem] != undo || got[redoItem] != redo {
			t.Errorf("the Edit menu shows %+v, want %+v", got, []menuItem{undo, redo})
		}
	}
	declared := editMenu()
	if declared[undoItem].Title != "Undo" || !declared[undoItem].Disabled ||
		declared[redoItem].Title != "Redo" || !declared[redoItem].Disabled ||
		declared[undoItem].Menu != "Edit" || declared[redoItem].Menu != "Edit" {
		t.Errorf("the Edit menu declares %+v, want Undo and Redo disabled", declared)
	}
	if declared[undoItem].Key != "z" || declared[redoItem].Key != "Z" {
		t.Errorf("the Edit menu's keys are %q and %q, want Cmd-Z and Shift-Cmd-Z", declared[undoItem].Key, declared[redoItem].Key)
	}
	if declared[undoItem].Msg != (Undo{}) || declared[redoItem].Msg != (Redo{}) {
		t.Errorf("the Edit menu's items post %T and %T", declared[undoItem].Msg, declared[redoItem].Msg)
	}
	off := func(title string) menuItem { return menuItem{Title: title, Disabled: true} }
	on := func(title string) menuItem { return menuItem{Title: title} }
	want(m, off("Undo"), off("Redo"))
	m = apply(t, update, m, DeleteCard{Selection{0, 0}})
	want(m, on("Undo Delete Card"), off("Redo"))
	m = apply(t, update, m, OpenAdd{Column: 1})
	want(m, on("Undo Delete Card"), off("Redo"))
	// typed is what a field reports after the user's typing, and reversed
	// what it reports after an Undo of all of that typing.
	typed := Typing{Undo: true, RedoEnded: true}
	reversed := Typing{Redo: true}
	m = apply(t, update, m, EndRedo{}, NewCardField{Opened: m.Opened, Typing: typed})
	want(m, off("Undo"), off("Redo"))
	m = apply(t, update, m, AddCard{Title: "Fix the gate"})
	want(m, on("Undo Add Card"), off("Redo"))
	m = apply(t, update, m, NewCardField{Opened: m.Opened - 1, Typing: typed})
	want(m, on("Undo Add Card"), off("Redo"))
	m = apply(t, update, m, EndRedo{}, NewCardField{Opened: m.Opened, Typing: typed}, NewCardField{Opened: m.Opened, Typing: Typing{}})
	want(m, on("Undo Add Card"), off("Redo"))
	m = apply(t, update, m, EndRedo{}, NewCardField{Opened: m.Opened, Typing: typed}, CloseAdd{})
	want(m, on("Undo Add Card"), off("Redo"))
	m = apply(t, update, m, Undo{})
	want(m, on("Undo Delete Card"), on("Redo Add Card"))
	m = apply(t, update, m, EditCard{Selection{0, 0}})
	want(m, on("Undo Delete Card"), on("Redo Add Card"))
	// Typing ends the board's Redo.
	m = apply(t, update, m, EndRedo{}, EditField{Edit: m.Edits, Typing: typed})
	want(m, off("Undo"), off("Redo"))
	m = apply(t, update, m, EditField{Edit: m.Edits, Typing: reversed})
	want(m, on("Undo Delete Card"), off("Redo"))
	// An Undo of the board after the field's Undo comes first for Redo.
	m = apply(t, update, m, Undo{})
	want(m, off("Undo"), on("Redo Delete Card"))
	m = apply(t, update, m, Redo{})
	want(m, on("Undo Delete Card"), off("Redo"))
	m = apply(t, update, m, EditField{Edit: m.Edits - 1, Typing: typed})
	want(m, on("Undo Delete Card"), off("Redo"))
	m = apply(t, update, m, CancelEdit{Edit: m.Edits}, ToggleCard{Selection{0, 0}})
	want(m, on("Undo Mark as Completed"), off("Redo"))
	m = apply(t, update, m, ToggleCard{Selection{0, 0}})
	want(m, on("Undo Mark as Not Completed"), off("Redo"))
	m = apply(t, update, m, Undo{}, Undo{}, Undo{})
	want(m, off("Undo"), on("Redo Delete Card"))
}

// chord presses name with mods, as the keyboard does.
func (rig *boardRig) chord(name key.Name, mods key.Modifiers) {
	rig.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	rig.drive()
}

// Cmd-Z on the board undoes the last change and Shift-Cmd-Z redoes it,
// while the column holds the keyboard.
func TestCmdZOnTheBoardUndoesAndRedoes(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(firstCard(0))
	rig.key(key.NameDeleteBackward)
	deleted := readFile(t, path)
	if deleted == fixtureBoard {
		t.Fatal("Delete wrote nothing")
	}
	rig.chord("Z", key.ModShortcut)
	if got := readFile(t, path); got != fixtureBoard {
		t.Fatalf("Cmd-Z wrote\n%q\nwant the fixture", got)
	}
	rig.chord("Z", key.ModShortcut|key.ModShift)
	if got := readFile(t, path); got != deleted {
		t.Fatalf("Shift-Cmd-Z wrote\n%q\nwant\n%q", got, deleted)
	}
}

// While a field holds the keyboard, Cmd-Z changes the field's text, and
// the board's stack does not move.
func TestCmdZInAFieldChangesTheFieldsText(t *testing.T) {
	rig, path := editRig(t)
	rig.model = apply(t, rig.update, rig.model, ToggleCard{Selection{1, 0}})
	rig.build()
	rig.drive()
	toggled := readFile(t, path)
	rig.key(key.NameReturn)
	rig.drive()
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("Return opened no edit of the selected card")
	}
	rig.addAtEnd(" for June")
	ed := rig.edit.state.tag.(*widget.Editor)
	if got := ed.Text(); got != "Draft the release notes for June" {
		t.Fatalf("the field holds %q before Cmd-Z", got)
	}
	from := len(rig.posted)
	rig.chord("Z", key.ModShortcut)
	if got := ed.Text(); strings.Contains(got, "June") {
		t.Errorf("Cmd-Z left the field's text %q", got)
	}
	for _, msg := range rig.posted[from:] {
		if _, ok := msg.(Undo); ok {
			t.Error("Cmd-Z in the field posted Undo")
		}
	}
	if len(rig.model.Undo) != 1 || len(rig.model.Redo) != 0 {
		t.Errorf("Cmd-Z in the field moved the stacks to %v and %v", stepNames(rig.model.Undo), stepNames(rig.model.Redo))
	}
	if got := readFile(t, path); got != toggled {
		t.Error("Cmd-Z in the field wrote the file")
	}
}

// The new card's field reports that it holds typed text and the keyboard,
// and the menu then gives Cmd-Z to the field. Return opens an empty new
// card, so the menu undoes the add again.
func TestTypingInTheNewCardDisablesTheMenu(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureEmptyColumn)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(addButtonAt(1))
	if rig.model.AddTyping != (Typing{}) {
		t.Fatal("an empty new card reports typed text")
	}
	rig.typeText("Call the plumber")
	if !rig.model.AddTyping.Undo || !editMenuState(rig.model)[undoItem].Disabled {
		t.Fatalf("typing in the new card left the menu at %+v", editMenuState(rig.model))
	}
	rig.key(key.NameReturn)
	if rig.model.AddTyping != (Typing{}) {
		t.Error("Return opened a new card that reports typed text")
	}
	if got := editMenuState(rig.model)[undoItem]; got != (menuItem{Title: "Undo Add Card"}) {
		t.Errorf("after Return the menu shows %+v, want Undo Add Card", got)
	}
	rig.typeText("Fix the tap")
	rig.press(firstCard(0))
	if rig.model.AddTyping != (Typing{}) {
		t.Error("the new card reports typed text after the keyboard left its field")
	}
}

// After an undo or a redo the selection stays on its card. An undo that
// puts a card back above the selected card moves the selection down with
// its card, and a redo moves it back up.
func TestTheSelectionFollowsItsCardAfterAnUndo(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	title := func(m Model) string {
		t.Helper()
		if !m.HasSelected || !m.Board.has(m.Selected) {
			t.Fatalf("no card is selected: %+v", m.Selected)
		}
		return m.Board.Columns[m.Selected.Column].Cards()[m.Selected.Card].Title
	}
	m = apply(t, update, m, DeleteCard{Selection{0, 0}}, SelectCard{Selection{0, 1}})
	selected := title(m)
	m = apply(t, update, m, Undo{})
	if m.Selected != (Selection{0, 2}) || title(m) != selected {
		t.Errorf("after the undo the selection is %+v on %q, want %q", m.Selected, title(m), selected)
	}
	m = apply(t, update, m, Redo{})
	if m.Selected != (Selection{0, 1}) || title(m) != selected {
		t.Errorf("after the redo the selection is %+v on %q, want %q", m.Selected, title(m), selected)
	}
	m = apply(t, update, m, ToggleCard{Selection{0, 1}}, Undo{})
	if m.Selected != (Selection{0, 1}) || title(m) != selected {
		t.Errorf("after the undo of its completion the selection is %+v on %q, want %q", m.Selected, title(m), selected)
	}
}

// followed finds a card by its place when its pointer changed, and a card
// that the step took away is gone.
func TestFollowedFindsACardByItsPlace(t *testing.T) {
	card := func(title string) Entry { return Entry{Card: &Card{Marker: "- [ ] ", Title: title, End: "\n"}} }
	a, b, c := card("A"), card("B"), card("C")
	from := Board{Columns: []Column{{Entries: []Entry{a, b, c}}}}
	ended := Board{Columns: []Column{{Entries: []Entry{card("A"), b}}}}
	if got, ok := followed(from, ended, Selection{0, 0}); !ok || got != (Selection{0, 0}) {
		t.Errorf("a copied card before the change is at %+v (%v), want its place", got, ok)
	}
	if got, ok := followed(from, ended, Selection{0, 2}); ok {
		t.Errorf("a card that the step took away is at %+v", got)
	}
	twins := Board{Columns: []Column{{Entries: []Entry{card("A"), a}}}}
	if got, ok := followed(Board{Columns: []Column{{Entries: []Entry{a}}}}, twins, Selection{0, 0}); !ok || got != (Selection{0, 1}) {
		t.Errorf("a card beside a twin is at %+v (%v), want its own place", got, ok)
	}
}

// undid reports whether Cmd-Z, pressed now, posts Undo and makes the board
// as it was before its last change: the file holds want.
func (rig *boardRig) undid(path, want string) bool {
	rig.t.Helper()
	from := len(rig.posted)
	rig.chord("Z", key.ModShortcut)
	posted := false
	for _, msg := range rig.posted[from:] {
		if _, ok := msg.(Undo); ok {
			posted = true
		}
	}
	return posted && readFile(rig.t, path) == want
}

// An open edit whose value the user has not changed leaves Cmd-Z to the
// board, and the edit stays on its card.
func TestAnUnchangedEditLeavesCmdZToTheBoard(t *testing.T) {
	rig, path := editRig(t)
	rig.model = apply(t, rig.update, rig.model, ToggleCard{Selection{1, 0}})
	rig.build()
	rig.drive()
	rig.key(key.NameReturn)
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("Return opened no edit of the selected card")
	}
	if got := editMenuState(rig.model)[undoItem]; got != (menuItem{Title: "Undo Mark as Completed"}) {
		t.Errorf("an unchanged edit leaves the menu at %+v", got)
	}
	if !rig.undid(path, fixtureBoard) {
		t.Fatal("Cmd-Z in an unchanged edit did not undo the board's change")
	}
	if !rig.editing(Selection{0, 0}) {
		t.Error("the edit left its card after the undo")
	}
}

// An empty new card leaves Cmd-Z to the board, and typing in it takes
// Cmd-Z for the field.
func TestAnEmptyNewCardLeavesCmdZToTheBoard(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.model = apply(t, rig.update, rig.model, ToggleCard{Selection{1, 0}}, OpenAdd{Column: 1})
	rig.build()
	rig.drive()
	toggled := readFile(t, path)
	rig.typeText("Fix the gate")
	if !rig.model.AddTyping.Undo {
		t.Fatal("typing in the new card reports no typing")
	}
	from := len(rig.posted)
	rig.chord("Z", key.ModShortcut)
	ed := rig.fresh.state.tag.(*widget.Editor)
	if got := ed.Text(); got != "" {
		t.Errorf("Cmd-Z left the new card's field at %q", got)
	}
	for _, msg := range rig.posted[from:] {
		if _, ok := msg.(Undo); ok {
			t.Error("Cmd-Z in a new card with typing posted Undo")
		}
	}
	if got := readFile(t, path); got != toggled {
		t.Error("Cmd-Z in a new card with typing wrote the file")
	}
	if !rig.undid(path, fixtureBoard) {
		t.Error("Cmd-Z in an empty new card did not undo the board's change")
	}
}

// A field whose typing is all reversed leaves Cmd-Z to the board, and the
// field keeps the card's title.
func TestAFieldWithItsTypingReversedLeavesCmdZToTheBoard(t *testing.T) {
	rig, path := editRig(t)
	rig.model = apply(t, rig.update, rig.model, ToggleCard{Selection{1, 0}})
	rig.build()
	rig.drive()
	rig.key(key.NameReturn)
	rig.addAtEnd(" for June")
	ed := rig.edit.state.tag.(*widget.Editor)
	rig.chord("Z", key.ModShortcut)
	if got := ed.Text(); got != "Draft the release notes" {
		t.Fatalf("Cmd-Z left the field at %q, want the card's title", got)
	}
	if !rig.undid(path, fixtureBoard) {
		t.Fatal("Cmd-Z in a field with its typing reversed did not undo the board's change")
	}
	if got := ed.Text(); got != "Draft the release notes" {
		t.Errorf("the board's undo left the field at %q", got)
	}
}

// An undo of a retitle while an unchanged edit of the card is open puts
// the old title in the field, and the edit ends with no new change.
func TestAnUndoOfARetitleGivesTheEditTheOldTitle(t *testing.T) {
	rig, path := editRig(t)
	rig.model = apply(t, rig.update, rig.model, EditCard{Selection{0, 0}}, KeepEdit{Edit: 1, Title: "Draft the notes"})
	rig.build()
	rig.drive()
	rig.key(key.NameReturn)
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("Return opened no edit of the selected card")
	}
	if !rig.undid(path, fixtureBoard) {
		t.Fatal("Cmd-Z in an unchanged edit did not undo the retitle")
	}
	ed := rig.edit.state.tag.(*widget.Editor)
	if got := ed.Text(); got != "Draft the release notes" {
		t.Errorf("after the undo the field holds %q, want the old title", got)
	}
	rig.key(key.NameReturn)
	if got := readFile(t, path); got != fixtureBoard {
		t.Errorf("Return after the undo wrote\n%q\nwant the fixture", got)
	}
	if len(rig.model.Undo) != 0 {
		t.Errorf("Return after the undo made the change %v", stepNames(rig.model.Undo))
	}
}

// menuChord presses Cmd-Z, or Shift-Cmd-Z when redo is true, as macOS
// gives the press: an enabled item of the Edit menu takes the press and
// posts its message, and a disabled item gives the press to the window.
func (rig *boardRig) menuChord(redo bool) {
	rig.t.Helper()
	item, mods := undoItem, key.ModShortcut
	if redo {
		item, mods = redoItem, key.ModShortcut|key.ModShift
	}
	if editMenuState(rig.model)[item].Disabled {
		rig.chord("Z", mods)
		return
	}
	msg := editMenu()[item].Msg
	rig.posted = append(rig.posted, msg)
	var cmd mvu.Command
	rig.model, cmd = rig.update(rig.model, msg)
	runCmd(rig.t, cmd)
	rig.build()
	rig.drive()
}

// Typing ends the board's Redo: after a retitle, its undo, and typing in
// an edit of the card, Shift-Cmd-Z does nothing and the typing stays.
func TestTypingEndsTheBoardsRedo(t *testing.T) {
	rig, path := editRig(t)
	rig.model = apply(t, rig.update, rig.model, EditCard{Selection{0, 0}}, KeepEdit{Edit: 1, Title: "Draft the notes"})
	rig.build()
	rig.drive()
	rig.menuChord(false)
	if got := readFile(t, path); got != fixtureBoard {
		t.Fatalf("Cmd-Z after the retitle wrote\n%q\nwant the fixture", got)
	}
	rig.key(key.NameReturn)
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("Return opened no edit of the selected card")
	}
	rig.addAtEnd(" for June")
	if got := editMenuState(rig.model)[redoItem]; got != (menuItem{Title: "Redo", Disabled: true}) {
		t.Errorf("after typing the menu's Redo is %+v, want it disabled", got)
	}
	from := len(rig.posted)
	rig.menuChord(true)
	for _, msg := range rig.posted[from:] {
		if _, ok := msg.(Redo); ok {
			t.Error("Shift-Cmd-Z after typing posted Redo")
		}
	}
	if got := readFile(t, path); got != fixtureBoard {
		t.Errorf("Shift-Cmd-Z after typing wrote\n%q\nwant the fixture", got)
	}
	if !rig.editing(Selection{0, 0}) {
		t.Fatal("the edit left its card after Shift-Cmd-Z")
	}
	if got := rig.edit.state.tag.(*widget.Editor).Text(); got != "Draft the release notes for June" {
		t.Errorf("after Shift-Cmd-Z the field holds %q, want the typing", got)
	}
	if len(rig.model.Redo) != 0 {
		t.Errorf("after typing the redo stack is %v, want none", stepNames(rig.model.Redo))
	}
}

// Redo makes the changes again in the opposite order of the undos: after
// an Undo of the typing and then an Undo of the board's change, Redo makes
// the board's change first and the typing next. The order is the same
// through the Edit menu and through the keys alone.
func TestRedoFollowsTheOrderOfTheUndos(t *testing.T) {
	presses := []struct {
		name  string
		press func(rig *boardRig, redo bool)
	}{
		{"menu", (*boardRig).menuChord},
		{"keys", func(rig *boardRig, redo bool) {
			mods := key.ModShortcut
			if redo {
				mods |= key.ModShift
			}
			rig.chord("Z", mods)
		}},
	}
	for _, p := range presses {
		t.Run(p.name, func(t *testing.T) {
			rig, path := editRig(t)
			rig.model = apply(t, rig.update, rig.model, ToggleCard{Selection{1, 0}})
			rig.build()
			rig.drive()
			toggled := readFile(t, path)
			rig.key(key.NameReturn)
			rig.addAtEnd(" for June")
			ed := rig.edit.state.tag.(*widget.Editor)
			text := func(want, what string) {
				t.Helper()
				if got := ed.Text(); got != want {
					t.Fatalf("%s the field holds %q, want %q", what, got, want)
				}
			}
			file := func(want, what string) {
				t.Helper()
				if got := readFile(t, path); got != want {
					t.Fatalf("%s the file is\n%q\nwant\n%q", what, got, want)
				}
			}
			p.press(rig, false)
			text("Draft the release notes", "after the first Undo")
			file(toggled, "after the first Undo")
			p.press(rig, false)
			text("Draft the release notes", "after the second Undo")
			file(fixtureBoard, "after the second Undo")
			if got := editMenuState(rig.model)[redoItem]; got != (menuItem{Title: "Redo Mark as Completed"}) {
				t.Errorf("after the board's undo the menu's Redo is %+v", got)
			}
			p.press(rig, true)
			text("Draft the release notes", "after the first Redo")
			file(toggled, "after the first Redo")
			p.press(rig, true)
			text("Draft the release notes for June", "after the second Redo")
			file(toggled, "after the second Redo")
			if !rig.editing(Selection{0, 0}) {
				t.Error("the edit left its card")
			}
		})
	}
}

// A field that loses the keyboard drops its typing. The user types in the
// new card, presses a card, and presses Cmd-Z on the board. When the field
// gets the keyboard again, it holds no typing: the menu's Redo names the
// board's undone change, and Shift-Cmd-Z makes it again.
func TestAFieldThatTakesTheKeyboardAgainLeavesRedoToTheBoard(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.model = apply(t, rig.update, rig.model, ToggleCard{Selection{1, 0}}, OpenAdd{Column: 1})
	rig.build()
	rig.drive()
	toggled := readFile(t, path)
	rig.typeText("Fix the gate")
	rig.press(firstCard(0))
	if !rig.undid(path, fixtureBoard) {
		t.Fatal("Cmd-Z on the board did not undo the board's change")
	}
	if !rig.model.IsAdding {
		t.Fatal("the press on a card closed the new card")
	}
	tag := rig.fresh.state.tag
	rig.r.Source().Execute(key.FocusCmd{Tag: tag})
	rig.drive()
	if !rig.r.Source().Focused(tag) {
		t.Fatal("the new card's field did not get the keyboard again")
	}
	if rig.model.AddTyping != (Typing{}) {
		t.Errorf("the field reports %+v after it got the keyboard again, want no typing", rig.model.AddTyping)
	}
	if got := editMenuState(rig.model)[redoItem]; got != (menuItem{Title: "Redo Mark as Completed"}) {
		t.Errorf("the menu's Redo is %+v, want the board's undone change", got)
	}
	from := len(rig.posted)
	rig.chord("Z", key.ModShortcut|key.ModShift)
	redone := false
	for _, msg := range rig.posted[from:] {
		if _, ok := msg.(Redo); ok {
			redone = true
		}
	}
	if !redone || readFile(t, path) != toggled {
		t.Error("Shift-Cmd-Z did not make the board's change again")
	}
	if got := tag.(*widget.Editor).Text(); got != "Fix the gate" {
		t.Errorf("the field holds %q, want its text", got)
	}
}
