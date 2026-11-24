package main

import "github.com/vibrantgio/mvu"

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
}

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
			return m, mvu.DoNothing()
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
