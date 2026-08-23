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

	// pageTheme is the Theme tab: the seed the palette grew from
	// (theme_seed.go), then the themer's palette section — ramps grid and
	// named picks — followed by the inventory's type ladder, following the
	// live theme (theme_tab.go).
	pageTheme = "theme"

	// pageComponents, pagePatterns and pageMarkdown are the three tabs cut
	// from the inventory's own groups (inventory_tabs.go). The inventory
	// publishes four groups; Foundations is not among them here, because
	// the Theme tab is the telling of what a theme is made of.
	pageComponents = "components"
	pagePatterns   = "patterns"
	pageMarkdown   = "markdown"
)

// tabPages is the strip order: Docs, Theme, Components, Patterns,
// Markdown. tabIndex and the OnSelect wiring both read it, so a click's
// index and the model's page identifier round-trip through the one list.
//
// Theme comes second, directly after the guide, because the three tabs
// after it are all drawn in the theme it shows: a reader meets the
// palette and the type ladder before meeting the widgets wearing them.
var tabPages = []string{pageDocs, pageTheme, pageComponents, pagePatterns, pageMarkdown}

// tabLabels is what the strip writes on each cell, in tabPages order.
var tabLabels = []string{"Docs", "Theme", "Components", "Patterns", "Markdown"}

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
