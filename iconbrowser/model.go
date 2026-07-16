package main

import (
	"strings"

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
