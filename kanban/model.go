package main

import (
	"strings"

	"github.com/vibrantgio/mvu"
)

// Model is the single application state. Every change goes through Update.
type Model struct {
	// Name is the file's name, which the toolbar shows.
	Name string
	// Board is the file's content read as a board.
	Board Board
	// Text is the file's content as the app last read it or last gave it
	// to the write.
	Text string
	// Seq counts the board's changes. A write carries the Seq of its
	// content, so an older content never replaces a newer one on disk.
	Seq int
	// Selected is the board's one selected card. It names a card only
	// while HasSelected is true.
	Selected    Selection
	HasSelected bool
	// Adding is the column whose new card is open. It names a column only
	// while IsAdding is true.
	Adding   int
	IsAdding bool
	// Opened counts the openings of a new card. A new count gives a new
	// card with an empty title.
	Opened int
	// Editing is the card whose title is edited in place. It names a card
	// only while IsEditing is true.
	Editing   Selection
	IsEditing bool
	// Edits counts the edits begun. A new count gives a new field that
	// holds the card's title.
	Edits int
	// Undo holds a step for each change that Cmd-Z can undo, the last
	// change last. Each step holds the board before its change. Redo holds
	// a step for each undone change, the last undone change last. Each of
	// those steps holds the board after its change. The file holds no
	// history.
	Undo, Redo []Step
}

// Step is one change on the undo or redo stack: the board on the other side
// of the change, and the change's name, which the Edit menu shows after
// "Undo" or "Redo".
type Step struct {
	Board Board
	Name  string
}

// The names of the changes, as the Edit menu shows them.
const (
	stepAdd      = "Add Card"
	stepDelete   = "Delete Card"
	stepComplete = "Mark as Completed"
	stepReopen   = "Mark as Not Completed"
	stepEdit     = "Edit Title"
)

// Undo makes the board as it was before the last change, and writes the
// file. It does nothing when there is no change to undo.
type Undo struct{}

// Redo makes the change that the last undo undid again, and writes the
// file. It does nothing when there is no undone change.
type Redo struct{}

// Selection names one card of the board: the column's index, and the
// card's index among the column's cards.
type Selection struct {
	Column, Card int
}

// SelectCard selects the card that Selection names.
type SelectCard struct{ Selection }

// DeleteCard deletes the card that Selection names, and clears the
// selection.
type DeleteCard struct{ Selection }

// OpenAdd opens a new card at the end of Column.
type OpenAdd struct{ Column int }

// CloseAdd removes the new card and the text typed in it.
type CloseAdd struct{}

// AddCard adds a card titled Title after the last card of the column whose
// new card is open, and opens the next new card under it. A line break in
// Title becomes a space, because a card's title is one line of the file.
type AddCard struct{ Title string }

// ToggleCard marks the card that Selection names completed, or marks a
// completed card not completed.
type ToggleCard struct{ Selection }

// EditCard begins the edit of the title of the card that Selection names,
// and selects the card.
type EditCard struct{ Selection }

// KeepEdit ends edit number Edit and gives the card the title Title. An
// empty title keeps the card's title. A line break in Title becomes a
// space.
type KeepEdit struct {
	Edit  int
	Title string
}

// CancelEdit ends edit number Edit and keeps the card's title.
type CancelEdit struct{ Edit int }

// has reports whether b holds the card that s names.
func (b Board) has(s Selection) bool {
	return s.Column >= 0 && s.Column < len(b.Columns) &&
		s.Card >= 0 && s.Card < len(b.Columns[s.Column].Cards())
}

// withoutCard is b without the card that s names, with the card's notes.
// The lines around the card stay. The result shares no slice that it
// changes with b, so a model that holds b keeps its board as it was.
func withoutCard(b Board, s Selection) Board {
	next := b
	next.Columns = append([]Column(nil), b.Columns...)
	old := b.Columns[s.Column].Entries
	entries := make([]Entry, 0, len(old))
	n := 0
	for _, e := range old {
		if e.Card != nil {
			n++
			if n-1 == s.Card {
				continue
			}
		}
		entries = append(entries, e)
	}
	next.Columns[s.Column].Entries = entries
	return next
}

// oneLine replaces each line break with a space. Shift-Return and a paste
// can put a line break in a title that a field gives.
var oneLine = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

// newModel is the model of a file named name whose content is data.
func newModel(name, data string) Model {
	return Model{Name: name, Board: Parse(data), Text: data}
}

// newUpdate is the update function for the board kept in f.
func newUpdate(f *boardFile) func(Model, mvu.Message) (Model, mvu.Command) {
	return func(m Model, message mvu.Message) (Model, mvu.Command) {
		switch msg := message.(type) {
		case FileChanged:
			if msg.Data == m.Text {
				return m, mvu.DoNothing()
			}
			// The stacks hold boards of the older file, so a change on
			// disk clears them.
			m.Board, m.Text = Parse(msg.Data), msg.Data
			m.Undo, m.Redo = nil, nil
			return corrected(m), mvu.DoNothing()
		case Undo:
			if len(m.Undo) == 0 {
				return m, mvu.DoNothing()
			}
			step := m.Undo[len(m.Undo)-1]
			m.Undo = m.Undo[: len(m.Undo)-1 : len(m.Undo)-1]
			m.Redo = pushed(m.Redo, Step{Board: m.Board, Name: step.Name})
			return travel(f, m, step.Board)
		case Redo:
			if len(m.Redo) == 0 {
				return m, mvu.DoNothing()
			}
			step := m.Redo[len(m.Redo)-1]
			m.Redo = m.Redo[: len(m.Redo)-1 : len(m.Redo)-1]
			m.Undo = pushed(m.Undo, Step{Board: m.Board, Name: step.Name})
			return travel(f, m, step.Board)
		case EditCard:
			if !m.Board.has(msg.Selection) {
				return m, mvu.DoNothing()
			}
			m.Selected, m.HasSelected = msg.Selection, true
			m.Editing, m.IsEditing, m.Edits = msg.Selection, true, m.Edits+1
			return m, mvu.DoNothing()
		case KeepEdit:
			if !m.IsEditing || msg.Edit != m.Edits {
				return m, mvu.DoNothing()
			}
			s := m.Editing
			m.Editing, m.IsEditing = Selection{}, false
			title := strings.TrimSpace(oneLine.Replace(msg.Title))
			if title == "" || !m.Board.has(s) {
				return m, mvu.DoNothing()
			}
			return change(f, m, retitled(m.Board, s, title), stepEdit)
		case CancelEdit:
			if m.IsEditing && msg.Edit == m.Edits {
				m.Editing, m.IsEditing = Selection{}, false
			}
			return m, mvu.DoNothing()
		case OpenAdd:
			if msg.Column < 0 || msg.Column >= len(m.Board.Columns) {
				return m, mvu.DoNothing()
			}
			m.Adding, m.IsAdding, m.Opened = msg.Column, true, m.Opened+1
			return m, mvu.DoNothing()
		case CloseAdd:
			m.Adding, m.IsAdding = 0, false
			return m, mvu.DoNothing()
		case AddCard:
			title := strings.TrimSpace(oneLine.Replace(msg.Title))
			if !m.IsAdding || title == "" {
				return m, mvu.DoNothing()
			}
			m.Opened++
			return change(f, m, withCard(m.Board, m.Adding, title), stepAdd)
		case ToggleCard:
			if !m.Board.has(msg.Selection) {
				return m, mvu.DoNothing()
			}
			name := stepComplete
			if m.Board.Columns[msg.Selection.Column].Cards()[msg.Selection.Card].Done() {
				name = stepReopen
			}
			return change(f, m, toggled(m.Board, msg.Selection), name)
		case SelectCard:
			if !m.Board.has(msg.Selection) {
				return m, mvu.DoNothing()
			}
			m.Selected, m.HasSelected = msg.Selection, true
			return m, mvu.DoNothing()
		case DeleteCard:
			if !m.Board.has(msg.Selection) {
				return m, mvu.DoNothing()
			}
			m.Selected, m.HasSelected = Selection{}, false
			return change(f, m, withoutCard(m.Board, msg.Selection), stepDelete)
		}
		return m, mvu.DoNothing()
	}
}

// change makes board the model's board and gives the command that writes
// it to f. Each message that changes the board goes through change, and
// name is the change's name. A change that changes the file's text puts
// the earlier board on the undo stack and clears the redo stack.
func change(f *boardFile, m Model, board Board, name string) (Model, mvu.Command) {
	text := Format(board)
	if text == m.Text {
		m.Board = board
		return m, mvu.DoNothing()
	}
	m.Undo = pushed(m.Undo, Step{Board: m.Board, Name: name})
	m.Redo = nil
	m.Board, m.Text, m.Seq = board, text, m.Seq+1
	return m, save(f, m.Seq, text)
}

// travel makes board, a board from the undo or the redo stack, the model's
// board, and gives the command that writes it to f.
func travel(f *boardFile, m Model, board Board) (Model, mvu.Command) {
	m.Board, m.Text, m.Seq = board, Format(board), m.Seq+1
	return corrected(m), save(f, m.Seq, m.Text)
}

// pushed is stack with step on top. The result shares no array with stack,
// because an earlier model holds stack, and its steps stay as they were.
func pushed(stack []Step, step Step) []Step {
	next := make([]Step, len(stack), len(stack)+1)
	copy(next, stack)
	return append(next, step)
}

// corrected is m with the selection, the new card and the edit cleared
// when they name a card or a column that m's board does not hold. A change
// on disk, an undo and a redo can each take that card or column away.
func corrected(m Model) Model {
	if m.HasSelected && !m.Board.has(m.Selected) {
		m.Selected, m.HasSelected = Selection{}, false
	}
	if m.IsAdding && m.Adding >= len(m.Board.Columns) {
		m.Adding, m.IsAdding = 0, false
	}
	if m.IsEditing && !m.Board.has(m.Editing) {
		m.Editing, m.IsEditing = Selection{}, false
	}
	return m
}
