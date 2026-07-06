// landing_content.go holds the copy that the sitedocs landing page renders.
// Centralising the strings here lets the layout in landing.go stay
// structural and lets future edits (synthetic-content tweaks, copy review)
// touch a single file.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/vibrantgio/cadence/feature"
	"github.com/vibrantgio/cadence/hero"
	"github.com/vibrantgio/cadence/pricing"
	"github.com/vibrantgio/cadence/testimonial"
)

// heroContent returns Hero props for the landing page. The primary CTA
// fires gotoDocs so the navbar's docs route is reachable from the hero;
// the secondary CTA is wired to a no-op because G5.1b does not depend on
// outbound links.
func heroContent(shaper *text.Shaper, gotoDocs func(gtx layout.Context)) hero.Props {
	return hero.Props{
		Eyebrow:      "Native desktop · Go",
		Title:        "VibrantGIO",
		Subtitle:     "Prism foundation, Cadence patterns, Spectrum platform glue, Pulse motion — a four-phase Gio toolkit.",
		PrimaryCTA:   &hero.CTA{Label: "Get started", OnClick: gotoDocs},
		SecondaryCTA: &hero.CTA{Label: "GitHub", OnClick: func(_ layout.Context) {}},
		Shaper:       shaper,
	}
}

// featureContent returns the 3-up feature grid naming the three foundational
// phases. The fourth phase (Spectrum) is intentionally omitted from this
// grid because the hero subtitle already enumerates all four; surfacing it
// again here would make the row feel padded.
func featureContent() feature.Props {
	return feature.Props{
		Columns: 3,
		Items: []feature.Item{
			{
				Title: "Prism — component foundation",
				Body:  "Theme tokens, primitives, and accessible widgets every layer above builds on.",
			},
			{
				Title: "Cadence — pattern library",
				Body:  "Marketing and product patterns composed from Prism primitives, copy-paste into your own apps.",
			},
			{
				Title: "Pulse — motion + effects",
				Body:  "Frame-driven physics and visual effects sharing the same theme stream as every widget.",
			},
		},
	}
}

// pricingContent returns the synthetic 3-tier pricing row. Tiers are
// distinguished by feature lists rather than the price column so the
// rendering proves the pattern handles uneven bullet counts. The middle
// tier is the Highlighted "Popular" one.
func pricingContent(shaper *text.Shaper) pricing.Props {
	return pricing.Props{
		Shaper: shaper,
		Tiers: []pricing.Tier{
			{
				Name:    "Free",
				Price:   "$0",
				Cadence: "/mo",
				Features: []string{
					"Single project",
					"Community support",
					"Public examples",
				},
				CTA: &pricing.CTA{Label: "Start free"},
			},
			{
				Name:        "Pro",
				Price:       "$19",
				Cadence:     "/mo",
				Highlighted: true,
				Features: []string{
					"Unlimited projects",
					"Priority support",
					"Pulse motion add-on",
					"Private examples",
				},
				CTA: &pricing.CTA{Label: "Upgrade"},
			},
			{
				Name:    "Enterprise",
				Price:   "$99",
				Cadence: "/seat",
				Features: []string{
					"SSO + audit log",
					"Custom theming",
					"Dedicated success engineer",
					"On-prem deployment",
				},
				CTA: &pricing.CTA{Label: "Contact sales"},
			},
		},
	}
}

// testimonialContent returns the 3-card testimonial grid. Quotes are
// synthetic but plausible; author names and roles are illustrative.
func testimonialContent(shaper *text.Shaper) testimonial.Props {
	return testimonial.Props{
		Variant: testimonial.Grid,
		Shaper:  shaper,
		Items: []testimonial.Item{
			{
				Quote:      "Cadence dropped into our docs app on a Friday. By Monday the Hero alone had saved us a week.",
				AuthorName: "Alex Pham",
				AuthorRole: "Founder, Treevue",
			},
			{
				Quote:      "Prism's token model is the first Go design system that matched what our brand team handed us.",
				AuthorName: "Maya Singh",
				AuthorRole: "Design Lead, Northstar",
			},
			{
				Quote:      "Pulse physics let us prototype micro-interactions without bolting on a separate animation runtime.",
				AuthorName: "Owen Reyes",
				AuthorRole: "Engineer, Forecast Labs",
			},
		},
	}
}
