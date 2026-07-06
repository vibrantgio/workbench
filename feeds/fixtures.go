package main

import (
	"fmt"
	"time"
)

// FeedID is the opaque route identifier for a feed entry.
type FeedID string

// ArticleID is the opaque identifier for one article row.
type ArticleID string

type feedEntry struct {
	ID    FeedID
	Label string
}

type feedGroup struct {
	Title   string
	Entries []feedEntry
}

type article struct {
	ID        ArticleID
	FeedID    FeedID
	Title     string
	Author    string
	Published time.Time
	Unread    bool
}

// hardCodedGroups returns the static feed tree used while persistence is
// not yet implemented.
func hardCodedGroups() []feedGroup {
	return []feedGroup{
		{
			Title: "Tech",
			Entries: []feedEntry{
				{ID: "go-blog", Label: "Go Blog"},
				{ID: "hn", Label: "Hacker News"},
				{ID: "lobsters", Label: "Lobste.rs"},
			},
		},
		{
			Title: "News",
			Entries: []feedEntry{
				{ID: "bbc", Label: "BBC World"},
				{ID: "reuters", Label: "Reuters"},
			},
		},
		{
			Title: "Personal",
			Entries: []feedEntry{
				{ID: "my-journal", Label: "My Journal"},
			},
		},
	}
}

// defaultFeedID returns the first feed of the first group. Used to seed
// the selectedFeed Subject so the articles table renders something on
// launch instead of an empty Main slot.
func defaultFeedID() FeedID {
	groups := hardCodedGroups()
	return groups[0].Entries[0].ID
}

// hardCodedArticles returns the static article fixture used while
// persistence is not yet implemented. 14 articles per feed × 6 feeds = 84
// rows total — comfortably above the G5.2b ≥80 threshold and enough to
// produce 9 pages of 10 rows for the largest feed (which is more than
// the table will paginate at a time).
//
// Published timestamps are spaced one day apart so sort-by-Published has
// a strictly-ordered key. The Unread flag toggles every third article so
// both states are visible in any page slice.
func hardCodedArticles() []article {
	type spec struct {
		feed   FeedID
		author string
		titles []string
	}
	specs := []spec{
		{"go-blog", "The Go Team", []string{
			"Go 1.26 Release Notes",
			"Generics: A Year of Lessons",
			"Tracing Goroutine Leaks in Production",
			"Faster Map Iteration in Go 1.26",
			"Profile-Guided Optimisation Revisited",
			"Range-Over-Func: When to Reach For It",
			"Improving the Compiler Diagnostics",
			"Go Modules and the Workspace Pattern",
			"Pprof, Five Years On",
			"Why We Rewrote the GC Pacer",
			"Generics and the Standard Library",
			"Lessons from the Slog Migration",
			"Async-Friendly Patterns in Go",
			"Beyond Channels: Coordination Primitives",
		}},
		{"hn", "submitted", []string{
			"Show HN: A Better Plain-Text Calendar",
			"Postgres 17 Replication: Field Notes",
			"Why I Stopped Using Kubernetes at Home",
			"The Hidden Cost of TypeScript Inference",
			"Rust to Go: A Migration Postmortem",
			"Ask HN: How Do You Handle On-Call Burnout?",
			"WebAssembly Components Are Finally Useful",
			"How Browsers Schedule Animation Frames",
			"The Forgotten History of Smalltalk",
			"Why Latency Beats Throughput in 2026",
			"Show HN: Tiny LLM for Embedded Devices",
			"Static Site Generators Have Gotten Too Complex",
			"On the Death of the Personal Homepage",
			"How We Cut Build Times by 80%",
		}},
		{"lobsters", "various", []string{
			"Type Systems Are Underrated",
			"A Defence of Boring Technology",
			"Notes on the SQLite WAL",
			"Why Vim's Modal Editing Still Wins",
			"Compiler Implementation in Five Weekends",
			"Plan 9 Has More to Teach Us",
			"How I Read Source Code",
			"The Case for Dynamic Languages",
			"Property-Based Testing in Anger",
			"Linux Kernel Locking, Demystified",
			"Reproducible Builds: Lessons Learned",
			"Tail-Call Optimisation Across Compilers",
			"Static Analysis Wins Worth Adopting",
			"Type Inference Without Tears",
		}},
		{"bbc", "BBC Newsroom", []string{
			"Markets React to Central Bank Pause",
			"Climate Summit Reaches Provisional Deal",
			"Election Results: Surprises in the Polls",
			"Tech Regulation Bill Clears Committee",
			"Storms Disrupt Travel Across Europe",
			"Pacific Trade Pact Set to Expand",
			"Universities Face Funding Crunch",
			"AI Safety Conference Concludes",
			"Cancer Trial Shows Promising Results",
			"Energy Grid Stress-Tested in Heat Wave",
			"City Sells Off Historic Waterfront Site",
			"Sports Federation Issues Doping Rules",
			"Migration Policy Faces Court Challenge",
			"New Subsea Cable Goes Live",
		}},
		{"reuters", "Reuters Staff", []string{
			"Currency Markets Steady After Volatility",
			"Earnings Beat Sends Index to Record",
			"Central Bank Signals Slower Hikes",
			"Oil Prices Slip on Demand Concerns",
			"Manufacturing Output Edges Higher",
			"Retail Sales Surprise Analysts",
			"Bond Yields Climb on Inflation Print",
			"Tech Mega-Cap Splits Stock",
			"Trade Talks Resume After Pause",
			"Supply Chain Index Falls Sharply",
			"Housing Starts Rebound from Trough",
			"Service Sector PMI Lifts Outlook",
			"Crypto Custody Rules Draft Released",
			"Insurance Claims Surge After Floods",
		}},
		{"my-journal", "me", []string{
			"Notebook: Morning Walk Through the Park",
			"On Slowing Down at the Right Moments",
			"Reading List for the Quarter",
			"Coffee, Quiet, and Continuous Learning",
			"The Garden Almost Survived the Frost",
			"A Letter I Did Not Send",
			"Late-Night Thoughts on Pacing",
			"Three Habits Worth Keeping",
			"A Year of Saying No More Often",
			"Notes on a Book I Did Not Finish",
			"Why I Keep Coming Back to Pencil",
			"Travel Plans for the Long Weekend",
			"On the Pleasure of Re-Reading",
			"A Short Inventory of Joys",
		}},
	}
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	var out []article
	k := 0
	for _, s := range specs {
		for i, t := range s.titles {
			out = append(out, article{
				ID:        ArticleID(fmt.Sprintf("%s-%02d", s.feed, i+1)),
				FeedID:    s.feed,
				Title:     t,
				Author:    s.author,
				Published: base.AddDate(0, 0, -k),
				Unread:    k%3 == 0,
			})
			k++
		}
	}
	return out
}

// articleByID returns the fixture article with the given ID. A linear scan
// over the 84-row fixture is cheap and runs only on detail-pane emissions
// (article selection, theme change), not per frame.
func articleByID(id ArticleID) (article, bool) {
	for _, a := range hardCodedArticles() {
		if a.ID == id {
			return a, true
		}
	}
	return article{}, false
}

// hardCodedBody returns the static article body shared by the detail pane's
// Reader and Raw tabs. Per the G5.2c spec there is no real formatting: both
// tabs render this same text, differing only in font (proportional vs
// monospace). The paragraphs interpolate the article's title and author so
// switching articles visibly changes the pane.
func hardCodedBody(a article) string {
	return "" +
		a.Title + " — by " + a.Author + ".\n\n" +
		"This is the fixture body for the feeds detail pane. It stands in " +
		"for fetched RSS content while persistence and networking are out " +
		"of scope for Phase 5. The paragraph is long enough to exercise " +
		"line wrapping at typical pane widths, which is the only " +
		"formatting behaviour the Reader tab promises.\n\n" +
		"A second paragraph confirms that paragraph breaks survive the " +
		"single-Label rendering path. The Raw tab renders these exact " +
		"bytes in a monospace face; any visual difference between the two " +
		"tabs beyond typeface is a defect.\n\n" +
		"Published " + a.Published.Format("January 2, 2006") + " in feed “" +
		string(a.FeedID) + "”."
}

// comment is one static placeholder row for the detail pane's Comments tab.
type comment struct {
	Author string
	Text   string
}

// hardCodedComments returns the static placeholder list the Comments tab
// renders for every article. Per the G5.2c spec the tab is a placeholder —
// no per-article comment data exists in the fixtures.
func hardCodedComments() []comment {
	return []comment{
		{Author: "ada", Text: "Great write-up — the second paragraph nails it."},
		{Author: "lin", Text: "Counterpoint: this overstates the trade-offs."},
		{Author: "sam", Text: "Bookmarking this for the weekend read."},
		{Author: "kit", Text: "The fixture comments are remarkably on-topic."},
	}
}
