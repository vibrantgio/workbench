package main

import "strings"

// Board is the content of one markdown file read as a board. The file's
// lines are kept as they were read, so [Format] gives the bytes that [Parse]
// was given. Lines that are not a column's heading or a card are kept as
// they are and are not shown.
type Board struct {
	// Head holds the lines before the first column's heading: the front
	// matter and any text above the columns.
	Head []string
	// Columns are the file's level-2 headings, in file order.
	Columns []Column
}

// Column is one level-2 heading and the lines under it, up to the next
// level-2 heading or the end of the file.
type Column struct {
	// Heading is the heading's line as it was read, its line end included.
	Heading string
	// Title is the heading's text, without the marker and the spaces.
	Title string
	// Entries are the lines under the heading, in file order: each one is
	// a card or a line that the app keeps as it is.
	Entries []Entry
}

// Entry is one line under a column's heading, or one card with its notes.
// Exactly one of Card and Line is set.
type Entry struct {
	Card *Card
	// Line is a line that is not a card, its line end included.
	Line string
}

// Card is one task list item that is not indented.
type Card struct {
	// Marker is the item's text before its title: the bullet, the box and
	// the space after the box, as in "- [ ] ".
	Marker string
	// Title is the item's text after the marker, without the line end.
	Title string
	// End is the line end of the item's line: "\n", "\r\n" or "" for a last
	// line that has no line end.
	End string
	// Notes are the lines indented under the item, each with its line end.
	Notes []string
}

// Done reports whether the task is complete: the box holds "x" or "X".
func (c Card) Done() bool {
	box := strings.IndexByte(c.Marker, '[')
	return box >= 0 && box+1 < len(c.Marker) && (c.Marker[box+1] == 'x' || c.Marker[box+1] == 'X')
}

// Cards returns the column's cards, in file order.
func (c Column) Cards() []Card {
	var cards []Card
	for _, e := range c.Entries {
		if e.Card != nil {
			cards = append(cards, *e.Card)
		}
	}
	return cards
}

// Parse reads a markdown file as a board. A line in the front matter, in a
// fenced code block or in a "%%" comment is never a heading or a card.
func Parse(data string) Board {
	var b Board
	lines := splitLines(data)
	if len(lines) > 0 {
		if text, _ := cutEnd(lines[0]); text == "---" {
			if n := frontMatterEnd(lines); n > 0 {
				b.Head = append(b.Head, lines[:n]...)
				lines = lines[n:]
			}
		}
	}
	return parseLines(b, lines)
}

// parseLines reads the lines after the front matter into b.
func parseLines(b Board, lines []string) Board {
	var column *Column
	var card *Card
	var pending []string
	flush := func() {
		if column != nil {
			for _, l := range pending {
				column.Entries = append(column.Entries, Entry{Line: l})
			}
		}
		pending = nil
		card = nil
	}
	keep := func(line string) {
		flush()
		if column == nil {
			b.Head = append(b.Head, line)
			return
		}
		column.Entries = append(column.Entries, Entry{Line: line})
	}
	closeColumn := func() {
		flush()
		if column != nil {
			b.Columns = append(b.Columns, *column)
			column = nil
		}
	}

	fence := ""
	comment := false
	for _, line := range lines {
		text, end := cutEnd(line)
		switch {
		case fence != "":
			keep(line)
			if isFenceClose(text, fence) {
				fence = ""
			}
		case comment:
			keep(line)
			if strings.HasSuffix(strings.TrimSpace(text), "%%") {
				comment = false
			}
		case fenceOpen(text) != "":
			keep(line)
			fence = fenceOpen(text)
		case strings.HasPrefix(strings.TrimSpace(text), "%%"):
			keep(line)
			t := strings.TrimSpace(text)
			comment = len(t) < 4 || !strings.HasSuffix(t, "%%")
		default:
			if card != nil && text != "" && (text[0] == ' ' || text[0] == '\t') && strings.TrimSpace(text) != "" {
				card.Notes = append(card.Notes, pending...)
				card.Notes = append(card.Notes, line)
				pending = nil
				continue
			}
			if title, ok := heading2(text); ok {
				closeColumn()
				column = &Column{Heading: line, Title: title}
				continue
			}
			if column != nil {
				if marker, title, ok := taskItem(text); ok {
					flush()
					card = &Card{Marker: marker, Title: title, End: end}
					column.Entries = append(column.Entries, Entry{Card: card})
					continue
				}
				if card != nil && strings.TrimSpace(text) == "" {
					pending = append(pending, line)
					continue
				}
			}
			keep(line)
		}
	}
	closeColumn()
	return b
}

// Format writes the board back as markdown. A board that [Parse] gave and
// that nothing changed formats to the bytes Parse was given.
func Format(b Board) string {
	var s strings.Builder
	for _, l := range b.Head {
		s.WriteString(l)
	}
	for _, c := range b.Columns {
		s.WriteString(c.Heading)
		for _, e := range c.Entries {
			if e.Card == nil {
				s.WriteString(e.Line)
				continue
			}
			s.WriteString(e.Card.Marker)
			s.WriteString(e.Card.Title)
			s.WriteString(e.Card.End)
			for _, n := range e.Card.Notes {
				s.WriteString(n)
			}
		}
	}
	return s.String()
}

// splitLines splits data after each "\n", so every line keeps its line end
// and only the last line can lack one.
func splitLines(data string) []string {
	if data == "" {
		return nil
	}
	return strings.SplitAfter(data, "\n")[:countLines(data)]
}

// countLines is the number of lines splitLines gives: SplitAfter adds an
// empty last element after a final "\n", which is not a line.
func countLines(data string) int {
	n := strings.Count(data, "\n")
	if !strings.HasSuffix(data, "\n") {
		n++
	}
	return n
}

// cutEnd splits a line into its text and its line end.
func cutEnd(line string) (text, end string) {
	if t, ok := strings.CutSuffix(line, "\r\n"); ok {
		return t, "\r\n"
	}
	if t, ok := strings.CutSuffix(line, "\n"); ok {
		return t, "\n"
	}
	return line, ""
}

// frontMatterEnd is the number of lines the front matter takes when lines
// starts with one: from the first "---" to the next "---" or "...", both
// included. It is 0 when no closing line follows.
func frontMatterEnd(lines []string) int {
	for i := 1; i < len(lines); i++ {
		text, _ := cutEnd(lines[i])
		if text == "---" || text == "..." {
			return i + 1
		}
	}
	return 0
}

// heading2 reports whether text is a level-2 ATX heading and gives its text.
// The heading can have up to three spaces before its marker, and a closing
// run of "#" after a space.
func heading2(text string) (string, bool) {
	t := strings.TrimLeft(text, " ")
	if len(text)-len(t) > 3 || !strings.HasPrefix(t, "##") {
		return "", false
	}
	rest := t[2:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if trimmed := strings.TrimRight(rest, "#"); trimmed != rest && (trimmed == "" || strings.HasSuffix(trimmed, " ")) {
		rest = strings.TrimSpace(trimmed)
	}
	return rest, true
}

// taskItem reports whether text is a task list item that is not indented,
// and splits it into its marker and its title.
func taskItem(text string) (marker, title string, ok bool) {
	if len(text) < 6 || (text[0] != '-' && text[0] != '*' && text[0] != '+') || text[1] != ' ' || text[2] != '[' || text[4] != ']' {
		return "", "", false
	}
	switch text[3] {
	case ' ', 'x', 'X':
	default:
		return "", "", false
	}
	if len(text) == 5 {
		return text, "", true
	}
	if text[5] != ' ' {
		return "", "", false
	}
	return text[:6], text[6:], true
}

// fenceOpen gives the fence that text opens: a run of three or more "`" or
// "~" after up to three spaces. It is "" when text opens no fence.
func fenceOpen(text string) string {
	t := strings.TrimLeft(text, " ")
	if len(text)-len(t) > 3 || len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return ""
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return ""
	}
	return t[:n]
}

// isFenceClose reports whether text closes a code block that fence opened:
// a run of the same character at least as long, with nothing after it.
func isFenceClose(text, fence string) bool {
	t := strings.TrimSpace(text)
	return len(t) >= len(fence) && strings.Trim(t, fence[:1]) == ""
}
