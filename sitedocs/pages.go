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

	pagePatternsPatterns = "patterns"
	// The shells slug keeps the old cadence name: the docs goldens
	// (testdata/golden/docs-{light,dark}-cadence-shells.png) derive their
	// filenames from it and its content renders into their pixels, and
	// G-G0D moves no pixels. It renames when those goldens are next
	// deliberately regenerated.
	pagePatternsShells = "cadence-shells"

	pageThemeWindow = "theme-window"
	pageThemeTheme  = "theme-live-theme"

	pageEffectsMotion  = "effects-motion"
	pageEffectsEffects = "effects-effects"

	pageMVULoop   = "mvu-loop"
	pageMVUWindow = "mvu-window"

	// pageGallery is the Gallery tab: the inventory's live controls in one
	// scrolling column (gallery.go). It joins the three-tab shell in G-AF4.
	pageGallery = "gallery"

	// pageTheme is the Theme tab: the themer's palette section — ramps
	// grid and named picks — following the live theme (theme_tab.go). It
	// joins the three-tab shell in G-AF4.
	pageTheme = "theme"
)

// pageDocsDefault is where generic "Docs" navigation entries land.
const pageDocsDefault = pageComponentsGettingStarted
