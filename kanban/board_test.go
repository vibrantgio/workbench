package main

import (
	"os"
	"reflect"
	"testing"
)

// TestAPluginFileIsWrittenBackWithTheSameBytes reads a file as the Obsidian
// Kanban plugin writes it and formats it again.
func TestAPluginFileIsWrittenBackWithTheSameBytes(t *testing.T) {
	data, err := os.ReadFile("testdata/plugin.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := Format(Parse(string(data))); got != string(data) {
		t.Errorf("Format(Parse(file)) differs from the file:\n%q\nwant\n%q", got, data)
	}
}

// TestThePluginFileGivesItsColumnsAndCards reads the columns, the cards,
// the completed tasks and the notes off the plugin's file.
func TestThePluginFileGivesItsColumnsAndCards(t *testing.T) {
	data, err := os.ReadFile("testdata/plugin.md")
	if err != nil {
		t.Fatal(err)
	}
	b := Parse(string(data))
	var titles []string
	for _, c := range b.Columns {
		titles = append(titles, c.Title)
	}
	if want := []string{"To Do", "In Progress", "Done", "Archive"}; !reflect.DeepEqual(titles, want) {
		t.Fatalf("columns = %q, want %q", titles, want)
	}
	todo := b.Columns[0].Cards()
	if len(todo) != 3 || todo[1].Title != "Book the dentist" || todo[1].Done() {
		t.Fatalf("To Do cards = %+v", todo)
	}
	if want := []string{"\tThursday or Friday morning.\n", "\tAsk about the cleaning.\n"}; !reflect.DeepEqual(todo[1].Notes, want) {
		t.Errorf("notes = %q, want %q", todo[1].Notes, want)
	}
	done := b.Columns[2].Cards()
	if len(done) != 2 || !done[0].Done() || !done[1].Done() {
		t.Errorf("Done cards = %+v, want two completed tasks", done)
	}
	if n := len(b.Columns[3].Cards()); n != 1 {
		t.Errorf("Archive has %d cards, want 1", n)
	}
}

// TestLinesThatAreNotColumnsOrCardsStay covers the lines the app keeps but
// does not show: a heading in the front matter or in a fenced block is not a
// column, an indented item is a note, and other list items are not cards.
func TestLinesThatAreNotColumnsOrCardsStay(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		columns []string
		cards   []int
	}{
		{"front matter", "---\n## not a column\n---\n## A\n- [ ] one\n", []string{"A"}, []int{1}},
		{"fence", "## A\n```\n## not a column\n- [ ] not a card\n```\n- [ ] one\n", []string{"A"}, []int{1}},
		{"comment", "## A\n%%\n## not a column\n%%\n", []string{"A"}, []int{0}},
		{"plain item", "## A\n- plain\n1. [ ] numbered\n- [ ] one\n", []string{"A"}, []int{1}},
		{"no final line end", "## A\n- [ ] one", []string{"A"}, []int{1}},
		{"crlf", "## A\r\n\r\n- [x] one\r\n  note\r\n\r\n## B\r\n", []string{"A", "B"}, []int{1, 0}},
		{"heading levels", "# Board\n### C\n## A ##\n####\n", []string{"A"}, []int{0}},
		{"empty", "", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := Parse(tc.data)
			var columns []string
			var cards []int
			for _, c := range b.Columns {
				columns = append(columns, c.Title)
				cards = append(cards, len(c.Cards()))
			}
			if !reflect.DeepEqual(columns, tc.columns) || !reflect.DeepEqual(cards, tc.cards) {
				t.Errorf("columns %q with cards %v, want %q with %v", columns, cards, tc.columns, tc.cards)
			}
			if got := Format(b); got != tc.data {
				t.Errorf("Format(Parse(data)) = %q, want %q", got, tc.data)
			}
		})
	}
}

// TestACRLFCardHasNoCarriageReturnInItsTitle keeps the line end out of the
// title the card shows.
func TestACRLFCardHasNoCarriageReturnInItsTitle(t *testing.T) {
	b := Parse("## A\r\n- [ ] one\r\n")
	if got := b.Columns[0].Cards()[0].Title; got != "one" {
		t.Errorf("title = %q, want %q", got, "one")
	}
}

// TestANewBoardHasTheThreeColumns reads the file the app makes when the file
// it opens does not exist.
func TestANewBoardHasTheThreeColumns(t *testing.T) {
	b := Parse(newBoard)
	var titles []string
	for _, c := range b.Columns {
		titles = append(titles, c.Title)
		if n := len(c.Cards()); n != 0 {
			t.Errorf("column %q has %d cards, want none", c.Title, n)
		}
	}
	if want := []string{"To Do", "In Progress", "Done"}; !reflect.DeepEqual(titles, want) {
		t.Errorf("columns = %q, want %q", titles, want)
	}
	if Format(b) != newBoard {
		t.Error("the new board does not format to its own bytes")
	}
}
