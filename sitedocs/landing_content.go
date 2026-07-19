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
// fires gotoDocs; the secondary CTA routes to the About page so no CTA
// is dead UI.
func heroContent(shaper *text.Shaper, gotoDocs, gotoAbout func(gtx layout.Context)) hero.Props {
	return hero.Props{
		Eyebrow:      "Native desktop · Go",
		Title:        "Vibrant Gio",
		Subtitle:     "Prism tokens and primitives, Cadence patterns, Spectrum platform glue, Pulse motion, and an MVU runtime — a design system for building native desktop apps with Gio.",
		PrimaryCTA:   &hero.CTA{Label: "Get started", OnClick: gotoDocs},
		SecondaryCTA: &hero.CTA{Label: "About", OnClick: gotoAbout},
		Shaper:       shaper,
	}
}

// featureContent returns the 3-up feature grid naming the marquee
// layers. Spectrum and MVU are enumerated in the hero subtitle rather
// than repeated here — a five-up row would feel padded.
func featureContent() feature.Props {
	return feature.Props{
		Columns: 3,
		Items: []feature.Item{
			{
				Title: "Prism — tokens & primitives",
				Body:  "Semantic token scales, themable widgets, and a11y helpers every layer above builds on.",
			},
			{
				Title: "Cadence — application patterns",
				Body:  "Shells, tables, modals, navigation and marketing sections — short source, copy into your app and modify.",
			},
			{
				Title: "Pulse — motion + effects",
				Body:  "Springs, tweens, glow and depth sharing the same theme stream as every widget.",
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
