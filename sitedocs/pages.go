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

	// pageGallery is the Gallery tab: the inventory's live controls in one
	// scrolling column (gallery.go).
	pageGallery = "gallery"

	// pageTheme is the Theme tab: the themer's palette section — ramps
	// grid and named picks — following the live theme (theme_tab.go).
	pageTheme = "theme"
)

// tabPages is the strip order: Docs, Gallery, Theme. tabIndex and the
// OnSelect wiring both read it, so a click's index and the model's page
// identifier round-trip through the one list.
var tabPages = []string{pageDocs, pageGallery, pageTheme}

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
