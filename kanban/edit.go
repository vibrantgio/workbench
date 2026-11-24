package main

import "strings"

// newMarker is the marker of a card the app adds: a task that is not
// completed, as the Obsidian Kanban plugin writes one.
const newMarker = "- [ ] "

// withColumn is b with a copy of column col whose entries are a copy too, so
// that a change to the copy's entries leaves b as it was.
func withColumn(b Board, col int) Board {
	next := b
	next.Columns = append([]Column(nil), b.Columns...)
	next.Columns[col].Entries = append([]Entry(nil), b.Columns[col].Entries...)
	return next
}

// withCard is b with a card titled title after the last card of column col,
// after the card's notes. A column without a card takes the card after the
// blank line under its heading, where the plugin writes a column's first
// card. The new line takes the file's line end, and a line before it that
// has no line end takes one.
func withCard(b Board, col int, title string) Board {
	end := lineEnd(b, col)
	next := withColumn(b, col)
	c := &next.Columns[col]
	at := -1
	for i, e := range c.Entries {
		if e.Card != nil {
			at = i + 1
		}
	}
	if at < 0 {
		at = 0
		if len(c.Entries) > 0 && c.Entries[0].Card == nil && strings.TrimSpace(c.Entries[0].Line) == "" {
			at = 1
		}
	}
	if at == 0 {
		if _, e := cutEnd(c.Heading); e == "" {
			c.Heading += end
		}
	} else {
		c.Entries[at-1] = endedEntry(c.Entries[at-1], end)
	}
	card := &Card{Marker: newMarker, Title: title, End: end}
	c.Entries = append(c.Entries[:at], append([]Entry{{Card: card}}, c.Entries[at:]...)...)
	return next
}

// endedEntry is e with end after its last line when that line has no line
// end. A card's last line is its last note, or the card's own line when it
// has no notes. The result shares no card with e.
func endedEntry(e Entry, end string) Entry {
	if e.Card == nil {
		if _, have := cutEnd(e.Line); have == "" {
			e.Line += end
		}
		return e
	}
	cd := *e.Card
	if n := len(cd.Notes); n > 0 {
		if _, have := cutEnd(cd.Notes[n-1]); have == "" {
			cd.Notes = append(append([]string(nil), cd.Notes[:n-1]...), cd.Notes[n-1]+end)
		}
	} else if cd.End == "" {
		cd.End = end
	}
	return Entry{Card: &cd}
}

// lineEnd is the line end of the file that b was read from: the line end of
// column col's heading, or else the first line end in the file. A file
// without a line end takes "\n".
func lineEnd(b Board, col int) string {
	if _, end := cutEnd(b.Columns[col].Heading); end != "" {
		return end
	}
	text := Format(b)
	if i := strings.IndexByte(text, '\n'); i > 0 && text[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// toggled is b with the card that s names marked completed, or marked not
// completed when it is completed. Only the character in the card's box
// changes.
func toggled(b Board, s Selection) Board {
	next := withColumn(b, s.Column)
	entries := next.Columns[s.Column].Entries
	n := -1
	for i, e := range entries {
		if e.Card == nil {
			continue
		}
		if n++; n != s.Card {
			continue
		}
		cd := *e.Card
		box := strings.IndexByte(cd.Marker, '[') + 1
		mark := byte('x')
		if cd.Done() {
			mark = ' '
		}
		cd.Marker = cd.Marker[:box] + string(mark) + cd.Marker[box+1:]
		entries[i] = Entry{Card: &cd}
		break
	}
	return next
}
