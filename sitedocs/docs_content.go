// docs_content.go holds the copy and code samples that the three docs
// pages render. Centralising the strings here keeps docs.go structural
// and lets a future content review touch a single file. The prose is
// deliberately brief — a sentence or two per paragraph echoing the
// DESIGN.md headings each page summarises.

package main

// gettingStartedContent populates the "Getting started" docs page. The
// content keeps the new-visitor scope tight: install the workspace,
// initialise a window, render a single Cadence pattern.
func gettingStartedContent() docsPageContent {
	return docsPageContent{
		Title: "Getting started",
		Paragraphs: []string{
			"VibrantGIO is a four-phase Gio toolkit: Prism for the foundation, Cadence for patterns, Spectrum for platform glue, Pulse for motion.",
			"Add VibrantGIO to your Go workspace, then bootstrap a window through spectrum/window.New. Every Cadence pattern is a function that takes a theme observable.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Clone the workspace",
				Lines: []string{
					"git clone https://github.com/vibrantgio/vibrantgio",
					"cd vibrantgio",
					"go work sync",
				},
			},
			{
				Caption: "Bootstrap a Spectrum window",
				Lines: []string{
					"mvuWin := mvu.NewWindow(app.Title(\"My App\"))",
					"w := specwin.New(mvuWin, theme.AutoLightDark())",
					"w.Render(buildLayers).Wait()",
				},
			},
		},
	}
}

// phasesOverviewContent populates the "Phases overview" docs page. One
// paragraph per phase, plus two illustrative code samples showing how a
// Prism token consumer and a Cadence pattern subscriber look in practice.
func phasesOverviewContent() docsPageContent {
	return docsPageContent{
		Title: "Phases overview",
		Paragraphs: []string{
			"Prism is the foundation: design tokens (color, type, spacing, radius, motion, elevation) and a small set of primitives. Every layer above consumes Prism's theme observable.",
			"Cadence is the pattern library: navbar, sidebar, hero, feature, pricing, testimonial, breadcrumb, accordion, card, and friends. Patterns are composed from Prism primitives and copy-paste into apps.",
			"Spectrum is platform glue: window bootstrap, live system theme, IPC adapters. Pulse is motion and effects, frame-driven physics layered on the same theme stream.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Consume a Prism color token",
				Lines: []string{
					"colors := rx.SwitchMap(th, func(t theme.Theme)",
					"  rx.Observable[tokens.ColorTokens] {",
					"    return t.Color",
					"})",
				},
			},
			{
				Caption: "Subscribe to a Cadence pattern",
				Lines: []string{
					"heroObs := hero.Hero(th, hero.Props{",
					"  Title:    \"Hello\",",
					"  Subtitle: \"world\",",
					"})",
				},
			},
		},
	}
}

// componentReferenceContent populates the "Component reference" docs
// page. The reference lists a handful of Cadence patterns with the
// canonical "Props + observable" snippet a caller would copy.
func componentReferenceContent() docsPageContent {
	return docsPageContent{
		Title: "Component reference",
		Paragraphs: []string{
			"Each Cadence pattern exports a callable function (Hero, Feature, Card, Breadcrumb, Accordion, ...) that takes a theme observable and Props, and returns an rx.Observable[layout.Widget].",
			"Static Render() variants exist on every pattern for golden-image testing and demos. The Render signature is uniform across patterns that depend on the same token categories.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Breadcrumb pattern",
				Lines: []string{
					"crumbs := breadcrumb.Breadcrumb(th,",
					"  breadcrumb.Props{Items: items, Shaper: sh})",
				},
			},
			{
				Caption: "Card pattern",
				Lines: []string{
					"cardW := card.Card(th, card.Props{",
					"  Header: header, Body: body, Footer: footer,",
					"})",
				},
			},
			{
				Caption: "Accordion pattern",
				Lines: []string{
					"accObs := accordion.Accordion(th, accordion.Props{",
					"  Sections: secs, Open: openObs,",
					"  OnToggle: toggle, SingleOpen: true,",
					"})",
				},
			},
		},
	}
}

