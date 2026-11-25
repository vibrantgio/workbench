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
// after the card's notes. The new line takes the file's line end.
func withCard(b Board, col int, title string) Board {
	card := &Card{Marker: newMarker, Title: title, End: lineEnd(b, col)}
	return inserted(b, col, len(b.Columns[col].Cards()), card)
}

// inserted is b with cd in column col, at index k among the column's
// cards: before the card that has index k, or after the last card and its
// notes when k is the card count. A column without a card takes cd after
// the blank line under its heading, where the plugin writes a column's
// first card. A line before cd that has no line end takes the file's line
// end, and so does cd's last line when a line comes after it. The result
// holds cd itself when cd's last line has a line end, and a copy of cd
// otherwise.
func inserted(b Board, col, k int, cd *Card) Board {
	end := lineEnd(b, col)
	next := withColumn(b, col)
	c := &next.Columns[col]
	at, last, n := -1, -1, 0
	for i, e := range c.Entries {
		if e.Card == nil {
			continue
		}
		if n == k {
			at = i
			break
		}
		n, last = n+1, i
	}
	if at < 0 {
		at = last + 1
		if last < 0 && len(c.Entries) > 0 && c.Entries[0].Card == nil && strings.TrimSpace(c.Entries[0].Line) == "" {
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
	entry := Entry{Card: cd}
	if !ended(cd) && (at < len(c.Entries) || col < len(next.Columns)-1) {
		entry = endedEntry(entry, end)
	}
	c.Entries = append(c.Entries[:at:at], append([]Entry{entry}, c.Entries[at:]...)...)
	return next
}

// moved is b with the card that from names taken out of its column and put
// in column to.Column at index to.Card, with its notes. The lines around
// the card's old place stay.
func moved(b Board, from, to Selection) Board {
	cd := cardsOf(b.Columns[from.Column])[from.Card]
	return inserted(withoutCard(b, from), to.Column, to.Card, cd)
}

// ended reports whether the last line of cd, its last note or else its
// own line, has a line end.
func ended(cd *Card) bool {
	if n := len(cd.Notes); n > 0 {
		_, e := cutEnd(cd.Notes[n-1])
		return e != ""
	}
	return cd.End != ""
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

// retitled is b with the title of the card that s names set to title. Only
// the card's title changes, and the result shares no card that it changes
// with b.
func retitled(b Board, s Selection, title string) Board {
	next := withColumn(b, s.Column)
	entries := next.Columns[s.Column].Entries
	n := -1
	for i, e := range entries {
		if e.Card == nil {
			continue
		}
		if n++; n == s.Card {
			cd := *e.Card
			cd.Title = title
			entries[i] = Entry{Card: &cd}
			break
		}
	}
	return next
}
