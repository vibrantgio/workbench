// pages.go enumerates the route identifiers consumed by routedMain. Each
// identifier is an opaque string published on the currentPage Subject; the
// router compares the live value against these constants to pick which
// page widget to render. New identifiers added here must also be wired
// into routedMain (main.go) and the docs sidebar.

package main

const (
	pageHome                = "home"
	pageDocsGettingStarted  = "docs-getting-started"
	pageDocsPhasesOverview  = "docs-phases-overview"
	pageDocsComponentRef    = "docs-component-reference"
)
