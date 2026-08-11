// docs_content.go binds the docs routes to their markdown sources. The
// page copy lives in content/*.md — one file per sidebar link, embedded
// at build time — and renders through vibrantgio/markdown (docs.go). This
// file is the remaining glue: the route registry with each page's
// breadcrumb metadata and embedded source.

package main

import "embed"

//go:embed content/*.md
var docsContentFS embed.FS

// docsPageDef binds a route identifier to its breadcrumb metadata and
// markdown source. docsPages is the single source of truth the router and
// sidebar both consume, so a link can never point at a page that does not
// exist.
type docsPageDef struct {
	ID string
	// Layer names the ecosystem layer the page documents (Components, Cadence,
	// Theme, Effects, MVU); it becomes the middle breadcrumb.
	Layer string
	// Title is the page title, the trailing breadcrumb.
	Title string
	// Source is the page's embedded markdown, content/<ID>.md.
	Source []byte
}

// docsPages returns every docs page in sidebar order, with its markdown
// source loaded from the embedded content directory.
func docsPages() []docsPageDef {
	defs := []docsPageDef{
		// Getting started keeps Layer "Prism": the layer is the middle
		// breadcrumb, which renders inside the page's golden viewport
		// (docs-{light,dark}-prism-getting-started.png), and G-G0D moves no
		// pixels. It becomes "Components" when those goldens are next
		// deliberately regenerated.
		{ID: pageComponentsGettingStarted, Layer: "Prism", Title: "Getting started"},
		{ID: pageComponentsTokens, Layer: "Components", Title: "Tokens & theme"},
		{ID: pageComponentsPrimitives, Layer: "Components", Title: "Primitives"},
		{ID: pageCadencePatterns, Layer: "Cadence", Title: "Patterns"},
		{ID: pageCadenceShells, Layer: "Cadence", Title: "Shells"},
		{ID: pageThemeWindow, Layer: "Theme", Title: "Window & system"},
		{ID: pageThemeTheme, Layer: "Theme", Title: "Live theme"},
		{ID: pageEffectsMotion, Layer: "Effects", Title: "Motion"},
		{ID: pageEffectsEffects, Layer: "Effects", Title: "Effects"},
		{ID: pageMVULoop, Layer: "MVU", Title: "The loop"},
		{ID: pageMVUWindow, Layer: "MVU", Title: "Reactive window"},
	}
	for i := range defs {
		src, err := docsContentFS.ReadFile("content/" + defs[i].ID + ".md")
		if err != nil {
			// Unreachable when the registry and content/ agree; the embed
			// is checked at build time and TestDocsPageConstructs covers
			// every route.
			panic("sitedocs: missing docs source for " + defs[i].ID + ": " + err.Error())
		}
		defs[i].Source = src
	}
	return defs
}
