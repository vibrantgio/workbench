// The fictional SimpleApps copy the marketing page renders. The product story
// is provenance, authenticity and custody of sources — not a second notes app,
// and not another company's catalogue.

package main

import (
	"gioui.org/layout"

	"github.com/vibrantgio/patterns/feature"
	"github.com/vibrantgio/patterns/hero"
	"github.com/vibrantgio/patterns/pricing"
	"github.com/vibrantgio/patterns/testimonial"
)

// heroContent returns Hero props. Primary CTA may scroll to pricing;
// secondary stays visual.
func heroContent(seePlans func(gtx layout.Context)) hero.Props {
	return hero.Props{
		Title:        "SimpleApps",
		Subtitle:     "Where did this sentence come from? Who wrote it, and was it generated? The trail stays in the file.",
		PrimaryCTA:   &hero.CTA{Label: "See plans", OnClick: seePlans},
		SecondaryCTA: &hero.CTA{Label: "Learn more"},
	}
}

func featureContent() feature.Props {
	return feature.Props{
		Columns: 3,
		Items: []feature.Item{
			{
				Title: "Provenance",
				Body:  "Every claim keeps the page, the quote and the date it came from. A rewrite does not drop the trail.",
			},
			{
				Title: "Authenticity",
				Body:  "Mark what you wrote, what a model drafted, and what a source attested. The distinction travels with the file.",
			},
			{
				Title: "Custody",
				Body:  "Sources stay on your device. Export a signed bundle, not a screenshot of a chat. No account, no feed.",
			},
		},
	}
}

func pricingContent() pricing.Props {
	return pricing.Props{
		Tiers: []pricing.Tier{
			{
				Name:    "Free",
				Price:   "€0",
				Cadence: "once",
				Features: []string{
					"Provenance on one device",
					"Manual source cards",
					"No ads, no sign-in",
				},
				CTA: &pricing.CTA{Label: "Start free"},
			},
			{
				Name:        "Pro",
				Price:       "€29",
				Cadence:     "once",
				Recommended: true,
				Features: []string{
					"Authenticity marks on every file",
					"Signed export bundles",
					"Phone and desktop, one platform",
					"Import from the browser",
				},
				CTA: &pricing.CTA{Label: "Buy Pro"},
			},
			{
				Name:    "Studio",
				Price:   "€79",
				Cadence: "once",
				Features: []string{
					"Pro on every platform you use",
					"Shared custody for a small team",
					"Attestation keys",
					"Priority mail",
				},
				CTA: &pricing.CTA{Label: "Contact us"},
			},
		},
	}
}

func testimonialContent() testimonial.Props {
	return testimonial.Props{
		Variant: testimonial.Grid,
		Items: []testimonial.Item{
			{
				Quote:      "I can open a brief from last year and still see which paragraph was mine and which was generated. That is the whole product.",
				AuthorName: "Kees de Wit",
				AuthorRole: "Editor, Westferry Review",
			},
			{
				Quote:      "Students must show the source, not the vibe. We keep the trail in the file, not in a slide they will lose.",
				AuthorName: "Amira Haddad",
				AuthorRole: "Lecturer, Harbour College",
			},
			{
				Quote:      "Discovery used to be a folder of unmarked drafts. Now each claim carries its page.",
				AuthorName: "Jonah Eller",
				AuthorRole: "Counsel, North Harbour",
			},
		},
	}
}
