package main

// SetQuery replaces the search query; the grid re-filters on every
// keystroke. Emitted by the search field's OnChange via mvu.MessageOp.
type SetQuery struct {
	Text string
}
