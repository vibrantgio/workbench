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

	// pageColours is the Colours tab: the shared colour board — every name
	// the platform answers for, with the value it carries in each
	// appearance — following the live theme (theme_tabs.go).
	pageColours = "colours"

	// pageTypography is the Typography tab: the inventory's type scale,
	// every typography role with its name and size, following the live
	// theme (theme_tabs.go).
	pageTypography = "typography"

	// pageComponents, pagePatterns and pageMarkdown are the three tabs cut
	// from the inventory's own groups (inventory_tabs.go). The inventory
	// publishes four groups; Foundations is not among them here, because
	// its two sections are what the Colours and Typography tabs tell.
	pageComponents = "components"
	pagePatterns   = "patterns"
	pageMarkdown   = "markdown"
)

// tabPages is the strip order: Docs, Colours, Typography, Components,
// Patterns, Markdown. tabIndex and the OnSelect wiring both read it, so a
// click's index and the model's page identifier round-trip through the one
// list.
//
// Colours and Typography come directly after the guide, because the three
// tabs after them are all drawn in the theme they show: a reader meets the
// colour set and the type scale before meeting the components wearing them.
// They are two cells rather than one because a colour set and a type scale
// are two different kinds of thing, and a strip cell names one kind.
var tabPages = []string{pageDocs, pageColours, pageTypography, pageComponents, pagePatterns, pageMarkdown}

// tabLabels is what the strip writes on each cell, in tabPages order.
var tabLabels = []string{"Docs", "Colours", "Typography", "Components", "Patterns", "Markdown"}

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
