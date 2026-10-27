// pages.go enumerates the route identifiers consumed by the tabbed
// shell. Each identifier is an opaque string carried in
// Model.currentPage; the shell compares the live value against these
// constants to pick which tab is selected. tabPages fixes the strip
// order, so the identifier list and the tab order can never disagree.

package main

const (
	// pageDocs is the Docs tab: the application guide — the workbench
	// root's llms.txt — as one markdown document with its ##/### outline
	// tree in the leading column (guide.go, docs_outline.go).
	pageDocs = "docs"
)

// tabPages is the strip order. tabIndex and the OnSelect wiring both read
// it, so a click's index and the model's page identifier round-trip
// through the one list.
var tabPages = []string{pageDocs}

// tabTitles is what the strip writes on each cell, in tabPages order.
var tabTitles = []string{"Docs"}

// tabIndex maps a route identifier to its strip position. An
// unrecognised identifier lands on the Docs tab, the app's home surface.
func tabIndex(page string) int {
	for i, p := range tabPages {
		if p == page {
			return i
		}
	}
	return 0
}
