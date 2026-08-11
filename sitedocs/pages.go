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

	// The getting-started slug keeps the old prism name: the docs goldens
	// (testdata/golden/docs-{light,dark}-prism-getting-started.png) derive
	// their filenames from it and its content renders into their pixels, and
	// G-G0D moves no pixels. It renames when those goldens are next
	// deliberately regenerated.
	pageComponentsGettingStarted = "prism-getting-started"
	pageComponentsTokens         = "components-tokens"
	pageComponentsPrimitives     = "components-primitives"

	pageCadencePatterns = "cadence-patterns"
	pageCadenceShells   = "cadence-shells"

	pageThemeWindow = "theme-window"
	pageThemeTheme  = "theme-live-theme"

	pageEffectsMotion  = "effects-motion"
	pageEffectsEffects = "effects-effects"

	pageMVULoop   = "mvu-loop"
	pageMVUWindow = "mvu-window"
)

// pageDocsDefault is where generic "Docs" navigation entries land.
const pageDocsDefault = pageComponentsGettingStarted
