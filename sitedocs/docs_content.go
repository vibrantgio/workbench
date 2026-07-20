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
	// Layer names the ecosystem layer the page documents (Prism, Cadence,
	// Spectrum, Pulse, MVU); it becomes the middle breadcrumb.
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
		{ID: pagePrismGettingStarted, Layer: "Prism", Title: "Getting started"},
		{ID: pagePrismTokens, Layer: "Prism", Title: "Tokens & theme"},
		{ID: pagePrismPrimitives, Layer: "Prism", Title: "Primitives"},
		{ID: pageCadencePatterns, Layer: "Cadence", Title: "Patterns"},
		{ID: pageCadenceShells, Layer: "Cadence", Title: "Shells"},
		{ID: pageSpectrumWindow, Layer: "Spectrum", Title: "Window & system"},
		{ID: pageSpectrumTheme, Layer: "Spectrum", Title: "Live theme"},
		{ID: pagePulseMotion, Layer: "Pulse", Title: "Motion"},
		{ID: pagePulseEffects, Layer: "Pulse", Title: "Effects"},
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
