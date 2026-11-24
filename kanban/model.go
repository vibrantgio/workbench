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
}

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
// can put a line break in the new card's title.
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
			m.Board, m.Text = Parse(msg.Data), msg.Data
			if m.HasSelected && !m.Board.has(m.Selected) {
				m.Selected, m.HasSelected = Selection{}, false
			}
			if m.IsAdding && m.Adding >= len(m.Board.Columns) {
				m.Adding, m.IsAdding = 0, false
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
			return change(f, m, withCard(m.Board, m.Adding, title))
		case ToggleCard:
			if !m.Board.has(msg.Selection) {
				return m, mvu.DoNothing()
			}
			return change(f, m, toggled(m.Board, msg.Selection))
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
			return change(f, m, withoutCard(m.Board, msg.Selection))
		}
		return m, mvu.DoNothing()
	}
}

// change makes board the model's board and gives the command that writes
// it to f. Each message that changes the board goes through change.
func change(f *boardFile, m Model, board Board) (Model, mvu.Command) {
	text := Format(board)
	if text == m.Text {
		m.Board = board
		return m, mvu.DoNothing()
	}
	m.Board, m.Text, m.Seq = board, text, m.Seq+1
	return m, save(f, m.Seq, text)
}
