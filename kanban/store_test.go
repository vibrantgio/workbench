package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vibrantgio/mvu"
)

// fixture copies the plugin's file into a new folder and opens it there.
func fixture(t *testing.T) (string, *boardFile, Model) {
	t.Helper()
	data, err := os.ReadFile("testdata/plugin.md")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Board.md")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	text, seen, err := openBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, newBoardFile(path, seen), newModel(filepath.Base(path), text)
}

// runCmd waits for cmd and gives the messages it sent.
func runCmd(t *testing.T, cmd mvu.Command) []mvu.Message {
	t.Helper()
	msgs, err := cmd.Slice()
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

func TestTheAppMakesTheFileWhenItDoesNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes", "Kanban.md")
	text, _, err := openBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != newBoard || text != newBoard {
		t.Errorf("the new file is %q, want %q", data, newBoard)
	}
}

// TestTheAppWritesAChangeToTheFile changes a card, runs the write, and reads
// the file the app changed.
func TestTheAppWritesAChangeToTheFile(t *testing.T) {
	path, f, m := fixture(t)
	before, _ := os.ReadFile(path)
	next, cmd := change(f, m, retitled(m.Board, Selection{Column: 0, Card: 1}, "Book the dentist for June"), stepEdit)
	runCmd(t, cmd)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != next.Text {
		t.Fatalf("the file is %q, want %q", data, next.Text)
	}
	b := Parse(string(data))
	if got := b.Columns[0].Cards()[1].Title; got != "Book the dentist for June" {
		t.Errorf("the changed card reads %q", got)
	}
	if want := string(before); Format(retitled(b, Selection{Column: 0, Card: 1}, "Book the dentist")) != want {
		t.Error("the write changed lines other than the card's")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the folder holds %d files after the write, want only the board", len(entries))
	}
}

// TestTheAppsOwnWriteIsNotReadAgain polls after the app's write: the poll
// sees no change, so the write does not come back as a reload.
func TestTheAppsOwnWriteIsNotReadAgain(t *testing.T) {
	_, f, m := fixture(t)
	_, cmd := change(f, m, retitled(m.Board, Selection{Column: 0, Card: 0}, "Draft the notes"), stepEdit)
	runCmd(t, cmd)
	if data, ok := f.poll(); ok {
		t.Errorf("the poll read the app's own write again: %q", data)
	}
}

// TestAChangeOnDiskIsReadAgain writes the file from outside the app, and the
// poll and the update read it again, once.
func TestAChangeOnDiskIsReadAgain(t *testing.T) {
	path, f, m := fixture(t)
	outside := "## Today\n\n- [ ] Call the bank\n"
	if err := os.WriteFile(path, []byte(outside), 0o644); err != nil {
		t.Fatal(err)
	}
	data, ok := f.poll()
	if !ok || data != outside {
		t.Fatalf("poll = %q, %v; want the outside change", data, ok)
	}
	next, _ := newUpdate(f)(m, FileChanged{Data: data})
	if len(next.Board.Columns) != 1 || next.Board.Columns[0].Title != "Today" {
		t.Errorf("the board after the change has columns %+v", next.Board.Columns)
	}
	if _, ok := f.poll(); ok {
		t.Error("the poll read the same change twice")
	}
}

// TestWatchGivesTheChange runs the poll on its clock.
func TestWatchGivesTheChange(t *testing.T) {
	path, f, _ := fixture(t)
	if err := os.WriteFile(path, []byte("## Later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := watch(f, 10*time.Millisecond).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := msg.(FileChanged); !ok || got.Data != "## Later\n" {
		t.Errorf("watch gave %#v", msg)
	}
}

// TestAReadOfWhatTheAppHoldsChangesNothing gives the update the content the
// app holds already.
func TestAReadOfWhatTheAppHoldsChangesNothing(t *testing.T) {
	_, f, m := fixture(t)
	next, _ := newUpdate(f)(m, FileChanged{Data: m.Text})
	if next.Text != m.Text || next.Seq != m.Seq {
		t.Error("the update changed the model for content it held already")
	}
}

// TestAnOlderWriteDoesNotReplaceANewerOne runs two writes out of order.
func TestAnOlderWriteDoesNotReplaceANewerOne(t *testing.T) {
	path, f, _ := fixture(t)
	if err := f.write(2, "## Newer\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.write(1, "## Older\n"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "## Newer\n" {
		t.Errorf("the file is %q, want the newer write", data)
	}
}

func TestAnUnchangedBoardIsNotWritten(t *testing.T) {
	_, f, m := fixture(t)
	next, cmd := change(f, m, m.Board, stepEdit)
	if next.Seq != m.Seq || len(next.Undo) != 0 {
		t.Error("an unchanged board counted as a change")
	}
	if msgs := runCmd(t, cmd); len(msgs) != 0 {
		t.Errorf("an unchanged board sent %v", msgs)
	}
}
