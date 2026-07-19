// pages.go enumerates the route identifiers consumed by the shell
// router. Each identifier is an opaque string carried in
// Model.currentPage; the router compares the live value against these
// constants to pick which shell (and, for docs routes, which page) to
// render. New docs identifiers added here must also be wired into
// docsPages (docs_content.go) and the docs sidebar.

package main

const (
	pageHome  = "home"
	pageAbout = "about"

	pagePrismGettingStarted = "prism-getting-started"
	pagePrismTokens         = "prism-tokens"
	pagePrismPrimitives     = "prism-primitives"

	pageCadencePatterns = "cadence-patterns"
	pageCadenceShells   = "cadence-shells"

	pageSpectrumWindow = "spectrum-window"
	pageSpectrumTheme  = "spectrum-live-theme"

	pagePulseMotion  = "pulse-motion"
	pagePulseEffects = "pulse-effects"

	pageMVULoop   = "mvu-loop"
	pageMVUWindow = "mvu-window"
)

// pageDocsDefault is where generic "Docs" navigation entries land.
const pageDocsDefault = pagePrismGettingStarted
