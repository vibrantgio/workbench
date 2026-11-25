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
	// AddTyping is what the new card's field holds while it holds the
	// keyboard.
	AddTyping Typing
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
	// EditTyping is what the edit's field holds while it holds the
	// keyboard.
	EditTyping Typing
	// Undo holds a step for each change that Cmd-Z can undo, the last
	// change last. Each step holds the board before its change. Redo holds
	// a step for each undone change, the last undone change last. Each of
	// those steps holds the board after its change. The file holds no
	// history.
	Undo, Redo []Step
	// Restores counts the undos and the redos that changed the board. A
	// new count moves the keyboard to the column that holds the selected
	// card when another column holds the keyboard.
	Restores int
}

// Typing is what a field of the board holds while it holds the keyboard:
// typing that Undo can reverse, typing that Redo can make again, and
// whether the user typed after the last Undo. The field takes Cmd-Z while
// it holds typing for Undo, and Shift-Cmd-Z while it holds typing for
// Redo. While RedoEnded is true, the field drops Shift-Cmd-Z. The field
// gives each other press to the board.
type Typing struct{ Undo, Redo, RedoEnded bool }

// EndRedo ends what the board's Redo can make again. A field posts it
// when the user types in it after the last Undo, because typing is a new
// change. A field that loses the keyboard drops its typing, so a field
// that gets the keyboard again holds no typing and posts no EndRedo.
type EndRedo struct{}

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
	stepMove     = "Move Card"
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

// NewCardField tells what the field of new card number Opened holds.
type NewCardField struct {
	Opened int
	Typing Typing
}

// EditField tells what the field of edit number Edit holds.
type EditField struct {
	Edit   int
	Typing Typing
}

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

// MoveCard moves the card that From names to column To.Column, where the
// card has index To.Card after the move, and selects the card at its new
// place. The card takes its notes with it. A move to the place where the
// card already is changes nothing.
type MoveCard struct{ From, To Selection }

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
			return restore(f, m, step.Board)
		case Redo:
			if len(m.Redo) == 0 {
				return m, mvu.DoNothing()
			}
			step := m.Redo[len(m.Redo)-1]
			m.Redo = m.Redo[: len(m.Redo)-1 : len(m.Redo)-1]
			m.Undo = pushed(m.Undo, Step{Board: m.Board, Name: step.Name})
			return restore(f, m, step.Board)
		case EndRedo:
			m.Redo = nil
			return m, mvu.DoNothing()
		case EditCard:
			if !m.Board.has(msg.Selection) {
				return m, mvu.DoNothing()
			}
			m.Selected, m.HasSelected = msg.Selection, true
			m.Editing, m.IsEditing, m.Edits = msg.Selection, true, m.Edits+1
			m.EditTyping = Typing{}
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
			m.AddTyping = Typing{}
			return m, mvu.DoNothing()
		case CloseAdd:
			m.Adding, m.IsAdding, m.AddTyping = 0, false, Typing{}
			return m, mvu.DoNothing()
		case NewCardField:
			if m.IsAdding && msg.Opened == m.Opened {
				m.AddTyping = msg.Typing
			}
			return m, mvu.DoNothing()
		case EditField:
			if m.IsEditing && msg.Edit == m.Edits {
				m.EditTyping = msg.Typing
			}
			return m, mvu.DoNothing()
		case AddCard:
			title := strings.TrimSpace(oneLine.Replace(msg.Title))
			if !m.IsAdding || title == "" {
				return m, mvu.DoNothing()
			}
			m.Opened, m.AddTyping = m.Opened+1, Typing{}
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
		case MoveCard:
			if !m.Board.has(msg.From) || msg.To.Column < 0 || msg.To.Column >= len(m.Board.Columns) {
				return m, mvu.DoNothing()
			}
			room := len(m.Board.Columns[msg.To.Column].Cards())
			if msg.To.Column == msg.From.Column {
				room--
			}
			if msg.To.Card < 0 || msg.To.Card > room || msg.To == msg.From {
				return m, mvu.DoNothing()
			}
			board := moved(m.Board, msg.From, msg.To)
			if m.IsEditing {
				m.Editing, m.IsEditing = followed(m.Board, board, m.Editing)
			}
			m.Selected, m.HasSelected = msg.To, true
			return change(f, m, board, stepMove)
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

// restore makes board, a board from the undo or the redo stack, the model's
// board, and gives the command that writes it to f. The selection and the
// edit stay on the card that they named, at that card's place in board.
// When the step changes the edited card's title, a new edit begins, so
// the field holds the card's title. The field holds no typing then,
// because a field with typing takes Cmd-Z itself.
func restore(f *boardFile, m Model, board Board) (Model, mvu.Command) {
	if m.HasSelected {
		m.Selected, m.HasSelected = followed(m.Board, board, m.Selected)
	}
	if m.IsEditing {
		was := titleAt(m.Board, m.Editing)
		m.Editing, m.IsEditing = followed(m.Board, board, m.Editing)
		if m.IsEditing && titleAt(board, m.Editing) != was {
			m.Edits, m.EditTyping = m.Edits+1, Typing{}
		}
	}
	m.Board, m.Text, m.Seq = board, Format(board), m.Seq+1
	m.Restores++
	return corrected(m), save(f, m.Seq, m.Text)
}

// followed names in board to the card that s names in board from, and
// reports whether to holds that card. from and to differ by one step: one
// card added, deleted or changed in one column, or one card moved to
// another place in any column. A step copies the card that it changes and
// a card that gains a line end, and each other card keeps its pointer, so
// the same pointer is the same card, in whichever column to holds it. A
// card that to holds under a pointer that from does not hold, with the
// same marker, title and notes, is the same card: a move copies a card
// that gains a line end at its new place. That match skips a candidate
// that another card of from matches, when to does not hold the other
// card's pointer: the candidate is then that other card's copy. Otherwise
// a card is found by its place in its own column: the cards before it and
// after it match in from and to, by pointer or by value. A card in the
// place of a changed card, in a column whose card count stays, is the
// changed card. Otherwise the card is gone.
func followed(from, to Board, s Selection) (Selection, bool) {
	if !from.has(s) {
		return Selection{}, false
	}
	card := cardsOf(from.Columns[s.Column])[s.Card]
	held := make(map[*Card]bool)
	for _, c := range from.Columns {
		for _, p := range cardsOf(c) {
			held[p] = true
		}
	}
	for col, c := range to.Columns {
		for i, p := range cardsOf(c) {
			if p == card {
				return Selection{Column: col, Card: i}, true
			}
		}
	}
	kept := make(map[*Card]bool)
	for _, c := range to.Columns {
		for _, p := range cardsOf(c) {
			kept[p] = true
		}
	}
	twin := func(p *Card) bool {
		for _, c := range from.Columns {
			for _, q := range cardsOf(c) {
				if q != card && !kept[q] && sameCard(q, p) {
					return true
				}
			}
		}
		return false
	}
	for col, c := range to.Columns {
		for i, p := range cardsOf(c) {
			if !held[p] && sameCard(p, card) && !twin(p) {
				return Selection{Column: col, Card: i}, true
			}
		}
	}
	if s.Column >= len(to.Columns) {
		return Selection{}, false
	}
	was, now := cardsOf(from.Columns[s.Column]), cardsOf(to.Columns[s.Column])
	before := 0
	for before < len(was) && before < len(now) && sameCard(was[before], now[before]) {
		before++
	}
	after := 0
	for after < len(was)-before && after < len(now)-before &&
		sameCard(was[len(was)-1-after], now[len(now)-1-after]) {
		after++
	}
	switch {
	case s.Card < before, s.Card >= len(was)-after:
		if s.Card >= before {
			s.Card += len(now) - len(was)
		}
		return s, true
	case len(was) == len(now):
		return s, true
	}
	return Selection{}, false
}

// cardsOf is the pointers of c's cards, in file order.
func cardsOf(c Column) []*Card {
	var cards []*Card
	for _, e := range c.Entries {
		if e.Card != nil {
			cards = append(cards, e.Card)
		}
	}
	return cards
}

// sameCard reports whether a and b are one card: the same pointer, or the
// same marker, title and notes. A card's last line gains a line end when a
// line comes after it, so no line end counts.
func sameCard(a, b *Card) bool {
	if a == b {
		return true
	}
	if a.Marker != b.Marker || a.Title != b.Title || len(a.Notes) != len(b.Notes) {
		return false
	}
	for i := range a.Notes {
		x, _ := cutEnd(a.Notes[i])
		y, _ := cutEnd(b.Notes[i])
		if x != y {
			return false
		}
	}
	return true
}

// pushed is stack with step on top. The result shares no array with stack,
// because an earlier model holds stack, and its steps stay as they were.
func pushed(stack []Step, step Step) []Step {
	next := make([]Step, len(stack), len(stack)+1)
	copy(next, stack)
	return append(next, step)
}

// titleAt is the title of the card that s names in b.
func titleAt(b Board, s Selection) string {
	return b.Columns[s.Column].Cards()[s.Card].Title
}

// corrected is m with the selection, the new card and the edit cleared
// when they name a card or a column that m's board does not hold. A change
// on disk, an undo and a redo can each take that card or column away.
func corrected(m Model) Model {
	if m.HasSelected && !m.Board.has(m.Selected) {
		m.Selected, m.HasSelected = Selection{}, false
	}
	if m.IsAdding && m.Adding >= len(m.Board.Columns) {
		m.Adding, m.IsAdding, m.AddTyping = 0, false, Typing{}
	}
	if m.IsEditing && !m.Board.has(m.Editing) {
		m.Editing, m.IsEditing = Selection{}, false
	}
	return m
}
