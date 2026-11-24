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
// is empty or a field is open on the board.
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
	if declared[undoItem].Msg != (Undo{}) || declared[redoItem].Msg != (Redo{}) {
		t.Errorf("the Edit menu's items post %T and %T", declared[undoItem].Msg, declared[redoItem].Msg)
	}
	off := func(title string) menuItem { return menuItem{Title: title, Disabled: true} }
	on := func(title string) menuItem { return menuItem{Title: title} }
	want(m, off("Undo"), off("Redo"))
	m = apply(t, update, m, DeleteCard{Selection{0, 0}})
	want(m, on("Undo Delete Card"), off("Redo"))
	m = apply(t, update, m, OpenAdd{Column: 1}, AddCard{Title: "Fix the gate"})
	want(m, off("Undo"), off("Redo"))
	m = apply(t, update, m, CloseAdd{})
	want(m, on("Undo Add Card"), off("Redo"))
	m = apply(t, update, m, Undo{})
	want(m, on("Undo Delete Card"), on("Redo Add Card"))
	m = apply(t, update, m, EditCard{Selection{0, 0}})
	want(m, off("Undo"), off("Redo"))
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
