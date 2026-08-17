package main

import (
	"strings"

	marks "github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/mvu"
)

// Model is the single application state: the current search query.
type Model struct {
	Query string
}

// Init returns the seed Model — no query, all icons visible — and no
// startup command.
func Init() (Model, mvu.Command) {
	return Model{}, mvu.DoNothing()
}

// FilterIcons returns the indices into IconTable whose names contain the
// query, case-insensitively. An empty query matches everything. Indices
// (not copies) keep the per-theme prebuilt icon widgets addressable.
func FilterIcons(query string) []int {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := make([]int, 0, len(IconTable))
	for i, icon := range IconTable {
		if query == "" || strings.Contains(strings.ToLower(icon.Name), query) {
			matches = append(matches, i)
		}
	}
	return matches
}

// FilterMarks returns the design system's own marks whose names contain the
// query, case-insensitively, in the set's own order. An empty query matches
// every mark. The names are what a call site writes, so they are what the
// search matches against — and the whole set is four names, so the answer is
// built fresh rather than indexed.
func FilterMarks(query string) []marks.Name {
	query = strings.ToLower(strings.TrimSpace(query))
	all := marks.Names()
	matches := make([]marks.Name, 0, len(all))
	for _, name := range all {
		if query == "" || strings.Contains(strings.ToLower(string(name)), query) {
			matches = append(matches, name)
		}
	}
	return matches
}
