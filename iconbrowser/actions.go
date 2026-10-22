package main

// SetQuery replaces the search query; the grid is narrowed to it on every
// keystroke. Posted by the search field's OnChange as an mvu.MessageOp.
type SetQuery struct {
	Text string
}
